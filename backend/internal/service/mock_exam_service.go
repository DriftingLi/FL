// Package service 模拟考试。
package service

import (
	"errors"

	"go.uber.org/zap"
	"gorm.io/gorm"

	"forklift-training/internal/model"
	"forklift-training/pkg/paging"
)

// mockExamDefaultCount 模拟考试默认题量（取消等级后：固定题量随机抽）。
const mockExamDefaultCount = 40

// MockExamService 模拟考试服务。
type MockExamService struct {
	db *gorm.DB
	ai *AIService

	logger *zap.Logger
}

// NewMockExamService 创建模拟考试服务实例。
func NewMockExamService(db *gorm.DB, ai *AIService, logger *zap.Logger) *MockExamService {
	return &MockExamService{db: db, ai: ai, logger: logger}
}

// ===== DTO（JSON 契约与 B6 前的 map key 逐字一致，前端零改动约束）=====

// MockExamStartDTO 开始模拟考试返回。
type MockExamStartDTO struct {
	MockExamID     int           `json:"mock_exam_id"`
	Duration       int           `json:"duration"`
	TotalScore     int           `json:"total_score"`
	TotalQuestions int           `json:"total_questions"`
	RemainingTime  int           `json:"remaining_time"`
	Questions      []QuestionDTO `json:"questions"`
}

// MockExamResumeDTO 恢复考试返回。
type MockExamResumeDTO struct {
	MockExamID    int           `json:"mock_exam_id"`
	Duration      int           `json:"duration"`
	RemainingTime int           `json:"remaining_time"`
	Questions     []QuestionDTO `json:"questions"`
	Answers       any           `json:"answers"`
	StartTime     string        `json:"start_time"`
}

// MockExamAnswerDetailDTO 交卷逐题明细。
type MockExamAnswerDetailDTO struct {
	QuestionID    int     `json:"question_id"`
	Type          string  `json:"type"`
	Content       string  `json:"content"`
	UserAnswer    any     `json:"user_answer"`
	CorrectAnswer string  `json:"correct_answer"`
	Score         float64 `json:"score"`
	MaxScore      float64 `json:"max_score"`
	Explanation   string  `json:"explanation"`
	Options       any     `json:"options"`
	IsCorrect     *bool   `json:"is_correct"`
	// AI 评分字段仅在短答 AI 评分成功时出现。
	AIScore    *float64 `json:"ai_score,omitempty"`
	AIComment  *string  `json:"ai_comment,omitempty"`
	AIFallback *bool    `json:"ai_fallback,omitempty"`
}

// MockExamSubmitDTO 交卷结果（同时落库为 result JSON）。
type MockExamSubmitDTO struct {
	TotalScore     float64                   `json:"total_score"`
	MaxScore       float64                   `json:"max_score"`
	CorrectCount   int                       `json:"correct_count"`
	TotalQuestions int                       `json:"total_questions"`
	Accuracy       float64                   `json:"accuracy"`
	Details        []MockExamAnswerDetailDTO `json:"details"`
}

// MockExamResultDTO 结果详情（交卷结果 + mock_exam_id + submit_time）。
type MockExamResultDTO struct {
	MockExamSubmitDTO
	MockExamID int    `json:"mock_exam_id"`
	SubmitTime string `json:"submit_time"`
}

// MockExamHistoryItemDTO 历史列表条目。
type MockExamHistoryItemDTO struct {
	ID            int      `json:"id"`
	StudentID     int      `json:"student_id"`
	QuestionIDs   any      `json:"question_ids"`
	Answers       any      `json:"answers"`
	StartTime     string   `json:"start_time"`
	SubmitTime    string   `json:"submit_time"`
	RemainingTime int      `json:"remaining_time"`
	Duration      int      `json:"duration"`
	Status        string   `json:"status"`
	Result        any      `json:"result"`
	CreatedAt     string   `json:"created_at"`
	Score         *float64 `json:"score"`
}

// MockExamHistoryDTO 历史列表信封。
type MockExamHistoryDTO struct {
	Total    int64                    `json:"total"`
	Page     int                      `json:"page"`
	PageSize int                      `json:"page_size"`
	Exams    []MockExamHistoryItemDTO `json:"exams"`
}

// Start 生成模拟考试：从 published 题库随机抽 count 题（不分等级、不分题型）。
func (s *MockExamService) Start(studentID, count, duration int) (*MockExamStartDTO, error) {
	if count <= 0 {
		count = mockExamDefaultCount
	}
	if duration <= 0 {
		duration = 90
	}

	selected, err := sampleQuestions(s.db, "", count)
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
		totalScore += int(mockExamMaxScore(&q))
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

	ordered := make([]QuestionDTO, 0, len(selected))
	for i := range selected {
		ordered = append(ordered, newQuestionDTO(&selected[i], false))
	}
	return &MockExamStartDTO{
		MockExamID:     mock.ID,
		Duration:       duration,
		TotalScore:     totalScore,
		TotalQuestions: len(questionIDs),
		RemainingTime:  mock.RemainingTime,
		Questions:      ordered,
	}, nil
}

// SaveProgress 保存进度。
func (s *MockExamService) SaveProgress(mockExamID, studentID int, answers map[string]any, remainingTime int) error {
	var mock model.MockExam
	if err := s.db.First(&mock, mockExamID).Error; err != nil {
		return errors.New("模拟考试不存在")
	}
	if err := guardOwnedInProgress(mock.StudentID, mock.Status, studentID, "无权操作此考试", "考试不在进行中"); err != nil {
		return err
	}
	ansJSON, _ := jsonMarshal(answers)
	mock.Answers = model.JSONB(ansJSON)
	mock.RemainingTime = remainingTime
	return s.db.Save(&mock).Error
}

// Resume 恢复考试。
func (s *MockExamService) Resume(mockExamID, studentID int) (*MockExamResumeDTO, error) {
	var mock model.MockExam
	if err := s.db.First(&mock, mockExamID).Error; err != nil {
		return nil, errors.New("模拟考试不存在")
	}
	if err := guardOwnedInProgress(mock.StudentID, mock.Status, studentID, "无权操作此考试", "考试不在进行中"); err != nil {
		return nil, err
	}

	var ids []int
	if len(mock.QuestionIDs) > 0 {
		_ = jsonUnmarshal(mock.QuestionIDs, &ids)
	}
	ordered, _ := loadOrderedQuestions(s.db, ids)
	questions := make([]QuestionDTO, 0, len(ordered))
	for i := range ordered {
		questions = append(questions, newQuestionDTO(&ordered[i], false))
	}
	answers := answersMapRoundTrip(mock.Answers)
	startISO := ""
	if mock.StartTime != nil {
		startISO = formatISO(*mock.StartTime)
	}
	return &MockExamResumeDTO{
		MockExamID:    mock.ID,
		Duration:      mock.Duration,
		RemainingTime: mock.RemainingTime,
		Questions:     questions,
		Answers:       answers,
		StartTime:     startISO,
	}, nil
}

// Submit 交卷。
func (s *MockExamService) Submit(mockExamID, studentID int) (*MockExamSubmitDTO, error) {
	var mock model.MockExam
	if err := s.db.First(&mock, mockExamID).Error; err != nil {
		return nil, errors.New("模拟考试不存在")
	}
	if err := guardOwnedInProgress(mock.StudentID, mock.Status, studentID, "无权操作此考试", "考试不在进行中"); err != nil {
		return nil, err
	}

	answersMap := answersMapRoundTrip(mock.Answers)
	var ids []int
	if len(mock.QuestionIDs) > 0 {
		_ = jsonUnmarshal(mock.QuestionIDs, &ids)
	}
	_, qMap := loadOrderedQuestions(s.db, ids)

	totalScore := 0.0
	maxScore := 0.0
	correctCount := 0
	details := make([]MockExamAnswerDetailDTO, 0, len(ids))

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

		detail := MockExamAnswerDetailDTO{
			QuestionID:    qid,
			Type:          question.Type,
			Content:       question.Content,
			UserAnswer:    userAnswer,
			CorrectAnswer: question.Answer,
			Score:         earned,
			MaxScore:      qMax,
			Explanation:   question.Explanation,
			IsCorrect:     isCorrect,
		}
		var options interface{}
		if len(question.Options) > 0 {
			_ = jsonUnmarshal(question.Options, &options)
		}
		detail.Options = options

		if question.Type == "short_answer" {
			if aiRes := aiGradeShortAnswer(s.ai, question.Content, question.ReferenceAnswer, question.ScoringCriteria, stringifyAnswer(userAnswer), qMax, nil); aiRes != nil {
				detail.AIScore = &aiRes.Score
				comment := aiRes.Comment
				detail.AIComment = &comment
				if aiRes.Fallback {
					fallback := true
					detail.AIFallback = &fallback
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
	result := MockExamSubmitDTO{
		TotalScore:     totalScore,
		MaxScore:       maxScore,
		CorrectCount:   correctCount,
		TotalQuestions: len(ids),
		Accuracy:       accuracy,
		Details:        details,
	}
	resultJSON, _ := jsonMarshal(result)
	mock.Result = model.JSONB(resultJSON)
	if err := s.db.Save(&mock).Error; err != nil {
		return nil, err
	}
	return &result, nil
}

// GetResult 获取结果。
func (s *MockExamService) GetResult(mockExamID, studentID int) (*MockExamResultDTO, error) {
	var mock model.MockExam
	if err := s.db.First(&mock, mockExamID).Error; err != nil {
		return nil, errors.New("模拟考试不存在")
	}
	if mock.StudentID != studentID {
		return nil, errors.New("无权查看此考试")
	}
	var result MockExamSubmitDTO
	if len(mock.Result) > 0 {
		_ = jsonUnmarshal(mock.Result, &result)
	}
	submitISO := ""
	if mock.SubmitTime != nil {
		submitISO = formatISO(*mock.SubmitTime)
	}
	return &MockExamResultDTO{
		MockExamSubmitDTO: result,
		MockExamID:        mock.ID,
		SubmitTime:        submitISO,
	}, nil
}

// GetHistory 历史列表。
func (s *MockExamService) GetHistory(studentID, page, pageSize int) *MockExamHistoryDTO {
	exams, total, page, pageSize := paging.Query[model.MockExam](s.db, page, pageSize, 10, "created_at DESC", func(q *gorm.DB) *gorm.DB {
		return q.Where("student_id = ?", studentID)
	})
	items := make([]MockExamHistoryItemDTO, 0, len(exams))
	for i := range exams {
		items = append(items, mockExamToDTO(&exams[i]))
	}
	return &MockExamHistoryDTO{
		Total:    total,
		Page:     page,
		PageSize: pageSize,
		Exams:    items,
	}
}

// ===== 辅助 =====

func mockExamMaxScore(q *model.Question) float64 {
	if q.Score > 0 {
		return float64(q.Score)
	}
	return questionMaxScore("mock_exam", q.Type)
}

// mockExamToDTO 历史条目构造（原 mockExamToDict 折叠入内）。
func mockExamToDTO(m *model.MockExam) MockExamHistoryItemDTO {
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
	return MockExamHistoryItemDTO{
		ID:            m.ID,
		StudentID:     m.StudentID,
		QuestionIDs:   ids,
		Answers:       answers,
		StartTime:     startISO,
		SubmitTime:    submitISO,
		RemainingTime: m.RemainingTime,
		Duration:      m.Duration,
		Status:        m.Status,
		Result:        result,
		CreatedAt:     formatISO(m.CreatedAt),
		Score:         m.Score,
	}
}
