// Package service 阅卷与复核。
package service

import (
	"errors"
	"fmt"

	"go.uber.org/zap"
	"gorm.io/gorm"

	"forklift-training/internal/model"
)

// GradingService 阅卷服务。
type GradingService struct {
	db *gorm.DB
	ai *AIService

	logger *zap.Logger
}

// NewGradingService 创建阅卷服务实例。
func NewGradingService(db *gorm.DB, ai *AIService, logger *zap.Logger) *GradingService {
	return &GradingService{db: db, ai: ai, logger: logger}
}

// GradingParticipantDTO 阅卷列表条目：participant 全字段 + 阅卷统计附加字段。
type GradingParticipantDTO struct {
	LevelExamParticipantDTO
	SessionName        string `json:"session_name"`
	StudentName        string `json:"student_name"`
	PassScore          int    `json:"pass_score"`
	UngradedCount      int    `json:"ungraded_count"`
	ObjectiveUngraded  int    `json:"objective_ungraded"`
	SubjectiveUngraded int    `json:"subjective_ungraded"`
	TotalAnswers       int    `json:"total_answers"`
	GradingStatus      string `json:"grading_status"`
}

// GradingParticipantDetailDTO 阅卷详情：participant 全字段 + 详情附加字段。
type GradingParticipantDetailDTO struct {
	LevelExamParticipantDTO
	SessionName        string               `json:"session_name"`
	StudentName        string               `json:"student_name"`
	PassScore          int                  `json:"pass_score"`
	Answers            []LevelExamAnswerDTO `json:"answers"`
	ObjectiveUngraded  int                  `json:"objective_ungraded"`
	SubjectiveUngraded int                  `json:"subjective_ungraded"`
}

// GetSubmittedParticipants 获取已提交的考试参与列表。
func (s *GradingService) GetSubmittedParticipants(sessionID *int) ([]GradingParticipantDTO, error) {
	q := s.db.Model(&model.ExamParticipant{}).Where("status IN ?", []string{"submitted", "timeout"})
	if sessionID != nil {
		q = q.Where("exam_session_id = ?", *sessionID)
	}
	var participants []model.ExamParticipant
	if err := q.Order("submit_time DESC").Find(&participants).Error; err != nil {
		return nil, err
	}
	result := make([]GradingParticipantDTO, 0, len(participants))
	for i := range participants {
		p := &participants[i]
		var session model.ExamSession
		s.db.First(&session, p.ExamSessionID)
		var student model.HrwaiUser
		s.db.First(&student, p.StudentID)

		var answers []model.ExamAnswer
		s.db.Where("exam_participant_id = ?", p.ID).Find(&answers)

		ungradedCount := 0
		objectiveUngraded := 0
		subjectiveUngraded := 0
		for j := range answers {
			a := &answers[j]
			if a.GraderID == nil {
				ungradedCount++
				var question model.Question
				if err := s.db.First(&question, a.QuestionID).Error; err == nil {
					if question.Type == "short_answer" {
						subjectiveUngraded++
					} else {
						objectiveUngraded++
					}
				}
			}
		}

		studentName := fmt.Sprintf("学员%d", p.StudentID)
		if student.ID != 0 {
			studentName = student.Username
		}
		passScore := 60
		if session.ID != 0 {
			passScore = session.PassScore
		}

		item := GradingParticipantDTO{LevelExamParticipantDTO: participantToDTO(p)}
		item.SessionName = session.Name
		item.StudentName = studentName
		item.PassScore = passScore
		item.UngradedCount = ungradedCount
		item.ObjectiveUngraded = objectiveUngraded
		item.SubjectiveUngraded = subjectiveUngraded
		item.TotalAnswers = len(answers)
		if ungradedCount > 0 {
			item.GradingStatus = "pending"
		} else {
			item.GradingStatus = "completed"
		}
		result = append(result, item)
	}
	return result, nil
}

// GetParticipantDetail 获取参与详情。
func (s *GradingService) GetParticipantDetail(participantID int) (*GradingParticipantDetailDTO, error) {
	var p model.ExamParticipant
	if err := s.db.First(&p, participantID).Error; err != nil {
		return nil, errors.New("考试参与记录不存在")
	}
	var session model.ExamSession
	s.db.First(&session, p.ExamSessionID)
	var student model.HrwaiUser
	s.db.First(&student, p.StudentID)

	var answers []model.ExamAnswer
	s.db.Where("exam_participant_id = ?", participantID).Find(&answers)

	answerList := make([]LevelExamAnswerDTO, 0, len(answers))
	objectiveUngraded := 0
	subjectiveUngraded := 0
	for i := range answers {
		a := &answers[i]
		item := examAnswerToDTO(a)
		var question model.Question
		if err := s.db.First(&question, a.QuestionID).Error; err == nil {
			item.Question = questionToDict(&question, true)
			if a.GraderID == nil {
				if question.Type == "short_answer" {
					subjectiveUngraded++
				} else {
					objectiveUngraded++
				}
			}
		}
		answerList = append(answerList, item)
	}

	studentName := fmt.Sprintf("学员%d", p.StudentID)
	if student.ID != 0 {
		studentName = student.Username
	}
	passScore := 60
	if session.ID != 0 {
		passScore = session.PassScore
	}

	item := GradingParticipantDetailDTO{LevelExamParticipantDTO: participantToDTO(&p)}
	item.SessionName = session.Name
	item.StudentName = studentName
	item.PassScore = passScore
	item.Answers = answerList
	item.ObjectiveUngraded = objectiveUngraded
	item.SubjectiveUngraded = subjectiveUngraded
	return &item, nil
}

// GradeAnswer 阅卷评分。
func (s *GradingService) GradeAnswer(answerID int, score float64, graderID int, comment string) (*LevelExamAnswerDTO, error) {
	var answer model.ExamAnswer
	if err := s.db.First(&answer, answerID).Error; err != nil {
		return nil, errors.New("答题记录不存在")
	}
	if answer.GraderID != nil {
		return nil, errors.New("该题已阅卷，请使用复核功能")
	}

	maxScore := s.questionMaxScore(answer.QuestionID)
	if score < 0 || score > maxScore {
		return nil, fmt.Errorf("分数应在0-%v之间", maxScore)
	}

	answer.Score = score
	correct := score >= maxScore*0.6
	answer.IsCorrect = &correct
	answer.GraderID = &graderID
	now := beijingNow()
	answer.GradedAt = &now
	answer.GradingComment = comment
	if err := s.db.Save(&answer).Error; err != nil {
		return nil, err
	}
	s.updateParticipantScore(answer.ExamParticipantID)
	d := examAnswerToDTO(&answer)
	return &d, nil
}

// RegradeAnswer 复核评分。
func (s *GradingService) RegradeAnswer(answerID int, score float64, graderID int, comment string) (*LevelExamAnswerDTO, error) {
	var answer model.ExamAnswer
	if err := s.db.First(&answer, answerID).Error; err != nil {
		return nil, errors.New("答题记录不存在")
	}
	if answer.GraderID == nil {
		return nil, errors.New("该题尚未阅卷，请使用阅卷功能")
	}

	maxScore := s.questionMaxScore(answer.QuestionID)
	if score < 0 || score > maxScore {
		return nil, fmt.Errorf("分数应在0-%v之间", maxScore)
	}

	answer.Score = score
	correct := score >= maxScore*0.6
	answer.IsCorrect = &correct
	answer.GraderID = &graderID
	now := beijingNow()
	answer.GradedAt = &now
	answer.GradingComment = comment
	if err := s.db.Save(&answer).Error; err != nil {
		return nil, err
	}
	s.updateParticipantScore(answer.ExamParticipantID)
	d := examAnswerToDTO(&answer)
	return &d, nil
}

// ConfirmAIGrading 确认 AI 评分。
func (s *GradingService) ConfirmAIGrading(answerID, graderID int) (*LevelExamAnswerDTO, error) {
	var answer model.ExamAnswer
	if err := s.db.First(&answer, answerID).Error; err != nil {
		return nil, errors.New("答题记录不存在")
	}
	if answer.AIScore == nil {
		return nil, errors.New("无AI评分可确认")
	}
	if answer.GraderID != nil {
		return nil, errors.New("该题已阅卷，请使用复核功能")
	}

	maxScore := s.questionMaxScore(answer.QuestionID)
	answer.Score = *answer.AIScore
	correct := *answer.AIScore >= maxScore*0.6
	answer.IsCorrect = &correct
	answer.GraderID = &graderID
	now := beijingNow()
	answer.GradedAt = &now
	answer.GradingComment = fmt.Sprintf("[AI评分确认] %s", answer.AIComment)
	if err := s.db.Save(&answer).Error; err != nil {
		return nil, err
	}
	s.updateParticipantScore(answer.ExamParticipantID)
	d := examAnswerToDTO(&answer)
	return &d, nil
}

// AIGradeAnswer AI 评分。
func (s *GradingService) AIGradeAnswer(answerID int, userID *int) (*LevelExamAnswerDTO, error) {
	var answer model.ExamAnswer
	if err := s.db.First(&answer, answerID).Error; err != nil {
		return nil, errors.New("答题记录不存在")
	}
	if answer.GraderID != nil {
		return nil, errors.New("该题已阅卷，无法重新AI评分")
	}
	var question model.Question
	if err := s.db.First(&question, answer.QuestionID).Error; err != nil {
		return nil, errors.New("题目不存在")
	}
	if question.Type != "short_answer" {
		return nil, errors.New("仅简答题支持AI评分")
	}
	if s.ai == nil {
		return nil, errors.New("AI服务不可用")
	}

	maxScore := float64(question.Score)
	if question.Score <= 0 {
		maxScore = examScoreMap[question.Type]
	}
	res := s.ai.GradeShortAnswer(question.Content, question.ReferenceAnswer, question.ScoringCriteria, answer.UserAnswer, maxScore, userID)
	if res == nil {
		return nil, errors.New("AI评分失败，请稍后重试或手动阅卷")
	}
	answer.AIScore = floatPtr(res.Score)
	comment := res.Comment
	if res.Fallback {
		comment = "[AI评分降级] " + comment
	}
	answer.AIComment = comment
	now := beijingNow()
	answer.AIGradedAt = &now
	if err := s.db.Save(&answer).Error; err != nil {
		return nil, err
	}
	d := examAnswerToDTO(&answer)
	return &d, nil
}

// BatchConfirmObjective 批量确认客观题。
func (s *GradingService) BatchConfirmObjective(participantID, graderID int) (map[string]any, error) {
	var p model.ExamParticipant
	if err := s.db.First(&p, participantID).Error; err != nil {
		return nil, errors.New("考试参与记录不存在")
	}
	var answers []model.ExamAnswer
	s.db.Where("exam_participant_id = ?", participantID).Find(&answers)
	if len(answers) == 0 {
		return nil, errors.New("没有答题记录")
	}

	confirmedCount := 0
	now := beijingNow()
	for i := range answers {
		a := &answers[i]
		if a.GraderID != nil {
			continue
		}
		var question model.Question
		if err := s.db.First(&question, a.QuestionID).Error; err != nil {
			continue
		}
		if question.Type != "short_answer" {
			a.GraderID = &graderID
			a.GradedAt = &now
			a.GradingComment = "[系统自动批改-导师确认]"
			s.db.Save(a)
			confirmedCount++
		}
	}
	if confirmedCount > 0 {
		s.updateParticipantScore(participantID)
	}
	return map[string]any{"confirmed_count": confirmedCount}, nil
}

// GetGradingStats 阅卷统计。
func (s *GradingService) GetGradingStats(sessionID *int) map[string]any {
	pendingQ := s.db.Model(&model.ExamAnswer{}).Where("is_correct IS NULL")
	gradedQ := s.db.Model(&model.ExamAnswer{}).Where("grader_id IS NOT NULL")
	aiPendingQ := s.db.Model(&model.ExamAnswer{}).Where("is_correct IS NULL AND grader_id IS NULL AND ai_score IS NOT NULL")
	if sessionID != nil {
		pendingQ = pendingQ.Joins("JOIN exam_participant ON exam_participant.id = exam_answer.exam_participant_id").Where("exam_participant.exam_session_id = ?", *sessionID)
		gradedQ = gradedQ.Joins("JOIN exam_participant ON exam_participant.id = exam_answer.exam_participant_id").Where("exam_participant.exam_session_id = ?", *sessionID)
		aiPendingQ = aiPendingQ.Joins("JOIN exam_participant ON exam_participant.id = exam_answer.exam_participant_id").Where("exam_participant.exam_session_id = ?", *sessionID)
	}
	var pendingCount, gradedCount, aiPendingCount int64
	pendingQ.Count(&pendingCount)
	gradedQ.Count(&gradedCount)
	aiPendingQ.Count(&aiPendingCount)
	return map[string]any{
		"pending_count":    pendingCount,
		"graded_count":     gradedCount,
		"ai_pending_count": aiPendingCount,
	}
}

// updateParticipantScore 更新参与记录总分。
func (s *GradingService) updateParticipantScore(participantID int) {
	var p model.ExamParticipant
	if err := s.db.First(&p, participantID).Error; err != nil {
		return
	}
	var answers []model.ExamAnswer
	s.db.Where("exam_participant_id = ?", participantID).Find(&answers)

	hasUngraded := false
	objectiveScore := 0.0
	subjectiveScore := 0.0
	for i := range answers {
		a := &answers[i]
		if a.GraderID == nil {
			hasUngraded = true
		}
		var question model.Question
		if err := s.db.First(&question, a.QuestionID).Error; err == nil {
			if question.Type == "short_answer" {
				subjectiveScore += a.Score
			} else {
				objectiveScore += a.Score
			}
		}
	}

	p.ObjectiveScore = floatPtr(objectiveScore)
	p.SubjectiveScore = floatPtr(subjectiveScore)
	total := objectiveScore + subjectiveScore

	if !hasUngraded {
		p.Score = floatPtr(total)
		var session model.ExamSession
		passScore := 60.0
		if err := s.db.First(&session, p.ExamSessionID).Error; err == nil {
			passScore = float64(session.PassScore)
		}
		p.IsPassed = total >= passScore
	} else {
		p.Score = nil
		p.IsPassed = false
	}
	s.db.Save(&p)
}

// questionMaxScore 获取题目满分（优先 question.score，否则用 examScoreMap）。
func (s *GradingService) questionMaxScore(questionID int) float64 {
	var question model.Question
	if err := s.db.First(&question, questionID).Error; err != nil {
		return 10
	}
	if question.Score > 0 {
		return float64(question.Score)
	}
	if v, ok := examScoreMap[question.Type]; ok {
		return v
	}
	return 10
}
