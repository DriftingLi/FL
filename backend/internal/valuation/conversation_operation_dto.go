package valuation

import "time"

const (
	OperationStatusPending   = "pending"
	OperationStatusSucceeded = "succeeded"
	OperationStatusFailed    = "failed"
)

type ConversationOperationRequest struct {
	OperationID  string            `json:"operation_id"`
	DraftID      string            `json:"draft_id"`
	DraftVersion int               `json:"draft_version"`
	Request      EvaluationRequest `json:"request"`
}

type ConversationOperation struct {
	ID                  string            `json:"id"`
	OperationID         string            `json:"operation_id"`
	DraftID             string            `json:"draft_id"`
	DraftVersion        int               `json:"draft_version"`
	Status              string            `json:"status"`
	ErrorCode           string            `json:"error_code,omitempty" extensions:"x-optional"`
	EvaluationID        int64             `json:"evaluation_id,omitempty" extensions:"x-optional"`
	Evaluation          *EvaluationDetail `json:"evaluation,omitempty" extensions:"x-optional"`
	FirstAcceptedUserID *int              `json:"first_accepted_user_id" extensions:"x-nullable"`
	RequestHash         string            `json:"-"`
	CreatedAt           time.Time         `json:"created_at"`
	UpdatedAt           time.Time         `json:"updated_at"`
}
