// 练习 / 模考 / 打标域的 nullable 行为例（批①-B 第③段）。
//
// 机制见 nullable_declaration_test.go 的 nullableOutletTables。
//
// 本段两格各自代表一种「nullable 是真的」的不同来源，分开记免得下一批按同一把尺子量：
//   - ProgressResultDTO.answers_state ⇒ **没有 error 出口的读面**：GetProgress 返回的是裸 DTO
//     （不是 (*DTO, error)），「没进度」按 200 + 零值答，于是那一格留 nil map。
//     `Get()` 的零值 map 与空 map 是两件事：`map[string]any{}` 发 `{}`，nil 发 `null`，
//     而这条出口走的正是前者都没写过的高档。
//   - MockExamSubmitDTO.details ⇒ **同一条声明经两个出口**：Submit 那侧是 `make(0,n)` 恒非 null，
//     GetResult 那侧在未交卷（result 列为空）时留零值 DTO ⇒ details 是 nil。
//     而 MockExamResultDTO 内嵌本 DTO、共用这一条声明，所以「恒非 null」在那条 GET 上是谎话。
//   - QuestionTagsResultDTO.tag_ids ⇒ **请求体回显**（该格的证据已随域包搬去
//     internal/training/nullable_outlets_test.go，波 3b-2；判词原文随之搬走）。
package service

import (
	"testing"

	"go.uber.org/zap"

	"forklift-training/internal/model"

	"forklift-training/internal/testutil"
)

var nullableOutletsPractice = map[string]func(t *testing.T) any{
	"service.ProgressResultDTO.answers_state": outletProgressWithoutSavedAnswers,
	"service.MockExamSubmitDTO.details":       outletMockExamResultUnsent,
}

func init() {
	nullableOutletTables = append(nullableOutletTables, nullableOutletsPractice)
}

// outletProgressWithoutSavedAnswers 一名学员、一场练习都没存过 ⇒ answers_state 是 nil map。
func outletProgressWithoutSavedAnswers(t *testing.T) any {
	t.Helper()
	db := testutil.NewMemoryDB(t)
	student := testutil.SeedStudent(t, db, "无进度学员", "x")
	svc := NewPracticeModeService(db, nil, zap.NewNop())
	return svc.GetProgress(student.ID, "sequential", nil)
}

// outletMockExamResultUnsent 已开考未交卷的模考：result 列没写过 ⇒ GetResult 里那个
// `var result MockExamSubmitDTO` 保持零值，内嵌进响应后 details 发 null（HTTP 仍是 200）。
func outletMockExamResultUnsent(t *testing.T) any {
	t.Helper()
	db := testutil.NewMemoryDB(t)
	student := testutil.SeedStudent(t, db, "模考学员", "x")
	exam := model.MockExam{
		ID: 9301, StudentID: student.ID, Status: "in_progress",
		QuestionIDs: model.JSONB([]byte(`[1,2]`)),
		CreatedAt:   testutil.Now(),
	}
	if err := db.Create(&exam).Error; err != nil {
		t.Fatalf("播未交卷模考失败: %v", err)
	}
	res, err := NewMockExamService(db, nil, zap.NewNop()).GetResult(exam.ID, student.ID)
	if err != nil {
		t.Fatalf("取未交卷模考结果失败: %v", err)
	}
	return res
}
