package valuation

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/gin-gonic/gin"

	"forklift-training/internal/clock"
	"forklift-training/internal/storage"
)

func TestConversationOperationFirstAcceptedUserIDIsNullableInteger(t *testing.T) {
	field, ok := reflect.TypeOf(ConversationOperation{}).FieldByName("FirstAcceptedUserID")
	if !ok || field.Type != reflect.TypeOf((*int)(nil)) {
		t.Fatalf("first_accepted_user_id 必须是可空整数，got %v", field.Type)
	}
	if !strings.Contains(field.Tag.Get("extensions"), "x-nullable") {
		t.Fatalf("first_accepted_user_id 必须声明 x-nullable，got %q", field.Tag.Get("extensions"))
	}
	encoded, err := json.Marshal(ConversationOperation{})
	if err != nil {
		t.Fatal(err)
	}
	var body map[string]any
	if err := json.Unmarshal(encoded, &body); err != nil {
		t.Fatal(err)
	}
	value, exists := body["first_accepted_user_id"]
	if !exists || value != nil {
		t.Fatalf("匿名首次受理必须显式发出 null: %s", encoded)
	}
}

func TestConversationOperationErrorCodeMappingRoundTrip(t *testing.T) {
	for _, item := range []struct {
		sentinel error
		code     string
	}{
		{ErrDraftNotFound, ErrorCodeDraftNotFound},
		{ErrDraftVersionConflict, ErrorCodeDraftVersionConflict},
		{ErrDraftNotEditing, ErrorCodeDraftNotEditing},
		{ErrDraftNothingToUndo, ErrorCodeDraftNothingToUndo},
		{ErrDraftIncomplete, ErrorCodeDraftIncomplete},
		{ErrInvalidDraftField, ErrorCodeInvalidDraftField},
		{ErrInvalidOperation, ErrorCodeInvalidOperation},
		{ErrEvaluationRejected, ErrorCodeEvaluationRejected},
		{ErrOperationResultMissing, ErrorCodeOperationResultMissing},
		{ErrOperationDraftMismatch, ErrorCodeOperationDraftMismatch},
	} {
		t.Run(item.code, func(t *testing.T) {
			if got := errorCodeFor(fmt.Errorf("wrapped: %w", item.sentinel)); got != item.code {
				t.Errorf("errorCodeFor(%v) = %q, want %q", item.sentinel, got, item.code)
			}
			if got := operationError(item.code); !errors.Is(got, item.sentinel) {
				t.Errorf("operationError(%q) = %v, want %v", item.code, got, item.sentinel)
			}
		})
	}
	if got := errorCodeFor(errors.New("unmapped")); got != "" {
		t.Errorf("未知错误不得持久化为业务错误码: %q", got)
	}
	if got := operationError("unknown"); !errors.Is(got, ErrInvalidOperation) {
		t.Errorf("未知持久化错误码必须降级为 ErrInvalidOperation: %v", got)
	}
}

type reservationObservingDictionary struct {
	*memDictStore
	beforeCalculation func() error
}

func (d *reservationObservingDictionary) FindOriginalPriceMatch(ctx context.Context, brand, vehicleType, series string,
	tonnage float64, configType, mastType string, mastHeightMM int) (OriginalPrice, error) {
	if err := d.beforeCalculation(); err != nil {
		return OriginalPrice{}, err
	}
	return d.memDictStore.FindOriginalPriceMatch(ctx, brand, vehicleType, series, tonnage, configType, mastType, mastHeightMM)
}

type reservationObservingRepository struct {
	OperationRepository
	onAccept func(*ConversationOperation)
}

func (r *reservationObservingRepository) AcceptOperation(ctx context.Context, operation *ConversationOperation,
	persist func(OperationEvaluationWriter) (int64, string, error)) (*ConversationOperation, bool, error) {
	r.onAccept(operation)
	return r.OperationRepository.AcceptOperation(ctx, operation, persist)
}

func TestConversationOperationReservesFirstAcceptedUserBeforeCalculation(t *testing.T) {
	dict := &reservationObservingDictionary{memDictStore: newSeedMemDict()}
	evaluations := newMemEvalStore()
	valuationSvc, err := NewService(dict, evaluations)
	if err != nil {
		t.Fatal(err)
	}
	var accepted *ConversationOperation
	operations := &reservationObservingRepository{
		OperationRepository: newMemOperationRepository(evaluations),
		onAccept: func(operation *ConversationOperation) {
			accepted = operation
		},
	}
	dict.beforeCalculation = func() error {
		if accepted == nil {
			return errors.New("计算开始前尚未争用 operation")
		}
		if accepted.FirstAcceptedUserID == nil || *accepted.FirstAcceptedUserID != 7 {
			return fmt.Errorf("计算开始前首次受理身份未记录: %v", accepted.FirstAcceptedUserID)
		}
		return nil
	}
	svc := NewConversationService(newMemDraftRepository(), operations, valuationSvc, evaluations, time.Now)
	draft, err := svc.CreateDraft(context.Background(), &CreateDraftRequest{Fields: completeDraftFields(t, baseEvalRequest())})
	if err != nil {
		t.Fatal(err)
	}
	result, err := svc.ExecuteOperation(context.Background(), &ConversationOperationRequest{
		OperationID: "reserve-before-calculation", DraftID: draft.ID, DraftVersion: draft.Version, Request: baseEvalRequest(),
	}, 7)
	if err != nil {
		t.Fatalf("首次受理后计算失败: %v", err)
	}
	if result.FirstAcceptedUserID == nil || *result.FirstAcceptedUserID != 7 {
		t.Fatalf("首次受理身份丢失: %v", result.FirstAcceptedUserID)
	}
	if evaluations.userIDByEvaluation[result.EvaluationID] != 7 {
		t.Fatalf("正式评估未归属首次受理用户: %v", evaluations.userIDByEvaluation)
	}
}

func TestConversationOperationIdempotentReplayAndHashConflict(t *testing.T) {
	r, _, evaluations := newTestValuationEngine(t)
	draft := createCompleteConversationDraft(t, r)
	request := baseEvalRequest()
	body := map[string]any{
		"operation_id": "confirm-001", "draft_id": draft["id"], "draft_version": 1, "request": request,
	}

	w := performRequest(r, http.MethodPost, "/api/valuation/conversation/operations", body)
	if w.Code != http.StatusOK {
		t.Fatalf("确认评估状态码 = %d\nbody=%s", w.Code, w.Body.String())
	}
	_, _, first := decodeBody(t, w)
	if first["status"] != "succeeded" || first["id"] != "confirm-001" {
		t.Fatalf("首次操作结果错误: %v", first)
	}
	evaluationID := int64(first["evaluation_id"].(float64))
	if evaluationID <= 0 || first["evaluation"].(map[string]any)["id"] != float64(evaluationID) {
		t.Fatalf("操作未返回内联正式结果: %v", first)
	}

	w = performRequest(r, http.MethodPost, "/api/valuation/conversation/operations", body)
	if w.Code != http.StatusOK {
		t.Fatalf("同键同哈希重试状态码 = %d\nbody=%s", w.Code, w.Body.String())
	}
	_, _, replay := decodeBody(t, w)
	if replay["evaluation_id"] != first["evaluation_id"] {
		t.Fatalf("重试生成了第二份结果: first=%v replay=%v", first, replay)
	}
	if evaluations.count() != 1 {
		t.Fatalf("同键重试应只有一条评估, got %d", evaluations.count())
	}

	w = performRequest(r, http.MethodGet, "/api/valuation/conversation/operations/confirm-001", nil)
	if w.Code != http.StatusOK {
		t.Fatalf("读取操作状态码 = %d\nbody=%s", w.Code, w.Body.String())
	}
	_, _, detail := decodeBody(t, w)
	if detail["evaluation_id"] != first["evaluation_id"] || detail["status"] != "succeeded" {
		t.Errorf("操作详情与首次结果不一致: %v", detail)
	}

	changed := baseEvalRequest()
	changed.UsageHours = 2000
	body["request"] = changed
	w = performRequest(r, http.MethodPost, "/api/valuation/conversation/operations", body)
	if w.Code != http.StatusConflict {
		t.Fatalf("同键不同请求哈希应返回 409, got %d\nbody=%s", w.Code, w.Body.String())
	}
	if evaluations.count() != 1 {
		t.Fatalf("哈希冲突不得创建评估, got %d", evaluations.count())
	}
}

func TestConversationOperationRejectsIncompleteDraft(t *testing.T) {
	r, _, evaluations := newTestValuationEngine(t)
	draft := createConversationDraft(t, r, map[string]any{})
	w := performRequest(r, http.MethodPost, "/api/valuation/conversation/operations", map[string]any{
		"operation_id": "incomplete-001", "draft_id": draft["id"], "draft_version": 1, "request": baseEvalRequest(),
	})
	if w.Code != http.StatusConflict {
		t.Fatalf("不完整草稿不得正式提交, got %d\nbody=%s", w.Code, w.Body.String())
	}
	if evaluations.count() != 0 {
		t.Fatalf("不完整草稿生成了评估: %d", evaluations.count())
	}
}

func TestConversationOperationAnonymousOwnershipSurvivesLoginRetry(t *testing.T) {
	fakeClock := clock.At(time.Date(2026, 11, 24, 10, 0, 0, 0, time.UTC))
	r, _, evaluations, _ := newTestValuationEngineWithDrafts(t, storage.NewLocalStorage(t.TempDir()), newMemDraftRepository(), fakeClock.Now)
	draft := createCompleteConversationDraft(t, r)
	body := map[string]any{
		"operation_id": "anonymous-001", "draft_id": draft["id"], "draft_version": 1, "request": baseEvalRequest(),
	}
	w := performRequest(r, http.MethodPost, "/api/valuation/conversation/operations", body)
	if w.Code != http.StatusOK {
		t.Fatalf("匿名确认失败: %d\nbody=%s", w.Code, w.Body.String())
	}
	_, _, first := decodeBody(t, w)
	if first["first_accepted_user_id"] != nil {
		t.Fatalf("匿名首次受理 user_id 应为 null: %v", first)
	}
	id := int64(first["evaluation_id"].(float64))
	if evaluations.userIDByEvaluation[id] != 0 {
		t.Fatalf("匿名评估应落 user_id=NULL, got %d", evaluations.userIDByEvaluation[id])
	}
	fakeClock.T = fakeClock.T.Add(DraftIdleTTL + time.Second)
	w = performRequestWithAuth(r, http.MethodPost, "/api/valuation/conversation/operations", body, authHeader(t, 7))
	if w.Code != http.StatusOK {
		t.Fatalf("登录后同键重试失败: %d\nbody=%s", w.Code, w.Body.String())
	}
	_, _, replay := decodeBody(t, w)
	if replay["evaluation_id"] != first["evaluation_id"] || replay["first_accepted_user_id"] != nil {
		t.Fatalf("登录重试改写了首次匿名归属: %v", replay)
	}
	if evaluations.userIDByEvaluation[id] != 0 || evaluations.count() != 1 {
		t.Fatalf("登录重试认领了匿名记录或生成新记录: users=%v count=%d", evaluations.userIDByEvaluation, evaluations.count())
	}
}

func TestConversationOperationConcurrentRetryCreatesOneEvaluation(t *testing.T) {
	r, _, evaluations := newTestValuationEngine(t)
	draft := createCompleteConversationDraft(t, r)
	body := map[string]any{
		"operation_id": "concurrent-001", "draft_id": draft["id"], "draft_version": 1, "request": baseEvalRequest(),
	}

	const workers = 8
	codes := make(chan int, workers)
	ids := make(chan any, workers)
	var wg sync.WaitGroup
	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			w := performRequest(r, http.MethodPost, "/api/valuation/conversation/operations", body)
			codes <- w.Code
			if w.Code == http.StatusOK {
				_, _, data := decodeBody(t, w)
				ids <- data["evaluation_id"]
			}
		}()
	}
	wg.Wait()
	close(codes)
	close(ids)
	for code := range codes {
		if code != http.StatusOK {
			t.Fatalf("并发同键重试应全部返回首次结果, got %d", code)
		}
	}
	var firstID any
	for id := range ids {
		if firstID == nil {
			firstID = id
		}
		if id != firstID {
			t.Fatalf("并发重试返回了不同评估: %v vs %v", firstID, id)
		}
	}
	if evaluations.count() != 1 {
		t.Fatalf("并发同键重试只应创建一条评估, got %d", evaluations.count())
	}
}

func createCompleteConversationDraft(t *testing.T, r *gin.Engine) map[string]any {
	t.Helper()
	return createCompleteConversationDraftForRequest(t, r, baseEvalRequest())
}

func createCompleteConversationDraftForRequest(t *testing.T, r *gin.Engine, request EvaluationRequest) map[string]any {
	t.Helper()
	encoded, err := json.Marshal(request)
	if err != nil {
		t.Fatal(err)
	}
	var values map[string]any
	if err := json.Unmarshal(encoded, &values); err != nil {
		t.Fatal(err)
	}
	fields := make(map[string]any, len(values))
	for name, value := range values {
		fields[name] = map[string]any{"state": "value", "value": value, "source": "user", "evidence": "用户已核对"}
	}
	draft := createConversationDraft(t, r, map[string]any{"fields": fields})
	if draft["can_submit"] != true {
		t.Fatalf("完整明确字段草稿应可提交: %v", draft)
	}
	return draft
}
