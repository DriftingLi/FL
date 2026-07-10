// Package service 题目相关共享逻辑与题库 CRUD。
package service

import (
	"encoding/json"
	"errors"
	"sort"
	"strings"

	"gorm.io/gorm"

	"forklift-training/internal/model"
)

// 题型与等级常量，与 Python VALID_TYPES/VALID_LEVELS 一致。
var (
	validQuestionTypes  = []string{"single_choice", "multi_choice", "true_false", "fault_image", "short_answer"}
	validQuestionLevels = []string{"beginner", "intermediate", "advanced"}
	validQuestionStatus = []string{"draft", "pending", "published"}
	examScoreMap        = map[string]float64{"single_choice": 3, "multi_choice": 4, "true_false": 2, "fault_image": 6, "short_answer": 5}
	mockExamScoreMap    = map[string]float64{"single_choice": 3, "multi_choice": 4, "true_false": 2, "fault_image": 4, "short_answer": 10}
	levelOrder          = map[string]int{"beginner": 1, "intermediate": 2, "advanced": 3, "expert": 4}
	levelPromotion      = map[string]string{"beginner": "intermediate", "intermediate": "advanced", "advanced": "expert"}
	examAllowedLevels   = map[string][]string{
		"beginner":     {"beginner"},
		"intermediate": {"beginner", "intermediate"},
		"advanced":     {"beginner", "intermediate", "advanced"},
	}
)

// questionToDict 将 Question 转为 dict，对应 Python Question.to_dict。
// includeAnswer=false 时剔除答案、解析、参考答案、评分标准（学员侧）。
func questionToDict(q *model.Question, includeAnswer bool) map[string]interface{} {
	var options interface{}
	if len(q.Options) > 0 {
		_ = json.Unmarshal(q.Options, &options)
	}
	d := map[string]interface{}{
		"id":                 q.ID,
		"type":               q.Type,
		"level":              q.Level,
		"content":            q.Content,
		"options":            options,
		"image_url":          q.ImageURL,
		"knowledge_point_id": q.KnowledgePointID,
		"status":             q.Status,
		"score":              q.Score,
		"created_by":         q.CreatedBy,
		"created_by_type":    q.CreatedByType,
		"created_at":         formatISO(q.CreatedAt),
		"updated_at":         formatISO(q.UpdatedAt),
	}
	if includeAnswer {
		d["answer"] = q.Answer
		d["explanation"] = q.Explanation
		d["reference_answer"] = q.ReferenceAnswer
		d["scoring_criteria"] = q.ScoringCriteria
	}
	return d
}

// checkAnswer 判定答案对错，对应 Python _check_answer。
// 返回 *bool：nil 表示无法判定（简答题/未作答），true/false 表示对/错。
func checkAnswer(q *model.Question, userAnswer interface{}) *bool {
	if userAnswer == nil {
		return nil
	}
	if q.Type == "short_answer" {
		return nil
	}
	if q.Type == "multi_choice" {
		correct := normalizeAnswerList(q.Answer)
		user := normalizeUserAnswerList(userAnswer)
		if user == nil {
			b := false
			return &b
		}
		eq := stringSliceEqual(correct, user)
		return &eq
	}
	// 单选/判断/故障识图：字符串大写比较
	ua := stringifyAnswer(userAnswer)
	res := strings.ToUpper(strings.TrimSpace(ua)) == strings.ToUpper(strings.TrimSpace(q.Answer))
	return &res
}

// gradeQuestion 评分，对应 Python _grade_question。
// 返回 (isCorrect, earned)：isCorrect 为 nil 表示无法判定；earned 为得分。
func gradeQuestion(q *model.Question, userAnswer interface{}, maxScore float64) (*bool, float64) {
	if userAnswer == nil {
		return nil, 0
	}
	if maxScore == 0 {
		maxScore = examScoreMap[q.Type]
	}
	switch q.Type {
	case "single_choice", "true_false", "fault_image":
		ua := stringifyAnswer(userAnswer)
		correct := strings.ToUpper(strings.TrimSpace(ua)) == strings.ToUpper(strings.TrimSpace(q.Answer))
		if correct {
			return &correct, maxScore
		}
		return &correct, 0
	case "multi_choice":
		correct := normalizeAnswerList(q.Answer)
		user := normalizeUserAnswerList(userAnswer)
		if user == nil {
			b := false
			return &b, 0
		}
		if stringSliceEqual(correct, user) {
			t := true
			return &t, maxScore
		}
		if subset(user, correct) && len(user) > 0 {
			partial := maxScore * float64(len(user)) / float64(len(correct)) * 0.5
			round1(&partial)
			f := false
			return &f, partial
		}
		f := false
		return &f, 0
	case "short_answer":
		return nil, 0
	}
	b := false
	return &b, 0
}

// addToWrongQuestions 错题入库（去重、计数），对应 Python _add_to_wrong_question。
func addToWrongQuestions(db *gorm.DB, studentID, questionID int) error {
	var wq model.WrongQuestion
	err := db.Where("student_id = ? AND question_id = ?", studentID, questionID).First(&wq).Error
	if err == nil {
		wq.WrongCount++
		wq.LastWrongAt = beijingNow()
		wq.IsRemoved = false
		return db.Save(&wq).Error
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return err
	}
	wq = model.WrongQuestion{
		StudentID:   studentID,
		QuestionID:  questionID,
		WrongCount:  1,
		LastWrongAt: beijingNow(),
		CreatedAt:   beijingNow(),
	}
	return db.Create(&wq).Error
}

// stringifyAnswer 将用户答案（字符串/列表）转为字符串，对应 Python 中 ','.join(list) 或 str。
func stringifyAnswer(a interface{}) string {
	if a == nil {
		return ""
	}
	switch v := a.(type) {
	case string:
		return v
	case []interface{}:
		parts := make([]string, 0, len(v))
		for _, p := range v {
			parts = append(parts, toString(p))
		}
		return strings.Join(parts, ",")
	case []string:
		return strings.Join(v, ",")
	}
	return ""
}

// normalizeAnswerList 将 "A,B,C" 拆分并排序，对应 Python sorted([x.strip() for x in answer.split(',')])。
func normalizeAnswerList(s string) []string {
	parts := strings.Split(s, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		if t := strings.TrimSpace(p); t != "" {
			out = append(out, t)
		}
	}
	sort.Strings(out)
	return out
}

// normalizeUserAnswerList 将用户答案归一化为排序后的 []string。
func normalizeUserAnswerList(a interface{}) []string {
	switch v := a.(type) {
	case []interface{}:
		out := make([]string, 0, len(v))
		for _, p := range v {
			out = append(out, strings.TrimSpace(toString(p)))
		}
		sort.Strings(out)
		return out
	case []string:
		out := make([]string, 0, len(v))
		for _, p := range v {
			out = append(out, strings.TrimSpace(p))
		}
		sort.Strings(out)
		return out
	case string:
		return normalizeAnswerList(v)
	}
	return nil
}

func toString(v interface{}) string {
	switch x := v.(type) {
	case string:
		return x
	case float64:
		return floatToString(x)
	case int:
		return intToString(x)
	}
	b, _ := json.Marshal(v)
	return string(b)
}

func stringSliceEqual(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func subset(a, b []string) bool {
	set := make(map[string]bool, len(b))
	for _, x := range b {
		set[x] = true
	}
	for _, x := range a {
		if !set[x] {
			return false
		}
	}
	return true
}

func round1(f *float64) {
	*f = float64(int(*f*10+0.5)) / 10
}

// ===== 题库服务（question_bank_service） =====

// QuestionBankService 题库 CRUD 与知识点管理。
type QuestionBankService struct {
	db *gorm.DB
}

// NewQuestionBankService 创建题库服务。
func NewQuestionBankService(db *gorm.DB) *QuestionBankService {
	return &QuestionBankService{db: db}
}

// CreateQuestion 创建题目，对应 Python create_question。
func (s *QuestionBankService) CreateQuestion(data map[string]interface{}, createdBy *int, createdByType string) (map[string]interface{}, error) {
	qType, _ := data["type"].(string)
	if !containsString(validQuestionTypes, qType) {
		return nil, errors.New("无效的题型，支持的题型：" + strings.Join(validQuestionTypes, ", "))
	}
	qLevel, _ := data["level"].(string)
	if !containsString(validQuestionLevels, qLevel) {
		return nil, errors.New("无效的等级，支持的等级：" + strings.Join(validQuestionLevels, ", "))
	}
	content, _ := data["content"].(string)
	if content == "" {
		return nil, errors.New("题干不能为空")
	}
	answer := stringifyAnswer(data["answer"])
	if answer == "" && qType != "short_answer" {
		return nil, errors.New("答案不能为空")
	}
	options := data["options"]
	if qType == "single_choice" || qType == "multi_choice" || qType == "fault_image" {
		if options == nil {
			return nil, errors.New("选项不能为空")
		}
	}
	var kpID *int
	if v, ok := data["knowledge_point_id"]; ok && v != nil {
		id := toInt(v)
		var kp model.KnowledgePoint
		if err := s.db.First(&kp, id).Error; err != nil {
			return nil, errors.New("知识点不存在")
		}
		kpID = &id
	}
	status, _ := data["status"].(string)
	if status == "" {
		status = "pending"
	}
	var optionsBytes model.JSONB
	if options != nil {
		if b, err := json.Marshal(options); err == nil {
			optionsBytes = model.JSONB(b)
		}
	}
	q := model.Question{
		Type:             qType,
		Level:            qLevel,
		Content:          content,
		Options:          optionsBytes,
		Answer:           answer,
		Explanation:      getString(data, "explanation"),
		ImageURL:         getString(data, "image_url"),
		ReferenceAnswer:  getString(data, "reference_answer"),
		ScoringCriteria:  getString(data, "scoring_criteria"),
		Score:            toIntDefault(data["score"], 0),
		KnowledgePointID: kpID,
		Status:           status,
		CreatedBy:        createdBy,
		CreatedByType:    orDefault(createdByType, "tutor"),
		CreatedAt:        beijingNow(),
		UpdatedAt:        beijingNow(),
	}
	if err := s.db.Create(&q).Error; err != nil {
		return nil, err
	}
	return questionToDict(&q, true), nil
}

// GetQuestion 查询题目详情。
func (s *QuestionBankService) GetQuestion(id int) (map[string]interface{}, error) {
	var q model.Question
	if err := s.db.First(&q, id).Error; err != nil {
		return nil, errors.New("题目不存在")
	}
	return questionToDict(&q, true), nil
}

// UpdateQuestion 更新题目。
func (s *QuestionBankService) UpdateQuestion(id int, data map[string]interface{}) (map[string]interface{}, error) {
	var q model.Question
	if err := s.db.First(&q, id).Error; err != nil {
		return nil, errors.New("题目不存在")
	}
	if t, ok := data["type"].(string); ok && !containsString(validQuestionTypes, t) {
		return nil, errors.New("无效的题型")
	}
	if l, ok := data["level"].(string); ok && !containsString(validQuestionLevels, l) {
		return nil, errors.New("无效的等级")
	}
	if st, ok := data["status"].(string); ok && !containsString(validQuestionStatus, st) {
		return nil, errors.New("无效的状态")
	}
	applyQuestionFields(&q, data)
	q.UpdatedAt = beijingNow()
	if err := s.db.Save(&q).Error; err != nil {
		return nil, err
	}
	return questionToDict(&q, true), nil
}

// DeleteQuestion 删除题目。
func (s *QuestionBankService) DeleteQuestion(id int) error {
	result := s.db.Delete(&model.Question{}, id)
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return errors.New("题目不存在")
	}
	return nil
}

// ListQuestions 题目列表分页查询。
func (s *QuestionBankService) ListQuestions(page, pageSize int, level, qType string, kpID *int, status, keyword string) map[string]interface{} {
	if page <= 0 {
		page = 1
	}
	if pageSize <= 0 {
		pageSize = 20
	}
	q := s.db.Model(&model.Question{})
	if level != "" {
		q = q.Where("level = ?", level)
	}
	if qType != "" {
		q = q.Where("type = ?", qType)
	}
	if kpID != nil {
		q = q.Where("knowledge_point_id = ?", *kpID)
	}
	if status != "" {
		q = q.Where("status = ?", status)
	}
	if keyword != "" {
		q = q.Where("content LIKE ?", "%"+keyword+"%")
	}
	var total int64
	q.Count(&total)
	var list []model.Question
	q.Order("created_at DESC").Offset((page - 1) * pageSize).Limit(pageSize).Find(&list)
	out := make([]map[string]interface{}, 0, len(list))
	for i := range list {
		out = append(out, questionToDict(&list[i], true))
	}
	return map[string]interface{}{
		"total":     total,
		"page":      page,
		"page_size": pageSize,
		"questions": out,
	}
}

// PublishQuestion 发布题目。
func (s *QuestionBankService) PublishQuestion(id int) (map[string]interface{}, error) {
	var q model.Question
	if err := s.db.First(&q, id).Error; err != nil {
		return nil, errors.New("题目不存在")
	}
	q.Status = "published"
	q.UpdatedAt = beijingNow()
	if err := s.db.Save(&q).Error; err != nil {
		return nil, err
	}
	return questionToDict(&q, true), nil
}

// BatchPublish 批量发布。
func (s *QuestionBankService) BatchPublish(ids []int) map[string]interface{} {
	count := 0
	if len(ids) > 0 {
		count64 := s.db.Model(&model.Question{}).Where("id IN ?", ids).Update("status", "published").RowsAffected
		count = int(count64)
	}
	return map[string]interface{}{"published_count": count}
}

// BatchImport 批量导入题目。
func (s *QuestionBankService) BatchImport(items []interface{}, createdBy *int) map[string]interface{} {
	success, errs := 0, []map[string]interface{}{}
	for i, item := range items {
		data, ok := item.(map[string]interface{})
		if !ok {
			errs = append(errs, map[string]interface{}{"index": i, "error": "无效数据"})
			continue
		}
		data["status"] = "pending"
		if _, err := s.CreateQuestion(data, createdBy, "tutor"); err != nil {
			errs = append(errs, map[string]interface{}{"index": i, "error": err.Error()})
			continue
		}
		success++
	}
	return map[string]interface{}{
		"success_count": success,
		"error_count":   len(errs),
		"errors":        errs,
	}
}

// GetStats 题库统计。
func (s *QuestionBankService) GetStats() map[string]interface{} {
	var total int64
	s.db.Model(&model.Question{}).Count(&total)
	byLevel := map[string]int64{}
	for _, l := range validQuestionLevels {
		var c int64
		s.db.Model(&model.Question{}).Where("level = ?", l).Count(&c)
		byLevel[l] = c
	}
	byType := map[string]int64{}
	for _, t := range validQuestionTypes {
		var c int64
		s.db.Model(&model.Question{}).Where("type = ?", t).Count(&c)
		byType[t] = c
	}
	byStatus := map[string]int64{}
	for _, st := range validQuestionStatus {
		var c int64
		s.db.Model(&model.Question{}).Where("status = ?", st).Count(&c)
		byStatus[st] = c
	}
	var kps []model.KnowledgePoint
	s.db.Find(&kps)
	byKP := make([]map[string]interface{}, 0, len(kps))
	for _, kp := range kps {
		var c int64
		s.db.Model(&model.Question{}).Where("knowledge_point_id = ?", kp.ID).Count(&c)
		byKP = append(byKP, map[string]interface{}{"id": kp.ID, "name": kp.Name, "count": c})
	}
	return map[string]interface{}{
		"total":              total,
		"by_level":           byLevel,
		"by_type":            byType,
		"by_status":          byStatus,
		"by_knowledge_point": byKP,
	}
}

// CreateKnowledgePoint 创建知识点。
func (s *QuestionBankService) CreateKnowledgePoint(data map[string]interface{}) (map[string]interface{}, error) {
	name, _ := data["name"].(string)
	if name == "" {
		return nil, errors.New("知识点名称不能为空")
	}
	level, _ := data["level"].(string)
	if !containsString(validQuestionLevels, level) {
		return nil, errors.New("无效的等级")
	}
	var parentID *int
	if v, ok := data["parent_id"]; ok && v != nil {
		pid := toInt(v)
		var p model.KnowledgePoint
		if err := s.db.First(&p, pid).Error; err != nil {
			return nil, errors.New("父知识点不存在")
		}
		parentID = &pid
	}
	kp := model.KnowledgePoint{
		Name:        name,
		Level:       level,
		ParentID:    parentID,
		Description: getString(data, "description"),
		CreatedAt:   beijingNow(),
	}
	if err := s.db.Create(&kp).Error; err != nil {
		return nil, err
	}
	return kpToDict(&kp), nil
}

// GetKnowledgePoints 查询知识点列表。
func (s *QuestionBankService) GetKnowledgePoints(level string, parentID *int) []map[string]interface{} {
	q := s.db.Model(&model.KnowledgePoint{})
	if level != "" {
		q = q.Where("level = ?", level)
	}
	if parentID != nil {
		q = q.Where("parent_id = ?", *parentID)
	}
	var kps []model.KnowledgePoint
	q.Order("created_at ASC").Find(&kps)
	out := make([]map[string]interface{}, 0, len(kps))
	for i := range kps {
		out = append(out, kpToDict(&kps[i]))
	}
	return out
}

// UpdateKnowledgePoint 更新知识点。
func (s *QuestionBankService) UpdateKnowledgePoint(id int, data map[string]interface{}) (map[string]interface{}, error) {
	var kp model.KnowledgePoint
	if err := s.db.First(&kp, id).Error; err != nil {
		return nil, errors.New("知识点不存在")
	}
	if v, ok := data["name"]; ok {
		kp.Name, _ = v.(string)
	}
	if v, ok := data["level"]; ok {
		if l, _ := v.(string); !containsString(validQuestionLevels, l) {
			return nil, errors.New("无效的等级")
		}
		kp.Level, _ = v.(string)
	}
	if v, ok := data["parent_id"]; ok {
		if v == nil {
			kp.ParentID = nil
		} else {
			pid := toInt(v)
			kp.ParentID = &pid
		}
	}
	if v, ok := data["description"]; ok {
		kp.Description, _ = v.(string)
	}
	if err := s.db.Save(&kp).Error; err != nil {
		return nil, err
	}
	return kpToDict(&kp), nil
}

// DeleteKnowledgePoint 删除知识点（有题目时拒绝）。
func (s *QuestionBankService) DeleteKnowledgePoint(id int) error {
	var c int64
	s.db.Model(&model.Question{}).Where("knowledge_point_id = ?", id).Count(&c)
	if c > 0 {
		return errors.New("该知识点下还有题目，无法删除")
	}
	if err := s.db.Delete(&model.KnowledgePoint{}, id).Error; err != nil {
		return errors.New("知识点不存在")
	}
	return nil
}

func kpToDict(kp *model.KnowledgePoint) map[string]interface{} {
	return map[string]interface{}{
		"id":          kp.ID,
		"name":        kp.Name,
		"level":       kp.Level,
		"parent_id":   kp.ParentID,
		"description": kp.Description,
		"created_at":  formatISO(kp.CreatedAt),
	}
}

func applyQuestionFields(q *model.Question, data map[string]interface{}) {
	if v, ok := data["type"]; ok {
		q.Type, _ = v.(string)
	}
	if v, ok := data["level"]; ok {
		q.Level, _ = v.(string)
	}
	if v, ok := data["content"]; ok {
		q.Content, _ = v.(string)
	}
	if v, ok := data["options"]; ok {
		if v == nil {
			q.Options = nil
		} else if b, err := json.Marshal(v); err == nil {
			q.Options = model.JSONB(b)
		}
	}
	if v, ok := data["answer"]; ok {
		q.Answer = stringifyAnswer(v)
	}
	if v, ok := data["explanation"]; ok {
		q.Explanation, _ = v.(string)
	}
	if v, ok := data["image_url"]; ok {
		q.ImageURL, _ = v.(string)
	}
	if v, ok := data["reference_answer"]; ok {
		q.ReferenceAnswer, _ = v.(string)
	}
	if v, ok := data["scoring_criteria"]; ok {
		q.ScoringCriteria, _ = v.(string)
	}
	if v, ok := data["score"]; ok {
		q.Score = toIntDefault(v, q.Score)
	}
	if v, ok := data["knowledge_point_id"]; ok {
		if v == nil {
			q.KnowledgePointID = nil
		} else {
			id := toInt(v)
			q.KnowledgePointID = &id
		}
	}
	if v, ok := data["status"]; ok {
		q.Status, _ = v.(string)
	}
}

// toInt 将任意数值转为 int。
func toInt(v interface{}) int {
	switch n := v.(type) {
	case float64:
		return int(n)
	case int:
		return n
	case int64:
		return int(n)
	case string:
		return parseInt(n)
	}
	return 0
}

// toIntDefault 将任意数值转为 int，失败返回 def。
func toIntDefault(v interface{}, def int) int {
	if v == nil {
		return def
	}
	return toInt(v)
}

// getString 从 map 取字符串。
func getString(m map[string]interface{}, key string) string {
	v, _ := m[key].(string)
	return v
}

func intToString(i int) string       { return toStringHelper(i) }
func floatToString(f float64) string { return toStringHelper(f) }

func toStringHelper(v interface{}) string {
	b, _ := json.Marshal(v)
	return string(b)
}
