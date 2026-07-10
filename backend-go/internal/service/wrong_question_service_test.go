// Package service 错题本服务测试，使用内存 sqlite 数据库。
package service

import (
	"testing"

	"gorm.io/gorm"

	"forklift-training/internal/model"
	"forklift-training/internal/testutil"
)

func newWrongQuestionSvc(t *testing.T) (*WrongQuestionService, *gorm.DB) {
	t.Helper()
	db := testutil.NewMemoryDB(t)
	return NewWrongQuestionService(db), db
}

func seedWrongQuestion(t *testing.T, db *gorm.DB, studentID, questionID, wrongCount int) {
	t.Helper()
	wq := model.WrongQuestion{
		StudentID:   studentID,
		QuestionID:  questionID,
		WrongCount:  wrongCount,
		LastWrongAt: testutil.Now(),
		CreatedAt:   testutil.Now(),
	}
	if err := db.Create(&wq).Error; err != nil {
		t.Fatalf("插入错题失败: %v", err)
	}
}

// --- GetWrongQuestions ---

func TestGetWrongQuestions_Empty(t *testing.T) {
	svc, _ := newWrongQuestionSvc(t)
	result := svc.GetWrongQuestions(1, 1, 20, "", nil, nil)
	if result["total"].(int64) != 0 {
		t.Fatalf("空库总数应为 0, got %v", result["total"])
	}
}

func TestGetWrongQuestions_WithData(t *testing.T) {
	svc, db := newWrongQuestionSvc(t)
	testutil.SeedQuestion(t, db, "single_choice", "beginner", "错题1", "A")
	seedWrongQuestion(t, db, 1, 1, 3)
	seedWrongQuestion(t, db, 1, 2, 1)

	result := svc.GetWrongQuestions(1, 1, 20, "", nil, nil)
	if result["total"].(int64) != 2 {
		t.Fatalf("总数应为 2, got %v", result["total"])
	}
}

func TestGetWrongQuestions_DefaultPaging(t *testing.T) {
	svc, _ := newWrongQuestionSvc(t)
	result := svc.GetWrongQuestions(1, 0, 0, "", nil, nil)
	if result["page"].(int) != 1 {
		t.Fatalf("默认页码应为 1, got %v", result["page"])
	}
	if result["page_size"].(int) != 20 {
		t.Fatalf("默认页大小应为 20, got %v", result["page_size"])
	}
}

// --- RemoveWrongQuestion ---

func TestRemoveWrongQuestion_Success(t *testing.T) {
	svc, db := newWrongQuestionSvc(t)
	testutil.SeedQuestion(t, db, "single_choice", "beginner", "test", "A")
	seedWrongQuestion(t, db, 1, 1, 2)

	result, err := svc.RemoveWrongQuestion(1, 1)
	if err != nil {
		t.Fatalf("移除错题失败: %v", err)
	}
	if removed, ok := result["removed"].(bool); !ok || !removed {
		t.Fatalf("应返回 removed=true, got %v", result["removed"])
	}
}

func TestRemoveWrongQuestion_NotFound(t *testing.T) {
	svc, _ := newWrongQuestionSvc(t)
	_, err := svc.RemoveWrongQuestion(1, 9999)
	if err == nil {
		t.Fatal("不存在的错题应返回错误")
	}
}

// --- GetStats ---

func TestGetStats_WrongQuestion_Empty(t *testing.T) {
	svc, _ := newWrongQuestionSvc(t)
	result := svc.GetStats(1)
	if result == nil {
		t.Fatal("GetStats 不应返回 nil")
	}
}

func TestGetStats_WrongQuestion_WithData(t *testing.T) {
	svc, db := newWrongQuestionSvc(t)
	testutil.SeedQuestion(t, db, "single_choice", "beginner", "q1", "A")
	seedWrongQuestion(t, db, 1, 1, 3)
	seedWrongQuestion(t, db, 1, 2, 1)

	result := svc.GetStats(1)
	if result["total"] == nil {
		t.Fatal("应返回 total")
	}
	if result["total"].(int) != 2 {
		t.Fatalf("总数应为 2, got %v", result["total"])
	}
}

// --- ExportWrongQuestions ---

func TestExportWrongQuestions_Empty(t *testing.T) {
	svc, _ := newWrongQuestionSvc(t)
	result := svc.ExportWrongQuestions(1)
	if len(result) != 0 {
		t.Fatalf("空库导出应 0 条, got %d", len(result))
	}
}

func TestExportWrongQuestions_WithData(t *testing.T) {
	svc, db := newWrongQuestionSvc(t)
	testutil.SeedQuestion(t, db, "single_choice", "beginner", "导出错题", "A")
	seedWrongQuestion(t, db, 1, 1, 2)

	result := svc.ExportWrongQuestions(1)
	if len(result) != 1 {
		t.Fatalf("应导出 1 条, got %d", len(result))
	}
}

// --- FormatWrongQuestionsText (纯函数) ---

func TestFormatWrongQuestionsText_Empty(t *testing.T) {
	text := FormatWrongQuestionsText([]map[string]interface{}{})
	if text == "" {
		t.Fatal("空列表应返回非空文本（标题）")
	}
}

func TestFormatWrongQuestionsText_WithData(t *testing.T) {
	data := []map[string]interface{}{
		{
			"question_id":   1,
			"content":       "叉车检查要点",
			"type":          "single_choice",
			"level":         "beginner",
			"wrong_count":   3,
			"last_wrong_at": "2026-06-01T10:00:00",
		},
	}
	text := FormatWrongQuestionsText(data)
	if text == "" {
		t.Fatal("文本不应为空")
	}
	// 验证包含题干内容
	if !containsStr(text, "叉车检查要点") {
		t.Fatalf("文本应包含题干: %s", text)
	}
}

func containsStr(s, sub string) bool {
	return len(s) >= len(sub) && (s == sub || indexOf(s, sub) >= 0)
}

func indexOf(s, sub string) int {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return i
		}
	}
	return -1
}

// --- RedoWrongQuestion ---

func TestRedoWrongQuestion_Correct(t *testing.T) {
	svc, db := newWrongQuestionSvc(t)
	q := testutil.SeedQuestion(t, db, "single_choice", "beginner", "重做题", "A")
	seedWrongQuestion(t, db, 1, q.ID, 2)

	result, err := svc.RedoWrongQuestion(1, q.ID, "A")
	if err != nil {
		t.Fatalf("重做失败: %v", err)
	}
	if result == nil {
		t.Fatal("结果不应为 nil")
	}
}

func TestRedoWrongQuestion_Wrong(t *testing.T) {
	svc, db := newWrongQuestionSvc(t)
	q := testutil.SeedQuestion(t, db, "single_choice", "beginner", "重做题", "A")
	seedWrongQuestion(t, db, 1, q.ID, 2)

	result, err := svc.RedoWrongQuestion(1, q.ID, "B")
	if err != nil {
		t.Fatalf("答错也不应报错: %v", err)
	}
	if result == nil {
		t.Fatal("结果不应为 nil")
	}
}

func TestRedoWrongQuestion_NotInWrongList(t *testing.T) {
	svc, db := newWrongQuestionSvc(t)
	q := testutil.SeedQuestion(t, db, "single_choice", "beginner", "test", "A")
	// 不在错题本中
	_, err := svc.RedoWrongQuestion(1, q.ID, "A")
	if err == nil {
		t.Fatal("不在错题本中应返回错误")
	}
}
