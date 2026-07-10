// Package service 等级考试与晋级，对应 Python level_exam_service。
package service

import (
	"errors"
	"math/rand"
	"time"

	"gorm.io/gorm"

	"forklift-training/internal/model"
)

// 等级考试组卷配置，与 Python EXAM_QUESTION_CONFIG 一致。
var examQuestionConfig = map[string]map[string]int{
	"single_choice": {"count": 12, "score": 3},
	"true_false":    {"count": 8, "score": 2},
	"multi_choice":  {"count": 5, "score": 4},
	"fault_image":   {"count": 3, "score": 6},
	"short_answer":  {"count": 2, "score": 5},
}

var validSessionStatuses = []string{"upcoming", "ongoing", "finished"}

// LevelExamService 等级考试服务。
type LevelExamService struct {
	db *gorm.DB
	ai *AIService
}

// NewLevelExamService 创建等级考试服务。
func NewLevelExamService(db *gorm.DB, ai *AIService) *LevelExamService {
	return &LevelExamService{db: db, ai: ai}
}

// CreateSession 创建考试场次，对应 Python create_exam_session。
func (s *LevelExamService) CreateSession(data map[string]interface{}, createdBy *int) (map[string]interface{}, error) {
	name, _ := data["name"].(string)
	if name == "" {
		return nil, errors.New("考试名称不能为空")
	}
	level, _ := data["level"].(string)
	if !containsString(validQuestionLevels, level) {
		return nil, errors.New("无效的等级，只能创建初级、中级或高级考试")
	}
	startStr, _ := data["start_time"].(string)
	endStr, _ := data["end_time"].(string)
	if startStr == "" || endStr == "" {
		return nil, errors.New("考试时间信息不完整")
	}
	startTime, err := parseFlexibleTime(startStr)
	if err != nil {
		return nil, errors.New("开始时间格式错误")
	}
	endTime, err := parseFlexibleTime(endStr)
	if err != nil {
		return nil, errors.New("结束时间格式错误")
	}
	session := model.ExamSession{
		Name:       name,
		Level:      level,
		StartTime:  startTime,
		EndTime:    endTime,
		Duration:   90,
		Status:     "upcoming",
		CreatedBy:  createdBy,
		TotalScore: 100,
		PassScore:  60,
		CreatedAt:  beijingNow(),
		UpdatedAt:  beijingNow(),
	}
	if err := s.db.Create(&session).Error; err != nil {
		return nil, err
	}
	return sessionToDict(&session), nil
}

// UpdateSession 更新场次，对应 Python update_exam_session。
func (s *LevelExamService) UpdateSession(id int, data map[string]interface{}) (map[string]interface{}, error) {
	var session model.ExamSession
	if err := s.db.First(&session, id).Error; err != nil {
		return nil, errors.New("考试场次不存在")
	}
	if session.Status != "upcoming" {
		return nil, errors.New("只能编辑未开始的考试")
	}
	if v, ok := data["name"]; ok {
		session.Name, _ = v.(string)
	}
	if v, ok := data["level"]; ok {
		session.Level, _ = v.(string)
	}
	if v, ok := data["start_time"]; ok {
		if t, err := parseFlexibleTime(toString(v)); err == nil {
			session.StartTime = t
		}
	}
	if v, ok := data["end_time"]; ok {
		if t, err := parseFlexibleTime(toString(v)); err == nil {
			session.EndTime = t
		}
	}
	if v, ok := data["question_config"]; ok {
		if b, err := jsonMarshal(v); err == nil {
			session.QuestionConfig = model.JSONB(b)
		}
	}
	session.UpdatedAt = beijingNow()
	if err := s.db.Save(&session).Error; err != nil {
		return nil, err
	}
	return sessionToDict(&session), nil
}

// DeleteSession 删除场次。
func (s *LevelExamService) DeleteSession(id int) error {
	var session model.ExamSession
	if err := s.db.First(&session, id).Error; err != nil {
		return errors.New("考试场次不存在")
	}
	if session.Status != "upcoming" {
		return errors.New("只能删除未开始的考试")
	}
	return s.db.Delete(&session).Error
}

// ListSessions 列表（自动推进 upcoming→ongoing），对应 Python get_exam_sessions。
func (s *LevelExamService) ListSessions(page, pageSize int, level, status string, includeParticipants bool) map[string]interface{} {
	if page <= 0 {
		page = 1
	}
	if pageSize <= 0 {
		pageSize = 20
	}
	q := s.db.Model(&model.ExamSession{})
	if level != "" {
		q = q.Where("level = ?", level)
	}
	if status != "" {
		q = q.Where("status = ?", status)
	}
	var total int64
	q.Count(&total)
	var sessions []model.ExamSession
	q.Order("start_time DESC").Offset((page - 1) * pageSize).Limit(pageSize).Find(&sessions)
	now := beijingNow()
	out := make([]map[string]interface{}, 0, len(sessions))
	for i := range sessions {
		sess := &sessions[i]
		if sess.Status == "upcoming" && now.After(sess.StartTime) {
			sess.Status = "ongoing"
			sess.UpdatedAt = beijingNow()
			s.db.Save(sess)
		}
		d := sessionToDict(sess)
		if includeParticipants {
			var parts []model.ExamParticipant
			s.db.Where("exam_session_id = ?", sess.ID).Find(&parts)
			ps := make([]map[string]interface{}, 0, len(parts))
			for j := range parts {
				pd := participantToDict(&parts[j])
				var st model.Student
				if err := s.db.First(&st, parts[j].StudentID).Error; err == nil {
					pd["student_name"] = st.Name
				}
				ps = append(ps, pd)
			}
			d["participants"] = ps
		}
		out = append(out, d)
	}
	return map[string]interface{}{
		"total":     total,
		"page":      page,
		"page_size": pageSize,
		"sessions":  out,
	}
}

// GetSessionDetail 场次详情。
func (s *LevelExamService) GetSessionDetail(id int) (map[string]interface{}, error) {
	var session model.ExamSession
	if err := s.db.First(&session, id).Error; err != nil {
		return nil, errors.New("考试场次不存在")
	}
	return sessionToDict(&session), nil
}

// UpdateSessionStatus 更新状态（带状态机校验）。
func (s *LevelExamService) UpdateSessionStatus(id int, status string) (map[string]interface{}, error) {
	var session model.ExamSession
	if err := s.db.First(&session, id).Error; err != nil {
		return nil, errors.New("考试场次不存在")
	}
	validTrans := map[string][]string{"upcoming": {"ongoing"}, "ongoing": {"finished"}, "finished": {}}
	allowed := validTrans[session.Status]
	if !containsString(allowed, status) {
		return nil, errors.New("不能从" + session.Status + "状态切换到" + status + "状态")
	}
	session.Status = status
	session.UpdatedAt = beijingNow()
	if err := s.db.Save(&session).Error; err != nil {
		return nil, err
	}
	return sessionToDict(&session), nil
}

// EnterExam 学员进入考试，组卷并创建参与记录，对应 Python enter_exam。
func (s *LevelExamService) EnterExam(sessionID, studentID int) (map[string]interface{}, error) {
	var session model.ExamSession
	if err := s.db.First(&session, sessionID).Error; err != nil {
		return nil, errors.New("考试场次不存在")
	}
	now := beijingNow()
	if session.Status == "upcoming" && now.After(session.StartTime) {
		session.Status = "ongoing"
		session.UpdatedAt = beijingNow()
		s.db.Save(&session)
	}
	if session.Status == "finished" || now.After(session.EndTime) {
		return nil, errors.New("考试已结束")
	}
	if session.Status == "upcoming" && now.Before(session.StartTime) {
		return nil, errors.New("考试尚未开始")
	}
	var student model.Student
	if err := s.db.First(&student, studentID).Error; err != nil {
		return nil, errors.New("学员不存在")
	}
	studentLevel := levelOrder[student.Level]
	if studentLevel == 0 {
		studentLevel = 1
	}
	examLevel := levelOrder[session.Level]
	if examLevel == 0 {
		examLevel = 1
	}
	if studentLevel != examLevel {
		levelNames := map[string]string{"beginner": "初级", "intermediate": "中级", "advanced": "高级", "expert": "顶级"}
		name := levelNames[student.Level]
		if name == "" {
			name = "初级"
		}
		return nil, errors.New(name + "学徒只能参加" + name + "等级考试")
	}
	var participant model.ExamParticipant
	err := s.db.Where("exam_session_id = ? AND student_id = ?", sessionID, studentID).First(&participant).Error
	if err == nil {
		if participant.Status == "submitted" {
			return nil, errors.New("您已提交过此考试")
		}
		if participant.Status == "in_progress" {
			return s.getExamData(&session, &participant)
		}
	}
	questionIDs, _ := s.generateQuestionIDs(&session)
	rand.Shuffle(len(questionIDs), func(i, j int) { questionIDs[i], questionIDs[j] = questionIDs[j], questionIDs[i] })
	idsJSON, _ := jsonMarshal(questionIDs)
	startTime := beijingNow()
	participant = model.ExamParticipant{
		ExamSessionID:   sessionID,
		StudentID:       studentID,
		Status:          "in_progress",
		StartTime:       &startTime,
		RemainingTime:   session.Duration * 60,
		QuestionIDs:     model.JSONB(idsJSON),
		AnswersSnapshot: model.JSONB([]byte("{}")),
		CreatedAt:       beijingNow(),
	}
	if err := s.db.Create(&participant).Error; err != nil {
		return nil, err
	}
	return s.getExamData(&session, &participant)
}

func (s *LevelExamService) generateQuestionIDs(session *model.ExamSession) ([]int, int) {
	allowed := examAllowedLevels[session.Level]
	if allowed == nil {
		allowed = []string{session.Level}
	}
	questionIDs := []int{}
	total := 0
	for qType, cfg := range examQuestionConfig {
		var questions []model.Question
		s.db.Where("type = ? AND status = ? AND level IN ?", qType, "published", allowed).Find(&questions)
		actual := cfg["count"]
		if actual > len(questions) {
			actual = len(questions)
		}
		if actual > 0 {
			perm := rand.Perm(len(questions))
			for i := 0; i < actual; i++ {
				questionIDs = append(questionIDs, questions[perm[i]].ID)
			}
		}
		total += actual * cfg["score"]
	}
	return questionIDs, total
}

func (s *LevelExamService) getExamData(session *model.ExamSession, p *model.ExamParticipant) (map[string]interface{}, error) {
	var ids []int
	if len(p.QuestionIDs) > 0 {
		_ = jsonUnmarshal(p.QuestionIDs, &ids)
	}
	var questions []model.Question
	if len(ids) > 0 {
		s.db.Where("id IN ?", ids).Find(&questions)
	}
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
	if len(p.AnswersSnapshot) > 0 {
		_ = jsonUnmarshal(p.AnswersSnapshot, &answers)
	}
	if answers == nil {
		answers = map[string]interface{}{}
	}
	startISO := ""
	if p.StartTime != nil {
		startISO = formatISO(*p.StartTime)
	}
	return map[string]interface{}{
		"participant_id": p.ID,
		"session":        sessionToDict(session),
		"questions":      ordered,
		"answers":        answers,
		"remaining_time": p.RemainingTime,
		"start_time":     startISO,
	}, nil
}

// SaveAnswer 保存答案快照，对应 Python save_exam_answer。
func (s *LevelExamService) SaveAnswer(participantID, studentID int, answers map[string]interface{}, remainingTime int) error {
	var p model.ExamParticipant
	if err := s.db.First(&p, participantID).Error; err != nil {
		return errors.New("考试参与记录不存在")
	}
	if p.StudentID != studentID {
		return errors.New("无权操作")
	}
	if p.Status != "in_progress" {
		return errors.New("考试不在进行中")
	}
	b, _ := jsonMarshal(answers)
	p.AnswersSnapshot = model.JSONB(b)
	p.RemainingTime = remainingTime
	return s.db.Save(&p).Error
}

// SubmitExam 交卷评分，对应 Python submit_exam。
func (s *LevelExamService) SubmitExam(participantID, studentID int, isTimeout bool) (map[string]interface{}, error) {
	var p model.ExamParticipant
	if err := s.db.First(&p, participantID).Error; err != nil {
		return nil, errors.New("考试参与记录不存在")
	}
	if p.StudentID != studentID {
		return nil, errors.New("无权操作")
	}
	if p.Status != "in_progress" {
		return nil, errors.New("考试不在进行中")
	}
	var answers map[string]interface{}
	if len(p.AnswersSnapshot) > 0 {
		_ = jsonUnmarshal(p.AnswersSnapshot, &answers)
	}
	var ids []int
	if len(p.QuestionIDs) > 0 {
		_ = jsonUnmarshal(p.QuestionIDs, &ids)
	}
	var questions []model.Question
	if len(ids) > 0 {
		s.db.Where("id IN ?", ids).Find(&questions)
	}
	qMap := map[int]*model.Question{}
	for i := range questions {
		qMap[questions[i].ID] = &questions[i]
	}

	objectiveScore := 0.0
	subjectiveScore := 0.0
	hasSubjective := false

	// 清旧答题
	s.db.Where("exam_participant_id = ?", p.ID).Delete(&model.ExamAnswer{})

	for _, qid := range ids {
		question := qMap[qid]
		if question == nil {
			continue
		}
		userAnswer := answers[intToString(qid)]
		cfg := examQuestionConfig[question.Type]
		maxScore := float64(cfg["score"])
		isCorrect, earned := gradeQuestion(question, userAnswer, maxScore)

		if question.Type == "short_answer" {
			hasSubjective = true
			ans := model.ExamAnswer{
				ExamParticipantID: p.ID,
				QuestionID:        qid,
				UserAnswer:        stringifyAnswer(userAnswer),
				Score:             0,
			}
			s.db.Create(&ans)
			if s.ai != nil {
				aiRes := s.ai.GradeShortAnswer(question.Content, question.ReferenceAnswer, question.ScoringCriteria, stringifyAnswer(userAnswer), maxScore, nil)
				if aiRes != nil {
					ans.AIScore = floatPtr(aiRes.Score)
					comment := aiRes.Comment
					if aiRes.Fallback {
						comment = "[AI评分降级] " + comment
					}
					ans.AIComment = comment
					now := beijingNow()
					ans.AIGradedAt = &now
					s.db.Save(&ans)
				}
			}
			_ = hasSubjective
		} else {
			objectiveScore += earned
			if isCorrect != nil && !*isCorrect {
				_ = addToWrongQuestions(s.db, studentID, qid)
			}
			ans := model.ExamAnswer{
				ExamParticipantID: p.ID,
				QuestionID:        qid,
				UserAnswer:        stringifyAnswer(userAnswer),
				Score:             earned,
			}
			if isCorrect != nil {
				ans.IsCorrect = isCorrect
			}
			s.db.Create(&ans)
		}
	}

	if isTimeout {
		p.Status = "timeout"
	} else {
		p.Status = "submitted"
	}
	submitTime := beijingNow()
	p.SubmitTime = &submitTime
	p.ObjectiveScore = floatPtr(objectiveScore)
	p.SubjectiveScore = floatPtr(subjectiveScore)

	// 是否还有未阅卷答题
	var ungradedCount int64
	s.db.Model(&model.ExamAnswer{}).Where("exam_participant_id = ? AND grader_id IS NULL", p.ID).Count(&ungradedCount)
	if ungradedCount == 0 {
		total := objectiveScore + subjectiveScore
		p.Score = floatPtr(total)
		var session model.ExamSession
		passScore := 60.0
		if err := s.db.First(&session, p.ExamSessionID).Error; err == nil {
			passScore = float64(session.PassScore)
		}
		passed := total >= passScore
		p.IsPassed = passed
		if passed {
			s.updateStudentLevel(p.StudentID, p.ExamSessionID)
		}
	} else {
		p.Score = nil
		p.IsPassed = false
	}
	if err := s.db.Save(&p).Error; err != nil {
		return nil, err
	}
	return participantToDict(&p), nil
}

// updateStudentLevel 晋级，对应 Python _update_student_level。
func (s *LevelExamService) updateStudentLevel(studentID, sessionID int) {
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

// GetResult 考试结果详情。
func (s *LevelExamService) GetResult(participantID, studentID int) (map[string]interface{}, error) {
	var p model.ExamParticipant
	if err := s.db.First(&p, participantID).Error; err != nil {
		return nil, errors.New("考试记录不存在")
	}
	if p.StudentID != studentID {
		return nil, errors.New("无权查看")
	}
	var answers []model.ExamAnswer
	s.db.Where("exam_participant_id = ?", p.ID).Find(&answers)
	details := make([]map[string]interface{}, 0, len(answers))
	for _, a := range answers {
		d := examAnswerToDict(&a)
		var q model.Question
		if err := s.db.First(&q, a.QuestionID).Error; err == nil {
			d["question"] = questionToDict(&q, true)
		}
		details = append(details, d)
	}
	return map[string]interface{}{
		"participant": participantToDict(&p),
		"answers":     details,
	}, nil
}

// GetStudentHistory 学员考试历史。
func (s *LevelExamService) GetStudentHistory(studentID, page, pageSize int) map[string]interface{} {
	if page <= 0 {
		page = 1
	}
	if pageSize <= 0 {
		pageSize = 10
	}
	q := s.db.Model(&model.ExamParticipant{}).Where("student_id = ?", studentID)
	var total int64
	q.Count(&total)
	var parts []model.ExamParticipant
	q.Order("created_at DESC").Offset((page - 1) * pageSize).Limit(pageSize).Find(&parts)
	items := make([]map[string]interface{}, 0, len(parts))
	for _, p := range parts {
		var sess model.ExamSession
		item := participantToDict(&p)
		if err := s.db.First(&sess, p.ExamSessionID).Error; err == nil {
			item["session_name"] = sess.Name
			item["level"] = sess.Level
		}
		items = append(items, item)
	}
	return map[string]interface{}{
		"total":     total,
		"page":      page,
		"page_size": pageSize,
		"records":   items,
	}
}

// GetAvailableExams 可用考试列表，对应 Python get_available_exams。
func (s *LevelExamService) GetAvailableExams(studentID int) ([]map[string]interface{}, error) {
	var student model.Student
	if err := s.db.First(&student, studentID).Error; err != nil {
		return nil, errors.New("学员不存在")
	}
	now := beijingNow()
	var sessions []model.ExamSession
	s.db.Order("start_time DESC").Find(&sessions)
	studentLevel := levelOrder[student.Level]
	if studentLevel == 0 {
		studentLevel = 1
	}
	available := []map[string]interface{}{}
	for i := range sessions {
		sess := &sessions[i]
		if sess.StartTime.IsZero() || sess.EndTime.IsZero() {
			continue
		}
		if sess.Status == "upcoming" && now.After(sess.StartTime) {
			sess.Status = "ongoing"
			sess.UpdatedAt = beijingNow()
			s.db.Save(sess)
		}
		effStatus := sess.Status
		if effStatus == "upcoming" && now.After(sess.StartTime) {
			effStatus = "ongoing"
		}
		if effStatus != "finished" && now.After(sess.EndTime) {
			effStatus = "finished"
		}
		var participant model.ExamParticipant
		hasPart := s.db.Where("exam_session_id = ? AND student_id = ?", sess.ID, studentID).First(&participant).Error == nil
		if effStatus == "finished" && !hasPart {
			continue
		}
		examLevel := levelOrder[sess.Level]
		if examLevel == 0 {
			examLevel = 1
		}
		canEnter := studentLevel == examLevel
		item := sessionToDict(sess)
		item["status"] = effStatus
		item["has_participated"] = hasPart
		if hasPart {
			item["participant_status"] = participant.Status
			item["participant_id"] = participant.ID
		} else {
			item["participant_status"] = nil
			item["participant_id"] = nil
		}
		item["can_enter"] = canEnter
		available = append(available, item)
	}
	return available, nil
}

// ===== dict 辅助 =====

func sessionToDict(s *model.ExamSession) map[string]interface{} {
	var qc interface{}
	if len(s.QuestionConfig) > 0 {
		_ = jsonUnmarshal(s.QuestionConfig, &qc)
	}
	return map[string]interface{}{
		"id":              s.ID,
		"name":            s.Name,
		"level":           s.Level,
		"start_time":      formatISO(s.StartTime),
		"end_time":        formatISO(s.EndTime),
		"duration":        s.Duration,
		"status":          s.Status,
		"created_by":      s.CreatedBy,
		"question_config": qc,
		"total_score":     s.TotalScore,
		"pass_score":      s.PassScore,
		"created_at":      formatISO(s.CreatedAt),
		"updated_at":      formatISO(s.UpdatedAt),
	}
}

func participantToDict(p *model.ExamParticipant) map[string]interface{} {
	var ids, snap interface{}
	if len(p.QuestionIDs) > 0 {
		_ = jsonUnmarshal(p.QuestionIDs, &ids)
	}
	if len(p.AnswersSnapshot) > 0 {
		_ = jsonUnmarshal(p.AnswersSnapshot, &snap)
	}
	startISO, submitISO := "", ""
	if p.StartTime != nil {
		startISO = formatISO(*p.StartTime)
	}
	if p.SubmitTime != nil {
		submitISO = formatISO(*p.SubmitTime)
	}
	d := map[string]interface{}{
		"id":               p.ID,
		"exam_session_id":  p.ExamSessionID,
		"student_id":       p.StudentID,
		"status":           p.Status,
		"start_time":       startISO,
		"submit_time":      submitISO,
		"remaining_time":   p.RemainingTime,
		"answers_snapshot": snap,
		"question_ids":     ids,
		"created_at":       formatISO(p.CreatedAt),
	}
	if p.Score != nil {
		d["score"] = *p.Score
	} else {
		d["score"] = nil
	}
	if p.ObjectiveScore != nil {
		d["objective_score"] = *p.ObjectiveScore
	} else {
		d["objective_score"] = nil
	}
	if p.SubjectiveScore != nil {
		d["subjective_score"] = *p.SubjectiveScore
	} else {
		d["subjective_score"] = nil
	}
	d["is_passed"] = p.IsPassed
	return d
}

func examAnswerToDict(a *model.ExamAnswer) map[string]interface{} {
	d := map[string]interface{}{
		"id":                  a.ID,
		"exam_participant_id": a.ExamParticipantID,
		"question_id":         a.QuestionID,
		"user_answer":         a.UserAnswer,
		"score":               a.Score,
		"grading_comment":     a.GradingComment,
		"ai_comment":          a.AIComment,
	}
	if a.IsCorrect != nil {
		d["is_correct"] = *a.IsCorrect
	} else {
		d["is_correct"] = nil
	}
	if a.GraderID != nil {
		d["grader_id"] = *a.GraderID
	} else {
		d["grader_id"] = nil
	}
	if a.GradedAt != nil {
		d["graded_at"] = formatISO(*a.GradedAt)
	} else {
		d["graded_at"] = nil
	}
	if a.AIScore != nil {
		d["ai_score"] = *a.AIScore
	} else {
		d["ai_score"] = nil
	}
	if a.AIGradedAt != nil {
		d["ai_graded_at"] = formatISO(*a.AIGradedAt)
	} else {
		d["ai_graded_at"] = nil
	}
	return d
}

// parseFlexibleTime 解析多种时间格式。
func parseFlexibleTime(s string) (time.Time, error) {
	formats := []string{
		time.RFC3339Nano, time.RFC3339,
		"2006-01-02T15:04:05.000000",
		"2006-01-02T15:04:05",
		"2006-01-02 15:04:05",
		"2006-01-02",
	}
	for _, f := range formats {
		if t, err := time.ParseInLocation(f, s, beijingLoc()); err == nil {
			return t, nil
		}
	}
	return time.Time{}, errors.New("时间格式错误")
}

func beijingLoc() *time.Location {
	loc, err := time.LoadLocation("Asia/Shanghai")
	if err != nil {
		loc = time.FixedZone("CST", 8*3600)
	}
	return loc
}

func jsonMarshal(v interface{}) ([]byte, error) {
	return jsonMarshalImpl(v)
}

func jsonUnmarshal(b []byte, v interface{}) error {
	return jsonUnmarshalImpl(b, v)
}
