// Package service 题库练习模式，对应 Python practice_mode_service。
package service

import (
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

var practiceLevelQuestionCount = map[string]int{"beginner": 10, "intermediate": 20, "advanced": 30}

// GetFreeQuestions 自由练习抽题，对应 Python get_free_practice_questions。
func (s *PracticeModeService) GetFreeQuestions(studentID int, qType string, kpID *int) ([]map[string]interface{}, error) {
	var student model.Student
	if err := s.db.First(&student, studentID).Error; err != nil {
		return nil, errors.New("学员不存在")
	}
	studentLevel := student.Level
	allowed, ok := examAllowedLevels[studentLevel]
	if !ok {
		studentLevel = "beginner"
		allowed = examAllowedLevels["beginner"]
	}
	count := practiceLevelQuestionCount[studentLevel]

	q := s.db.Model(&model.Question{}).Where("status = ? AND level IN ?", "published", allowed)
	if qType != "" {
		q = q.Where("type = ?", qType)
	}
	if kpID != nil {
		q = q.Where("knowledge_point_id = ?", *kpID)
	}
	var questions []model.Question
	if err := q.Find(&questions).Error; err != nil {
		return nil, errors.New("查询题目失败")
	}
	if len(questions) == 0 {
		return nil, errors.New("没有符合条件的题目")
	}
	if count > len(questions) {
		count = len(questions)
	}
	rand.Shuffle(len(questions), func(i, j int) { questions[i], questions[j] = questions[j], questions[i] })
	selected := questions[:count]
	out := make([]map[string]interface{}, 0, len(selected))
	for i := range selected {
		out = append(out, questionToDict(&selected[i], false))
	}
	return out, nil
}

// GetKnowledgePointPractice 知识点练习，对应 Python get_knowledge_point_practice。
func (s *PracticeModeService) GetKnowledgePointPractice(studentID, kpID int, count int, randomOrder bool) (map[string]interface{}, error) {
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
	list := make([]map[string]interface{}, 0, len(selected))
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
	return map[string]interface{}{
		"knowledge_point": kpToDict(&kp),
		"questions":       list,
		"total":           len(questions),
	}, nil
}

// GetKnowledgePointProgress 知识点进度，对应 Python get_knowledge_point_progress。
func (s *PracticeModeService) GetKnowledgePointProgress(studentID int, kpID *int) ([]map[string]interface{}, error) {
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
	result := make([]map[string]interface{}, 0, len(kps))
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
		childrenProg := make([]map[string]interface{}, 0, len(children))
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
			childrenProg = append(childrenProg, map[string]interface{}{
				"id":             child.ID,
				"name":           child.Name,
				"total_questions": childTotal,
				"answered":       childAnsCount,
				"correct":        childCorrect,
				"accuracy":       childAcc,
			})
		}
		result = append(result, map[string]interface{}{
			"id":              kp.ID,
			"name":            kp.Name,
			"level":           kp.Level,
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

// SubmitAnswer 提交答案并判定，对应 Python submit_practice_answer。
func (s *PracticeModeService) SubmitAnswer(studentID, questionID int, userAnswer interface{}, practiceType string) (map[string]interface{}, error) {
	var q model.Question
	if err := s.db.First(&q, questionID).Error; err != nil {
		return nil, errors.New("题目不存在")
	}
	isCorrect := checkAnswer(&q, userAnswer)
	userAnswerStr := stringifyAnswer(userAnswer)
	rec := model.QuestionPracticeRecord{
		StudentID:    studentID,
		QuestionID:   questionID,
		Level:        q.Level,
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

	result := map[string]interface{}{
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

// GetStats 学员练习统计，对应 Python get_practice_stats（practice_mode）。
func (s *PracticeModeService) GetStats(studentID int) map[string]interface{} {
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
	byLevel := map[string]map[string]int64{}
	for _, l := range validQuestionLevels {
		var lt, lc int64
		s.db.Model(&model.QuestionPracticeRecord{}).Where("student_id = ? AND level = ?", studentID, l).Count(&lt)
		s.db.Model(&model.QuestionPracticeRecord{}).Where("student_id = ? AND level = ? AND is_correct = ?", studentID, l, true).Count(&lc)
		byLevel[l] = map[string]int64{"total": lt, "correct": lc}
	}
	return map[string]interface{}{
		"total":                 total,
		"correct":               correct,
		"wrong":                 wrong,
		"accuracy":              accuracy,
		"by_type":               byType,
		"by_level":              byLevel,
		"weak_knowledge_points": s.getWeakKnowledgePoints(studentID),
	}
}

func (s *PracticeModeService) getWeakKnowledgePoints(studentID int) []map[string]interface{} {
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
	weak := []map[string]interface{}{}
	for _, r := range rows {
		acc := 0.0
		if r.Total > 0 {
			acc = roundFloat1(float64(r.Correct) / float64(r.Total) * 100)
		}
		if acc < 70 {
			weak = append(weak, map[string]interface{}{
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

// GetHistory 练习历史分页，对应 Python get_practice_history。
func (s *PracticeModeService) GetHistory(studentID, page, pageSize int, qType, startDate, endDate string) map[string]interface{} {
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
	items := make([]map[string]interface{}, 0, len(records))
	for _, r := range records {
		item := map[string]interface{}{
			"id":             r.ID,
			"student_id":     r.StudentID,
			"question_id":    r.QuestionID,
			"level":          r.Level,
			"is_correct":     r.IsCorrect,
			"practice_type":  r.PracticeType,
			"user_answer":    r.UserAnswer,
			"created_at":     formatISO(r.CreatedAt),
		}
		var qq model.Question
		if err := s.db.First(&qq, r.QuestionID).Error; err == nil {
			item["question"] = questionToDict(&qq, false)
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
