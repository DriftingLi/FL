package valuation

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type explanationRepository struct {
	pool *pgxpool.Pool
}

func NewExplanationRepository(pool *pgxpool.Pool) ExplanationRepository {
	return &explanationRepository{pool: pool}
}

func (r *explanationRepository) GetExplanation(ctx context.Context, operationID string) (*ConversationOperation, error) {
	return getValuationOperation(ctx, r.pool, operationID, ConversationOperationKindExplanation)
}

// AcceptExplanation 复用建议的幂等与调用记账骨架；草稿卡只有在绑定当前草稿版本时才校验草稿，
// 携带修订快照的旧卡直接按快照受理，不读取当前草稿。
func (r *explanationRepository) AcceptExplanation(ctx context.Context, operation *ConversationOperation, reservation SuggestionReservation, now time.Time) (*ConversationOperation, bool, error) {
	return acceptValuationOperation(ctx, r.pool, operation, ConversationOperationKindExplanation, func(tx pgx.Tx) (*ConversationOperation, error) {
		if explanationPinsLiveDraft(operation) {
			if _, err := lockEditingDraft(ctx, tx, operation.CardKey, operation.DraftVersion, now); err != nil {
				return nil, err
			}
		}
		if err := reserveValuationCall(ctx, tx, operation, reservation, now); err != nil {
			return nil, err
		}
		return scanOperation(tx.QueryRow(ctx, `SELECT `+operationColumns+`
			FROM valuation_operations WHERE operation_key = $1`, operation.ID))
	})
}

// explanationPinsLiveDraft 表示草稿卡仍绑定当前草稿版本（卡片键 = 草稿 ID）；
// 旧卡使用“草稿 ID@版本”作为卡片键，完成时也不得让后来的纠错把解释判成 stale。
func explanationPinsLiveDraft(operation *ConversationOperation) bool {
	return operation.CardKind == ExplanationCardDraft && operation.CardKey == operation.DraftID
}

func (r *explanationRepository) CompleteExplanation(ctx context.Context, operationID string, result *ValuationExplanationResult, now time.Time) (string, error) {
	status := ""
	err := transactValuationOperation(ctx, r.pool, ConversationOperationKindExplanation, func(tx pgx.Tx) error {
		op, err := scanOperation(tx.QueryRow(ctx, `SELECT `+operationColumns+`
			FROM valuation_operations WHERE operation_key = $1 FOR UPDATE`, operationID))
		if err != nil {
			return err
		}
		if op.Kind != ConversationOperationKindExplanation {
			return ErrOperationNotFound
		}
		if op.Status != OperationStatusPending {
			status = op.Status
			return nil
		}
		if explanationPinsLiveDraft(op) {
			var raw string
			err := tx.QueryRow(ctx, `SELECT cache_value FROM ephemeral_kv
				WHERE cache_key = $1 AND expires_at > $2 FOR UPDATE`, draftKey(op.CardKey), now).Scan(&raw)
			if err != nil && !errors.Is(err, pgx.ErrNoRows) {
				return fmt.Errorf("lock valuation explanation draft: %w", err)
			}
			stale := errors.Is(err, pgx.ErrNoRows) || draftVersionFromRaw(raw) != op.DraftVersion || draftStatusFromRaw(raw) != DraftStatusEditing
			if stale {
				if _, err := tx.Exec(ctx, `UPDATE valuation_operations
					SET status = $2, error_code = $3, updated_at = $4 WHERE operation_key = $1`,
					operationID, OperationStatusStale, ErrorCodeDraftVersionConflict, now); err != nil {
					return err
				}
				status = OperationStatusStale
				return nil
			}
		}
		encoded, err := json.Marshal(result)
		if err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, `UPDATE valuation_operations
			SET status = $2, result_json = $3, error_code = NULL, updated_at = $4 WHERE operation_key = $1`,
			operationID, OperationStatusSucceeded, encoded, now); err != nil {
			return err
		}
		status = OperationStatusSucceeded
		return nil
	})
	if err != nil {
		return "", err
	}
	return status, nil
}

func (r *explanationRepository) FailExplanation(ctx context.Context, operationID, code string, now time.Time) error {
	return failValuationOperation(ctx, r.pool, operationID, ConversationOperationKindExplanation, code, now)
}

func (r *explanationRepository) ExpireExplanation(ctx context.Context, operationID string, olderThan, now time.Time) error {
	return expireValuationOperation(ctx, r.pool, operationID, ConversationOperationKindExplanation, olderThan, now)
}
