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

type suggestionRepository struct {
	pool *pgxpool.Pool
}

func NewSuggestionRepository(pool *pgxpool.Pool) SuggestionRepository {
	return &suggestionRepository{pool: pool}
}

func (r *suggestionRepository) GetSuggestion(ctx context.Context, operationID string) (*ConversationOperation, error) {
	operation, err := scanOperation(r.pool.QueryRow(ctx, `SELECT `+operationColumns+`
		FROM valuation_operations WHERE operation_key = $1`, operationID))
	if err != nil {
		return nil, err
	}
	if operation.Kind != ConversationOperationKindSuggestion {
		return nil, ErrOperationNotFound
	}
	return operation, nil
}

func (r *suggestionRepository) AcceptSuggestion(ctx context.Context, operation *ConversationOperation, reservation SuggestionReservation, now time.Time) (*ConversationOperation, bool, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return nil, false, fmt.Errorf("begin valuation suggestion: %w", err)
	}
	defer func() { _ = tx.Rollback(context.Background()) }()

	tag, err := tx.Exec(ctx, `INSERT INTO valuation_operations
		(operation_key, request_hash, draft_id, draft_version, first_accepted_user_id, status, operation_kind)
		VALUES ($1, $2, $3, $4, $5, $6, $7) ON CONFLICT (operation_key) DO NOTHING`,
		operation.ID, operation.RequestHash, operation.DraftID, operation.BaseVersion,
		operation.FirstAcceptedUserID, OperationStatusPending, ConversationOperationKindSuggestion)
	if err != nil {
		return nil, false, fmt.Errorf("reserve valuation suggestion: %w", err)
	}
	if tag.RowsAffected() == 0 {
		existing, err := scanOperation(tx.QueryRow(ctx, `SELECT `+operationColumns+`
			FROM valuation_operations WHERE operation_key = $1`, operation.ID))
		if err != nil {
			return nil, false, err
		}
		if existing.Kind != ConversationOperationKindSuggestion {
			return nil, false, ErrOperationHashConflict
		}
		return existing, false, nil
	}

	var raw string
	err = tx.QueryRow(ctx, `SELECT cache_value FROM ephemeral_kv
		WHERE cache_key = $1 AND expires_at > $2 FOR UPDATE`, draftKey(operation.DraftID), now).Scan(&raw)
	if err != nil {
		if pgx.ErrNoRows == err {
			return nil, false, ErrDraftNotFound
		}
		return nil, false, fmt.Errorf("lock valuation suggestion draft: %w", err)
	}
	draft, err := decodeStoredDraft(raw)
	if err != nil {
		return nil, false, err
	}
	if draft.Draft.Version != operation.BaseVersion {
		return nil, false, ErrDraftVersionConflict
	}
	if draft.Draft.Status != DraftStatusEditing {
		return nil, false, ErrDraftNotEditing
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
	if _, err := tx.Exec(ctx, `INSERT INTO valuation_suggestion_calls
		(call_key, operation_key, draft_id, ip_hash, reserved_at) VALUES ($1, $2, $3, $4, $5)`,
		reservation.CallKey, operation.ID, operation.DraftID, reservation.IPHash, now); err != nil {
		return nil, false, fmt.Errorf("record valuation suggestion call: %w", err)
	}
	accepted, err := scanOperation(tx.QueryRow(ctx, `SELECT `+operationColumns+`
		FROM valuation_operations WHERE operation_key = $1`, operation.ID))
	if err != nil {
		return nil, false, err
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, false, fmt.Errorf("commit valuation suggestion reservation: %w", err)
	}
	return accepted, true, nil
}

func reserveSuggestionBudget(ctx context.Context, tx pgx.Tx, scope, key string, limit int, window time.Duration, now time.Time) (int, error) {
	var used int
	err := tx.QueryRow(ctx, `INSERT INTO valuation_suggestion_budgets
		(scope, budget_key, used, window_started_at) VALUES ($1, $2, 1, $3)
		ON CONFLICT (scope, budget_key) DO UPDATE SET
			used = CASE WHEN $4 > 0 AND valuation_suggestion_budgets.window_started_at <= $3 - ($4 * interval '1 second')
				THEN 1 ELSE valuation_suggestion_budgets.used + 1 END,
			window_started_at = CASE WHEN $4 > 0 AND valuation_suggestion_budgets.window_started_at <= $3 - ($4 * interval '1 second')
				THEN $3 ELSE valuation_suggestion_budgets.window_started_at END
		RETURNING used`, scope, key, now, int64(window/time.Second)).Scan(&used)
	if err != nil {
		return 0, fmt.Errorf("reserve valuation suggestion budget: %w", err)
	}
	if used > limit {
		return 0, ErrSuggestionQuotaExceeded
	}
	return used, nil
}

func (r *suggestionRepository) CompleteSuggestion(ctx context.Context, operationID string, next *storedDraft, result *ValuationSuggestionResult, now time.Time) (string, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return "", fmt.Errorf("begin valuation suggestion completion: %w", err)
	}
	defer func() { _ = tx.Rollback(context.Background()) }()
	operation, err := scanOperation(tx.QueryRow(ctx, `SELECT `+operationColumns+`
		FROM valuation_operations WHERE operation_key = $1 FOR UPDATE`, operationID))
	if err != nil {
		return "", err
	}
	if operation.Kind != ConversationOperationKindSuggestion {
		return "", ErrOperationNotFound
	}
	if operation.Status != OperationStatusPending {
		return operation.Status, nil
	}
	var raw string
	err = tx.QueryRow(ctx, `SELECT cache_value FROM ephemeral_kv
		WHERE cache_key = $1 AND expires_at > $2 FOR UPDATE`, draftKey(operation.DraftID), now).Scan(&raw)
	if errors.Is(err, pgx.ErrNoRows) || draftVersionFromRaw(raw) != operation.BaseVersion || draftStatusFromRaw(raw) != DraftStatusEditing {
		if _, updateErr := tx.Exec(ctx, `UPDATE valuation_operations
			SET status = $2, error_code = $3, updated_at = $4 WHERE operation_key = $1`,
			operationID, OperationStatusStale, ErrorCodeDraftVersionConflict, now); updateErr != nil {
			return "", updateErr
		}
		if commitErr := tx.Commit(ctx); commitErr != nil {
			return "", commitErr
		}
		return OperationStatusStale, nil
	}
	if err != nil {
		return "", fmt.Errorf("lock valuation suggestion draft: %w", err)
	}
	if next != nil {
		encoded, err := encodeStoredDraft(next)
		if err != nil {
			return "", err
		}
		tag, err := tx.Exec(ctx, `UPDATE ephemeral_kv SET cache_value = $2, expires_at = $3
			WHERE cache_key = $1 AND cache_value = $4 AND expires_at > $5`,
			draftKey(operation.DraftID), encoded, next.Draft.ExpiresAt, raw, now)
		if err != nil {
			return "", fmt.Errorf("CAS valuation suggestion draft: %w", err)
		}
		if tag.RowsAffected() != 1 {
			if _, updateErr := tx.Exec(ctx, `UPDATE valuation_operations
				SET status = $2, error_code = $3, updated_at = $4 WHERE operation_key = $1`,
				operationID, OperationStatusStale, ErrorCodeDraftVersionConflict, now); updateErr != nil {
				return "", updateErr
			}
			if commitErr := tx.Commit(ctx); commitErr != nil {
				return "", commitErr
			}
			return OperationStatusStale, nil
		}
	}
	resultJSON, err := json.Marshal(result)
	if err != nil {
		return "", fmt.Errorf("marshal valuation suggestion result: %w", err)
	}
	if _, err := tx.Exec(ctx, `UPDATE valuation_operations
		SET status = $2, result_json = $3, error_code = NULL, updated_at = $4 WHERE operation_key = $1`,
		operationID, OperationStatusSucceeded, resultJSON, now); err != nil {
		return "", fmt.Errorf("complete valuation suggestion: %w", err)
	}
	if err := tx.Commit(ctx); err != nil {
		return "", fmt.Errorf("commit valuation suggestion completion: %w", err)
	}
	return OperationStatusSucceeded, nil
}

func draftVersionFromRaw(raw string) int {
	if raw == "" {
		return 0
	}
	draft, err := decodeStoredDraft(raw)
	if err != nil {
		return 0
	}
	return draft.Draft.Version
}

func draftStatusFromRaw(raw string) string {
	if raw == "" {
		return ""
	}
	draft, err := decodeStoredDraft(raw)
	if err != nil {
		return ""
	}
	return draft.Draft.Status
}

func (r *suggestionRepository) FailSuggestion(ctx context.Context, operationID, code string, now time.Time) error {
	_, err := r.pool.Exec(ctx, `UPDATE valuation_operations
		SET status = $2, error_code = $3, updated_at = $4
		WHERE operation_key = $1 AND operation_kind = $5 AND status = $6`,
		operationID, OperationStatusFailed, code, now, ConversationOperationKindSuggestion, OperationStatusPending)
	return err
}

func (r *suggestionRepository) ExpireSuggestion(ctx context.Context, operationID string, olderThan, now time.Time) error {
	_, err := r.pool.Exec(ctx, `UPDATE valuation_operations
		SET status = $2, error_code = $3, updated_at = $4
		WHERE operation_key = $1 AND operation_kind = $5 AND status = $6 AND updated_at < $7`,
		operationID, OperationStatusFailed, ErrorCodeSuggestionTimeout, now,
		ConversationOperationKindSuggestion, OperationStatusPending, olderThan)
	return err
}
