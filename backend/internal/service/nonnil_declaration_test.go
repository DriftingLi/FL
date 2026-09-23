// 表态为 nonnil 的字段必须真的恒非 null（ADR-0064 决策 8 表态锁的行为例那一半）。
//
// 为什么单独立这条：tag 是一句**关于出口的断言**，而断言可以被任何一次重构悄悄破坏——
// 有人把 `out := make([]T, 0, n)` 改成 `var out []T` 时，表态锁（读源码那把）不会红，
// 只有走真实出口、看发出的是 `[]` 还是 `null` 才会红。
// 刻意不 marshal 零值 DTO：零值切片必然是 nil，那不构成反例。
package service

import (
	"encoding/json"
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

func TestNonNilDeclaredOutletsEmitEmptyArrays(t *testing.T) {
	t.Run("RecruitListResult.items", func(t *testing.T) {
		db := testutil.NewMemoryDB(t)
		svc := NewRecruitService(db, zap.NewNop())
		res, err := svc.List(RecruitListParams{Page: 1, PageSize: 20})
		if err != nil {
			t.Fatalf("空库拉列表失败: %v", err)
		}
		if got := marshalKey(t, res, "items"); got != "[]" {
			t.Errorf("声明 nonnil 却发出 %s", got)
		}
	})

	t.Run("QuestionPageDTO.questions（学员面空池）", func(t *testing.T) {
		db := testutil.NewMemoryDB(t)
		svc := NewQuestionBankService(db, nil, zap.NewNop())
		res, err := svc.ListPoolQuestions(1, 20, "", "", nil, NewQuestionReadScope(nil), "")
		if err != nil {
			t.Fatalf("空池列表失败: %v", err)
		}
		if got := marshalKey(t, res, "questions"); got != "[]" {
			t.Errorf("声明 nonnil 却发出 %s", got)
		}
	})

	// ContactRequestListResult.items 与 QuestionImportResultDTO.errors 两条不在这里验：
	// 前者的 items 由 api/contact.go 从 ListForStudent 的切片组装（切片那半边已由
	// contact_company_disable_contract_test.go 覆盖），后者只在批量导入出口成形，
	// 其非 nil 初始化在源码里有明示。剩下两条要补的是 HTTP 层空集例，不是把本表拉长。

	t.Run("CourseDetailDTO.chapters（无章节的可见课程）", func(t *testing.T) {
		db := testutil.NewMemoryDB(t)
		svc := NewCourseService(db, nil, zap.NewNop())
		courseID := seedVisibleCourse(t, db)
		res, err := svc.GetCourseDetail(courseID, 0)
		if err != nil {
			t.Fatalf("课程详情失败: %v", err)
		}
		if got := marshalKey(t, res, "chapters"); got != "[]" {
			t.Errorf("声明 nonnil 却发出 %s", got)
		}
	})
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
