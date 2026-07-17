// Package service 模拟考试。
package service

import (
	"errors"

	"gorm.io/gorm"

	"forklift-training/internal/model"
)

// mockExamDefaultCount 模拟考试默认题量（取消等级后：固定题量随机抽）。
const mockExamDefaultCount = 40

// MockExamService 模拟考试服务。
type MockExamService struct {
	db *gorm.DB
	ai *AIService
}

// NewMockExamService 创建模拟考试服务实例。
func NewMockExamService(db *gorm.DB, ai *AIService) *MockExamService {
	return &MockExamService{db: db, ai: ai}
}

// Start 生成模拟考试：从 published 题库随机抽 count 题（不分等级、不分题型）。
func (s *MockExamService) Start(studentID, count, duration int) (map[string]any, error) {
	if count <= 0 {
		count = mockExamDefaultCount
	}
	if duration <= 0 {
		duration = 90
	}

	selected, err := sampleQuestions(s.db, "", nil, count)
	if err != nil {
		return nil, errors.New("查询题目失败")
	}
	if len(selected) == 0 {
		return nil, errors.New("题库暂无可用的题目")
	}

	questionIDs := make([]int, len(selected))
	totalScore := 0
	for i, q := range selected {
		questionIDs[i] = q.ID
		score := float64(q.Score)
		if q.Score <= 0 {
			score = mockExamScoreMap[q.Type]
		}
		totalScore += int(score)
	}

	idsJSON, _ := jsonMarshal(questionIDs)
	emptyJSON, _ := jsonMarshal(map[string]any{})
	startTime := beijingNow()
	mock := model.MockExam{
		StudentID:     studentID,
		QuestionIDs:   model.JSONB(idsJSON),
		Answers:       model.JSONB(emptyJSON),
		Duration:      duration,
		Status:        "in_progress",
		StartTime:     &startTime,
		RemainingTime: duration * 60,
	}
	if err := s.db.Create(&mock).Error; err != nil {
		return nil, err
	}

	ordered := make([]map[string]any, 0, len(selected))
	for i := range selected {
		ordered = append(ordered, questionToDict(&selected[i], false))
	}
	return map[string]any{
		"mock_exam_id":    mock.ID,
		"duration":        duration,
		"total_score":     totalScore,
		"total_questions": len(questionIDs),
		"remaining_time":  mock.RemainingTime,
		"questions":       ordered,
	}, nil
}

// SaveProgress 保存进度。
func (s *MockExamService) SaveProgress(mockExamID, studentID int, answers map[string]any, remainingTime int) error {
	var mock model.MockExam
	if err := s.db.First(&mock, mockExamID).Error; err != nil {
		return errors.New("模拟考试不存在")
	}
	if mock.StudentID != studentID {
		return errors.New("无权操作此考试")
	}
	ansJSON, _ := jsonMarshal(answers)
	mock.Answers = model.JSONB(ansJSON)
	mock.RemainingTime = remainingTime
	return s.db.Save(&mock).Error
}

// Resume 恢复考试。
func (s *MockExamService) Resume(mockExamID, studentID int) (map[string]any, error) {
	var mock model.MockExam
	if err := s.db.First(&mock, mockExamID).Error; err != nil {
		return nil, errors.New("模拟考试不存在")
	}
	if mock.StudentID != studentID {
		return nil, errors.New("无权操作此考试")
	}
	if mock.Status != "in_progress" {
		return nil, errors.New("考试不在进行中")
	}

	var ids []int
	if len(mock.QuestionIDs) > 0 {
		_ = jsonUnmarshal(mock.QuestionIDs, &ids)
	}
	var questions []model.Question
	s.db.Where("id IN ?", ids).Find(&questions)
	qMap := map[int]*model.Question{}
	for i := range questions {
		qMap[questions[i].ID] = &questions[i]
	}
	ordered := make([]map[string]any, 0, len(ids))
	for _, qid := range ids {
		if q, ok := qMap[qid]; ok {
			ordered = append(ordered, questionToDict(q, false))
		}
	}
	var answers interface{}
	if len(mock.Answers) > 0 {
		_ = jsonUnmarshal(mock.Answers, &answers)
	}
	startISO := ""
	if mock.StartTime != nil {
		startISO = formatISO(*mock.StartTime)
	}
	return map[string]any{
		"mock_exam_id":   mock.ID,
		"duration":       mock.Duration,
		"remaining_time": mock.RemainingTime,
		"questions":      ordered,
		"answers":        answers,
		"start_time":     startISO,
	}, nil
}

// Submit 交卷。
func (s *MockExamService) Submit(mockExamID, studentID int) (map[string]any, error) {
	var mock model.MockExam
	if err := s.db.First(&mock, mockExamID).Error; err != nil {
		return nil, errors.New("模拟考试不存在")
	}
	if mock.StudentID != studentID {
		return nil, errors.New("无权操作此考试")
	}
	if mock.Status != "in_progress" {
		return nil, errors.New("考试不在进行中")
	}

	var answersMap map[string]any
	if len(mock.Answers) > 0 {
		_ = jsonUnmarshal(mock.Answers, &answersMap)
	}
	if answersMap == nil {
		answersMap = map[string]any{}
	}
	var ids []int
	if len(mock.QuestionIDs) > 0 {
		_ = jsonUnmarshal(mock.QuestionIDs, &ids)
	}
	var questions []model.Question
	s.db.Where("id IN ?", ids).Find(&questions)
	qMap := map[int]*model.Question{}
	for i := range questions {
		qMap[questions[i].ID] = &questions[i]
	}

	totalScore := 0.0
	maxScore := 0.0
	correctCount := 0
	details := make([]map[string]any, 0, len(ids))

	for _, qid := range ids {
		question, ok := qMap[qid]
		if !ok {
			continue
		}
		userAnswer := answersMap[intToString(qid)]
		isCorrect, earned := gradeQuestion(question, userAnswer, mockExamMaxScore(question))
		qMax := mockExamMaxScore(question)
		maxScore += qMax

		if isCorrect != nil && *isCorrect {
			correctCount++
			totalScore += earned
		} else if isCorrect != nil && !*isCorrect {
			_ = addToWrongQuestions(s.db, studentID, qid)
		}

		detail := map[string]any{
			"question_id":    qid,
			"type":           question.Type,
			"content":        question.Content,
			"user_answer":    userAnswer,
			"correct_answer": question.Answer,
			"score":          earned,
			"max_score":      qMax,
			"explanation":    question.Explanation,
		}
		var options interface{}
		if len(question.Options) > 0 {
			_ = jsonUnmarshal(question.Options, &options)
		}
		detail["options"] = options
		if isCorrect == nil {
			detail["is_correct"] = nil
		} else {
			detail["is_correct"] = *isCorrect
		}

		if question.Type == "short_answer" && s.ai != nil {
			aiRes := s.ai.GradeShortAnswer(question.Content, question.ReferenceAnswer, question.ScoringCriteria, stringifyAnswer(userAnswer), qMax, nil)
			if aiRes != nil {
				detail["ai_score"] = aiRes.Score
				detail["ai_comment"] = aiRes.Comment
				if aiRes.Fallback {
					detail["ai_fallback"] = true
				}
			}
		}
		details = append(details, detail)
	}

	mock.Status = "submitted"
	submitTime := beijingNow()
	mock.SubmitTime = &submitTime
	mock.Score = floatPtr(totalScore)
	accuracy := 0.0
	if len(ids) > 0 {
		accuracy = roundFloat1(float64(correctCount) / float64(len(ids)) * 100)
	}
	result := map[string]any{
		"total_score":     totalScore,
		"max_score":       maxScore,
		"correct_count":   correctCount,
		"total_questions": len(ids),
		"accuracy":        accuracy,
		"details":         details,
	}
	resultJSON, _ := jsonMarshal(result)
	mock.Result = model.JSONB(resultJSON)
	if err := s.db.Save(&mock).Error; err != nil {
		return nil, err
	}
	return result, nil
}

// GetResult 获取结果。
func (s *MockExamService) GetResult(mockExamID, studentID int) (map[string]any, error) {
	var mock model.MockExam
	if err := s.db.First(&mock, mockExamID).Error; err != nil {
		return nil, errors.New("模拟考试不存在")
	}
	if mock.StudentID != studentID {
		return nil, errors.New("无权查看此考试")
	}
	var result interface{}
	if len(mock.Result) > 0 {
		_ = jsonUnmarshal(mock.Result, &result)
	}
	resultMap := map[string]any{}
	if m, ok := result.(map[string]any); ok {
		resultMap = m
	}
	resultMap["mock_exam_id"] = mock.ID
	submitISO := ""
	if mock.SubmitTime != nil {
		submitISO = formatISO(*mock.SubmitTime)
	}
	resultMap["submit_time"] = submitISO
	return resultMap, nil
}

// GetHistory 历史列表。
func (s *MockExamService) GetHistory(studentID, page, pageSize int) map[string]any {
	if page <= 0 {
		page = 1
	}
	if pageSize <= 0 {
		pageSize = 10
	}
	q := s.db.Model(&model.MockExam{}).Where("student_id = ?", studentID)
	var total int64
	q.Count(&total)
	var exams []model.MockExam
	q.Order("created_at DESC").Offset((page - 1) * pageSize).Limit(pageSize).Find(&exams)
	items := make([]map[string]any, 0, len(exams))
	for i := range exams {
		items = append(items, mockExamToDict(&exams[i]))
	}
	return map[string]any{
		"total":     total,
		"page":      page,
		"page_size": pageSize,
		"exams":     items,
	}
}

// ===== 辅助 =====

func mockExamMaxScore(q *model.Question) float64 {
	if q.Score > 0 {
		return float64(q.Score)
	}
	if v, ok := mockExamScoreMap[q.Type]; ok {
		return v
	}
	return 0
}

func mockExamToDict(m *model.MockExam) map[string]any {
	var ids, answers, result any
	if len(m.QuestionIDs) > 0 {
		_ = jsonUnmarshal(m.QuestionIDs, &ids)
	}
	if len(m.Answers) > 0 {
		_ = jsonUnmarshal(m.Answers, &answers)
	}
	if len(m.Result) > 0 {
		_ = jsonUnmarshal(m.Result, &result)
	}
	startISO, submitISO := "", ""
	if m.StartTime != nil {
		startISO = formatISO(*m.StartTime)
	}
	if m.SubmitTime != nil {
		submitISO = formatISO(*m.SubmitTime)
	}
	d := map[string]any{
		"id":             m.ID,
		"student_id":     m.StudentID,
		"question_ids":   ids,
		"answers":        answers,
		"start_time":     startISO,
		"submit_time":    submitISO,
		"remaining_time": m.RemainingTime,
		"duration":       m.Duration,
		"status":         m.Status,
		"result":         result,
		"created_at":     formatISO(m.CreatedAt),
	}
	if m.Score != nil {
		d["score"] = *m.Score
	} else {
		d["score"] = nil
	}
	return d
}
