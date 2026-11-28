package valuation

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"regexp"
	"sort"
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
	card.OperationID = strings.TrimSpace(card.OperationID)
	card.RequestID = strings.TrimSpace(card.RequestID)
	card.SnapshotID = strings.TrimSpace(card.SnapshotID)
	snapshot, err := s.loadExplanationCard(ctx, &card, userID)
	if err != nil {
		return nil, err
	}
	if snapshot.Version != req.DraftVersion {
		return nil, ErrDraftVersionConflict
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
		CardKind: card.Kind, CardKey: snapshot.CardKey, DraftVersion: req.DraftVersion, BaseVersion: req.DraftVersion,
		Status: OperationStatusPending, RequestHash: hash, CreatedAt: now, UpdatedAt: now}
	if userID > 0 {
		op.FirstAcceptedUserID = &userID
	}
	if card.Kind == ExplanationCardDraft {
		op.DraftID = snapshot.CardKey
	}
	reservation := SuggestionReservation{DraftKey: snapshot.CardKey, DraftLimit: runner.config.DraftLimit,
		IPHash: suggestionIPHash(runner.config.HMACKey, ip), IPLimit: runner.config.IPLimit, IPWindow: runner.config.IPWindow,
		GlobalLimit: runner.config.GlobalLimit, GlobalWindow: runner.config.GlobalWindow, CallKey: operationKey}
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
	messages := []*schema.Message{schema.SystemMessage(explanationSystemPrompt), schema.UserMessage(question + "\n卡片版本=" + fmt.Sprint(req.DraftVersion) + "\n事实键=" + strings.Join(sortedFactIDs(snapshot.Facts), ","))}
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
		if _, err := s.GetDraft(ctx, op.CardKey); err != nil {
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

const (
	ExplanationCardDraft  = "draft"
	ExplanationCardFormal = "formal"
	ExplanationCardTrial  = "trial"
)

func (s *ConversationService) loadExplanationCard(ctx context.Context, card *ExplanationCard, userID int) (*explanationSnapshot, error) {
	switch card.Kind {
	case ExplanationCardDraft:
		id, err := normalizeDraftID(card.DraftID)
		if err != nil || card.OperationID != "" || card.RequestID != "" || card.SnapshotID != "" || card.EvaluationID != 0 {
			return nil, fmt.Errorf("%w: invalid draft card", ErrInvalidOperation)
		}
		draft, err := s.GetDraft(ctx, id)
		if err != nil {
			return nil, err
		}
		snapshot := &explanationSnapshot{CardKey: id, CardKind: card.Kind, SnapshotID: id, Version: draft.Version, Facts: map[string]ExplanationFact{}}
		for _, name := range requiredDraftFields {
			field := draft.Fields[name]
			value := field.Value
			if field.State == DraftFieldRange && field.Range != nil {
				value = map[string]any{"min": field.Range.Min, "max": field.Range.Max}
			}
			snapshot.Facts["input."+name] = ExplanationFact{ID: "input." + name, Label: name, Value: value}
		}
		return snapshot, nil
	case ExplanationCardFormal:
		id, err := normalizeConversationOperationKey(card.OperationID)
		if err != nil || card.DraftID != "" || card.RequestID != "" || card.SnapshotID != "" || (card.EvaluationID <= 0 && userID == 0) {
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
		if s.trialCards == nil || card.RequestID == "" || card.SnapshotID == "" || card.DraftID != "" || card.OperationID != "" || card.EvaluationID != 0 {
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

func formalExplanationSnapshot(evaluation *EvaluationDetail) *explanationSnapshot {
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
	for id, value := range map[string]any{"result.original_price": evaluation.OriginalPrice, "coefficient.k_time": evaluation.KTime,
		"coefficient.k_time_adjusted": evaluation.KTimeAdjusted, "coefficient.k_hours": evaluation.KHours, "coefficient.k_brand": evaluation.KBrand,
		"coefficient.k_condition": evaluation.KCondition, "coefficient.k_market": evaluation.KMarket,
		"result.estimated_value": evaluation.EstimatedValue, "result.confidence_low": evaluation.ConfidenceLow, "result.confidence_high": evaluation.ConfidenceHigh,
		"formula.version": evaluation.FormulaVersion} {
		snapshot.Facts[id] = ExplanationFact{ID: id, Label: id, Value: value}
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
	snapshot.Facts["formula.version"] = ExplanationFact{ID: "formula.version", Label: "公式版本", Value: card.Response.Snapshot.FormulaVersion}
	snapshot.Facts["formula.hash"] = ExplanationFact{ID: "formula.hash", Label: "公式哈希", Value: card.Response.Snapshot.FormulaHash}
	snapshot.Facts["snapshot.hash"] = ExplanationFact{ID: "snapshot.hash", Label: "快照哈希", Value: card.SnapshotID}
	if interval := card.Response.Interval; interval != nil {
		for name, value := range map[string]any{"result.estimated_value": interval.Mid, "result.confidence_low": interval.Min, "result.confidence_high": interval.Max} {
			snapshot.Facts[name] = ExplanationFact{ID: name, Label: name, Value: value}
		}
	}
	return snapshot
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
	"hours_coefficient":     "该卡片记录的工时为 {{input.usage_hours}}，对应使用强度系数为 {{coefficient.k_hours}}；卡片结果与区间均按评估时点锁定。",
	"value_interval":        "卡片记录的结果为 {{result.estimated_value}}，置信区间为 {{result.confidence_low}} 至 {{result.confidence_high}}。",
	"vehicle_configuration": "解释仅依据卡片已确认的车型、配置与目录匹配结果，不补写未确认参数。",
	"locked_formula":        "本卡片使用公式版本 {{formula.version}}，系数与结果均来自原快照。",
	"assumptions":           "区间和结果只对原卡片的已确认输入与假设成立。",
	"condition_market":      "车况与市场因素只按卡片锁定的系数解释，不代表其他车况或地区的报价。",
}

var evidenceIDPattern = regexp.MustCompile(`^[a-z][a-z0-9_]*(\.[a-z][a-z0-9_]*)+$`)

func prepareExplanation(snapshot *explanationSnapshot, model *ExplanationModelResult, counterfactual bool) (*ValuationExplanationResult, error) {
	required := []string{"formula.version"}
	switch model.TemplateKey {
	case "hours_coefficient":
		required = append(required, "input.usage_hours")
		if _, ok := snapshot.Facts["coefficient.k_hours"]; ok {
			required = append(required, "coefficient.k_hours")
		}
	case "value_interval":
		required = append(required, "result.estimated_value", "result.confidence_low", "result.confidence_high")
	}
	ids := append([]string{}, model.EvidenceIDs...)
	for _, id := range required {
		if _, ok := snapshot.Facts[id]; ok && !containsString(ids, id) {
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

func counterfactualHoursQuestion(question string) bool {
	if !strings.Contains(question, "工时") {
		return false
	}
	for _, word := range []string{"如果", "假如", "要是", "变成", "改为", "改成", "反事实", "少", "多"} {
		if strings.Contains(question, word) {
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

const explanationSystemPrompt = `你是估值卡片解释器。只返回 schema_version、action、template_key、evidence_ids 四个键。禁止自由正文、金额、区间、配置、工时、假设或公式版本。evidence_ids 只能从用户消息列出的真实事实键选择。工时反事实必须 action=recalculate，由用户确认后按原快照重算，不推测差价。`
