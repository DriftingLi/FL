// 表态为 nullable 的字段必须真的发得出 null（ADR-0065 决策 5，表态锁判据 4 的证据源）。
//
// 为什么单独立这条：`nullability:"nullable"` 是一句**关于出口的断言**，而它今天只需要被写出来
// 就能成立——没有任何测试去举出那个 null。第 3 条（x-nullable 债务）管的是「说了可空，契约有没有
// 跟着改」，管不到「这句可空是不是真的」。本文件把后者变成证据：走真实出口、marshal 一次、
// 断言发出的就是 `null`。
//
// 与 nonnil_declaration_test.go 是对称的两半：那边断言「声明 nonnil 的出口发 `[]`」，
// 这边断言「声明 nullable 的出口发 `null`」。两张表都只能是**跑过**的，
// apitypes 那把锁按本文件的表键核账（见 nullability_lock_test.go 判据 4）。
package service

import (
	"strings"
	"testing"

	"go.uber.org/zap"

	"forklift-training/internal/model"
	"forklift-training/internal/testutil"
)

// nullableOutlets 键 = 包名.类型名.json键（与判据 4 的账同一格式），值 = 走真实出口取到的结果。
//
// 新增一条 nullable 声明时必须同时在这里留一行，否则 apitypes 的判据 4 当场点名；
// 反过来，把某条改判成 nonnil 而不删这里的键，会被「表里有键、字段却不是 nullable」那半边抓到。
var nullableOutlets = map[string]func(t *testing.T) any{
	"service.ChapterSlidesDTO.slides": outletChapterSlidesWithoutRenderer,
}

// nullableOutletTables 是若干张**分域 nullable 表**的汇总点，形状与 nonnil 那侧对称
// （见 nonnil_declaration_test.go 的同名变量与理由：一条表态的证据住在它自己那一层，
// 一个域一张表比把所有出口堆进一个巨型测试更好读）。
//
// 为什么必须有这张汇总点而不是各文件自带 runner：判据 4 的键由 apitypes 按变量名前缀
// **从 AST 读**，而「读得到名字」不等于「跑过」——一张只有 AST 在看的表就是花名册
// （HEAD 那笔 CI 修红修的就是 nonnil 那侧的这个洞）。这里的表被下面的 runner 逐条执行，
// 所以新加一个 `nullableOutlets<域名>` 文件只需在自己的 init 里 append 一行。
var nullableOutletTables []map[string]func(t *testing.T) any

func init() {
	nullableOutletTables = append(nullableOutletTables, nullableOutlets)
}

// allNullableOutlets 展开所有分表。同名键出现在两张表里即判红：一条事实两处举证，改一处另一处还在过。
func allNullableOutlets(t *testing.T) map[string]func(t *testing.T) any {
	t.Helper()
	out := map[string]func(t *testing.T) any{}
	owner := map[string]int{}
	for i, tbl := range nullableOutletTables {
		for k, v := range tbl {
			if prev, dup := owner[k]; dup {
				t.Fatalf("%s 被第 %d 张与第 %d 张表各自举证——一条事实一个证据，留一处", k, prev, i)
			}
			owner[k] = i
			out[k] = v
		}
	}
	return out
}

// outletChapterSlidesWithoutRenderer 真出口：章节挂了 PPT 但服务没注入 slideRenderer ⇒
// generateSlides 直接返回 nil，DTO 的 slides 键发出 `null`（不是 `[]`）。
func outletChapterSlidesWithoutRenderer(t *testing.T) any {
	t.Helper()
	db := testutil.NewMemoryDB(t)
	spec := model.Specialty{Code: "nle", Name: "台账方向", SortOrder: 1, Status: 1}
	if err := db.Create(&spec).Error; err != nil {
		t.Fatalf("播种方向失败: %v", err)
	}
	lv := model.CourseLevel{Code: "nle-lv", Name: "台账等级", SortOrder: 1, Status: 1}
	if err := db.Create(&lv).Error; err != nil {
		t.Fatalf("播种等级失败: %v", err)
	}
	course := model.Course{Name: "台账课程", Status: 1, SpecialtyID: &spec.SpecialtyID, LevelID: &lv.LevelID}
	if err := db.Create(&course).Error; err != nil {
		t.Fatalf("播种课程失败: %v", err)
	}
	ch := model.Chapter{
		CourseID: course.CourseID, Title: "台账章节", OrderNum: 1,
		ContentType: "ppt", FileURL: "/uploads/ledger.ppt",
	}
	if err := db.Create(&ch).Error; err != nil {
		t.Fatalf("播种章节失败: %v", err)
	}
	// slideRenderer 传 nil 就是生产上「未配置转图能力」那一档，不是为测试造的分支。
	svc := NewCourseService(db, nil, zap.NewNop())
	out, err := svc.GetChapterSlides(ch.ChapterID, 1)
	if err != nil {
		t.Fatalf("取幻灯片失败: %v", err)
	}
	return out
}

// TestNullableDeclaredOutletsEmitNull 每条登记过的出口都必须真的发出 null。
//
// 键名反推 json 键：表键最后一段就是要看的键；对不上（字段被改名、被 omitempty 掉、
// 或整个类型不在射程里）一律判红，不留「找不到就当通过」的分支。
func TestNullableDeclaredOutletsEmitNull(t *testing.T) {
	outlets := allNullableOutlets(t)
	if len(outlets) == 0 {
		t.Fatal("证据表是空的：判据 4 会因此空转，这里必须同步红")
	}
	for key, build := range outlets {
		t.Run(key, func(t *testing.T) {
			jsonKey := key[strings.LastIndex(key, ".")+1:]
			got := marshalKey(t, build(t), jsonKey)
			if got != "null" {
				t.Fatalf("声明 nullable 的 %s 实际发出 %s——这句表态没有出口证明，"+
					"要么找一条真发 null 的出口，要么按实测改判 nonnil", key, got)
			}
		})
	}
}
