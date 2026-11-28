package valuation

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"strings"
	"time"

	"github.com/cloudwego/eino/schema"
	"go.uber.org/zap"

	"forklift-training/internal/aiassistant"
)

const (
	suggestionDraftLimit   = 8
	maxSuggestionTextRunes = 4000
)

type suggestionRunner struct {
	owner      *ConversationService
	model      aiassistant.ModelPort
	repository SuggestionRepository
	config     SuggestionConfig
	logger     *zap.Logger
	now        func() time.Time
}

func (s *ConversationService) EnableSuggestions(model aiassistant.ModelPort, repository SuggestionRepository, config SuggestionConfig, logger *zap.Logger) {
	config = normalizeSuggestionConfig(config)
	if logger == nil {
		logger = zap.NewNop()
	}
	s.suggestions = &suggestionRunner{owner: s, model: model, repository: repository, config: config, logger: logger, now: s.now}
}

func normalizeSuggestionConfig(config SuggestionConfig) SuggestionConfig {
	if config.DraftLimit <= 0 || config.DraftLimit > suggestionDraftLimit {
		config.DraftLimit = suggestionDraftLimit
	}
	if config.IPLimit <= 0 {
		config.IPLimit = 20
	}
	if config.IPWindow <= 0 {
		config.IPWindow = time.Hour
	}
	if config.GlobalLimit <= 0 {
		config.GlobalLimit = 500
	}
	if config.GlobalWindow <= 0 {
		config.GlobalWindow = 24 * time.Hour
	}
	if config.Timeout <= 0 {
		config.Timeout = 45 * time.Second
	}
	return config
}

func (s *ConversationService) RequestSuggestions(ctx context.Context, draftID string, req *SuggestDraftRequest, userID int, clientIP string) (*ConversationOperation, error) {
	runner := s.suggestions
	if runner == nil || runner.model == nil || runner.repository == nil || runner.config.HMACKey == "" {
		return nil, ErrSuggestionUnavailable
	}
	id, err := normalizeDraftID(draftID)
	if err != nil {
		return nil, err
	}
	operationID, err := normalizeOperationID(req.OperationID)
	if err != nil {
		return nil, err
	}
	if req.BaseVersion <= 0 {
		return nil, fmt.Errorf("%w: base_version must be positive", ErrInvalidOperation)
	}
	text := strings.TrimSpace(req.Text)
	if text == "" || len([]rune(text)) > maxSuggestionTextRunes {
		return nil, fmt.Errorf("%w: text must contain 1-4000 characters", ErrInvalidOperation)
	}
	ip := net.ParseIP(strings.TrimSpace(clientIP))
	if ip == nil {
		return nil, ErrInvalidClientIP
	}
	requestHash, err := hashSuggestionRequest(req.BaseVersion, text)
	if err != nil {
		return nil, err
	}
	operationKey := suggestionOperationKey(id, operationID)
	if existing, err := runner.repository.GetSuggestion(ctx, operationKey); err == nil {
		if existing.RequestHash != requestHash {
			return nil, ErrOperationHashConflict
		}
		return existing, nil
	} else if !errors.Is(err, ErrOperationNotFound) {
		return nil, err
	}

	base, err := s.drafts.GetDraft(ctx, id, s.now())
	if err != nil {
		return nil, err
	}
	if base.Draft.Version != req.BaseVersion {
		return nil, ErrDraftVersionConflict
	}
	if base.Draft.Status != DraftStatusEditing {
		return nil, ErrDraftNotEditing
	}
	catalog, err := s.buildSuggestionCatalog(ctx, &base.Draft)
	if err != nil {
		return nil, fmt.Errorf("%w: catalog: %v", ErrSuggestionValidation, err)
	}
	messages, err := buildSuggestionMessages(&base.Draft, text, catalog)
	if err != nil {
		return nil, err
	}
	now := s.now()
	operation := &ConversationOperation{
		ID: operationKey, Kind: ConversationOperationKindSuggestion, OperationID: operationKey,
		DraftID: id, BaseVersion: req.BaseVersion, Status: OperationStatusPending, RequestHash: requestHash,
		CreatedAt: now, UpdatedAt: now,
	}
	if userID > 0 {
		operation.FirstAcceptedUserID = &userID
	}
	reservation := SuggestionReservation{
		DraftKey: id, DraftLimit: runner.config.DraftLimit,
		IPHash: suggestionIPHash(runner.config.HMACKey, ip), IPLimit: runner.config.IPLimit, IPWindow: runner.config.IPWindow,
		GlobalLimit: runner.config.GlobalLimit, GlobalWindow: runner.config.GlobalWindow, CallKey: operationKey,
	}
	accepted, inserted, err := runner.repository.AcceptSuggestion(ctx, operation, reservation, now)
	if err != nil {
		return nil, err
	}
	if accepted.RequestHash != requestHash {
		return nil, ErrOperationHashConflict
	}
	if !inserted {
		return accepted, nil
	}
	runner.logger.Info("估值草稿建议已受理",
		zap.String("operation_key", operationKey), zap.String("status", accepted.Status),
		zap.Int("base_version", req.BaseVersion), zap.Bool("authenticated", userID > 0))
	go runner.run(context.Background(), operationKey, messages, &base.Draft)
	return accepted, nil
}

func (r *suggestionRunner) run(ctx context.Context, operationKey string, messages []*schema.Message, base *ValuationDraft) {
	ctx, cancel := context.WithTimeout(ctx, r.config.Timeout)
	defer cancel()
	content, err := r.model.CompleteStructured(ctx, aiassistant.FeatureValuationDraftSuggest, messages,
		valuationSuggestionSchemaJSON, aiassistant.StructuredCompleteOptions{
			CompleteOptions: aiassistant.CompleteOptions{MaxTokens: 1800, Temperature: 0.1},
			Timeout:         r.config.Timeout,
		})
	if err != nil {
		code := ErrorCodeSuggestionUnavailable
		if errors.Is(err, context.DeadlineExceeded) || errors.Is(err, context.Canceled) {
			code = ErrorCodeSuggestionTimeout
		}
		r.fail(operationKey, code)
		return
	}
	modelResult, err := decodeSuggestionResult(content)
	if err != nil {
		r.fail(operationKey, ErrorCodeSuggestionValidation)
		return
	}
	now := r.now()
	next, result, err := r.owner.prepareSuggestion(ctx, base, modelResult, now)
	if err != nil {
		r.fail(operationKey, ErrorCodeSuggestionValidation)
		return
	}
	status, err := r.repository.CompleteSuggestion(ctx, operationKey, next, result, now)
	if err != nil {
		r.fail(operationKey, ErrorCodeSuggestionUnavailable)
		return
	}
	r.logger.Info("估值草稿建议完成", zap.String("operation_key", operationKey), zap.String("status", status))
}

func (r *suggestionRunner) fail(operationKey, code string) {
	_ = r.repository.FailSuggestion(context.Background(), operationKey, code, r.now())
	r.logger.Info("估值草稿建议失败", zap.String("operation_key", operationKey), zap.String("error_code", code))
}

func suggestionOperationKey(draftID, operationID string) string {
	return "op:" + draftID + ":" + operationID
}

func hashSuggestionRequest(baseVersion int, text string) (string, error) {
	payload, err := json.Marshal(struct {
		BaseVersion int    `json:"base_version"`
		Text        string `json:"text"`
	}{baseVersion, text})
	if err != nil {
		return "", fmt.Errorf("hash valuation suggestion: %w", err)
	}
	sum := sha256.Sum256(payload)
	return hex.EncodeToString(sum[:]), nil
}

func suggestionIPHash(secret string, ip net.IP) string {
	mac := hmac.New(sha256.New, []byte(secret))
	_, _ = mac.Write([]byte(ip.String()))
	return hex.EncodeToString(mac.Sum(nil))
}
