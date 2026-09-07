// Package service 外部诊断 RAG 助手 adapter（计划 批次1）：
// 第二个生产 AIModelPort adapter（ADR-0029 T2，"单一端口、多实现"）——调用 forklift-assistant
// 交付包（FastAPI + bge-m3 本地向量 RAG）的阻塞 JSON /chat 与 /chat/with-image，经 onChunk
// 伪流式转译为 AIModelPort.Stream 回调（助手不支持 SSE，2026-09-05 契约钉死）。
// 特性：
//   - 凭证不入 AIConfigResolver：baseURL 来自 config.DiagnosisAssistantURL（直接注入），
//     不走管理端模型绑定（端口签名只传 selector，解析全在 adapter 内部）。
//   - 图片路径：复用 StreamChat 的多模态消息重组（text + base64 image part），adapter 从
//     part 还原字节后 re-post multipart 到 /chat/with-image（无需新增转发端点）。
package service

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"

	"github.com/cloudwego/eino/schema"
	"go.uber.org/zap"
)

// ---- ctx 透传通道：诊断参数（品牌/车型）与来源数据 ----

// diagnosisParamsCtxKey ctx 键：诊断请求参数（品牌/车型），随调用声明。
// 仿 withAIPromptChars 透传机制：StreamChatReq 的扩展字段经 ctx 到达 diagnosis adapter，
// selector/端口签名零改动（eino 路径不感知）。
type diagnosisParamsCtxKey struct{}

type diagnosisParams struct {
	brand string
	model string
}

// WithDiagnosisParams 随调用声明诊断参数（仅 fault_diagnosis 消费；空值按助手默认 all/无）。
func WithDiagnosisParams(ctx context.Context, brand, model string) context.Context {
	return context.WithValue(ctx, diagnosisParamsCtxKey{}, diagnosisParams{brand: brand, model: model})
}

func diagnosisParamsFrom(ctx context.Context) diagnosisParams {
	p, _ := ctx.Value(diagnosisParamsCtxKey{}).(diagnosisParams)
	return p
}

// diagnosisSourcesCtxKey ctx 键：诊断响应来源数据容器（可变指针——adapter 与调用方共享）。
// ctx value 存指针容器是既有灰色地带的显式使用：AIModelPort.Stream 返回签名固定
// (string, *AIUsage, error)，sources 属助手专用产物，不污染通用端口语义——用 box 完成
// adapter（写）→ handler（读）的跨层透传，并发安全。
type diagnosisSourcesCtxKey struct{}

type diagnosisSourcesBox struct {
	mu      sync.Mutex
	sources []DiagnosisSource
}

// WithDiagnosisSources 初始化来源容器（handler 在调 port 前注入；service 透传同一 ctx）。
func WithDiagnosisSources(ctx context.Context) context.Context {
	return context.WithValue(ctx, diagnosisSourcesCtxKey{}, &diagnosisSourcesBox{})
}

// DiagnosisSourcesFrom 读取诊断来源（无容器/未写入时返回空切片；诊断功能以外的调用恒为空）。
func DiagnosisSourcesFrom(ctx context.Context) []DiagnosisSource {
	b, _ := ctx.Value(diagnosisSourcesCtxKey{}).(*diagnosisSourcesBox)
	if b == nil {
		return nil
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	return append([]DiagnosisSource(nil), b.sources...)
}

func (b *diagnosisSourcesBox) set(s []DiagnosisSource) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.sources = append([]DiagnosisSource(nil), s...)
}

// ---- 助手 API 契约（2026-09-05 openapi 钉死）----

// DiagnosisSource 来源资料条目（answer_sources）：文本内嵌 <<IMAGE:/assistant/static/manual/...>>
// 溯源标记；metadata.source_url 为 PDF 原文外链，page_start/end 为页码。
type DiagnosisSource struct {
	ID       int    `json:"id"`
	Text     string `json:"text"`
	Metadata struct {
		SourceURL string `json:"source_url"`
		PageStart int    `json:"page_start"`
		PageEnd   int    `json:"page_end"`
	} `json:"metadata"`
}

// diagnosisChatRequest 助手 /chat 请求体（chat_history 为 array）。
type diagnosisChatRequest struct {
	Query       string              `json:"query"`
	Brand       string              `json:"brand"`
	Model       *string             `json:"model,omitempty"`
	ChatHistory []diagnosisChatTurn `json:"chat_history,omitempty"`
}

// diagnosisChatTurn 助手会话轮次（role: user|assistant）。
type diagnosisChatTurn struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// diagnosisChatResponse 助手响应（data 仅取消费字段：sop_text + answer_sources）。
// Code 容忍数字/字符串两态（外部服务实现漂移时 "200" 字符串仍放行）。
type diagnosisChatResponse struct {
	Code diagnosisCode `json:"code"`
	Data struct {
		SOPText       string            `json:"sop_text"`
		AnswerSources []DiagnosisSource `json:"answer_sources"`
	} `json:"data"`
}

// diagnosisCode 业务码：数字 0/200 与字符串 "0"/"200" 同视为成功。
type diagnosisCode int

func (c *diagnosisCode) UnmarshalJSON(raw []byte) error {
	var n int
	if err := json.Unmarshal(raw, &n); err == nil {
		*c = diagnosisCode(n)
		return nil
	}
	var s string
	if err := json.Unmarshal(raw, &s); err != nil {
		return err
	}
	trimmed := strings.TrimSpace(s)
	if trimmed == "" {
		*c = 0
		return nil
	}
	parsed, err := strconv.Atoi(trimmed)
	if err != nil {
		return err
	}
	*c = diagnosisCode(parsed)
	return nil
}

// diagnosisAssistantAdapter 诊断 RAG 助手 adapter（第二个生产 AIModelPort 实现）。
// 自有 HTTP client：baseURL 构造期注入（config），不经 resolver、无 client 签名缓存
// （助手仅一个端点，无凭证切换）。
type diagnosisAssistantAdapter struct {
	baseURL string
	client  *http.Client
	logger  *zap.Logger
}

// NewDiagnosisAssistantModel 构建诊断 adapter。baseURL 为空时 Stream 返回「未配置」友好错误
// （功能不可用不炸栈）。
func NewDiagnosisAssistantModel(baseURL string, logger *zap.Logger) AIModelPort {
	return &diagnosisAssistantAdapter{
		baseURL: strings.TrimSuffix(baseURL, "/"),
		client:  &http.Client{Timeout: aiStreamTimeout}, // 与 eino 流式 300s 纪律一致
		logger:  logger,
	}
}

var _ AIModelPort = (*diagnosisAssistantAdapter)(nil)

// Complete 诊断功能无阻塞补全消费：返回不支持错误（routing adapter 兜底到 eino 前先声明）。
func (a *diagnosisAssistantAdapter) Complete(_ string, _ []*schema.Message, _ AICompleteOptions) (string, error) {
	return "", errors.New("智能维修诊断不支持阻塞补全")
}

// Stream 阻塞调用助手 → 伪流式：整包响应到达后按段落切块经 onChunk 回调，
// 返回完整回复与计量产出（nil——计量闸门在本 adapter 外层装饰器）。响应 answer_sources
// 写入 ctx 来源容器（handler 发 SSE sources 事件）。
func (a *diagnosisAssistantAdapter) Stream(ctx context.Context, sel AIModelSelector, msgs []*schema.Message, onChunk func(string)) (string, *AIUsage, error) {
	if a.baseURL == "" {
		return "", nil, errors.New("智能维修诊断服务未配置，请联系管理员")
	}

	// 最后一条用户消息 = 本轮 query（含文本/图片）；此前全部消息 = chat_history
	query, history, imageBinary, imageName, err := a.buildDiagnosisPayload(msgs)
	if err != nil {
		return "", nil, err
	}

	resp := &diagnosisChatResponse{}
	if imageBinary != nil {
		err = a.callWithImage(ctx, query, history, imageBinary, imageName, resp)
	} else {
		err = a.callText(ctx, query, history, resp)
	}
	if err != nil {
		return "", nil, err
	}

	// 请求标识透传（计费闸门消费）；来源写入共享容器
	if b, _ := ctx.Value(diagnosisSourcesCtxKey{}).(*diagnosisSourcesBox); b != nil {
		b.set(resp.Data.AnswerSources)
	}

	// 伪流式切块：按空行段落切（正文 markdown 分段结构），段落间补分隔符——
	// 除末段外每段尾附 "\n\n"，各块拼接恒等于全文。空产出时至少落一个空块。
	content := resp.Data.SOPText
	if strings.TrimSpace(content) == "" {
		if onChunk != nil {
			onChunk("")
		}
		return "", nil, nil
	}
	paras := strings.Split(content, "\n\n")
	for i, para := range paras {
		para = strings.Trim(para, "\n")
		if para == "" {
			continue
		}
		if onChunk != nil {
			chunk := para
			if i < len(paras)-1 {
				chunk += "\n\n"
			}
			onChunk(chunk)
		}
	}
	return content, nil, nil
}

// ---- 助手调用 ----

func (a *diagnosisAssistantAdapter) callText(ctx context.Context, query string, history []diagnosisChatTurn, out *diagnosisChatResponse) error {
	body := diagnosisChatRequest{Query: query, Brand: "all", ChatHistory: history}
	if p := diagnosisParamsFrom(ctx); p.brand != "" {
		body.Brand = p.brand
	}
	if p := diagnosisParamsFrom(ctx); p.model != "" {
		body.Model = &p.model
	}
	return a.doCall(ctx, "/assistant/api/chat", func(req *http.Request) error {
		req.Header.Set("Content-Type", "application/json")
		raw, err := json.Marshal(body)
		if err != nil {
			return err
		}
		req.Body = io.NopCloser(bytes.NewReader(raw))
		return nil
	}, out)
}

func (a *diagnosisAssistantAdapter) callWithImage(ctx context.Context, query string, history []diagnosisChatTurn, imageBinary []byte, imageName string, out *diagnosisChatResponse) error {
	return a.doCall(ctx, "/assistant/api/chat/with-image", func(req *http.Request) error {
		var buf bytes.Buffer
		w := multipart.NewWriter(&buf)
		_ = w.WriteField("query", query)
		brand, model := "all", ""
		if p := diagnosisParamsFrom(ctx); p.brand != "" {
			brand = p.brand
		}
		if p := diagnosisParamsFrom(ctx); p.model != "" {
			model = p.model
		}
		_ = w.WriteField("brand", brand)
		if model != "" {
			_ = w.WriteField("model", model)
		}
		// 注意契约坑：with-image 的 chat_history 是 JSON 字符串（与 /chat 的 array 不一致）
		if hb, err := json.Marshal(history); err == nil {
			_ = w.WriteField("chat_history", string(hb))
		}
		if imageName == "" {
			imageName = "scene.jpg"
		}
		part, err := w.CreateFormFile("image", imageName)
		if err != nil {
			return err
		}
		if _, err := part.Write(imageBinary); err != nil {
			return err
		}
		_ = w.Close()
		req.Header.Set("Content-Type", w.FormDataContentType())
		req.Body = io.NopCloser(&buf)
		return nil
	}, out)
}

// doCall 统一执行助手请求：构造 → 发送（ctx 透传超时/取消）→ 非 200 提取 detail/message →
// 解析 {code,data}。HTTP 层错误映射为友好中文文案（handler 直接进 SSE error 事件）。
func (a *diagnosisAssistantAdapter) doCall(ctx context.Context, path string, build func(*http.Request) error, out *diagnosisChatResponse) error {
	u, err := url.Parse(a.baseURL + path)
	if err != nil {
		return fmt.Errorf("智能维修诊断服务地址无效: %w", err)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, u.String(), nil)
	if err != nil {
		return fmt.Errorf("构造诊断请求失败: %w", err)
	}
	if err := build(req); err != nil {
		return fmt.Errorf("构造诊断请求失败: %w", err)
	}

	resp, err := a.client.Do(req)
	if err != nil {
		if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
			return err
		}
		a.logger.Error("诊断助手调用失败", zap.String("path", path), zap.Error(err))
		return errors.New("智能维修诊断服务暂时不可用，请稍后重试")
	}
	defer resp.Body.Close()

	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return errors.New("读取诊断服务响应失败")
	}
	if resp.StatusCode != http.StatusOK {
		// 兼容助手两种错误信封：{detail|message|error} 直接透出；否则截断原文
		var env struct {
			Detail  string `json:"detail"`
			Message string `json:"message"`
			Error   string `json:"error"`
		}
		if err := json.Unmarshal(raw, &env); err == nil {
			for _, m := range []string{env.Detail, env.Message, env.Error} {
				if m != "" {
					return errors.New("诊断失败：" + m)
				}
			}
		}
		return fmt.Errorf("诊断服务响应异常（HTTP %d）", resp.StatusCode)
	}
	if err := json.Unmarshal(raw, out); err != nil {
		// 坏包分类 + 原始包截断日志（定位网关页/空包/截断包/契约漂移）。
		// 截断包走宽容抢救：能抠出 sop_text 即降级可用（标记截断），抠不出才判死刑。
		preview := truncateDiagnosisPreview(raw)
		trimmed := bytes.TrimSpace(raw)
		a.logger.Error("解析诊断响应失败",
			zap.String("path", path),
			zap.Int("bytes", len(raw)),
			zap.String("preview", preview),
			zap.Error(err))
		switch {
		case len(trimmed) == 0:
			return errors.New("诊断服务返回了空响应，请重试")
		case bytes.HasPrefix(trimmed, []byte("<")):
			return errors.New("诊断服务网关异常（收到非 JSON 响应），请稍后重试")
		default:
			if salvaged, ok := salvageDiagnosisSOP(raw); ok {
				a.logger.Warn("诊断响应截断，已抢救正文可用部分",
					zap.String("path", path), zap.Int("bytes", len(raw)))
				out.Code = 0
				out.Data.SOPText = salvaged + "\n\n> 诊断报告传输不完整，以上为已收到的部分内容，可重试获取完整报告。"
				out.Data.AnswerSources = nil
				break
			}
			return errors.New("诊断服务响应格式异常，请重试")
		}
	}
	if out.Code != 0 && out.Code != 200 {
		a.logger.Warn("诊断业务码异常", zap.String("path", path), zap.Int("code", int(out.Code)))
		return fmt.Errorf("诊断失败（code %d），请重试", out.Code)
	}
	return nil
}

// salvageDiagnosisSOP 截断包宽容抢救：从未闭合的 JSON 原文中抠出 "sop_text" 的
// 已到达部分（要求 ≥20 个 rune 才算可用，避免半句误导）。answer_sources 等结构化
// 字段不抢救（截断即不可信，置空）。仅处理「解析失败但正文可见」一类坏包；空包、
// 网关 HTML 页、code 类型漂移（已由 diagnosisCode 容忍）不在此处理。
func salvageDiagnosisSOP(raw []byte) (string, bool) {
	const key = `"sop_text"`
	idx := bytes.LastIndex(raw, []byte(key))
	if idx < 0 {
		return "", false
	}
	rest := bytes.TrimSpace(raw[idx+len(key):])
	if !bytes.HasPrefix(rest, []byte(":")) {
		return "", false
	}
	rest = bytes.TrimSpace(rest[1:])
	if !bytes.HasPrefix(rest, []byte(`"`)) {
		return "", false
	}
	// 截断位置即字符串结尾：取最后一个完整转义边界之前的内容做 JSON 反转义。
	body := rest[1:]
	end := len(body)
	if i := bytes.LastIndexByte(body, '\\'); i >= 0 && i == len(body)-1 {
		end = i
	}
	var decoded string
	if err := json.Unmarshal([]byte(`"`+string(body[:end])+`"`), &decoded); err != nil {
		return "", false
	}
	decoded = strings.TrimSpace(decoded)
	if len([]rune(decoded)) < 20 {
		return "", false
	}
	return decoded, true
}

// truncateDiagnosisPreview 坏包日志预览（截断防爆日志；二进制/长包只留头部）。
func truncateDiagnosisPreview(raw []byte) string {
	const maxPreview = 512
	preview := raw
	if len(preview) > maxPreview {
		preview = preview[:maxPreview]
	}
	return strings.Map(func(r rune) rune {
		if r == '\n' || r == '\r' || r == '\t' {
			return ' '
		}
		return r
	}, strings.ToValidUTF8(string(preview), "�"))
}

// ---- 消息转译 ----

// buildDiagnosisPayload 把端口消息转译为助手调用负载：
//   - chat_history = 除最后一条用户消息外的全部 user/assistant 轮次（system 前缀忽略）
//   - query = 最后一条用户消息的文本 part（空时对齐包前端默认话术；纯图片轮次也如此）
//   - imageBinary/imageName = 最后一条用户消息的 image part 还原字节（无图 → nil）
func (a *diagnosisAssistantAdapter) buildDiagnosisPayload(msgs []*schema.Message) (query string, history []diagnosisChatTurn, imageBinary []byte, imageName string, err error) {
	if len(msgs) == 0 {
		return "", nil, nil, "", errors.New("消息不能为空")
	}
	// 定位最后一条用户消息：其后不应再跟有效内容（StreamChat 组装保证）
	lastUserIdx := -1
	for i := len(msgs) - 1; i >= 0; i-- {
		if msgs[i] != nil && msgs[i].Role == schema.User {
			lastUserIdx = i
			break
		}
	}
	if lastUserIdx < 0 {
		return "", nil, nil, "", errors.New("消息不能为空")
	}

	// chat_history：最后一条用户消息之前的轮次（跳过 system prompt）
	for i := 0; i < lastUserIdx; i++ {
		m := msgs[i]
		if m == nil || m.Role != schema.User && m.Role != schema.Assistant {
			continue
		}
		if c := strings.TrimSpace(m.Content); c != "" {
			history = append(history, diagnosisChatTurn{Role: string(m.Role), Content: c})
		}
	}

	last := msgs[lastUserIdx]
	for _, part := range last.UserInputMultiContent {
		switch part.Type {
		case schema.ChatMessagePartTypeText:
			query = part.Text
		case schema.ChatMessagePartTypeImageURL:
			if part.Image != nil && part.Image.Base64Data != nil {
				b, derr := base64.StdEncoding.DecodeString(*part.Image.Base64Data)
				if derr != nil || len(b) == 0 {
					a.logger.Warn("诊断图片 base64 解码失败，跳过图片", zap.Error(derr))
					continue
				}
				imageBinary = b
				switch part.Image.MIMEType {
				case "image/png":
					imageName = "scene.png"
				case "image/webp":
					imageName = "scene.webp"
				default:
					imageName = "scene.jpg"
				}
			}
		}
	}
	// eino schema.UserMessage 等纯文本消息走 Content 字段（无 multi-content part）
	if query == "" {
		query = last.Content
	}
	if query == "" && imageBinary != nil {
		// 纯图片轮次：对齐包前端行为，默认走图片分析话术
		query = "请根据现场图片进行分析"
	}
	return query, history, imageBinary, imageName, nil
}

// ---- routing adapter：按 FeatureKey 分发 ----

// routingAIModel 按功能键分发到两个生产 adapter（ADR-0029 T2 第二实现接入点）：
//
//	普通 AI（eino 解析管理端/双绑定凭证）   ——完整/流式一切既往
//	智能维修诊断（外部 RAG 助手，自持凭证）——仅流式消费
//
// 消耗面（AIAssistantService.StreamChat 等）无感知：仍面对单一 AIModelPort。
type routingAIModel struct {
	normal    AIModelPort
	diagnosis AIModelPort
}

// NewRoutingAIModel 构建分发端口。normal/diagnosis 均必须非 nil（构造期注入是不变量）。
func NewRoutingAIModel(normal, diagnosis AIModelPort) AIModelPort {
	return &routingAIModel{normal: normal, diagnosis: diagnosis}
}

var _ AIModelPort = (*routingAIModel)(nil)

// Complete 诊断功能不支持阻塞补全；其余走 normal（评分/解析/章节生成等）。
func (r *routingAIModel) Complete(featureKey string, msgs []*schema.Message, opts AICompleteOptions) (string, error) {
	if featureKey == FeatureFaultDiagnosis {
		return r.diagnosis.Complete(featureKey, msgs, opts)
	}
	return r.normal.Complete(featureKey, msgs, opts)
}

// Stream 诊断功能分发到助手 adapter，其余走 eino（含未注册键回退路径）。
func (r *routingAIModel) Stream(ctx context.Context, sel AIModelSelector, msgs []*schema.Message, onChunk func(string)) (string, *AIUsage, error) {
	if sel.FeatureKey == FeatureFaultDiagnosis {
		return r.diagnosis.Stream(ctx, sel, msgs, onChunk)
	}
	return r.normal.Stream(ctx, sel, msgs, onChunk)
}
