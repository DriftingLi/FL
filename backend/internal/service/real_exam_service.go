// Package service 真题套卷：列表/按卷练习/按卷考试（ADR-0022）。
// 卷题来自 real_exam_paper_question 的固定题序；权益按套粒度（real_paper:<id>）。
package service

import (
	"errors"
	"strconv"

	"go.uber.org/zap"
	"gorm.io/gorm"

	"forklift-training/internal/model"
)

// RealExamService 真题套卷服务。
type RealExamService struct {
	db     *gorm.DB
	points *PointsService
	logger *zap.Logger
}

// NewRealExamService 创建真题套卷服务。
func NewRealExamService(db *gorm.DB, points *PointsService, logger *zap.Logger) *RealExamService {
	return &RealExamService{db: db, points: points, logger: logger}
}

// RealExamPaperDTO 套卷列表项。
type RealExamPaperDTO struct {
	PaperID         int    `json:"paper_id"`
	Title           string `json:"title"`
	Year            *int   `json:"year,omitempty" extensions:"x-optional"`
	Source          string `json:"source,omitempty" extensions:"x-optional"`
	QuestionCount   int    `json:"question_count"`
	DurationMinutes int    `json:"duration_minutes"`
	Entitled        bool   `json:"entitled"`
	Price           int    `json:"price"`
}

// paperQuestionIDs 卷内题目 id（按 order_num 升序，仅 published）。
// 按卷练习/开考链路上「三件不同的事」各自的载体（ADR-0064 决策 1/2）。此前它们与
// ErrRealPaperUnavailable 混在一格 errStatusAll(404) 里，A 批在 real_exam.go 的注释里
// 把这件事登记为「正解在 service 侧升哨兵」：
//   - ErrRealPaperNotRedeemed：这份内容**在平台上、也可见**，只是当前主体没为它付过 ——
//     与「不存在」是两件事；真题卷的可见性本来就是公开的（列表里能看见），所以这里不必
//     套 ADR-0062 决策 3 那条「未兑换按不存在、不泄漏存在性」，直接如实回 4xx 与其文案。
//   - ErrRealPaperEmpty：卷内暂无已发布题目 —— 域状态问题，既不是没找到也不是服务端故障。
//
// 两条都留在 4xx 侧，是因为 ADR-0064 决策 9 之后 5xx 不再外发原文：把它们做成 500
// 会让学员看到天书（A 批注释点名的正是这一条）。
var (
	ErrRealPaperNotRedeemed = errors.New("请先兑换该真题卷")
	ErrRealPaperEmpty       = errors.New("该卷暂无已发布题目")
)

func (s *RealExamService) paperQuestionIDs(paperID int) ([]int, []model.Question, error) {
	var relIDs []int
	if err := s.db.Model(&model.RealExamPaperQuestion{}).
		Where("paper_id = ?", paperID).
		Order("order_num ASC, question_id ASC").
		Pluck("question_id", &relIDs).Error; err != nil {
		return nil, nil, errors.New("查询卷题失败")
	}
	if len(relIDs) == 0 {
		return nil, nil, nil
	}
	var all []model.Question
	if err := s.db.Where("id IN ? AND status = ?", relIDs, "published").Find(&all).Error; err != nil {
		return nil, nil, errors.New("查询题目失败")
	}
	byID := make(map[int]model.Question, len(all))
	for i := range all {
		byID[all[i].ID] = all[i]
	}
	ordered := make([]model.Question, 0, len(relIDs))
	ids := make([]int, 0, len(relIDs))
	for _, id := range relIDs {
		if q, ok := byID[id]; ok {
			ids = append(ids, id)
			ordered = append(ordered, q)
		}
	}
	return ids, ordered, nil
}

// ListPapers 当前证件的套卷列表（year desc，附兑换状态与单价）。
func (s *RealExamService) ListPapers(userID, credentialID int) []RealExamPaperDTO {
	out := make([]RealExamPaperDTO, 0, 8)
	if credentialID <= 0 {
		return out
	}
	var papers []model.RealExamPaper
	// 套卷按自身证件列分区（归属分区，ADR-0056 §2）：上方 credentialID<=0 已早退，不存在 nil 分支。
	if err := EntityOwnedBy(s.db.Model(&model.RealExamPaper{}), "credential_id", &credentialID).
		Where("status = 1").
		Order("year DESC NULLS LAST, paper_id DESC").
		Find(&papers).Error; err != nil {
		s.logger.Warn("查询真题卷列表失败", zap.Int("credential_id", credentialID), zap.Error(err))
		return out
	}
	price := s.points.realPaperPrice()
	for i := range papers {
		p := &papers[i]
		entitled, entErr := s.points.HasEntitlement(userID, RealPaperSKU(p.PaperID), strconv.Itoa(p.PaperID))
		if entErr != nil {
			// 有意尽力而为（ADR-0062 票6 的声明式例外，故记日志）：列表上的「已解锁」徽标查不到
			// 时按未解锁显示，只少一个标记；真正的门禁在 StartPractice / StartExam 两条读路径上
			// 如实上抛，不在这里放行。
			s.logger.Warn("查询真题卷兑换状态失败", zap.Int("paper_id", p.PaperID), zap.Error(entErr))
		}
		out = append(out, RealExamPaperDTO{
			PaperID:         p.PaperID,
			Title:           p.Title,
			Year:            p.Year,
			Source:          derefString(p.Source),
			QuestionCount:   p.QuestionCount,
			DurationMinutes: p.DurationMinutes,
			Entitled:        entitled,
			Price:           price,
		})
	}
	return out
}

// StartPaperPractice 按卷练习开始/续练：固定卷序（不随机），断点续练复用 practice_progress。
// 装配形态（#385）：续练协商（同集沿用卷序与游标/集合变化刷新复位）走 ResumeSet 单点。
func (s *RealExamService) StartPaperPractice(studentID, paperID int) (*PracticeStartResultDTO, error) {
	var paper model.RealExamPaper
	if err := s.db.Where("paper_id = ? AND status = 1", paperID).First(&paper).Error; err != nil {
		return nil, ErrRealPaperUnavailable
	}
	entitled, entErr := s.points.HasEntitlement(studentID, RealPaperSKU(paperID), strconv.Itoa(paperID))
	if entErr != nil {
		return nil, entErr
	}
	if !entitled {
		return nil, ErrRealPaperNotRedeemed
	}
	allIDs, all, err := s.paperQuestionIDs(paperID)
	if err != nil {
		return nil, err
	}
	if len(allIDs) == 0 {
		return nil, ErrRealPaperEmpty
	}
	byID := make(map[int]model.Question, len(all))
	for i := range all {
		byID[all[i].ID] = all[i]
	}

	ids, startIdx, err := ResumeSet(s.db, studentID, nil, ResumeSetSpec{
		Mode:       string(PracticeModePaper(paperID)),
		FreshIDs:   allIDs,
		ReuseSaved: true,
	})
	if err != nil {
		return nil, err
	}

	out := make([]QuestionDTO, 0, len(ids))
	for _, id := range ids {
		if q, ok := byID[id]; ok {
			out = append(out, newQuestionDTO(&q, false))
		}
	}
	return &PracticeStartResultDTO{
		Questions:    out,
		CurrentIndex: startIdx,
		Total:        len(ids),
		Completed:    startIdx,
	}, nil
}

// StartPaperExam 按卷开考：固定题集 + 卷时长写入 mock_exam（paper_id 归卷），
// 之后的 save/resume/submit/result 原样复用模拟考链路。
func (s *RealExamService) StartPaperExam(studentID, paperID int) (*MockExamStartDTO, error) {
	var paper model.RealExamPaper
	if err := s.db.Where("paper_id = ? AND status = 1", paperID).First(&paper).Error; err != nil {
		return nil, ErrRealPaperUnavailable
	}
	entitled, entErr := s.points.HasEntitlement(studentID, RealPaperSKU(paperID), strconv.Itoa(paperID))
	if entErr != nil {
		return nil, entErr
	}
	if !entitled {
		return nil, ErrRealPaperNotRedeemed
	}
	questionIDs, ordered, err := s.paperQuestionIDs(paperID)
	if err != nil {
		return nil, err
	}
	if len(questionIDs) == 0 {
		return nil, ErrRealPaperEmpty
	}

	// 清理废弃未交卷记录（与随机模拟考同口径）。
	if err := s.db.
		Where("student_id = ? AND status <> ? AND created_at < ?",
			studentID, mockExamStatusSubmitted, beijingNow().Add(-mockExamAbandonTTL)).
		Delete(&model.MockExam{}).Error; err != nil {
		s.logger.Warn("清理废弃模拟考试记录失败", zap.Int("student_id", studentID), zap.Error(err))
	}

	duration := paper.DurationMinutes
	if duration <= 0 {
		duration = 90
	}
	totalScore := 0
	for i := range ordered {
		totalScore += int(mockExamMaxScore(&ordered[i]))
	}

	idsJSON, _ := jsonMarshal(questionIDs)
	emptyJSON, _ := jsonMarshal(map[string]any{})
	startTime := beijingNow()
	paperIDCopy := paperID
	mock := model.MockExam{
		StudentID: studentID,
		// 分区取**卷所属证件**（不是学员当前证件）：按卷开考的分区由卷本身决定 ——
		// 学员显式浏览别的证件的卷时，记录也该落在卷的证件下，否则切回本证件反而看不到它（#1003）。
		CredentialID:  &paper.CredentialID,
		QuestionIDs:   model.JSONB(idsJSON),
		Answers:       model.JSONB(emptyJSON),
		Duration:      duration,
		Status:        mockExamStatusInProgress,
		StartTime:     &startTime,
		RemainingTime: duration * 60,
		PaperID:       &paperIDCopy,
	}
	if err := s.db.Create(&mock).Error; err != nil {
		return nil, err
	}

	questionsOut := make([]QuestionDTO, 0, len(ordered))
	for i := range ordered {
		questionsOut = append(questionsOut, newQuestionDTO(&ordered[i], false))
	}
	return &MockExamStartDTO{
		MockExamID:     mock.ID,
		Duration:       duration,
		TotalScore:     totalScore,
		TotalQuestions: len(questionIDs),
		RemainingTime:  mock.RemainingTime,
		Questions:      questionsOut,
	}, nil
}

// derefString 字符串指针解引用（nil → ""）。
func derefString(p *string) string {
	if p == nil {
		return ""
	}
	return *p
}
