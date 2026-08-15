// Package service 等级考试与晋级。
package service

import (
	"encoding/json"
	"errors"
	"math/rand"
	"time"

	"go.uber.org/zap"
	"gorm.io/gorm"

	"forklift-training/internal/model"
	"forklift-training/pkg/paging"
)

// 等级考试组卷配置（各题型题量；分值见 questionScoreByFlow["level_exam"]）。
var examQuestionConfig = map[string]int{
	"single_choice": 12,
	"true_false":    8,
	"multi_choice":  5,
	"fault_image":   3,
	"short_answer":  2,
}

var validSessionStatuses = []string{"upcoming", "ongoing", "finished"} //nolint:unused

// LevelExamService 等级考试服务。
type LevelExamService struct {
	db *gorm.DB
	ai *AIService

	logger *zap.Logger
}

// NewLevelExamService 创建等级考试服务。
func NewLevelExamService(db *gorm.DB, ai *AIService, logger *zap.Logger) *LevelExamService {
	return &LevelExamService{db: db, ai: ai, logger: logger}
}

// ===== DTO（JSON 契约与 B4 前的 map key 逐字一致，前端零改动约束）=====

// LevelExamSessionDTO 考试场次。
type LevelExamSessionDTO struct {
	ID             int                       `json:"id"`
	Name           string                    `json:"name"`
	StartTime      string                    `json:"start_time"`
	EndTime        string                    `json:"end_time"`
	Duration       int                       `json:"duration"`
	Status         string                    `json:"status"`
	CreatedBy      *int                      `json:"created_by"`
	QuestionConfig any                       `json:"question_config"`
	TotalScore     int                       `json:"total_score"`
	PassScore      int                       `json:"pass_score"`
	CreatedAt      string                    `json:"created_at"`
	UpdatedAt      string                    `json:"updated_at"`
	Participants   []LevelExamParticipantDTO `json:"participants,omitempty"`
}

// LevelExamParticipantDTO 考试参与记录。
type LevelExamParticipantDTO struct {
	ID              int      `json:"id"`
	ExamSessionID   int      `json:"exam_session_id"`
	StudentID       int      `json:"student_id"`
	Status          string   `json:"status"`
	StartTime       string   `json:"start_time"`
	SubmitTime      string   `json:"submit_time"`
	RemainingTime   int      `json:"remaining_time"`
	AnswersSnapshot any      `json:"answers_snapshot"`
	QuestionIDs     any      `json:"question_ids"`
	CreatedAt       string   `json:"created_at"`
	Score           *float64 `json:"score"`
	ObjectiveScore  *float64 `json:"objective_score"`
	SubjectiveScore *float64 `json:"subjective_score"`
	IsPassed        bool     `json:"is_passed"`
	// 列表路径附带：学员姓名 / 场次名称（未加载时省略）。
	StudentName string `json:"student_name,omitempty"`
	SessionName string `json:"session_name,omitempty"`
}

// LevelExamAnswerDTO 考试答题记录。
type LevelExamAnswerDTO struct {
	ID                int      `json:"id"`
	ExamParticipantID int      `json:"exam_participant_id"`
	QuestionID        int      `json:"question_id"`
	UserAnswer        string   `json:"user_answer"`
	Score             float64  `json:"score"`
	GradingComment    string   `json:"grading_comment"`
	AIComment         string   `json:"ai_comment"`
	IsCorrect         *bool    `json:"is_correct"`
	GraderID          *int     `json:"grader_id"`
	GradedAt          *string  `json:"graded_at"`
	AIScore           *float64 `json:"ai_score"`
	AIGradedAt        *string  `json:"ai_graded_at"`
	// 结果详情路径附带题目（未加载时省略）。
	// AI 评分降级标记（加性变更：短答 AI 评分降级时出现，否则省略）。
	AIFallback *bool        `json:"ai_fallback,omitempty"`
	Question   *QuestionDTO `json:"question,omitempty"`
}

// LevelExamDataDTO 进入考试返回（participant + 试卷 + 快照）。
type LevelExamDataDTO struct {
	ParticipantID int                 `json:"participant_id"`
	Session       LevelExamSessionDTO `json:"session"`
	Questions     []QuestionDTO       `json:"questions"`
	Answers       any                 `json:"answers"`
	RemainingTime int                 `json:"remaining_time"`
	StartTime     string              `json:"start_time"`
}

// LevelExamSessionListDTO 场次列表信封。
type LevelExamSessionListDTO struct {
	Total    int64                 `json:"total"`
	Page     int                   `json:"page"`
	PageSize int                   `json:"page_size"`
	Sessions []LevelExamSessionDTO `json:"sessions"`
}

// LevelExamHistoryDTO 学员考试历史信封。
type LevelExamHistoryDTO struct {
	Total    int64                     `json:"total"`
	Page     int                       `json:"page"`
	PageSize int                       `json:"page_size"`
	Records  []LevelExamParticipantDTO `json:"records"`
}

// LevelExamAvailableDTO 可用考试条目（session 全字段 + 可用性附加字段，
// 其中 status 覆盖 session 的原始状态为生效状态）。
type LevelExamAvailableDTO struct {
	LevelExamSessionDTO
	Status            string `json:"status"`
	HasParticipated   bool   `json:"has_participated"`
	ParticipantStatus any    `json:"participant_status"`
	ParticipantID     any    `json:"participant_id"`
	CanEnter          bool   `json:"can_enter"`
}

// LevelExamResultDTO 考试结果详情信封。
type LevelExamResultDTO struct {
	Participant LevelExamParticipantDTO `json:"participant"`
	Answers     []LevelExamAnswerDTO    `json:"answers"`
}

// CreateSession 创建考试场次。
func (s *LevelExamService) CreateSession(data map[string]any, createdBy *int) (*LevelExamSessionDTO, error) {
	name, _ := data["name"].(string)
	if name == "" {
		return nil, errors.New("考试名称不能为空")
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
	d := sessionToDTO(&session)
	return &d, nil
}

// UpdateSession 更新场次。
func (s *LevelExamService) UpdateSession(id int, data map[string]any) (*LevelExamSessionDTO, error) {
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
	d := sessionToDTO(&session)
	return &d, nil
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

// ListSessions 列表（GET 纯读：展示用基于时间的生效状态，不写库）。
func (s *LevelExamService) ListSessions(page, pageSize int, status string, includeParticipants bool) *LevelExamSessionListDTO {
	sessions, total, page, pageSize := paging.Query[model.ExamSession](s.db, page, pageSize, 20, "start_time DESC", func(q *gorm.DB) *gorm.DB {
		if status != "" {
			q = q.Where("status = ?", status)
		}
		return q
	})
	now := beijingNow()
	out := make([]LevelExamSessionDTO, 0, len(sessions))
	for i := range sessions {
		sess := &sessions[i]
		d := sessionToDTO(sess)
		d.Status = effectiveExamStatus(sess.Status, sess.StartTime, sess.EndTime, now)
		if includeParticipants {
			var parts []model.ExamParticipant
			s.db.Where("exam_session_id = ?", sess.ID).Find(&parts)
			ps := make([]LevelExamParticipantDTO, 0, len(parts))
			for j := range parts {
				pd := participantToDTO(&parts[j])
				var st model.HrwaiUser
				if err := s.db.First(&st, parts[j].StudentID).Error; err == nil {
					pd.StudentName = st.Username
				}
				ps = append(ps, pd)
			}
			d.Participants = ps
		}
		out = append(out, d)
	}
	return &LevelExamSessionListDTO{
		Total:    total,
		Page:     page,
		PageSize: pageSize,
		Sessions: out,
	}
}

// GetSessionDetail 场次详情（展示基于时间的生效状态，不写库）。
func (s *LevelExamService) GetSessionDetail(id int) (*LevelExamSessionDTO, error) {
	var session model.ExamSession
	if err := s.db.First(&session, id).Error; err != nil {
		return nil, errors.New("考试场次不存在")
	}
	d := sessionToDTO(&session)
	d.Status = effectiveExamStatus(session.Status, session.StartTime, session.EndTime, beijingNow())
	return &d, nil
}

// UpdateSessionStatus 更新状态（带状态机校验）。
func (s *LevelExamService) UpdateSessionStatus(id int, status string) (*LevelExamSessionDTO, error) {
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
	d := sessionToDTO(&session)
	return &d, nil
}

// EnterExam 学员进入考试，组卷并创建参与记录。
func (s *LevelExamService) EnterExam(sessionID, studentID int) (*LevelExamDataDTO, error) {
	var session model.ExamSession
	if err := s.db.First(&session, sessionID).Error; err != nil {
		return nil, errors.New("考试场次不存在")
	}
	now := beijingNow()
	if newStatus, advanced := advanceExamStatus(session.Status, session.StartTime, now); advanced {
		session.Status = newStatus
		session.UpdatedAt = beijingNow()
		s.db.Save(&session)
	}
	if session.Status == "finished" || now.After(session.EndTime) {
		return nil, errors.New("考试已结束")
	}
	if session.Status == "upcoming" && now.Before(session.StartTime) {
		return nil, errors.New("考试尚未开始")
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
	questionIDs := []int{}
	total := 0
	for qType, count := range examQuestionConfig {
		var questions []model.Question
		s.db.Where("type = ? AND status = ?", qType, "published").Find(&questions)
		actual := count
		if actual > len(questions) {
			actual = len(questions)
		}
		if actual > 0 {
			perm := rand.Perm(len(questions))
			for i := 0; i < actual; i++ {
				questionIDs = append(questionIDs, questions[perm[i]].ID)
			}
		}
		total += actual * int(questionMaxScore("level_exam", qType))
	}
	return questionIDs, total
}

func (s *LevelExamService) getExamData(session *model.ExamSession, p *model.ExamParticipant) (*LevelExamDataDTO, error) {
	var ids []int
	if len(p.QuestionIDs) > 0 {
		_ = jsonUnmarshal(p.QuestionIDs, &ids)
	}
	ordered, _ := loadOrderedQuestions(s.db, ids)
	questions := make([]QuestionDTO, 0, len(ordered))
	for i := range ordered {
		questions = append(questions, newQuestionDTO(&ordered[i], false))
	}
	answers := answersMapRoundTrip(p.AnswersSnapshot)
	startISO := ""
	if p.StartTime != nil {
		startISO = formatISO(*p.StartTime)
	}
	d := sessionToDTO(session)
	return &LevelExamDataDTO{
		ParticipantID: p.ID,
		Session:       d,
		Questions:     questions,
		Answers:       answers,
		RemainingTime: p.RemainingTime,
		StartTime:     startISO,
	}, nil
}

// SaveAnswer 保存答案快照。
// 经保存会话进度深模块（session_progress.go）唯一实现：load → 守卫（本人+进行中）→
// 快照 JSONB 三态归一 → db.Save（与 mock SaveProgress 同口径）。
func (s *LevelExamService) SaveAnswer(participantID, studentID int, answers map[string]any, remainingTime int) error {
	return saveSessionProgress(s.db, SessionProgressSpec[model.ExamParticipant]{
		notFoundErr: "考试参与记录不存在",
		load: func(db *gorm.DB) (model.ExamParticipant, error) {
			var p model.ExamParticipant
			return p, db.First(&p, participantID).Error
		},
		guard: func(p model.ExamParticipant) error {
			return guardOwnedInProgress(p.StudentID, p.Status, studentID, "无权操作", "考试不在进行中")
		},
		write: func(p *model.ExamParticipant, snapshot model.JSONB, rt int) {
			p.AnswersSnapshot = snapshot
			p.RemainingTime = rt
		},
	}, answers, remainingTime)
}

// SubmitExam 交卷评分。
// 语义：提交后即「待阅卷结算」——客观题即时判题回填 ObjectiveScore，
// 简答/未人工阅卷答题的分数与 IsPassed 不在此结算，统由阅卷端
// GradingService.updateParticipantScore 在所有答题阅卷后置后重算。
func (s *LevelExamService) SubmitExam(participantID, studentID int, isTimeout bool) (*LevelExamParticipantDTO, error) {
	var p model.ExamParticipant
	if err := s.db.First(&p, participantID).Error; err != nil {
		return nil, errors.New("考试参与记录不存在")
	}
	if err := guardOwnedInProgress(p.StudentID, p.Status, studentID, "无权操作", "考试不在进行中"); err != nil {
		return nil, err
	}
	answers := answersMapRoundTrip(p.AnswersSnapshot)
	var ids []int
	if len(p.QuestionIDs) > 0 {
		_ = jsonUnmarshal(p.QuestionIDs, &ids)
	}
	_, qMap := loadOrderedQuestions(s.db, ids)

	objectiveScore := 0.0

	// 清旧答题
	s.db.Where("exam_participant_id = ?", p.ID).Delete(&model.ExamAnswer{})

	engine := newGradingEngine(s.db)
	flow := gradingFlow{
		ai: shortAnswerGraderOf(s.ai),
		maxScore: func(q *model.Question) float64 {
			return questionMaxScore("level_exam", q.Type)
		},
	}
	results := engine.gradeSet(flow, qMap, ids, answers, studentID)

	for _, r := range results {
		question := r.Question
		qid := question.ID

		if question.Type == "short_answer" {
			ans := model.ExamAnswer{
				ExamParticipantID: p.ID,
				QuestionID:        qid,
				UserAnswer:        stringifyAnswer(r.UserAnswer),
				Score:             0,
			}
			s.db.Create(&ans)
			if r.ShortAnswer != nil {
				ans.AIScore = floatPtr(r.ShortAnswer.Score)
				ans.AIComment = r.ShortAnswer.Comment
				now := beijingNow()
				ans.AIGradedAt = &now
				s.db.Save(&ans)
			}
		} else {
			objectiveScore += r.Earned
			ans := model.ExamAnswer{
				ExamParticipantID: p.ID,
				QuestionID:        qid,
				UserAnswer:        stringifyAnswer(r.UserAnswer),
				Score:             r.Earned,
			}
			if r.IsCorrect != nil {
				ans.IsCorrect = r.IsCorrect
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
	// 主观分（简答）待阅卷结算：提交时不回填真实分，恒记 0；
	// 阅卷完成后由 GradingService.updateParticipantScore 置后重算并回填。
	p.SubjectiveScore = floatPtr(0)

	// 提交后待阅卷结算：交卷时尚未手动阅卷的答题（grader_id IS NULL，含全部简答与客观题）
	// 不计入总分。Score/IsPassed 不在此判定，而由阅卷端 GradingService.updateParticipantScore
	// 在所有答题阅卷完成后置后重算；此处仅当确无未阅卷答题时用客观分结算（主观分提交时恒 0）。
	var ungradedCount int64
	s.db.Model(&model.ExamAnswer{}).Where("exam_participant_id = ? AND grader_id IS NULL", p.ID).Count(&ungradedCount)
	if ungradedCount == 0 {
		total := objectiveScore // 主观分提交时恒 0，故 total 即客观分（等价于 objectiveScore + 0）
		p.Score = floatPtr(total)
		var session model.ExamSession
		passScore := 60.0
		if err := s.db.First(&session, p.ExamSessionID).Error; err == nil {
			passScore = float64(session.PassScore)
		}
		passed := total >= passScore
		p.IsPassed = passed
	} else {
		p.Score = nil
		p.IsPassed = false
	}
	if err := s.db.Save(&p).Error; err != nil {
		return nil, err
	}
	d := participantToDTO(&p)
	return &d, nil
}

// GetResult 考试结果详情。
func (s *LevelExamService) GetResult(participantID, studentID int) (*LevelExamResultDTO, error) {
	var p model.ExamParticipant
	if err := s.db.First(&p, participantID).Error; err != nil {
		return nil, errors.New("考试记录不存在")
	}
	if p.StudentID != studentID {
		return nil, errors.New("无权查看")
	}
	var answers []model.ExamAnswer
	s.db.Where("exam_participant_id = ?", p.ID).Find(&answers)
	qIDs := make([]int, 0, len(answers))
	for i := range answers {
		qIDs = append(qIDs, answers[i].QuestionID)
	}
	questions := loadQuestionsByIDs(s.db, qIDs)
	details := make([]LevelExamAnswerDTO, 0, len(answers))
	for _, a := range answers {
		d := examAnswerToDTO(&a)
		if q, ok := questions[a.QuestionID]; ok {
			question := newQuestionDTO(q, true)
			d.Question = &question
		}
		details = append(details, d)
	}
	return &LevelExamResultDTO{
		Participant: participantToDTO(&p),
		Answers:     details,
	}, nil
}

// GetStudentHistory 学员考试历史。
func (s *LevelExamService) GetStudentHistory(studentID, page, pageSize int) *LevelExamHistoryDTO {
	parts, total, page, pageSize := paging.Query[model.ExamParticipant](s.db, page, pageSize, 10, "created_at DESC", func(q *gorm.DB) *gorm.DB {
		return q.Where("student_id = ?", studentID)
	})
	items := make([]LevelExamParticipantDTO, 0, len(parts))
	for _, p := range parts {
		var sess model.ExamSession
		item := participantToDTO(&p)
		if err := s.db.First(&sess, p.ExamSessionID).Error; err == nil {
			item.SessionName = sess.Name
		}
		items = append(items, item)
	}
	return &LevelExamHistoryDTO{
		Total:    total,
		Page:     page,
		PageSize: pageSize,
		Records:  items,
	}
}

// GetAvailableExams 可用考试列表。
func (s *LevelExamService) GetAvailableExams(studentID int) ([]LevelExamAvailableDTO, error) {
	now := beijingNow()
	var sessions []model.ExamSession
	s.db.Order("start_time DESC").Find(&sessions)
	available := []LevelExamAvailableDTO{}
	for i := range sessions {
		sess := &sessions[i]
		if sess.StartTime.IsZero() || sess.EndTime.IsZero() {
			continue
		}
		effStatus := effectiveExamStatus(sess.Status, sess.StartTime, sess.EndTime, now)
		var participant model.ExamParticipant
		hasPart := s.db.Where("exam_session_id = ? AND student_id = ?", sess.ID, studentID).First(&participant).Error == nil
		if effStatus == "finished" && !hasPart {
			continue
		}
		// 取消等级制度：可进入 = 未结束 且 未提交过
		canEnter := effStatus != "finished" && !(hasPart && participant.Status == "submitted")
		item := LevelExamAvailableDTO{
			LevelExamSessionDTO: sessionToDTO(sess),
			Status:              effStatus,
			HasParticipated:     hasPart,
			CanEnter:            canEnter,
		}
		if hasPart {
			item.ParticipantStatus = participant.Status
			item.ParticipantID = participant.ID
		} else {
			item.ParticipantStatus = nil
			item.ParticipantID = nil
		}
		available = append(available, item)
	}
	return available, nil
}

// ===== DTO 构造（原 sessionToDict/participantToDict/examAnswerToDict 折叠入内）=====

func sessionToDTO(s *model.ExamSession) LevelExamSessionDTO {
	var qc any
	if len(s.QuestionConfig) > 0 {
		_ = jsonUnmarshal(s.QuestionConfig, &qc)
	}
	return LevelExamSessionDTO{
		ID:             s.ID,
		Name:           s.Name,
		StartTime:      formatISO(s.StartTime),
		EndTime:        formatISO(s.EndTime),
		Duration:       s.Duration,
		Status:         s.Status,
		CreatedBy:      s.CreatedBy,
		QuestionConfig: qc,
		TotalScore:     s.TotalScore,
		PassScore:      s.PassScore,
		CreatedAt:      formatISO(s.CreatedAt),
		UpdatedAt:      formatISO(s.UpdatedAt),
	}
}

func participantToDTO(p *model.ExamParticipant) LevelExamParticipantDTO {
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
	return LevelExamParticipantDTO{
		ID:              p.ID,
		ExamSessionID:   p.ExamSessionID,
		StudentID:       p.StudentID,
		Status:          p.Status,
		StartTime:       startISO,
		SubmitTime:      submitISO,
		RemainingTime:   p.RemainingTime,
		AnswersSnapshot: snap,
		QuestionIDs:     ids,
		CreatedAt:       formatISO(p.CreatedAt),
		Score:           p.Score,
		ObjectiveScore:  p.ObjectiveScore,
		SubjectiveScore: p.SubjectiveScore,
		IsPassed:        p.IsPassed,
	}
}

func examAnswerToDTO(a *model.ExamAnswer) LevelExamAnswerDTO {
	d := LevelExamAnswerDTO{
		ID:                a.ID,
		ExamParticipantID: a.ExamParticipantID,
		QuestionID:        a.QuestionID,
		UserAnswer:        a.UserAnswer,
		Score:             a.Score,
		GradingComment:    a.GradingComment,
		AIComment:         a.AIComment,
		IsCorrect:         a.IsCorrect,
		GraderID:          a.GraderID,
		GradedAt:          isoPtr(a.GradedAt),
		AIScore:           a.AIScore,
		AIGradedAt:        isoPtr(a.AIGradedAt),
	}
	// 降级标记单点还原：AI 注释携带统一前缀即视为降级（读侧无独立落库列）。
	if hasFallbackComment(a.AIComment) {
		fb := true
		d.AIFallback = &fb
	}
	return d
}

// isoPtr 将时间转为 ISO 字符串指针（DTO 中 nil 时间序列化为 null，与旧契约一致）。
func isoPtr(t *time.Time) *string {
	if t == nil {
		return nil
	}
	s := formatISO(*t)
	return &s
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
	return json.Marshal(v)
}

func jsonUnmarshal(b []byte, v interface{}) error {
	return json.Unmarshal(b, v)
}
