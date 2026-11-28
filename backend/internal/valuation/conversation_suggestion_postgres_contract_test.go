package valuation

import (
	"context"
	"errors"
	"fmt"
	"net"
	"sync"
	"testing"
	"time"

	"github.com/cloudwego/eino/schema"
	"github.com/jackc/pgx/v5/pgxpool"
	"go.uber.org/zap"
	"gorm.io/driver/postgres"

	"forklift-training/internal/aiassistant"
	"forklift-training/internal/clock"
	"forklift-training/internal/testutil"
)

func newSuggestionPostgresService(t *testing.T) (*ConversationService, *pgxpool.Pool, *clock.Fake, *suggestionModelFake) {
	t.Helper()
	testutil.SetTestGinMode()
	db := testutil.NewPostgresDB(t)
	dialector, ok := db.Dialector.(*postgres.Dialector)
	if !ok {
		t.Fatalf("测试库不是 PostgreSQL: %T", db.Dialector)
	}
	pool, err := pgxpool.New(context.Background(), dialector.DSN)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)
	dict := NewDictionaryRepository(pool)
	evaluations := NewEvaluationRepository(pool)
	valuationSvc, err := NewService(dict, evaluations)
	if err != nil {
		t.Fatal(err)
	}
	fakeClock := clock.At(time.Now().UTC())
	conversation := NewConversationService(NewDraftRepository(pool), NewOperationRepository(pool), valuationSvc, evaluations, fakeClock.Now)
	model := &suggestionModelFake{}
	conversation.EnableSuggestions(model, NewSuggestionRepository(pool), defaultSuggestionConfig(), zap.NewNop())
	return conversation, pool, fakeClock, model
}

func awaitSuggestionPostgres(t *testing.T, conversation *ConversationService, operationKey string) *ConversationOperation {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		operation, err := conversation.suggestions.repository.GetSuggestion(context.Background(), operationKey)
		if err != nil {
			t.Fatal(err)
		}
		if operation.Status != OperationStatusPending {
			return operation
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatal("PostgreSQL 建议操作未在预算内结束")
	return nil
}

func TestValuationSuggestionPostgresAtomicQuotaAndIdempotency(t *testing.T) {
	conversation, pool, _, model := newSuggestionPostgresService(t)
	ctx := context.Background()
	ipHash := suggestionIPHash(conversation.suggestions.config.HMACKey, net.ParseIP("203.0.113.8"))
	draft, err := conversation.CreateDraft(ctx, &CreateDraftRequest{})
	if err != nil {
		t.Fatal(err)
	}
	request := &SuggestDraftRequest{OperationID: "same", BaseVersion: 1, Text: "并发同键"}
	const sameWorkers = 12
	codes := make(chan error, sameWorkers)
	var wg sync.WaitGroup
	for i := 0; i < sameWorkers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, err := conversation.RequestSuggestions(ctx, draft.ID, request, 0, "203.0.113.8")
			codes <- err
		}()
	}
	wg.Wait()
	close(codes)
	for err := range codes {
		if err != nil {
			t.Fatalf("真实 PG 并发同键应统一受理: %v", err)
		}
	}
	operationKey := suggestionOperationKey(draft.ID, "same")
	operation := awaitSuggestionPostgres(t, conversation, operationKey)
	if operation.Status != OperationStatusSucceeded || model.calls.Load() != 1 {
		t.Fatalf("同键并发只允许一次模型调用: status=%s calls=%d", operation.Status, model.calls.Load())
	}
	if _, err := conversation.RequestSuggestions(ctx, draft.ID, request, 7, "203.0.113.8"); err != nil {
		t.Fatalf("登录后同键重试应恢复: %v", err)
	}
	if model.calls.Load() != 1 {
		t.Fatalf("同键重试重复调用模型: %d", model.calls.Load())
	}
	version := 1
	if _, err := conversation.PatchDraft(ctx, draft.ID, &PatchDraftRequest{
		BaseVersion: &version,
		Fields:      map[string]DraftField{"usage_hours": {State: DraftFieldValue, Value: 0, Source: "user"}},
	}); err != nil {
		t.Fatalf("手动修改失败: %v", err)
	}

	quotaDraft, err := conversation.CreateDraft(ctx, &CreateDraftRequest{})
	if err != nil {
		t.Fatal(err)
	}
	const distinctWorkers = 16
	accepted := make(chan error, distinctWorkers)
	for i := 0; i < distinctWorkers; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			_, err := conversation.RequestSuggestions(ctx, quotaDraft.ID, &SuggestDraftRequest{
				OperationID: fmt.Sprintf("draft-budget-%d", i), BaseVersion: 1, Text: "草稿预算",
			}, 0, "203.0.113.8")
			accepted <- err
		}(i)
	}
	wg.Wait()
	close(accepted)
	acceptedCount := 0
	for err := range accepted {
		if err == nil {
			acceptedCount++
			continue
		}
		if !errors.Is(err, ErrSuggestionQuotaExceeded) {
			t.Fatalf("草稿预算耗尽应返回配额错误: %v", err)
		}
	}
	if acceptedCount != 8 {
		t.Fatalf("真实 PG 草稿 8 次预算应原子放行 8 次: %d", acceptedCount)
	}

	var globalUsed, ipUsed int
	if err := pool.QueryRow(ctx, `SELECT used FROM valuation_suggestion_budgets WHERE scope = 'global' AND budget_key = 'global'`).Scan(&globalUsed); err != nil {
		t.Fatal(err)
	}
	if err := pool.QueryRow(ctx, `SELECT used FROM valuation_suggestion_budgets WHERE scope = 'ip' AND budget_key = $1`, ipHash).Scan(&ipUsed); err != nil {
		t.Fatal(err)
	}
	conversation.suggestions.config.GlobalLimit = globalUsed + 2
	globalDraft, err := conversation.CreateDraft(ctx, &CreateDraftRequest{})
	if err != nil {
		t.Fatal(err)
	}
	accepted = make(chan error, 8)
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			_, err := conversation.RequestSuggestions(ctx, globalDraft.ID, &SuggestDraftRequest{
				OperationID: fmt.Sprintf("global-budget-%d", i), BaseVersion: 1, Text: "全站预算",
			}, 0, "203.0.113.8")
			accepted <- err
		}(i)
	}
	wg.Wait()
	close(accepted)
	acceptedCount = 0
	for err := range accepted {
		if err == nil {
			acceptedCount++
		} else if !errors.Is(err, ErrSuggestionQuotaExceeded) {
			t.Fatalf("全站预算错误类型异常: %v", err)
		}
	}
	if acceptedCount != 2 {
		t.Fatalf("真实 PG 全站预算应原子放行 2 次: %d", acceptedCount)
	}

	conversation.suggestions.config.GlobalLimit = 1000
	conversation.suggestions.config.IPLimit = ipUsed + 5
	ipDraft, err := conversation.CreateDraft(ctx, &CreateDraftRequest{})
	if err != nil {
		t.Fatal(err)
	}
	accepted = make(chan error, 6)
	for i := 0; i < 6; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			_, err := conversation.RequestSuggestions(ctx, ipDraft.ID, &SuggestDraftRequest{
				OperationID: fmt.Sprintf("ip-budget-%d", i), BaseVersion: 1, Text: "IP 预算",
			}, 0, "203.0.113.8")
			accepted <- err
		}(i)
	}
	wg.Wait()
	close(accepted)
	acceptedCount = 0
	for err := range accepted {
		if err == nil {
			acceptedCount++
		} else if !errors.Is(err, ErrSuggestionQuotaExceeded) {
			t.Fatalf("IP 预算错误类型异常: %v", err)
		}
	}
	if acceptedCount != 3 {
		t.Fatalf("真实 PG IP 预算应原子放行 3 次: %d", acceptedCount)
	}

	var callCount, storedGlobal, storedIP int
	if err := pool.QueryRow(ctx, `SELECT COUNT(*) FROM valuation_suggestion_calls`).Scan(&callCount); err != nil {
		t.Fatal(err)
	}
	if err := pool.QueryRow(ctx, `SELECT used FROM valuation_suggestion_budgets WHERE scope = 'global' AND budget_key = 'global'`).Scan(&storedGlobal); err != nil {
		t.Fatal(err)
	}
	if err := pool.QueryRow(ctx, `SELECT used FROM valuation_suggestion_budgets WHERE scope = 'ip' AND budget_key = $1`, ipHash).Scan(&storedIP); err != nil {
		t.Fatal(err)
	}
	if callCount != 14 || storedGlobal != 14 || storedIP != 14 || model.calls.Load() != 14 {
		t.Fatalf("实际调用与三本预算账不一致: calls=%d model=%d global=%d ip=%d", callCount, model.calls.Load(), storedGlobal, storedIP)
	}
}

func TestValuationSuggestionPostgresLateResponseIsStale(t *testing.T) {
	conversation, _, _, _ := newSuggestionPostgresService(t)
	ctx := context.Background()
	started := make(chan struct{})
	release := make(chan struct{})
	model := &suggestionModelFake{structured: func(ctx context.Context, _ string, _ []*schema.Message, _ string, _ aiassistant.StructuredCompleteOptions) (string, error) {
		close(started)
		select {
		case <-release:
			return suggestionJSON(`[{"field":"usage_hours","state":"value","value":9900}]`, `{}`, `[]`), nil
		case <-ctx.Done():
			return "", ctx.Err()
		}
	}}
	conversation.suggestions.model = model
	draft, err := conversation.CreateDraft(ctx, &CreateDraftRequest{})
	if err != nil {
		t.Fatal(err)
	}
	operationKey := suggestionOperationKey(draft.ID, "late")
	if _, err := conversation.RequestSuggestions(ctx, draft.ID, &SuggestDraftRequest{OperationID: "late", BaseVersion: 1, Text: "晚响应"}, 0, "203.0.113.9"); err != nil {
		t.Fatal(err)
	}
	<-started
	version := 1
	if _, err := conversation.PatchDraft(ctx, draft.ID, &PatchDraftRequest{
		BaseVersion: &version,
		Fields:      map[string]DraftField{"usage_hours": {State: DraftFieldValue, Value: 1234, Source: "user"}},
	}); err != nil {
		t.Fatal(err)
	}
	close(release)
	operation := awaitSuggestionPostgres(t, conversation, operationKey)
	if operation.Status != OperationStatusStale || operation.ErrorCode != ErrorCodeDraftVersionConflict {
		t.Fatalf("真实 PG 晚响应必须 stale: %+v", operation)
	}
	stored, err := conversation.GetDraft(ctx, draft.ID)
	if err != nil {
		t.Fatal(err)
	}
	if stored.Version != 2 {
		t.Fatalf("晚响应覆盖了用户新草稿版本: %+v", stored)
	}
	if hours, ok := draftNumberValue(stored, "usage_hours"); !ok || hours != 1234 {
		t.Fatalf("晚响应覆盖了用户新草稿工时: %+v", stored)
	}
}
