// Package service 阅卷与复核，对应 Python grading_service。
package service

import (
	"errors"
	"fmt"

	"gorm.io/gorm"

	"forklift-training/internal/model"
)

// GradingService 阅卷服务。
type GradingService struct {
	db *gorm.DB
	ai *AIService
}

// NewGradingService 创建阅卷服务实例。
func NewGradingService(db *gorm.DB, ai *AIService) *GradingService {
	return &GradingService{db: db, ai: ai}
}

// GetSubmittedParticipants 获取已提交的考试参与列表，对应 Python get_submitted_participants。
func (s *GradingService) GetSubmittedParticipants(sessionID *int) ([]map[string]interface{}, error) {
	q := s.db.Model(&model.ExamParticipant{}).Where("status IN ?", []string{"submitted", "timeout"})
	if sessionID != nil {
		q = q.Where("exam_session_id = ?", *sessionID)
	}
	var participants []model.ExamParticipant
	if err := q.Order("submit_time DESC").Find(&participants).Error; err != nil {
		return nil, err
	}

	result := make([]map[string]interface{}, 0, len(participants))
	for i := range participants {
		p := &participants[i]
		var session model.ExamSession
		s.db.First(&session, p.ExamSessionID)
		var student model.Student
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
		if student.StudentID != 0 {
			studentName = student.Name
		}
		passScore := 60
		if session.ID != 0 {
			passScore = session.PassScore
		}

		item := participantToDict(p)
		item["session_name"] = session.Name
		item["session_level"] = session.Level
		item["student_name"] = studentName
		item["pass_score"] = passScore
		item["ungraded_count"] = ungradedCount
		item["objective_ungraded"] = objectiveUngraded
		item["subjective_ungraded"] = subjectiveUngraded
		item["total_answers"] = len(answers)
		if ungradedCount > 0 {
			item["grading_status"] = "pending"
		} else {
			item["grading_status"] = "completed"
		}
		result = append(result, item)
	}
	return result, nil
}

// GetParticipantDetail 获取参与详情，对应 Python get_participant_detail。
func (s *GradingService) GetParticipantDetail(participantID int) (map[string]interface{}, error) {
	var p model.ExamParticipant
	if err := s.db.First(&p, participantID).Error; err != nil {
		return nil, errors.New("考试参与记录不存在")
	}
	var session model.ExamSession
	s.db.First(&session, p.ExamSessionID)
	var student model.Student
	s.db.First(&student, p.StudentID)

	var answers []model.ExamAnswer
	s.db.Where("exam_participant_id = ?", participantID).Find(&answers)

	answerList := make([]map[string]interface{}, 0, len(answers))
	objectiveUngraded := 0
	subjectiveUngraded := 0
	for i := range answers {
		a := &answers[i]
		item := examAnswerToDict(a)
		var question model.Question
		if err := s.db.First(&question, a.QuestionID).Error; err == nil {
			item["question"] = questionToDict(&question, true)
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
	if student.StudentID != 0 {
		studentName = student.Name
	}
	passScore := 60
	if session.ID != 0 {
		passScore = session.PassScore
	}

	result := participantToDict(&p)
	result["session_name"] = session.Name
	result["session_level"] = session.Level
	result["student_name"] = studentName
	result["pass_score"] = passScore
	result["answers"] = answerList
	result["objective_ungraded"] = objectiveUngraded
	result["subjective_ungraded"] = subjectiveUngraded
	return result, nil
}

// GradeAnswer 阅卷评分，对应 Python grade_answer。
func (s *GradingService) GradeAnswer(answerID int, score float64, graderID int, comment string) (map[string]interface{}, error) {
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
	return examAnswerToDict(&answer), nil
}

// RegradeAnswer 复核评分，对应 Python regrade_answer。
func (s *GradingService) RegradeAnswer(answerID int, score float64, graderID int, comment string) (map[string]interface{}, error) {
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
	return examAnswerToDict(&answer), nil
}

// ConfirmAIGrading 确认 AI 评分，对应 Python confirm_ai_grading。
func (s *GradingService) ConfirmAIGrading(answerID, graderID int) (map[string]interface{}, error) {
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
	return examAnswerToDict(&answer), nil
}

// AIGradeAnswer AI 评分，对应 Python ai_grade_answer。
func (s *GradingService) AIGradeAnswer(answerID int, userID *int) (map[string]interface{}, error) {
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
	return examAnswerToDict(&answer), nil
}

// BatchConfirmObjective 批量确认客观题，对应 Python batch_confirm_objective。
func (s *GradingService) BatchConfirmObjective(participantID, graderID int) (map[string]interface{}, error) {
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
	return map[string]interface{}{"confirmed_count": confirmedCount}, nil
}

// GetGradingStats 阅卷统计，对应 Python get_grading_stats。
func (s *GradingService) GetGradingStats(sessionID *int) map[string]interface{} {
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
	return map[string]interface{}{
		"pending_count":    pendingCount,
		"graded_count":     gradedCount,
		"ai_pending_count": aiPendingCount,
	}
}

// updateParticipantScore 更新参与记录总分，对应 Python _update_participant_score。
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
		if p.IsPassed {
			s.updateStudentLevel(p.StudentID, p.ExamSessionID)
		}
	} else {
		p.Score = nil
		p.IsPassed = false
	}
	s.db.Save(&p)
}

// updateStudentLevel 晋级，对应 Python _update_student_level。
func (s *GradingService) updateStudentLevel(studentID, sessionID int) {
	var session model.ExamSession
	if err := s.db.First(&session, sessionID).Error; err != nil {
		return
	}
	var student model.Student
	if err := s.db.First(&student, studentID).Error; err != nil {
		return
	}
	nextLevel, ok := levelPromotion[session.Level]
	if !ok {
		return
	}
	current := levelOrder[student.Level]
	next := levelOrder[nextLevel]
	if next > current {
		student.Level = nextLevel
		now := beijingNow()
		student.LevelUpdatedAt = &now
		s.db.Save(&student)
	}
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
