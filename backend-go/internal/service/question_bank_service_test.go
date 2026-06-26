// Package service 题库服务 CRUD 测试，使用内存 sqlite 数据库。
package service

import (
	"testing"

	"gorm.io/gorm"

	"forklift-training/internal/testutil"
)

func newQuestionBankSvc(t *testing.T) (*QuestionBankService, *gorm.DB) {
	t.Helper()
	db := testutil.NewMemoryDB(t)
	return NewQuestionBankService(db), db
}

// --- CreateQuestion ---

func TestCreateQuestion_SingleChoice_Success(t *testing.T) {
	svc, _ := newQuestionBankSvc(t)
	createdBy := 1
	data := map[string]interface{}{
		"type":    "single_choice",
		"level":   "beginner",
		"content": "叉车作业前应检查什么？",
		"options": []string{"轮胎气压", "油位", "制动系统", "以上全部"},
		"answer":  "D",
	}
	result, err := svc.CreateQuestion(data, &createdBy, "tutor")
	if err != nil {
		t.Fatalf("创建题目失败: %v", err)
	}
	if result["type"] != "single_choice" || result["content"] != "叉车作业前应检查什么？" {
		t.Fatalf("创建结果不匹配: %+v", result)
	}
	if result["status"] != "pending" {
		t.Fatalf("默认状态应为 pending, got %v", result["status"])
	}
	if result["id"] == nil || result["id"].(int) == 0 {
		t.Fatal("题目 ID 不应为空")
	}
}

func TestCreateQuestion_InvalidType(t *testing.T) {
	svc, _ := newQuestionBankSvc(t)
	data := map[string]interface{}{
		"type":    "invalid_type",
		"level":   "beginner",
		"content": "test",
		"answer":  "A",
	}
	_, err := svc.CreateQuestion(data, nil, "tutor")
	if err == nil {
		t.Fatal("应拒绝无效题型")
	}
}

func TestCreateQuestion_InvalidLevel(t *testing.T) {
	svc, _ := newQuestionBankSvc(t)
	data := map[string]interface{}{
		"type":    "single_choice",
		"level":   "expert",
		"content": "test",
		"answer":  "A",
		"options": []string{"A", "B"},
	}
	_, err := svc.CreateQuestion(data, nil, "tutor")
	if err == nil {
		t.Fatal("应拒绝无效等级 expert")
	}
}

func TestCreateQuestion_EmptyContent(t *testing.T) {
	svc, _ := newQuestionBankSvc(t)
	data := map[string]interface{}{
		"type":   "single_choice",
		"level":  "beginner",
		"answer": "A",
		"options": []string{"A", "B"},
	}
	_, err := svc.CreateQuestion(data, nil, "tutor")
	if err == nil {
		t.Fatal("应拒绝空题干")
	}
}

func TestCreateQuestion_MissingOptions(t *testing.T) {
	svc, _ := newQuestionBankSvc(t)
	data := map[string]interface{}{
		"type":    "single_choice",
		"level":   "beginner",
		"content": "test",
		"answer":  "A",
	}
	_, err := svc.CreateQuestion(data, nil, "tutor")
	if err == nil {
		t.Fatal("单选题应要求选项")
	}
}

func TestCreateQuestion_ShortAnswer_NoAnswer(t *testing.T) {
	svc, _ := newQuestionBankSvc(t)
	data := map[string]interface{}{
		"type":    "short_answer",
		"level":   "advanced",
		"content": "请描述液压系统工作原理",
	}
	result, err := svc.CreateQuestion(data, nil, "tutor")
	if err != nil {
		t.Fatalf("简答题可不提供答案: %v", err)
	}
	if result["type"] != "short_answer" {
		t.Fatalf("题型应为 short_answer, got %v", result["type"])
	}
}

func TestCreateQuestion_WithKnowledgePoint(t *testing.T) {
	svc, db := newQuestionBankSvc(t)
	// 先创建知识点
	kp := testutil.SeedKnowledgePoint(t, db, "液压系统", "beginner")
	data := map[string]interface{}{
		"type":               "single_choice",
		"level":              "beginner",
		"content":            "test",
		"options":            []string{"A", "B"},
		"answer":             "A",
		"knowledge_point_id": kp.ID,
	}
	result, err := svc.CreateQuestion(data, nil, "tutor")
	if err != nil {
		t.Fatalf("带知识点创建失败: %v", err)
	}
	kpID, ok := result["knowledge_point_id"].(*int)
	if !ok || kpID == nil || *kpID != kp.ID {
		t.Fatalf("知识点 ID 不匹配: got %v, want %d", result["knowledge_point_id"], kp.ID)
	}
}

// --- GetQuestion ---

func TestGetQuestion_Success(t *testing.T) {
	svc, db := newQuestionBankSvc(t)
	q := testutil.SeedQuestion(t, db, "true_false", "beginner", "叉车可以超载运行", "false")
	result, err := svc.GetQuestion(q.ID)
	if err != nil {
		t.Fatalf("查询题目失败: %v", err)
	}
	if result["content"] != "叉车可以超载运行" {
		t.Fatalf("内容不匹配: %v", result["content"])
	}
}

func TestGetQuestion_NotFound(t *testing.T) {
	svc, _ := newQuestionBankSvc(t)
	_, err := svc.GetQuestion(9999)
	if err == nil {
		t.Fatal("应返回题目不存在")
	}
}

// --- UpdateQuestion ---

func TestUpdateQuestion_Success(t *testing.T) {
	svc, db := newQuestionBankSvc(t)
	q := testutil.SeedQuestion(t, db, "single_choice", "beginner", "旧题干", "A")
	data := map[string]interface{}{
		"content": "新题干",
		"level":   "intermediate",
	}
	result, err := svc.UpdateQuestion(q.ID, data)
	if err != nil {
		t.Fatalf("更新失败: %v", err)
	}
	if result["content"] != "新题干" {
		t.Fatalf("内容未更新: %v", result["content"])
	}
	if result["level"] != "intermediate" {
		t.Fatalf("等级未更新: %v", result["level"])
	}
}

func TestUpdateQuestion_InvalidStatus(t *testing.T) {
	svc, db := newQuestionBankSvc(t)
	q := testutil.SeedQuestion(t, db, "single_choice", "beginner", "test", "A")
	data := map[string]interface{}{"status": "invalid_status"}
	_, err := svc.UpdateQuestion(q.ID, data)
	if err == nil {
		t.Fatal("应拒绝无效状态")
	}
}

func TestUpdateQuestion_NotFound(t *testing.T) {
	svc, _ := newQuestionBankSvc(t)
	_, err := svc.UpdateQuestion(9999, map[string]interface{}{"content": "x"})
	if err == nil {
		t.Fatal("应返回题目不存在")
	}
}

// --- DeleteQuestion ---

func TestDeleteQuestion_Success(t *testing.T) {
	svc, db := newQuestionBankSvc(t)
	q := testutil.SeedQuestion(t, db, "single_choice", "beginner", "test", "A")
	if err := svc.DeleteQuestion(q.ID); err != nil {
		t.Fatalf("删除失败: %v", err)
	}
	_, err := svc.GetQuestion(q.ID)
	if err == nil {
		t.Fatal("删除后应查询不到")
	}
}

func TestDeleteQuestion_NotFound(t *testing.T) {
	svc, _ := newQuestionBankSvc(t)
	err := svc.DeleteQuestion(9999)
	if err == nil {
		t.Fatal("应返回题目不存在")
	}
}

// --- ListQuestions ---

func TestListQuestions_Pagination(t *testing.T) {
	svc, db := newQuestionBankSvc(t)
	for i := 0; i < 5; i++ {
		testutil.SeedQuestion(t, db, "single_choice", "beginner", "题目前5", "A")
	}
	result := svc.ListQuestions(1, 2, "", "", nil, "", "")
	if result["total"].(int64) != 5 {
		t.Fatalf("总数应为 5, got %v", result["total"])
	}
	questions := result["questions"].([]map[string]interface{})
	if len(questions) != 2 {
		t.Fatalf("本页应 2 条, got %d", len(questions))
	}
	if result["page"].(int) != 1 || result["page_size"].(int) != 2 {
		t.Fatalf("分页参数不匹配: %+v", result)
	}
}

func TestListQuestions_FilterByLevel(t *testing.T) {
	svc, db := newQuestionBankSvc(t)
	testutil.SeedQuestion(t, db, "single_choice", "beginner", "初级题", "A")
	testutil.SeedQuestion(t, db, "single_choice", "advanced", "高级题", "A")
	result := svc.ListQuestions(1, 20, "advanced", "", nil, "", "")
	if result["total"].(int64) != 1 {
		t.Fatalf("高级题应 1 条, got %v", result["total"])
	}
}

func TestListQuestions_FilterByType(t *testing.T) {
	svc, db := newQuestionBankSvc(t)
	testutil.SeedQuestion(t, db, "single_choice", "beginner", "单选题", "A")
	testutil.SeedQuestion(t, db, "true_false", "beginner", "判断题", "true")
	result := svc.ListQuestions(1, 20, "", "true_false", nil, "", "")
	if result["total"].(int64) != 1 {
		t.Fatalf("判断题应 1 条, got %v", result["total"])
	}
}

func TestListQuestions_DefaultPage(t *testing.T) {
	svc, _ := newQuestionBankSvc(t)
	result := svc.ListQuestions(0, 0, "", "", nil, "", "")
	if result["page"].(int) != 1 {
		t.Fatalf("默认页码应为 1, got %v", result["page"])
	}
	if result["page_size"].(int) != 20 {
		t.Fatalf("默认页大小应为 20, got %v", result["page_size"])
	}
}

// --- PublishQuestion ---

func TestPublishQuestion_Success(t *testing.T) {
	svc, db := newQuestionBankSvc(t)
	q := testutil.SeedQuestion(t, db, "single_choice", "beginner", "test", "A")
	result, err := svc.PublishQuestion(q.ID)
	if err != nil {
		t.Fatalf("发布失败: %v", err)
	}
	if result["status"] != "published" {
		t.Fatalf("状态应为 published, got %v", result["status"])
	}
}

func TestPublishQuestion_NotFound(t *testing.T) {
	svc, _ := newQuestionBankSvc(t)
	_, err := svc.PublishQuestion(9999)
	if err == nil {
		t.Fatal("应返回题目不存在")
	}
}

// --- BatchPublish ---

func TestBatchPublish_Success(t *testing.T) {
	svc, db := newQuestionBankSvc(t)
	q1 := testutil.SeedQuestion(t, db, "single_choice", "beginner", "q1", "A")
	q2 := testutil.SeedQuestion(t, db, "single_choice", "beginner", "q2", "A")
	result := svc.BatchPublish([]int{q1.ID, q2.ID})
	if result["published_count"].(int) != 2 {
		t.Fatalf("应发布 2 条, got %v", result["published_count"])
	}
}

func TestBatchPublish_PartialNotFound(t *testing.T) {
	svc, db := newQuestionBankSvc(t)
	q1 := testutil.SeedQuestion(t, db, "single_choice", "beginner", "q1", "A")
	result := svc.BatchPublish([]int{q1.ID, 9999})
	if result["published_count"].(int) != 1 {
		t.Fatalf("应发布 1 条, got %v", result["published_count"])
	}
}

func TestBatchPublish_Empty(t *testing.T) {
	svc, _ := newQuestionBankSvc(t)
	result := svc.BatchPublish([]int{})
	if result["published_count"].(int) != 0 {
		t.Fatalf("空列表应 0 条, got %v", result["published_count"])
	}
}

// --- BatchImport ---

func TestBatchImport_Success(t *testing.T) {
	svc, _ := newQuestionBankSvc(t)
	items := []interface{}{
		map[string]interface{}{
			"type":    "single_choice",
			"level":   "beginner",
			"content": "导入题1",
			"options": []string{"A", "B"},
			"answer":  "A",
		},
		map[string]interface{}{
			"type":    "true_false",
			"level":   "intermediate",
			"content": "导入题2",
			"answer":  "true",
		},
	}
	createdBy := 1
	result := svc.BatchImport(items, &createdBy)
	if result["success_count"].(int) != 2 {
		t.Fatalf("应成功 2 条, got %v", result["success_count"])
	}
	if result["error_count"].(int) != 0 {
		t.Fatalf("应无错误, got %v", result["error_count"])
	}
}

func TestBatchImport_WithErrors(t *testing.T) {
	svc, _ := newQuestionBankSvc(t)
	items := []interface{}{
		map[string]interface{}{
			"type":    "single_choice",
			"level":   "beginner",
			"content": "有效题",
			"options": []string{"A", "B"},
			"answer":  "A",
		},
		map[string]interface{}{
			"type":  "invalid_type",
			"level": "beginner",
			"content": "无效题",
		},
		"not-a-map", // 无效数据
	}
	result := svc.BatchImport(items, nil)
	if result["success_count"].(int) != 1 {
		t.Fatalf("应成功 1 条, got %v", result["success_count"])
	}
	if result["error_count"].(int) != 2 {
		t.Fatalf("应错误 2 条, got %v", result["error_count"])
	}
}

// --- GetStats ---

func TestGetStats_Empty(t *testing.T) {
	svc, _ := newQuestionBankSvc(t)
	result := svc.GetStats()
	if result["total"].(int64) != 0 {
		t.Fatalf("空库总数应为 0, got %v", result["total"])
	}
}

func TestGetStats_WithData(t *testing.T) {
	svc, db := newQuestionBankSvc(t)
	testutil.SeedQuestion(t, db, "single_choice", "beginner", "q1", "A")
	testutil.SeedQuestion(t, db, "single_choice", "intermediate", "q2", "A")
	testutil.SeedQuestion(t, db, "true_false", "advanced", "q3", "true")
	result := svc.GetStats()
	if result["total"].(int64) != 3 {
		t.Fatalf("总数应为 3, got %v", result["total"])
	}
}
