package aiassistant

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"reflect"
	"sync/atomic"
	"testing"
	"time"

	"github.com/cloudwego/eino/schema"
	"go.uber.org/zap"
)

type structuredResolverStub struct {
	settings ChatSettings
}

func (r structuredResolverStub) ResolveFeatureSettings(context.Context, string) (ChatSettings, error) {
	return r.settings, nil
}

func (r structuredResolverStub) ResolveChatSettings(context.Context, ModelSelector) (ChatSettings, error) {
	return r.settings, nil
}

type structuredWireRequest struct {
	ResponseFormat struct {
		Type       string `json:"type"`
		JSONSchema struct {
			Name       string          `json:"name"`
			Strict     bool            `json:"strict"`
			JSONSchema json.RawMessage `json:"schema"`
		} `json:"json_schema"`
	} `json:"response_format"`
}

func writeStructuredCompletion(t *testing.T, w http.ResponseWriter) {
	t.Helper()
	w.Header().Set("Content-Type", "application/json")
	_, _ = w.Write([]byte(`{"id":"structured-test","object":"chat.completion","created":0,"model":"test-model","choices":[{"index":0,"message":{"role":"assistant","content":"{\"ok\":true}"},"finish_reason":"stop"}],"usage":{"prompt_tokens":1,"completion_tokens":1,"total_tokens":2}}`))
}

func TestCompleteStructuredSingleGenerateAndSchemaCacheSignature(t *testing.T) {
	var calls atomic.Int32
	requests := make(chan structuredWireRequest, 3)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		var payload structuredWireRequest
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			t.Errorf("解码结构化请求失败: %v", err)
		}
		requests <- payload
		writeStructuredCompletion(t, w)
	}))
	defer server.Close()

	adapter := NewEinoModel(structuredResolverStub{ChatSettings{
		APIKey: "test-key", BaseURL: server.URL, Model: "test-model", Source: "binding:test",
	}}, zap.NewNop()).(*einoAIAdapter)
	schemaA := `{"type":"object","additionalProperties":false,"required":["a"],"properties":{"a":{"type":"string"}}}`
	schemaB := `{"type":"object","additionalProperties":false,"required":["b"],"properties":{"b":{"type":"integer"}}}`
	opts := StructuredCompleteOptions{CompleteOptions: CompleteOptions{MaxTokens: 100, Temperature: 0}, Timeout: time.Second}
	for i, want := range []string{schemaA, schemaA, schemaB} {
		content, err := adapter.CompleteStructured(context.Background(), FeatureValuationDraftSuggest, []*schema.Message{schema.UserMessage("test")}, want, opts)
		if err != nil || content != `{"ok":true}` {
			t.Fatalf("结构化补全失败: content=%q err=%v", content, err)
		}
		request := <-requests
		if request.ResponseFormat.Type != "json_schema" || !request.ResponseFormat.JSONSchema.Strict {
			t.Fatalf("上游必须收到严格 json_schema: %+v", request.ResponseFormat)
		}
		var got, expected any
		if err := json.Unmarshal(request.ResponseFormat.JSONSchema.JSONSchema, &got); err != nil {
			t.Fatal(err)
		}
		if err := json.Unmarshal([]byte(want), &expected); err != nil {
			t.Fatal(err)
		}
		if !reflect.DeepEqual(got, expected) {
			t.Fatalf("上游 schema 被改写: got=%v want=%v", got, expected)
		}
		if calls.Load() != int32(i+1) {
			t.Fatalf("每次 CompleteStructured 只能产生一次请求: calls=%d expected=%d", calls.Load(), i+1)
		}
	}
	if calls.Load() != 3 {
		t.Fatalf("Generate 次数 = %d, want 3", calls.Load())
	}
}

func TestCompleteStructuredFailureHasNoHiddenRetry(t *testing.T) {
	var calls atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		calls.Add(1)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte(`{"error":{"message":"upstream unavailable"}}`))
	}))
	defer server.Close()
	adapter := NewEinoModel(structuredResolverStub{ChatSettings{APIKey: "test-key", BaseURL: server.URL, Model: "test-model"}}, zap.NewNop())
	_, err := adapter.CompleteStructured(context.Background(), FeatureValuationDraftSuggest,
		[]*schema.Message{schema.UserMessage("test")}, `{"type":"object"}`, StructuredCompleteOptions{Timeout: time.Second})
	if err == nil {
		t.Fatal("上游失败必须返回错误")
	}
	if calls.Load() != 1 {
		t.Fatalf("失败不得隐藏重试: calls=%d", calls.Load())
	}
}

func TestCompleteStructuredHonorsCancellationAndTimeout(t *testing.T) {
	started := make(chan struct{})
	release := make(chan struct{})
	var calls atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		close(started)
		select {
		case <-r.Context().Done():
		case <-release:
		}
	}))
	defer server.Close()
	adapter := NewEinoModel(structuredResolverStub{ChatSettings{APIKey: "test-key", BaseURL: server.URL, Model: "test-model"}}, zap.NewNop())
	begin := time.Now()
	_, err := adapter.CompleteStructured(context.Background(), FeatureValuationDraftSuggest,
		[]*schema.Message{schema.UserMessage("test")}, `{"type":"object"}`, StructuredCompleteOptions{Timeout: 30 * time.Millisecond})
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("超时必须映射为 context deadline: %v", err)
	}
	if elapsed := time.Since(begin); elapsed > time.Second {
		t.Fatalf("超时未及时取消上游: %v", elapsed)
	}
	<-started
	close(release)
	if calls.Load() != 1 {
		t.Fatalf("取消不得触发第二次请求: calls=%d", calls.Load())
	}
}

func TestCompleteStructuredRejectsInvalidSchemaBeforeCallingModel(t *testing.T) {
	adapter := NewEinoModel(structuredResolverStub{}, zap.NewNop())
	if _, err := adapter.CompleteStructured(context.Background(), FeatureValuationDraftSuggest,
		[]*schema.Message{schema.UserMessage("test")}, `{invalid`, StructuredCompleteOptions{}); err == nil {
		t.Fatal("非法 schema 必须在 resolver/Generate 前失败")
	}
}
