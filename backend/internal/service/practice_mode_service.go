// Package service 题库练习模式。
package service

import (
	"encoding/json"
	"errors"
	"fmt"
	"math/rand"

	"go.uber.org/zap"
	"gorm.io/gorm"

	"forklift-training/internal/model"
	"forklift-training/pkg/paging"
)

// PracticeModeService 题库练习模式服务。
type PracticeModeService struct {
	db *gorm.DB
	ai *AIService

	logger *zap.Logger
}

// NewPracticeModeService 创建题库练习服务，ai 可为 nil（简答题降级）。
func NewPracticeModeService(db *gorm.DB, ai *AIService, logger *zap.Logger) *PracticeModeService {
	return &PracticeModeService{db: db, ai: ai, logger: logger}
}

// GetFreeQuestions 随机练习抽题：从 published 题库按条件随机抽取 count 题。
// count <= 0 时返回全部符合条件的题目（按 id 升序，不打乱）。
func (s *PracticeModeService) GetFreeQuestions(qType string, count int) ([]QuestionDTO, error) {
	selected, err := sampleQuestions(s.db, qType, count)
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
func (s *PracticeModeService) StartTagPractice(studentID, tagID, count int) (map[string]any, error) {
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
	if err := s.db.Model(&model.Question{}).
		Where("id IN (SELECT question_id FROM question_tag_relation WHERE tag_id = ?) AND status = ?", tagID, "published").
		Order("id ASC").
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
	return map[string]any{
		"questions":     out,
		"current_index": startIdx,
		"total":         len(ids),
		"completed":     startIdx,
	}, nil
}

// StartSequential 顺序练习：加载全部 published 题目（按 id 升序），
// 复用已有 practice_progress 游标续练；一次性返回全部题目，前端从游标处开始作答。
func (s *PracticeModeService) StartSequential(studentID int) (map[string]any, error) {
	var questions []model.Question
	if err := s.db.Where("status = ?", "published").Order("id ASC").Find(&questions).Error; err != nil {
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
	return map[string]any{
		"questions":     all,
		"current_index": prog.CurrentIndex,
		"total":         prog.Total,
		"completed":     prog.CurrentIndex,
	}, nil
}

// SaveProgress 保存练习游标和答题状态。upsert 语义：记录不存在则创建。
// practiceMode 为空时默认 "sequential"；total > 0 时同步更新 total；
// answersState 经三态初始化（nil/空/显式 null → {}，#142）。
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
func (s *PracticeModeService) GetProgress(studentID int, practiceMode string) map[string]any {
	if practiceMode == "" {
		practiceMode = "sequential"
	}
	var prog model.PracticeProgress
	if err := s.db.Where("student_id = ? AND practice_mode = ?", studentID, practiceMode).Limit(1).Find(&prog).Error; err != nil {
		return map[string]any{"completed": 0, "total": 0, "current_index": 0, "answers_state": map[string]any{}}
	}
	if prog.ID == 0 {
		return map[string]any{"completed": 0, "total": 0, "current_index": 0, "answers_state": map[string]any{}}
	}
	// 解析 answers_state JSONB 为 map
	var stateMap map[string]any
	if len(prog.AnswersState) > 0 {
		_ = json.Unmarshal(prog.AnswersState, &stateMap)
	}
	if stateMap == nil {
		stateMap = map[string]any{}
	}
	return map[string]any{
		"completed":     prog.CurrentIndex,
		"total":         prog.Total,
		"current_index": prog.CurrentIndex,
		"answers_state": stateMap,
	}
}

// GetSequentialProgress 查询顺序练习进度（卡片展示用，向后兼容）。
func (s *PracticeModeService) GetSequentialProgress(studentID int) map[string]any {
	return s.GetProgress(studentID, "sequential")
}

// SubmitAnswer 提交答案并判定。
func (s *PracticeModeService) SubmitAnswer(studentID, questionID int, userAnswer any, practiceType string) (map[string]any, error) {
	var q model.Question
	if err := s.db.First(&q, questionID).Error; err != nil {
		return nil, errors.New("题目不存在")
	}
	isCorrect, _ := gradeQuestion(&q, userAnswer, 0)
	userAnswerStr := stringifyAnswer(userAnswer)
	rec := model.QuestionPracticeRecord{
		StudentID:    studentID,
		QuestionID:   questionID,
		IsCorrect:    isCorrect != nil && *isCorrect,
		PracticeType: orDefault(practiceType, "free"),
		UserAnswer:   userAnswerStr,
		CreatedAt:    beijingNow(),
	}
	if err := s.db.Create(&rec).Error; err != nil {
		return nil, err
	}
	// 错题入库
	if isCorrect != nil && !*isCorrect {
		_ = addToWrongQuestions(s.db, studentID, questionID)
	}

	result := map[string]any{
		"is_correct":     isCorrect,
		"correct_answer": q.Answer,
		"explanation":    q.Explanation,
		"question_id":    questionID,
		"user_answer":    userAnswer,
	}
	if q.Type == "short_answer" {
		result["reference_answer"] = q.ReferenceAnswer
		result["scoring_criteria"] = q.ScoringCriteria
		maxScore := q.Score
		if maxScore <= 0 {
			maxScore = 10
		}
		result["max_score"] = maxScore
		if aiRes := aiGradeShortAnswer(s.ai, q.Content, q.ReferenceAnswer, q.ScoringCriteria, userAnswerStr, float64(maxScore), nil); aiRes != nil {
			result["ai_score"] = aiRes.Score
			result["ai_comment"] = aiRes.Comment
			if aiRes.Fallback {
				result["ai_fallback"] = true
			} else {
				passed := shortAnswerPassed(aiRes.Score, float64(maxScore))
				result["is_correct"] = passed
				rec.IsCorrect = passed
				s.db.Save(&rec)
			}
		}
	}
	return result, nil
}

// GetStats 学员练习统计。
func (s *PracticeModeService) GetStats(studentID int) map[string]any {
	var total, correct int64
	s.db.Model(&model.QuestionPracticeRecord{}).Where("student_id = ?", studentID).Count(&total)
	s.db.Model(&model.QuestionPracticeRecord{}).Where("student_id = ? AND is_correct = ?", studentID, true).Count(&correct)
	wrong := total - correct
	accuracy := 0.0
	if total > 0 {
		accuracy = roundFloat1(float64(correct) / float64(total) * 100)
	}
	byType := map[string]map[string]int64{}
	for _, t := range validQuestionTypes {
		var tt, tc int64
		s.db.Joins("JOIN question ON question.id = question_practice_record.question_id").
			Where("question_practice_record.student_id = ? AND question.type = ?", studentID, t).
			Count(&tt)
		s.db.Joins("JOIN question ON question.id = question_practice_record.question_id").
			Where("question_practice_record.student_id = ? AND is_correct = ? AND question.type = ?", studentID, true, t).
			Count(&tc)
		acc := 0.0
		if tt > 0 {
			acc = roundFloat1(float64(tc) / float64(tt) * 100)
		}
		byType[t] = map[string]int64{"total": tt, "correct": tc}
		_ = acc
	}
	return map[string]any{
		"total":    total,
		"correct":  correct,
		"wrong":    wrong,
		"accuracy": accuracy,
		"by_type":  byType,
	}
}

// GetHistory 练习历史分页。
func (s *PracticeModeService) GetHistory(studentID, page, pageSize int, qType, startDate, endDate string) map[string]any {
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

	items := make([]map[string]any, 0, len(records))
	for _, r := range records {
		item := map[string]any{
			"id":            r.ID,
			"student_id":    r.StudentID,
			"question_id":   r.QuestionID,
			"is_correct":    r.IsCorrect,
			"practice_type": r.PracticeType,
			"user_answer":   r.UserAnswer,
			"created_at":    formatISO(r.CreatedAt),
		}
		if qq, ok := questions[r.QuestionID]; ok {
			item["question"] = newQuestionDTO(qq, false)
		}
		items = append(items, item)
	}
	return map[string]any{
		"total":     total,
		"page":      page,
		"page_size": pageSize,
		"records":   items,
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
