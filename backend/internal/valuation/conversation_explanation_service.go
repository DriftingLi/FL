package valuation

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"net"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"github.com/cloudwego/eino/schema"
	"go.uber.org/zap"

	"forklift-training/internal/aiassistant"
)

const maxExplanationTextRunes = 4000

type explanationRunner struct {
	owner      *ConversationService
	model      aiassistant.ModelPort
	repository ExplanationRepository
	config     SuggestionConfig
	logger     *zap.Logger
}

func (s *ConversationService) EnableExplanations(model aiassistant.ModelPort, repository ExplanationRepository, config SuggestionConfig, logger *zap.Logger) {
	s.explanations = &explanationRunner{owner: s, model: model, repository: repository, config: normalizeSuggestionConfig(config), logger: logger}
	if s.explanations.logger == nil {
		s.explanations.logger = zap.NewNop()
	}
}

func (s *ConversationService) SetTrialCardReader(reader TrialCardReader) {
	s.trialCards = reader
}

func (s *ConversationService) RequestExplanation(ctx context.Context, req *ExplainCardRequest, userID int, clientIP string) (*ConversationOperation, error) {
	runner := s.explanations
	if runner == nil || runner.model == nil || runner.repository == nil || runner.config.HMACKey == "" {
		return nil, ErrSuggestionUnavailable
	}
	operationID, err := normalizeOperationID(req.OperationID)
	if err != nil {
		return nil, err
	}
	if req.DraftVersion <= 0 {
		return nil, fmt.Errorf("%w: draft_version must be positive", ErrInvalidOperation)
	}
	question := strings.TrimSpace(req.Question)
	if question == "" || len([]rune(question)) > maxExplanationTextRunes {
		return nil, fmt.Errorf("%w: question must contain 1-4000 characters", ErrInvalidOperation)
	}
	ip := net.ParseIP(strings.TrimSpace(clientIP))
	if ip == nil {
		return nil, ErrInvalidClientIP
	}
	card := req.Card
	card.Kind = strings.TrimSpace(card.Kind)
	card.DraftID = strings.TrimSpace(card.DraftID)
	card.OperationID = strings.TrimSpace(card.OperationID)
	card.RequestID = strings.TrimSpace(card.RequestID)
	card.SnapshotID = strings.TrimSpace(card.SnapshotID)
	snapshot, err := s.loadExplanationCard(ctx, &card, req.DraftVersion, userID)
	if err != nil {
		return nil, err
	}
	operationKey := explanationOperationKey(snapshot.CardKey, operationID)
	hash, err := hashExplanationRequest(snapshot.CardKey, req.DraftVersion, question)
	if err != nil {
		return nil, err
	}
	if existing, err := runner.repository.GetExplanation(ctx, operationKey); err == nil {
		if existing.RequestHash != hash {
			return nil, ErrOperationHashConflict
		}
		return existing, nil
	} else if !errors.Is(err, ErrOperationNotFound) {
		return nil, err
	}
	now := s.now()
	op := &ConversationOperation{ID: operationKey, OperationID: operationKey, Kind: ConversationOperationKindExplanation,
		CardKind: card.Kind, CardKey: snapshot.CardKey, DraftID: snapshot.DraftID, DraftVersion: req.DraftVersion, BaseVersion: req.DraftVersion,
		Status: OperationStatusPending, RequestHash: hash, CreatedAt: now, UpdatedAt: now}
	if userID > 0 {
		op.FirstAcceptedUserID = &userID
	}
	reservation := suggestionReservation(runner.config, explanationBudgetKey(snapshot), operationKey, ip)
	accepted, inserted, err := runner.repository.AcceptExplanation(ctx, op, reservation, now)
	if err != nil {
		return nil, err
	}
	if accepted.RequestHash != hash {
		return nil, ErrOperationHashConflict
	}
	if !inserted {
		return accepted, nil
	}
	messages := []*schema.Message{
		schema.SystemMessage(explanationSystemPrompt),
		schema.UserMessage(fmt.Sprintf("卡片类型=%s\n卡片版本=%d\n问题=%s\n事实键=%s", card.Kind, req.DraftVersion, question, strings.Join(sortedFactIDs(snapshot.Facts), ","))),
	}
	runner.logger.Info("估值解释已受理", zap.String("operation_key", operationKey), zap.String("card_kind", card.Kind), zap.Int("card_version", req.DraftVersion), zap.Bool("authenticated", userID > 0))
	go runner.run(context.Background(), operationKey, messages, snapshot, counterfactualHoursQuestion(question))
	return accepted, nil
}

func (r *explanationRunner) run(ctx context.Context, operationKey string, messages []*schema.Message, snapshot *explanationSnapshot, counterfactual bool) {
	ctx, cancel := context.WithTimeout(ctx, r.config.Timeout)
	defer cancel()
	content, err := r.model.CompleteStructured(ctx, aiassistant.FeatureValuationDraftSuggest, messages, valuationExplanationSchemaJSON,
		aiassistant.StructuredCompleteOptions{CompleteOptions: aiassistant.CompleteOptions{MaxTokens: 900, Temperature: 0}, Timeout: r.config.Timeout})
	if err != nil {
		code := ErrorCodeSuggestionUnavailable
		if errors.Is(err, context.DeadlineExceeded) || errors.Is(err, context.Canceled) {
			code = ErrorCodeSuggestionTimeout
		}
		r.fail(operationKey, code)
		return
	}
	modelResult, err := decodeExplanationResult(content)
	if err != nil {
		r.fail(operationKey, ErrorCodeSuggestionValidation)
		return
	}
	result, err := prepareExplanation(snapshot, modelResult, counterfactual)
	if err != nil {
		r.fail(operationKey, ErrorCodeSuggestionValidation)
		return
	}
	if _, err := r.repository.CompleteExplanation(ctx, operationKey, result, r.owner.now()); err != nil {
		r.fail(operationKey, ErrorCodeSuggestionUnavailable)
		return
	}
	r.logger.Info("估值解释完成", zap.String("operation_key", operationKey), zap.String("action", result.Action))
}

func (r *explanationRunner) fail(operationKey, code string) {
	_ = r.repository.FailExplanation(context.Background(), operationKey, code, r.owner.now())
	r.logger.Info("估值解释失败", zap.String("operation_key", operationKey), zap.String("error_code", code))
}

func (s *ConversationService) GetExplanation(ctx context.Context, operationKey string, userID int) (*ConversationOperation, error) {
	if s.explanations == nil {
		return nil, ErrOperationNotFound
	}
	id, err := normalizeConversationOperationKey(operationKey)
	if err != nil {
		return nil, err
	}
	op, err := s.explanations.repository.GetExplanation(ctx, id)
	if err != nil {
		return nil, err
	}
	switch op.CardKind {
	case ExplanationCardDraft:
		if _, err := s.GetDraft(ctx, op.DraftID); err != nil {
			return nil, ErrDraftNotFound
		}
	case ExplanationCardFormal:
		original, err := s.operations.GetOperation(ctx, op.CardKey)
		if err != nil || (userID > 0 && (original.FirstAcceptedUserID == nil || *original.FirstAcceptedUserID != userID)) {
			return nil, ErrOperationNotFound
		}
	}
	if op.Status == OperationStatusPending {
		_ = s.explanations.repository.ExpireExplanation(ctx, id, s.now().Add(-s.explanations.config.Timeout), s.now())
		return s.explanations.repository.GetExplanation(ctx, id)
	}
	return op, nil
}

// loadExplanationCard 按卡片类型装载锁定事实。草稿卡携带 Snapshot 时以请求快照为准，
// 不读取当前草稿内容——旧卡因此可解释，也不会被后来的纠错覆盖。
func (s *ConversationService) loadExplanationCard(ctx context.Context, card *ExplanationCard, draftVersion, userID int) (*explanationSnapshot, error) {
	switch card.Kind {
	case ExplanationCardDraft:
		id, err := normalizeDraftID(card.DraftID)
		if err != nil || card.OperationID != "" || card.RequestID != "" || card.SnapshotID != "" || card.EvaluationID != 0 {
			return nil, fmt.Errorf("%w: invalid draft card", ErrInvalidOperation)
		}
		if len(card.Snapshot) > 0 {
			// 快照只在草稿仍存在时可解释，避免凭空构造事实绕开卡片能力 ID。
			if _, err := s.GetDraft(ctx, id); err != nil {
				return nil, err
			}
			facts, err := explanationFactsFromDraftFields(card.Snapshot)
			if err != nil {
				return nil, err
			}
			cardKey := draftCardRevisionKey(id, draftVersion)
			return &explanationSnapshot{CardKey: cardKey, CardKind: card.Kind, DraftID: id, SnapshotID: cardKey,
				Version: draftVersion, Facts: facts}, nil
		}
		draft, err := s.GetDraft(ctx, id)
		if err != nil {
			return nil, err
		}
		if draft.Version != draftVersion {
			return nil, ErrDraftVersionConflict
		}
		facts, err := explanationFactsFromDraftFields(draft.Fields)
		if err != nil {
			return nil, err
		}
		return &explanationSnapshot{CardKey: id, CardKind: card.Kind, DraftID: id, SnapshotID: id,
			Version: draft.Version, Facts: facts}, nil
	case ExplanationCardFormal:
		id, err := normalizeConversationOperationKey(card.OperationID)
		if err != nil || card.DraftID != "" || len(card.Snapshot) > 0 || card.RequestID != "" || card.SnapshotID != "" || (card.EvaluationID <= 0 && userID == 0) {
			return nil, fmt.Errorf("%w: invalid formal card", ErrInvalidOperation)
		}
		original, err := s.operations.GetOperation(ctx, id)
		if err != nil || original.Kind != ConversationOperationKindEvaluation {
			return nil, ErrOperationNotFound
		}
		if userID > 0 && (original.FirstAcceptedUserID == nil || *original.FirstAcceptedUserID != userID) {
			return nil, ErrOperationNotFound
		}
		if card.EvaluationID > 0 && original.EvaluationID != card.EvaluationID {
			return nil, ErrOperationNotFound
		}
		evaluation, err := s.evaluations.GetEvaluation(ctx, original.EvaluationID)
		if err != nil {
			return nil, ErrOperationNotFound
		}
		snapshot := formalExplanationSnapshot(evaluation)
		snapshot.CardKey = id
		return snapshot, nil
	case ExplanationCardTrial:
		if s.trialCards == nil || card.RequestID == "" || card.SnapshotID == "" || card.DraftID != "" || len(card.Snapshot) > 0 || card.OperationID != "" || card.EvaluationID != 0 {
			return nil, ErrOperationNotFound
		}
		stored, err := s.trialCards.LoadTrialCard(ctx, card.RequestID)
		if err != nil || stored == nil || (stored.SnapshotID != card.SnapshotID && shortHash(stored.SnapshotID) != card.SnapshotID) {
			return nil, ErrOperationNotFound
		}
		return trialExplanationSnapshot(stored), nil
	default:
		return nil, fmt.Errorf("%w: unknown card kind", ErrInvalidOperation)
	}
}

func draftCardRevisionKey(draftID string, version int) string {
	return draftID + "@" + strconv.Itoa(version)
}

// explanationBudgetKey 解释与建议共用同一份草稿/IP/全站额度：草稿卡按草稿 ID 计，其余卡片按卡片键计。
func explanationBudgetKey(snapshot *explanationSnapshot) string {
	if snapshot.DraftID != "" {
		return snapshot.DraftID
	}
	return snapshot.CardKey
}

func explanationFactsFromDraftFields(fields map[string]DraftField) (map[string]ExplanationFact, error) {
	for name, field := range fields {
		if !isRequiredDraftField(name) {
			return nil, fmt.Errorf("%w: unknown draft snapshot field %q", ErrInvalidOperation, name)
		}
		if err := validateDraftField(name, field); err != nil {
			return nil, fmt.Errorf("%w: %v", ErrInvalidOperation, err)
		}
	}
	facts := make(map[string]ExplanationFact, len(fields))
	for name, field := range fields {
		var value any
		switch field.State {
		case DraftFieldValue:
			value = field.Value
		case DraftFieldRange:
			if field.Range == nil {
				continue
			}
			value = map[string]any{"min": field.Range.Min, "max": field.Range.Max}
		case DraftFieldCandidate:
			values := make([]any, 0, len(field.Candidates))
			for _, candidate := range field.Candidates {
				values = append(values, candidate.Value)
			}
			value = values
		default:
			continue
		}
		facts["input."+name] = ExplanationFact{ID: "input." + name, Label: name, Value: value}
	}
	return facts, nil
}

func formalExplanationSnapshot(evaluation *EvaluationDetail) *explanationSnapshot {
	widthRatio := explanationWidthRatio(evaluation.EstimatedValue, evaluation.ConfidenceLow, evaluation.ConfidenceHigh)
	snapshot := &explanationSnapshot{CardKey: fmt.Sprint(evaluation.ID), CardKind: ExplanationCardFormal,
		SnapshotID: fmt.Sprint(evaluation.ID), Version: 1, FormulaVersion: evaluation.FormulaVersion, Facts: map[string]ExplanationFact{}}
	values := map[string]any{"brand": evaluation.Brand, "vehicle_type": evaluation.VehicleType, "series": evaluation.Series,
		"tonnage": evaluation.Tonnage, "config_type": evaluation.ConfigType, "mast_type": evaluation.MastType, "mast_height_mm": evaluation.MastHeightMM,
		"factory_year": evaluation.FactoryYear, "sale_year": evaluation.SaleYear, "usage_hours": evaluation.UsageHours,
		"original_paint": evaluation.OriginalPaint, "province": evaluation.Province, "city": evaluation.City,
		"has_license_plate": evaluation.HasLicensePlate, "has_registration_certificate": evaluation.HasRegistrationCertificate,
		"has_maintenance_records": evaluation.HasMaintenanceRecords, "condition_rating": evaluation.ConditionRating}
	for name, value := range values {
		id := "input." + name
		snapshot.Facts[id] = ExplanationFact{ID: id, Label: name, Value: value}
	}
	results := map[string]any{
		"result.original_price": evaluation.OriginalPrice, "result.estimated_value": evaluation.EstimatedValue,
		"result.confidence_low": evaluation.ConfidenceLow, "result.confidence_high": evaluation.ConfidenceHigh,
		"result.width_ratio": widthRatio, "result.interval_width": evaluation.ConfidenceHigh - evaluation.ConfidenceLow,
		"coefficient.k_time": evaluation.KTime, "coefficient.k_time_adjusted": evaluation.KTimeAdjusted,
		"coefficient.k_hours": evaluation.KHours, "coefficient.k_brand": evaluation.KBrand,
		"coefficient.k_condition": evaluation.KCondition, "coefficient.k_market": evaluation.KMarket,
		"formula.lambda_electric": evaluation.LambdaElectric, "formula.lambda_combustion": evaluation.LambdaCombustion,
		"formula.decay_anchor": evaluation.DecayAnchor, "formula.version": evaluation.FormulaVersion,
	}
	for id, value := range results {
		snapshot.Facts[id] = ExplanationFact{ID: id, Label: explanationFactLabel(id), Value: value}
	}
	return snapshot
}

func trialExplanationSnapshot(card *TrialCard) *explanationSnapshot {
	snapshot := &explanationSnapshot{CardKey: card.RequestID, CardKind: ExplanationCardTrial, SnapshotID: card.SnapshotID,
		Version: 1, FormulaVersion: card.Response.Snapshot.FormulaVersion, Facts: map[string]ExplanationFact{}}
	encoded, _ := json.Marshal(card.Response.Input)
	var input map[string]any
	_ = json.Unmarshal(encoded, &input)
	for name, value := range input {
		id := "input." + name
		snapshot.Facts[id] = ExplanationFact{ID: id, Label: name, Value: value}
	}
	for id, value := range map[string]any{
		"formula.version": card.Response.Snapshot.FormulaVersion, "formula.hash": card.Response.Snapshot.FormulaHash,
		"policy.name": card.Response.Snapshot.Policy, "policy.version": card.Response.Snapshot.PolicyVersion,
		"snapshot.hash": card.SnapshotID, "snapshot.sha7": card.Response.Snapshot.SHA7,
	} {
		snapshot.Facts[id] = ExplanationFact{ID: id, Label: explanationFactLabel(id), Value: value}
	}
	snapshot.Facts["assumptions.list"] = ExplanationFact{ID: "assumptions.list", Label: "试算假设", Value: card.Response.Assumptions}
	snapshot.Facts["assumptions.count"] = ExplanationFact{ID: "assumptions.count", Label: "假设条数", Value: len(card.Response.Assumptions)}
	snapshot.Facts["candidates.list"] = ExplanationFact{ID: "candidates.list", Label: "目录候选", Value: card.Response.Candidates}
	snapshot.Facts["candidates.count"] = ExplanationFact{ID: "candidates.count", Label: "候选个数", Value: len(card.Response.Candidates)}
	if interval := card.Response.Interval; interval != nil {
		widthRatio := explanationWidthRatio(interval.Mid, interval.Min, interval.Max)
		for name, value := range map[string]any{
			"result.estimated_value": interval.Mid, "result.confidence_low": interval.Min, "result.confidence_high": interval.Max,
			"result.interval_width": interval.Width, "result.width_ratio": widthRatio,
		} {
			snapshot.Facts[name] = ExplanationFact{ID: name, Label: explanationFactLabel(name), Value: value}
		}
	}
	return snapshot
}

// explanationWidthRatio 把区间宽度折算成中值的倍数；中值为 0 时返回 0，避免除零与 NaN。
func explanationWidthRatio(mid, low, high float64) float64 {
	if mid == 0 || math.IsNaN(mid) || math.IsNaN(low) || math.IsNaN(high) {
		return 0
	}
	return math.Round((high-low)/mid*10000) / 10000
}

var explanationFactLabels = map[string]string{
	"result.original_price": "基准原价", "result.estimated_value": "估算残值",
	"result.confidence_low": "区间下限", "result.confidence_high": "区间上限",
	"result.interval_width": "区间宽度", "result.width_ratio": "区间宽比",
	"coefficient.k_time": "时间系数", "coefficient.k_time_adjusted": "调整后时间系数",
	"coefficient.k_hours": "工时系数", "coefficient.k_brand": "品牌系数",
	"coefficient.k_condition": "车况系数", "coefficient.k_market": "市场系数",
	"formula.version": "公式版本", "formula.hash": "公式哈希",
	"formula.lambda_electric": "电动衰减率", "formula.lambda_combustion": "内燃衰减率",
	"formula.decay_anchor": "未来价值锚点", "policy.name": "试算策略", "policy.version": "策略版本",
	"snapshot.hash": "快照哈希", "snapshot.sha7": "快照短哈希",
}

func explanationFactLabel(id string) string {
	if label, ok := explanationFactLabels[id]; ok {
		return label
	}
	return id
}

func explanationOperationKey(cardKey, operationID string) string {
	return "explain:" + cardKey + ":" + operationID
}

func hashExplanationRequest(cardKey string, version int, question string) (string, error) {
	payload, err := json.Marshal(struct {
		CardKey  string `json:"card_key"`
		Version  int    `json:"version"`
		Question string `json:"question"`
	}{cardKey, version, question})
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(payload)
	return hex.EncodeToString(sum[:]), nil
}

func decodeExplanationResult(content string) (*ExplanationModelResult, error) {
	if len(content) > 32*1024 {
		return nil, ErrSuggestionValidation
	}
	decoder := json.NewDecoder(strings.NewReader(content))
	decoder.DisallowUnknownFields()
	var result ExplanationModelResult
	if err := decoder.Decode(&result); err != nil {
		return nil, fmt.Errorf("%w: %v", ErrSuggestionValidation, err)
	}
	var extra any
	if err := decoder.Decode(&extra); !errors.Is(err, io.EOF) {
		return nil, ErrSuggestionValidation
	}
	if result.SchemaVersion != ExplanationSchemaVersion || result.EvidenceIDs == nil || len(result.EvidenceIDs) > 16 {
		return nil, ErrSuggestionValidation
	}
	if result.Action != ExplanationActionExplain && result.Action != ExplanationActionRecalculate {
		return nil, ErrSuggestionValidation
	}
	if _, ok := explanationTemplates[result.TemplateKey]; !ok {
		return nil, ErrSuggestionValidation
	}
	seen := map[string]bool{}
	for _, id := range result.EvidenceIDs {
		if id == "" || len(id) > 160 || seen[id] {
			return nil, ErrSuggestionValidation
		}
		seen[id] = true
	}
	return &result, nil
}

var explanationTemplates = map[string]string{
	"hours_coefficient":     "该卡片记录的工时为 {{input.usage_hours}}；金额、系数与区间只按卡片锁定的原快照解释，不推测其他工时下的估值。",
	"value_interval":        "卡片结果 {{result.estimated_value}}，置信区间 {{result.confidence_low}} 至 {{result.confidence_high}}，区间宽度约为中值的 {{result.width_ratio}}。",
	"interval_width":        "区间宽度由目录基准价与未确认输入决定：本次按 {{candidates.count}} 个目录候选与 {{assumptions.count}} 条假设枚举，宽度约为中值的 {{result.width_ratio}}。",
	"factor_breakdown":      "锁定输入的工时为 {{input.usage_hours}}；结果 {{result.estimated_value}} 由基准原价 {{result.original_price}} 乘以时间系数 {{coefficient.k_time}}（按衰减调整为 {{coefficient.k_time_adjusted}}）、工时系数 {{coefficient.k_hours}}、品牌系数 {{coefficient.k_brand}}、车况系数 {{coefficient.k_condition}}、市场系数 {{coefficient.k_market}} 得到，公式版本 {{formula.version}}。",
	"vehicle_configuration": "解释仅依据卡片已确认的车型、配置与目录匹配结果，不补写未确认参数。",
	"locked_formula":        "本卡片使用公式版本 {{formula.version}}，系数与结果均来自原快照。",
	"assumptions":           "区间和结果只对原卡片的已确认输入与假设成立。",
	"condition_market":      "车况与市场因素只按卡片锁定的系数解释，不代表其他车况或地区的报价。",
}

// explanationTemplateRequirements 每个模板引用的必需事实；缺失即拒绝模型输出，避免模板占位符泄漏。
var explanationTemplateRequirements = map[string][]string{
	"hours_coefficient": {"input.usage_hours"},
	"value_interval":    {"result.estimated_value", "result.confidence_low", "result.confidence_high", "result.width_ratio"},
	"interval_width":    {"result.width_ratio", "assumptions.count", "candidates.count"},
	"factor_breakdown": {"input.usage_hours", "result.original_price", "result.estimated_value", "coefficient.k_time",
		"coefficient.k_time_adjusted", "coefficient.k_hours", "coefficient.k_brand", "coefficient.k_condition",
		"coefficient.k_market", "formula.version"},
	"locked_formula": {"formula.version"},
}

var evidenceIDPattern = regexp.MustCompile(`^[a-z][a-z0-9_]*(\.[a-z][a-z0-9_]*)+$`)

func prepareExplanation(snapshot *explanationSnapshot, model *ExplanationModelResult, counterfactual bool) (*ValuationExplanationResult, error) {
	ids := append([]string{}, model.EvidenceIDs...)
	for _, id := range explanationTemplateRequirements[model.TemplateKey] {
		if !containsString(ids, id) {
			ids = append(ids, id)
		}
	}
	facts := make([]ExplanationFact, 0, len(ids))
	for _, id := range ids {
		if !evidenceIDPattern.MatchString(id) {
			return nil, ErrSuggestionValidation
		}
		fact, ok := snapshot.Facts[id]
		if !ok {
			return nil, ErrSuggestionValidation
		}
		facts = append(facts, fact)
	}
	action := model.Action
	if counterfactual || action == ExplanationActionRecalculate {
		action = ExplanationActionRecalculate
	}
	return &ValuationExplanationResult{SchemaVersion: ExplanationSchemaVersion, Action: action, TemplateKey: model.TemplateKey,
		Template: explanationTemplates[model.TemplateKey], Facts: facts, EvidenceIDs: ids, FormulaVersion: snapshot.FormulaVersion,
		SnapshotID: snapshot.SnapshotID, RequiresUserAction: action == ExplanationActionRecalculate, CardVersion: snapshot.Version}, nil
}

var (
	hoursChangeWords     = []string{"改", "换", "调整", "调成", "调到", "变成", "变为", "设为", "设成", "提高到", "降到"}
	hoursDifferenceWords = []string{"差价", "价差", "差额", "差多少", "差几", "相差", "少多少钱", "多多少钱", "少花", "多花"}
	hoursHypothesisWords = []string{"如果", "假如", "要是", "假设", "反事实"}
	hoursFromToPattern   = regexp.MustCompile(`从\s*\d[\d,]*(?:\.\d+)?\s*(?:小时|工时|h|H)?\s*(?:到|至)\s*\d`)
	hoursNumberPattern   = regexp.MustCompile(`\d[\d,]*(?:\.\d+)?`)
)

// counterfactualHoursQuestion 只把明确的改/换/调整工时、两个工时数值的差价或区间意图、
// 带数值的假设式提问判为反事实；纯“为什么工时这么多 / 这么少”仍是解释，不触发重算。
func counterfactualHoursQuestion(question string) bool {
	if !strings.Contains(question, "工时") {
		return false
	}
	if containsAny(question, hoursChangeWords) || containsAny(question, hoursDifferenceWords) {
		return true
	}
	if containsAny(question, hoursHypothesisWords) && hoursNumberPattern.MatchString(question) {
		return true
	}
	return hoursFromToPattern.MatchString(question)
}

func containsAny(text string, words []string) bool {
	for _, word := range words {
		if strings.Contains(text, word) {
			return true
		}
	}
	return false
}

func sortedFactIDs(facts map[string]ExplanationFact) []string {
	ids := make([]string, 0, len(facts))
	for id := range facts {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	return ids
}

const explanationSystemPrompt = `你是估值卡片解释器。只返回 schema_version、action、template_key、evidence_ids 四个键。禁止自由正文、金额、区间、配置、工时、假设或公式版本。evidence_ids 只能从用户消息列出的真实事实键选择。区间宽度与候选/假设影响用 interval_width；正式卡逐项系数用 factor_breakdown；工时反事实必须 action=recalculate，由用户确认后按原快照重算，不推测差价。草稿卡问题按卡内快照事实解释，不假设当前草稿内容。`
