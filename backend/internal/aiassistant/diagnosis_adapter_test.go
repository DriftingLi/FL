// 外部诊断 RAG 助手 adapter 测试（v2 单轨，票 2237）：
// httptest fake 助手钉死 v2 契约转译（SSE 帧格式 / content 单文本通道 / images dataURL 数组 /
// status 下沉 / done 的 answer 为权威并跑归一 / badges+elapsed 进元数据 / 错误分类）。
package aiassistant

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/cloudwego/eino/schema"
	"go.uber.org/zap"
)

const testOSSBase = "https://hrwai-guangzhou.oss-cn-beijing.aliyuncs.com/forklift/repair_img"

// fakeDiagnosisServer 最小 v2 助手 fake：按请求路径返回固定 SSE 帧，并记录最近一次请求体。
type fakeDiagnosisServer struct {
	t         *testing.T
	frames    []string // 每项是 SSE 帧的 JSON 体（不带 data: 前缀）
	raw       string   // 非空则原样回写这些字节（绕开「用 Go 类型回编码」的盲区）
	status    int      // 非 0 或非 200 时走 HTTP 失败分支
	detail    string   // 失败分支的 detail 文本
	lastPath  string
	lastJSON  string
	lastKey   string
	lastAccep string
}

func (f *fakeDiagnosisServer) handler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		f.lastPath = r.URL.Path
		f.lastKey = r.Header.Get("X-API-Key")
		f.lastAccep = r.Header.Get("Accept")
		body, _ := io.ReadAll(r.Body)
		f.lastJSON = string(body)

		if f.status != 0 && f.status != http.StatusOK {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(f.status)
			_, _ = w.Write([]byte(`{"detail":"` + f.detail + `"}`))
			return
		}
		if f.lastPath != "/api/v2/chat" {
			f.t.Errorf("未预期的助手路径: %s", f.lastPath)
			w.WriteHeader(http.StatusNotFound)
			return
		}
		w.Header().Set("Content-Type", "text/event-stream")
		if f.raw != "" {
			// 真实响应字节原样回写：厂商改帧形状时这条路径会红，而按 Go 类型回编码的
			// 老路数只会让「解析不出字段」伪装成「字段为空」而恒绿。
			_, _ = w.Write([]byte(f.raw))
			return
		}
		for _, frame := range f.frames {
			_, _ = fmt.Fprintf(w, "data: %s\n\n", frame)
		}
	})
}

func newDiagnosisForTest(server *httptest.Server) ModelPort {
	return NewDiagnosisAssistantModel(server.URL, "test-key", testOSSBase, zap.NewNop())
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

// TestDiagnosisAdapterStream_SSEAndMeta status 下沉、delta 只为观感、done 的 answer 为权威、
// badges/elapsed 进元数据；请求体是 v2 形状且带 X-API-Key。
func TestDiagnosisAdapterStream_SSEAndMeta(t *testing.T) {
	answer := "## 一、检查方向\n\n先断电并实施驻车制动。\n\n## 二、工具与步骤\n\n使用万用表测量。"
	fake := &fakeDiagnosisServer{t: t, frames: []string{
		`{"type":"status","text":"正在检索维修知识库…"}`,
		`{"type":"delta","text":"## 一、检查方向\n\n"}`,
		`{"type":"ping"}`,
		`{"type":"delta","text":"先断电并实施驻车制动。"}`,
		// 关键：done 的 answer 与前面的 delta **不是同一版本**（delta 未清洗，answer 是清洗后的权威全文）
		`{"type":"done","answer":` + mustJSON(t, answer) + `,"badges":[{"name":"知识检索","emoji":"📚","cls":"cap-rag"}],"trace":["a","b"],"elapsed":12.5}`,
	}}
	server := httptest.NewServer(fake.handler())
	defer server.Close()

	sink := &recordingSink{}
	content, usage, err := newDiagnosisForTest(server).Stream(context.Background(), ModelSelector{FeatureKey: FeatureFaultDiagnosis},
		msgsSample("叉车无法行驶怎么排查？", 2), sink)
	if err != nil {
		t.Fatalf("Stream 异常: %v", err)
	}
	if content != answer {
		t.Fatalf("正文应以 done 的 answer 为权威:\n got=%q\nwant=%q", content, answer)
	}
	if usage != nil {
		t.Fatalf("诊断适配器不应自行造 usage（闸门在装饰器）: %+v", usage)
	}
	if len(sink.statuses) != 1 || !strings.Contains(sink.statuses[0], "正在检索") {
		t.Fatalf("status 应即刻下沉: %v", sink.statuses)
	}
	if len(sink.chunks) != 2 {
		t.Fatalf("delta 应逐帧透传（ping 忽略），got %d 块: %v", len(sink.chunks), sink.chunks)
	}

	// 元数据：徽章三字段原样、elapsed 保留、trace 不收
	if len(sink.metas) == 0 {
		t.Fatal("done 应产出元数据")
	}
	meta := sink.metas[0]
	if len(meta.Badges) != 1 || meta.Badges[0].Name != "知识检索" || meta.Badges[0].Emoji != "📚" || meta.Badges[0].Cls != "cap-rag" {
		t.Fatalf("徽章应原样三字段: %+v", meta.Badges)
	}
	if meta.Elapsed != 12.5 {
		t.Fatalf("elapsed 不符: %v", meta.Elapsed)
	}

	// 请求形状：v2 只有 content + images，且带鉴权头
	if fake.lastKey != "test-key" {
		t.Fatalf("应带 X-API-Key，got %q", fake.lastKey)
	}
	if fake.lastAccep != "text/event-stream" {
		t.Fatalf("Accept 应为 text/event-stream，got %q", fake.lastAccep)
	}
	var req diagnosisChatRequest
	if err := json.Unmarshal([]byte(fake.lastJSON), &req); err != nil {
		t.Fatalf("解析请求体失败: %v", err)
	}
	if !strings.Contains(req.Content, "叉车无法行驶怎么排查？") || !strings.Contains(req.Content, "[对话上下文]") {
		t.Fatalf("content 应折叠历史上下文: %q", req.Content)
	}
	if len(req.Images) != 0 {
		t.Fatalf("无图轮次不应带 images，got %v", req.Images)
	}
}

// mustJSON 把字符串编成 JSON 字面量（帧体拼接用）。
func mustJSON(t *testing.T, v any) string {
	t.Helper()
	b, err := json.Marshal(v)
	if err != nil {
		t.Fatalf("json.Marshal 失败: %v", err)
	}
	return string(b)
}

// TestDiagnosisAdapterStream_Images 图片：base64 part → dataURL 数组；纯图片轮次回退默认话术；
// 超 3 张截断并在正文尾追加提示；单张超 5MB 前置拒绝。
func TestDiagnosisAdapterStream_Images(t *testing.T) {
	newImgs := func(count int, size int) []*schema.Message {
		msgs := []*schema.Message{schema.SystemMessage(diagnosisSystemPrompt)}
		last := &schema.Message{Role: schema.User}
		for i := 0; i < count; i++ {
			b64 := base64.StdEncoding.EncodeToString(make([]byte, size))
			last.UserInputMultiContent = append(last.UserInputMultiContent, schema.MessageInputPart{
				Type:  schema.ChatMessagePartTypeImageURL,
				Image: &schema.MessageInputImage{MessagePartCommon: schema.MessagePartCommon{Base64Data: &b64, MIMEType: "image/png"}},
			})
		}
		return append(msgs, last)
	}

	t.Run("纯图片回退默认话术", func(t *testing.T) {
		fake := &fakeDiagnosisServer{t: t, frames: []string{`{"type":"done","answer":"图里是液压泵渗油。"}`}}
		server := httptest.NewServer(fake.handler())
		defer server.Close()

		_, _, err := newDiagnosisForTest(server).Stream(context.Background(), ModelSelector{}, newImgs(1, 16), nil)
		if err != nil {
			t.Fatalf("Stream 异常: %v", err)
		}
		var req diagnosisChatRequest
		if err := json.Unmarshal([]byte(fake.lastJSON), &req); err != nil {
			t.Fatalf("解析请求体失败: %v", err)
		}
		if !strings.HasPrefix(req.Content, "请根据现场图片进行分析") {
			t.Fatalf("纯图片轮次应回退默认话术，got %q", req.Content)
		}
		if len(req.Images) != 1 || !strings.HasPrefix(req.Images[0], "data:image/png;base64,") {
			t.Fatalf("images 应为 dataURL，got %v", req.Images)
		}
	})

	t.Run("超三张截断", func(t *testing.T) {
		fake := &fakeDiagnosisServer{t: t, frames: []string{`{"type":"done","answer":"ok"}`}}
		server := httptest.NewServer(fake.handler())
		defer server.Close()

		_, _, err := newDiagnosisForTest(server).Stream(context.Background(), ModelSelector{}, newImgs(5, 16), nil)
		if err != nil {
			t.Fatalf("Stream 异常: %v", err)
		}
		var req diagnosisChatRequest
		if err := json.Unmarshal([]byte(fake.lastJSON), &req); err != nil {
			t.Fatalf("解析请求体失败: %v", err)
		}
		if len(req.Images) != diagnosisMaxImages {
			t.Fatalf("应只送前 %d 张，got %d", diagnosisMaxImages, len(req.Images))
		}
		if !strings.Contains(req.Content, "仅前 3 张送达") {
			t.Fatalf("截断应有提示行，got %q", req.Content)
		}
	})

	t.Run("单张超限拒绝", func(t *testing.T) {
		fake := &fakeDiagnosisServer{t: t, frames: []string{`{"type":"done","answer":"ok"}`}}
		server := httptest.NewServer(fake.handler())
		defer server.Close()

		_, _, err := newDiagnosisForTest(server).Stream(context.Background(), ModelSelector{},
			newImgs(1, diagnosisMaxImageCap+1), nil)
		if err == nil || !strings.Contains(err.Error(), "5MB") {
			t.Fatalf("超限应前置拒绝，got %v", err)
		}
		if fake.lastJSON != "" {
			t.Fatal("超限不应发出请求")
		}
	})
}

// TestFoldHistoryIntoQuery 折叠预算：空历史透传、轮次上限、单轮截断、总预算、原 query 保留在末尾。
func TestFoldHistoryIntoQuery(t *testing.T) {
	if got := foldHistoryIntoQuery(nil, "本轮问题"); got != "本轮问题" {
		t.Fatalf("无历史应原样透传，got %q", got)
	}
	history := make([]diagnosisChatTurn, 0, foldMaxTurns+2)
	for i := 0; i < foldMaxTurns+2; i++ {
		role := "user"
		if i%2 == 1 {
			role = "assistant"
		}
		history = append(history, diagnosisChatTurn{Role: role, Content: strings.Repeat("长", foldMaxTurnRunes+50)})
	}
	got := foldHistoryIntoQuery(history, "具体步骤是什么？")
	if strings.Count(got, "长") > foldMaxTurns*foldMaxTurnRunes {
		t.Fatalf("折叠内容超预算")
	}
	if !strings.Contains(got, "[对话上下文]") || !strings.Contains(got, "[本轮问题]") {
		t.Fatalf("折叠格式缺失标记: %q", got[:120])
	}
	if !strings.HasSuffix(got, "[本轮问题]（请结合上文理解本轮追问；勿重复回答历史问题）\n具体步骤是什么？") {
		t.Fatalf("原 query 应在折叠文本末尾，got tail: %q", got[len(got)-80:])
	}
}

// TestDiagnosisAdapterStream_Errors 错误分类：未配置 / 401 / 403 两态 / 429 / 400 / 502 / 413 /
// 未知状态 / SSE error 帧。
func TestDiagnosisAdapterStream_Errors(t *testing.T) {
	t.Run("未配置", func(t *testing.T) {
		unconf, _ := NewDiagnosisAssistantModel("", "", "", zap.NewNop()).(*diagnosisAssistantAdapter)
		if unconf.baseURL != "" {
			t.Fatal("空地址应被 trim 后保持空")
		}
		_, _, err := unconf.Stream(context.Background(), ModelSelector{}, msgsSample("x", 0), nil)
		if err == nil || !strings.Contains(err.Error(), "未配置") {
			t.Fatalf("未配置应返回友好错误: %v", err)
		}
	})

	cases := []struct {
		name   string
		status int
		detail string
		want   string
	}{
		{"401 缺 Key", http.StatusUnauthorized, "", "鉴权失败"},
		{"403 IP", http.StatusForbidden, "IP 不在白名单", "白名单"},
		{"403 停用", http.StatusForbidden, "Key 已停用", "已停用"},
		{"403 分不出", http.StatusForbidden, "", "访问被拒绝"},
		{"429", http.StatusTooManyRequests, "", "过于频繁"},
		{"400 图片超限", http.StatusBadRequest, "图片超过 5MB", "图片超过 5MB"},
		{"502 上游", http.StatusBadGateway, "百炼超时", "百炼超时"},
		{"413", http.StatusRequestEntityTooLarge, "", "过大"},
		{"未知状态", http.StatusTeapot, "", "HTTP 418"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			server := httptest.NewServer((&fakeDiagnosisServer{t: t, status: c.status, detail: c.detail}).handler())
			defer server.Close()
			_, _, err := newDiagnosisForTest(server).Stream(context.Background(), ModelSelector{}, msgsSample("x", 0), nil)
			if err == nil || !strings.Contains(err.Error(), c.want) {
				t.Fatalf("HTTP %d 应映射 %q，got %v", c.status, c.want, err)
			}
		})
	}

	t.Run("SSE error 帧", func(t *testing.T) {
		server := httptest.NewServer((&fakeDiagnosisServer{t: t, frames: []string{
			`{"type":"error","message":"知识库暂时不可用"}`,
		}}).handler())
		defer server.Close()
		_, _, err := newDiagnosisForTest(server).Stream(context.Background(), ModelSelector{}, msgsSample("x", 0), nil)
		if err == nil || !strings.Contains(err.Error(), "知识库暂时不可用") {
			t.Fatalf("error 帧文案应透出，got %v", err)
		}
	})
}

// TestV2ErrorDetail 422 的 detail 是数组（契约 §1.4），不能被当成字符串而丢文案。
func TestV2ErrorDetail(t *testing.T) {
	cases := []struct {
		name string
		body string
		want string
	}{
		{"字符串 detail", `{"detail":"图片超过 5MB"}`, "图片超过 5MB"},
		{"数组 detail", `{"detail":[{"loc":["body","images"],"msg":"图片格式不支持","type":"value_error"}]}`, "图片格式不支持"},
		{"无 detail", `{"oops":1}`, ""},
		{"非 JSON", `<html>502</html>`, ""},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := v2ErrorDetail([]byte(c.body)); got != c.want {
				t.Fatalf("got %q want %q", got, c.want)
			}
		})
	}
}

// TestDiagnosisAdapterStream_NoDone 没有 done 帧 = 不完整结果（不许把半截正文当成功）。
func TestDiagnosisAdapterStream_NoDone(t *testing.T) {
	server := httptest.NewServer((&fakeDiagnosisServer{t: t, frames: []string{
		`{"type":"delta","text":"## 一、检查方向"}`,
	}}).handler())
	defer server.Close()

	_, _, err := newDiagnosisForTest(server).Stream(context.Background(), ModelSelector{}, msgsSample("x", 0), nil)
	if err == nil || !strings.Contains(err.Error(), "未返回完整结果") {
		t.Fatalf("缺 done 应报不完整，got %v", err)
	}
}

// TestDiagnosisAdapterStream_TrailingFrameWithoutBlankLine 尾帧没有空行收尾也要被消费。
func TestDiagnosisAdapterStream_TrailingFrameWithoutBlankLine(t *testing.T) {
	raw := `data: {"type":"done","answer":"## 一、检查方向\n\n先断电。"}` // 无尾随空行
	server := httptest.NewServer((&fakeDiagnosisServer{t: t, raw: raw}).handler())
	defer server.Close()

	content, _, err := newDiagnosisForTest(server).Stream(context.Background(), ModelSelector{}, msgsSample("x", 0), nil)
	if err != nil {
		t.Fatalf("尾帧应被消费，got %v", err)
	}
	if !strings.Contains(content, "先断电") {
		t.Fatalf("正文不符: %q", content)
	}
}

// TestDiagnosisAdapterSourceID 历史来源 ID 两态（线上根因：吐 "fault-15" 字符串）：
// 数字 7 与字符串 "fault-15" 都能解出，出站恒为字符串。
func TestDiagnosisAdapterSourceID(t *testing.T) {
	for _, body := range []string{
		`{"id":7,"text":"t","metadata":{"source_url":"u","page_start":1,"page_end":1}}`,
		`{"id":"fault-15","text":"t","metadata":{"source_url":"u","page_start":1,"page_end":1}}`,
	} {
		var s DiagnosisSource
		if err := json.Unmarshal([]byte(body), &s); err != nil {
			t.Fatalf("两态 ID 应能解出，body=%s err=%v", body, err)
		}
		if s.ID == "" || s.Metadata.SourceURL != "u" {
			t.Fatalf("解出的来源不符: %+v", s)
		}
	}
}

// TestRoutingAIModel_Dispatch 分发：fault_diagnosis → diagnosis；其余 → normal（fake 记录）。
func TestRoutingAIModel_Dispatch(t *testing.T) {
	normal := &fakeAIModelPort{content: "eino 回复"}
	server := httptest.NewServer((&fakeDiagnosisServer{t: t, frames: []string{
		`{"type":"done","answer":"SOP 内容"}`,
	}}).handler())
	defer server.Close()
	diag := newDiagnosisForTest(server)
	r := NewRoutingModel(normal, diag)

	c, _, err := r.Stream(context.Background(), ModelSelector{FeatureKey: FeatureMaintenanceKnowledge}, msgsSample("x", 0), nil)
	if err != nil || c != "eino 回复" {
		t.Fatalf("non-diagnosis 应走 normal: content=%q err=%v", c, err)
	}
	if normal.streamN == 0 {
		t.Fatal("normal adapter 应被调用")
	}
	c, _, err = r.Stream(context.Background(), ModelSelector{FeatureKey: FeatureFaultDiagnosis}, msgsSample("x", 0), nil)
	if err != nil || c != "SOP 内容" {
		t.Fatalf("diagnosis 应走助手 adapter: content=%q err=%v", c, err)
	}
	if n := normal.streamN; n != 1 {
		t.Fatalf("diagnosis 不应走 normal，got streamN=%d", n)
	}
	if _, err := r.Complete(FeatureFaultDiagnosis, nil, CompleteOptions{}); err == nil {
		t.Fatal("诊断功能 Complete 应报不支持")
	}
	before := normal.completeN
	if _, err := r.Complete(FeatureGradeShortAnswer, nil, CompleteOptions{}); err != nil {
		t.Fatalf("非诊断 Complete 应走 normal: %v", err)
	}
	if normal.completeN <= before {
		t.Fatal("非诊断 Complete 应调用 normal adapter")
	}
}
