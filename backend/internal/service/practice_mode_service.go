// Package service 题库练习模式。
package service

import (
	"encoding/json"
	"errors"
	"math/rand"

	"gorm.io/gorm"

	"forklift-training/internal/model"
)

// PracticeModeService 题库练习模式服务。
type PracticeModeService struct {
	db *gorm.DB
	ai *AIService
}

// NewPracticeModeService 创建题库练习服务，ai 可为 nil（简答题降级）。
func NewPracticeModeService(db *gorm.DB, ai *AIService) *PracticeModeService {
	return &PracticeModeService{db: db, ai: ai}
}

// GetFreeQuestions 随机练习抽题：从 published 题库按条件随机抽取 count 题。
// count <= 0 时返回全部符合条件的题目（按 id 升序，不打乱）。
func (s *PracticeModeService) GetFreeQuestions(qType string, kpID *int, count int) ([]map[string]any, error) {
	var kpIDs []int
	if kpID != nil {
		kpIDs = []int{*kpID}
	}
	selected, err := sampleQuestions(s.db, qType, kpIDs, count)
	if err != nil {
		return nil, errors.New("查询题目失败")
	}
	if len(selected) == 0 {
		return nil, errors.New("没有符合条件的题目")
	}
	out := make([]map[string]any, 0, len(selected))
	for i := range selected {
		out = append(out, questionToDict(&selected[i], false))
	}
	return out, nil
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
	all := make([]map[string]any, 0, len(questions))
	for i := range questions {
		all = append(all, questionToDict(&questions[i], false))
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
// answersState 非空时更新答题状态 JSONB。
func (s *PracticeModeService) SaveProgress(studentID, index int, practiceMode string, total int, answersState json.RawMessage) error {
	if practiceMode == "" {
		practiceMode = "sequential"
	}
	// 默认空对象
	if len(answersState) == 0 {
		answersState = json.RawMessage("{}")
	}
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
func (s *PracticeModeService) GetProgress(studentID int, practiceMode string) map[string]any {
	if practiceMode == "" {
		practiceMode = "sequential"
	}
	var prog model.PracticeProgress
	if err := s.db.Where("student_id = ? AND practice_mode = ?", studentID, practiceMode).First(&prog).Error; err != nil {
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

// GetCategoryQuestions 章节练习：按课程分类（经 knowledge_point.category）抽题。
func (s *PracticeModeService) GetCategoryQuestions(category string, count int) ([]map[string]any, error) {
	if !containsString(validCategories, category) {
		return nil, errors.New("无效的课程分类")
	}
	// 查该分类下所有知识点 ID
	var kps []model.KnowledgePoint
	s.db.Where("category = ?", category).Find(&kps)
	if len(kps) == 0 {
		return nil, errors.New("该分类下暂无知识点")
	}
	kpIDs := make([]int, len(kps))
	for i, kp := range kps {
		kpIDs[i] = kp.ID
	}
	selected, err := sampleQuestions(s.db, "", kpIDs, count)
	if err != nil {
		return nil, errors.New("查询题目失败")
	}
	if len(selected) == 0 {
		return nil, errors.New("该分类下没有题目")
	}
	out := make([]map[string]any, 0, len(selected))
	for i := range selected {
		out = append(out, questionToDict(&selected[i], false))
	}
	return out, nil
}

// GetKnowledgePointPractice 知识点练习。
func (s *PracticeModeService) GetKnowledgePointPractice(studentID, kpID int, count int, randomOrder bool) (map[string]any, error) {
	var kp model.KnowledgePoint
	if err := s.db.First(&kp, kpID).Error; err != nil {
		return nil, errors.New("知识点不存在")
	}
	var children []model.KnowledgePoint
	s.db.Where("parent_id = ?", kpID).Find(&children)
	kpIDs := []int{kpID}
	for _, c := range children {
		kpIDs = append(kpIDs, c.ID)
	}
	var questions []model.Question
	s.db.Where("knowledge_point_id IN ? AND status = ?", kpIDs, "published").Find(&questions)
	if len(questions) == 0 {
		return nil, errors.New("该知识点下没有题目")
	}
	var selected []model.Question
	if count > 0 && count < len(questions) {
		perm := rand.Perm(len(questions))
		for i := 0; i < count; i++ {
			selected = append(selected, questions[perm[i]])
		}
	} else {
		selected = append(selected, questions...)
	}
	if randomOrder {
		rand.Shuffle(len(selected), func(i, j int) { selected[i], selected[j] = selected[j], selected[i] })
	}
	// 已答题目 ID 集合
	answeredIDs := map[int]bool{}
	if studentID != 0 {
		ids := make([]int, 0, len(selected))
		for _, q := range selected {
			ids = append(ids, q.ID)
		}
		var recs []model.QuestionPracticeRecord
		s.db.Where("student_id = ? AND question_id IN ?", studentID, ids).Find(&recs)
		for _, r := range recs {
			answeredIDs[r.QuestionID] = true
		}
	}
	// 子知识点名缓存
	kpCache := map[int]string{}
	list := make([]map[string]any, 0, len(selected))
	for i := range selected {
		q := &selected[i]
		d := questionToDict(q, false)
		d["answered"] = answeredIDs[q.ID]
		if q.KnowledgePointID != nil && *q.KnowledgePointID != kpID {
			if _, ok := kpCache[*q.KnowledgePointID]; !ok {
				var ck model.KnowledgePoint
				if err := s.db.First(&ck, *q.KnowledgePointID).Error; err == nil {
					kpCache[*q.KnowledgePointID] = ck.Name
				}
			}
			d["kp_name"] = kpCache[*q.KnowledgePointID]
		}
		list = append(list, d)
	}
	return map[string]any{
		"knowledge_point": kpToDict(&kp),
		"questions":       list,
		"total":           len(questions),
	}, nil
}

// GetKnowledgePointProgress 知识点进度。
func (s *PracticeModeService) GetKnowledgePointProgress(studentID int, kpID *int) ([]map[string]any, error) {
	var kps []model.KnowledgePoint
	if kpID != nil {
		var kp model.KnowledgePoint
		if err := s.db.First(&kp, *kpID).Error; err != nil {
			return nil, errors.New("知识点不存在")
		}
		kps = []model.KnowledgePoint{kp}
	} else {
		s.db.Where("parent_id IS NULL").Order("created_at ASC").Find(&kps)
	}
	result := make([]map[string]any, 0, len(kps))
	for _, kp := range kps {
		var children []model.KnowledgePoint
		s.db.Where("parent_id = ?", kp.ID).Order("created_at ASC").Find(&children)
		allIDs := []int{kp.ID}
		for _, c := range children {
			allIDs = append(allIDs, c.ID)
		}
		var totalQ int64
		s.db.Model(&model.Question{}).Where("knowledge_point_id IN ? AND status = ?", allIDs, "published").Count(&totalQ)

		var recs []model.QuestionPracticeRecord
		s.db.Joins("JOIN question ON question.id = question_practice_record.question_id").
			Where("question_practice_record.student_id = ? AND question.knowledge_point_id IN ?", studentID, allIDs).
			Find(&recs)
		answeredIDs := map[int]bool{}
		correctCount := 0
		for _, r := range recs {
			answeredIDs[r.QuestionID] = true
			if r.IsCorrect {
				correctCount++
			}
		}
		answeredCount := len(answeredIDs)
		accuracy := 0.0
		if answeredCount > 0 {
			accuracy = roundFloat1(float64(correctCount) / float64(answeredCount) * 100)
		}
		childrenProg := make([]map[string]any, 0, len(children))
		for _, child := range children {
			var childTotal int64
			s.db.Model(&model.Question{}).Where("knowledge_point_id = ? AND status = ?", child.ID, "published").Count(&childTotal)
			var childRecs []model.QuestionPracticeRecord
			s.db.Joins("JOIN question ON question.id = question_practice_record.question_id").
				Where("question_practice_record.student_id = ? AND question.knowledge_point_id = ?", studentID, child.ID).
				Find(&childRecs)
			childAnswered := map[int]bool{}
			childCorrect := 0
			for _, r := range childRecs {
				childAnswered[r.QuestionID] = true
				if r.IsCorrect {
					childCorrect++
				}
			}
			childAnsCount := len(childAnswered)
			childAcc := 0.0
			if childAnsCount > 0 {
				childAcc = roundFloat1(float64(childCorrect) / float64(childAnsCount) * 100)
			}
			childrenProg = append(childrenProg, map[string]any{
				"id":              child.ID,
				"name":            child.Name,
				"total_questions": childTotal,
				"answered":        childAnsCount,
				"correct":         childCorrect,
				"accuracy":        childAcc,
			})
		}
		result = append(result, map[string]any{
			"id":              kp.ID,
			"name":            kp.Name,
			"category":        kp.Category,
			"description":     kp.Description,
			"total_questions": totalQ,
			"answered":        answeredCount,
			"correct":         correctCount,
			"accuracy":        accuracy,
			"children":        childrenProg,
		})
	}
	return result, nil
}

// SubmitAnswer 提交答案并判定。
func (s *PracticeModeService) SubmitAnswer(studentID, questionID int, userAnswer any, practiceType string) (map[string]any, error) {
	var q model.Question
	if err := s.db.First(&q, questionID).Error; err != nil {
		return nil, errors.New("题目不存在")
	}
	isCorrect := checkAnswer(&q, userAnswer)
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
		if s.ai != nil {
			aiRes := s.ai.GradeShortAnswer(q.Content, q.ReferenceAnswer, q.ScoringCriteria, userAnswerStr, float64(maxScore), nil)
			if aiRes != nil {
				result["ai_score"] = aiRes.Score
				result["ai_comment"] = aiRes.Comment
				if aiRes.Fallback {
					result["ai_fallback"] = true
				} else {
					passed := aiRes.Score >= float64(maxScore)*0.6
					result["is_correct"] = passed
					rec.IsCorrect = passed
					s.db.Save(&rec)
				}
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
		"total":                 total,
		"correct":               correct,
		"wrong":                 wrong,
		"accuracy":              accuracy,
		"by_type":               byType,
		"weak_knowledge_points": s.getWeakKnowledgePoints(studentID),
	}
}

func (s *PracticeModeService) getWeakKnowledgePoints(studentID int) []map[string]any {
	type row struct {
		ID      int
		Name    string
		Total   int64
		Correct int64
	}
	var rows []row
	s.db.Table("knowledge_point").
		Select("knowledge_point.id AS id, knowledge_point.name AS name, COUNT(question_practice_record.id) AS total, SUM(CASE WHEN question_practice_record.is_correct = true THEN 1 ELSE 0 END) AS correct").
		Joins("JOIN question ON question.knowledge_point_id = knowledge_point.id").
		Joins("JOIN question_practice_record ON question_practice_record.question_id = question.id").
		Where("question_practice_record.student_id = ?", studentID).
		Group("knowledge_point.id").Scan(&rows)
	weak := []map[string]any{}
	for _, r := range rows {
		acc := 0.0
		if r.Total > 0 {
			acc = roundFloat1(float64(r.Correct) / float64(r.Total) * 100)
		}
		if acc < 70 {
			weak = append(weak, map[string]any{
				"id":       r.ID,
				"name":     r.Name,
				"total":    r.Total,
				"correct":  r.Correct,
				"accuracy": acc,
			})
		}
	}
	// 按 accuracy 升序，取前 5
	for i := 1; i < len(weak); i++ {
		for j := i; j > 0; j-- {
			a := weak[j]["accuracy"].(float64)
			b := weak[j-1]["accuracy"].(float64)
			if a < b {
				weak[j], weak[j-1] = weak[j-1], weak[j]
			}
		}
	}
	if len(weak) > 5 {
		weak = weak[:5]
	}
	return weak
}

// GetHistory 练习历史分页。
func (s *PracticeModeService) GetHistory(studentID, page, pageSize int, qType, startDate, endDate string) map[string]any {
	if page <= 0 {
		page = 1
	}
	if pageSize <= 0 {
		pageSize = 20
	}
	q := s.db.Model(&model.QuestionPracticeRecord{}).Where("student_id = ?", studentID)
	if qType != "" {
		q = q.Joins("JOIN question ON question.id = question_practice_record.question_id").Where("question.type = ?", qType)
	}
	if startDate != "" {
		q = q.Where("created_at >= ?", startDate)
	}
	if endDate != "" {
		q = q.Where("created_at <= ?", endDate)
	}
	var total int64
	q.Count(&total)
	var records []model.QuestionPracticeRecord
	q.Order("created_at DESC").Offset((page - 1) * pageSize).Limit(pageSize).Find(&records)
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
		var qq model.Question
		if err := s.db.First(&qq, r.QuestionID).Error; err == nil {
			item["question"] = questionToDict(&qq, false)
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
