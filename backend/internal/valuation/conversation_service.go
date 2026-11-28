package valuation

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"math"
	"strings"
	"time"
)

const DraftIdleTTL = 30 * time.Minute

type ConversationService struct {
	drafts      DraftRepository
	operations  OperationRepository
	valuation   *Service
	evaluations EvaluationStore
	suggestions *suggestionRunner
	now         func() time.Time
}

func NewConversationService(drafts DraftRepository, operations OperationRepository, valuation *Service, evaluations EvaluationStore, now func() time.Time) *ConversationService {
	if now == nil {
		now = time.Now
	}
	return &ConversationService{drafts: drafts, operations: operations, valuation: valuation, evaluations: evaluations, now: now}
}

func (s *ConversationService) CreateDraft(ctx context.Context, req *CreateDraftRequest) (*ValuationDraft, error) {
	now := s.now()
	id, err := newDraftID()
	if err != nil {
		return nil, err
	}
	fields := make(map[string]DraftField, len(requiredDraftFields))
	for _, name := range requiredDraftFields {
		fields[name] = DraftField{State: DraftFieldMissing}
	}
	if err := applyDraftFields(fields, req.Fields); err != nil {
		return nil, err
	}
	draft := ValuationDraft{
		ID: id, Version: 1, Status: DraftStatusEditing, Fields: fields,
		Supplemental: nonNilMap(req.Supplemental), UpdatedAt: now, ExpiresAt: now.Add(DraftIdleTTL),
	}
	draft.CanSubmit = draftComplete(draft)
	if err := s.drafts.CreateDraft(ctx, &storedDraft{Draft: draft}, draft.ExpiresAt); err != nil {
		return nil, fmt.Errorf("create valuation draft: %w", err)
	}
	return &draft, nil
}

func (s *ConversationService) GetDraft(ctx context.Context, id string) (*ValuationDraft, error) {
	stored, err := s.drafts.GetDraft(ctx, id, s.now())
	if err != nil {
		return nil, err
	}
	return &stored.Draft, nil
}

func (s *ConversationService) PatchDraft(ctx context.Context, id string, req *PatchDraftRequest) (*ValuationDraft, error) {
	if req.BaseVersion == nil || *req.BaseVersion <= 0 {
		return nil, fmt.Errorf("%w: base_version must be a positive integer", ErrInvalidDraftField)
	}
	return s.mutateDraft(ctx, id, *req.BaseVersion, func(draft *ValuationDraft) error {
		if draft.Status != DraftStatusEditing {
			return ErrDraftNotEditing
		}
		if err := applyDraftFields(draft.Fields, req.Fields); err != nil {
			return err
		}
		if err := s.revalidateDraftDownstream(ctx, draft, req.Fields); err != nil {
			return err
		}
		if req.Supplemental != nil {
			draft.Supplemental = nonNilMap(*req.Supplemental)
		}
		draft.CanSubmit = draftComplete(*draft)
		return nil
	}, false)
}

func (s *ConversationService) UndoDraft(ctx context.Context, id string, baseVersion *int) (*ValuationDraft, error) {
	if baseVersion == nil {
		draft, err := s.GetDraft(ctx, id)
		if err != nil {
			return nil, err
		}
		baseVersion = &draft.Version
	}
	if *baseVersion <= 0 {
		return nil, fmt.Errorf("%w: base_version must be a positive integer", ErrInvalidDraftField)
	}
	return s.mutateDraft(ctx, id, *baseVersion, func(draft *ValuationDraft) error {
		return nil
	}, true)
}

func (s *ConversationService) DeleteDraft(ctx context.Context, id string) error {
	return s.drafts.DeleteDraft(ctx, id)
}

func (s *ConversationService) mutateDraft(ctx context.Context, id string, baseVersion int, mutate func(*ValuationDraft) error, undo bool) (*ValuationDraft, error) {
	for attempt := 0; attempt < 3; attempt++ {
		stored, err := s.drafts.GetDraft(ctx, id, s.now())
		if err != nil {
			return nil, err
		}
		if stored.Draft.Version != baseVersion {
			return nil, ErrDraftVersionConflict
		}
		if undo && stored.Previous == nil {
			return nil, ErrDraftNothingToUndo
		}
		previous, err := cloneDraft(&stored.Draft)
		if err != nil {
			return nil, err
		}
		draft, err := cloneDraft(&stored.Draft)
		if err != nil {
			return nil, err
		}
		if undo {
			draft = stored.Previous
		}
		if err := mutate(draft); err != nil {
			return nil, err
		}
		now := s.now()
		draft.ID = id
		draft.Version = baseVersion + 1
		draft.Status = DraftStatusEditing
		draft.UpdatedAt = now
		draft.ExpiresAt = now.Add(DraftIdleTTL)
		draft.CanUndo = !undo
		draft.CanSubmit = draftComplete(*draft)
		next := &storedDraft{Draft: *draft}
		if !undo {
			next.Previous = previous
		}
		saved, err := s.drafts.SaveDraft(ctx, id, stored.Raw, next, draft.ExpiresAt, now)
		if err != nil {
			return nil, err
		}
		if saved {
			return draft, nil
		}
	}
	return nil, ErrDraftVersionConflict
}

func newDraftID() (string, error) {
	buf := make([]byte, 32)
	if _, err := rand.Read(buf); err != nil {
		return "", fmt.Errorf("generate valuation draft id: %w", err)
	}
	return base64.RawURLEncoding.EncodeToString(buf), nil
}

func applyDraftFields(fields map[string]DraftField, patch map[string]DraftField) error {
	for name, field := range patch {
		if !isRequiredDraftField(name) {
			return fmt.Errorf("%w: unknown field %q", ErrInvalidDraftField, name)
		}
		if err := validateDraftField(name, field); err != nil {
			return err
		}
		fields[name] = field
	}
	return nil
}

func validateDraftField(name string, field DraftField) error {
	invalid := func(reason string) error {
		return fmt.Errorf("%w: %s %s", ErrInvalidDraftField, name, reason)
	}
	switch field.State {
	case DraftFieldMissing, DraftFieldUnknown:
		return nil
	case DraftFieldValue:
		if field.Value == nil {
			return invalid("value state requires a value")
		}
		return validateDraftValueType(name, field.Value)
	case DraftFieldRange:
		if field.Range == nil || field.Range.Min < 0 || field.Range.Max < field.Range.Min {
			return invalid("range requires 0 <= min <= max")
		}
		if !draftFieldSpecByName[name].allowRange {
			return invalid("field does not support range")
		}
		return nil
	case DraftFieldCandidate:
		if len(field.Candidates) == 0 {
			return invalid("candidate state requires candidates")
		}
		for _, candidate := range field.Candidates {
			if candidate.Value == nil {
				return invalid("candidate value cannot be null")
			}
			if err := validateDraftValueType(name, candidate.Value); err != nil {
				return err
			}
		}
		return nil
	default:
		return invalid("has unknown state")
	}
}

func validateDraftValueType(name string, value any) error {
	kind := draftFieldKind(name)
	switch kind {
	case "string":
		if _, ok := value.(string); !ok {
			return fmt.Errorf("%w: %s must be a string", ErrInvalidDraftField, name)
		}
	case "bool":
		if _, ok := value.(bool); !ok {
			return fmt.Errorf("%w: %s must be a boolean", ErrInvalidDraftField, name)
		}
	case "number":
		if _, ok := draftNumber(value); !ok {
			return fmt.Errorf("%w: %s must be a number", ErrInvalidDraftField, name)
		}
	case "integer":
		n, ok := draftNumber(value)
		if !ok || math.Trunc(n) != n {
			return fmt.Errorf("%w: %s must be an integer", ErrInvalidDraftField, name)
		}
	}
	return nil
}

func draftNumber(value any) (float64, bool) {
	switch n := value.(type) {
	case float64:
		return n, true
	case float32:
		return float64(n), true
	case int:
		return float64(n), true
	case int64:
		return float64(n), true
	case json.Number:
		v, err := n.Float64()
		return v, err == nil
	default:
		return 0, false
	}
}

func draftFieldKind(name string) string {
	return draftFieldSpecByName[name].kind
}

func isRequiredDraftField(name string) bool {
	_, ok := draftFieldSpecByName[name]
	return ok
}

func draftComplete(draft ValuationDraft) bool {
	for _, name := range requiredDraftFields {
		if draft.Fields[name].State != DraftFieldValue || draft.Fields[name].Value == nil {
			return false
		}
	}
	return true
}

func nonNilMap(values map[string]any) map[string]any {
	if values == nil {
		return map[string]any{}
	}
	return values
}

func cloneDraft(draft *ValuationDraft) (*ValuationDraft, error) {
	value, err := json.Marshal(draft)
	if err != nil {
		return nil, err
	}
	var cloned ValuationDraft
	if err := json.Unmarshal(value, &cloned); err != nil {
		return nil, err
	}
	return &cloned, nil
}

func normalizeDraftID(id string) (string, error) {
	id = strings.TrimSpace(id)
	if id == "" || len(id) > 128 || strings.ContainsAny(id, "/?#") {
		return "", fmt.Errorf("%w: invalid draft id", ErrInvalidOperation)
	}
	return id, nil
}
