package aiassistant

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"github.com/cloudwego/eino/schema"
	"github.com/gin-gonic/gin"
)

// ssePort 复刻 adapter 的两种返回：增量推给浏览器、返回的是归一后的权威全文。
// 真实场景里增量含第三方图片 URL、返回值已被 normalizeDiagnosisImages 改成本站代理。
type ssePort struct {
	mu     sync.Mutex
	chunks []string
	final  string
}

func (p *ssePort) Complete(string, []*schema.Message, CompleteOptions) (string, error) {
	return p.final, nil
}

func (p *ssePort) Stream(_ context.Context, _ ModelSelector, _ []*schema.Message, onChunk func(string)) (string, *Usage, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	for _, c := range p.chunks {
		if onChunk != nil {
			onChunk(c)
		}
	}
	return p.final, nil, nil
}

// driveChat 起一个最小 gin 引擎跑 StreamChat，返回原始 SSE 响应体。
func driveChat(t *testing.T, port ModelPort) string {
	t.Helper()
	gin.SetMode(gin.TestMode)
	svc := &Service{port: port, logger: nil}
	h := newHandler(svc)

	r := gin.New()
	r.POST("/chat", h.StreamChat)

	body, _ := json.Marshal(StreamChatReq{
		FeatureKey: FeatureFaultDiagnosis,
		Messages:   []StreamMessage{{Role: "user", Content: "不举升"}},
	})
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/chat", strings.NewReader(string(body)))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)
	return w.Body.String()
}

// doneFrame 取 SSE 里 done 事件的 data 行；空载荷返回空串。
func doneFrame(t *testing.T, raw string) string {
	t.Helper()
	lines := strings.Split(raw, "\n")
	for i, ln := range lines {
		if strings.TrimSpace(ln) == "event: done" {
			for _, nxt := range lines[i+1:] {
				if v, ok := strings.CutPrefix(strings.TrimSpace(nxt), "data:"); ok {
					return strings.TrimSpace(v)
				}
			}
		}
	}
	t.Fatalf("SSE 里没有 done 事件：%q", raw)
	return ""
}

// TestStreamChatDoneCarriesAuthoritativeContent 票 2312：增量与权威全文不一致时，
// done 必须补发 content —— 否则诊断配图的归一结果只进数据库、浏览器仍直连第三方主机。
func TestStreamChatDoneCarriesAuthoritativeContent(t *testing.T) {
	port := &ssePort{
		chunks: []string{"看图 ![结构图](https://cdn.example.com/a.png) 结束"},
		final:  "看图 结构图 结束",
	}
	payload := doneFrame(t, driveChat(t, port))

	var got struct {
		Content string             `json:"content"`
		Meta    *AIChatMessageMeta `json:"meta"`
	}
	if err := json.Unmarshal([]byte(payload), &got); err != nil {
		t.Fatalf("done 载荷不是 JSON: %v (%q)", err, payload)
	}
	if got.Content != "看图 结构图 结束" {
		t.Fatalf("done 应携带归一后的权威全文，得到 %q", got.Content)
	}
}

// TestStreamChatDoneOmitsContentWhenIdentical 普通对话增量与全文恒等：
// done 不应多发一个字节（否则每个对话都多一份完整正文的传输与解析成本）。
func TestStreamChatDoneOmitsContentWhenIdentical(t *testing.T) {
	port := &ssePort{
		chunks: []string{"第一段", "第二段"},
		final:  "第一段第二段",
	}
	payload := doneFrame(t, driveChat(t, port))

	if payload == "" || payload == "null" {
		return // 空载荷即正确：没有 meta、也没有需要补发的全文
	}
	var got map[string]any
	if err := json.Unmarshal([]byte(payload), &got); err != nil {
		t.Fatalf("done 载荷不是 JSON: %v (%q)", err, payload)
	}
	if _, ok := got["content"]; ok {
		t.Fatalf("增量与全文一致时 done 不应带 content：%q", payload)
	}
}
