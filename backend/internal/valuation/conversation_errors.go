package valuation

import "errors"

var (
	ErrDraftNotFound          = errors.New("valuation draft not found")
	ErrDraftVersionConflict   = errors.New("valuation draft version conflict")
	ErrDraftNotEditing        = errors.New("valuation draft is not editable")
	ErrDraftNothingToUndo     = errors.New("valuation draft has nothing to undo")
	ErrDraftIncomplete        = errors.New("valuation draft is incomplete")
	ErrInvalidDraftField      = errors.New("invalid valuation draft field")
	ErrOperationNotFound      = errors.New("valuation operation not found")
	ErrOperationHashConflict  = errors.New("valuation operation request hash conflict")
	ErrInvalidOperation       = errors.New("invalid valuation operation")
	ErrOperationResultMissing = errors.New("valuation operation evaluation missing")
	ErrOperationDraftMismatch = errors.New("valuation operation request does not match draft")
	ErrEvaluationRejected     = errors.New("valuation evaluation rejected")
)

const (
	ErrorCodeDraftNotFound          = "draft_not_found"
	ErrorCodeDraftVersionConflict   = "draft_version_conflict"
	ErrorCodeDraftNotEditing        = "draft_not_editing"
	ErrorCodeDraftNothingToUndo     = "draft_nothing_to_undo"
	ErrorCodeDraftIncomplete        = "draft_incomplete"
	ErrorCodeInvalidDraftField      = "invalid_draft_field"
	ErrorCodeInvalidOperation       = "invalid_operation"
	ErrorCodeEvaluationRejected     = "evaluation_rejected"
	ErrorCodeOperationResultMissing = "operation_result_missing"
	ErrorCodeOperationDraftMismatch = "operation_draft_mismatch"
)

var operationErrorMappings = [...]struct {
	sentinel error
	code     string
}{
	{ErrDraftNotFound, ErrorCodeDraftNotFound},
	{ErrDraftVersionConflict, ErrorCodeDraftVersionConflict},
	{ErrDraftNotEditing, ErrorCodeDraftNotEditing},
	{ErrDraftNothingToUndo, ErrorCodeDraftNothingToUndo},
	{ErrDraftIncomplete, ErrorCodeDraftIncomplete},
	{ErrInvalidDraftField, ErrorCodeInvalidDraftField},
	{ErrInvalidOperation, ErrorCodeInvalidOperation},
	{ErrEvaluationRejected, ErrorCodeEvaluationRejected},
	{ErrOperationResultMissing, ErrorCodeOperationResultMissing},
	{ErrOperationDraftMismatch, ErrorCodeOperationDraftMismatch},
}
