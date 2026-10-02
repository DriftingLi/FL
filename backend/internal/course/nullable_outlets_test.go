// 课程域的 nullable 行为例（域包版本；由 internal/service/nullable_declaration_test.go 拆出，波 3b-1）。
//
// 机制见本文件下半段的 nullableOutletTables（分域表 + init 并进汇总，runner 逐条执行，
// 不是给 AST 查名字的花名册）。**域包不得 import internal/service 的测试文件**（ADR-0070 决策 9：
// 证据表可住域包），所以这里自带一份形状相同的 runner，照抄
// internal/service/nullable_declaration_test.go。键格式 = <包名>.<类型>.<json键>，
// 登记处是 internal/apitypes/nullability_lock_test.go 的 nullableEvidenceSources
// （它按**目录**收表：域包一行）。
package course

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
	"course.ChapterSlidesDTO.slides": outletChapterSlidesWithoutRenderer,
}

// nullableOutletTables 是域包这张 nullable 表的汇总点；runner 逐条执行它（理由同留驻侧
// internal/service/nullable_declaration_test.go：一张只有 AST 在看的表就是花名册）。
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
			got := testutil.MarshalKey(t, build(t), jsonKey)
			if got != "null" {
				t.Fatalf("声明 nullable 的 %s 实际发出 %s——这句表态没有出口证明，"+
					"要么找一条真发 null 的出口，要么按实测改判 nonnil", key, got)
			}
		})
	}
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
	crs := model.Course{Name: "台账课程", Status: 1, SpecialtyID: &spec.SpecialtyID, LevelID: &lv.LevelID}
	if err := db.Create(&crs).Error; err != nil {
		t.Fatalf("播种课程失败: %v", err)
	}
	ch := model.Chapter{
		CourseID: crs.CourseID, Title: "台账章节", OrderNum: 1,
		ContentType: "ppt", FileURL: "/uploads/ledger.ppt",
	}
	if err := db.Create(&ch).Error; err != nil {
		t.Fatalf("播种章节失败: %v", err)
	}
	// slideRenderer 传 nil 就是生产上「未配置转图能力」那一档，不是为测试造的分支。
	out, err := NewService(db, nil, zap.NewNop()).GetChapterSlides(ch.ChapterID, 1)
	if err != nil {
		t.Fatalf("取幻灯片失败: %v", err)
	}
	return out
}
