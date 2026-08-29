// Package service 错题本服务测试，使用内存 sqlite 数据库。
package service

import (
	"testing"
	"time"

	"go.uber.org/zap"
	"gorm.io/gorm"

	"forklift-training/internal/model"
	"forklift-training/internal/testutil"
)

func newWrongQuestionSvc(t *testing.T) (*WrongQuestionService, *gorm.DB) {
	t.Helper()
	db := testutil.NewMemoryDB(t)
	return NewWrongQuestionService(db, nil, zap.NewNop()), db
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
	result := svc.GetWrongQuestions(1, 1, 20, "", nil, false, "")
	if result["total"].(int64) != 0 {
		t.Fatalf("空库总数应为 0, got %v", result["total"])
	}
}

func TestGetWrongQuestions_WithData(t *testing.T) {
	svc, db := newWrongQuestionSvc(t)
	testutil.SeedQuestion(t, db, "single_choice", "错题1", "A")
	seedWrongQuestion(t, db, 1, 1, 3)
	seedWrongQuestion(t, db, 1, 2, 1)

	result := svc.GetWrongQuestions(1, 1, 20, "", nil, false, "")
	if result["total"].(int64) != 2 {
		t.Fatalf("总数应为 2, got %v", result["total"])
	}
}

func TestGetWrongQuestions_DefaultPaging(t *testing.T) {
	svc, _ := newWrongQuestionSvc(t)
	result := svc.GetWrongQuestions(1, 0, 0, "", nil, false, "")
	if result["page"].(int) != 1 {
		t.Fatalf("默认页码应为 1, got %v", result["page"])
	}
	if result["page_size"].(int) != 20 {
		t.Fatalf("默认页大小应为 20, got %v", result["page_size"])
	}
}

func TestGetWrongQuestions_FavoritedFilter(t *testing.T) {
	svc, db := newWrongQuestionSvc(t)
	testutil.SeedQuestion(t, db, "single_choice", "错题1", "A")
	testutil.SeedQuestion(t, db, "single_choice", "错题2", "B")
	seedWrongQuestion(t, db, 1, 1, 3)
	seedWrongQuestion(t, db, 1, 2, 1)
	if err := db.Create(&model.Favorite{UserID: 1, TargetType: "question", TargetID: 1, CreatedAt: testutil.Now()}).Error; err != nil {
		t.Fatalf("插入收藏失败: %v", err)
	}

	result := svc.GetWrongQuestions(1, 1, 20, "", nil, true, "")
	if result["total"].(int64) != 1 {
		t.Fatalf("收藏过滤后总数应为 1, got %v", result["total"])
	}
	items := result["items"].([]map[string]any)
	if len(items) != 1 || items[0]["question_id"].(int) != 1 {
		t.Fatalf("收藏过滤后应仅剩题目 1, got %v", items)
	}
}

func TestGetWrongQuestions_SortAsc(t *testing.T) {
	svc, db := newWrongQuestionSvc(t)
	testutil.SeedQuestion(t, db, "single_choice", "早错题", "A")
	testutil.SeedQuestion(t, db, "single_choice", "晚错题", "B")
	now := testutil.Now()
	early := model.WrongQuestion{StudentID: 1, QuestionID: 1, WrongCount: 1, LastWrongAt: now.Add(-2 * time.Hour), CreatedAt: now}
	late := model.WrongQuestion{StudentID: 1, QuestionID: 2, WrongCount: 1, LastWrongAt: now, CreatedAt: now}
	if err := db.Create(&early).Error; err != nil {
		t.Fatalf("插入错题失败: %v", err)
	}
	if err := db.Create(&late).Error; err != nil {
		t.Fatalf("插入错题失败: %v", err)
	}

	result := svc.GetWrongQuestions(1, 1, 20, "", nil, false, "time_asc")
	items := result["items"].([]map[string]any)
	if len(items) != 2 || items[0]["question_id"].(int) != 1 {
		t.Fatalf("升序时首项应为较早错误的题目 1, got %v", items)
	}

	result = svc.GetWrongQuestions(1, 1, 20, "", nil, false, "")
	items = result["items"].([]map[string]any)
	if len(items) != 2 || items[0]["question_id"].(int) != 2 {
		t.Fatalf("默认降序时首项应为最近错误的题目 2, got %v", items)
	}
}

func TestGetWrongQuestions_FavoritedField(t *testing.T) {
	svc, db := newWrongQuestionSvc(t)
	testutil.SeedQuestion(t, db, "single_choice", "错题1", "A")
	testutil.SeedQuestion(t, db, "single_choice", "错题2", "B")
	seedWrongQuestion(t, db, 1, 1, 3)
	seedWrongQuestion(t, db, 1, 2, 1)
	fav := model.Favorite{UserID: 1, TargetType: "question", TargetID: 1, CreatedAt: testutil.Now()}
	if err := db.Create(&fav).Error; err != nil {
		t.Fatalf("插入收藏失败: %v", err)
	}

	result := svc.GetWrongQuestions(1, 1, 20, "", nil, false, "")
	items := result["items"].([]map[string]any)
	byQID := make(map[int]map[string]any, len(items))
	for _, item := range items {
		byQID[item["question_id"].(int)] = item
	}
	if !byQID[1]["favorited"].(bool) || byQID[1]["favorite_id"].(int64) != fav.FavoriteID {
		t.Fatalf("题目 1 应回填已收藏状态, got %v", byQID[1])
	}
	if byQID[2]["favorited"].(bool) || byQID[2]["favorite_id"].(int64) != 0 {
		t.Fatalf("题目 2 应回填未收藏状态, got %v", byQID[2])
	}
}

// --- RemoveWrongQuestion ---

func TestRemoveWrongQuestion_Success(t *testing.T) {
	svc, db := newWrongQuestionSvc(t)
	testutil.SeedQuestion(t, db, "single_choice", "test", "A")
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
	testutil.SeedQuestion(t, db, "single_choice", "q1", "A")
	seedWrongQuestion(t, db, 1, 1, 3)
	seedWrongQuestion(t, db, 1, 2, 1)

	result := svc.GetStats(1)
	if result.Total != 2 {
		t.Fatalf("总数应为 2, got %v", result.Total)
	}
	// 仅存在的题目贡献类型（question 2 无对应题目，不计入 by_type）
	if result.ByType["single_choice"] != 1 {
		t.Fatalf("by_type 应统计 1 道有题目对应关系的单选错题, got %v", result.ByType)
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
	testutil.SeedQuestion(t, db, "single_choice", "导出错题", "A")
	seedWrongQuestion(t, db, 1, 1, 2)

	result := svc.ExportWrongQuestions(1)
	if len(result) != 1 {
		t.Fatalf("应导出 1 条, got %d", len(result))
	}
}

// --- FormatWrongQuestionsText (纯函数) ---

func TestFormatWrongQuestionsText_Empty(t *testing.T) {
	text := FormatWrongQuestionsText([]map[string]any{})
	if text == "" {
		t.Fatal("空列表应返回非空文本（标题）")
	}
}

func TestFormatWrongQuestionsText_WithData(t *testing.T) {
	data := []map[string]any{
		{
			"question_id":   1,
			"content":       "叉车检查要点",
			"type":          "single_choice",
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
	q := testutil.SeedQuestion(t, db, "single_choice", "重做题", "A")
	seedWrongQuestion(t, db, 1, q.ID, 2)

	result, err := svc.RedoWrongQuestion(1, q.ID, "A")
	if err != nil {
		t.Fatalf("重做失败: %v", err)
	}
	if result == nil {
		t.Fatal("结果不应为 nil")
	}
	if result.IsCorrect == nil || !*result.IsCorrect {
		t.Fatalf("答对应 is_correct=true, got %v", boolPtrVal(result.IsCorrect))
	}
	if result.CorrectAnswer != "A" || result.QuestionID != q.ID {
		t.Fatalf("typed 契约字段缺失: %+v", result)
	}
	// 重做结果落练习记录（PracticeType=redo）
	var cnt int64
	db.Model(&model.QuestionPracticeRecord{}).Where("student_id = ? AND question_id = ? AND practice_type = ? AND is_correct = ?", 1, q.ID, "redo", true).Count(&cnt)
	if cnt != 1 {
		t.Fatalf("答对重做应落一条正确的练习记录, got %d", cnt)
	}
	// 错题本状态机：is_redone 置位
	var wq model.WrongQuestion
	db.First(&wq, "student_id = ? AND question_id = ?", 1, q.ID)
	if !wq.IsRedone {
		t.Fatal("答对后应置 is_redone=true")
	}
	if wq.WrongCount != 2 {
		t.Fatalf("答对不得改计数, got %d", wq.WrongCount)
	}
}

func TestRedoWrongQuestion_Wrong(t *testing.T) {
	svc, db := newWrongQuestionSvc(t)
	q := testutil.SeedQuestion(t, db, "single_choice", "重做题", "A")
	seedWrongQuestion(t, db, 1, q.ID, 2)

	result, err := svc.RedoWrongQuestion(1, q.ID, "B")
	if err != nil {
		t.Fatalf("答错也不应报错: %v", err)
	}
	if result.IsCorrect == nil || *result.IsCorrect {
		t.Fatalf("答错应 is_correct=false, got %v", boolPtrVal(result.IsCorrect))
	}
	// 判错经 gradeOne 入库计数（wrong_count++），且 is_redone 复位
	var wq model.WrongQuestion
	db.First(&wq, "student_id = ? AND question_id = ?", 1, q.ID)
	if wq.WrongCount != 3 {
		t.Fatalf("答错应计数 +1, got %d", wq.WrongCount)
	}
	if wq.IsRedone {
		t.Fatal("答错应复位 is_redone=false")
	}
	var cnt int64
	db.Model(&model.QuestionPracticeRecord{}).Where("question_id = ? AND practice_type = ? AND is_correct = ?", q.ID, "redo", false).Count(&cnt)
	if cnt != 1 {
		t.Fatalf("答错重做应落一条错误练习记录, got %d", cnt)
	}
}

func TestRedoWrongQuestion_NotInWrongList(t *testing.T) {
	svc, db := newWrongQuestionSvc(t)
	q := testutil.SeedQuestion(t, db, "single_choice", "test", "A")
	// 不在错题本中
	_, err := svc.RedoWrongQuestion(1, q.ID, "A")
	if err == nil {
		t.Fatal("不在错题本中应返回错误")
	}
}

func TestRedoWrongQuestion_AIExplanationCached(t *testing.T) {
	svc, db := newWrongQuestionSvc(t)
	q := testutil.SeedQuestion(t, db, "single_choice", "缓存解析题", "A")
	db.Model(&model.Question{}).Where("id = ?", q.ID).Update("ai_explanation", "缓存解析")
	seedWrongQuestion(t, db, 1, q.ID, 1)

	result, err := svc.RedoWrongQuestion(1, q.ID, "A")
	if err != nil {
		t.Fatalf("重做失败: %v", err)
	}
	if result.AIExplanation != "缓存解析" {
		t.Fatalf("应返回缓存的 AI 解析, got %q", result.AIExplanation)
	}
}
