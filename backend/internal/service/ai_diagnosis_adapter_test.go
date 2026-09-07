// Package service 外部诊断 RAG 助手 adapter 测试（计划 批次1）：
// httptest fake 助手钉死契约转译（chat_history array / with-image multipart JSON-string /
// 伪流式切块 / sources 透传 / 错误降级），并验证 routing adapter 按功能键分发。
package service

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/cloudwego/eino/schema"
	"go.uber.org/zap"
)

// fakeDiagnosisServer 最小助手 fake：按请求路径返回固定应答，并记录最近一次请求体。
type fakeDiagnosisServer struct {
	t            *testing.T
	sopText      string
	sources      []DiagnosisSource
	lastPath     string
	lastJSON     string // /chat 的请求体原文
	lastFormBody map[string]string
	retCode      int    // 默认 200
	retDetail    string // 非 200 时的 detail
}

func (f *fakeDiagnosisServer) handler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		f.lastPath = r.URL.Path
		f.lastFormBody = nil
		if f.retCode != 0 {
			w.WriteHeader(f.retCode)
			_, _ = w.Write([]byte(`{"detail":"` + f.retDetail + `"}`))
			return
		}
		switch f.lastPath {
		case "/assistant/api/chat":
			body, _ := io.ReadAll(r.Body)
			f.lastJSON = string(body)
		case "/assistant/api/chat/with-image":
			if err := r.ParseMultipartForm(10 << 20); err != nil {
				f.t.Fatalf("解析 multipart 失败: %v", err)
			}
			f.lastFormBody = map[string]string{}
			for k, v := range r.MultipartForm.Value {
				f.lastFormBody[k] = v[0]
			}
		default:
			f.t.Fatalf("未预期的助手路径: %s", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		resp := diagnosisChatResponse{Code: 200}
		resp.Data.SOPText = f.sopText
		resp.Data.AnswerSources = f.sources
		_ = json.NewEncoder(w).Encode(resp)
	})
}

func newDiagnosisForTest(server *httptest.Server) AIModelPort {
	return NewDiagnosisAssistantModel(server.URL, zap.NewNop())
}

// msgsSample 组装流量：system + 历史轮次 + 最后用户消息（文本，无图）。
func msgsSample(query string, extraTurns int) []*schema.Message {
	msgs := []*schema.Message{schema.SystemMessage(diagnosisSystemPrompt)}
	for i := 1; i <= extraTurns; i++ {
		msgs = append(msgs, schema.UserMessage("追问"+string(rune('0'+i))))
		msgs = append(msgs, &schema.Message{Role: schema.Assistant, Content: "前序回答"})
	}
	msgs = append(msgs, schema.UserMessage(query))
	return msgs
}

// TestDiagnosisAdapterStream_PseudostreamAndSources 文本路径：伪流式切块累加 = 全文；
// chat_history 转译正确（system 忽略、历史保留）；sources 写入 ctx 容器。
func TestDiagnosisAdapterStream_PseudostreamAndSources(t *testing.T) {
	fake := &fakeDiagnosisServer{
		t:       t,
		sopText: "## 一、检查方向\n\n先断电并实施驻车制动。\n\n## 二、工具与步骤\n\n使用万用表测量。",
		sources: []DiagnosisSource{{ID: "7", Text: "林德服务指南…", Metadata: struct {
			SourceURL string `json:"source_url"`
			PageStart int    `json:"page_start"`
			PageEnd   int    `json:"page_end"`
		}{SourceURL: "https://example.com/manual.pdf", PageStart: 12, PageEnd: 14}}},
	}
	server := httptest.NewServer(fake.handler())
	defer server.Close()
	adapter := newDiagnosisForTest(server)

	var chunks []string
	ctx := context.Background()
	callCtx := WithDiagnosisSources(ctx)
	content, usage, err := adapter.Stream(callCtx, AIModelSelector{FeatureKey: FeatureFaultDiagnosis}, msgsSample("叉车无法行驶怎么排查？", 2), func(c string) {
		chunks = append(chunks, c)
	})
	if err != nil {
		t.Fatalf("Stream 异常: %v", err)
	}
	if content != fake.sopText {
		t.Fatalf("累积内容不符:\n got=%q\nwant=%q", content, fake.sopText)
	}
	if strings.Join(chunks, "") != fake.sopText {
		t.Fatalf("伪流式切块累加 != 全文: got=%q", strings.Join(chunks, ""))
	}
	if len(chunks) < 2 {
		t.Fatalf("应按段落切出多块，got %d 块: %v", len(chunks), chunks)
	}
	if usage != nil {
		t.Fatalf("诊断适配器不应自行造 usage（闸门在装饰器）: %+v", usage)
	}
	if fake.lastPath != "/assistant/api/chat" {
		t.Fatalf("文本路径应走 /assistant/api/chat，got %s", fake.lastPath)
	}
	var req diagnosisChatRequest
	if err := json.Unmarshal([]byte(fake.lastJSON), &req); err != nil {
		t.Fatalf("解析请求体失败: %v", err)
	}
	if req.Query != "叉车无法行驶怎么排查？" {
		t.Fatalf("query 不符: %q", req.Query)
	}
	if len(req.ChatHistory) != 4 {
		t.Fatalf("chat_history 应为 4 轮（2 追问+2 回答），got %d", len(req.ChatHistory))
	}
	if req.ChatHistory[0].Role != "user" || req.ChatHistory[0].Content != "追问1" {
		t.Fatalf("chat_history 首个轮次不符: %+v", req.ChatHistory[0])
	}
	if req.ChatHistory[1].Role != "assistant" || req.ChatHistory[1].Content != "前序回答" {
		t.Fatalf("chat_history 第二个轮次不符: %+v", req.ChatHistory[1])
	}
	// sources 透传（数字 ID 原样透出字符串 "7"）
	gotSources := DiagnosisSourcesFrom(callCtx)
	if len(gotSources) != 1 || string(gotSources[0].ID) != "7" || gotSources[0].Metadata.SourceURL == "" {
		t.Fatalf("sources 透传不符: %+v", gotSources)
	}
	// 未初始化容器的 ctx 读取为空（其他功能路径安全）
	if s := DiagnosisSourcesFrom(ctx); s != nil {
		t.Fatalf("无容器的 ctx 读取应为 nil，got %v", s)
	}
}

// TestDiagnosisAdapterStream_WithImage 图片路径：base64 image part → multipart re-post；
// with-image 的 chat_history 为 JSON 字符串（契约坑）；query 为空回退默认话术。
func TestDiagnosisAdapterStream_WithImage(t *testing.T) {
	fake := &fakeDiagnosisServer{t: t, sopText: "根据现场图片分析如下…"}
	server := httptest.NewServer(fake.handler())
	defer server.Close()
	adapter := newDiagnosisForTest(server)

	imgB64 := base64.StdEncoding.EncodeToString([]byte("\x89PNG-fake-bytes"))
	last := &schema.Message{Role: schema.User, UserInputMultiContent: []schema.MessageInputPart{
		{Type: schema.ChatMessagePartTypeImageURL, Image: &schema.MessageInputImage{MessagePartCommon: schema.MessagePartCommon{Base64Data: &imgB64, MIMEType: "image/png"}}},
	}}
	// system + 1 轮历史 + 纯图最后消息（无文本）
	msgs := []*schema.Message{
		schema.SystemMessage(diagnosisSystemPrompt),
		schema.UserMessage("追问1"),
		&schema.Message{Role: schema.Assistant, Content: "前序回答"},
		last,
	}

	_, _, err := adapter.Stream(context.Background(), AIModelSelector{FeatureKey: FeatureFaultDiagnosis}, msgs, func(string) {})
	if err != nil {
		t.Fatalf("Stream 异常: %v", err)
	}
	if fake.lastPath != "/assistant/api/chat/with-image" {
		t.Fatalf("图片路径应走 /chat/with-image，got %s", fake.lastPath)
	}
	if fake.lastFormBody["query"] != "请根据现场图片进行分析" {
		t.Fatalf("空 query 应回退默认话术，got %q", fake.lastFormBody["query"])
	}
	if fake.lastFormBody["brand"] != "all" {
		t.Fatalf("brand 默认应为 all，got %q", fake.lastFormBody["brand"])
	}
	// 历史轮次经 JSON 字符串透传（system 忽略、最后一条用户消息不含在内）
	var hist []diagnosisChatTurn
	if err := json.Unmarshal([]byte(fake.lastFormBody["chat_history"]), &hist); err != nil {
		t.Fatalf("chat_history 应为 JSON 字符串，解析失败: %v", err)
	}
	if len(hist) != 2 || hist[0].Content != "追问1" || hist[1].Content != "前序回答" {
		t.Fatalf("with-image chat_history 转译不符: %+v", hist)
	}
}

// TestDiagnosisAdapterStream_BrandModel 品牌/车型经 ctx 透传（WithDiagnosisParams）。
func TestDiagnosisAdapterStream_BrandModel(t *testing.T) {
	fake := &fakeDiagnosisServer{t: t, sopText: "杭叉车型应答…"}
	server := httptest.NewServer(fake.handler())
	defer server.Close()
	adapter := newDiagnosisForTest(server)

	ctx := WithDiagnosisParams(context.Background(), "杭叉", "H3C-30")
	_, _, err := adapter.Stream(ctx, AIModelSelector{}, msgsSample("提升缓慢？", 0), func(string) {})
	if err != nil {
		t.Fatalf("Stream 异常: %v", err)
	}
	var req diagnosisChatRequest
	if err := json.Unmarshal([]byte(fake.lastJSON), &req); err != nil {
		t.Fatalf("解析请求体失败: %v", err)
	}
	if req.Brand != "杭叉" {
		t.Fatalf("brand 透传不符: %q", req.Brand)
	}
	if req.Model == nil || *req.Model != "H3C-30" {
		t.Fatalf("model 透传不符: %v", req.Model)
	}
}

// TestDiagnosisAdapterStream_Errors 错误降级：未配置 baseURL / 助手非 200 / 助手 _code 非 0。
func TestDiagnosisAdapterStream_Errors(t *testing.T) {
	// 1. 未配置
	unconf, _ := NewDiagnosisAssistantModel("", zap.NewNop()).(*diagnosisAssistantAdapter)
	if unconf.baseURL != "" {
		t.Fatal("空地址应被 trim 后保持空")
	}
	_, _, err := NewDiagnosisAssistantModel("", zap.NewNop()).Stream(context.Background(), AIModelSelector{}, msgsSample("x", 0), nil)
	if err == nil || !strings.Contains(err.Error(), "未配置") {
		t.Fatalf("未配置应返回友好错误: %v", err)
	}

	// 2. 助手返回非 200（detail 透出）
	server := httptest.NewServer((&fakeDiagnosisServer{t: t, retCode: 503, retDetail: "service overloaded"}).handler())
	defer server.Close()
	_, _, err = newDiagnosisForTest(server).Stream(context.Background(), AIModelSelector{}, msgsSample("x", 0), nil)
	if err == nil || !strings.Contains(err.Error(), "service overloaded") {
		t.Fatalf("非 200 应透出 detail: %v", err)
	}
}

// TestDiagnosisAdapterStream_BadPayload 坏包分类：空包/网关 HTML/截断 JSON
// 分别映射可行动的友好文案；截断包若 sop_text 可用则宽容抢救（标记截断不断流）。
func TestDiagnosisAdapterStream_BadPayload(t *testing.T) {
	cases := []struct {
		name string
		body string
		want string
	}{
		{"空包", "", "空响应"},
		{"网关页", "<html>502 Bad Gateway</html>", "网关异常"},
		{"截断过短仍失败", `{"code":200,"data":{"sop_text":"未闭合`, "格式异常"},
		{"业务码异常", `{"code":500,"data":{}}`, "code 500"},
		{"code字符串容忍", `{"code":"200","data":{"sop_text":"## 一、检查方向\n\n先断电并实施驻车制动。","answer_sources":[]}}`, ""},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				_, _ = w.Write([]byte(c.body))
			}))
			defer server.Close()
			content, _, err := newDiagnosisForTest(server).Stream(context.Background(), AIModelSelector{}, msgsSample("x", 0), nil)
			if c.want == "" {
				if err != nil {
					t.Fatalf("code 字符串两态应容忍，got err=%v", err)
				}
				if content == "" {
					t.Fatal("code 字符串容忍后正文不应为空")
				}
				return
			}
			if err == nil || !strings.Contains(err.Error(), c.want) {
				t.Fatalf("坏包 %q 应映射 %q，got err=%v", c.body, c.want, err)
			}
		})
	}
}

// TestDiagnosisAdapterStream_TruncatedSalvage 截断包宽容抢救：sop_text 已到达
// 部分可用 → 不判死刑，正文抢救 + 截断标记；sources 置空不可信。
func TestDiagnosisAdapterStream_TruncatedSalvage(t *testing.T) {
	body := `{"code":200,"data":{"sop_text":"## 一、检查方向\n\n先断电并实施驻车制动，测量电压是否正常，确认故障范围后再深入排查`
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(body))
	}))
	defer server.Close()
	var chunks []string
	content, _, err := newDiagnosisForTest(server).Stream(context.Background(), AIModelSelector{}, msgsSample("x", 0), func(c string) {
		chunks = append(chunks, c)
	})
	if err != nil {
		t.Fatalf("截断包应抢救不断流，got err=%v", err)
	}
	if !strings.Contains(content, "先断电并实施驻车制动") || !strings.Contains(content, "传输不完整") {
		t.Fatalf("抢救正文应含可用部分 + 截断标记，got %q", content)
	}
	if strings.Join(chunks, "") != content {
		t.Fatalf("伪流式切块累加应等于抢救全文")
	}
}

// TestDiagnosisAdapterSourceID 两态 ID（线上根因：结构化故障码来源吐 "fault-15" 字符串）：
// 数字 7 与字符串 "fault-15" 都不断流，ID 原样透出。
func TestDiagnosisAdapterSourceID(t *testing.T) {
	for _, body := range []string{
		`{"code":200,"data":{"sop_text":"## 一、检查方向\n\n先断电并实施驻车制动，测量电压确认。","answer_sources":[{"id":7,"text":"t","metadata":{"source_url":"u","page_start":1,"page_end":1}}]}}`,
		`{"code":200,"data":{"sop_text":"## 一、检查方向\n\n先断电并实施驻车制动，测量电压确认。","answer_sources":[{"id":"fault-15","text":"t","metadata":{"source_url":"u","page_start":1,"page_end":1}}]}}`,
	} {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(body))
		}))
		content, _, err := NewDiagnosisAssistantModel(server.URL, zap.NewNop()).Stream(
			WithDiagnosisSources(context.Background()), AIModelSelector{}, msgsSample("x", 0), nil)
		server.Close()
		if err != nil {
			t.Fatalf("两态 ID 不断流，body=%s err=%v", body, err)
		}
		if content == "" {
			t.Fatalf("两态 ID 正文不应为空，body=%s", body)
		}
	}
}

// TestRoutingAIModel_Dispatch 分发：fault_diagnosis → diagnosis；其余 → normal（fake 记录）。
func TestRoutingAIModel_Dispatch(t *testing.T) {
	normal := &fakeAIModelPort{content: "eino 回复"}
	server := httptest.NewServer((&fakeDiagnosisServer{t: t, sopText: "SOP 内容"}).handler())
	defer server.Close()
	diag := newDiagnosisForTest(server)
	r := NewRoutingAIModel(normal, diag)

	// 非诊断键走 eino
	c, _, err := r.Stream(context.Background(), AIModelSelector{FeatureKey: FeatureMaintenanceKnowledge}, msgsSample("x", 0), nil)
	if err != nil || c != "eino 回复" {
		t.Fatalf("non-diagnosis 应走 normal: content=%q err=%v", c, err)
	}
	if normal.streamN == 0 {
		t.Fatal("normal adapter 应被调用")
	}
	// 诊断键走 diagnosis
	c, _, err = r.Stream(context.Background(), AIModelSelector{FeatureKey: FeatureFaultDiagnosis}, msgsSample("x", 0), nil)
	if err != nil || c != "SOP 内容" {
		t.Fatalf("diagnosis 应走助手 adapter: content=%q err=%v", c, err)
	}
	if n := normal.streamN; n != 1 {
		t.Fatalf("diagnosis 不应走 normal，got streamN=%d", n)
	}
	// Complete：诊断不支持、其他走 normal
	if _, err := r.Complete(FeatureFaultDiagnosis, nil, AICompleteOptions{}); err == nil {
		t.Fatal("诊断功能 Complete 应报不支持")
	}
	if before := normal.completeN; true {
		if _, err := r.Complete(FeatureGradeShortAnswer, nil, AICompleteOptions{}); err != nil {
			t.Fatalf("非诊断 Complete 应走 normal: %v", err)
		}
		if normal.completeN <= before {
			t.Fatal("非诊断 Complete 应调用 normal adapter")
		}
	}
}
