// Package service 错题本，对应 Python wrong_question_service。
package service

import (
	"errors"
	"fmt"
	"strings"

	"gorm.io/gorm"

	"forklift-training/internal/model"
)

// WrongQuestionService 错题本服务。
type WrongQuestionService struct {
	db *gorm.DB
}

// NewWrongQuestionService 创建错题本服务实例。
func NewWrongQuestionService(db *gorm.DB) *WrongQuestionService {
	return &WrongQuestionService{db: db}
}

// GetWrongQuestions 错题列表，对应 Python get_wrong_questions。
func (s *WrongQuestionService) GetWrongQuestions(studentID, page, pageSize int, qType string, knowledgePointID *int, minWrongCount *int) map[string]interface{} {
	if page <= 0 {
		page = 1
	}
	if pageSize <= 0 {
		pageSize = 20
	}
	q := s.db.Model(&model.WrongQuestion{}).Where("student_id = ? AND is_removed = ?", studentID, false)
	if qType != "" || knowledgePointID != nil {
		q = q.Joins("JOIN question ON question.id = wrong_question.question_id")
		if qType != "" {
			q = q.Where("question.type = ?", qType)
		}
		if knowledgePointID != nil {
			q = q.Where("question.knowledge_point_id = ?", *knowledgePointID)
		}
	}
	if minWrongCount != nil {
		q = q.Where("wrong_question.wrong_count >= ?", *minWrongCount)
	}
	var total int64
	q.Count(&total)
	var items []model.WrongQuestion
	q.Order("wrong_question.last_wrong_at DESC").Offset((page - 1) * pageSize).Limit(pageSize).Find(&items)

	result := make([]map[string]interface{}, 0, len(items))
	for i := range items {
		wq := &items[i]
		item := wrongQuestionToDict(wq)
		var question model.Question
		if err := s.db.First(&question, wq.QuestionID).Error; err == nil {
			item["question"] = questionToDict(&question, true)
		}
		result = append(result, item)
	}
	return map[string]interface{}{
		"total":     total,
		"page":      page,
		"page_size": pageSize,
		"items":     result,
	}
}

// RedoWrongQuestion 重做错题，对应 Python redo_wrong_question。
func (s *WrongQuestionService) RedoWrongQuestion(studentID, questionID int, userAnswer interface{}) (map[string]interface{}, error) {
	var wq model.WrongQuestion
	if err := s.db.Where("student_id = ? AND question_id = ? AND is_removed = ?", studentID, questionID, false).First(&wq).Error; err != nil {
		return nil, errors.New("错题记录不存在")
	}
	var question model.Question
	if err := s.db.First(&question, questionID).Error; err != nil {
		return nil, errors.New("题目不存在")
	}

	isCorrect := checkAnswer(&question, userAnswer)
	if isCorrect != nil && *isCorrect {
		wq.IsRemoved = true
	} else if isCorrect != nil && !*isCorrect {
		wq.WrongCount++
		wq.LastWrongAt = beijingNow()
	}
	s.db.Save(&wq)

	result := map[string]interface{}{
		"correct_answer": question.Answer,
		"explanation":    question.Explanation,
		"is_removed":     wq.IsRemoved,
	}
	if isCorrect == nil {
		result["is_correct"] = nil
	} else {
		result["is_correct"] = *isCorrect
	}
	return result, nil
}

// RemoveWrongQuestion 移除错题，对应 Python remove_wrong_question。
func (s *WrongQuestionService) RemoveWrongQuestion(studentID, questionID int) (map[string]interface{}, error) {
	var wq model.WrongQuestion
	if err := s.db.Where("student_id = ? AND question_id = ? AND is_removed = ?", studentID, questionID, false).First(&wq).Error; err != nil {
		return nil, errors.New("错题记录不存在")
	}
	wq.IsRemoved = true
	s.db.Save(&wq)
	return map[string]interface{}{"removed": true}, nil
}

// GetStats 错题统计，对应 Python get_wrong_question_stats。
func (s *WrongQuestionService) GetStats(studentID int) map[string]interface{} {
	var items []model.WrongQuestion
	s.db.Where("student_id = ? AND is_removed = ?", studentID, false).Find(&items)

	byType := map[string]int{}
	byKnowledgePoint := map[string]int{}
	total := len(items)
	for i := range items {
		wq := &items[i]
		var question model.Question
		if err := s.db.First(&question, wq.QuestionID).Error; err != nil {
			continue
		}
		byType[question.Type]++
		if question.KnowledgePointID != nil {
			var kp model.KnowledgePoint
			kpName := "未分类"
			if err := s.db.First(&kp, *question.KnowledgePointID).Error; err == nil {
				kpName = kp.Name
			}
			byKnowledgePoint[kpName]++
		}
	}
	return map[string]interface{}{
		"total":              total,
		"by_type":            byType,
		"by_knowledge_point": byKnowledgePoint,
	}
}

// ExportWrongQuestions 导出错题，对应 Python export_wrong_questions。
func (s *WrongQuestionService) ExportWrongQuestions(studentID int) []map[string]interface{} {
	var items []model.WrongQuestion
	s.db.Where("student_id = ? AND is_removed = ?", studentID, false).Find(&items)

	exportData := make([]map[string]interface{}, 0, len(items))
	for i := range items {
		wq := &items[i]
		var question model.Question
		if err := s.db.First(&question, wq.QuestionID).Error; err != nil {
			continue
		}
		var options interface{}
		if len(question.Options) > 0 {
			_ = jsonUnmarshal(question.Options, &options)
		}
		item := map[string]interface{}{
			"question_id":        question.ID,
			"type":               question.Type,
			"content":            question.Content,
			"options":            options,
			"correct_answer":     question.Answer,
			"explanation":        question.Explanation,
			"wrong_count":        wq.WrongCount,
			"image_url":          question.ImageURL,
			"knowledge_point_id": question.KnowledgePointID,
			"level":              question.Level,
			"last_wrong_at":      formatISO(wq.LastWrongAt),
		}
		exportData = append(exportData, item)
	}
	return exportData
}

// FormatWrongQuestionsText 格式化错题文本，对应 Python format_wrong_questions_text。
func FormatWrongQuestionsText(exportData []map[string]interface{}) string {
	typeMap := map[string]string{
		"single_choice": "单选题",
		"multi_choice":  "多选题",
		"true_false":    "判断题",
		"fault_image":   "故障识图",
		"short_answer":  "简答题",
	}
	levelMap := map[string]string{
		"beginner":     "初级",
		"intermediate": "中级",
		"advanced":     "高级",
	}
	now := beijingNow().Format("2006-01-02 15:04:05")
	var sb strings.Builder
	sb.WriteString(strings.Repeat("=", 50))
	sb.WriteString("\n错题本导出\n")
	sb.WriteString(fmt.Sprintf("导出时间: %s\n", now))
	sb.WriteString(fmt.Sprintf("错题总数: %d\n", len(exportData)))
	sb.WriteString(strings.Repeat("=", 50))

	for idx, item := range exportData {
		sb.WriteString("\n")
		sb.WriteString(fmt.Sprintf("【第%d题】\n", idx+1))
		sb.WriteString(strings.Repeat("-", 40))
		sb.WriteString("\n")
		qType, _ := item["type"].(string)
		sb.WriteString(fmt.Sprintf("题型: %s\n", mapOr(qType, typeMap, qType)))
		level, _ := item["level"].(string)
		sb.WriteString(fmt.Sprintf("难度: %s\n", mapOr(level, levelMap, level)))
		content, _ := item["content"].(string)
		sb.WriteString(fmt.Sprintf("题目: %s\n", content))

		if options, ok := item["options"].(map[string]interface{}); ok && len(options) > 0 {
			sb.WriteString("选项:\n")
			keys := make([]string, 0, len(options))
			for k := range options {
				keys = append(keys, k)
			}
			sortStrings(keys)
			for _, k := range keys {
				sb.WriteString(fmt.Sprintf("  %s. %v\n", k, options[k]))
			}
		}

		correctAnswer, _ := item["correct_answer"].(string)
		sb.WriteString(fmt.Sprintf("正确答案: %s\n", correctAnswer))
		if explanation, ok := item["explanation"].(string); ok && explanation != "" {
			sb.WriteString(fmt.Sprintf("解析: %s\n", explanation))
		}
		wrongCount := toInt(item["wrong_count"])
		sb.WriteString(fmt.Sprintf("错误次数: %d\n", wrongCount))
		if lastWrong, ok := item["last_wrong_at"].(string); ok && lastWrong != "" {
			sb.WriteString(fmt.Sprintf("最近错误时间: %s\n", lastWrong))
		}
		if imgURL, ok := item["image_url"].(string); ok && imgURL != "" {
			sb.WriteString(fmt.Sprintf("图片: %s\n", imgURL))
		}
		sb.WriteString(strings.Repeat("-", 40))
	}

	sb.WriteString("\n")
	sb.WriteString(strings.Repeat("=", 50))
	sb.WriteString(fmt.Sprintf("\n共 %d 道错题\n", len(exportData)))
	sb.WriteString(strings.Repeat("=", 50))
	return sb.String()
}

// ===== dict 辅助 =====

func wrongQuestionToDict(wq *model.WrongQuestion) map[string]interface{} {
	return map[string]interface{}{
		"id":            wq.ID,
		"student_id":    wq.StudentID,
		"question_id":   wq.QuestionID,
		"wrong_count":   wq.WrongCount,
		"last_wrong_at": formatISO(wq.LastWrongAt),
		"is_removed":    wq.IsRemoved,
		"created_at":    formatISO(wq.CreatedAt),
	}
}

func mapOr(key string, m map[string]string, def string) string {
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
