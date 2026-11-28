package valuation

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/cloudwego/eino/schema"

	"forklift-training/internal/aiassistant"
)

func TestValuationExplanationPostgresSharedQuotaIdempotencyAndOldCard(t *testing.T) {
	conversation, pool, _, model := newSuggestionPostgresService(t)
	model.structured = func(context.Context, string, []*schema.Message, string, aiassistant.StructuredCompleteOptions) (string, error) {
		return explanationJSON("explain", "hours_coefficient", []string{"input.usage_hours"}), nil
	}
	ctx := context.Background()
	draft, err := conversation.CreateDraft(ctx, &CreateDraftRequest{})
	if err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 4; i++ {
		if _, err := conversation.RequestSuggestions(ctx, draft.ID, &SuggestDraftRequest{OperationID: fmt.Sprintf("s%d", i), BaseVersion: 1, Text: "建议"}, 0, "203.0.113.70"); err != nil {
			t.Fatal(err)
		}
		awaitSuggestionPostgres(t, conversation, suggestionOperationKey(draft.ID, fmt.Sprintf("s%d", i)))
	}
	req := &ExplainCardRequest{OperationID: "same", DraftVersion: 1, Card: ExplanationCard{Kind: ExplanationCardDraft, DraftID: draft.ID}, Question: "工时如何影响？"}
	const workers = 12
	errs := make(chan error, workers)
	for i := 0; i < workers; i++ {
		go func() {
			_, err := conversation.RequestExplanation(ctx, req, 0, "203.0.113.70")
			errs <- err
		}()
	}
	for i := 0; i < workers; i++ {
		if err := <-errs; err != nil {
			t.Fatalf("并发同键应幂等受理: %v", err)
		}
	}
	op := awaitExplanationPostgres(t, conversation, explanationOperationKey(draft.ID, "same"))
	if op.Status != OperationStatusSucceeded || op.Explanation == nil {
		t.Fatalf("解释未完成: %+v", op)
	}
	for i := 0; i < 3; i++ {
		if _, err := conversation.RequestExplanation(ctx, &ExplainCardRequest{OperationID: fmt.Sprintf("e%d", i), DraftVersion: 1,
			Card: req.Card, Question: "解释"}, 0, "203.0.113.70"); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := conversation.RequestExplanation(ctx, &ExplainCardRequest{OperationID: "ninth", DraftVersion: 1, Card: req.Card, Question: "解释"}, 0, "203.0.113.70"); !errors.Is(err, ErrSuggestionQuotaExceeded) {
		t.Fatalf("建议与解释共享 8 次草稿额度: %v", err)
	}
	before := model.calls.Load()
	if _, err := conversation.RequestExplanation(ctx, req, 9, "203.0.113.70"); err != nil {
		t.Fatal(err)
	}
	if model.calls.Load() != before {
		t.Fatal("同键重试不得再次调用模型")
	}
	var calls int
	if err := pool.QueryRow(ctx, `SELECT COUNT(*) FROM valuation_explanation_calls`).Scan(&calls); err != nil {
		t.Fatal(err)
	}
	if calls != 4 {
		t.Fatalf("实际解释调用账 = %d, want 4", calls)
	}
	version := 1
	if _, err := conversation.PatchDraft(ctx, draft.ID, &PatchDraftRequest{BaseVersion: &version,
		Fields: map[string]DraftField{"usage_hours": {State: DraftFieldValue, Value: 5000}}}); err != nil {
		t.Fatal(err)
	}
	if _, err := conversation.RequestExplanation(ctx, req, 0, "203.0.113.70"); !errors.Is(err, ErrDraftVersionConflict) {
		t.Fatalf("旧卡必须版本冲突: %v", err)
	}
}

func TestValuationExplanationPostgresLateResponseIsStale(t *testing.T) {
	conversation, _, _, model := newSuggestionPostgresService(t)
	ctx := context.Background()
	started := make(chan struct{})
	release := make(chan struct{})
	model.structured = func(ctx context.Context, _ string, _ []*schema.Message, _ string, _ aiassistant.StructuredCompleteOptions) (string, error) {
		close(started)
		select {
		case <-release:
			return explanationJSON("explain", "hours_coefficient", []string{"input.usage_hours"}), nil
		case <-ctx.Done():
			return "", ctx.Err()
		}
	}
	draft, err := conversation.CreateDraft(ctx, &CreateDraftRequest{})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := conversation.RequestExplanation(ctx, &ExplainCardRequest{OperationID: "late", DraftVersion: 1,
		Card: ExplanationCard{Kind: ExplanationCardDraft, DraftID: draft.ID}, Question: "解释"}, 0, "203.0.113.71"); err != nil {
		t.Fatal(err)
	}
	<-started
	version := 1
	if _, err := conversation.PatchDraft(ctx, draft.ID, &PatchDraftRequest{BaseVersion: &version,
		Fields: map[string]DraftField{"usage_hours": {State: DraftFieldValue, Value: 9000}}}); err != nil {
		t.Fatal(err)
	}
	close(release)
	op := awaitExplanationPostgres(t, conversation, explanationOperationKey(draft.ID, "late"))
	if op.Status != OperationStatusStale || op.ErrorCode != ErrorCodeDraftVersionConflict {
		t.Fatalf("晚响应必须 stale: %+v", op)
	}
}

func TestValuationExplanationPostgresTrialCardUsesLockedSnapshot(t *testing.T) {
	conversation, pool, _, model := newSuggestionPostgresService(t)
	model.structured = func(context.Context, string, []*schema.Message, string, aiassistant.StructuredCompleteOptions) (string, error) {
		return explanationJSON("explain", "value_interval", []string{"result.estimated_value", "result.confidence_low", "result.confidence_high"}), nil
	}
	price := OriginalPrice{ID: 1, Brand: "林德", VehicleType: "电动叉车", Series: "K系列", Tonnage: 3,
		ConfigType: "标准", MastType: "标准门架", MastHeightMM: 3000, EarliestFactoryYear: 2020, OriginalPrice: 100000}
	snapshot := newTrialSnapshotForTest(t, []OriginalPrice{price})
	req := trialCompleteRequest()
	requestJSON, err := json.Marshal(req)
	if err != nil {
		t.Fatal(err)
	}
	snapshotJSON, err := json.Marshal(snapshot.ReplayInput)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(context.Background(), `INSERT INTO valuation_trial_snapshots
		(snapshot_id,request_id,request_json,snapshot_json,formula_hash) VALUES ($1,$2,$3::jsonb,$4::jsonb,$5)`,
		snapshot.Hash, req.RequestID, string(requestJSON), string(snapshotJSON), TrialFormulaHash); err != nil {
		t.Fatal(err)
	}
	op, err := conversation.RequestExplanation(context.Background(), &ExplainCardRequest{OperationID: "trial-why", DraftVersion: 1,
		Card: ExplanationCard{Kind: ExplanationCardTrial, RequestID: req.RequestID, SnapshotID: shortHash(snapshot.Hash)}, Question: "为什么是这个区间？"}, 0, "203.0.113.72")
	if err != nil {
		t.Fatal(err)
	}
	completed := awaitExplanationPostgres(t, conversation, op.ID)
	if completed.Status != OperationStatusSucceeded || completed.Explanation.SnapshotID != snapshot.Hash || completed.Explanation.FormulaVersion != FormulaVersionV2 {
		t.Fatalf("试算解释必须锁定原快照与公式: %+v", completed)
	}
	if len(completed.Explanation.Facts) < 3 {
		t.Fatalf("试算区间事实不足: %+v", completed.Explanation)
	}
	if _, err := conversation.RequestExplanation(context.Background(), &ExplainCardRequest{OperationID: "wrong-snapshot", DraftVersion: 1,
		Card: ExplanationCard{Kind: ExplanationCardTrial, RequestID: req.RequestID, SnapshotID: "deadbeef"}, Question: "为什么？"}, 0, "203.0.113.72"); err == nil {
		t.Fatal("错误快照必须拒绝")
	}
}

func awaitExplanationPostgres(t *testing.T, conversation *ConversationService, id string) *ConversationOperation {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		op, err := conversation.explanations.repository.GetExplanation(context.Background(), id)
		if err != nil {
			t.Fatal(err)
		}
		if op.Status != OperationStatusPending {
			return op
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatal("解释操作未结束")
	return nil
}
