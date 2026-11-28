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

type OperationEvaluationWriter interface {
	CreateEvaluation(context.Context, *CreateEvaluationParams) (int64, error)
}

// OperationRepository 保存正式评估的最小幂等凭据。终态记录随正式评估持久保留、不设 TTL；
// 草稿的 30 分钟闲置状态独立落在 ephemeral_kv，清理草稿不能重新打开已受理 operation 的重复提交面。
type OperationRepository interface {
	GetOperation(ctx context.Context, operationID string) (*ConversationOperation, error)
	// AcceptOperation 只有唯一键争用的胜者会执行 persist；带 failureCode 的业务失败提交终态凭据，
	// 无错误码的持久化故障则回滚占位，允许同一操作重试。同键竞争者拿不到选主锁即回滚并退避重试，
	// 等待期间不占用连接池连接。
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
	first_accepted_user_id, evaluation_id, status, error_code, operation_kind, result_json, created_at, updated_at, card_kind, card_key`

func (r *operationRepository) GetOperation(ctx context.Context, operationID string) (*ConversationOperation, error) {
	row := r.pool.QueryRow(ctx, `SELECT `+operationColumns+` FROM valuation_operations WHERE operation_key = $1`, operationID)
	return scanOperation(row)
}

func (r *operationRepository) AcceptOperation(ctx context.Context, operation *ConversationOperation,
	persist func(OperationEvaluationWriter) (int64, string, error)) (*ConversationOperation, bool, error) {
	return acceptValuationOperation(ctx, r.pool, operation, ConversationOperationKindEvaluation, func(tx pgx.Tx) (*ConversationOperation, error) {
		evaluationID, failureCode, err := persist(pgxEvaluationWriter{tx: tx})
		if err != nil && failureCode == "" {
			return nil, fmt.Errorf("persist valuation evaluation: %w", err)
		}
		if err != nil {
			if _, err := tx.Exec(ctx, `UPDATE valuation_operations
				SET status = $2, error_code = $3, updated_at = NOW()
				WHERE operation_key = $1`, operation.OperationID, OperationStatusFailed, failureCode); err != nil {
				return nil, fmt.Errorf("record valuation operation failure: %w", err)
			}
		} else {
			if _, err := tx.Exec(ctx, `UPDATE valuation_operations
				SET evaluation_id = $2, status = $3, error_code = NULL, updated_at = NOW()
				WHERE operation_key = $1`, operation.OperationID, evaluationID, OperationStatusSucceeded); err != nil {
				return nil, fmt.Errorf("complete valuation operation: %w", err)
			}
		}
		return scanOperation(tx.QueryRow(ctx, `SELECT `+operationColumns+`
			FROM valuation_operations WHERE operation_key = $1`, operation.OperationID))
	})
}

// acceptValuationOperation 是三种操作共用的受理骨架。同键受理先用事务级咨询锁选主：
// 胜者持有锁完成占位插入与评估写入，其余请求拿锁失败即回滚、退避后重试，等待期间不占住
// 连接池连接。若让等待方阻塞在 ON CONFLICT 的插入上，评估事务内的草稿/字典读取（另借连接）
// 会与这些占位插入互相等待，4 连接池（4 核 CI 的默认上限）下必现端点超时 500（#2954 #2957）。
// 锁键取 operation_key 的 64 位哈希：撞哈希只会让无关操作多退避一次，去重仍由唯一键的
// ON CONFLICT 保证，不影响正确性。
func acceptValuationOperation(ctx context.Context, pool *pgxpool.Pool, operation *ConversationOperation, kind string,
	persist func(pgx.Tx) (*ConversationOperation, error)) (accepted *ConversationOperation, inserted bool, err error) {
	for {
		accepted, inserted, err = tryAcceptValuationOperation(ctx, pool, operation, kind, persist)
		if !errors.Is(err, errOperationKeyBusy) {
			return accepted, inserted, err
		}
		if err := waitOperationKeyRelease(ctx); err != nil {
			return nil, false, err
		}
	}
}

// errOperationKeyBusy 表示同键受理已有胜者持锁：事务已回滚，等待方退避后重试。
var errOperationKeyBusy = errors.New("valuation operation key busy")

// operationKeyRetryInterval 是同键选主失败后的退避间隔；胜者受理通常只有毫秒级，
// 固定小间隔即可，端点 10s 超时前允许约 2000 次重试。
const operationKeyRetryInterval = 5 * time.Millisecond

func waitOperationKeyRelease(ctx context.Context) error {
	timer := time.NewTimer(operationKeyRetryInterval)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}

func tryAcceptValuationOperation(ctx context.Context, pool *pgxpool.Pool, operation *ConversationOperation, kind string,
	persist func(pgx.Tx) (*ConversationOperation, error)) (accepted *ConversationOperation, inserted bool, err error) {
	err = transactValuationOperation(ctx, pool, kind, func(tx pgx.Tx) error {
		var elected bool
		if err := tx.QueryRow(ctx, `SELECT pg_try_advisory_xact_lock(hashtextextended($1, 0))`,
			operation.OperationID).Scan(&elected); err != nil {
			return fmt.Errorf("elect valuation %s operation: %w", kind, err)
		}
		if !elected {
			return errOperationKeyBusy
		}
		draftVersion := operation.DraftVersion
		if kind == ConversationOperationKindSuggestion {
			draftVersion = operation.BaseVersion
		}
		// 草稿卡的卡片键就是草稿 ID；正式卡/试算卡没有草稿，draft_id 沿用卡片键以满足非空约束。
		draftID := operation.DraftID
		if draftID == "" {
			draftID = operation.CardKey
		}
		tag, err := tx.Exec(ctx, `INSERT INTO valuation_operations
			(operation_key, request_hash, draft_id, draft_version, first_accepted_user_id, status, operation_kind, card_kind, card_key)
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9) ON CONFLICT (operation_key) DO NOTHING`,
			operation.OperationID, operation.RequestHash, draftID, draftVersion,
			operation.FirstAcceptedUserID, OperationStatusPending, kind,
			nullableString(operation.CardKind), nullableString(operation.CardKey))
		if err != nil {
			return fmt.Errorf("reserve valuation %s operation: %w", kind, err)
		}
		if tag.RowsAffected() == 0 {
			existing, err := scanOperation(tx.QueryRow(ctx, `SELECT `+operationColumns+`
				FROM valuation_operations WHERE operation_key = $1`, operation.OperationID))
			if err != nil {
				return err
			}
			if existing.Kind != kind {
				return ErrOperationHashConflict
			}
			accepted = existing
			return nil
		}
		accepted, err = persist(tx)
		inserted = true
		return err
	})
	if err != nil {
		return nil, false, err
	}
	return accepted, inserted, nil
}

// getValuationOperation 按 operation 类型读取；类型不符与不存在同样按 404 处理。
func getValuationOperation(ctx context.Context, pool *pgxpool.Pool, operationID, kind string) (*ConversationOperation, error) {
	operation, err := scanOperation(pool.QueryRow(ctx, `SELECT `+operationColumns+`
		FROM valuation_operations WHERE operation_key = $1`, operationID))
	if err != nil {
		return nil, err
	}
	if operation.Kind != kind {
		return nil, ErrOperationNotFound
	}
	return operation, nil
}

// failValuationOperation 只把仍处于 pending 的同类操作改为失败，迟到响应不得覆盖终态。
func failValuationOperation(ctx context.Context, pool *pgxpool.Pool, operationID, kind, code string, now time.Time) error {
	_, err := pool.Exec(ctx, `UPDATE valuation_operations SET status = $2, error_code = $3, updated_at = $4
		WHERE operation_key = $1 AND operation_kind = $5 AND status = $6`,
		operationID, OperationStatusFailed, code, now, kind, OperationStatusPending)
	return err
}

func expireValuationOperation(ctx context.Context, pool *pgxpool.Pool, operationID, kind string, olderThan, now time.Time) error {
	_, err := pool.Exec(ctx, `UPDATE valuation_operations SET status = $2, error_code = $3, updated_at = $4
		WHERE operation_key = $1 AND operation_kind = $5 AND status = $6 AND updated_at < $7`,
		operationID, OperationStatusFailed, ErrorCodeSuggestionTimeout, now, kind, OperationStatusPending, olderThan)
	return err
}

func transactValuationOperation(ctx context.Context, pool *pgxpool.Pool, kind string, persist func(pgx.Tx) error) error {
	tx, err := pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin valuation %s operation: %w", kind, err)
	}
	defer func() { _ = tx.Rollback(context.Background()) }()
	if err := persist(tx); err != nil {
		return err
	}
	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit valuation %s operation: %w", kind, err)
	}
	return nil
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
	var operationKind string
	var resultJSON []byte
	var cardKind, cardKey *string
	var createdAt, updatedAt time.Time
	if err := row.Scan(&operation.OperationID, &operation.RequestHash, &operation.DraftID, &operation.DraftVersion,
		&userID, &evaluationID, &operation.Status, &errorCode, &operationKind, &resultJSON, &createdAt, &updatedAt, &cardKind, &cardKey); err != nil {
		if err == pgx.ErrNoRows {
			return nil, ErrOperationNotFound
		}
		return nil, fmt.Errorf("scan valuation operation: %w", err)
	}
	operation.ID = operation.OperationID
	operation.Kind = operationKind
	if cardKind != nil {
		operation.CardKind = *cardKind
	}
	if cardKey != nil {
		operation.CardKey = *cardKey
	}
	operation.FirstAcceptedUserID = userID
	if operationKind == ConversationOperationKindSuggestion {
		operation.BaseVersion = operation.DraftVersion
	}
	if len(resultJSON) > 0 {
		if operationKind == ConversationOperationKindExplanation {
			var result ValuationExplanationResult
			if err := json.Unmarshal(resultJSON, &result); err != nil {
				return nil, fmt.Errorf("decode valuation explanation result: %w", err)
			}
			operation.Explanation = &result
		} else {
			var result ValuationSuggestionResult
			if err := json.Unmarshal(resultJSON, &result); err != nil {
				return nil, fmt.Errorf("decode valuation suggestion result: %w", err)
			}
			operation.Result = &result
		}
	}
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
