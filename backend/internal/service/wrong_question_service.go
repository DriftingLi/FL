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

	logger *zap.Logger
}

// NewWrongQuestionService 创建错题本服务实例。
func NewWrongQuestionService(db *gorm.DB, logger *zap.Logger) *WrongQuestionService {
	return &WrongQuestionService{db: db, logger: logger}
}

// GetWrongQuestions 错题列表。
func (s *WrongQuestionService) GetWrongQuestions(studentID, page, pageSize int, qType string, minWrongCount *int) map[string]any {
	items, total, page, pageSize := paging.Query[model.WrongQuestion](s.db, page, pageSize, 20, "wrong_question.last_wrong_at DESC", func(q *gorm.DB) *gorm.DB {
		q = q.Where("student_id = ? AND is_removed = ?", studentID, false)
		if qType != "" {
			q = q.Joins("JOIN question ON question.id = wrong_question.question_id")
			q = q.Where("question.type = ?", qType)
		}
		if minWrongCount != nil {
			q = q.Where("wrong_question.wrong_count >= ?", *minWrongCount)
		}
		return q
	})

	questionIDs := make([]int, 0, len(items))
	for i := range items {
		questionIDs = append(questionIDs, items[i].QuestionID)
	}
	questions := loadQuestionsByIDs(s.db, questionIDs)

	result := make([]map[string]any, 0, len(items))
	for i := range items {
		wq := &items[i]
		item := wrongQuestionToDict(wq)
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

// RedoWrongQuestion 重做错题。
func (s *WrongQuestionService) RedoWrongQuestion(studentID, questionID int, userAnswer interface{}) (map[string]any, error) {
	var wq model.WrongQuestion
	if err := s.db.Where("student_id = ? AND question_id = ? AND is_removed = ?", studentID, questionID, false).First(&wq).Error; err != nil {
		return nil, errors.New("错题记录不存在")
	}
	var question model.Question
	if err := s.db.First(&question, questionID).Error; err != nil {
		return nil, errors.New("题目不存在")
	}

	isCorrect, _ := gradeQuestion(&question, userAnswer, 0)
	if isCorrect != nil && *isCorrect {
		wq.IsRedone = true
	} else if isCorrect != nil && !*isCorrect {
		wq.WrongCount++
		wq.LastWrongAt = beijingNow()
		wq.IsRedone = false
	}
	s.db.Save(&wq)

	result := map[string]any{
		"correct_answer": question.Answer,
		"explanation":    question.Explanation,
		"is_removed":     wq.IsRemoved,
	}
	if isCorrect == nil {
		result["is_correct"] = nil
	} else {
		result["is_correct"] = *isCorrect
	}
	// 解析增强：补充全站正确率与易错项（基于练习记录）
	if stats := s.questionStats(questionID, question.Type); stats != nil {
		if stats.accuracyRate != nil {
			result["accuracy_rate"] = *stats.accuracyRate
		}
		if stats.commonWrong != nil {
			result["common_wrong"] = *stats.commonWrong
		}
		result["total_attempts"] = stats.total
	}
	// AI 解析：有缓存用缓存，否则降级静态解析（错题流暂不触发 AI 生成）
	if question.AIExplanation != "" {
		result["ai_explanation"] = question.AIExplanation
	} else {
		result["ai_explanation"] = question.Explanation
	}
	return result, nil
}

type wqQuestionStat struct {
	total        int
	accuracyRate *float64
	commonWrong  *string
}

func (s *WrongQuestionService) questionStats(questionID int, qType string) *wqQuestionStat {
	var total int64
	s.db.Model(&model.QuestionPracticeRecord{}).Where("question_id = ?", questionID).Count(&total)
	res := &wqQuestionStat{total: int(total)}
	if total < 5 {
		return res
	}
	var correct int64
	s.db.Model(&model.QuestionPracticeRecord{}).Where("question_id = ? AND is_correct = ?", questionID, true).Count(&correct)
	acc := roundFloat1(float64(correct) / float64(total) * 100)
	res.accuracyRate = &acc
	if qType == "short_answer" {
		return res
	}
	type aggRow struct {
		UserAnswer string `gorm:"column:user_answer"`
		Cnt        int64  `gorm:"column:cnt"`
	}
	var row aggRow
	err := s.db.Model(&model.QuestionPracticeRecord{}).
		Select("user_answer, COUNT(*) as cnt").
		Where("question_id = ? AND is_correct = ?", questionID, false).
		Group("user_answer").
		Order("cnt DESC").
		Limit(1).
		Scan(&row).Error
	if err == nil && row.Cnt > 0 {
		res.commonWrong = &row.UserAnswer
	}
	return res
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
