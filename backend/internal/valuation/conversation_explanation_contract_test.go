package valuation

import (
	"context"
	"fmt"
	"net/http"
	"testing"

	"github.com/cloudwego/eino/schema"
	"go.uber.org/zap"

	"forklift-training/internal/aiassistant"
)

func TestValuationExplanationFormalCardRequiresOwnerAfterLogin(t *testing.T) {
	model := &suggestionModelFake{structured: func(context.Context, string, []*schema.Message, string, aiassistant.StructuredCompleteOptions) (string, error) {
		return explanationJSON("explain", "value_interval", []string{"result.estimated_value", "result.confidence_low", "result.confidence_high"}), nil
	}}
	r, _, _, _ := newExplanationTestEngine(t, model)
	draft := createCompleteConversationDraft(t, r)
	created := performRequestWithAuth(r, http.MethodPost, "/api/valuation/conversation/operations", map[string]any{
		"operation_id": "formal-source", "draft_id": draft["id"], "draft_version": 1, "request": baseEvalRequest(),
	}, authHeader(t, 7))
	if created.Code != http.StatusOK {
		t.Fatalf("创建正式卡失败: %d %s", created.Code, created.Body.String())
	}
	body := map[string]any{"operation_id": "why", "draft_version": 1,
		"card": map[string]any{"kind": "formal", "operation_id": "formal-source", "evaluation_id": float64(1)}, "question": "为什么是这个区间？"}
	if w := requestExplanation(t, r, body, authHeader(t, 8)); w.Code != http.StatusNotFound {
		t.Fatalf("越权必须 404: %d %s", w.Code, w.Body.String())
	}
	if w := requestExplanation(t, r, body, ""); w.Code != http.StatusAccepted {
		t.Fatalf("匿名 operation 能力可访问: %d %s", w.Code, w.Body.String())
	}
	op := waitForExplanation(t, r, "explain:formal-source:why")
	if op["status"] != OperationStatusSucceeded {
		t.Fatalf("正式卡解释失败: %v", op)
	}
	if w := performRequestWithAuth(r, http.MethodGet, "/api/valuation/conversation/operations/explain:formal-source:why", nil, authHeader(t, 8)); w.Code != http.StatusNotFound {
		t.Fatalf("登录读越权解释必须 404: %d %s", w.Code, w.Body.String())
	}
	if w := performRequestWithAuth(r, http.MethodGet, "/api/valuation/conversation/operations/explain:formal-source:why", nil, authHeader(t, 7)); w.Code != http.StatusOK {
		t.Fatalf("owner 可读: %d %s", w.Code, w.Body.String())
	}
}

func TestValuationExplanationOldCardAndCrossCardOperationAreIsolated(t *testing.T) {
	model := &suggestionModelFake{}
	r, _, _, _ := newExplanationTestEngine(t, model)
	first := createConversationDraft(t, r, map[string]any{})
	second := createConversationDraft(t, r, map[string]any{})
	version := 1
	if w := performRequest(r, http.MethodPatch, "/api/valuation/conversation/drafts/"+first["id"].(string), map[string]any{
		"base_version": version, "fields": map[string]any{"usage_hours": map[string]any{"state": "value", "value": 88}},
	}); w.Code != http.StatusOK {
		t.Fatal(w.Body.String())
	}
	body := map[string]any{"operation_id": "same", "draft_version": 1,
		"card": map[string]any{"kind": "draft", "draft_id": first["id"]}, "question": "工时如何影响？"}
	if w := requestExplanation(t, r, body, ""); w.Code != http.StatusConflict {
		t.Fatalf("旧卡必须 409: %d %s", w.Code, w.Body.String())
	}
	body["card"] = map[string]any{"kind": "draft", "draft_id": second["id"]}
	if w := requestExplanation(t, r, body, ""); w.Code != http.StatusAccepted {
		t.Fatalf("另一张当前卡可受理: %d %s", w.Code, w.Body.String())
	}
	waitForExplanation(t, r, "explain:"+second["id"].(string)+":same")
	if w := performRequest(r, http.MethodGet, "/api/valuation/conversation/operations/explain:"+first["id"].(string)+":same", nil); w.Code != http.StatusNotFound {
		t.Fatalf("操作 ID 不得跨卡: %d %s", w.Code, w.Body.String())
	}
}

func TestValuationExplanationRetryDoesNotRepeatModelCall(t *testing.T) {
	model := &suggestionModelFake{}
	r, _, _, _ := newExplanationTestEngine(t, model)
	draft := createConversationDraft(t, r, map[string]any{})
	body := map[string]any{"operation_id": "retry", "draft_version": 1,
		"card": map[string]any{"kind": "draft", "draft_id": draft["id"]}, "question": "为什么？"}
	if w := requestExplanation(t, r, body, ""); w.Code != http.StatusAccepted {
		t.Fatal(w.Body.String())
	}
	waitForExplanation(t, r, "explain:"+draft["id"].(string)+":retry")
	if w := requestExplanation(t, r, body, ""); w.Code != http.StatusAccepted {
		t.Fatalf("同键同负载重试: %d %s", w.Code, w.Body.String())
	}
	body["question"] = "不同问题"
	if w := requestExplanation(t, r, body, ""); w.Code != http.StatusConflict {
		t.Fatalf("同键不同负载必须 409: %d %s", w.Code, w.Body.String())
	}
	if model.calls.Load() != 1 {
		t.Fatalf("重试重复调用模型: %d", model.calls.Load())
	}
}

func TestValuationExplanationSharesSuggestionQuotaAndShutdown(t *testing.T) {
	model := &suggestionModelFake{}
	r, _, _, _ := newExplanationTestEngine(t, model)
	draft := createConversationDraft(t, r, map[string]any{})
	for i := 0; i < 7; i++ {
		if w := requestSuggestion(t, r, draft["id"].(string), fmt.Sprintf("s%d", i), 1, "整理", ""); w.Code != http.StatusAccepted {
			t.Fatal(w.Body.String())
		}
		waitForSuggestion(t, r, "op:"+draft["id"].(string)+fmt.Sprintf(":s%d", i))
	}
	body := map[string]any{"operation_id": "last", "draft_version": 1,
		"card": map[string]any{"kind": "draft", "draft_id": draft["id"]}, "question": "为什么？"}
	if w := requestExplanation(t, r, body, ""); w.Code != http.StatusAccepted {
		t.Fatalf("第 8 次共享额度应放行: %d %s", w.Code, w.Body.String())
	}
	waitForExplanation(t, r, "explain:"+draft["id"].(string)+":last")
	body["operation_id"] = "ninth"
	if w := requestExplanation(t, r, body, ""); w.Code != http.StatusTooManyRequests {
		t.Fatalf("第 9 次必须 429: %d %s", w.Code, w.Body.String())
	}
	shutdown, _, _, _ := newSuggestionTestEngine(t, model, defaultSuggestionConfig(), zap.NewNop())
	draft = createConversationDraft(t, shutdown, map[string]any{})
	body["card"] = map[string]any{"kind": "draft", "draft_id": draft["id"]}
	if w := requestExplanation(t, shutdown, body, ""); w.Code != http.StatusInternalServerError {
		t.Fatalf("关停必须固定不可用: %d %s", w.Code, w.Body.String())
	}
}
