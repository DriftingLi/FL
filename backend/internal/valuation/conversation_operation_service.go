package valuation

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
)

func (s *ConversationService) ExecuteOperation(ctx context.Context, req *ConversationOperationRequest, userID int) (*ConversationOperation, error) {
	operationID, err := normalizeOperationID(req.OperationID)
	if err != nil {
		return nil, err
	}
	if _, err := normalizeDraftID(req.DraftID); err != nil {
		return nil, fmt.Errorf("%w: invalid draft_id", ErrInvalidOperation)
	}
	if req.DraftVersion <= 0 {
		return nil, fmt.Errorf("%w: draft_version must be positive", ErrInvalidOperation)
	}
	hash, err := hashOperationRequest(req.DraftID, req.DraftVersion, req.Request)
	if err != nil {
		return nil, err
	}
	existing, err := s.operations.GetOperation(ctx, operationID)
	if err == nil {
		if existing.RequestHash != hash {
			return nil, ErrOperationHashConflict
		}
		if existing.Status == OperationStatusFailed {
			return nil, operationError(existing.ErrorCode)
		}
		return s.operationResponse(ctx, existing)
	}
	if !errors.Is(err, ErrOperationNotFound) {
		return nil, err
	}

	now := s.now()
	operation := &ConversationOperation{
		OperationID: operationID, DraftID: req.DraftID, DraftVersion: req.DraftVersion,
		Kind: ConversationOperationKindEvaluation, Status: OperationStatusPending, RequestHash: hash, CreatedAt: now, UpdatedAt: now,
	}
	if userID > 0 {
		operation.FirstAcceptedUserID = &userID
	}
	var evaluationErr error
	accepted, inserted, err := s.operations.AcceptOperation(ctx, operation,
		func(writer OperationEvaluationWriter) (int64, string, error) {
			result, err := s.evaluateOperationDraft(ctx, req)
			if err != nil {
				evaluationErr = err
				return 0, errorCodeFor(err), err
			}
			evaluationID, err := writer.CreateEvaluation(ctx, evaluationParams(result, userID))
			return evaluationID, "", err
		})
	if err != nil {
		return nil, err
	}
	if accepted.RequestHash != hash {
		return nil, ErrOperationHashConflict
	}
	if accepted.Status == OperationStatusFailed {
		if inserted {
			return nil, evaluationErr
		}
		return nil, operationError(accepted.ErrorCode)
	}
	if inserted {
		// 草稿是临时展示状态；正式事实已由 operation/evaluation 事务保证，标记失败不得撤回已提交结果。
		_ = s.markDraftSubmitted(ctx, req.DraftID, req.DraftVersion)
	}
	return s.operationResponse(ctx, accepted)
}

func (s *ConversationService) GetOperation(ctx context.Context, operationID string) (*ConversationOperation, error) {
	id, err := normalizeConversationOperationKey(operationID)
	if err != nil {
		return nil, err
	}
	if strings.HasPrefix(id, "op:") && s.suggestions != nil {
		operation, err := s.suggestions.repository.GetSuggestion(ctx, id)
		if err == nil {
			if operation.Status == OperationStatusPending {
				_ = s.suggestions.repository.ExpireSuggestion(ctx, id, s.now().Add(-s.suggestions.config.Timeout), s.now())
				return s.suggestions.repository.GetSuggestion(ctx, id)
			}
			return operation, nil
		}
		if !errors.Is(err, ErrOperationNotFound) {
			return nil, err
		}
	}
	operation, err := s.operations.GetOperation(ctx, id)
	if err != nil {
		return nil, err
	}
	if operation.Kind == ConversationOperationKindSuggestion {
		return operation, nil
	}
	if operation.Status == OperationStatusFailed {
		return operation, nil
	}
	return s.operationResponse(ctx, operation)
}

func (s *ConversationService) evaluateOperationDraft(ctx context.Context, req *ConversationOperationRequest) (*EvaluationResult, error) {
	draft, err := s.GetDraft(ctx, req.DraftID)
	if err != nil {
		return nil, err
	}
	if draft.Version != req.DraftVersion {
		return nil, ErrDraftVersionConflict
	}
	if draft.Status != DraftStatusEditing {
		return nil, ErrDraftNotEditing
	}
	if !draftComplete(*draft) {
		return nil, ErrDraftIncomplete
	}
	evaluationRequest, err := evaluationRequestFromDraft(draft)
	if err != nil {
		return nil, err
	}
	matches, err := sameEvaluationRequest(*evaluationRequest, req.Request)
	if err != nil {
		return nil, err
	}
	if !matches {
		return nil, ErrOperationDraftMismatch
	}
	if err := evaluationRequest.Validate(); err != nil {
		return nil, fmt.Errorf("%w: %v", ErrEvaluationRejected, err)
	}
	result, err := s.valuation.Evaluate(ctx, evaluationRequest)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrEvaluationRejected, err)
	}
	return result, nil
}

func (s *ConversationService) operationResponse(ctx context.Context, operation *ConversationOperation) (*ConversationOperation, error) {
	result := *operation
	result.Evaluation = nil
	if operation.Kind == ConversationOperationKindSuggestion || operation.Status != OperationStatusSucceeded {
		return &result, nil
	}
	if operation.EvaluationID <= 0 {
		return nil, ErrOperationResultMissing
	}
	evaluation, err := s.evaluations.GetEvaluation(ctx, operation.EvaluationID)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrOperationResultMissing, err)
	}
	RebuildDerivedFromDetail(evaluation)
	if evaluation.Suggestions == nil {
		evaluation.Suggestions = []string{}
	}
	result.Evaluation = evaluation
	return &result, nil
}

func (s *ConversationService) markDraftSubmitted(ctx context.Context, id string, version int) error {
	for attempt := 0; attempt < 3; attempt++ {
		stored, err := s.drafts.GetDraft(ctx, id, s.now())
		if err != nil {
			return err
		}
		if stored.Draft.Version != version || stored.Draft.Status != DraftStatusEditing {
			return nil
		}
		draft, err := cloneDraft(&stored.Draft)
		if err != nil {
			return err
		}
		now := s.now()
		draft.Status = DraftStatusSubmitted
		draft.UpdatedAt = now
		draft.ExpiresAt = now.Add(DraftIdleTTL)
		saved, err := s.drafts.SaveDraft(ctx, id, stored.Raw, &storedDraft{Draft: *draft, Previous: stored.Previous}, draft.ExpiresAt, now)
		if err != nil {
			return err
		}
		if saved {
			return nil
		}
	}
	return ErrDraftVersionConflict
}

func hashOperationRequest(draftID string, draftVersion int, request EvaluationRequest) (string, error) {
	payload := struct {
		DraftID      string            `json:"draft_id"`
		DraftVersion int               `json:"draft_version"`
		Request      EvaluationRequest `json:"request"`
	}{draftID, draftVersion, request}
	value, err := json.Marshal(payload)
	if err != nil {
		return "", fmt.Errorf("hash valuation operation: %w", err)
	}
	sum := sha256.Sum256(value)
	return hex.EncodeToString(sum[:]), nil
}

func sameEvaluationRequest(left, right EvaluationRequest) (bool, error) {
	a, err := json.Marshal(left)
	if err != nil {
		return false, err
	}
	b, err := json.Marshal(right)
	if err != nil {
		return false, err
	}
	return bytes.Equal(a, b), nil
}

func evaluationRequestFromDraft(draft *ValuationDraft) (*EvaluationRequest, error) {
	request := &EvaluationRequest{}
	for _, spec := range draftFieldSpecs {
		field := draft.Fields[spec.name]
		if field.State != DraftFieldValue || field.Value == nil {
			return nil, ErrDraftIncomplete
		}
		if err := assignDraftValue(spec.requestPtr(request), field.Value); err != nil {
			return nil, fmt.Errorf("%w: %s", ErrInvalidDraftField, spec.name)
		}
	}
	return request, nil
}

func assignDraftValue(target, value any) error {
	switch out := target.(type) {
	case *string:
		v, ok := value.(string)
		if !ok {
			return errors.New("not a string")
		}
		*out = v
	case *bool:
		v, ok := value.(bool)
		if !ok {
			return errors.New("not a boolean")
		}
		*out = v
	case *float64:
		v, ok := draftNumber(value)
		if !ok {
			return errors.New("not a number")
		}
		*out = v
	case *int:
		v, ok := draftNumber(value)
		if !ok || v != float64(int(v)) {
			return errors.New("not an integer")
		}
		*out = int(v)
	default:
		return errors.New("unsupported target")
	}
	return nil
}

func normalizeOperationID(id string) (string, error) {
	id = strings.TrimSpace(id)
	if id == "" || len(id) > 128 || strings.ContainsAny(id, "/?#") {
		return "", fmt.Errorf("%w: invalid operation_id", ErrInvalidOperation)
	}
	return id, nil
}

func normalizeConversationOperationKey(id string) (string, error) {
	id = strings.TrimSpace(id)
	if id == "" || len(id) > 320 || strings.ContainsAny(id, "/?#") {
		return "", fmt.Errorf("%w: invalid operation_id", ErrInvalidOperation)
	}
	return id, nil
}

func errorCodeFor(err error) string {
	for _, mapping := range operationErrorMappings {
		if errors.Is(err, mapping.sentinel) {
			return mapping.code
		}
	}
	return ""
}

func operationError(code string) error {
	for _, mapping := range operationErrorMappings {
		if mapping.code == code {
			return mapping.sentinel
		}
	}
	return ErrInvalidOperation
}
