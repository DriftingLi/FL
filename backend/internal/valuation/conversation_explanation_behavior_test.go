package valuation

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/cloudwego/eino/schema"
	"github.com/gin-gonic/gin"
	"go.uber.org/zap"

	"forklift-training/internal/aiassistant"
	"forklift-training/internal/clock"
	"forklift-training/internal/security"
	"forklift-training/internal/storage"
)

type memExplanationRepository struct {
	mu          sync.Mutex
	rows        map[string]*ConversationOperation
	suggestions *memSuggestionRepository
}

func newMemExplanationRepository(suggestions *memSuggestionRepository) *memExplanationRepository {
	return &memExplanationRepository{rows: map[string]*ConversationOperation{}, suggestions: suggestions}
}

func (m *memExplanationRepository) GetExplanation(_ context.Context, id string) (*ConversationOperation, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	op, ok := m.rows[id]
	if !ok {
		return nil, ErrOperationNotFound
	}
	copy := *op
	return &copy, nil
}

func (m *memExplanationRepository) AcceptExplanation(ctx context.Context, op *ConversationOperation, reservation SuggestionReservation, now time.Time) (*ConversationOperation, bool, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if old, ok := m.rows[op.ID]; ok {
		copy := *old
		return &copy, false, nil
	}
	if m.suggestions != nil {
		if _, _, err := m.suggestions.AcceptExplanationCall(op, reservation, now); err != nil {
			return nil, false, err
		}
	}
	copy := *op
	copy.Kind = ConversationOperationKindExplanation
	copy.Status = OperationStatusPending
	m.rows[op.ID] = &copy
	return &copy, true, nil
}

func (m *memExplanationRepository) CompleteExplanation(_ context.Context, id string, result *ValuationExplanationResult, now time.Time) (string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	op := m.rows[id]
	op.Status = OperationStatusSucceeded
	op.Explanation = result
	op.UpdatedAt = now
	return op.Status, nil
}

func (m *memExplanationRepository) FailExplanation(_ context.Context, id, code string, now time.Time) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if op := m.rows[id]; op != nil && op.Status == OperationStatusPending {
		op.Status = OperationStatusFailed
		op.ErrorCode = code
		op.UpdatedAt = now
	}
	return nil
}

func (m *memExplanationRepository) ExpireExplanation(_ context.Context, id string, olderThan, now time.Time) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if op := m.rows[id]; op != nil && op.Status == OperationStatusPending && op.UpdatedAt.Before(olderThan) {
		op.Status = OperationStatusFailed
		op.ErrorCode = ErrorCodeSuggestionTimeout
		op.UpdatedAt = now
	}
	return nil
}

func explanationJSON(action, template string, evidence []string) string {
	return `{"schema_version":1,"action":"` + action + `","template_key":"` + template + `","evidence_ids":` + mustJSON(evidence) + `}`
}

func mustJSON(v any) string {
	b, _ := json.Marshal(v)
	return string(b)
}

func newExplanationTestEngine(t *testing.T, model aiassistant.ModelPort) (*gin.Engine, *clock.Fake, *memSuggestionRepository, *memExplanationRepository) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	dict := newSeedMemDict()
	evaluations := newMemEvalStore()
	valuationSvc, err := NewService(dict, evaluations)
	if err != nil {
		t.Fatal(err)
	}
	drafts := newMemDraftRepository()
	operations := newMemOperationRepository(evaluations)
	suggestions := newMemSuggestionRepository(drafts, operations)
	explanations := newMemExplanationRepository(suggestions)
	fakeClock := clock.At(time.Date(2026, 11, 26, 10, 0, 0, 0, time.UTC))
	conversation := NewConversationService(drafts, operations, valuationSvc, evaluations, fakeClock.Now)
	cfg := defaultSuggestionConfig()
	conversation.EnableSuggestions(model, suggestions, cfg, zap.NewNop())
	conversation.EnableExplanations(model, explanations, cfg, zap.NewNop())
	sess := security.NewSession("test-secret", time.Hour, security.CookieConfig{Name: "hrwai_token"})
	r := gin.New()
	r.Use(gin.Recovery())
	attachAdminCapabilities(r)
	RegisterRoutes(r, sess, zap.NewNop(), nil, dict, evaluations, NewBatteryRepository(nil), valuationSvc,
		NewBatteryRULService(), &memReportGenerator{}, storage.NewLocalStorage(t.TempDir()), nil, nil, conversation)
	return r, fakeClock, suggestions, explanations
}

func requestExplanation(t *testing.T, r *gin.Engine, body map[string]any, auth string) *httptest.ResponseRecorder {
	t.Helper()
	return performRequestWithAuth(r, http.MethodPost, "/api/valuation/conversation/explanations", body, auth)
}

func waitForExplanation(t *testing.T, r *gin.Engine, id string) map[string]any {
	t.Helper()
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		w := performRequest(r, http.MethodGet, "/api/valuation/conversation/operations/"+id, nil)
		if w.Code != http.StatusOK {
			t.Fatalf("读取解释失败: %d %s", w.Code, w.Body.String())
		}
		_, _, data := decodeBody(t, w)
		if data["status"] != OperationStatusPending {
			return data
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatal("解释操作未结束")
	return nil
}

func TestValuationExplanationBackfillsFactsAndCounterfactualRequiresUserRecalculation(t *testing.T) {
	model := &suggestionModelFake{structured: func(context.Context, string, []*schema.Message, string, aiassistant.StructuredCompleteOptions) (string, error) {
		return explanationJSON("explain", "hours_coefficient", []string{"input.usage_hours"}), nil
	}}
	r, _, _, _ := newExplanationTestEngine(t, model)
	w := performRequest(r, http.MethodPost, "/api/valuation/conversation/drafts", map[string]any{
		"fields": map[string]any{"usage_hours": map[string]any{"state": "value", "value": 1200}},
	})
	if w.Code != http.StatusOK {
		t.Fatal(w.Body.String())
	}
	_, _, draft := decodeBody(t, w)
	version := int(draft["version"].(float64))
	w = requestExplanation(t, r, map[string]any{
		"operation_id": "counterfactual", "draft_version": version,
		"card":     map[string]any{"kind": "draft", "draft_id": draft["id"]},
		"question": "如果工时变成5000小时，会差多少钱？",
	}, "")
	if w.Code != http.StatusAccepted {
		t.Fatalf("反事实应受理: %d %s", w.Code, w.Body.String())
	}
	op := waitForExplanation(t, r, "explain:"+draft["id"].(string)+":counterfactual")
	if op["status"] != OperationStatusSucceeded {
		t.Fatalf("反事实应返回重算动作: %v", op)
	}
	result := op["explanation"].(map[string]any)
	if result["action"] != "recalculate" || result["requires_user_action"] != true {
		t.Fatalf("不得直接回答工时差价: %v", result)
	}
	if result["template_key"] != "hours_coefficient" {
		t.Fatalf("模板键错误: %v", result)
	}
	facts := map[string]any{}
	for _, raw := range result["facts"].([]any) {
		fact := raw.(map[string]any)
		facts[fact["id"].(string)] = fact["value"]
	}
	if facts["input.usage_hours"] != float64(1200) {
		t.Fatalf("工时必须来自原草稿而不是模型: %v", facts)
	}
	if _, ok := facts["result.estimated_value"]; ok {
		t.Fatalf("草稿不得凭空补写金额: %v", facts)
	}
	if _, ok := facts["invented_value"]; ok {
		t.Fatalf("模型不得增加事实: %v", facts)
	}
}

func TestValuationExplanationRejectsUnknownEvidenceAndFreeText(t *testing.T) {
	for _, tc := range []struct {
		name, content, code string
	}{
		{"引用不存在", explanationJSON("explain", "hours_coefficient", []string{"invented.value"}), ErrorCodeSuggestionValidation},
		{"自由正文", `{"schema_version":1,"action":"explain","template_key":"hours_coefficient","evidence_ids":[],"text":"残值999999"}`, ErrorCodeSuggestionValidation},
	} {
		t.Run(tc.name, func(t *testing.T) {
			model := &suggestionModelFake{structured: func(context.Context, string, []*schema.Message, string, aiassistant.StructuredCompleteOptions) (string, error) {
				return tc.content, nil
			}}
			r, _, _, _ := newExplanationTestEngine(t, model)
			draft := createConversationDraft(t, r, map[string]any{})
			w := requestExplanation(t, r, map[string]any{"operation_id": "bad", "draft_version": 1,
				"card": map[string]any{"kind": "draft", "draft_id": draft["id"]}, "question": "为什么？"}, "")
			if w.Code != http.StatusAccepted {
				t.Fatal(w.Body.String())
			}
			op := waitForExplanation(t, r, "explain:"+draft["id"].(string)+":bad")
			if op["status"] != OperationStatusFailed || op["error_code"] != tc.code {
				t.Fatalf("坏输出应固定错误码: %v", op)
			}
		})
	}
}

func TestValuationExplanationFailuresAreFixedCodes(t *testing.T) {
	for _, tc := range []struct {
		name string
		err  error
		code string
	}{
		{"超时", context.DeadlineExceeded, ErrorCodeSuggestionTimeout},
		{"不可用", errors.New("provider secret"), ErrorCodeSuggestionUnavailable},
	} {
		t.Run(tc.name, func(t *testing.T) {
			model := &suggestionModelFake{structured: func(context.Context, string, []*schema.Message, string, aiassistant.StructuredCompleteOptions) (string, error) {
				return "", tc.err
			}}
			r, _, _, _ := newExplanationTestEngine(t, model)
			draft := createConversationDraft(t, r, map[string]any{})
			requestExplanation(t, r, map[string]any{"operation_id": "failure", "draft_version": 1,
				"card": map[string]any{"kind": "draft", "draft_id": draft["id"]}, "question": "为什么？"}, "")
			op := waitForExplanation(t, r, "explain:"+draft["id"].(string)+":failure")
			if op["status"] != OperationStatusFailed || op["error_code"] != tc.code {
				t.Fatalf("错误码 = %v", op)
			}
		})
	}
}

func TestValuationExplanationCounterfactualHoursIntent(t *testing.T) {
	for _, tc := range []struct {
		question string
		want     bool
	}{
		{"为什么工时这么多？", false},
		{"为什么工时这么少？", false},
		{"工时为什么是 1200 小时？", false},
		{"2020 年出厂、工时 1200 小时，为什么残值这么低？", false},
		{"工时高所以残值低吗？", false},
		{"工时从 1200 改成 5000 会差多少钱？", true},
		{"如果工时变成 5000 小时，会差多少钱？", true},
		{"工时 1200 和 5000 差多少钱？", true},
		{"改一下工时看看差多少", true},
		{"换一个工时能差多少", true},
		{"调整工时之后残值差多少", true},
		{"工时 1200 到 5000 的区间大概差多少", true},
	} {
		if got := counterfactualHoursQuestion(tc.question); got != tc.want {
			t.Fatalf("反事实判定 %q = %v, want %v", tc.question, got, tc.want)
		}
	}
}

func TestValuationExplanationPureHoursQuestionStaysExplain(t *testing.T) {
	model := &suggestionModelFake{structured: func(context.Context, string, []*schema.Message, string, aiassistant.StructuredCompleteOptions) (string, error) {
		return explanationJSON("explain", "hours_coefficient", []string{"input.usage_hours"}), nil
	}}
	r, _, _, _ := newExplanationTestEngine(t, model)
	draft := createConversationDraft(t, r, map[string]any{
		"fields": map[string]any{"usage_hours": map[string]any{"state": "value", "value": 1200}},
	})
	w := requestExplanation(t, r, map[string]any{"operation_id": "why-many", "draft_version": 1,
		"card": map[string]any{"kind": "draft", "draft_id": draft["id"]}, "question": "为什么工时这么多？"}, "")
	if w.Code != http.StatusAccepted {
		t.Fatal(w.Body.String())
	}
	op := waitForExplanation(t, r, "explain:"+draft["id"].(string)+":why-many")
	result := op["explanation"].(map[string]any)
	if result["action"] != "explain" || result["requires_user_action"] != false {
		t.Fatalf("纯工时疑问不得误判为反事实: %v", result)
	}
}

func TestValuationExplanationSchemaOnlyAllowsTemplateAndEvidence(t *testing.T) {
	var root map[string]any
	if err := json.Unmarshal([]byte(valuationExplanationSchemaJSON), &root); err != nil {
		t.Fatal(err)
	}
	properties := root["properties"].(map[string]any)
	if len(properties) != 4 || properties["text"] != nil || properties["value"] != nil {
		t.Fatalf("解释 schema 不得接收自由正文或数值: %v", properties)
	}
	if _, err := decodeExplanationResult(`{"schema_version":1,"action":"explain","template_key":"hours_coefficient","evidence_ids":[],"estimated_value":1}`); err == nil {
		t.Fatal("必须拒绝模型编造金额")
	}
	_ = schema.UserMessage("test")
}
