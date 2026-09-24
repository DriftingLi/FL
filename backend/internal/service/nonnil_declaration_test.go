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

// nonnilOutlets 键 = 包名.类型名.json键，值 = 走真实出口取到的结果。
// 一条键对应一次真实调用；同一类型多条字段可以共用一次调用（各占一键、各自 marshal）。
var nonnilOutlets = map[string]func(t *testing.T) any{
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

// TestNonNilDeclaredOutletsEmitEmptyArrays 每条登记过的出口都必须发出 `[]`，不是 `null`。
// 键名最后一段就是要看的 JSON 键；对不上（改名 / 被 omitempty 掉）由 marshalKey 判红。
func TestNonNilDeclaredOutletsEmitEmptyArrays(t *testing.T) {
	if len(nonnilOutlets) == 0 {
		t.Fatal("证据表是空的：判据 5 会因此空转，这里必须同步红")
	}
	for key, build := range nonnilOutlets {
		t.Run(key, func(t *testing.T) {
			jsonKey := key[strings.LastIndex(key, ".")+1:]
			if got := marshalKey(t, build(t), jsonKey); got != "[]" {
				t.Errorf("声明 nonnil 的 %s 实际发出 %s", key, got)
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
