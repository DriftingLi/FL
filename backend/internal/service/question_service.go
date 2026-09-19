// Package service 题目相关共享逻辑与题库 CRUD。
package service

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"math/rand"
	"sort"
	"strings"

	"go.uber.org/zap"
	"gorm.io/gorm"

	"forklift-training/internal/model"
	"forklift-training/pkg/paging"
)

// ErrQuestionNotFound 题目不存在（题库域哨兵，ADR-0024）：handler 以 errors.Is 映射 404。
var ErrQuestionNotFound = errors.New("题目不存在")

// 题库写面哨兵族（第十二波票 6，#1168）：形态照票 5 论坛域——业务事实各有哨兵，
// api 域表按档渲染；未命中（DB 故障）一律 500，不再由 fallback 吞成 400。
var (
	ErrQuestionTypeInvalid        = errors.New("无效的题型")
	ErrQuestionContentRequired    = errors.New("题干不能为空")
	ErrQuestionAnswerRequired     = errors.New("答案不能为空")
	ErrQuestionOptionsRequired    = errors.New("选项不能为空")
	ErrQuestionAnswerInvalid      = errors.New("answer 形态无效（仅支持字符串或数组）")
	ErrQuestionCredentialNotFound = errors.New("所属证件不存在")
	ErrSubmitNotDraft             = errors.New("仅待提交（draft）题目可提交审核")
	ErrRejectReasonRequired       = errors.New("请填写驳回理由")
)

// QuestionCreateInput 创建题目的 typed 入参（票 6：字段类型不符即绑定失败，不再静默落零值）。
// 不携带 status 通道——状态迁移只经显式动作（创建固定入 pending 队列）。
type QuestionCreateInput struct {
	Type            string          `json:"type"`
	Content         string          `json:"content"`
	Options         json.RawMessage `json:"options"`
	Answer          json.RawMessage `json:"answer"`
	Explanation     string          `json:"explanation"`
	ImageURL        string          `json:"image_url"`
	ReferenceAnswer string          `json:"reference_answer"`
	ScoringCriteria string          `json:"scoring_criteria"`
	Score           int             `json:"score"`
	CredentialID    int             `json:"credential_id"`
	TagIDs          []int           `json:"tag_ids"`
}

// QuestionUpdateInput 更新题目的 typed 入参（指针 = 「未提供」与「提供零值」可分，部分更新语义不变）。
// 同样不携带 status 通道；credential_id 维持既有面（更新不改证件归属）。
type QuestionUpdateInput struct {
	Type            *string          `json:"type"`
	Content         *string          `json:"content"`
	Options         *json.RawMessage `json:"options"`
	Answer          *json.RawMessage `json:"answer"`
	Explanation     *string          `json:"explanation"`
	ImageURL        *string          `json:"image_url"`
	ReferenceAnswer *string          `json:"reference_answer"`
	ScoringCriteria *string          `json:"scoring_criteria"`
	Score           *int             `json:"score"`
	TagIDs          *[]int           `json:"tag_ids"`
}

// QuestionBatchImportInput 批量导入的 typed body（票 6：swagger 面由 object 变 typed）。
type QuestionBatchImportInput struct {
	Questions []QuestionCreateInput `json:"questions"`
}

// stringifyAnswerJSON 把 typed 入参里的 answer 原始 JSON（字符串或数组）归一为存储字符串
// （口径与旧 stringifyAnswer(any) 逐字一致：数组以逗号连接）。
func stringifyAnswerJSON(raw json.RawMessage) (string, error) {
	if len(raw) == 0 {
		return "", nil
	}
	var s string
	if err := json.Unmarshal(raw, &s); err == nil {
		return s, nil
	}
	var arr []any
	if err := json.Unmarshal(raw, &arr); err == nil {
		return stringifyAnswer(arr), nil
	}
	return "", ErrQuestionAnswerInvalid
}

// effectiveOptions 归一 typed 写面的 options 原始 JSON：缺字段与 JSON null 都归入空选项桶。
// RawMessage 面上 null 是 4 字节非空值，不归一会绕过「选项不能为空」校验并把 "null" 落库。
func effectiveOptions(raw json.RawMessage) json.RawMessage {
	if len(raw) == 0 || string(raw) == "null" {
		return nil
	}
	return raw
}

// 题型与课程分类常量（已取消等级制度）。
var (
	validQuestionTypes  = []string{"single_choice", "multi_choice", "true_false", "fault_image", "short_answer"}
	validQuestionStatus = []string{"draft", "pending", "published"}
)

// sampleQuestions 统一抽题函数：从 published 题库按条件随机抽取 count 题。
// qType 为空表示不限题型。始终排除来源标记标签的题目。
func sampleQuestions(db *gorm.DB, qType string, count int, credentialID *int) ([]model.Question, error) {
	return sampleQuestionsByOpts(db, sampleQuestionsOpts{qType: qType, count: count, cred: credentialID, shuffle: true})
}

// sampleQuestionsOpts 抽题参数面（#385）：题库池口径（published + 排真题 + 证件分区）
// 之上叠加 全量/抽样、洗牌/排序 两个正交开关。
type sampleQuestionsOpts struct {
	qType   string // 题型过滤（空 = 不限）
	tagID   int    // >0：限定标签（专项练习）
	count   int    // >0 且 shuffle：抽样截断数
	shuffle bool   // true：洗牌后按 count 截断（随机抽样）；false：按 id 升序全量
	cred    *int   // 非 nil：按当前证件分区
}

// sampleQuestionsByOpts 抽题池统一实现（#385 单点）：收编随机练习、标签专项、
// 顺序练习与模拟考抽题的题库池 scope（question_pool_scope.go，ADR-0050 决策 1）。
// shuffle=false 时按 id 升序返回全量（顺序练习/标签专项的固定顺序来源）；
// shuffle=true 时洗牌、count>0 且超额则截断（随机练习/模拟考的抽样语义）。
// poolFilter 抽题侧在题库池 scope（question_pool_scope.go，唯一出处）之上叠加题型/标签读面差异。
// sampleQuestionsByOpts（抽题）与 countPoolByOpts（计数）共用，保证同参下「计数 == 抽题数量」
// 的一致性断言成立（#413 池计数单点，口径定义见 CONTEXT.md「题库池」）。
func poolFilter(q *gorm.DB, o sampleQuestionsOpts) *gorm.DB {
	q = QuestionPoolScope(q, o.cred)
	if o.tagID > 0 {
		q = q.Where("id IN (SELECT question_id FROM question_tag_relation WHERE tag_id = ?)", o.tagID)
	}
	if o.qType != "" {
		q = q.Where("type = ?", o.qType)
	}
	return q
}

// countPoolByOpts 池计数单点（#413）：与抽题共用 poolFilter，语义即「当前证件题库池数量」。
func countPoolByOpts(db *gorm.DB, o sampleQuestionsOpts) (int64, error) {
	var total int64
	if err := poolFilter(db.Model(&model.Question{}), o).Count(&total).Error; err != nil {
		return 0, err
	}
	return total, nil
}

func sampleQuestionsByOpts(db *gorm.DB, o sampleQuestionsOpts) ([]model.Question, error) {
	q := poolFilter(db.Model(&model.Question{}), o)
	if !o.shuffle {
		q = q.Order("id ASC")
	}
	var all []model.Question
	if err := q.Find(&all).Error; err != nil {
		return nil, err
	}
	if o.shuffle {
		all = shuffleTruncate(all, o.count)
	}
	return all, nil
}

// shuffleTruncate 洗牌截断（抽样固定顺序）：count<=0 或题量不足时原样返回。
func shuffleTruncate[T any](items []T, count int) []T {
	if count > 0 && len(items) > count {
		rand.Shuffle(len(items), func(i, j int) { items[i], items[j] = items[j], items[i] })
		items = items[:count]
	}
	return items
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

// CreateQuestion 创建题目（票 6 typed 面）：字段类型不符在绑定层即失败；status 通道不存在，
// 创建固定入 pending 审核队列；证件校验遇 DB 故障如实上抛（旧实现 fail-open 吞错）。
func (s *QuestionBankService) CreateQuestion(in QuestionCreateInput, createdBy *int, createdByType string) (QuestionDTO, error) {
	if !containsString(validQuestionTypes, in.Type) {
		return QuestionDTO{}, fmt.Errorf("%w，支持的题型：%s", ErrQuestionTypeInvalid, strings.Join(validQuestionTypes, ", "))
	}
	if in.Content == "" {
		return QuestionDTO{}, ErrQuestionContentRequired
	}
	answer, err := stringifyAnswerJSON(in.Answer)
	if err != nil {
		return QuestionDTO{}, err
	}
	if answer == "" && in.Type != "short_answer" {
		return QuestionDTO{}, ErrQuestionAnswerRequired
	}
	if (in.Type == "single_choice" || in.Type == "multi_choice" || in.Type == "fault_image") && len(effectiveOptions(in.Options)) == 0 {
		return QuestionDTO{}, ErrQuestionOptionsRequired
	}
	var credentialID *int
	if in.CredentialID > 0 {
		var cnt int64
		if err := s.db.Model(&model.Credential{}).Where("id = ?", in.CredentialID).Count(&cnt).Error; err != nil {
			return QuestionDTO{}, err
		}
		if cnt == 0 {
			return QuestionDTO{}, ErrQuestionCredentialNotFound
		}
		cid := in.CredentialID
		credentialID = &cid
	}
	var optionsBytes model.JSONB
	if opts := effectiveOptions(in.Options); len(opts) > 0 {
		optionsBytes = model.JSONB(opts)
	}
	q := model.Question{
		Type:            in.Type,
		Content:         in.Content,
		Options:         optionsBytes,
		Answer:          answer,
		Explanation:     in.Explanation,
		ImageURL:        in.ImageURL,
		ReferenceAnswer: in.ReferenceAnswer,
		ScoringCriteria: in.ScoringCriteria,
		Score:           in.Score,
		CredentialID:    credentialID,
		Status:          "pending",
		CreatedBy:       createdBy,
		CreatedByType:   orDefault(createdByType, "tutor"),
		CreatedAt:       beijingNow(),
		UpdatedAt:       beijingNow(),
	}
	if err := s.db.Create(&q).Error; err != nil {
		return QuestionDTO{}, err
	}
	if in.TagIDs != nil {
		if err := replaceQuestionTags(s.db, q.ID, in.TagIDs); err != nil {
			return QuestionDTO{}, err
		}
	}
	d := newQuestionDTO(&q, true)
	d.Tags = s.loadTagsByQuestion(q.ID)
	return d, nil
}

// GetQuestion 查询题目详情。
// GetQuestionForStudent 学员侧按 id 取题（ADR-0049「题库池是可见性口径，覆盖每条读路径」）：
// 过池口径（published + 排源标记真题题 + 当前证件）；不满足一律 ErrQuestionNotFound ——
// 404 而不是 403，避免把「这题存在但不能看」变成存在性泄露。
func (s *QuestionBankService) GetQuestionForStudent(id int, credentialID *int) (QuestionDTO, error) {
	var q model.Question
	err := poolFilter(s.db.Model(&model.Question{}), sampleQuestionsOpts{cred: credentialID}).
		Where("id = ?", id).First(&q).Error
	if err != nil {
		return QuestionDTO{}, ErrQuestionNotFound
	}
	d := newQuestionDTO(&q, true)
	d.Tags = s.loadTagsByQuestion(q.ID)
	return d, nil
}

// GetQuestion 查询题目详情（编辑面：作者/审核者，含 draft）。
func (s *QuestionBankService) GetQuestion(id int) (QuestionDTO, error) {
	var q model.Question
	if err := s.db.First(&q, id).Error; err != nil {
		return QuestionDTO{}, ErrQuestionNotFound
	}
	d := newQuestionDTO(&q, true)
	d.Tags = s.loadTagsByQuestion(q.ID)
	return d, nil
}

// UpdateQuestion 更新题目（票 6 typed 面）：写面不携带 status 通道；「编辑未改不动」——
// 只有内容与计分字段（题型/题干/选项/答案/解析/分值）实际变化才触发审核不变式：
//   - 讲师（非 admin）修改已发布题的内容 → 回 pending 重审（暂离题库池），并清驳回理由；
//   - 管理员即审核者，修改保持原状态即时生效（自审无意义）；
//   - 纯分区属性（标签）修改不动状态。
func (s *QuestionBankService) UpdateQuestion(id int, in QuestionUpdateInput, actorType string) (QuestionDTO, error) {
	var q model.Question
	if err := s.db.First(&q, id).Error; err != nil {
		return QuestionDTO{}, ErrQuestionNotFound
	}
	if in.Type != nil && !containsString(validQuestionTypes, *in.Type) {
		return QuestionDTO{}, ErrQuestionTypeInvalid
	}
	answerChanged := false
	if in.Answer != nil {
		newAnswer, err := stringifyAnswerJSON(*in.Answer)
		if err != nil {
			return QuestionDTO{}, err
		}
		if newAnswer != q.Answer {
			answerChanged = true
		}
		q.Answer = newAnswer
	}
	contentChanged := questionContentTouched(&q, in, answerChanged)
	applyQuestionUpdateFields(&q, in)
	if contentChanged && q.Status == "published" && actorType != "admin" {
		q.Status = "pending"
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
	if in.TagIDs != nil {
		if err := replaceQuestionTags(s.db, id, *in.TagIDs); err != nil {
			return QuestionDTO{}, err
		}
	}
	d := newQuestionDTO(&q, true)
	d.Tags = s.loadTagsByQuestion(id)
	return d, nil
}

// questionContentTouched 判定「内容与计分字段是否实际变化」（票 6 不变式谓词）：
// 只比较请求提供的字段与库中现值；q 此刻尚未被本请求改写（answer 除外，由调用方传入判定结果）。
func questionContentTouched(q *model.Question, in QuestionUpdateInput, answerChanged bool) bool {
	touched := false
	if in.Type != nil && *in.Type != q.Type {
		touched = true
	}
	if in.Content != nil && *in.Content != q.Content {
		touched = true
	}
	if in.Options != nil && !jsonEqualCompact(string(q.Options), effectiveOptions(*in.Options)) {
		touched = true
	}
	if answerChanged {
		touched = true
	}
	if in.Explanation != nil && *in.Explanation != q.Explanation {
		touched = true
	}
	if in.ReferenceAnswer != nil && *in.ReferenceAnswer != q.ReferenceAnswer {
		touched = true
	}
	if in.ScoringCriteria != nil && *in.ScoringCriteria != q.ScoringCriteria {
		touched = true
	}
	if in.Score != nil && *in.Score != q.Score {
		touched = true
	}
	return touched
}

// jsonEqualCompact 比较两段 JSON 的紧凑形态是否语义相等（键序无关的字节比较近似：
// 存储与请求都来自同一序列化链，紧凑化后按字节比即可）。
func jsonEqualCompact(a string, b json.RawMessage) bool {
	return compactJSON([]byte(a)) == compactJSON(b)
}

func compactJSON(raw []byte) string {
	if len(raw) == 0 {
		return ""
	}
	var buf bytes.Buffer
	if err := json.Compact(&buf, raw); err != nil {
		return string(raw)
	}
	return buf.String()
}

// applyQuestionUpdateFields 把 typed 入参中「已提供」的字段写入模型（与旧 map 版逐字段对应；
// status 通道已删）。answer 由调用方先行处理（stringify 校验）。
func applyQuestionUpdateFields(q *model.Question, in QuestionUpdateInput) {
	if in.Type != nil {
		q.Type = *in.Type
	}
	if in.Content != nil {
		q.Content = *in.Content
	}
	if in.Options != nil {
		if eff := effectiveOptions(*in.Options); len(eff) == 0 {
			q.Options = nil
		} else {
			q.Options = model.JSONB(eff)
		}
	}
	if in.Explanation != nil {
		q.Explanation = *in.Explanation
	}
	if in.ImageURL != nil {
		q.ImageURL = *in.ImageURL
	}
	if in.ReferenceAnswer != nil {
		q.ReferenceAnswer = *in.ReferenceAnswer
	}
	if in.ScoringCriteria != nil {
		q.ScoringCriteria = *in.ScoringCriteria
	}
	if in.Score != nil {
		q.Score = *in.Score
	}
}

// SubmitQuestion 显式「提交审核」动作（票 6）：draft → pending，清空驳回理由。
// 取代旧「编辑时顺手把 status 传回去」的实现巧合（前端 QuestionManage 的提交按钮改接本端点）。
func (s *QuestionBankService) SubmitQuestion(id int) (QuestionDTO, error) {
	var q model.Question
	if err := s.db.First(&q, id).Error; err != nil {
		return QuestionDTO{}, ErrQuestionNotFound
	}
	if q.Status != "draft" {
		return QuestionDTO{}, ErrSubmitNotDraft
	}
	q.Status = "pending"
	q.RejectReason = ""
	q.UpdatedAt = beijingNow()
	if err := s.db.Save(&q).Error; err != nil {
		return QuestionDTO{}, err
	}
	d := newQuestionDTO(&q, true)
	d.Tags = s.loadTagsByQuestion(q.ID)
	return d, nil
}

// DeleteQuestion 删除题目，并清理题图存储文件。
func (s *QuestionBankService) DeleteQuestion(id int) error {
	var q model.Question
	if err := s.db.First(&q, id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ErrQuestionNotFound
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

// QuestionPageDTO 题库分页结果（ADR-0009 §2 typed DTO / spec #940 片三）。
// 字段按 JSON key 字母序声明（page / page_size / questions / total）—— 旧 map 的序列化序。
type QuestionPageDTO struct {
	Page      int           `json:"page"`
	PageSize  int           `json:"page_size"`
	Questions []QuestionDTO `json:"questions"`
	Total     int64         `json:"total"`
}

// QuestionImportErrorDTO 单条导入失败：index 为入参下标，error 为原因。
type QuestionImportErrorDTO struct {
	Error string `json:"error"`
	Index int    `json:"index"`
}

// QuestionImportResultDTO 批量导入结果（字段按 JSON key 字母序：error_count / errors / success_count）。
// Errors 用非 nil 空切片初始化，保证无失败时序列化为 []（与旧 map 的 []map[string]any{} 同形）。
type QuestionImportResultDTO struct {
	ErrorCount   int                      `json:"error_count"`
	Errors       []QuestionImportErrorDTO `json:"errors"`
	SuccessCount int                      `json:"success_count"`
}

// QuestionPublishResultDTO 批量发布结果。
type QuestionPublishResultDTO struct {
	PublishedCount int `json:"published_count"`
}

// QuestionImageUploadDTO 题目图片上传结果 {"url": "..."}（原 handler 内联 gin.H，ADR-0048 片六收口）。
type QuestionImageUploadDTO struct {
	URL string `json:"url"`
}

// QuestionRejectResultDTO 批量驳回结果。
type QuestionRejectResultDTO struct {
	RejectedCount int `json:"rejected_count"`
}

// ListQuestions 题目列表分页查询（可按标签 tagID 过滤，结果附带标签列表）。
func (s *QuestionBankService) ListQuestions(page, pageSize int, qType string, status, keyword string, tagID *int, credentialID *int, sortBy string) (*QuestionPageDTO, error) {
	// 排序口径（#412）：缺省保持现状「最新提交优先」（created_at DESC, id ASC）；
	// 讲师端显式传 id_asc 请求按 ID 升序，翻页时 ID 单调推进、不再呈锯齿跳回。
	// #1096：排序位由变参改显式命名参数（空串 = 缺省口径）。
	order := "created_at DESC, id ASC"
	if sortBy == "id_asc" {
		order = "id ASC"
	}
	list, total, page, pageSize, err := paging.Query[model.Question](s.db, page, pageSize, 20, order, func(q *gorm.DB) *gorm.DB {
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
		q = EntityOwnedBy(q, "credential_id", credentialID)
		return q
	})
	if err != nil {
		return nil, err
	}
	out := make([]QuestionDTO, 0, len(list))
	ids := make([]int, 0, len(list))
	for i := range list {
		out = append(out, newQuestionDTO(&list[i], true))
		ids = append(ids, list[i].ID)
	}
	s.attachTagsBatch(ids, out)
	return &QuestionPageDTO{
		Page:      page,
		PageSize:  pageSize,
		Questions: out,
		Total:     total,
	}, nil
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
		return QuestionDTO{}, ErrQuestionNotFound
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
func (s *QuestionBankService) BatchPublish(ids []int) *QuestionPublishResultDTO {
	count := 0
	if len(ids) > 0 {
		count64 := s.db.Model(&model.Question{}).
			Where("id IN ?", ids).
			Updates(map[string]any{"status": "published", "reject_reason": ""}).
			RowsAffected
		count = int(count64)
	}
	return &QuestionPublishResultDTO{PublishedCount: count}
}

// RejectQuestion 驳回题目（管理员审核）。状态回退为 draft，记录驳回理由供导师查看修改。
func (s *QuestionBankService) RejectQuestion(id int, reason string) (QuestionDTO, error) {
	if reason == "" {
		return QuestionDTO{}, ErrRejectReasonRequired
	}
	var q model.Question
	if err := s.db.First(&q, id).Error; err != nil {
		return QuestionDTO{}, ErrQuestionNotFound
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
func (s *QuestionBankService) BatchReject(ids []int, reason string) (*QuestionRejectResultDTO, error) {
	if reason == "" {
		return nil, ErrRejectReasonRequired
	}
	if len(ids) == 0 {
		return &QuestionRejectResultDTO{RejectedCount: 0}, nil
	}
	count64 := s.db.Model(&model.Question{}).
		Where("id IN ?", ids).
		Updates(map[string]any{"status": "draft", "reject_reason": reason}).
		RowsAffected
	return &QuestionRejectResultDTO{RejectedCount: int(count64)}, nil
}

// BatchImport 批量导入题目（票 6 typed 面：逐条 QuestionCreateInput，类型不符整条计入 errors）。
func (s *QuestionBankService) BatchImport(items []QuestionCreateInput, createdBy *int) *QuestionImportResultDTO {
	success, errs := 0, make([]QuestionImportErrorDTO, 0)
	for i, item := range items {
		if _, err := s.CreateQuestion(item, createdBy, "tutor"); err != nil {
			errs = append(errs, QuestionImportErrorDTO{Index: i, Error: err.Error()})
			continue
		}
		success++
	}
	return &QuestionImportResultDTO{
		ErrorCount:   len(errs),
		Errors:       errs,
		SuccessCount: success,
	}
}

// GetStats 题库统计（经统计聚合 module，一次 GROUP BY + 维度零填充）。
// Total 为题库池数量（#413）：已发布 + 排除来源标记标签 + 可选证件分区，非全表计数；
// 学员端顺序练习卡片分母的唯一消费方。by_type / by_status 保留全局口径。
func (s *QuestionBankService) GetStats(credentialID *int) *QuestionBankStatsDTO {
	total, err := countPoolByOpts(s.db, sampleQuestionsOpts{cred: credentialID})
	if err != nil {
		total = 0
	}
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

func intToString(i int) string       { return toStringHelper(i) }
func floatToString(f float64) string { return toStringHelper(f) }

func toStringHelper(v interface{}) string {
	b, _ := json.Marshal(v)
	return string(b)
}
