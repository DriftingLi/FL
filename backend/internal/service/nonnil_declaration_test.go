// 表态为 nonnil 的字段必须真的恒非 null（ADR-0064 决策 8 的行为例那一半 + ADR-0065 决策 5 判据 5）。
//
// 为什么单独立这条：tag 是一句**关于出口的断言**，而断言可以被任何一次重构悄悄破坏——
// 有人把 `out := make([]T, 0, n)` 改成 `var out []T` 时，读源码那把表态锁不会红，
// 只有走真实出口、看发出的是 `[]` 还是 `null` 才会红。
// 刻意不 marshal 零值 DTO：零值切片必然是 nil，那不构成反例。
//
// 本文件是判据 5 的**证据源**：`nullableOutlets` 那张表的键由 apitypes 的锁按 AST 读取，
// 一条 `nullability:"nonnil"` 的响应字段若不在这里，锁当场红（零容忍，见 nullability_lock_test.go
// 判据 5 的理由）。表按「出口」组织而不是按用例名组织，是因为一个出口常常同时举证多条字段
// ——例如课程详情一次调用就把 chapters 与 prerequisites 两格都证明了。
package service

import (
	"encoding/json"
	"strings"
	"testing"

	"go.uber.org/zap"
	"gorm.io/gorm"

	"forklift-training/internal/model"

	"forklift-training/internal/testutil"
)

// marshalKey 把出口结果序列化，取某个键发出的字面量（"[]" 还是 "null"）。
func marshalKey(t *testing.T, v any, key string) string {
	t.Helper()
	b, err := json.Marshal(v)
	if err != nil {
		t.Fatalf("序列化失败: %v", err)
	}
	var m map[string]json.RawMessage
	if err := json.Unmarshal(b, &m); err != nil {
		t.Fatalf("响应不是对象: %s", b)
	}
	got, ok := m[key]
	if !ok {
		t.Fatalf("响应里没有键 %q（body=%s）—— 该字段被加了 omitempty？那 nonnil 表态就不成立了", key, b)
	}
	return string(got)
}

// 一条本文件的用例测不到的边界，登记在此以免被下一个人重新踩：
//
//	marshalKey 只看「这一次出口发出了什么」，所以它**分不清**「代码保证非 null」与
//	「列默认值恰好是 '[]'」。同一形状的三格 JSONArray（recruit_service.go 的脱敏卡）就是例子：
//	重建过的那格恒 `[]`，只加 len==0 守卫的两格在列里存着 JSON `null` 时照样发出 null。
//	⇒ 改判 JSONArray / JSONB 字段前必须去读它那条投影，而不是只跑一次出口。
//	repository.AlgorithmParameters 那四格也在改判范围之外：判据 5 的证据源只扫 ../service 与
//	../api（这两个包没有可脱离真库跑的出口），所以它们会一直留在判据 4 的账上——那是结构性
//	的够不着，不是没人去举证。
//
// nonnilOutletTables 是**分域的若干张表**的汇总点。
//
// 为什么不是一张表：69 处改判一次写进一个文件会变成没人能读完的巨型测试，也无法并行推进。
// 每个域自己声明一张 `nonnilOutlets<域名>` 并在 init 里并进来；apitypes 那把锁按变量名前缀
// 扫目录收键（见 nullability_lock_test.go 的 outletSource），所以「表在哪」由前缀决定，
// 不需要在新加文件时回来改这里的一行清单——那正是一个会漏改的第二宿主。
var nonnilOutletTables []map[string]func(t *testing.T) any

func init() {
	nonnilOutletTables = append(nonnilOutletTables, nonnilOutletsCore)
}

// allNonNilOutlets 展开所有分表。同名键出现在两张表里即判红：一条事实两处举证，改一处另一处还在过。
func allNonNilOutlets(t *testing.T) map[string]func(t *testing.T) any {
	t.Helper()
	out := map[string]func(t *testing.T) any{}
	owner := map[string]int{}
	for i, tbl := range nonnilOutletTables {
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

// nonnilOutletsCore 键 = 包名.类型名.json键，值 = 走真实出口取到的结果。
// 一条键对应一次真实调用；同一类型多条字段可以共用一次调用（各占一键、各自 marshal）。
var nonnilOutletsCore = map[string]func(t *testing.T) any{
	"service.RecruitListResult.items":        outletRecruitListEmpty,
	"service.QuestionPageDTO.questions":      outletQuestionPageEmptyPool,
	"service.QuestionImportResultDTO.errors": outletQuestionImportEmpty,
	"service.CourseDetailDTO.chapters":       outletCourseDetailNoChapters,
}

func outletRecruitListEmpty(t *testing.T) any {
	t.Helper()
	svc := NewRecruitService(testutil.NewMemoryDB(t), zap.NewNop())
	res, err := svc.List(RecruitListParams{Page: 1, PageSize: 20})
	if err != nil {
		t.Fatalf("空库拉列表失败: %v", err)
	}
	return res
}

func outletQuestionPageEmptyPool(t *testing.T) any {
	t.Helper()
	svc := NewQuestionBankService(testutil.NewMemoryDB(t), nil, zap.NewNop())
	res, err := svc.ListPoolQuestions(1, 20, "", "", nil, NewQuestionReadScope(nil), "")
	if err != nil {
		t.Fatalf("空池列表失败: %v", err)
	}
	return res
}

func outletQuestionImportEmpty(t *testing.T) any {
	t.Helper()
	svc := NewQuestionBankService(testutil.NewMemoryDB(t), nil, zap.NewNop())
	return svc.BatchImport(nil, nil)
}

func outletCourseDetailNoChapters(t *testing.T) any {
	t.Helper()
	db := testutil.NewMemoryDB(t)
	svc := NewCourseService(db, nil, zap.NewNop())
	res, err := svc.GetCourseDetail(seedVisibleCourse(t, db), 0)
	if err != nil {
		t.Fatalf("课程详情失败: %v", err)
	}
	return res
}

// TestNonNilDeclaredOutletsNeverEmitNull 每条登记过的出口都必须发出**非 null**，键也必须在场。
//
// 判据原本写成「必须等于 `[]`」，那对切片是对的、对映射是错的：`map[K]V` 的恒非 null 形状是 `{}`，
// 于是 4 条 map 字段被判据自己挡在举证之外——一条只认一种形状的锁会把它没覆盖的那一类留在原地，
// 而那正是第②批 B 段与批⑤ 反复踩过的同一个坑。marshalKey 在键缺席时判红，所以
// 「键在 + 值不是 null」两件事一起断言，改名或被 omitempty 掉都跑不掉。
func TestNonNilDeclaredOutletsNeverEmitNull(t *testing.T) {
	outlets := allNonNilOutlets(t)
	if len(outlets) == 0 {
		t.Fatal("证据表是空的：判据 5 会因此空转，这里必须同步红")
	}
	for key, build := range outlets {
		t.Run(key, func(t *testing.T) {
			jsonKey := key[strings.LastIndex(key, ".")+1:]
			if got := marshalKey(t, build(t), jsonKey); got == "null" {
				t.Errorf("声明 nonnil 的 %s 实际发出 null", key)
			}
		})
	}
}

// seedVisibleCourse 播一门「已发布 + 已挂载」但**没有章节**的课程，返回其 id。
func seedVisibleCourse(t *testing.T, db *gorm.DB) int {
	t.Helper()
	spec := model.Specialty{Code: "nonnil", Name: "非空方向", SortOrder: 1, Status: 1}
	if err := db.Create(&spec).Error; err != nil {
		t.Fatalf("播种方向失败: %v", err)
	}
	lv := model.CourseLevel{Code: "nonnil-lv", Name: "非空等级", SortOrder: 1, Status: 1}
	if err := db.Create(&lv).Error; err != nil {
		t.Fatalf("播种等级失败: %v", err)
	}
	pos := spec.SpecialtyID
	lvl := lv.LevelID
	course := model.Course{Name: "非空课程", Status: 1, SpecialtyID: &pos, LevelID: &lvl, CreatedAt: testutil.Now()}
	if err := db.Create(&course).Error; err != nil {
		t.Fatalf("播种课程失败: %v", err)
	}
	return course.CourseID
}
