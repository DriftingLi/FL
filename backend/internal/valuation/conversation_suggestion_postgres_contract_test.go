package valuation

import (
	"context"
	"errors"
	"fmt"
	"net"
	"reflect"
	"strings"
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

func TestValuationSuggestionPostgresMergedCombinationKeepsIndependentPatches(t *testing.T) {
	conversation, pool, _, model := newSuggestionPostgresService(t)
	ctx := context.Background()
	if _, err := pool.Exec(ctx, `INSERT INTO brands (name, k_brand, is_active) VALUES ('合力', 1, true) ON CONFLICT (name) DO UPDATE SET is_active = true`); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, `INSERT INTO original_prices
		(brand, vehicle_type, series, tonnage, config_type, mast_type, mast_height_mm, original_price)
		VALUES
		('合力', '电动叉车', 'K系列', 3, '标准', '标准门架', 3000, 100000),
		('合力', '电动叉车', 'K系列', 3, '其他', '标准门架', 4000, 110000)`); err != nil {
		t.Fatal(err)
	}
	vehicleFields := map[string]DraftField{
		"brand":          {State: DraftFieldValue, Value: "合力", Source: "user"},
		"vehicle_type":   {State: DraftFieldValue, Value: "电动叉车", Source: "user"},
		"series":         {State: DraftFieldValue, Value: "K系列", Source: "user"},
		"tonnage":        {State: DraftFieldValue, Value: 3, Source: "user"},
		"config_type":    {State: DraftFieldValue, Value: "标准", Source: "user"},
		"mast_type":      {State: DraftFieldValue, Value: "标准门架", Source: "user"},
		"mast_height_mm": {State: DraftFieldValue, Value: 3000, Source: "user"},
	}
	draft, err := conversation.CreateDraft(ctx, &CreateDraftRequest{Fields: vehicleFields})
	if err != nil {
		t.Fatal(err)
	}
	model.structured = func(context.Context, string, []*schema.Message, string, aiassistant.StructuredCompleteOptions) (string, error) {
		return suggestionJSON(`[
			{"field":"mast_height_mm","state":"value","value":4000},
			{"field":"usage_hours","state":"value","value":123},
			{"field":"original_paint","state":"range","range":{"min":0,"max":1}}
		]`, `{}`, `[]`), nil
	}
	operationKey := suggestionOperationKey(draft.ID, "merged-combination")
	if _, err := conversation.RequestSuggestions(ctx, draft.ID, &SuggestDraftRequest{
		OperationID: "merged-combination", BaseVersion: 1, Text: "门架 4000，工时 123，原厂漆范围",
	}, 0, "203.0.113.10"); err != nil {
		t.Fatal(err)
	}
	operation := awaitSuggestionPostgres(t, conversation, operationKey)
	if operation.Status != OperationStatusSucceeded || operation.Result == nil {
		t.Fatalf("部分失败应成功保存合法补丁: %+v", operation)
	}
	stored, err := conversation.GetDraft(ctx, draft.ID)
	if err != nil {
		t.Fatal(err)
	}
	if stored.Version != 2 || stored.Fields["mast_height_mm"].Value != float64(3000) {
		t.Fatalf("4000 虽在级联目录中，但不属于当前完整车型组合: %+v", stored.Fields["mast_height_mm"])
	}
	if stored.Fields["usage_hours"].Value != float64(123) {
		t.Fatalf("真实 PG 合法工时补丁未独立保留: %+v", stored.Fields["usage_hours"])
	}
	if stored.Fields["original_paint"].State != DraftFieldMissing {
		t.Fatalf("布尔 range 不应落盘: %+v", stored.Fields["original_paint"])
	}
	questions := strings.Join(operation.Result.Questions, "\n")
	if !strings.Contains(questions, "mast_height_mm") || !strings.Contains(questions, "original_paint") {
		t.Fatalf("失败字段问题未随结果持久化: %v", operation.Result.Questions)
	}
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
	rows, err := pool.Query(ctx, `SELECT budget_key, used FROM valuation_suggestion_budgets WHERE scope = 'draft'`)
	if err != nil {
		t.Fatal(err)
	}
	draftUsage := map[string]int{}
	for rows.Next() {
		var key string
		var used int
		if err := rows.Scan(&key, &used); err != nil {
			t.Fatal(err)
		}
		draftUsage[key] = used
	}
	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}
	rows.Close()
	wantDraftUsage := map[string]int{draft.ID: 1, quotaDraft.ID: 8, globalDraft.ID: 2, ipDraft.ID: 3}
	if !reflect.DeepEqual(draftUsage, wantDraftUsage) {
		t.Fatalf("草稿预算串账: got=%v want=%v", draftUsage, wantDraftUsage)
	}
	var draftTotal, typedScopes int
	if err := pool.QueryRow(ctx, `SELECT COALESCE(SUM(used), 0) FROM valuation_suggestion_budgets WHERE scope = 'draft'`).Scan(&draftTotal); err != nil {
		t.Fatal(err)
	}
	if err := pool.QueryRow(ctx, `SELECT COUNT(DISTINCT scope) FROM valuation_suggestion_budgets`).Scan(&typedScopes); err != nil {
		t.Fatal(err)
	}
	if draftTotal != callCount || typedScopes != 3 {
		t.Fatalf("三本预算账未按 scope 隔离: draft=%d calls=%d scopes=%d", draftTotal, callCount, typedScopes)
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
