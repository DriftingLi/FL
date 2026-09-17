package api

import (
	"bytes"
	"encoding/json"
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"path/filepath"
	"reflect"
	"runtime"
	"sort"
	"strings"
	"testing"

	"forklift-training/pkg/paging"
)

// 信封登记表的锁（ADR-0056 §1 / issue #1095）：
//
//  1. 形状锁：每行声明的 Keys 必须与 Sample 实际 marshal 出来的顶层 key **顺序**逐字一致
//     （键序是契约，ADR-0009 §2）；方言必须与键集合自洽（pages 与 page_size 不共存）。
//  2. 覆盖锁：源码里每个「含 total + 切片字段」的导出 struct 都必须在登记表内
//     （信封或载荷）；反向也查——登记表里的名字必须还在源码里存在。
//  3. 判定面自测：覆盖锁不是空转（空登记表跑扫描必须报出已知类型）。

// jsonKeyOrder 取 v marshal 后的顶层 key 顺序。
func jsonKeyOrder(t *testing.T, v any) []string {
	t.Helper()
	raw, err := json.Marshal(v)
	if err != nil {
		t.Fatalf("marshal 失败: %v", err)
	}
	dec := json.NewDecoder(bytes.NewReader(raw))
	tok, err := dec.Token()
	if err != nil {
		t.Fatalf("读 token 失败: %v", err)
	}
	if d, ok := tok.(json.Delim); !ok || d != '{' {
		t.Fatalf("顶层不是 JSON 对象（信封必须是对象）: %s", raw)
	}
	var keys []string
	for dec.More() {
		kt, err := dec.Token()
		if err != nil {
			t.Fatalf("读 key 失败: %v", err)
		}
		keys = append(keys, kt.(string))
		var skip json.RawMessage
		if err := dec.Decode(&skip); err != nil {
			t.Fatalf("跳过 value 失败: %v", err)
		}
	}
	return keys
}

func containsString(list []string, want string) bool {
	for _, s := range list {
		if s == want {
			return true
		}
	}
	return false
}

// TestEnvelopeRegistryShapeLock 形状锁：键集合 + 键序 + 方言 + 类型名逐行对齐。
func TestEnvelopeRegistryShapeLock(t *testing.T) {
	rows := Envelopes()
	if len(rows) == 0 {
		t.Fatal("信封登记表为空")
	}
	for _, row := range rows {
		t.Run(row.Result, func(t *testing.T) {
			if row.Sample == nil {
				t.Fatalf("%s 缺 Sample（键序锁无法验证）", row.Result)
			}
			// 泛型实例化的 reflect 名会带上类型实参的完整包路径
			//（paging.ItemsPage[forklift-training/internal/model.X]），登记表里写可读的短名，这里归一化模块前缀后比较。
			typeName := strings.ReplaceAll(reflect.TypeOf(row.Sample).String(), "forklift-training/internal/", "")
			if typeName != row.Result {
				t.Fatalf("Result = %q, Sample 的 reflect 类型名 = %q", row.Result, typeName)
			}
			got := jsonKeyOrder(t, row.Sample)
			if !reflect.DeepEqual(got, row.Keys) {
				t.Fatalf("键序漂移（ADR-0009 §2）：实际 %v, 登记 %v", got, row.Keys)
			}
			seen := map[string]bool{}
			for _, k := range row.Keys {
				if seen[k] {
					t.Fatalf("键 %q 重复登记", k)
				}
				seen[k] = true
			}
			if !seen["total"] {
				t.Fatalf("%s 是列表信封但没有 total 键: %v", row.Result, row.Keys)
			}
			hasPages, hasPageSize := seen["pages"], seen["page_size"]
			switch row.Dialect {
			case paging.DialectPages:
				if !hasPages || hasPageSize {
					t.Fatalf("方言 pages 与键集合不符: %v", row.Keys)
				}
			case paging.DialectPageSize:
				if !hasPageSize || hasPages {
					t.Fatalf("方言 page_size 与键集合不符: %v", row.Keys)
				}
			case paging.DialectNone:
				if hasPages || hasPageSize {
					t.Fatalf("方言 none 但键集合含分页元数据: %v", row.Keys)
				}
			default:
				t.Fatalf("方言未登记: %q", row.Dialect)
			}
		})
	}
}

// TestEnvelopeRegistryEndpointsUnique 端点跨行唯一：每个端点恰有一行声明（「每端点一行」的机检）。
func TestEnvelopeRegistryEndpointsUnique(t *testing.T) {
	seen := map[string]string{}
	for _, row := range Envelopes() {
		if len(row.Endpoints) == 0 {
			t.Errorf("%s 未登记端点", row.Result)
		}
		for _, ep := range row.Endpoints {
			if prev, ok := seen[ep]; ok {
				t.Errorf("端点 %s 被重复登记：%s / %s", ep, prev, row.Result)
			}
			seen[ep] = row.Result
		}
	}
}

// moduleRoot 返回 backend 模块根（本测试文件位于 backend/internal/api/）。
func moduleRoot(t *testing.T) string {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller 失败")
	}
	return filepath.Dir(filepath.Dir(filepath.Dir(file)))
}

// scanTotalListTypes 扫描模块源码，返回「含 json:"total" 且至少一个切片字段」的**导出** struct 类型
// （包名.类型名）。跳过 paging 包（它是装配原语与登记表的宿主）与非导出类型（不出现在跨包响应里）。
func scanTotalListTypes(t *testing.T, root string) []string {
	t.Helper()
	var out []string
	fset := token.NewFileSet()
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			if d.Name() == ".git" || d.Name() == "node_modules" || strings.HasPrefix(d.Name(), ".") {
				return fs.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return nil
		}
		rel, relErr := filepath.Rel(root, path)
		if relErr != nil {
			return relErr
		}
		if filepath.ToSlash(filepath.Dir(rel)) == "pkg/paging" {
			return nil
		}
		file, perr := parser.ParseFile(fset, path, nil, 0)
		if perr != nil {
			t.Fatalf("解析 %s 失败: %v", rel, perr)
		}
		for _, decl := range file.Decls {
			gd, ok := decl.(*ast.GenDecl)
			if !ok || gd.Tok != token.TYPE {
				continue
			}
			for _, spec := range gd.Specs {
				ts, ok := spec.(*ast.TypeSpec)
				if !ok || !ts.Name.IsExported() || ts.TypeParams != nil {
					continue
				}
				st, ok := ts.Type.(*ast.StructType)
				if !ok {
					continue
				}
				hasTotal, hasSlice := false, false
				for _, field := range st.Fields.List {
					if field.Tag != nil && strings.Contains(field.Tag.Value, `json:"total"`) {
						hasTotal = true
					}
					if _, ok := field.Type.(*ast.ArrayType); ok {
						hasSlice = true
					}
				}
				if hasTotal && hasSlice {
					out = append(out, file.Name.Name+"."+ts.Name.Name)
				}
			}
		}
		return nil
	})
	if err != nil {
		t.Fatalf("走查源码失败: %v", err)
	}
	sort.Strings(out)
	return out
}

// TestEnvelopeRegistryCoversTotalListTypes 覆盖锁（两向）：
// 源码里每个含 total + 切片字段的导出类型都在登记表内；登记表里的名字也都还在源码里。
func TestEnvelopeRegistryCoversTotalListTypes(t *testing.T) {
	registered := map[string]bool{}
	fromSource := map[string]bool{}
	for _, row := range Envelopes() {
		registered[row.Result] = true
	}
	for _, p := range Payloads() {
		if p.Reason == "" {
			t.Errorf("载荷登记 %s 缺理由", p.Result)
		}
		registered[p.Result] = true
	}
	found := scanTotalListTypes(t, moduleRoot(t))
	for _, name := range found {
		fromSource[name] = true
		if !registered[name] {
			t.Errorf("含 total 的列表结果类型 %s 未登记（信封 EnvelopeSpec 或载荷 PayloadSpec）", name)
		}
	}
	for name := range registered {
		// paging.ItemsPage[T] 的实例化不出现在源码扫描面（泛型声明在 paging 包内，已跳过）。
		if strings.HasPrefix(name, "paging.") {
			continue
		}
		if !fromSource[name] {
			t.Errorf("登记表里的 %s 在源码里找不到（类型改名/删除后登记表未同步）", name)
		}
	}
}

// TestEnvelopeCoverageDetectsMissingRegistration 判定面自测：
// 扫描器必须真的能报出漏登记的类型（否则覆盖锁是空转的假绿）。
func TestEnvelopeCoverageDetectsMissingRegistration(t *testing.T) {
	found := scanTotalListTypes(t, moduleRoot(t))
	for _, want := range []string{
		"api.AuditLogPageResult",
		"model.ListBatteryResponse",
		"service.QuestionPageDTO",
		"service.FavoritePageResult",
	} {
		if !containsString(found, want) {
			t.Fatalf("覆盖锁判定面失效：扫描未报出 %s（扫描结果 %v）", want, found)
		}
	}
}
