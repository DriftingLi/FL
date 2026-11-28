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

// 卡片类型（ExplanationCard.Kind）：
const (
	// ExplanationCardDraft 草稿卡：尚未正式评估，只解释卡内已确认的输入事实。
	// 这张卡没有服务端持久快照，解释所需事实由请求的 Snapshot 携带；旧卡不被当前草稿覆盖。
	ExplanationCardDraft = "draft"
	// ExplanationCardFormal 正式卡：按 operation_id（登录 owner 可附 evaluation_id 绑定）读取持久评估快照解释。
	ExplanationCardFormal = "formal"
	// ExplanationCardTrial 试算卡：按 request_id + snapshot_id 回放试算快照解释。
	ExplanationCardTrial = "trial"
)

// ExplanationCard 指向用户选中的那张结果卡；Kind 决定必填的引用字段，其余引用字段必须为空。
type ExplanationCard struct {
	// Kind 只取 draft / formal / trial，决定下方引用字段与事实来源。
	Kind string `json:"kind"`
	// DraftID 草稿能力 ID，仅 draft 卡使用。
	DraftID string `json:"draft_id,omitempty" extensions:"x-optional"`
	// Snapshot 草稿卡锁定的字段事实（键与 ValuationDraft.Fields 相同），仅 draft 卡使用。
	// 携带时按快照解释、不再读取当前草稿，因此已纠错的旧草稿卡仍可解释；未携带时要求 draft_version 与当前草稿一致。
	Snapshot map[string]DraftField `json:"snapshot,omitempty" extensions:"x-optional"`
	// OperationID 正式卡的创建 operation ID，仅 formal 卡使用。
	OperationID string `json:"operation_id,omitempty" extensions:"x-optional"`
	// EvaluationID 正式卡绑定的评估 ID，仅 formal 卡使用（登录 owner 可附带，用于确认卡片仍是同一评估）。
	EvaluationID int64 `json:"evaluation_id,omitempty" extensions:"x-optional"`
	// RequestID 试算请求 ID，仅 trial 卡使用。
	RequestID string `json:"request_id,omitempty" extensions:"x-optional"`
	// SnapshotID 试算快照哈希（完整值或 7 位短哈希），仅 trial 卡使用。
	SnapshotID string `json:"snapshot_id,omitempty" extensions:"x-optional"`
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

// ExplanationFact 解释引用的事实；ID 是模板占位键，Value 只来自卡片锁定快照。
type ExplanationFact struct {
	ID    string `json:"id"`
	Label string `json:"label"`
	Value any    `json:"value"`
}

// ValuationExplanationResult 解释结果。SnapshotID 的语义随卡片类型：formal = evaluation ID，
// trial = 试算快照哈希，draft = 卡片修订标识（当前草稿为 draft ID，旧卡为 draft ID@version）。
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

type explanationSnapshot struct {
	CardKey        string
	CardKind       string
	DraftID        string
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
