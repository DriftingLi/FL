package valuation

import (
	"context"
	"time"
)

const (
	ConversationOperationKindExplanation = "explanation"
	ExplanationSchemaVersion             = 1
	ExplanationActionExplain             = "explain"
	ExplanationActionRecalculate         = "recalculate"
)

type ExplanationCard struct {
	Kind         string `json:"kind"`
	DraftID      string `json:"draft_id,omitempty" extensions:"x-optional"`
	OperationID  string `json:"operation_id,omitempty" extensions:"x-optional"`
	EvaluationID int64  `json:"evaluation_id,omitempty" extensions:"x-optional"`
	RequestID    string `json:"request_id,omitempty" extensions:"x-optional"`
	SnapshotID   string `json:"snapshot_id,omitempty" extensions:"x-optional"`
}

type ExplainCardRequest struct {
	OperationID  string          `json:"operation_id"`
	DraftVersion int             `json:"draft_version"`
	Card         ExplanationCard `json:"card"`
	Question     string          `json:"question"`
}

type ExplanationModelResult struct {
	SchemaVersion int      `json:"schema_version"`
	Action        string   `json:"action"`
	TemplateKey   string   `json:"template_key"`
	EvidenceIDs   []string `json:"evidence_ids"`
}

type ExplanationFact struct {
	ID    string `json:"id"`
	Label string `json:"label"`
	Value any    `json:"value"`
}

type ValuationExplanationResult struct {
	SchemaVersion      int               `json:"schema_version"`
	Action             string            `json:"action"`
	TemplateKey        string            `json:"template_key"`
	Template           string            `json:"template"`
	Facts              []ExplanationFact `json:"facts" nullability:"nonnil"`
	EvidenceIDs        []string          `json:"evidence_ids" nullability:"nonnil"`
	FormulaVersion     string            `json:"formula_version"`
	SnapshotID         string            `json:"snapshot_id,omitempty" extensions:"x-optional"`
	RequiresUserAction bool              `json:"requires_user_action"`
	CardVersion        int               `json:"card_version"`
}

type ExplanationRepository interface {
	GetExplanation(ctx context.Context, operationID string) (*ConversationOperation, error)
	AcceptExplanation(ctx context.Context, operation *ConversationOperation, reservation SuggestionReservation, now time.Time) (*ConversationOperation, bool, error)
	CompleteExplanation(ctx context.Context, operationID string, result *ValuationExplanationResult, now time.Time) (string, error)
	FailExplanation(ctx context.Context, operationID, code string, now time.Time) error
	ExpireExplanation(ctx context.Context, operationID string, olderThan, now time.Time) error
}

type ExplanationConfig struct {
	DraftLimit   int
	IPLimit      int
	IPWindow     time.Duration
	GlobalLimit  int
	GlobalWindow time.Duration
	Timeout      time.Duration
	HMACKey      string
}

type explanationSnapshot struct {
	CardKey        string
	CardKind       string
	SnapshotID     string
	Version        int
	FormulaVersion string
	Facts          map[string]ExplanationFact
}

type TrialCard struct {
	RequestID  string
	SnapshotID string
	Request    TrialRequest
	Response   TrialResponse
	Snapshot   *TrialSnapshot
}

type TrialCardReader interface {
	LoadTrialCard(ctx context.Context, requestID string) (*TrialCard, error)
}
