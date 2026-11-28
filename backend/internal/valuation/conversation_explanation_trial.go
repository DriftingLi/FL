package valuation

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
)

func (r *PostgresTrialSnapshotReader) LoadTrialCard(ctx context.Context, requestID string) (*TrialCard, error) {
	var snapshotID, requestJSON, snapshotJSON, formulaHash string
	err := r.pool.QueryRow(ctx, `SELECT snapshot_id, request_json, snapshot_json, formula_hash
		FROM valuation_trial_snapshots WHERE request_id=$1`, requestID).
		Scan(&snapshotID, &requestJSON, &snapshotJSON, &formulaHash)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrOperationNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("read valuation trial card: %w", err)
	}
	snapshot, err := decodeStoredTrialSnapshot(snapshotID, requestJSON, snapshotJSON, formulaHash)
	if err != nil {
		return nil, err
	}
	var req TrialRequest
	if err := json.Unmarshal([]byte(requestJSON), &req); err != nil {
		return nil, err
	}
	// 旧卡是已出卡的历史快照：全局开关只约束新试算受理（#2958），
	// 不得让默认关闭的开关把已定价旧卡的「为什么」打成 not_found（#2957）。
	snapshot.Control = TrialControl{Enabled: true, Version: 1}
	response, err := CalculateTrial(ctx, req, snapshot, "development")
	if err != nil || response.Status != TrialStatusPriced {
		return nil, ErrOperationNotFound
	}
	return &TrialCard{RequestID: requestID, SnapshotID: snapshotID, Request: req, Response: response, Snapshot: snapshot}, nil
}
