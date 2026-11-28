package valuation

import "time"

const (
	OperationStatusPending   = "pending"
	OperationStatusSucceeded = "succeeded"
	OperationStatusFailed    = "failed"
	OperationStatusStale     = "stale"
)

type ConversationOperationRequest struct {
	OperationID  string            `json:"operation_id"`
	DraftID      string            `json:"draft_id"`
	DraftVersion int               `json:"draft_version"`
	Request      EvaluationRequest `json:"request"`
}

type ConversationOperation struct {
	ID           string                      `json:"id"`
	Kind         string                      `json:"kind"`
	OperationID  string                      `json:"operation_id"`
	DraftID      string                      `json:"draft_id"`
	DraftVersion int                         `json:"draft_version"`
	BaseVersion  int                         `json:"base_version,omitempty" extensions:"x-optional"`
	Status       string                      `json:"status"`
	ErrorCode    string                      `json:"error_code,omitempty" extensions:"x-optional"`
	EvaluationID int64                       `json:"evaluation_id,omitempty" extensions:"x-optional"`
	Evaluation   *EvaluationDetail           `json:"evaluation,omitempty" extensions:"x-optional"`
	Result       *ValuationSuggestionResult  `json:"result,omitempty" extensions:"x-optional"`
	Explanation  *ValuationExplanationResult `json:"explanation,omitempty" extensions:"x-optional"`
	// CardKind / CardKey 记录解释操作引用的卡片：draft 卡为草稿 ID（旧卡为 草稿 ID@版本），
	// formal 卡为正式 operation ID，trial 卡为试算 request ID。
	CardKind            string    `json:"card_kind,omitempty" extensions:"x-optional"`
	CardKey             string    `json:"card_key,omitempty" extensions:"x-optional"`
	FirstAcceptedUserID *int      `json:"first_accepted_user_id" extensions:"x-nullable"`
	RequestHash         string    `json:"-"`
	CreatedAt           time.Time `json:"created_at"`
	UpdatedAt           time.Time `json:"updated_at"`
}
