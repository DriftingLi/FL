package valuation

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/cloudwego/eino/schema"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"go.uber.org/zap/zaptest/observer"

	"forklift-training/internal/aiassistant"
)

func TestValuationSuggestionCanBeUndone(t *testing.T) {
	model := &suggestionModelFake{structured: func(context.Context, string, []*schema.Message, string, aiassistant.StructuredCompleteOptions) (string, error) {
		return suggestionJSON(`[{"field":"usage_hours","state":"value","value":4800}]`, `{}`, `[]`), nil
	}}
	r, _, _, _ := newSuggestionTestEngine(t, model, defaultSuggestionConfig(), zap.NewNop())
	draft := createConversationDraft(t, r, map[string]any{})
	operationID := "op:" + draft["id"].(string) + ":undo"
	requestSuggestion(t, r, draft["id"].(string), "undo", 1, "工时 4800", "")
	waitForSuggestion(t, r, operationID)
	w := performRequest(r, http.MethodPost, "/api/valuation/conversation/drafts/"+draft["id"].(string)+"/undo", nil)
	if w.Code != http.StatusOK {
		t.Fatalf("建议预填应可 undo: %d\n%s", w.Code, w.Body.String())
	}
	_, _, got := decodeBody(t, w)
	if got["version"] != float64(3) || got["fields"].(map[string]any)["usage_hours"].(map[string]any)["state"] != string(DraftFieldMissing) {
		t.Fatalf("undo 未恢复建议前状态: %v", got)
	}
}

func TestValuationSuggestionNinthCallRejectedEvenAfterLogin(t *testing.T) {
	model := &suggestionModelFake{}
	r, _, _, _ := newSuggestionTestEngine(t, model, defaultSuggestionConfig(), zap.NewNop())
	draft := createConversationDraft(t, r, map[string]any{})
	for i := 0; i < 8; i++ {
		id := fmt.Sprintf("call-%d", i)
		opID := "op:" + draft["id"].(string) + ":" + id
		w := requestSuggestion(t, r, draft["id"].(string), id, 1, "没有更多信息", "")
		if w.Code != http.StatusAccepted {
			t.Fatalf("第 %d 次应受理: %d\n%s", i+1, w.Code, w.Body.String())
		}
		waitForSuggestion(t, r, opID)
	}
	w := requestSuggestion(t, r, draft["id"].(string), "ninth", 1, "登录也不能清零", authHeader(t, 7))
	if w.Code != http.StatusTooManyRequests {
		t.Fatalf("第九次实际模型调用应 429: %d\n%s", w.Code, w.Body.String())
	}
	if model.calls.Load() != 8 {
		t.Fatalf("模型调用次数 = %d, 期望 8", model.calls.Load())
	}
}

func TestValuationSuggestionConcurrentSameOperationCallsModelOnce(t *testing.T) {
	started := make(chan struct{})
	release := make(chan struct{})
	model := &suggestionModelFake{structured: func(ctx context.Context, _ string, _ []*schema.Message, _ string, _ aiassistant.StructuredCompleteOptions) (string, error) {
		close(started)
		select {
		case <-release:
			return suggestionJSON(`[]`, `{}`, `[]`), nil
		case <-ctx.Done():
			return "", ctx.Err()
		}
	}}
	r, _, _, _ := newSuggestionTestEngine(t, model, defaultSuggestionConfig(), zap.NewNop())
	draft := createConversationDraft(t, r, map[string]any{})
	opID := "op:" + draft["id"].(string) + ":concurrent"
	const workers = 6
	codes := make(chan int, workers)
	var wg sync.WaitGroup
	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			codes <- requestSuggestion(t, r, draft["id"].(string), "concurrent", 1, "同一次请求", "").Code
		}()
	}
	wg.Wait()
	close(codes)
	for code := range codes {
		if code != http.StatusAccepted {
			t.Fatalf("并发同键请求应统一 202, got %d", code)
		}
	}
	<-started
	close(release)
	waitForSuggestion(t, r, opID)
	if model.calls.Load() != 1 {
		t.Fatalf("并发同操作只允许一次 Generate, got %d", model.calls.Load())
	}
}

func TestValuationSuggestionTimeoutKeepsManualCompletion(t *testing.T) {
	model := &suggestionModelFake{structured: func(ctx context.Context, _ string, _ []*schema.Message, _ string, _ aiassistant.StructuredCompleteOptions) (string, error) {
		<-ctx.Done()
		return "", ctx.Err()
	}}
	cfg := defaultSuggestionConfig()
	cfg.Timeout = 20 * time.Millisecond
	r, _, _, _ := newSuggestionTestEngine(t, model, cfg, zap.NewNop())
	draft := createConversationDraft(t, r, map[string]any{})
	opID := "op:" + draft["id"].(string) + ":timeout"
	requestSuggestion(t, r, draft["id"].(string), "timeout", 1, "模型超时", "")
	operation := waitForSuggestion(t, r, opID)
	if operation["status"] != OperationStatusFailed || operation["error_code"] != ErrorCodeSuggestionTimeout {
		t.Fatalf("超时应终态为 timeout: %v", operation)
	}
	w := performRequest(r, http.MethodPatch, "/api/valuation/conversation/drafts/"+draft["id"].(string), map[string]any{
		"base_version": 1,
		"fields":       map[string]any{"usage_hours": map[string]any{"state": "value", "value": 0}},
	})
	if w.Code != http.StatusOK {
		t.Fatalf("超时后仍应允许手动完成: %d\n%s", w.Code, w.Body.String())
	}
}

func TestValuationSuggestionLogsMetadataOnly(t *testing.T) {
	const secretText = "PRIVATE-BODY-DO-NOT-LOG"
	core, logs := observer.New(zapcore.DebugLevel)
	model := &suggestionModelFake{structured: func(context.Context, string, []*schema.Message, string, aiassistant.StructuredCompleteOptions) (string, error) {
		return suggestionJSON(`[{"field":"usage_hours","state":"value","value":1,"evidence":["PRIVATE-BODY-DO-NOT-LOG"]}]`, `{}`, `[]`), nil
	}}
	r, _, _, _ := newSuggestionTestEngine(t, model, defaultSuggestionConfig(), zap.New(core))
	draft := createConversationDraft(t, r, map[string]any{})
	opID := "op:" + draft["id"].(string) + ":logs"
	requestSuggestion(t, r, draft["id"].(string), "logs", 1, secretText, "")
	waitForSuggestion(t, r, opID)
	if strings.Contains(fmt.Sprint(logs.All()), secretText) {
		t.Fatalf("日志泄露建议正文: %v", logs.All())
	}
}

func TestValuationSuggestionIdempotentHashAndUnresolvedIP(t *testing.T) {
	model := &suggestionModelFake{}
	r, _, _, _ := newSuggestionTestEngine(t, model, defaultSuggestionConfig(), zap.NewNop())
	draft := createConversationDraft(t, r, map[string]any{})
	body := map[string]any{"operation_id": "same", "base_version": 1, "text": "same payload"}
	path := "/api/valuation/conversation/drafts/" + draft["id"].(string) + "/suggestions"
	w := performRequest(r, http.MethodPost, path, body)
	if w.Code != http.StatusAccepted {
		t.Fatalf("首次受理失败: %d", w.Code)
	}
	opID := "op:" + draft["id"].(string) + ":same"
	waitForSuggestion(t, r, opID)
	w = performRequest(r, http.MethodPost, path, body)
	if w.Code != http.StatusAccepted || model.calls.Load() != 1 {
		t.Fatalf("同键同负载应恢复且不重调: code=%d calls=%d", w.Code, model.calls.Load())
	}
	body["text"] = "different payload"
	w = performRequest(r, http.MethodPost, path, body)
	if w.Code != http.StatusConflict {
		t.Fatalf("同键不同负载应 409: %d\n%s", w.Code, w.Body.String())
	}
	req := httptest.NewRequest(http.MethodPost, path, strings.NewReader(`{"operation_id":"bad-ip","base_version":1,"text":"x"}`))
	req.RemoteAddr = "not-an-ip"
	req.Header.Set("Content-Type", "application/json")
	recorder := httptest.NewRecorder()
	r.ServeHTTP(recorder, req)
	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("IP 不可解析应 fail-closed 400, got %d", recorder.Code)
	}
}

func TestValuationSuggestionIPAndGlobalBudgetsAreAtomic(t *testing.T) {
	for _, tc := range []struct {
		name        string
		ipLimit     int
		globalLimit int
	}{
		{name: "ip", ipLimit: 3, globalLimit: 100},
		{name: "global", ipLimit: 100, globalLimit: 3},
	} {
		t.Run(tc.name, func(t *testing.T) {
			model := &suggestionModelFake{}
			cfg := defaultSuggestionConfig()
			cfg.IPLimit = tc.ipLimit
			cfg.GlobalLimit = tc.globalLimit
			r, _, _, repository := newSuggestionTestEngine(t, model, cfg, zap.NewNop())
			draft := createConversationDraft(t, r, map[string]any{})
			const workers = 10
			codes := make(chan int, workers)
			var wg sync.WaitGroup
			for i := 0; i < workers; i++ {
				wg.Add(1)
				go func(i int) {
					defer wg.Done()
					codes <- requestSuggestion(t, r, draft["id"].(string), fmt.Sprintf("budget-%d", i), 1, "并发预算", "").Code
				}(i)
			}
			wg.Wait()
			close(codes)
			accepted := 0
			for code := range codes {
				switch code {
				case http.StatusAccepted:
					accepted++
				case http.StatusTooManyRequests:
				default:
					t.Fatalf("并发预算响应码异常: %d", code)
				}
			}
			if accepted != 3 || model.calls.Load() != 3 {
				t.Fatalf("预算应原子放行 3 次: accepted=%d calls=%d", accepted, model.calls.Load())
			}
			repository.mu.Lock()
			defer repository.mu.Unlock()
			if len(repository.rows) != 3 || repository.draftCounts[draft["id"].(string)].used != 3 {
				t.Fatalf("失败预留必须回滚: rows=%d draft=%v", len(repository.rows), repository.draftCounts)
			}
		})
	}
}

func TestValuationSuggestionManualPatchDoesNotConsumeBudget(t *testing.T) {
	model := &suggestionModelFake{}
	r, _, _, repository := newSuggestionTestEngine(t, model, defaultSuggestionConfig(), zap.NewNop())
	draft := createConversationDraft(t, r, map[string]any{})
	opID := "op:" + draft["id"].(string) + ":before-manual"
	requestSuggestion(t, r, draft["id"].(string), "before-manual", 1, "模型调用", "")
	waitForSuggestion(t, r, opID)
	w := performRequest(r, http.MethodPatch, "/api/valuation/conversation/drafts/"+draft["id"].(string), map[string]any{
		"base_version": 1,
		"fields":       map[string]any{"usage_hours": map[string]any{"state": "value", "value": 0, "source": "user"}},
	})
	if w.Code != http.StatusOK {
		t.Fatalf("手动修改失败: %d\n%s", w.Code, w.Body.String())
	}
	repository.mu.Lock()
	defer repository.mu.Unlock()
	if model.calls.Load() != 1 || repository.draftCounts[draft["id"].(string)].used != 1 {
		t.Fatalf("手动修改不得调用模型或计数: calls=%d budgets=%v", model.calls.Load(), repository.draftCounts)
	}
}

func TestValuationSuggestionIPWindowResetsWithoutResettingDraftLimit(t *testing.T) {
	model := &suggestionModelFake{}
	cfg := defaultSuggestionConfig()
	cfg.IPLimit = 1
	cfg.IPWindow = 10 * time.Minute
	r, _, clock, _ := newSuggestionTestEngine(t, model, cfg, zap.NewNop())
	draft := createConversationDraft(t, r, map[string]any{})
	requestSuggestion(t, r, draft["id"].(string), "window-1", 1, "第一次", "")
	waitForSuggestion(t, r, "op:"+draft["id"].(string)+":window-1")
	if w := requestSuggestion(t, r, draft["id"].(string), "window-2", 1, "窗口内", ""); w.Code != http.StatusTooManyRequests {
		t.Fatalf("IP 窗口内第二次应 429: %d", w.Code)
	}
	clock.T = clock.T.Add(20 * time.Minute)
	if w := requestSuggestion(t, r, draft["id"].(string), "window-3", 1, "窗口后", ""); w.Code != http.StatusAccepted {
		t.Fatalf("IP 窗口后应恢复: %d\n%s", w.Code, w.Body.String())
	}
	waitForSuggestion(t, r, "op:"+draft["id"].(string)+":window-3")
	if model.calls.Load() != 2 {
		t.Fatalf("窗口重置只应放行一次新调用: %d", model.calls.Load())
	}
}
