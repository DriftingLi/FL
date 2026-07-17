// Package service 题目相关共享逻辑与题库 CRUD。
package service

import (
	"encoding/json"
	"errors"
	"math/rand"
	"sort"
	"strings"

	"gorm.io/gorm"

	"forklift-training/internal/model"
)

// 题型与课程分类常量（已取消等级制度）。
var (
	validQuestionTypes  = []string{"single_choice", "multi_choice", "true_false", "fault_image", "short_answer"}
	validQuestionStatus = []string{"draft", "pending", "published"}
	validCategories     = []string{"CATEGORY_01", "CATEGORY_02", "CATEGORY_03", "CATEGORY_04"}
	examScoreMap        = map[string]float64{"single_choice": 3, "multi_choice": 4, "true_false": 2, "fault_image": 6, "short_answer": 5}
	mockExamScoreMap    = map[string]float64{"single_choice": 3, "multi_choice": 4, "true_false": 2, "fault_image": 4, "short_answer": 10}
)

// sampleQuestions 统一抽题函数：从 published 题库按条件随机抽取 count 题。
// qType 为空表示不限题型；kpIDs 为空表示不限知识点。
func sampleQuestions(db *gorm.DB, qType string, kpIDs []int, count int) ([]model.Question, error) {
	q := db.Model(&model.Question{}).Where("status = ?", "published")
	if qType != "" {
		q = q.Where("type = ?", qType)
	}
	if len(kpIDs) > 0 {
		q = q.Where("knowledge_point_id IN ?", kpIDs)
	}
	var all []model.Question
	if err := q.Find(&all).Error; err != nil {
		return nil, err
	}
	if count > 0 && len(all) > count {
		rand.Shuffle(len(all), func(i, j int) { all[i], all[j] = all[j], all[i] })
		all = all[:count]
	}
	return all, nil
}

// questionToDict 将 Question 转为 dict。
// includeAnswer=false 时剔除答案、解析、参考答案、评分标准（学员侧）。
func questionToDict(q *model.Question, includeAnswer bool) map[string]any {
	var options any
	if len(q.Options) > 0 {
		_ = json.Unmarshal(q.Options, &options)
	}
	d := map[string]any{
		"id":                 q.ID,
		"type":               q.Type,
		"content":            q.Content,
		"options":            options,
		"image_url":          q.ImageURL,
		"knowledge_point_id": q.KnowledgePointID,
		"status":             q.Status,
		"reject_reason":      q.RejectReason,
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

// checkAnswer 判定答案对错。
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
	//nolint:staticcheck
	res := strings.EqualFold(strings.TrimSpace(ua), strings.TrimSpace(q.Answer))
	return &res
}

// gradeQuestion 评分。
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
		//nolint:staticcheck
		correct := strings.EqualFold(strings.TrimSpace(ua), strings.TrimSpace(q.Answer))
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

// addToWrongQuestions 错题入库（去重、计数）。
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

// stringifyAnswer 将用户答案（字符串/列表）转为字符串。
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

// normalizeAnswerList 将 "A,B,C" 拆分并排序。
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

// CreateQuestion 创建题目。
func (s *QuestionBankService) CreateQuestion(data map[string]any, createdBy *int, createdByType string) (map[string]any, error) {
	qType, _ := data["type"].(string)
	if !containsString(validQuestionTypes, qType) {
		return nil, errors.New("无效的题型，支持的题型：" + strings.Join(validQuestionTypes, ", "))
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
func (s *QuestionBankService) GetQuestion(id int) (map[string]any, error) {
	var q model.Question
	if err := s.db.First(&q, id).Error; err != nil {
		return nil, errors.New("题目不存在")
	}
	return questionToDict(&q, true), nil
}

// UpdateQuestion 更新题目。
// 特殊处理：当 status 由 draft 改为 pending（导师重新提交审核）时，清空驳回理由。
func (s *QuestionBankService) UpdateQuestion(id int, data map[string]any) (map[string]any, error) {
	var q model.Question
	if err := s.db.First(&q, id).Error; err != nil {
		return nil, errors.New("题目不存在")
	}
	if t, ok := data["type"].(string); ok && !containsString(validQuestionTypes, t) {
		return nil, errors.New("无效的题型")
	}
	if st, ok := data["status"].(string); ok && !containsString(validQuestionStatus, st) {
		return nil, errors.New("无效的状态")
	}
	// 检测重新提交：原本 draft 状态，新状态为 pending → 视为导师修改后重新提交，清空驳回理由
	resubmitting := q.Status == "draft" && data["status"] == "pending"
	applyQuestionFields(&q, data)
	if resubmitting {
		q.RejectReason = ""
	}
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
func (s *QuestionBankService) ListQuestions(page, pageSize int, qType string, kpID *int, status, keyword string) map[string]any {
	if page <= 0 {
		page = 1
	}
	if pageSize <= 0 {
		pageSize = 20
	}
	q := s.db.Model(&model.Question{})
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
	q.Order("created_at DESC, id ASC").Offset((page - 1) * pageSize).Limit(pageSize).Find(&list)
	out := make([]map[string]any, 0, len(list))
	for i := range list {
		out = append(out, questionToDict(&list[i], true))
	}
	return map[string]any{
		"total":     total,
		"page":      page,
		"page_size": pageSize,
		"questions": out,
	}
}

// PublishQuestion 发布题目（管理员审核通过）。同时清空驳回理由。
func (s *QuestionBankService) PublishQuestion(id int) (map[string]any, error) {
	var q model.Question
	if err := s.db.First(&q, id).Error; err != nil {
		return nil, errors.New("题目不存在")
	}
	q.Status = "published"
	q.RejectReason = ""
	q.UpdatedAt = beijingNow()
	if err := s.db.Save(&q).Error; err != nil {
		return nil, err
	}
	return questionToDict(&q, true), nil
}

// BatchPublish 批量发布（管理员审核通过）。同时清空驳回理由。
func (s *QuestionBankService) BatchPublish(ids []int) map[string]any {
	count := 0
	if len(ids) > 0 {
		count64 := s.db.Model(&model.Question{}).
			Where("id IN ?", ids).
			Updates(map[string]any{"status": "published", "reject_reason": ""}).
			RowsAffected
		count = int(count64)
	}
	return map[string]any{"published_count": count}
}

// RejectQuestion 驳回题目（管理员审核）。状态回退为 draft，记录驳回理由供导师查看修改。
func (s *QuestionBankService) RejectQuestion(id int, reason string) (map[string]any, error) {
	if reason == "" {
		return nil, errors.New("请填写驳回理由")
	}
	var q model.Question
	if err := s.db.First(&q, id).Error; err != nil {
		return nil, errors.New("题目不存在")
	}
	q.Status = "draft"
	q.RejectReason = reason
	q.UpdatedAt = beijingNow()
	if err := s.db.Save(&q).Error; err != nil {
		return nil, err
	}
	return questionToDict(&q, true), nil
}

// BatchReject 批量驳回（管理员审核）。状态回退为 draft，统一记录同一驳回理由。
func (s *QuestionBankService) BatchReject(ids []int, reason string) (map[string]any, error) {
	if reason == "" {
		return nil, errors.New("请填写驳回理由")
	}
	if len(ids) == 0 {
		return map[string]any{"rejected_count": 0}, nil
	}
	count64 := s.db.Model(&model.Question{}).
		Where("id IN ?", ids).
		Updates(map[string]any{"status": "draft", "reject_reason": reason}).
		RowsAffected
	return map[string]any{"rejected_count": int(count64)}, nil
}

// BatchImport 批量导入题目。
func (s *QuestionBankService) BatchImport(items []any, createdBy *int) map[string]any {
	success, errs := 0, []map[string]any{}
	for i, item := range items {
		data, ok := item.(map[string]any)
		if !ok {
			errs = append(errs, map[string]any{"index": i, "error": "无效数据"})
			continue
		}
		data["status"] = "pending"
		if _, err := s.CreateQuestion(data, createdBy, "tutor"); err != nil {
			errs = append(errs, map[string]any{"index": i, "error": err.Error()})
			continue
		}
		success++
	}
	return map[string]any{
		"success_count": success,
		"error_count":   len(errs),
		"errors":        errs,
	}
}

// GetStats 题库统计。
func (s *QuestionBankService) GetStats() map[string]any {
	var total int64
	s.db.Model(&model.Question{}).Count(&total)
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
	// 按课程分类统计（经 knowledge_point.category 关联）
	byCategory := map[string]int64{}
	for _, cat := range validCategories {
		var c int64
		s.db.Model(&model.Question{}).
			Joins("LEFT JOIN knowledge_point ON knowledge_point.id = question.knowledge_point_id").
			Where("knowledge_point.category = ? AND question.status = ?", cat, "published").
			Count(&c)
		byCategory[cat] = c
	}
	var kps []model.KnowledgePoint
	s.db.Find(&kps)
	byKP := make([]map[string]any, 0, len(kps))
	for _, kp := range kps {
		var c int64
		s.db.Model(&model.Question{}).Where("knowledge_point_id = ?", kp.ID).Count(&c)
		byKP = append(byKP, map[string]any{"id": kp.ID, "name": kp.Name, "category": kp.Category, "count": c})
	}
	return map[string]any{
		"total":              total,
		"by_type":            byType,
		"by_status":          byStatus,
		"by_category":        byCategory,
		"by_knowledge_point": byKP,
	}
}

// GetCategories 返回课程四分类及其题目数。
func (s *QuestionBankService) GetCategories() []map[string]any {
	out := make([]map[string]any, 0, len(validCategories))
	for _, cat := range validCategories {
		var c int64
		s.db.Model(&model.Question{}).
			Joins("LEFT JOIN knowledge_point ON knowledge_point.id = question.knowledge_point_id").
			Where("knowledge_point.category = ? AND question.status = ?", cat, "published").
			Count(&c)
		out = append(out, map[string]any{"category": cat, "count": c})
	}
	return out
}

// CreateKnowledgePoint 创建知识点。
func (s *QuestionBankService) CreateKnowledgePoint(data map[string]any) (map[string]any, error) {
	name, _ := data["name"].(string)
	if name == "" {
		return nil, errors.New("知识点名称不能为空")
	}
	category, _ := data["category"].(string)
	if category != "" && !containsString(validCategories, category) {
		return nil, errors.New("无效的课程分类")
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
		Category:    category,
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
func (s *QuestionBankService) GetKnowledgePoints(category string, parentID *int) []map[string]any {
	q := s.db.Model(&model.KnowledgePoint{})
	if category != "" {
		q = q.Where("category = ?", category)
	}
	if parentID != nil {
		q = q.Where("parent_id = ?", *parentID)
	}
	var kps []model.KnowledgePoint
	q.Order("created_at ASC").Find(&kps)
	out := make([]map[string]any, 0, len(kps))
	for i := range kps {
		out = append(out, kpToDict(&kps[i]))
	}
	return out
}

// UpdateKnowledgePoint 更新知识点。
func (s *QuestionBankService) UpdateKnowledgePoint(id int, data map[string]any) (map[string]any, error) {
	var kp model.KnowledgePoint
	if err := s.db.First(&kp, id).Error; err != nil {
		return nil, errors.New("知识点不存在")
	}
	if v, ok := data["name"]; ok {
		kp.Name, _ = v.(string)
	}
	if v, ok := data["category"]; ok {
		if c, _ := v.(string); c != "" && !containsString(validCategories, c) {
			return nil, errors.New("无效的课程分类")
		}
		kp.Category, _ = v.(string)
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

func kpToDict(kp *model.KnowledgePoint) map[string]any {
	return map[string]any{
		"id":          kp.ID,
		"name":        kp.Name,
		"category":    kp.Category,
		"parent_id":   kp.ParentID,
		"description": kp.Description,
		"created_at":  formatISO(kp.CreatedAt),
	}
}

func applyQuestionFields(q *model.Question, data map[string]any) {
	if v, ok := data["type"]; ok {
		q.Type, _ = v.(string)
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
func getString(m map[string]any, key string) string {
	v, _ := m[key].(string)
	return v
}

func intToString(i int) string       { return toStringHelper(i) }
func floatToString(f float64) string { return toStringHelper(f) }

func toStringHelper(v interface{}) string {
	b, _ := json.Marshal(v)
	return string(b)
}
