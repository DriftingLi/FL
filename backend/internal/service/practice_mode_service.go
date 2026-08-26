// Package service 题库练习模式。
package service

import (
	"encoding/json"
	"errors"
	"fmt"
	"math/rand"
	"time"

	"go.uber.org/zap"
	"gorm.io/gorm"

	"forklift-training/internal/clock"
	"forklift-training/internal/model"
	"forklift-training/pkg/paging"
)

// PracticeModeService 题库练习模式服务。
type PracticeModeService struct {
	db *gorm.DB
	// grader 短答 AI 判分 adapter（nil 时简答降级，与错题重做口径一致）。
	grader ShortAnswerGrader
	// explainer AI 解析 module（缓存/生成/降级策略单点，spec #295）。
	explainer *QuestionExplanation

	logger *zap.Logger
	clk    clock.Clock
}

// NewPracticeModeService 创建题库练习服务，ai 可为 nil（简答题与解析降级）。
func NewPracticeModeService(db *gorm.DB, ai *AIService, logger *zap.Logger) *PracticeModeService {
	return &PracticeModeService{
		db:        db,
		grader:    shortAnswerGraderOf(ai),
		explainer: NewQuestionExplanation(db, ai, logger),
		logger:    logger,
		clk:       clock.Real(),
	}
}

// NewPracticeModeServiceWithClock 注入式构造（测试用 Clock 定格，生产仍用 Real）。
func NewPracticeModeServiceWithClock(db *gorm.DB, ai *AIService, logger *zap.Logger, clk clock.Clock) *PracticeModeService {
	if clk == nil {
		clk = clock.Real()
	}
	return &PracticeModeService{
		db:        db,
		grader:    shortAnswerGraderOf(ai),
		explainer: NewQuestionExplanation(db, ai, logger),
		logger:    logger,
		clk:       clk,
	}
}

// SetClock 覆写时钟（测试用，参考 CheckInService 的 clk 注入形态）。
func (s *PracticeModeService) SetClock(clk clock.Clock) {
	if clk != nil {
		s.clk = clk
	}
}

// GetFreeQuestions 随机练习抽题：从 published 题库按条件随机抽取 count 题。
// count <= 0 时返回全部符合条件的题目（按 id 升序，不打乱）。
func (s *PracticeModeService) GetFreeQuestions(qType string, count int, credentialID ...*int) ([]QuestionDTO, error) {
	selected, err := sampleQuestions(s.db, qType, count, credentialID...)
	if err != nil {
		return nil, errors.New("查询题目失败")
	}
	if len(selected) == 0 {
		return nil, errors.New("没有符合条件的题目")
	}
	out := make([]QuestionDTO, 0, len(selected))
	for i := range selected {
		out = append(out, newQuestionDTO(&selected[i], false))
	}
	return out, nil
}

// StartTagPractice 标签练习开始/续练：首次进入按标签抽题并持久化题目顺序，
// 再次进入复用已保存顺序与游标（断点续练）；已完成则重新抽题。
// mode = "tag:<tagID>"，count <= 0 表示该标签全部题目。
func (s *PracticeModeService) StartTagPractice(studentID, tagID, count int, credentialID ...*int) (*PracticeStartResultDTO, error) {
	if tagID <= 0 {
		return nil, errors.New("请指定题库标签")
	}
	var tagCount int64
	if err := s.db.Model(&model.QuestionTag{}).Where("id = ? AND status = ?", tagID, 1).Count(&tagCount).Error; err != nil {
		return nil, errors.New("查询标签失败")
	}
	if tagCount == 0 {
		return nil, errors.New("题库标签不存在或已停用")
	}
	var all []model.Question
	qAll := s.db.Model(&model.Question{}).
		Where("id IN (SELECT question_id FROM question_tag_relation WHERE tag_id = ?) AND status = ?", tagID, "published")
	if len(credentialID) > 0 && credentialID[0] != nil {
		qAll = qAll.Where("credential_id = ?", *credentialID[0])
	}
	if err := qAll.Order("id ASC").
		Find(&all).Error; err != nil {
		return nil, errors.New("查询题目失败")
	}
	if len(all) == 0 {
		return nil, errors.New("该标签下暂无已发布题目")
	}
	allIDs := make([]int, len(all))
	for i := range all {
		allIDs[i] = all[i].ID
	}
	byID := make(map[int]model.Question, len(all))
	for i := range all {
		byID[all[i].ID] = all[i]
	}

	mode := fmt.Sprintf("tag:%d", tagID)
	var prog model.PracticeProgress
	if err := s.db.Where("student_id = ? AND practice_mode = ?", studentID, mode).Limit(1).Find(&prog).Error; err != nil {
		return nil, err
	}

	ids := allIDs
	startIdx := 0
	if prog.ID != 0 && prog.CurrentIndex < prog.Total {
		// 续练：解析已保存的题目顺序（顺序固定，游标位置才有效）
		var saved []int
		if err := json.Unmarshal(prog.QuestionIDs, &saved); err == nil && len(saved) > 0 {
			// 题目集合未变则沿用保存顺序；已变则按新集合刷新（游标截断保护）
			if sameIDSet(saved, allIDs) {
				ids = saved
				startIdx = prog.CurrentIndex
			}
		}
	}
	if startIdx == 0 && (prog.ID == 0 || prog.CurrentIndex >= prog.Total) {
		// 首次进入或已完成：随机抽题并固定顺序
		if count > 0 && len(ids) > count {
			rand.Shuffle(len(ids), func(i, j int) { ids[i], ids[j] = ids[j], ids[i] })
			ids = ids[:count]
		}
	}
	idsJSON, _ := json.Marshal(ids)
	if prog.ID == 0 {
		prog = model.PracticeProgress{
			StudentID:    studentID,
			PracticeMode: mode,
			QuestionIDs:  model.JSONB(idsJSON),
			CurrentIndex: 0,
			Total:        len(ids),
			AnswersState: model.JSONB("{}"),
			UpdatedAt:    beijingNow(),
		}
		if err := s.db.Create(&prog).Error; err != nil {
			return nil, err
		}
	} else {
		updates := map[string]any{"question_ids": model.JSONB(idsJSON), "updated_at": beijingNow()}
		if startIdx == 0 {
			updates["current_index"] = 0
		}
		updates["total"] = len(ids)
		if err := s.db.Model(&prog).Updates(updates).Error; err != nil {
			return nil, err
		}
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

// StartSequential 顺序练习：加载全部 published 题目（按 id 升序），
// 复用已有 practice_progress 游标续练；一次性返回全部题目，前端从游标处开始作答。
func (s *PracticeModeService) StartSequential(studentID int, credentialID ...*int) (*PracticeStartResultDTO, error) {
	var questions []model.Question
	q := s.db.Where("status = ?", "published")
	if len(credentialID) > 0 && credentialID[0] != nil {
		q = q.Where("credential_id = ?", *credentialID[0])
	}
	if err := q.Order("id ASC").Find(&questions).Error; err != nil {
		return nil, errors.New("查询题目失败")
	}
	if len(questions) == 0 {
		return nil, errors.New("题库暂无题目")
	}
	ids := make([]int, len(questions))
	for i, q := range questions {
		ids[i] = q.ID
	}
	idsJSON, _ := json.Marshal(ids)

	// upsert 进度：使用 Limit(1).Find() 避免首次进入练习时 GORM logger 误报 record not found
	var prog model.PracticeProgress
	err := s.db.Where("student_id = ? AND practice_mode = ?", studentID, "sequential").Limit(1).Find(&prog).Error
	if err != nil {
		return nil, err
	}
	if prog.ID == 0 {
		prog = model.PracticeProgress{
			StudentID:    studentID,
			PracticeMode: "sequential",
			QuestionIDs:  model.JSONB(idsJSON),
			CurrentIndex: 0,
			Total:        len(ids),
			AnswersState: model.JSONB("{}"),
			UpdatedAt:    beijingNow(),
		}
		if err := s.db.Create(&prog).Error; err != nil {
			return nil, err
		}
	} else {
		// 题库变化时刷新列表，但保留游标（不超过新总数）
		prog.QuestionIDs = model.JSONB(idsJSON)
		prog.Total = len(ids)
		if prog.CurrentIndex >= prog.Total {
			prog.CurrentIndex = 0
		}
		prog.UpdatedAt = beijingNow()
		s.db.Save(&prog)
	}

	// 一次性返回全部题目，前端从游标处开始作答
	all := make([]QuestionDTO, 0, len(questions))
	for i := range questions {
		all = append(all, newQuestionDTO(&questions[i], false))
	}
	return &PracticeStartResultDTO{
		Questions:    all,
		CurrentIndex: prog.CurrentIndex,
		Total:        prog.Total,
		Completed:    prog.CurrentIndex,
	}, nil
}

// SaveProgress 保存练习游标和答题状态。upsert 语义：记录不存在则创建。
// practiceMode 为空时默认 "sequential"；total > 0 时同步更新 total；
// answersState 经三态初始化（nil/空/显式 null → {}，#142）。
// 守卫口径对齐（session_progress.go）：practice_progress 无 status 字段（schema 冻结
// ADR-0010），经 (student_id, practice_mode) 定位即天然归属本人，且无终端状态，
// 恒视为在途——故无需在途校验；JSONB 归一复用共享实现 initAnswersState。
func (s *PracticeModeService) SaveProgress(studentID, index int, practiceMode string, total int, answersState json.RawMessage) error {
	if practiceMode == "" {
		practiceMode = "sequential"
	}
	answersState = initAnswersState(answersState)
	var prog model.PracticeProgress
	err := s.db.Where("student_id = ? AND practice_mode = ?", studentID, practiceMode).Limit(1).Find(&prog).Error
	if err != nil {
		return err
	}
	if prog.ID == 0 {
		prog = model.PracticeProgress{
			StudentID:    studentID,
			PracticeMode: practiceMode,
			QuestionIDs:  model.JSONB([]byte("[]")),
			CurrentIndex: index,
			Total:        total,
			AnswersState: model.JSONB(answersState),
			UpdatedAt:    beijingNow(),
		}
		if err := s.db.Create(&prog).Error; err != nil {
			return err
		}
	} else {
		updates := map[string]any{
			"current_index": index,
			"answers_state": model.JSONB(answersState),
			"updated_at":    beijingNow(),
		}
		if total > 0 {
			updates["total"] = total
		}
		if err := s.db.Model(&prog).Updates(updates).Error; err != nil {
			return err
		}
	}
	return nil
}

// GetProgress 查询任意模式的练习进度（卡片展示/断点续练用）。
// 使用 Limit(1).Find() 替代 First()，避免首次进入时 GORM logger 误报 record not found
func (s *PracticeModeService) GetProgress(studentID int, practiceMode string) *ProgressResultDTO {
	if practiceMode == "" {
		practiceMode = "sequential"
	}
	var prog model.PracticeProgress
	if err := s.db.Where("student_id = ? AND practice_mode = ?", studentID, practiceMode).Limit(1).Find(&prog).Error; err != nil {
		return emptyProgressResult()
	}
	if prog.ID == 0 {
		return emptyProgressResult()
	}
	// 解析 answers_state JSONB 为 map
	var stateMap map[string]any
	if len(prog.AnswersState) > 0 {
		_ = json.Unmarshal(prog.AnswersState, &stateMap)
	}
	if stateMap == nil {
		stateMap = map[string]any{}
	}
	completed := 0
	for _, v := range stateMap {
		if v == nil {
			continue
		}
		switch val := v.(type) {
		case string:
			if val != "" {
				completed++
			}
		case []any:
			if len(val) > 0 {
				completed++
			}
		case []string:
			if len(val) > 0 {
				completed++
			}
		default:
			completed++
		}
	}
	return &ProgressResultDTO{
		Completed:    completed,
		Total:        prog.Total,
		CurrentIndex: prog.CurrentIndex,
		AnswersState: stateMap,
	}
}

// emptyProgressResult 无进度记录时的空结果（与旧 map 输出的空态逐字一致）。
func emptyProgressResult() *ProgressResultDTO {
	return &ProgressResultDTO{Completed: 0, Total: 0, CurrentIndex: 0, AnswersState: map[string]any{}}
}

// GetSequentialProgress 查询顺序练习进度（卡片展示用，向后兼容）。
func (s *PracticeModeService) GetSequentialProgress(studentID int) *ProgressResultDTO {
	return s.GetProgress(studentID, "sequential")
}

// SubmitAnswer 提交答案并判定。判分经 grading_engine.gradeOne 单题入口（错题入库/分值表经 flow 注入）。
func (s *PracticeModeService) SubmitAnswer(studentID, questionID int, userAnswer any, practiceType string) (*SubmitResultDTO, error) {
	var q model.Question
	if err := s.db.First(&q, questionID).Error; err != nil {
		return nil, errors.New("题目不存在")
	}

	engine := newGradingEngine(s.db)
	flow := gradingFlow{
		ai:       s.grader,
		maxScore: practiceMaxScore,
	}
	gr := engine.gradeOne(flow, &q, userAnswer, studentID)

	rec := model.QuestionPracticeRecord{
		StudentID:    studentID,
		QuestionID:   questionID,
		IsCorrect:    gr.IsCorrect != nil && *gr.IsCorrect,
		PracticeType: orDefault(practiceType, "free"),
		UserAnswer:   stringifyAnswer(userAnswer),
		CreatedAt:    beijingNow(),
	}
	if err := s.db.Create(&rec).Error; err != nil {
		return nil, err
	}

	result := &SubmitResultDTO{
		IsCorrect:     gr.IsCorrect,
		CorrectAnswer: q.Answer,
		Explanation:   q.Explanation,
		QuestionID:    questionID,
		UserAnswer:    userAnswer,
	}
	finalizeSubmitResult(s.db, s.explainer, result, gr, &rec, &q)
	return result, nil
}

// finalizeSubmitResult 练习提交/错题重做共享的结果装配尾段（spec #295/#300 装配单点）：
// 全站统计回填 → AI 解析（QuestionExplanation module 单点）→ 简答分支
// （AI 及格覆写 IsCorrect 并二次 Save 同步练习记录，降级时 AIFallback 同写）。
func finalizeSubmitResult(db *gorm.DB, explainer *QuestionExplanation, result *SubmitResultDTO, gr GradeResult, rec *model.QuestionPracticeRecord, q *model.Question) {
	if stats := questionStats(db, q.ID, q.Type); stats != nil {
		result.AccuracyRate = stats.accuracyRate
		result.CommonWrong = stats.commonWrong
		result.TotalAttempts = stats.total
	}
	result.AIExplanation = explainer.GetOrGenerate(q)
	if q.Type == "short_answer" {
		result.ReferenceAnswer = q.ReferenceAnswer
		result.ScoringCriteria = q.ScoringCriteria
		result.MaxScore = int(gr.MaxScore)
		if sg := gr.ShortAnswer; sg != nil {
			result.AIScore = &sg.Score
			result.AIComment = sg.Comment
			if sg.Fallback {
				result.AIFallback = boolPtr(true)
			} else {
				// 练习流保留 IsCorrect 重写语义：AI 及格即覆盖并同步练习记录。
				result.IsCorrect = boolPtr(sg.Passed)
				rec.IsCorrect = sg.Passed
				db.Save(rec)
			}
		}
	}
}

// practiceMaxScore 练习流单题满分解析：客观题按练习分值表（原定级表，已正名 practice），简答题按题目自定义分（默认 10）。
// 与 SubmitAnswer 既有语义一致（客观走 practice 分值表、简答走 q.Score）。
func practiceMaxScore(q *model.Question) float64 {
	if q.Type == "short_answer" {
		if q.Score > 0 {
			return float64(q.Score)
		}
		return 10
	}
	return questionMaxScore("practice", q.Type)
}

// GetPracticeStats 刷题聚合统计（Ticket #329，独立于 /stats）：
// today_count 按 Asia/Shanghai 自然日 00:00~次日 00:00 区间过滤，
// total_count 为全量（含重做，question_practice_record 事实源），
// total_days 为 distinct 自然日去重（Go 侧按 Asia/Shanghai day string 去重，兼容 postgres/sqlite 双驱动，
// 语义等价于 COUNT(DISTINCT DATE(created_at AT TIME ZONE 'Asia/Shanghai'))）。
// 均按 student_id 过滤，credentialID 非空时 JOIN question 按 credential_id 分区（复用 sampleQuestions 的 JOIN 模式）。
// 索引说明：现有 idx_qpr_student(student_id) + idx_qpr_created(created_at) 已覆盖范围扫描；
// JOIN 分区路径依赖 question.credential_id 索引（question 表相关索引）与 question_practice_record.question_id；
// 高并发可追加复合索引 (student_id, created_at) 或 (student_id, question_id, created_at)，本期仅注释说明，无新增 migration。
func (s *PracticeModeService) GetPracticeStats(studentID int, credentialID *int) (*PracticePracticeStatsDTO, error) {
	clk := s.clk
	if clk == nil {
		clk = clock.Real()
	}
	loc := clock.Location()
	nowInLoc := clk.Now().In(loc)
	todayStart := time.Date(nowInLoc.Year(), nowInLoc.Month(), nowInLoc.Day(), 0, 0, 0, 0, loc)
	tomorrow := todayStart.AddDate(0, 0, 1)

	base := func() *gorm.DB {
		q := s.db.Model(&model.QuestionPracticeRecord{}).Where("question_practice_record.student_id = ?", studentID)
		if credentialID != nil {
			q = q.Joins("JOIN question ON question.id = question_practice_record.question_id").Where("question.credential_id = ?", *credentialID)
		}
		return q
	}

	var totalCount int64
	if err := base().Count(&totalCount).Error; err != nil {
		return nil, err
	}

	var todayCount int64
	if err := base().Where("question_practice_record.created_at >= ? AND question_practice_record.created_at < ?", todayStart, tomorrow).Count(&todayCount).Error; err != nil {
		return nil, err
	}

	var timestamps []time.Time
	pluckQ := s.db.Model(&model.QuestionPracticeRecord{}).Where("question_practice_record.student_id = ?", studentID)
	if credentialID != nil {
		pluckQ = pluckQ.Joins("JOIN question ON question.id = question_practice_record.question_id").Where("question.credential_id = ?", *credentialID)
	}
	if err := pluckQ.Pluck("question_practice_record.created_at", &timestamps).Error; err != nil {
		return nil, err
	}
	daySet := make(map[string]struct{}, len(timestamps))
	for _, t := range timestamps {
		daySet[t.In(loc).Format("2006-01-02")] = struct{}{}
	}

	return &PracticePracticeStatsDTO{
		TodayCount: todayCount,
		TotalCount: totalCount,
		TotalDays:  int64(len(daySet)),
	}, nil
}

// GetStats 学员练习统计（经统计聚合 module，一次 GROUP BY 按题型聚合；by_type 正确率为加性新增 key）。
func (s *PracticeModeService) GetStats(studentID int) *PracticeStatsDTO {
	var total, correct int64
	s.db.Model(&model.QuestionPracticeRecord{}).Where("student_id = ?", studentID).Count(&total)
	s.db.Model(&model.QuestionPracticeRecord{}).Where("student_id = ? AND is_correct = ?", studentID, true).Count(&correct)
	wrong := total - correct
	accuracy := 0.0
	if total > 0 {
		accuracy = roundFloat1(float64(correct) / float64(total) * 100)
	}
	base := s.db.Model(&model.QuestionPracticeRecord{}).
		Joins("JOIN question ON question.id = question_practice_record.question_id").
		Where("question_practice_record.student_id = ?", studentID)
	all, filtered := groupByCountWithFilter(base, "question.type", "CASE WHEN question_practice_record.is_correct THEN 1 ELSE 0 END")
	// 保留旧语义：by_type 对合法题型零填充；accuracy 为每题型正确率（加性新 key）。
	byType := make(map[string]PracticeTypeStat, len(validQuestionTypes))
	for _, t := range validQuestionTypes {
		tt := all[t]
		tc := filtered[t]
		acc := 0.0
		if tt > 0 {
			acc = roundFloat1(float64(tc) / float64(tt) * 100)
		}
		byType[t] = PracticeTypeStat{Total: tt, Correct: tc, Accuracy: acc}
	}
	return &PracticeStatsDTO{
		Total:    total,
		Correct:  correct,
		Wrong:    wrong,
		Accuracy: accuracy,
		ByType:   byType,
	}
}

type questionStatResult struct {
	total        int
	accuracyRate *float64
	commonWrong  *string
}

// questionStats 单题全站统计（练习/错题重做共享唯一实现）：总数、样本≥5 时的正确率
// 与易错项（易错项仅统计选择题，简答早退）。
func questionStats(db *gorm.DB, questionID int, qType string) *questionStatResult {
	var total int64
	db.Model(&model.QuestionPracticeRecord{}).Where("question_id = ?", questionID).Count(&total)
	res := &questionStatResult{total: int(total)}
	if total < 5 {
		return res
	}
	var correct int64
	db.Model(&model.QuestionPracticeRecord{}).Where("question_id = ? AND is_correct = ?", questionID, true).Count(&correct)
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
	err := db.Model(&model.QuestionPracticeRecord{}).
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

// GetHistory 练习历史分页。
func (s *PracticeModeService) GetHistory(studentID, page, pageSize int, qType, startDate, endDate string) *HistoryResultDTO {
	records, total, page, pageSize := paging.Query[model.QuestionPracticeRecord](s.db, page, pageSize, 20, "created_at DESC", func(q *gorm.DB) *gorm.DB {
		q = q.Where("student_id = ?", studentID)
		if qType != "" {
			q = q.Joins("JOIN question ON question.id = question_practice_record.question_id").Where("question.type = ?", qType)
		}
		if startDate != "" {
			q = q.Where("created_at >= ?", startDate)
		}
		if endDate != "" {
			q = q.Where("created_at <= ?", endDate)
		}
		return q
	})
	questionIDs := make([]int, 0, len(records))
	for i := range records {
		questionIDs = append(questionIDs, records[i].QuestionID)
	}
	questions := loadQuestionsByIDs(s.db, questionIDs)

	items := make([]HistoryItemDTO, 0, len(records))
	for _, rc := range records {
		item := HistoryItemDTO{
			ID:           rc.ID,
			StudentID:    rc.StudentID,
			QuestionID:   rc.QuestionID,
			IsCorrect:    rc.IsCorrect,
			PracticeType: rc.PracticeType,
			UserAnswer:   rc.UserAnswer,
			CreatedAt:    formatISO(rc.CreatedAt),
		}
		if qq, ok := questions[rc.QuestionID]; ok {
			d := newQuestionDTO(qq, false)
			item.Question = &d
		}
		items = append(items, item)
	}
	return &HistoryResultDTO{
		Total:    total,
		Page:     page,
		PageSize: pageSize,
		Records:  items,
	}
}

// sameIDSet 判断两个 ID 列表是否为同一集合（忽略顺序）。
func sameIDSet(a, b []int) bool {
	if len(a) != len(b) {
		return false
	}
	set := make(map[int]bool, len(a))
	for _, v := range a {
		set[v] = true
	}
	for _, v := range b {
		if !set[v] {
			return false
		}
	}
	return true
}
