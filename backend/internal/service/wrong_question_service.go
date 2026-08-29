// Package service 错题本。
package service

import (
	"errors"
	"fmt"
	"strings"

	"go.uber.org/zap"
	"gorm.io/gorm"

	"forklift-training/internal/model"
	"forklift-training/pkg/paging"
)

// WrongQuestionService 错题本服务。
type WrongQuestionService struct {
	db *gorm.DB
	// grader 短答 AI 判分 adapter（nil 时简答重做降级，与练习流口径一致）。
	grader ShortAnswerGrader
	// explainer AI 解析 module（与练习提交共用同一 get-or-generate 入口，spec #295/#300）。
	explainer *QuestionExplanation

	logger *zap.Logger
}

// NewWrongQuestionService 创建错题本服务实例。ai 可为 nil（简答判分与解析降级）。
func NewWrongQuestionService(db *gorm.DB, ai *AIService, logger *zap.Logger) *WrongQuestionService {
	return &WrongQuestionService{
		db:        db,
		grader:    shortAnswerGraderOf(ai),
		explainer: NewQuestionExplanation(db, ai, logger),
		logger:    logger,
	}
}

// GetWrongQuestions 错题列表。
// sort: "time_asc" 按最近错误时间升序，其余按降序（默认）；
// favorited: 仅返回已收藏的错题（JOIN favorite，user_id 与 student_id 同源）。
func (s *WrongQuestionService) GetWrongQuestions(studentID, page, pageSize int, qType string, minWrongCount *int, favorited bool, sort string) map[string]any {
	orderBy := "wrong_question.last_wrong_at DESC"
	if sort == "time_asc" {
		orderBy = "wrong_question.last_wrong_at ASC"
	}
	items, total, page, pageSize := paging.Query[model.WrongQuestion](s.db, page, pageSize, 20, orderBy, func(q *gorm.DB) *gorm.DB {
		q = q.Where("student_id = ? AND is_removed = ?", studentID, false)
		if qType != "" {
			q = q.Joins("JOIN question ON question.id = wrong_question.question_id")
			q = q.Where("question.type = ?", qType)
		}
		if minWrongCount != nil {
			q = q.Where("wrong_question.wrong_count >= ?", *minWrongCount)
		}
		if favorited {
			q = q.Joins("JOIN favorite ON favorite.user_id = wrong_question.student_id AND favorite.target_type = ? AND favorite.target_id = wrong_question.question_id", FavoriteTargetQuestion)
		}
		return q
	})

	questionIDs := make([]int, 0, len(items))
	for i := range items {
		questionIDs = append(questionIDs, items[i].QuestionID)
	}
	questions := loadQuestionsByIDs(s.db, questionIDs)
	favoriteIDs := s.loadFavoriteIDs(studentID, questionIDs)

	result := make([]map[string]any, 0, len(items))
	for i := range items {
		wq := &items[i]
		item := wrongQuestionToDict(wq)
		item["favorited"] = favoriteIDs[wq.QuestionID] > 0
		item["favorite_id"] = favoriteIDs[wq.QuestionID]
		if q, ok := questions[wq.QuestionID]; ok {
			item["question"] = newQuestionDTO(q, true)
		}
		result = append(result, item)
	}
	return map[string]any{
		"total":     total,
		"page":      page,
		"page_size": pageSize,
		"items":     result,
	}
}

// loadFavoriteIDs 批量查询题目收藏 ID（question_id → favorite_id，未收藏为 0）。
func (s *WrongQuestionService) loadFavoriteIDs(studentID int, questionIDs []int) map[int]int64 {
	result := make(map[int]int64, len(questionIDs))
	if len(questionIDs) == 0 {
		return result
	}
	var rows []model.Favorite
	s.db.Where("user_id = ? AND target_type = ? AND target_id IN ?", studentID, FavoriteTargetQuestion, questionIDs).Find(&rows)
	for i := range rows {
		result[rows[i].TargetID] = rows[i].FavoriteID
	}
	return result
}

// RedoWrongQuestion 重做错题：与练习/模拟考试共享同一判分管线（gradeOne）与解析五模块装配。
// 单题即时重做形态（无会话生命周期）；结果落 question_practice_record，正确率/易错项统计含重做。
func (s *WrongQuestionService) RedoWrongQuestion(studentID, questionID int, userAnswer interface{}) (*SubmitResultDTO, error) {
	var wq model.WrongQuestion
	if err := s.db.Where("student_id = ? AND question_id = ? AND is_removed = ?", studentID, questionID, false).First(&wq).Error; err != nil {
		return nil, errors.New("错题记录不存在")
	}
	var question model.Question
	if err := s.db.First(&question, questionID).Error; err != nil {
		return nil, errors.New("题目不存在")
	}

	engine := newGradingEngine(s.db)
	flow := gradingFlow{ai: s.grader, maxScore: practiceMaxScore}
	gr := engine.gradeOne(flow, &question, userAnswer, studentID)

	// 重做结果与练习同口径落练习记录（统计事实源单一）。
	rec := model.QuestionPracticeRecord{
		StudentID:    studentID,
		QuestionID:   questionID,
		IsCorrect:    gr.IsCorrect != nil && *gr.IsCorrect,
		PracticeType: "redo",
		UserAnswer:   stringifyAnswer(userAnswer),
		CreatedAt:    beijingNow(),
	}
	if err := s.db.Create(&rec).Error; err != nil {
		return nil, err
	}

	// 错题本状态机：判错路径 gradeOne 已入库计数；此处仅按重做结果维护 is_redone 标记
	// （定向更新，避免覆盖 addToWrongQuestions 刚写入的计数）。
	if gr.IsCorrect != nil {
		if err := s.db.Model(&model.WrongQuestion{}).Where("id = ?", wq.ID).Update("is_redone", *gr.IsCorrect).Error; err != nil {
			return nil, err
		}
	}

	result := &SubmitResultDTO{
		IsCorrect:     gr.IsCorrect,
		CorrectAnswer: question.Answer,
		Explanation:   question.Explanation,
		QuestionID:    questionID,
		UserAnswer:    userAnswer,
	}
	finalizeSubmitResult(s.db, s.explainer, result, gr, &rec, &question)
	return result, nil
}

// RemoveWrongQuestion 移除错题。
func (s *WrongQuestionService) RemoveWrongQuestion(studentID, questionID int) (map[string]any, error) {
	var wq model.WrongQuestion
	if err := s.db.Where("student_id = ? AND question_id = ? AND is_removed = ?", studentID, questionID, false).First(&wq).Error; err != nil {
		return nil, errors.New("错题记录不存在")
	}
	wq.IsRemoved = true
	s.db.Save(&wq)
	return map[string]any{"removed": true}, nil
}

// GetStats 错题统计（经统计聚合 module，一次 GROUP BY）。
// 保留旧语义：仅统计未移除错题；by_type 只含实际存在题型的维度（不零填充）。
func (s *WrongQuestionService) GetStats(studentID int) *WrongQuestionStatsDTO {
	var total int64
	s.db.Model(&model.WrongQuestion{}).Where("student_id = ? AND is_removed = ?", studentID, false).Count(&total)
	byType := groupByCount(
		s.db.Model(&model.WrongQuestion{}).
			Joins("JOIN question ON question.id = wrong_question.question_id").
			Where("wrong_question.student_id = ? AND wrong_question.is_removed = ?", studentID, false),
		"question.type",
	)
	return &WrongQuestionStatsDTO{Total: total, ByType: byType}
}

// ExportWrongQuestions 导出错题。
func (s *WrongQuestionService) ExportWrongQuestions(studentID int) []map[string]any {
	var items []model.WrongQuestion
	s.db.Where("student_id = ? AND is_removed = ?", studentID, false).Find(&items)

	qIDs := make([]int, 0, len(items))
	for i := range items {
		qIDs = append(qIDs, items[i].QuestionID)
	}
	questions := loadQuestionsByIDs(s.db, qIDs)

	exportData := make([]map[string]any, 0, len(items))
	for i := range items {
		wq := &items[i]
		question, ok := questions[wq.QuestionID]
		if !ok {
			continue
		}
		var options interface{}
		if len(question.Options) > 0 {
			_ = jsonUnmarshal(question.Options, &options)
		}
		item := map[string]any{
			"question_id":    question.ID,
			"type":           question.Type,
			"content":        question.Content,
			"options":        options,
			"correct_answer": question.Answer,
			"explanation":    question.Explanation,
			"wrong_count":    wq.WrongCount,
			"image_url":      question.ImageURL,
			"last_wrong_at":  formatISO(wq.LastWrongAt),
		}
		exportData = append(exportData, item)
	}
	return exportData
}

// FormatWrongQuestionsText 格式化错题文本。
func FormatWrongQuestionsText(exportData []map[string]any) string {
	typeMap := map[string]any{
		"single_choice": "单选题",
		"multi_choice":  "多选题",
		"true_false":    "判断题",
		"fault_image":   "故障识图",
		"short_answer":  "简答题",
	}
	now := beijingNow().Format("2006-01-02 15:04:05")
	var sb strings.Builder
	sb.WriteString(strings.Repeat("=", 50))
	sb.WriteString("\n错题本导出\n")
	fmt.Fprintf(&sb, "导出时间: %s\n", now)
	fmt.Fprintf(&sb, "错题总数: %d\n", len(exportData))
	sb.WriteString(strings.Repeat("=", 50))

	for idx, item := range exportData {
		sb.WriteString("\n")
		fmt.Fprintf(&sb, "【第%d题】\n", idx+1)
		sb.WriteString(strings.Repeat("-", 40))
		sb.WriteString("\n")
		qType, _ := item["type"].(string)
		fmt.Fprintf(&sb, "题型: %s\n", mapOr(qType, typeMap, qType))
		content, _ := item["content"].(string)
		fmt.Fprintf(&sb, "题目: %s\n", content)

		if options, ok := item["options"].(map[string]any); ok && len(options) > 0 {
			sb.WriteString("选项:\n")
			keys := make([]string, 0, len(options))
			for k := range options {
				keys = append(keys, k)
			}
			sortStrings(keys)
			for _, k := range keys {
				fmt.Fprintf(&sb, "  %s. %v\n", k, options[k])
			}
		}

		correctAnswer, _ := item["correct_answer"].(string)
		fmt.Fprintf(&sb, "正确答案: %s\n", correctAnswer)
		if explanation, ok := item["explanation"].(string); ok && explanation != "" {
			fmt.Fprintf(&sb, "解析: %s\n", explanation)
		}
		wrongCount := toInt(item["wrong_count"])
		fmt.Fprintf(&sb, "错误次数: %d\n", wrongCount)
		if lastWrong, ok := item["last_wrong_at"].(string); ok && lastWrong != "" {
			fmt.Fprintf(&sb, "最近错误时间: %s\n", lastWrong)
		}
		if imgURL, ok := item["image_url"].(string); ok && imgURL != "" {
			fmt.Fprintf(&sb, "图片: %s\n", imgURL)
		}
		sb.WriteString(strings.Repeat("-", 40))
	}

	sb.WriteString("\n")
	fmt.Fprintf(&sb, "\n共 %d 道错题\n", len(exportData))
	fmt.Fprintf(&sb, "%s\n", strings.Repeat("=", 50))
	return sb.String()
}

// ===== dict 辅助 =====

func wrongQuestionToDict(wq *model.WrongQuestion) map[string]any {
	return map[string]any{
		"id":            wq.ID,
		"student_id":    wq.StudentID,
		"question_id":   wq.QuestionID,
		"wrong_count":   wq.WrongCount,
		"last_wrong_at": formatISO(wq.LastWrongAt),
		"is_removed":    wq.IsRemoved,
		"is_redone":     wq.IsRedone,
		"created_at":    formatISO(wq.CreatedAt),
	}
}

// BatchRemoveWrongQuestions 批量移出错题本
func (s *WrongQuestionService) BatchRemoveWrongQuestions(studentID int, questionIDs []int) (int, error) {
	if len(questionIDs) == 0 {
		return 0, errors.New("请选择要移除的题目")
	}
	res := s.db.Model(&model.WrongQuestion{}).Where("student_id = ? AND question_id IN ? AND is_removed = ?", studentID, questionIDs, false).Update("is_removed", true)
	return int(res.RowsAffected), res.Error
}

func mapOr(key string, m map[string]any, def any) any {
	if v, ok := m[key]; ok {
		return v
	}
	return def
}

func sortStrings(s []string) {
	for i := 1; i < len(s); i++ {
		for j := i; j > 0 && s[j-1] > s[j]; j-- {
			s[j-1], s[j] = s[j], s[j-1]
		}
	}
}
