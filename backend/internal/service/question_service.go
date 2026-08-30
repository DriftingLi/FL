// Package service 题目相关共享逻辑与题库 CRUD。
package service

import (
	"encoding/json"
	"errors"
	"math/rand"
	"sort"
	"strings"

	"go.uber.org/zap"
	"gorm.io/gorm"

	"forklift-training/internal/model"
	"forklift-training/pkg/paging"
)

// 题型与课程分类常量（已取消等级制度）。
var (
	validQuestionTypes  = []string{"single_choice", "multi_choice", "true_false", "fault_image", "short_answer"}
	validQuestionStatus = []string{"draft", "pending", "published"}
)

// excludeSourceTagsSQL 排除来源标记标签（is_source_tag，如真题）题目的公共过滤片段：
// 这类题目只能经真题卷作答，不进顺序/随机/专项练习与模拟考抽题池（ADR-0022）。
const excludeSourceTagsSQL = "NOT EXISTS (SELECT 1 FROM question_tag_relation qtr JOIN question_tag qt ON qt.id = qtr.tag_id WHERE qtr.question_id = question.id AND qt.is_source_tag)"

// sampleQuestions 统一抽题函数：从 published 题库按条件随机抽取 count 题。
// qType 为空表示不限题型。始终排除来源标记标签的题目。
func sampleQuestions(db *gorm.DB, qType string, count int, credentialID ...*int) ([]model.Question, error) {
	q := db.Model(&model.Question{}).Where("status = ?", "published").Where(excludeSourceTagsSQL)
	if len(credentialID) > 0 && credentialID[0] != nil {
		q = q.Where("credential_id = ?", *credentialID[0])
	}
	if qType != "" {
		q = q.Where("type = ?", qType)
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

// gradeQuestion 评分（判题唯一实现）。
// 返回 (isCorrect, earned)：isCorrect 为 nil 表示无法判定（简答题/未作答），earned 为得分。
// maxScore 为 0 时按练习分值表取默认值。
func gradeQuestion(q *model.Question, userAnswer interface{}, maxScore float64) (*bool, float64) {
	if userAnswer == nil {
		return nil, 0
	}
	if maxScore == 0 {
		maxScore = questionMaxScore("practice", q.Type)
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
// 使用 Limit(1).Find() 替代 First()，避免首次错题入库时 GORM logger 误报 record not found
func addToWrongQuestions(db *gorm.DB, studentID, questionID int) error {
	var wq model.WrongQuestion
	err := db.Where("student_id = ? AND question_id = ?", studentID, questionID).Limit(1).Find(&wq).Error
	if err != nil {
		return err
	}
	if wq.ID != 0 {
		wq.WrongCount++
		wq.LastWrongAt = beijingNow()
		wq.IsRemoved = false
		return db.Save(&wq).Error
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
	db      *gorm.DB
	fileSvc *FileStore

	logger *zap.Logger
}

// NewQuestionBankService 创建题库服务。fileSvc 用于删除题目时清理题图（可 nil，nil 时跳过）。
func NewQuestionBankService(db *gorm.DB, fileSvc *FileStore, logger *zap.Logger) *QuestionBankService {
	return &QuestionBankService{db: db, fileSvc: fileSvc, logger: logger}
}

// CreateQuestion 创建题目。
func (s *QuestionBankService) CreateQuestion(data map[string]any, createdBy *int, createdByType string) (QuestionDTO, error) {
	qType, _ := data["type"].(string)
	if !containsString(validQuestionTypes, qType) {
		return QuestionDTO{}, errors.New("无效的题型，支持的题型：" + strings.Join(validQuestionTypes, ", "))
	}
	content, _ := data["content"].(string)
	if content == "" {
		return QuestionDTO{}, errors.New("题干不能为空")
	}
	answer := stringifyAnswer(data["answer"])
	if answer == "" && qType != "short_answer" {
		return QuestionDTO{}, errors.New("答案不能为空")
	}
	options := data["options"]
	if qType == "single_choice" || qType == "multi_choice" || qType == "fault_image" {
		if options == nil {
			return QuestionDTO{}, errors.New("选项不能为空")
		}
	}
	status, _ := data["status"].(string)
	if status == "" {
		status = "pending"
	}
	var credentialID *int
	if v, ok := data["credential_id"]; ok {
		if cid := toInt(v); cid > 0 {
			var cnt int64
			if err := s.db.Model(&model.Credential{}).Where("id = ?", cid).Count(&cnt).Error; err == nil && cnt == 0 {
				return QuestionDTO{}, errors.New("所属证件不存在")
			}
			credentialID = &cid
		}
	}
	var optionsBytes model.JSONB
	if options != nil {
		if b, err := json.Marshal(options); err == nil {
			optionsBytes = model.JSONB(b)
		}
	}
	q := model.Question{
		Type:            qType,
		Content:         content,
		Options:         optionsBytes,
		Answer:          answer,
		Explanation:     getString(data, "explanation"),
		ImageURL:        getString(data, "image_url"),
		ReferenceAnswer: getString(data, "reference_answer"),
		ScoringCriteria: getString(data, "scoring_criteria"),
		Score:           toIntDefault(data["score"], 0),
		CredentialID:    credentialID,
		Status:          status,
		CreatedBy:       createdBy,
		CreatedByType:   orDefault(createdByType, "tutor"),
		CreatedAt:       beijingNow(),
		UpdatedAt:       beijingNow(),
	}
	if err := s.db.Create(&q).Error; err != nil {
		return QuestionDTO{}, err
	}
	if v, ok := data["tag_ids"]; ok {
		if err := replaceQuestionTags(s.db, q.ID, toIntSlice(v)); err != nil {
			return QuestionDTO{}, err
		}
	}
	d := newQuestionDTO(&q, true)
	d.Tags = s.loadTagsByQuestion(q.ID)
	return d, nil
}

// GetQuestion 查询题目详情。
func (s *QuestionBankService) GetQuestion(id int) (QuestionDTO, error) {
	var q model.Question
	if err := s.db.First(&q, id).Error; err != nil {
		return QuestionDTO{}, errors.New("题目不存在")
	}
	d := newQuestionDTO(&q, true)
	d.Tags = s.loadTagsByQuestion(q.ID)
	return d, nil
}

// UpdateQuestion 更新题目。
// 特殊处理：当 status 由 draft 改为 pending（导师重新提交审核）时，清空驳回理由。
func (s *QuestionBankService) UpdateQuestion(id int, data map[string]any) (QuestionDTO, error) {
	var q model.Question
	if err := s.db.First(&q, id).Error; err != nil {
		return QuestionDTO{}, errors.New("题目不存在")
	}
	if t, ok := data["type"].(string); ok && !containsString(validQuestionTypes, t) {
		return QuestionDTO{}, errors.New("无效的题型")
	}
	if st, ok := data["status"].(string); ok && !containsString(validQuestionStatus, st) {
		return QuestionDTO{}, errors.New("无效的状态")
	}
	// 检测重新提交：原本 draft 状态，新状态为 pending → 视为导师修改后重新提交，清空驳回理由
	resubmitting := q.Status == "draft" && data["status"] == "pending"
	applyQuestionFields(&q, data)
	if resubmitting {
		q.RejectReason = ""
	}
	q.UpdatedAt = beijingNow()
	// 改题即失效旧 AI 解析（spec #295）：题目内容变更后缓存不再可信，与保存同事务清列。
	if err := s.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Save(&q).Error; err != nil {
			return err
		}
		return tx.Model(&model.Question{}).Where("id = ?", id).Update("ai_explanation", "").Error
	}); err != nil {
		return QuestionDTO{}, err
	}
	if v, ok := data["tag_ids"]; ok {
		if err := replaceQuestionTags(s.db, id, toIntSlice(v)); err != nil {
			return QuestionDTO{}, err
		}
	}
	d := newQuestionDTO(&q, true)
	d.Tags = s.loadTagsByQuestion(id)
	return d, nil
}

// DeleteQuestion 删除题目，并清理题图存储文件。
func (s *QuestionBankService) DeleteQuestion(id int) error {
	var q model.Question
	if err := s.db.First(&q, id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return errors.New("题目不存在")
		}
		return err
	}
	if err := s.db.Delete(&q).Error; err != nil {
		return err
	}
	if s.fileSvc != nil && q.ImageURL != "" {
		_ = s.fileSvc.Delete(q.ImageURL)
	}
	return nil
}

// ListQuestions 题目列表分页查询（可按标签 tagID 过滤，结果附带标签列表）。
func (s *QuestionBankService) ListQuestions(page, pageSize int, qType string, status, keyword string, tagID *int, credentialID ...*int) map[string]any {
	list, total, page, pageSize := paging.Query[model.Question](s.db, page, pageSize, 20, "created_at DESC, id ASC", func(q *gorm.DB) *gorm.DB {
		if qType != "" {
			q = q.Where("type = ?", qType)
		}
		if status != "" {
			q = q.Where("status = ?", status)
		}
		if keyword != "" {
			q = q.Where("content LIKE ?", "%"+keyword+"%")
		}
		if tagID != nil {
			q = q.Where("id IN (SELECT question_id FROM question_tag_relation WHERE tag_id = ?)", *tagID)
		}
		if len(credentialID) > 0 && credentialID[0] != nil {
			q = q.Where("credential_id = ?", *credentialID[0])
		}
		return q
	})
	out := make([]QuestionDTO, 0, len(list))
	ids := make([]int, 0, len(list))
	for i := range list {
		out = append(out, newQuestionDTO(&list[i], true))
		ids = append(ids, list[i].ID)
	}
	s.attachTagsBatch(ids, out)
	return map[string]any{
		"total":     total,
		"page":      page,
		"page_size": pageSize,
		"questions": out,
	}
}

// loadTagsByQuestion 加载单题标签列表。
func (s *QuestionBankService) loadTagsByQuestion(questionID int) []map[string]any {
	return s.loadTagsBatch([]int{questionID})[questionID]
}

// attachTagsBatch 批量附加标签到题目 DTO（避免逐题查询 N+1）。
func (s *QuestionBankService) attachTagsBatch(questionIDs []int, dtos []QuestionDTO) {
	if len(questionIDs) == 0 {
		return
	}
	byID := s.loadTagsBatch(questionIDs)
	for i, id := range questionIDs {
		if tags, ok := byID[id]; ok {
			dtos[i].Tags = tags
		} else {
			dtos[i].Tags = []map[string]any{}
		}
	}
}

// loadTagsBatch 批量加载题目标签，key 为题目 ID。
func (s *QuestionBankService) loadTagsBatch(questionIDs []int) map[int][]map[string]any {
	result := make(map[int][]map[string]any, len(questionIDs))
	if len(questionIDs) == 0 {
		return result
	}
	type tagRow struct {
		QuestionID int    `gorm:"column:question_id"`
		TagID      int    `gorm:"column:tag_id"`
		TagCode    string `gorm:"column:tag_code"`
		TagName    string `gorm:"column:tag_name"`
		SortOrder  int    `gorm:"column:tag_sort"`
		Status     int16  `gorm:"column:tag_status"`
	}
	var rows []tagRow
	if err := s.db.Table("question_tag_relation AS qtr").
		Select("qtr.question_id, qtr.tag_id, t.code AS tag_code, t.name AS tag_name, t.sort_order AS tag_sort, t.status AS tag_status").
		Joins("JOIN question_tag AS t ON t.id = qtr.tag_id").
		Where("qtr.question_id IN ?", questionIDs).
		Order("t.sort_order ASC, t.id ASC").
		Scan(&rows).Error; err != nil {
		return result
	}
	for i := range rows {
		result[rows[i].QuestionID] = append(result[rows[i].QuestionID], map[string]any{
			"id":         rows[i].TagID,
			"code":       rows[i].TagCode,
			"name":       rows[i].TagName,
			"sort_order": rows[i].SortOrder,
			"status":     rows[i].Status,
		})
	}
	return result
}

// PublishQuestion 发布题目（管理员审核通过）。同时清空驳回理由。
func (s *QuestionBankService) PublishQuestion(id int) (QuestionDTO, error) {
	var q model.Question
	if err := s.db.First(&q, id).Error; err != nil {
		return QuestionDTO{}, errors.New("题目不存在")
	}
	q.Status = "published"
	q.RejectReason = ""
	q.UpdatedAt = beijingNow()
	if err := s.db.Save(&q).Error; err != nil {
		return QuestionDTO{}, err
	}
	return newQuestionDTO(&q, true), nil
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
func (s *QuestionBankService) RejectQuestion(id int, reason string) (QuestionDTO, error) {
	if reason == "" {
		return QuestionDTO{}, errors.New("请填写驳回理由")
	}
	var q model.Question
	if err := s.db.First(&q, id).Error; err != nil {
		return QuestionDTO{}, errors.New("题目不存在")
	}
	q.Status = "draft"
	q.RejectReason = reason
	q.UpdatedAt = beijingNow()
	if err := s.db.Save(&q).Error; err != nil {
		return QuestionDTO{}, err
	}
	return newQuestionDTO(&q, true), nil
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

// GetStats 题库统计（经统计聚合 module，一次 GROUP BY + 维度零填充）。
func (s *QuestionBankService) GetStats() *QuestionBankStatsDTO {
	var total int64
	s.db.Model(&model.Question{}).Count(&total)
	byType := groupByCount(s.db.Model(&model.Question{}), "type")
	byStatus := groupByCount(s.db.Model(&model.Question{}), "status")
	// 保留旧语义：by_type / by_status 对合法维度零填充（未出现的维度以 0 呈现）。
	byt := make(map[string]int64, len(validQuestionTypes))
	for _, t := range validQuestionTypes {
		byt[t] = byType[t]
	}
	bys := make(map[string]int64, len(validQuestionStatus))
	for _, st := range validQuestionStatus {
		bys[st] = byStatus[st]
	}
	return &QuestionBankStatsDTO{Total: total, ByType: byt, ByStatus: bys}
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
		i, _ := parseInt(n)
		return i
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
