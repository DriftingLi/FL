// 模考 / 打标域的 nullable 行为例（批①-B 第③段；本文件的练习那一格已随域包搬去
// internal/practicemode/nullable_outlets_test.go，ADR-0070 波 3c-2）。
//
// 机制见 nullable_declaration_test.go 的 nullableOutletTables。
//
// MockExamSubmitDTO.details ⇒ **同一条声明经两个出口**：Submit 那侧是 `make(0,n)` 恒非 null，
// GetResult 那侧在未交卷（result 列为空）时留零值 DTO ⇒ details 是 nil。
// 而 MockExamResultDTO 内嵌本 DTO、共用这一条声明，所以「恒非 null」在那条 GET 上是谎话。
//
// QuestionTagsResultDTO.tag_ids ⇒ **请求体回显**（该格的证据已随域包搬去
// internal/training/nullable_outlets_test.go，波 3b-2；判词原文随之搬走）。
package service

import (
	"testing"

	"go.uber.org/zap"

	"forklift-training/internal/model"

	"forklift-training/internal/testutil"
)

var nullableOutletsExam = map[string]func(t *testing.T) any{
	"service.MockExamSubmitDTO.details": outletMockExamResultUnsent,
}

func init() {
	nullableOutletTables = append(nullableOutletTables, nullableOutletsExam)
}

// outletMockExamResultUnsent 已开考未交卷的模考：result 列没写过 ⇒ GetResult 里那个
// `var result MockExamSubmitDTO` 保持零值，内嵌进响应后 details 发 null（HTTP 仍是 200）。
func outletMockExamResultUnsent(t *testing.T) any {
	t.Helper()
	db := testutil.NewMemoryDB(t)
	student := testutil.SeedStudent(t, db, "模考学员", "x")
	exam := model.MockExam{
		ID: 9301, StudentID: student.ID, Status: "in_progress",
		QuestionIDs: model.JSONB([]byte("`[1,2]`")),
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
