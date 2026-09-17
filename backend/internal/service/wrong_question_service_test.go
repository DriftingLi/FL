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
	result, err := svc.GetWrongQuestions(1, 1, 20, "", nil, false, "", nil)
	if err != nil {
		t.Fatalf("GetWrongQuestions 失败: %v", err)
	}
	if result.Total != 0 {
		t.Fatalf("空库总数应为 0, got %v", result.Total)
	}
}

func TestGetWrongQuestions_WithData(t *testing.T) {
	svc, db := newWrongQuestionSvc(t)
	testutil.SeedQuestion(t, db, "single_choice", "错题1", "A")
	seedWrongQuestion(t, db, 1, 1, 3)
	seedWrongQuestion(t, db, 1, 2, 1)

	result, err := svc.GetWrongQuestions(1, 1, 20, "", nil, false, "", nil)
	if err != nil {
		t.Fatalf("GetWrongQuestions 失败: %v", err)
	}
	if result.Total != 2 {
		t.Fatalf("总数应为 2, got %v", result.Total)
	}
}

func TestGetWrongQuestions_DefaultPaging(t *testing.T) {
	svc, _ := newWrongQuestionSvc(t)
	result, err := svc.GetWrongQuestions(1, 0, 0, "", nil, false, "", nil)
	if err != nil {
		t.Fatalf("GetWrongQuestions 失败: %v", err)
	}
	if result.Page != 1 {
		t.Fatalf("默认页码应为 1, got %v", result.Page)
	}
	if result.PageSize != 20 {
		t.Fatalf("默认页大小应为 20, got %v", result.PageSize)
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

	result, err := svc.GetWrongQuestions(1, 1, 20, "", nil, true, "", nil)
	if err != nil {
		t.Fatalf("GetWrongQuestions 失败: %v", err)
	}
	if result.Total != 1 {
		t.Fatalf("收藏过滤后总数应为 1, got %v", result.Total)
	}
	items := result.Items
	if len(items) != 1 || items[0].QuestionID != 1 {
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

	result, err := svc.GetWrongQuestions(1, 1, 20, "", nil, false, "time_asc", nil)
	if err != nil {
		t.Fatalf("GetWrongQuestions 失败: %v", err)
	}
	items := result.Items
	if len(items) != 2 || items[0].QuestionID != 1 {
		t.Fatalf("升序时首项应为较早错误的题目 1, got %v", items)
	}

	result, err = svc.GetWrongQuestions(1, 1, 20, "", nil, false, "", nil)
	if err != nil {
		t.Fatalf("GetWrongQuestions 失败: %v", err)
	}
	items = result.Items
	if len(items) != 2 || items[0].QuestionID != 2 {
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

	result, err := svc.GetWrongQuestions(1, 1, 20, "", nil, false, "", nil)
	if err != nil {
		t.Fatalf("GetWrongQuestions 失败: %v", err)
	}
	items := result.Items
	byQID := make(map[int]WrongQuestionDTO, len(items))
	for _, item := range items {
		byQID[item.QuestionID] = item
	}
	if !byQID[1].Favorited || byQID[1].FavoriteID != fav.FavoriteID {
		t.Fatalf("题目 1 应回填已收藏状态, got %v", byQID[1])
	}
	if byQID[2].Favorited || byQID[2].FavoriteID != 0 {
		t.Fatalf("题目 2 应回填未收藏状态, got %v", byQID[2])
	}
}

// TestGetWrongQuestions_LastUserAnswer（#1077）：错题卡片要展示「我上次选的答案」，
// 事实源是**最近一条** question_practice_record（不是最早、不是任意一条），且只认本人记录。
func TestGetWrongQuestions_LastUserAnswer(t *testing.T) {
	svc, db := newWrongQuestionSvc(t)
	q1 := testutil.SeedQuestion(t, db, "single_choice", "错题1", "A")
	q2 := testutil.SeedQuestion(t, db, "single_choice", "错题2", "B")
	q3 := testutil.SeedQuestion(t, db, "multi_choice", "从未作答过的错题", "C")
	seedWrongQuestion(t, db, 1, q1.ID, 3)
	seedWrongQuestion(t, db, 1, q2.ID, 2)
	seedWrongQuestion(t, db, 1, q3.ID, 1)

	now := testutil.Now()
	recs := []model.QuestionPracticeRecord{
		// q1 先答 B（早）、后答 D（晚）——应取 D
		{StudentID: 1, QuestionID: q1.ID, IsCorrect: false, PracticeType: "free", UserAnswer: "B", CreatedAt: now.Add(-2 * time.Hour)},
		{StudentID: 1, QuestionID: q1.ID, IsCorrect: false, PracticeType: "redo", UserAnswer: "D", CreatedAt: now},
		// q2 只有**别人**的记录——不得串入
		{StudentID: 2, QuestionID: q2.ID, IsCorrect: false, PracticeType: "free", UserAnswer: "别人选的", CreatedAt: now},
	}
	for i := range recs {
		if err := db.Create(&recs[i]).Error; err != nil {
			t.Fatalf("插入练习记录失败: %v", err)
		}
	}

	result, err := svc.GetWrongQuestions(1, 1, 20, "", nil, false, "", nil)
	if err != nil {
		t.Fatalf("GetWrongQuestions 失败: %v", err)
	}
	byQID := make(map[int]WrongQuestionDTO, len(result.Items))
	for _, item := range result.Items {
		byQID[item.QuestionID] = item
	}
	if got := byQID[q1.ID].LastUserAnswer; got != "D" {
		t.Fatalf("应取最近一条作答（D），got %q", got)
	}
	if got := byQID[q2.ID].LastUserAnswer; got != "" {
		t.Fatalf("别的学员的作答不得串入，got %q", got)
	}
	if got := byQID[q3.ID].LastUserAnswer; got != "" {
		t.Fatalf("从未作答过应为空串，got %q", got)
	}
}

// credentialID 过滤（#387）：错题按题目所属证件分区，与课程/题库同口径；
// qType 与 credentialID 同时传入时共用一次 JOIN question，不产生重复 JOIN。
func TestGetWrongQuestions_CredentialFilter(t *testing.T) {
	svc, db := newWrongQuestionSvc(t)
	credA := 11
	credB := 12
	qA := testutil.SeedQuestion(t, db, "single_choice", "证件A错题", "A")
	qA.CredentialID = &credA
	if err := db.Save(qA).Error; err != nil {
		t.Fatalf("更新题目证件失败: %v", err)
	}
	qB := testutil.SeedQuestion(t, db, "single_choice", "证件B错题", "B")
	qB.CredentialID = &credB
	if err := db.Save(qB).Error; err != nil {
		t.Fatalf("更新题目证件失败: %v", err)
	}
	seedWrongQuestion(t, db, 1, int(qA.ID), 1)
	seedWrongQuestion(t, db, 1, int(qB.ID), 1)

	result, err := svc.GetWrongQuestions(1, 1, 20, "", nil, false, "", &credA)
	if err != nil {
		t.Fatalf("GetWrongQuestions 失败: %v", err)
	}
	if result.Total != 1 {
		t.Fatalf("按证件A过滤后总数应为 1, got %v", result.Total)
	}
	items := result.Items
	if len(items) != 1 || items[0].QuestionID != int(qA.ID) {
		t.Fatalf("按证件A过滤后应仅剩题目 %d, got %v", qA.ID, items)
	}

	// nil = 不过滤（旧行为）
	result, err = svc.GetWrongQuestions(1, 1, 20, "", nil, false, "", nil)
	if err != nil {
		t.Fatalf("GetWrongQuestions 失败: %v", err)
	}
	if result.Total != 2 {
		t.Fatalf("不过滤时总数应为 2, got %v", result.Total)
	}

	// 证件过滤与题型过滤叠加（同一 JOIN 作用域）
	result, err = svc.GetWrongQuestions(1, 1, 20, "single_choice", nil, false, "", &credB)
	if err != nil {
		t.Fatalf("GetWrongQuestions 失败: %v", err)
	}
	if result.Total != 1 {
		t.Fatalf("证件B+单选过滤后总数应为 1, got %v", result.Total)
	}
	items = result.Items
	if len(items) != 1 || items[0].QuestionID != int(qB.ID) {
		t.Fatalf("证件B+单选过滤后应仅剩题目 %d, got %v", qB.ID, items)
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
	if !result.Removed {
		t.Fatalf("应返回 removed=true, got %v", result.Removed)
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

	result, err := svc.RedoWrongQuestion(1, q.ID, "A", nil)
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

	result, err := svc.RedoWrongQuestion(1, q.ID, "B", nil)
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
	_, err := svc.RedoWrongQuestion(1, q.ID, "A", nil)
	if err == nil {
		t.Fatal("不在错题本中应返回错误")
	}
}

// TestRedoWrongQuestion_CredentialScope 重做的证件口径（#1007 / ADR-0051）：
//  1. 传证件时取题按该证件校验 —— 别的证件的题不得重做（错题本列表本就按题目证件过滤）；
//  2. 重做记录落「重做那一刻的当前证件」（写入时冻结，与练习记录同口径）。
func TestRedoWrongQuestion_CredentialScope(t *testing.T) {
	svc, db := newWrongQuestionSvc(t)
	credA := model.Credential{Code: "redoScopeA", Name: "叉车司机N1", Category: "special_operation", Status: 1}
	credB := model.Credential{Code: "redoScopeB", Name: "低压电工", Category: "special_operation", Status: 1}
	for _, c := range []*model.Credential{&credA, &credB} {
		if err := db.Create(c).Error; err != nil {
			t.Fatalf("建证件失败: %v", err)
		}
	}
	qA := testutil.SeedQuestion(t, db, "single_choice", "A 证重做题", "A")
	if err := db.Model(qA).Update("credential_id", credA.ID).Error; err != nil {
		t.Fatalf("题目挂证件失败: %v", err)
	}
	seedWrongQuestion(t, db, 1, qA.ID, 1)

	// 用 B 证件重做 A 证件的题 → 拒绝（且不落记录）
	if _, err := svc.RedoWrongQuestion(1, qA.ID, "A", &credB.ID); err == nil {
		t.Fatal("跨证件重做应被拒绝")
	}
	var cnt int64
	db.Model(&model.QuestionPracticeRecord{}).Where("student_id = ? AND practice_type = ?", 1, "redo").Count(&cnt)
	if cnt != 0 {
		t.Fatalf("被拒绝的重做不得落记录, got %d 条", cnt)
	}

	// 用 A 证件重做 → 落记录且分区 = 作答那一刻的证件
	if _, err := svc.RedoWrongQuestion(1, qA.ID, "A", &credA.ID); err != nil {
		t.Fatalf("同证件重做失败: %v", err)
	}
	var rec model.QuestionPracticeRecord
	if err := db.Where("student_id = ? AND question_id = ? AND practice_type = ?", 1, qA.ID, "redo").First(&rec).Error; err != nil {
		t.Fatalf("查重做记录失败: %v", err)
	}
	if rec.CredentialID == nil || *rec.CredentialID != credA.ID {
		t.Fatalf("重做记录应落证件 %d（写入时冻结）, got %v", credA.ID, rec.CredentialID)
	}
}

func TestRedoWrongQuestion_AIExplanationCached(t *testing.T) {
	svc, db := newWrongQuestionSvc(t)
	q := testutil.SeedQuestion(t, db, "single_choice", "缓存解析题", "A")
	db.Model(&model.Question{}).Where("id = ?", q.ID).Update("ai_explanation", "缓存解析")
	seedWrongQuestion(t, db, 1, q.ID, 1)

	result, err := svc.RedoWrongQuestion(1, q.ID, "A", nil)
	if err != nil {
		t.Fatalf("重做失败: %v", err)
	}
	if result.AIExplanation != "缓存解析" {
		t.Fatalf("应返回缓存的 AI 解析, got %q", result.AIExplanation)
	}
}
