package valuation

import (
	"context"
	"encoding/json"
	"errors"
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
	op, err := scanOperation(r.pool.QueryRow(ctx, `SELECT `+operationColumns+` FROM valuation_operations WHERE operation_key = $1`, operationID))
	if err != nil {
		return nil, err
	}
	if op.Kind != ConversationOperationKindExplanation {
		return nil, ErrOperationNotFound
	}
	return op, nil
}

func (r *explanationRepository) AcceptExplanation(ctx context.Context, operation *ConversationOperation, reservation SuggestionReservation, now time.Time) (*ConversationOperation, bool, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return nil, false, err
	}
	defer func() { _ = tx.Rollback(context.Background()) }()
	tag, err := tx.Exec(ctx, `INSERT INTO valuation_operations
		(operation_key, request_hash, draft_id, draft_version, first_accepted_user_id, status, operation_kind, card_kind, card_key)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9) ON CONFLICT (operation_key) DO NOTHING`,
		operation.ID, operation.RequestHash, operation.CardKey, operation.DraftVersion, operation.FirstAcceptedUserID,
		OperationStatusPending, ConversationOperationKindExplanation, operation.CardKind, operation.CardKey)
	if err != nil {
		return nil, false, err
	}
	if tag.RowsAffected() == 0 {
		op, err := scanOperation(tx.QueryRow(ctx, `SELECT `+operationColumns+` FROM valuation_operations WHERE operation_key=$1`, operation.ID))
		if err == nil && op.Kind != ConversationOperationKindExplanation {
			return nil, false, ErrOperationHashConflict
		}
		return op, false, err
	}
	if operation.CardKind == ExplanationCardDraft {
		var raw string
		err := tx.QueryRow(ctx, `SELECT cache_value FROM ephemeral_kv WHERE cache_key=$1 AND expires_at>$2 FOR UPDATE`, draftKey(operation.CardKey), now).Scan(&raw)
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, false, ErrDraftNotFound
		}
		if err != nil {
			return nil, false, err
		}
		if draftVersionFromRaw(raw) != operation.DraftVersion || draftStatusFromRaw(raw) != DraftStatusEditing {
			return nil, false, ErrDraftVersionConflict
		}
	}
	if _, err := reserveSuggestionBudget(ctx, tx, "draft", reservation.DraftKey, reservation.DraftLimit, 0, now); err != nil {
		return nil, false, err
	}
	if _, err := reserveSuggestionBudget(ctx, tx, "ip", reservation.IPHash, reservation.IPLimit, reservation.IPWindow, now); err != nil {
		return nil, false, err
	}
	if _, err := reserveSuggestionBudget(ctx, tx, "global", "global", reservation.GlobalLimit, reservation.GlobalWindow, now); err != nil {
		return nil, false, err
	}
	if _, err := tx.Exec(ctx, `INSERT INTO valuation_explanation_calls (call_key,operation_key,card_kind,card_key,ip_hash,reserved_at)
		VALUES ($1,$2,$3,$4,$5,$6)`, reservation.CallKey, operation.ID, operation.CardKind, operation.CardKey, reservation.IPHash, now); err != nil {
		return nil, false, err
	}
	accepted, err := scanOperation(tx.QueryRow(ctx, `SELECT `+operationColumns+` FROM valuation_operations WHERE operation_key=$1`, operation.ID))
	if err != nil {
		return nil, false, err
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, false, err
	}
	return accepted, true, nil
}

func (r *explanationRepository) CompleteExplanation(ctx context.Context, operationID string, result *ValuationExplanationResult, now time.Time) (string, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return "", err
	}
	defer func() { _ = tx.Rollback(context.Background()) }()
	op, err := scanOperation(tx.QueryRow(ctx, `SELECT `+operationColumns+` FROM valuation_operations WHERE operation_key=$1 FOR UPDATE`, operationID))
	if err != nil {
		return "", err
	}
	if op.Kind != ConversationOperationKindExplanation {
		return "", ErrOperationNotFound
	}
	if op.Status != OperationStatusPending {
		return op.Status, nil
	}
	if op.CardKind == ExplanationCardDraft {
		var raw string
		err := tx.QueryRow(ctx, `SELECT cache_value FROM ephemeral_kv WHERE cache_key=$1 AND expires_at>$2`, draftKey(op.CardKey), now).Scan(&raw)
		if errors.Is(err, pgx.ErrNoRows) || draftVersionFromRaw(raw) != op.DraftVersion || draftStatusFromRaw(raw) != DraftStatusEditing {
			_, err = tx.Exec(ctx, `UPDATE valuation_operations SET status=$2,error_code=$3,updated_at=$4 WHERE operation_key=$1`,
				operationID, OperationStatusStale, ErrorCodeDraftVersionConflict, now)
			if err == nil {
				err = tx.Commit(ctx)
			}
			return OperationStatusStale, err
		}
		if err != nil {
			return "", err
		}
	}
	encoded, err := json.Marshal(result)
	if err != nil {
		return "", err
	}
	_, err = tx.Exec(ctx, `UPDATE valuation_operations SET status=$2,result_json=$3,error_code=NULL,updated_at=$4 WHERE operation_key=$1`,
		operationID, OperationStatusSucceeded, encoded, now)
	if err != nil {
		return "", err
	}
	if err := tx.Commit(ctx); err != nil {
		return "", err
	}
	return OperationStatusSucceeded, nil
}

func (r *explanationRepository) FailExplanation(ctx context.Context, operationID, code string, now time.Time) error {
	_, err := r.pool.Exec(ctx, `UPDATE valuation_operations SET status=$2,error_code=$3,updated_at=$4
		WHERE operation_key=$1 AND operation_kind=$5 AND status=$6`, operationID, OperationStatusFailed, code, now, ConversationOperationKindExplanation, OperationStatusPending)
	return err
}

func (r *explanationRepository) ExpireExplanation(ctx context.Context, operationID string, olderThan, now time.Time) error {
	_, err := r.pool.Exec(ctx, `UPDATE valuation_operations SET status=$2,error_code=$3,updated_at=$4
		WHERE operation_key=$1 AND operation_kind=$5 AND status=$6 AND updated_at<$7`,
		operationID, OperationStatusFailed, ErrorCodeSuggestionTimeout, now, ConversationOperationKindExplanation, OperationStatusPending, olderThan)
	return err
}
