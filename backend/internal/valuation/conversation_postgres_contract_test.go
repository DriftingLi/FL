package valuation

import (
	"context"
	"net/http"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgxpool"
	"go.uber.org/zap"
	"gorm.io/driver/postgres"

	"forklift-training/internal/clock"
	"forklift-training/internal/security"
	"forklift-training/internal/storage"
	"forklift-training/internal/testutil"
)

func TestConversationPostgresContract(t *testing.T) {
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

	fakeClock := clock.At(time.Now().UTC())
	dict := NewDictionaryRepository(pool)
	evaluations := NewEvaluationRepository(pool)
	valuationSvc, err := NewService(dict, evaluations)
	if err != nil {
		t.Fatal(err)
	}
	conversationSvc := NewConversationService(NewDraftRepository(pool), NewOperationRepository(pool), valuationSvc, evaluations, fakeClock.Now)
	session := security.NewSession("test-secret", time.Hour, security.CookieConfig{Name: "hrwai_token"})
	r := gin.New()
	r.Use(gin.Recovery())
	attachAdminCapabilities(r)
	RegisterRoutes(r, session, zap.NewNop(), nil, dict, evaluations, NewBatteryRepository(pool), valuationSvc,
		NewBatteryRULService(), &memReportGenerator{}, storage.NewLocalStorage(t.TempDir()), nil, nil, conversationSvc)

	request := EvaluationRequest{
		Brand: "比亚迪", VehicleType: "电动平衡重式", Series: "CPD系列", Tonnage: 3,
		ConfigType: "磷酸铁锂(LFP)", MastType: "两级标准门架", MastHeightMM: 3000,
		FactoryYear: 2020, SaleYear: 2024, UsageHours: 0,
		Province: "安徽省", City: "合肥市", ConditionRating: "A",
		HasRegistrationCertificate: false,
	}
	draft := createCompleteConversationDraftForRequest(t, r, request)
	body := map[string]any{
		"operation_id": "pg-concurrent", "draft_id": draft["id"], "draft_version": 1, "request": request,
	}

	type result struct {
		code       int
		response   string
		evaluation any
	}
	const workers = 6
	results := make(chan result, workers)
	var wg sync.WaitGroup
	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			w := performRequest(r, http.MethodPost, "/api/valuation/conversation/operations", body)
			item := result{code: w.Code, response: w.Body.String()}
			if w.Code == http.StatusOK {
				_, _, data := decodeBody(t, w)
				item.evaluation = data["evaluation_id"]
			}
			results <- item
		}()
	}
	wg.Wait()
	close(results)
	var evaluationID any
	for item := range results {
		if item.code != http.StatusOK {
			t.Fatalf("真实 PG 并发同键重试失败: %d\n%s", item.code, item.response)
		}
		if evaluationID == nil {
			evaluationID = item.evaluation
		}
		if item.evaluation != evaluationID {
			t.Fatalf("真实 PG 并发重试返回不同评估: %v vs %v", item.evaluation, evaluationID)
		}
	}

	w := performRequestWithAuth(r, http.MethodPost, "/api/valuation/conversation/operations", body, authHeader(t, 7))
	if w.Code != http.StatusOK {
		t.Fatalf("登录后重试失败: %d\n%s", w.Code, w.Body.String())
	}
	_, _, replay := decodeBody(t, w)
	if replay["evaluation_id"] != evaluationID || replay["first_accepted_user_id"] != nil {
		t.Fatalf("登录重试改写匿名首次归属: %v", replay)
	}

	var (
		status, requestHash string
		errorCode           *string
		firstUserID         *int
		storedEvaluationID  int64
		evaluationUserID    *int
		evaluationCount     int
		draftCount          int
	)
	ctx := context.Background()
	if err := pool.QueryRow(ctx, `SELECT status, request_hash, first_accepted_user_id, evaluation_id, error_code
		FROM valuation_operations WHERE operation_key = $1`, "pg-concurrent").Scan(&status, &requestHash, &firstUserID, &storedEvaluationID, &errorCode); err != nil {
		t.Fatal(err)
	}
	if status != OperationStatusSucceeded || len(requestHash) != 64 || firstUserID != nil || errorCode != nil {
		t.Fatalf("operation 持久化字段错误: status=%s hash=%s user=%v error=%v", status, requestHash, firstUserID, errorCode)
	}
	if int64(evaluationID.(float64)) != storedEvaluationID {
		t.Fatalf("operation.evaluation_id 与首次结果不一致: %v vs %d", evaluationID, storedEvaluationID)
	}
	if err := pool.QueryRow(ctx, `SELECT user_id FROM evaluations WHERE id = $1`, storedEvaluationID).Scan(&evaluationUserID); err != nil {
		t.Fatal(err)
	}
	if evaluationUserID != nil {
		t.Fatalf("匿名首次受理的 evaluation 应 user_id=NULL, got %v", *evaluationUserID)
	}
	if err := pool.QueryRow(ctx, `SELECT COUNT(*) FROM evaluations`).Scan(&evaluationCount); err != nil {
		t.Fatal(err)
	}
	if evaluationCount != 1 {
		t.Fatalf("同键并发只应创建一条 evaluation, got %d", evaluationCount)
	}
	if err := pool.QueryRow(ctx, `SELECT COUNT(*) FROM ephemeral_kv WHERE cache_key = $1`, draftKey(draft["id"].(string))).Scan(&draftCount); err != nil {
		t.Fatal(err)
	}
	if draftCount != 1 {
		t.Fatalf("草稿应落在 ephemeral_kv, got %d", draftCount)
	}

	changed := request
	changed.UsageHours = 100
	body["request"] = changed
	w = performRequest(r, http.MethodPost, "/api/valuation/conversation/operations", body)
	if w.Code != http.StatusConflict {
		t.Fatalf("真实 PG 同键不同哈希应 409, got %d\n%s", w.Code, w.Body.String())
	}

	incomplete := createConversationDraft(t, r, map[string]any{})
	w = performRequest(r, http.MethodPost, "/api/valuation/conversation/operations", map[string]any{
		"operation_id": "pg-incomplete", "draft_id": incomplete["id"], "draft_version": 1, "request": request,
	})
	if w.Code != http.StatusConflict {
		t.Fatalf("不完整草稿应 409, got %d", w.Code)
	}
	if err := pool.QueryRow(ctx, `SELECT status, error_code FROM valuation_operations WHERE operation_key = $1`, "pg-incomplete").Scan(&status, &errorCode); err != nil {
		t.Fatal(err)
	}
	if status != OperationStatusFailed || errorCode == nil || *errorCode != ErrorCodeDraftIncomplete {
		t.Fatalf("失败 operation 未持久化: status=%s error=%v", status, errorCode)
	}

	assertDraftCASAndTTLOnPostgres(t, r, fakeClock)
	assertOperationEvaluationTransactionRollback(t, pool)
	assertOperationEvaluationPartialUniqueIndex(t, pool)
}

func assertDraftCASAndTTLOnPostgres(t *testing.T, r *gin.Engine, fakeClock *clock.Fake) {
	t.Helper()
	draft := createConversationDraft(t, r, map[string]any{})
	id := draft["id"].(string)
	patch := map[string]any{"base_version": 1, "fields": map[string]any{
		"brand": map[string]any{"state": "value", "value": "合力"},
	}}
	codes := make(chan int, 2)
	var wg sync.WaitGroup
	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			codes <- performRequest(r, http.MethodPatch, "/api/valuation/conversation/drafts/"+id, patch).Code
		}()
	}
	wg.Wait()
	close(codes)
	var okCount, conflictCount int
	for code := range codes {
		switch code {
		case http.StatusOK:
			okCount++
		case http.StatusConflict:
			conflictCount++
		default:
			t.Fatalf("真实 PG 草稿 CAS 返回异常状态: %d", code)
		}
	}
	if okCount != 1 || conflictCount != 1 {
		t.Fatalf("真实 PG 同版本 CAS 应一成功一冲突: ok=%d conflict=%d", okCount, conflictCount)
	}

	fakeClock.T = fakeClock.T.Add(DraftIdleTTL + time.Second)
	w := performRequest(r, http.MethodGet, "/api/valuation/conversation/drafts/"+id, nil)
	if w.Code != http.StatusNotFound {
		t.Fatalf("真实 PG 草稿闲置 30 分钟应过期, got %d", w.Code)
	}
}

func assertOperationEvaluationTransactionRollback(t *testing.T, pool *pgxpool.Pool) {
	t.Helper()
	repository := NewOperationRepository(pool)
	operation := &ConversationOperation{
		OperationID: "pg-rollback", DraftID: "draft", DraftVersion: 1,
		Status: OperationStatusPending, RequestHash: strings.Repeat("a", 64),
	}
	params := testEvalParams()
	params.Brand = strings.Repeat("x", 100)
	_, inserted, err := repository.AcceptOperation(context.Background(), operation,
		func(writer OperationEvaluationWriter) (int64, error) {
			return writer.CreateEvaluation(context.Background(), params)
		})
	if err == nil || inserted {
		t.Fatalf("evaluation 写入失败应回滚 operation 占位: inserted=%v err=%v", inserted, err)
	}
	var count int
	if err := pool.QueryRow(context.Background(), `SELECT COUNT(*) FROM valuation_operations WHERE operation_key = $1`, operation.OperationID).Scan(&count); err != nil {
		t.Fatal(err)
	}
	if count != 0 {
		t.Fatalf("回滚后不得残留 operation, got %d", count)
	}
}

func assertOperationEvaluationPartialUniqueIndex(t *testing.T, pool *pgxpool.Pool) {
	t.Helper()
	ctx := context.Background()
	for _, key := range []string{"pg-null-a", "pg-null-b"} {
		if _, err := pool.Exec(ctx, `INSERT INTO valuation_operations
			(operation_key, request_hash, draft_id, draft_version, status) VALUES ($1, $2, 'draft', 1, 'failed')`,
			key, strings.Repeat("b", 64)); err != nil {
			t.Fatal(err)
		}
	}
	for _, key := range []string{"pg-eval-a", "pg-eval-b"} {
		_, err := pool.Exec(ctx, `INSERT INTO valuation_operations
			(operation_key, request_hash, draft_id, draft_version, evaluation_id, status)
			VALUES ($1, $2, 'draft', 1, 987654321, 'succeeded')`, key, strings.Repeat("c", 64))
		if key == "pg-eval-a" && err != nil {
			t.Fatal(err)
		}
		if key == "pg-eval-b" && err == nil {
			t.Fatal("偏唯一索引未阻止 evaluation_id 重复归属")
		}
	}
}
