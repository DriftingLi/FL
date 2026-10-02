// 表态为 nonnil 的字段必须真的恒非 null（ADR-0064 决策 8 的行为例那一半 + ADR-0065 决策 5 判据 5）。
//
// 为什么单独立这条：tag 是一句**关于出口的断言**，而断言可以被任何一次重构悄悄破坏——
// 有人把 `out := make([]T, 0, n)` 改成 `var out []T` 时，读源码那把表态锁不会红，
// 只有走真实出口、看发出的是 `[]` 还是 `null` 才会红。
// 刻意不 marshal 零值 DTO：零值切片必然是 nil，那不构成反例。
//
// 本文件是判据 5 的**证据源**：`nonnilOutlets*` 那些表的键由 apitypes 的锁按 AST 读取，
// 一条 `nullability:"nonnil"` 的响应字段若不在这里，锁当场红（零容忍，见 nullability_lock_test.go
// 判据 5 的理由）。表按「出口」组织而不是按用例名组织，是因为一个出口常常同时举证多条字段
// ——例如课程详情一次调用就把 chapters 与 prerequisites 两格都证明了。
package service

import (
	"testing"

	"go.uber.org/zap"
	"gorm.io/gorm"

	"forklift-training/internal/model"

	"forklift-training/internal/testutil"
)

// 一条本文件的用例测不到的边界，登记在此以免被下一个人重新踩：
//
//	判据只跑一次出口，**分不清**「代码保证非 null」与「列默认值恰好是 '[]'」。同一形状的三格
//	JSONArray（recruit_service.go 的脱敏卡）就是例子：重建过的那格恒 `[]`，只加 len==0 守卫的
//	两格在列里存着 JSON `null` 时照样发出 null。⇒ 改判 JSONArray / JSONB 字段前必须去读那条投影，
//	而不是只跑一次出口（盲区在同处登记：testutil/nonnil.go 的 MarshalKey）。
//	repository.AlgorithmParameters 那四格也在改判范围之外：判据 5 的证据源只扫
//	../service、../api 与 ../faq 三处的 `nonnilOutlets*` 表（这些包没有可脱离真库跑的出口），
//	所以它们会一直留在判据 4 的账上——那是结构性的够不着，不是没人去举证。
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

// nonnilOutletsCore 键 = 包名.类型名.json键，值 = 走真实出口取到的结果。
// 一条键对应一次真实调用；同一类型多条字段可以共用一次调用（各占一键、各自 marshal）。
var nonnilOutletsCore = map[string]func(t *testing.T) any{
	"service.RecruitListResult.items":        outletRecruitListEmpty,
	"service.QuestionPageDTO.questions":      outletQuestionPageEmptyPool,
	"service.QuestionImportResultDTO.errors": outletQuestionImportEmpty,
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

// TestNonNilDeclaredOutletsNeverEmitNull 本包（含各分域表）的举证入口。判据本体在
// testutil.AssertNonNilOutlets：逐键断言「键在场 + 不是 null」，另有「同一事实被两张表各自举证」
// 与「表是空的」两处判红 —— 域包拆出去之后（ADR-0070）各域包调**同一份**，不再各抄一个 runner。
func TestNonNilDeclaredOutletsNeverEmitNull(t *testing.T) {
	testutil.AssertNonNilOutlets(t, nonnilOutletTables)
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
