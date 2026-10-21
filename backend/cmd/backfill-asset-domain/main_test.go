package main

// 回填命令的三条锁（票 #1742）：
//   1. 清单覆盖锁：model 包里凡「URL 形态」的列，必须在回填清单里出现或登记豁免理由，
//      且清单自陈的 text/jsonb 形态要与 model 的 gorm tag 对得上
//      —— 漏一列或标错形态，都是这类一次性迁移最典型的静默失败；
//   2. 改写语义的 PG 契约测试：真实 Postgres（随机 schema）上验证 dry-run 不落库、
//      -write 只换 scheme+host（路径/中文/空格原样）、第二次跑命中 0 行；
//   3. 原子性：中途有一列失败时整轮回滚，不留下「改了一半」的库。
//
// 本机无 DATABASE_URL 时契约测试干净 skip（首跑在 CI），与仓库既有 PG 契约纪律一致。

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"forklift-training/internal/testutil"
)

// exemptColumns 已实测存在、但**不是**资源 URL 的列（键 = 表.列，值 = 理由）。
// 登记而非忽略：将来新增同类列会让本锁判红，必须要么纳入清单、要么在此写明理由。
var exemptColumns = map[string]string{
	"ai_configs.base_url":     "AI 服务端点，不是对象存储里的资源 URL",
	"ai_user_models.base_url": "AI 服务端点，不是对象存储里的资源 URL",
}

// assetColumnPattern URL 形态的列名判据（与 model 包的实际命名对齐）。
// 只收「整名结尾」的形态，避免 file_name / content_type 这类同前缀误判。
func assetColumnPattern() *regexp.Regexp {
	return regexp.MustCompile(`(?:_url|_urls|_image|_images|^photos|^images|^slides)$`)
}

// fieldInfo model 里的一列：列名 + 是否 jsonb（由 gorm tag 的 type:jsonb 判定）。
type fieldInfo struct {
	name  string
	jsonb bool
}

// TestTargetsCoverModelColumns 清单覆盖锁：model 里所有 URL 形态的列都要有归属。
//
// 判据按 (表, 列) 而不是只按列名：不同表可以各有一张 `cover_image`，
// 只按列名判会让「某张表漏了」被同名的另一张表掩盖（本锁的首版就是这么错过的）。
// 同一趟里核对 text/jsonb：标错形态只有生产首跑才会报 `position(unknown, jsonb)`。
func TestTargetsCoverModelColumns(t *testing.T) {
	covered := map[string]columnKind{}
	for _, tg := range defaultTargets {
		for _, c := range tg.Columns {
			covered[tg.Table+"."+c.Name] = c.Kind
		}
	}

	pat := assetColumnPattern()
	tagRe := regexp.MustCompile(`column:([a-z_0-9]+)`)
	dir := filepath.Join("..", "..", "internal", "model")
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("读 model 目录失败: %v", err)
	}

	type structInfo struct {
		file   string
		table  string
		fields []fieldInfo
	}
	byName := map[string]*structInfo{}
	pending := []*ast.FuncDecl{}
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".go") || strings.HasSuffix(e.Name(), "_test.go") {
			continue
		}
		fset := token.NewFileSet()
		file, err := parser.ParseFile(fset, filepath.Join(dir, e.Name()), nil, 0)
		if err != nil {
			t.Fatalf("解析 %s 失败: %v", e.Name(), err)
		}
		ast.Inspect(file, func(n ast.Node) bool {
			switch node := n.(type) {
			case *ast.TypeSpec:
				st, ok := node.Type.(*ast.StructType)
				if !ok || st.Fields == nil {
					return true
				}
				info := &structInfo{file: e.Name()}
				for _, f := range st.Fields.List {
					if f.Tag == nil {
						continue
					}
					tag := strings.Trim(f.Tag.Value, "`")
					m := tagRe.FindStringSubmatch(tag)
					if m == nil {
						continue
					}
					info.fields = append(info.fields, fieldInfo{
						name:  m[1],
						jsonb: strings.Contains(tag, "type:jsonb"),
					})
				}
				byName[node.Name.Name] = info
			case *ast.FuncDecl:
				if node.Name.Name == "TableName" && node.Recv != nil && len(node.Recv.List) == 1 {
					pending = append(pending, node)
				}
			}
			return true
		})
	}

	// TableName() 的返回值把「结构体 → 真实表名」接上（表名是回填清单里的键）。
	for _, fn := range pending {
		recv, ok := fn.Recv.List[0].Type.(*ast.Ident)
		if !ok {
			continue
		}
		info, ok := byName[recv.Name]
		if !ok || fn.Body == nil || len(fn.Body.List) != 1 {
			continue
		}
		ret, ok := fn.Body.List[0].(*ast.ReturnStmt)
		if !ok || len(ret.Results) != 1 {
			continue
		}
		lit, ok := ret.Results[0].(*ast.BasicLit)
		if !ok || lit.Kind != token.STRING {
			continue
		}
		info.table = strings.Trim(lit.Value, "\"")
	}

	checked, seenCols := 0, 0
	for name, info := range byName {
		if info.table == "" || len(info.fields) == 0 {
			continue
		}
		for _, f := range info.fields {
			seenCols++
			if !pat.MatchString(f.name) {
				continue
			}
			checked++
			key := info.table + "." + f.name
			kind, ok := covered[key]
			if ok {
				wantJSONB := kind == kindJSONB
				if wantJSONB != f.jsonb {
					t.Errorf("%s 的存储形态与清单不符：model %s / 清单 %s", key, modelKind(f.jsonb), kind)
				}
				continue
			}
			if reason, ok := exemptColumns[key]; ok {
				t.Logf("已登记的豁免列：%s（%s）—— %s", key, name, reason)
				continue
			}
			t.Errorf("model 列 %s 是 URL 形态但不在回填清单里，也没登记豁免理由（定义见 %s）", key, info.file)
		}
	}
	if checked == 0 || seenCols == 0 {
		t.Fatal("没从 model 包里解析到任何 URL 形态的列：判据失效（标签形态或 TableName 写法变了？）")
	}

	// 反向：清单里的 (表, 列) 必须真的存在（拼错 / 改名时立刻暴露，而不是运行时静默 0 行）。
	exists := map[string]bool{}
	for _, info := range byName {
		if info.table == "" {
			continue
		}
		for _, f := range info.fields {
			exists[info.table+"."+f.name] = true
		}
	}
	for key := range covered {
		if !exists[key] {
			t.Errorf("回填清单里的 %s 在 model 包里不存在（表名/列名拼错或已改名）", key)
		}
	}
	// 豁免表自身也要活着：被豁免的列不存在时，这条豁免就是一条死条目。
	for key := range exemptColumns {
		if !exists[key] {
			t.Errorf("豁免表里的 %s 在 model 包里不存在（死条目，删掉或改键）", key)
		}
	}
}

func modelKind(jsonb bool) columnKind {
	if jsonb {
		return kindJSONB
	}
	return kindText
}

// TestRunRewritesOnlySchemeAndHost PG 契约测试：真实 Postgres 上跑改写器。
func TestRunRewritesOnlySchemeAndHost(t *testing.T) {
	db := testutil.NewPostgresDB(t)

	if err := db.Exec(`CREATE TABLE asset_backfill_probe (
		id serial PRIMARY KEY,
		text_col text,
		json_col jsonb
	)`).Error; err != nil {
		t.Fatalf("建探针表失败: %v", err)
	}

	const from = "https://www.gccsmile.com/"
	const to = "https://cdn.gccsmile.com/"
	targets := []target{{Table: "asset_backfill_probe", Columns: []column{
		{Name: "text_col", Kind: kindText},
		{Name: "json_col", Kind: kindJSONB},
	}}}

	// 带空格与中文的真实文件名形态：只该换前缀，其余逐字保留。
	const hardPath = "featured/屏幕截图 2026-08-28 134456_1787903856173.webp"
	if err := db.Exec(`INSERT INTO asset_backfill_probe (text_col, json_col) VALUES
		(?, ?::jsonb),
		(?, ?::jsonb),
		(?, ?::jsonb)`,
		"封面 "+from+hardPath, `["`+from+hardPath+`"]`,
		"外站 https://cdn.other.example/featured/x.webp", `["https://cdn.other.example/featured/x.webp"]`,
		"没有链接的一段文字", `[]`,
	).Error; err != nil {
		t.Fatalf("播种失败: %v", err)
	}

	// dry-run：两列各命中 1 行，库不变。
	stats, err := run(db, targets, from, to, false)
	if err != nil {
		t.Fatalf("dry-run 失败: %v", err)
	}
	if stats[0].Matched != 1 || stats[1].Matched != 1 {
		t.Fatalf("dry-run 报的命中行数 = text %d / json %d，期望各 1", stats[0].Matched, stats[1].Matched)
	}
	if stats[0].Changed != 0 || stats[1].Changed != 0 {
		t.Fatalf("dry-run 竟然报了改写行数：%+v", stats)
	}
	var before string
	if err := db.Raw("SELECT text_col FROM asset_backfill_probe ORDER BY id LIMIT 1").Scan(&before).Error; err != nil {
		t.Fatalf("dry-run 后读库失败: %v", err)
	}
	if !strings.Contains(before, from) {
		t.Fatalf("dry-run 改动了数据：%q", before)
	}

	// -write：两列都换掉，且只换前缀。
	stats, err = run(db, targets, from, to, true)
	if err != nil {
		t.Fatalf("写入失败: %v", err)
	}
	if stats[0].Changed != 1 || stats[1].Changed != 1 {
		t.Fatalf("改写行数 = text %d / json %d，期望各 1", stats[0].Changed, stats[1].Changed)
	}
	var gotText, gotJSON string
	if err := db.Raw("SELECT text_col, json_col::text FROM asset_backfill_probe ORDER BY id LIMIT 1").Row().Scan(&gotText, &gotJSON); err != nil {
		t.Fatalf("改写后读库失败: %v", err)
	}
	if want := "封面 " + to + hardPath; gotText != want {
		t.Fatalf("文本列改写结果不对：\n  got  = %q\n  want = %q", gotText, want)
	}
	if !strings.Contains(gotJSON, to+hardPath) || strings.Contains(gotJSON, from) {
		t.Fatalf("JSONB 列改写结果不对：%q", gotJSON)
	}

	// 外站与无链接行原样保留。
	rows, err := db.Raw("SELECT text_col FROM asset_backfill_probe ORDER BY id OFFSET 1").Rows()
	if err != nil {
		t.Fatalf("读剩余行失败: %v", err)
	}
	defer rows.Close()
	var others []string
	for rows.Next() {
		var s string
		if err := rows.Scan(&s); err != nil {
			t.Fatalf("扫描剩余行失败: %v", err)
		}
		others = append(others, s)
	}
	if len(others) != 2 || !strings.Contains(others[0], "cdn.other.example") || others[1] != "没有链接的一段文字" {
		t.Fatalf("非本站行被改动：%v", others)
	}

	// 幂等：第二次跑命中 0 行。
	stats, err = run(db, targets, from, to, true)
	if err != nil {
		t.Fatalf("第二次写入失败: %v", err)
	}
	for _, s := range stats {
		if s.Matched != 0 || s.Changed != 0 {
			t.Fatalf("第二遍不幂等：%+v", s)
		}
	}
}

// TestRunRollsBackOnFailure 原子性：第 k 列失败时，前 k-1 列不许留在库里。
func TestRunRollsBackOnFailure(t *testing.T) {
	db := testutil.NewPostgresDB(t)

	if err := db.Exec(`CREATE TABLE asset_backfill_rollback (
		id serial PRIMARY KEY,
		text_col text
	)`).Error; err != nil {
		t.Fatalf("建探针表失败: %v", err)
	}
	const from = "https://www.gccsmile.com/"
	const to = "https://cdn.gccsmile.com/"
	if err := db.Exec("INSERT INTO asset_backfill_rollback (text_col) VALUES (?)", from+"featured/a.webp").Error; err != nil {
		t.Fatalf("播种失败: %v", err)
	}

	// 第二列不存在：第一列若已提交，就会出现「改了一半」的库。
	targets := []target{{Table: "asset_backfill_rollback", Columns: []column{
		{Name: "text_col", Kind: kindText},
		{Name: "missing_col", Kind: kindText},
	}}}
	if _, err := run(db, targets, from, to, true); err == nil {
		t.Fatal("打不存在的列竟然成功了：事务没有拦住失败")
	}

	var got string
	if err := db.Raw("SELECT text_col FROM asset_backfill_rollback LIMIT 1").Scan(&got).Error; err != nil {
		t.Fatalf("失败后读库失败: %v", err)
	}
	if !strings.Contains(got, from) {
		t.Fatalf("失败后第一列被改动了（没有回滚）：%q", got)
	}
}
