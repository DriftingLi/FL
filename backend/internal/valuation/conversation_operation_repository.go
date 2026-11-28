package valuation

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type OperationEvaluationWriter interface {
	CreateEvaluation(context.Context, *CreateEvaluationParams) (int64, error)
}

// OperationRepository 保存正式评估的最小幂等凭据。终态记录随正式评估持久保留、不设 TTL；
// 草稿的 30 分钟闲置状态独立落在 ephemeral_kv，清理草稿不能重新打开已受理 operation 的重复提交面。
type OperationRepository interface {
	GetOperation(ctx context.Context, operationID string) (*ConversationOperation, error)
	// AcceptOperation 只有唯一键争用的胜者会执行 persist；带 failureCode 的业务失败提交终态凭据，
	// 无错误码的持久化故障则回滚占位，允许同一操作重试。同键竞争者在胜者提交前阻塞。
	AcceptOperation(ctx context.Context, operation *ConversationOperation,
		persist func(OperationEvaluationWriter) (int64, string, error)) (*ConversationOperation, bool, error)
}

type operationRepository struct {
	pool *pgxpool.Pool
}

func NewOperationRepository(pool *pgxpool.Pool) OperationRepository {
	return &operationRepository{pool: pool}
}

const operationColumns = `operation_key, request_hash, draft_id, draft_version,
	first_accepted_user_id, evaluation_id, status, error_code, created_at, updated_at`

func (r *operationRepository) GetOperation(ctx context.Context, operationID string) (*ConversationOperation, error) {
	row := r.pool.QueryRow(ctx, `SELECT `+operationColumns+` FROM valuation_operations WHERE operation_key = $1`, operationID)
	return scanOperation(row)
}

func (r *operationRepository) AcceptOperation(ctx context.Context, operation *ConversationOperation,
	persist func(OperationEvaluationWriter) (int64, string, error)) (*ConversationOperation, bool, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return nil, false, fmt.Errorf("begin valuation operation: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	tag, err := tx.Exec(ctx, `INSERT INTO valuation_operations
		(operation_key, request_hash, draft_id, draft_version, first_accepted_user_id, status)
		VALUES ($1, $2, $3, $4, $5, $6) ON CONFLICT (operation_key) DO NOTHING`,
		operation.OperationID, operation.RequestHash, operation.DraftID, operation.DraftVersion,
		operation.FirstAcceptedUserID, OperationStatusPending)
	if err != nil {
		return nil, false, fmt.Errorf("reserve valuation operation: %w", err)
	}
	if tag.RowsAffected() == 0 {
		existing, err := scanOperation(tx.QueryRow(ctx, `SELECT `+operationColumns+`
			FROM valuation_operations WHERE operation_key = $1`, operation.OperationID))
		if err != nil {
			return nil, false, err
		}
		return existing, false, nil
	}

	evaluationID, failureCode, err := persist(pgxEvaluationWriter{tx: tx})
	if err != nil && failureCode == "" {
		return nil, false, fmt.Errorf("persist valuation evaluation: %w", err)
	}
	if err != nil {
		if _, err := tx.Exec(ctx, `UPDATE valuation_operations
			SET status = $2, error_code = $3, updated_at = NOW()
			WHERE operation_key = $1`, operation.OperationID, OperationStatusFailed, failureCode); err != nil {
			return nil, false, fmt.Errorf("record valuation operation failure: %w", err)
		}
		accepted, err := scanOperation(tx.QueryRow(ctx, `SELECT `+operationColumns+`
			FROM valuation_operations WHERE operation_key = $1`, operation.OperationID))
		if err != nil {
			return nil, false, err
		}
		if err := tx.Commit(ctx); err != nil {
			return nil, false, fmt.Errorf("commit failed valuation operation: %w", err)
		}
		return accepted, true, nil
	}
	if _, err := tx.Exec(ctx, `UPDATE valuation_operations
		SET evaluation_id = $2, status = $3, error_code = NULL, updated_at = NOW()
		WHERE operation_key = $1`, operation.OperationID, evaluationID, OperationStatusSucceeded); err != nil {
		return nil, false, fmt.Errorf("complete valuation operation: %w", err)
	}
	accepted, err := scanOperation(tx.QueryRow(ctx, `SELECT `+operationColumns+`
		FROM valuation_operations WHERE operation_key = $1`, operation.OperationID))
	if err != nil {
		return nil, false, err
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, false, fmt.Errorf("commit valuation operation: %w", err)
	}
	return accepted, true, nil
}

type pgxEvaluationWriter struct {
	tx pgx.Tx
}

func (w pgxEvaluationWriter) CreateEvaluation(ctx context.Context, params *CreateEvaluationParams) (int64, error) {
	return createEvaluation(ctx, w.tx, params)
}

func scanOperation(row pgx.Row) (*ConversationOperation, error) {
	var operation ConversationOperation
	var userID *int
	var evaluationID *int64
	var errorCode *string
	var createdAt, updatedAt time.Time
	if err := row.Scan(&operation.OperationID, &operation.RequestHash, &operation.DraftID, &operation.DraftVersion,
		&userID, &evaluationID, &operation.Status, &errorCode, &createdAt, &updatedAt); err != nil {
		if err == pgx.ErrNoRows {
			return nil, ErrOperationNotFound
		}
		return nil, fmt.Errorf("scan valuation operation: %w", err)
	}
	operation.ID = operation.OperationID
	operation.FirstAcceptedUserID = userID
	if evaluationID != nil {
		operation.EvaluationID = *evaluationID
	}
	if errorCode != nil {
		operation.ErrorCode = *errorCode
	}
	operation.CreatedAt = createdAt
	operation.UpdatedAt = updatedAt
	return &operation, nil
}
