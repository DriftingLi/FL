package valuation

import (
	"context"
	"time"
)

const (
	ConversationOperationKindEvaluation = "evaluation"
	ConversationOperationKindSuggestion = "suggestion"
	SuggestionSchemaVersion             = 1
)

type SuggestDraftRequest struct {
	OperationID string `json:"operation_id"`
	BaseVersion int    `json:"base_version"`
	Text        string `json:"text"`
}

type SuggestionPatch struct {
	Field         string          `json:"field"`
	State         DraftFieldState `json:"state"`
	Value         any             `json:"value,omitempty" extensions:"x-optional"`
	Range         *DraftRange     `json:"range,omitempty" extensions:"x-optional"`
	CandidateKeys []string        `json:"candidate_keys,omitempty" extensions:"x-optional" nullability:"nonnil"`
	Evidence      []string        `json:"evidence,omitempty" extensions:"x-optional" nullability:"nonnil"`
}

type SuggestionModelResult struct {
	SchemaVersion int               `json:"schema_version"`
	Patches       []SuggestionPatch `json:"patches"`
	Supplements   map[string]string `json:"supplements"`
	Questions     []string          `json:"questions"`
}

type ValuationSuggestionResult struct {
	SchemaVersion int                   `json:"schema_version"`
	Patches       map[string]DraftField `json:"patches" nullability:"nonnil"`
	Supplements   map[string]string     `json:"supplements" nullability:"nonnil"`
	Questions     []string              `json:"questions" nullability:"nonnil"`
	DraftVersion  int                   `json:"draft_version"`
}

type SuggestionConfig struct {
	DraftLimit   int
	IPLimit      int
	IPWindow     time.Duration
	GlobalLimit  int
	GlobalWindow time.Duration
	Timeout      time.Duration
	HMACKey      string
}

type SuggestionReservation struct {
	DraftKey     string
	DraftUsed    int
	DraftLimit   int
	IPHash       string
	IPUsed       int
	IPLimit      int
	IPWindow     time.Duration
	GlobalUsed   int
	GlobalLimit  int
	GlobalWindow time.Duration
	CallKey      string
}

type SuggestionRepository interface {
	GetSuggestion(ctx context.Context, operationID string) (*ConversationOperation, error)
	AcceptSuggestion(ctx context.Context, operation *ConversationOperation, reservation SuggestionReservation, now time.Time) (*ConversationOperation, bool, error)
	CompleteSuggestion(ctx context.Context, operationID string, next *storedDraft, result *ValuationSuggestionResult, now time.Time) (string, error)
	FailSuggestion(ctx context.Context, operationID, code string, now time.Time) error
	ExpireSuggestion(ctx context.Context, operationID string, olderThan time.Time, now time.Time) error
}
