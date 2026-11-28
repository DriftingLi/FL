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
	return getValuationOperation(ctx, r.pool, operationID, ConversationOperationKindSuggestion)
}

func (r *suggestionRepository) AcceptSuggestion(ctx context.Context, operation *ConversationOperation, reservation SuggestionReservation, now time.Time) (*ConversationOperation, bool, error) {
	return acceptValuationOperation(ctx, r.pool, operation, ConversationOperationKindSuggestion, func(tx pgx.Tx) (*ConversationOperation, error) {
		if _, err := lockEditingDraft(ctx, tx, operation.DraftID, operation.BaseVersion, now); err != nil {
			return nil, err
		}
		if err := reserveValuationCall(ctx, tx, operation, reservation, now); err != nil {
			return nil, err
		}
		return scanOperation(tx.QueryRow(ctx, `SELECT `+operationColumns+`
			FROM valuation_operations WHERE operation_key = $1`, operation.ID))
	})
}

// lockEditingDraft 锁住草稿行并校验版本与编辑态；建议与解释的受理/完成共用这一版本门。
func lockEditingDraft(ctx context.Context, tx pgx.Tx, draftID string, version int, now time.Time) (string, error) {
	var raw string
	err := tx.QueryRow(ctx, `SELECT cache_value FROM ephemeral_kv
		WHERE cache_key = $1 AND expires_at > $2 FOR UPDATE`, draftKey(draftID), now).Scan(&raw)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", ErrDraftNotFound
	}
	if err != nil {
		return "", fmt.Errorf("lock valuation draft: %w", err)
	}
	draft, err := decodeStoredDraft(raw)
	if err != nil {
		return "", err
	}
	if draft.Draft.Version != version {
		return raw, ErrDraftVersionConflict
	}
	if draft.Draft.Status != DraftStatusEditing {
		return raw, ErrDraftNotEditing
	}
	return raw, nil
}

// reserveValuationCall 是建议与解释共享的调用记账骨架：同一事务内扣三档预算并记一次实际模型调用。
func reserveValuationCall(ctx context.Context, tx pgx.Tx, operation *ConversationOperation, reservation SuggestionReservation, now time.Time) error {
	for _, budget := range []suggestionBudget{
		reservation.budget(suggestionBudgetDraft),
		reservation.budget(suggestionBudgetIP),
		reservation.budget(suggestionBudgetGlobal),
	} {
		if _, err := reserveSuggestionBudget(ctx, tx, budget, now); err != nil {
			return err
		}
	}
	switch operation.Kind {
	case ConversationOperationKindSuggestion:
		if _, err := tx.Exec(ctx, `INSERT INTO valuation_suggestion_calls
			(call_key, operation_key, draft_id, ip_hash, reserved_at) VALUES ($1, $2, $3, $4, $5)`,
			reservation.CallKey, operation.ID, operation.DraftID, reservation.IPHash, now); err != nil {
			return fmt.Errorf("record valuation suggestion call: %w", err)
		}
	case ConversationOperationKindExplanation:
		if _, err := tx.Exec(ctx, `INSERT INTO valuation_explanation_calls
			(call_key, operation_key, card_kind, card_key, ip_hash, reserved_at) VALUES ($1, $2, $3, $4, $5, $6)`,
			reservation.CallKey, operation.ID, operation.CardKind, operation.CardKey, reservation.IPHash, now); err != nil {
			return fmt.Errorf("record valuation explanation call: %w", err)
		}
	}
	return nil
}

func reserveSuggestionBudget(ctx context.Context, tx pgx.Tx, budget suggestionBudget, now time.Time) (int, error) {
	var used int
	err := tx.QueryRow(ctx, `INSERT INTO valuation_suggestion_budgets
		(scope, budget_key, used, window_started_at) VALUES ($1, $2, 1, $3)
		ON CONFLICT (scope, budget_key) DO UPDATE SET
			used = CASE WHEN $4 > 0 AND valuation_suggestion_budgets.window_started_at <= $3 - ($4 * interval '1 second')
				THEN 1 ELSE valuation_suggestion_budgets.used + 1 END,
			window_started_at = CASE WHEN $4 > 0 AND valuation_suggestion_budgets.window_started_at <= $3 - ($4 * interval '1 second')
				THEN $3 ELSE valuation_suggestion_budgets.window_started_at END
		RETURNING used`, budget.Scope, budget.Key, now, int64(budget.Window/time.Second)).Scan(&used)
	if err != nil {
		return 0, fmt.Errorf("reserve valuation suggestion budget: %w", err)
	}
	if used > budget.Limit {
		return 0, ErrSuggestionQuotaExceeded
	}
	return used, nil
}

func (r *suggestionRepository) CompleteSuggestion(ctx context.Context, operationID string, next *storedDraft, result *ValuationSuggestionResult, now time.Time) (status string, err error) {
	err = transactValuationOperation(ctx, r.pool, ConversationOperationKindSuggestion, func(tx pgx.Tx) error {
		operation, err := scanOperation(tx.QueryRow(ctx, `SELECT `+operationColumns+`
			FROM valuation_operations WHERE operation_key = $1 FOR UPDATE`, operationID))
		if err != nil {
			return err
		}
		if operation.Kind != ConversationOperationKindSuggestion {
			return ErrOperationNotFound
		}
		if operation.Status != OperationStatusPending {
			status = operation.Status
			return nil
		}
		var raw string
		err = tx.QueryRow(ctx, `SELECT cache_value FROM ephemeral_kv
			WHERE cache_key = $1 AND expires_at > $2 FOR UPDATE`, draftKey(operation.DraftID), now).Scan(&raw)
		if err != nil && !errors.Is(err, pgx.ErrNoRows) {
			return fmt.Errorf("lock valuation suggestion draft: %w", err)
		}
		stale := errors.Is(err, pgx.ErrNoRows) || draftVersionFromRaw(raw) != operation.BaseVersion || draftStatusFromRaw(raw) != DraftStatusEditing
		if stale {
			if _, err := tx.Exec(ctx, `UPDATE valuation_operations
				SET status = $2, error_code = $3, updated_at = $4 WHERE operation_key = $1`,
				operationID, OperationStatusStale, ErrorCodeDraftVersionConflict, now); err != nil {
				return err
			}
			status = OperationStatusStale
			return nil
		}
		if next != nil {
			encoded, err := encodeStoredDraft(next)
			if err != nil {
				return err
			}
			tag, err := tx.Exec(ctx, `UPDATE ephemeral_kv SET cache_value = $2, expires_at = $3
				WHERE cache_key = $1 AND cache_value = $4 AND expires_at > $5`,
				draftKey(operation.DraftID), encoded, next.Draft.ExpiresAt, raw, now)
			if err != nil {
				return fmt.Errorf("CAS valuation suggestion draft: %w", err)
			}
			if tag.RowsAffected() != 1 {
				if _, err := tx.Exec(ctx, `UPDATE valuation_operations
					SET status = $2, error_code = $3, updated_at = $4 WHERE operation_key = $1`,
					operationID, OperationStatusStale, ErrorCodeDraftVersionConflict, now); err != nil {
					return err
				}
				status = OperationStatusStale
				return nil
			}
		}
		resultJSON, err := json.Marshal(result)
		if err != nil {
			return fmt.Errorf("marshal valuation suggestion result: %w", err)
		}
		if _, err := tx.Exec(ctx, `UPDATE valuation_operations
			SET status = $2, result_json = $3, error_code = NULL, updated_at = $4 WHERE operation_key = $1`,
			operationID, OperationStatusSucceeded, resultJSON, now); err != nil {
			return fmt.Errorf("complete valuation suggestion: %w", err)
		}
		status = OperationStatusSucceeded
		return nil
	})
	return status, err
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
	return failValuationOperation(ctx, r.pool, operationID, ConversationOperationKindSuggestion, code, now)
}

func (r *suggestionRepository) ExpireSuggestion(ctx context.Context, operationID string, olderThan, now time.Time) error {
	return expireValuationOperation(ctx, r.pool, operationID, ConversationOperationKindSuggestion, olderThan, now)
}
