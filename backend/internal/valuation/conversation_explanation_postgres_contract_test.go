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
	draft, err := conversation.CreateDraft(ctx, &CreateDraftRequest{Fields: map[string]DraftField{
		"usage_hours": {State: DraftFieldValue, Value: 1200},
	}})
	if err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 3; i++ {
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
		awaitExplanationPostgres(t, conversation, explanationOperationKey(draft.ID, fmt.Sprintf("e%d", i)))
	}
	before := model.calls.Load()
	if _, err := conversation.RequestExplanation(ctx, req, 9, "203.0.113.70"); err != nil {
		t.Fatal(err)
	}
	if model.calls.Load() != before {
		t.Fatal("同键重试不得再次调用模型")
	}
	version := 1
	if _, err := conversation.PatchDraft(ctx, draft.ID, &PatchDraftRequest{BaseVersion: &version,
		Fields: map[string]DraftField{"usage_hours": {State: DraftFieldValue, Value: 5000}}}); err != nil {
		t.Fatal(err)
	}
	oldCard := &ExplainCardRequest{OperationID: "old-card", DraftVersion: 1,
		Card: ExplanationCard{Kind: ExplanationCardDraft, DraftID: draft.ID,
			Snapshot: map[string]DraftField{"usage_hours": {State: DraftFieldValue, Value: 1200}}},
		Question: "工时如何影响？"}
	old, err := conversation.RequestExplanation(ctx, oldCard, 0, "203.0.113.70")
	if err != nil {
		t.Fatalf("旧卡携带快照必须可解释: %v", err)
	}
	completed := awaitExplanationPostgres(t, conversation, old.ID)
	if completed.Status != OperationStatusSucceeded || completed.Explanation == nil || completed.Explanation.SnapshotID != draft.ID+"@1" {
		t.Fatalf("旧卡快照解释未完成: %+v", completed)
	}
	if _, err := conversation.RequestExplanation(ctx, &ExplainCardRequest{OperationID: "ninth", DraftVersion: 1,
		Card: oldCard.Card, Question: "工时如何影响？"}, 0, "203.0.113.70"); !errors.Is(err, ErrSuggestionQuotaExceeded) {
		t.Fatalf("建议与解释共享 8 次草稿额度: %v", err)
	}
	var calls int
	if err := pool.QueryRow(ctx, `SELECT COUNT(*) FROM valuation_explanation_calls`).Scan(&calls); err != nil {
		t.Fatal(err)
	}
	if calls != 5 {
		t.Fatalf("实际解释调用账 = %d, want 5", calls)
	}
}

func TestValuationExplanationPostgresOldDraftSnapshotSkipsCurrentDraft(t *testing.T) {
	conversation, pool, _, model := newSuggestionPostgresService(t)
	ctx := context.Background()
	model.structured = func(context.Context, string, []*schema.Message, string, aiassistant.StructuredCompleteOptions) (string, error) {
		return explanationJSON("explain", "hours_coefficient", []string{"input.usage_hours"}), nil
	}
	draft, err := conversation.CreateDraft(ctx, &CreateDraftRequest{Fields: map[string]DraftField{
		"usage_hours": {State: DraftFieldValue, Value: 1200},
	}})
	if err != nil {
		t.Fatal(err)
	}
	version := 1
	if _, err := conversation.PatchDraft(ctx, draft.ID, &PatchDraftRequest{BaseVersion: &version,
		Fields: map[string]DraftField{"usage_hours": {State: DraftFieldValue, Value: 5000}}}); err != nil {
		t.Fatal(err)
	}
	op, err := conversation.RequestExplanation(ctx, &ExplainCardRequest{OperationID: "old", DraftVersion: 1,
		Card: ExplanationCard{Kind: ExplanationCardDraft, DraftID: draft.ID,
			Snapshot: map[string]DraftField{"usage_hours": {State: DraftFieldValue, Value: 1200}}},
		Question: "工时如何影响？"}, 0, "203.0.113.73")
	if err != nil {
		t.Fatalf("旧卡必须按快照受理: %v", err)
	}
	completed := awaitExplanationPostgres(t, conversation, op.ID)
	if completed.Status != OperationStatusSucceeded || completed.Explanation == nil {
		t.Fatalf("旧卡解释未完成: %+v", completed)
	}
	if completed.Explanation.SnapshotID != draft.ID+"@1" {
		t.Fatalf("旧卡快照标识错误: %+v", completed.Explanation)
	}
	facts := map[string]any{}
	for _, fact := range completed.Explanation.Facts {
		facts[fact.ID] = fact.Value
	}
	if facts["input.usage_hours"] != float64(1200) {
		t.Fatalf("旧卡必须解释请求快照而不是当前草稿: %v", facts)
	}
	var calls int
	if err := pool.QueryRow(ctx, `SELECT COUNT(*) FROM valuation_explanation_calls WHERE card_key=$1`, draft.ID+"@1").Scan(&calls); err != nil {
		t.Fatal(err)
	}
	if calls != 1 {
		t.Fatalf("旧卡解释调用账 = %d, want 1", calls)
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
	draft, err := conversation.CreateDraft(ctx, &CreateDraftRequest{Fields: map[string]DraftField{
		"usage_hours": {State: DraftFieldValue, Value: 1200},
	}})
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

func TestValuationExplanationPostgresOldCardLateResponseNotStale(t *testing.T) {
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
	draft, err := conversation.CreateDraft(ctx, &CreateDraftRequest{Fields: map[string]DraftField{
		"usage_hours": {State: DraftFieldValue, Value: 1200},
	}})
	if err != nil {
		t.Fatal(err)
	}
	op, err := conversation.RequestExplanation(ctx, &ExplainCardRequest{OperationID: "old-late", DraftVersion: 1,
		Card: ExplanationCard{Kind: ExplanationCardDraft, DraftID: draft.ID,
			Snapshot: map[string]DraftField{"usage_hours": {State: DraftFieldValue, Value: 1200}}},
		Question: "工时如何影响？"}, 0, "203.0.113.74")
	if err != nil {
		t.Fatal(err)
	}
	<-started
	version := 1
	if _, err := conversation.PatchDraft(ctx, draft.ID, &PatchDraftRequest{BaseVersion: &version,
		Fields: map[string]DraftField{"usage_hours": {State: DraftFieldValue, Value: 9000}}}); err != nil {
		t.Fatal(err)
	}
	close(release)
	completed := awaitExplanationPostgres(t, conversation, op.ID)
	if completed.Status != OperationStatusSucceeded || completed.Explanation == nil {
		t.Fatalf("旧卡快照的晚响应不得被当前草稿判 stale: %+v", completed)
	}
	facts := map[string]any{}
	for _, fact := range completed.Explanation.Facts {
		facts[fact.ID] = fact.Value
	}
	if facts["input.usage_hours"] != float64(1200) {
		t.Fatalf("旧卡解释事实必须来自请求快照: %v", facts)
	}
}

func TestValuationExplanationPostgresTrialCardUsesLockedSnapshot(t *testing.T) {
	conversation, pool, _, model := newSuggestionPostgresService(t)
	model.structured = func(context.Context, string, []*schema.Message, string, aiassistant.StructuredCompleteOptions) (string, error) {
		return explanationJSON("explain", "interval_width", []string{"result.estimated_value", "formula.version", "assumptions.list", "candidates.list"}), nil
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
	facts := map[string]any{}
	for _, fact := range completed.Explanation.Facts {
		facts[fact.ID] = fact.Value
	}
	for _, id := range []string{"result.estimated_value", "result.width_ratio", "assumptions.count", "assumptions.list", "candidates.count", "candidates.list", "formula.version"} {
		if _, ok := facts[id]; !ok {
			t.Fatalf("试算解释缺少事实 %s: %v", id, facts)
		}
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
