// Package service 模拟考试，对应 Python mock_exam_service。
package service

import (
	"errors"
	"math/rand"
	"time"

	"gorm.io/gorm"

	"forklift-training/internal/model"
)

// mockExamDefaultConfig 组卷配置，与 Python DEFAULT_CONFIG 一致。
var mockExamDefaultConfig = map[string]map[string]int{
	"beginner":     {"single_choice": 20, "multi_choice": 10, "true_false": 10, "fault_image": 5, "short_answer": 2},
	"intermediate": {"single_choice": 15, "multi_choice": 15, "true_false": 8, "fault_image": 8, "short_answer": 3},
	"advanced":     {"single_choice": 10, "multi_choice": 15, "true_false": 5, "fault_image": 10, "short_answer": 5},
}

// MockExamService 模拟考试服务。
type MockExamService struct {
	db  *gorm.DB
	ai  *AIService
	rng *rand.Rand
}

// NewMockExamService 创建模拟考试服务实例。
func NewMockExamService(db *gorm.DB, ai *AIService) *MockExamService {
	return &MockExamService{db: db, ai: ai, rng: rand.New(rand.NewSource(time.Now().UnixNano()))}
}

// Start 生成模拟考试，对应 Python generate_mock_exam。
func (s *MockExamService) Start(studentID int, level string, duration int) (map[string]interface{}, error) {
	if !containsString(validQuestionLevels, level) {
		return nil, errors.New("无效的等级")
	}
	if duration <= 0 {
		duration = 90
	}
	config, ok := mockExamDefaultConfig[level]
	if !ok {
		config = mockExamDefaultConfig["beginner"]
	}

	questionIDs := make([]int, 0)
	totalScore := 0
	for _, qType := range validQuestionTypes {
		count := config[qType]
		if count <= 0 {
			continue
		}
		var questions []model.Question
		s.db.Where("level = ? AND type = ? AND status = ?", level, qType, "published").Find(&questions)
		actualCount := count
		if actualCount > len(questions) {
			actualCount = len(questions)
		}
		if actualCount > 0 {
			s.rng.Shuffle(len(questions), func(i, j int) { questions[i], questions[j] = questions[j], questions[i] })
			for i := 0; i < actualCount; i++ {
				questionIDs = append(questionIDs, questions[i].ID)
				score := float64(questions[i].Score)
				if questions[i].Score <= 0 {
					score = mockExamScoreMap[qType]
				}
				totalScore += int(score)
			}
		}
	}

	if len(questionIDs) == 0 {
		return nil, errors.New("该等级下没有可用的题目")
	}
	s.rng.Shuffle(len(questionIDs), func(i, j int) { questionIDs[i], questionIDs[j] = questionIDs[j], questionIDs[i] })

	idsJSON, _ := jsonMarshal(questionIDs)
	emptyJSON, _ := jsonMarshal(map[string]interface{}{})
	startTime := beijingNow()
	mock := model.MockExam{
		StudentID:     studentID,
		Level:         level,
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

	var questions []model.Question
	s.db.Where("id IN ?", questionIDs).Find(&questions)
	qMap := map[int]*model.Question{}
	for i := range questions {
		qMap[questions[i].ID] = &questions[i]
	}
	ordered := make([]map[string]interface{}, 0, len(questionIDs))
	for _, qid := range questionIDs {
		if q, ok := qMap[qid]; ok {
			ordered = append(ordered, questionToDict(q, false))
		}
	}
	return map[string]interface{}{
		"mock_exam_id":    mock.ID,
		"level":           level,
		"duration":        duration,
		"total_score":     totalScore,
		"total_questions": len(questionIDs),
		"remaining_time":  mock.RemainingTime,
		"questions":       ordered,
	}, nil
}

// SaveProgress 保存进度，对应 Python save_mock_exam_progress。
func (s *MockExamService) SaveProgress(mockExamID, studentID int, answers map[string]interface{}, remainingTime int) error {
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

// Resume 恢复考试，对应 Python resume_mock_exam。
func (s *MockExamService) Resume(mockExamID, studentID int) (map[string]interface{}, error) {
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
	ordered := make([]map[string]interface{}, 0, len(ids))
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
	return map[string]interface{}{
		"mock_exam_id":   mock.ID,
		"level":          mock.Level,
		"duration":       mock.Duration,
		"remaining_time": mock.RemainingTime,
		"questions":      ordered,
		"answers":        answers,
		"start_time":     startISO,
	}, nil
}

// Submit 交卷，对应 Python submit_mock_exam。
func (s *MockExamService) Submit(mockExamID, studentID int) (map[string]interface{}, error) {
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

	var answersMap map[string]interface{}
	if len(mock.Answers) > 0 {
		_ = jsonUnmarshal(mock.Answers, &answersMap)
	}
	if answersMap == nil {
		answersMap = map[string]interface{}{}
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
	details := make([]map[string]interface{}, 0, len(ids))

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

		detail := map[string]interface{}{
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
	result := map[string]interface{}{
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

// GetResult 获取结果，对应 Python get_mock_exam_result。
func (s *MockExamService) GetResult(mockExamID, studentID int) (map[string]interface{}, error) {
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
	resultMap := map[string]interface{}{}
	if m, ok := result.(map[string]interface{}); ok {
		resultMap = m
	}
	resultMap["mock_exam_id"] = mock.ID
	resultMap["level"] = mock.Level
	submitISO := ""
	if mock.SubmitTime != nil {
		submitISO = formatISO(*mock.SubmitTime)
	}
	resultMap["submit_time"] = submitISO
	return resultMap, nil
}

// GetHistory 历史列表，对应 Python get_mock_exam_history。
func (s *MockExamService) GetHistory(studentID, page, pageSize int) map[string]interface{} {
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
	items := make([]map[string]interface{}, 0, len(exams))
	for i := range exams {
		items = append(items, mockExamToDict(&exams[i]))
	}
	return map[string]interface{}{
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

func mockExamToDict(m *model.MockExam) map[string]interface{} {
	var ids, answers, result interface{}
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
	d := map[string]interface{}{
		"id":             m.ID,
		"student_id":     m.StudentID,
		"level":          m.Level,
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
