// 外部诊断 RAG 助手 adapter（v2 单轨）：
// 第二个生产 ModelPort adapter（ADR-0029 T2，"单一端口、多实现"）——调用外部
// forklift-assistant 的 v2 流式端点 POST /api/v2/chat（SSE，X-API-Key 鉴权）。
//
// v1（lxc101 交付包的阻塞 JSON /chat 与 /chat/with-image 伪流式）已整体退役：旧端点不再部署，
// 本适配器只认 v2（票 2237）。三条不变式：
//   - 凭证不入 ConfigResolver：baseURL / API Key / OSS base 构造期注入，不走管理端模型绑定。
//   - 请求只有 content 一个文本通道 + images（dataURL 数组）：品牌/型号等结构化过滤在 v2
//     不存在，组句责任在 Web 前端（票 2241 的 diagnosisCompose）。
//   - delta 只为观感、done 的 answer 为权威：归一链只跑在 done 的 answer 上，于是
//     「SSE 中继出的最终正文 = 归一后的全文」这条不变式仍成立（票 2245 决策 4）。
package aiassistant

import (
	"bufio"
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strings"
	"sync"

	"github.com/cloudwego/eino/schema"
	"go.uber.org/zap"
)

// ---- ctx 透传通道：来源、过程状态与元数据 ----

// diagnosisSourcesCtxKey ctx 键：诊断响应来源数据容器（可变指针——adapter 与调用方共享）。
// ModelPort.Stream 返回签名固定 (string, *Usage, error)，sources 属助手专用产物，
// 不污染通用端口语义——用 box 完成 adapter（写）→ handler（读）的跨层透传，并发安全。
//
// v2 的 done 事件不再带来源资料（只有 answer/badges/trace/elapsed），本容器现阶段恒空；
// 保留它是因为历史行仍按 ADR-0033 回放来源。
type diagnosisSourcesCtxKey struct{}

type diagnosisSourcesBox struct {
	mu      sync.Mutex
	sources []DiagnosisSource
}

// WithDiagnosisSources 初始化来源容器（handler 在调 port 前注入；service 透传同一 ctx）。
func WithDiagnosisSources(ctx context.Context) context.Context {
	return context.WithValue(ctx, diagnosisSourcesCtxKey{}, &diagnosisSourcesBox{})
}

// DiagnosisSourcesFrom 读取诊断来源（无容器/未写入时返回空切片）。
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

// diagnosisStatusCtxKey ctx 键：上游 status 事件的下沉通道（「正在检索维修知识库…」这类过程文案）。
// v2 在首个 delta 之前会先发 status，正好填住我方 8~15 秒只有加载点的空档。
//
// 用回调而不是「最后一条文案」的容器：status 必须**即刻**转给前端，等 Stream 返回再读，
// 那时 delta 早已开始、文案没有意义了（票 2245 决策 5）。
type diagnosisStatusCtxKey struct{}

// WithDiagnosisStatusSink 注册状态下沉回调（handler 注入；sink 为 nil 即丢弃状态）。
func WithDiagnosisStatusSink(ctx context.Context, sink func(text string)) context.Context {
	if sink == nil {
		return ctx
	}
	return context.WithValue(ctx, diagnosisStatusCtxKey{}, sink)
}

func diagnosisStatusSinkFrom(ctx context.Context) func(string) {
	sink, _ := ctx.Value(diagnosisStatusCtxKey{}).(func(string))
	return sink
}

// DiagnosisBadge 能力徽章（v2 九种固定词表；原样存对方给的三字段，不压成逗号串——
// 对方自己的回放把 emoji/cls 重建成了空串，是反面教材）。
type DiagnosisBadge struct {
	Name  string `json:"name"`
	Emoji string `json:"emoji"`
	Cls   string `json:"cls"`
}

// DiagnosisMeta 回答的附加产物（徽章 + 端到端耗时）。trace 刻意不收：契约自己写着
// 「排查用，可不展示给用户」，且对方存了也从不回放（票 2245 决策 1）。
type DiagnosisMeta struct {
	Badges  []DiagnosisBadge `json:"badges,omitempty"`
	Elapsed float64          `json:"elapsed,omitempty"`
}

// diagnosisMetaCtxKey ctx 键：元数据透传容器（adapter 写 → handler 发 done 载荷）。
type diagnosisMetaCtxKey struct{}

type diagnosisMetaBox struct {
	mu   sync.Mutex
	meta DiagnosisMeta
	ok   bool
}

// WithDiagnosisMeta 初始化元数据容器（handler 注入）。
func WithDiagnosisMeta(ctx context.Context) context.Context {
	return context.WithValue(ctx, diagnosisMetaCtxKey{}, &diagnosisMetaBox{})
}

// DiagnosisMetaFrom 读取元数据（第二返回值为 false 表示本轮没有元数据）。
func DiagnosisMetaFrom(ctx context.Context) (DiagnosisMeta, bool) {
	b, _ := ctx.Value(diagnosisMetaCtxKey{}).(*diagnosisMetaBox)
	if b == nil {
		return DiagnosisMeta{}, false
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.meta, b.ok
}

func (b *diagnosisMetaBox) set(meta DiagnosisMeta) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.meta = meta
	b.ok = true
}

// ---- 助手 API 契约（历史回放仍在用） ----

// DiagnosisSourceMetadata 来源条目的元信息（PDF 原文外链 + 页码区间）。
// 命名后进入 definitions 传递闭包——swag 对内联匿名对象只吐 properties，生成物渲染不出字段。
//
// v2 不再返回来源资料；这两个类型保留是因为历史消息行的 sources 列仍按 ADR-0033 回放。
type DiagnosisSourceMetadata struct {
	SourceURL string `json:"source_url"`
	PageStart int    `json:"page_start"`
	PageEnd   int    `json:"page_end"`
}

// DiagnosisSource 来源资料条目（v1 的 answer_sources）：文本内嵌溯源标记，
// metadata.source_url 为 PDF 原文外链，page_start/end 为页码。
// ID 兼容数字/字符串两态（实测结构化故障码来源吐 "fault-15" 字符串）；出站恒为 JSON 字符串。
type DiagnosisSource struct {
	ID       DiagnosisSourceID       `json:"id"`
	Text     string                  `json:"text"`
	Metadata DiagnosisSourceMetadata `json:"metadata"`
}

// DiagnosisSourceID 来源 ID：数字与字符串两态。
type DiagnosisSourceID string

func (id *DiagnosisSourceID) UnmarshalJSON(raw []byte) error {
	var s string
	if err := json.Unmarshal(raw, &s); err == nil {
		*id = DiagnosisSourceID(s)
		return nil
	}
	var n json.Number
	if err := json.Unmarshal(raw, &n); err != nil {
		return err
	}
	*id = DiagnosisSourceID(n.String())
	return nil
}

// diagnosisChatRequest v2 请求体：content 是唯一文本通道，images 是 dataURL 数组。
type diagnosisChatRequest struct {
	Content string   `json:"content"`
	Images  []string `json:"images,omitempty"`
}

// diagnosisSSEEvent v2 SSE 单帧。注意：type 在 JSON 体内——没有 event: 行，
// 与本站对外的 SSE 帧（event: X + data:）不同形，两者不可混用。
type diagnosisSSEEvent struct {
	Type    string           `json:"type"`
	Text    string           `json:"text"`
	Answer  string           `json:"answer"`
	Badges  []DiagnosisBadge `json:"badges"`
	Elapsed float64          `json:"elapsed"`
	Message string           `json:"message"`
}

// diagnosisAssistantAdapter 诊断 RAG 助手 adapter（第二个生产 ModelPort 实现）。
// 自有 HTTP client：baseURL / apiKey / ossBase 均构造期注入，无凭证切换、无签名缓存。
type diagnosisAssistantAdapter struct {
	baseURL string
	apiKey  string
	ossBase string
	client  *http.Client
	logger  *zap.Logger
}

// NewDiagnosisAssistantModel 构建诊断 adapter。baseURL 或 apiKey 为空时 Stream 返回
// 「未配置」友好错误（功能不可用不炸栈）；ossBase 为空时图片全部降级为文字（ADR-0091 决策 5）。
func NewDiagnosisAssistantModel(baseURL, apiKey, ossBase string, logger *zap.Logger) ModelPort {
	return &diagnosisAssistantAdapter{
		baseURL: strings.TrimSuffix(baseURL, "/"),
		apiKey:  apiKey,
		ossBase: strings.TrimSuffix(ossBase, "/"),
		// 不跟随重定向：白名单与凭证只对声明的上游主机成立，302 到别处必须显式失败
		// （ADR-0091 决策 8）。
		client: &http.Client{
			Timeout: aiStreamTimeout, // 契约 §1.5：SSE 超时 180~300 秒
			CheckRedirect: func(*http.Request, []*http.Request) error {
				return http.ErrUseLastResponse
			},
		},
		logger: logger,
	}
}

var _ ModelPort = (*diagnosisAssistantAdapter)(nil)

// Complete 诊断功能无阻塞补全消费：返回不支持错误（routing adapter 兜底到 eino 前先声明）。
func (a *diagnosisAssistantAdapter) Complete(_ string, _ []*schema.Message, _ CompleteOptions) (string, error) {
	return "", errors.New("智能维修诊断不支持阻塞补全")
}

// Stream 调 v2 SSE：delta 原样经 onChunk 透传（只为观感），done 的 answer 为权威全文并跑
// 归一链，badges/elapsed 写入 ctx 元数据容器，status 写入状态容器。
func (a *diagnosisAssistantAdapter) Stream(ctx context.Context, _ ModelSelector, msgs []*schema.Message, onChunk func(string)) (string, *Usage, error) {
	if a.baseURL == "" || a.apiKey == "" {
		return "", nil, errors.New("智能维修诊断服务未配置，请联系管理员")
	}

	content, images, err := a.buildDiagnosisPayload(msgs)
	if err != nil {
		return "", nil, err
	}

	answer, err := a.callSSE(ctx, content, images, onChunk)
	if err != nil {
		return "", nil, err
	}
	if strings.TrimSpace(answer) == "" {
		return "", nil, errors.New("诊断服务没有返回内容，请重试")
	}

	// 归一必须在返回与落库之前：SSE、历史回放、Web 与移动端读到的都是同一份归一后正文。
	return normalizeDiagnosisImages(a.ossBase, answer, a.logger), nil, nil
}

// callSSE 发起 v2 流式请求并按帧消费；返回 done 的 answer（权威全文）。
func (a *diagnosisAssistantAdapter) callSSE(ctx context.Context, content string, images []string, onChunk func(string)) (string, error) {
	payload, err := json.Marshal(diagnosisChatRequest{Content: content, Images: images})
	if err != nil {
		return "", fmt.Errorf("构造诊断请求失败: %w", err)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, a.baseURL+"/api/v2/chat", bytes.NewReader(payload))
	if err != nil {
		return "", fmt.Errorf("构造诊断请求失败: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "text/event-stream")
	req.Header.Set("X-API-Key", a.apiKey)

	resp, err := a.client.Do(req)
	if err != nil {
		if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
			return "", err
		}
		if a.logger != nil {
			a.logger.Error("诊断助手调用失败", zap.String("path", "/api/v2/chat"), zap.Error(err))
		}
		return "", errors.New("智能维修诊断服务暂时不可用，请稍后重试")
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return "", a.statusError(resp)
	}

	return a.consumeSSE(ctx, resp.Body, onChunk)
}

// consumeSSE 按契约 §1.5 铁律以空行切帧、逐帧读 data: 里的 JSON，按 type 分派。
func (a *diagnosisAssistantAdapter) consumeSSE(ctx context.Context, body io.Reader, onChunk func(string)) (string, error) {
	scanner := bufio.NewScanner(body)
	scanner.Buffer(make([]byte, 0, 64*1024), 4*1024*1024)

	var data strings.Builder
	answer := ""
	done := false

	// dispatch 处理单帧；返回 done=true 表示收到终帧。
	dispatch := func(raw string) (bool, error) {
		var ev diagnosisSSEEvent
		if err := json.Unmarshal([]byte(raw), &ev); err != nil {
			return false, nil // 非 JSON 帧忽略（SSE 允许注释帧）
		}
		switch ev.Type {
		case "status":
			if sink := diagnosisStatusSinkFrom(ctx); sink != nil && ev.Text != "" {
				sink(ev.Text)
			}
		case "delta":
			// 只为观感：全文以 done 的 answer 为准，delta 不做归一。
			if onChunk != nil && ev.Text != "" {
				onChunk(ev.Text)
			}
		case "ping", "user_saved":
			// 忽略：心跳与「用户图片已落盘」对学员端无展示价值
		case "done":
			answer = ev.Answer
			if b, _ := ctx.Value(diagnosisMetaCtxKey{}).(*diagnosisMetaBox); b != nil {
				if len(ev.Badges) > 0 || ev.Elapsed > 0 {
					b.set(DiagnosisMeta{Badges: ev.Badges, Elapsed: ev.Elapsed})
				}
			}
			return true, nil
		case "error":
			msg := strings.TrimSpace(ev.Message)
			if msg == "" {
				msg = "诊断服务返回了错误，请重试"
			}
			return false, errors.New(msg)
		}
		return false, nil
	}

	for scanner.Scan() {
		line := scanner.Text()
		if line != "" {
			if v, ok := strings.CutPrefix(line, "data:"); ok {
				data.WriteString(strings.TrimPrefix(v, " "))
			}
			// 契约里没有 event: 行；即便出现也不消费（type 以 JSON 体内为准）。
			continue
		}
		// 空行 = 一帧结束
		raw := strings.TrimSpace(data.String())
		data.Reset()
		if raw == "" {
			continue
		}
		finished, err := dispatch(raw)
		if err != nil {
			return "", err
		}
		if finished {
			done = true
			break
		}
	}
	if err := scanner.Err(); err != nil {
		if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
			return "", err
		}
		if a.logger != nil {
			a.logger.Error("读取诊断流失败", zap.Error(err))
		}
		return "", errors.New("读取诊断服务响应失败")
	}
	// 尾帧可能没有以空行收尾
	if !done {
		if raw := strings.TrimSpace(data.String()); raw != "" {
			finished, err := dispatch(raw)
			if err != nil {
				return "", err
			}
			done = finished
		}
	}
	if !done {
		return "", errors.New("诊断服务未返回完整结果，请重试")
	}
	return answer, nil
}

// statusError 把 v2 的 HTTP 失败翻成中文人话（不回显对方原始 JSON）。
// 403 的两种（Key 停用 / IP 不在白名单）只有错误体说得清时才分，分不出不猜（票 2237 A.5）。
func (a *diagnosisAssistantAdapter) statusError(resp *http.Response) error {
	raw, _ := io.ReadAll(io.LimitReader(resp.Body, 8192))
	detail := v2ErrorDetail(raw)
	if a.logger != nil {
		a.logger.Warn("诊断助手返回失败状态",
			zap.Int("status", resp.StatusCode), zap.String("detail", detail))
	}

	switch resp.StatusCode {
	case http.StatusUnauthorized:
		return errors.New("智能维修诊断服务鉴权失败（API Key 缺失或无效），请联系管理员")
	case http.StatusForbidden:
		if strings.Contains(detail, "IP") || strings.Contains(detail, "白名单") {
			return errors.New("访问被拒绝：服务器出口 IP 不在对方白名单内，请联系管理员")
		}
		if strings.Contains(detail, "停用") || strings.Contains(detail, "禁用") {
			return errors.New("访问被拒绝：诊断服务 API Key 已停用，请联系管理员")
		}
		return errors.New("访问被拒绝（Key 或 IP 白名单），请联系管理员")
	case http.StatusTooManyRequests:
		return errors.New("诊断服务请求过于频繁，请稍后再试")
	case http.StatusBadRequest:
		if detail != "" {
			return errors.New("诊断失败：" + detail)
		}
		return errors.New("诊断请求不被接受（可能图片过大），请重试")
	case http.StatusRequestEntityTooLarge:
		return errors.New("上传的图片过大，请压缩后重试")
	case http.StatusBadGateway:
		if detail != "" {
			return errors.New("诊断失败：" + detail)
		}
		return errors.New("诊断服务上游异常，请稍后重试")
	default:
		return fmt.Errorf("诊断服务响应异常（HTTP %d），请稍后重试", resp.StatusCode)
	}
}

// v2ErrorDetail 取 v2 错误体里的 detail：400/401/403/429/502 是字符串，
// 422 是数组（契约 §1.4），两种情况都收成一行可读文本。
func v2ErrorDetail(raw []byte) string {
	var env struct {
		Detail json.RawMessage `json:"detail"`
	}
	if err := json.Unmarshal(raw, &env); err != nil || len(env.Detail) == 0 {
		return ""
	}
	var s string
	if err := json.Unmarshal(env.Detail, &s); err == nil {
		return strings.TrimSpace(s)
	}
	var arr []struct {
		Msg string `json:"msg"`
	}
	if err := json.Unmarshal(env.Detail, &arr); err == nil {
		parts := make([]string, 0, len(arr))
		for _, item := range arr {
			if item.Msg != "" {
				parts = append(parts, item.Msg)
			}
		}
		return strings.TrimSpace(strings.Join(parts, "；"))
	}
	return ""
}

// ---- 消息转译 ----

// 对方单次最多收 3 张图、单张解码后上限 5MB（本站上传闸门是 20MB，故必须另判）。
const (
	diagnosisMaxImages   = 3
	diagnosisMaxImageCap = 5 * 1024 * 1024
)

// buildDiagnosisPayload 把端口消息转译为 v2 请求负载：
//   - content = 最后一条用户消息的文本（有历史时把近几轮折叠进来，见 foldHistoryIntoQuery）
//   - images = 最后一条用户消息的 image part 还原成的 dataURL 数组（无图 → nil）
//
// 字节来源不变：service.buildImageUserMessage 已把文件读成 base64 塞进 part，这里只解回来，
// 不二次拉文件。超过上限即拒绝/截断，并在正文尾补一行提示。
func (a *diagnosisAssistantAdapter) buildDiagnosisPayload(msgs []*schema.Message) (content string, images []string, err error) {
	if len(msgs) == 0 {
		return "", nil, errors.New("消息不能为空")
	}
	lastUserIdx := -1
	for i := len(msgs) - 1; i >= 0; i-- {
		if msgs[i] != nil && msgs[i].Role == schema.User {
			lastUserIdx = i
			break
		}
	}
	if lastUserIdx < 0 {
		return "", nil, errors.New("消息不能为空")
	}

	// 历史轮次（跳过 system prompt）
	var history []diagnosisChatTurn
	for i := 0; i < lastUserIdx; i++ {
		m := msgs[i]
		if m == nil || (m.Role != schema.User && m.Role != schema.Assistant) {
			continue
		}
		if c := strings.TrimSpace(m.Content); c != "" {
			history = append(history, diagnosisChatTurn{Role: string(m.Role), Content: c})
		}
	}

	last := msgs[lastUserIdx]
	var text string
	received := 0
	for _, part := range last.UserInputMultiContent {
		switch part.Type {
		case schema.ChatMessagePartTypeText:
			if part.Text != "" {
				text = part.Text
			}
		case schema.ChatMessagePartTypeImageURL:
			if part.Image == nil || part.Image.Base64Data == nil {
				continue
			}
			received++
			if len(images) >= diagnosisMaxImages {
				continue
			}
			b, derr := base64.StdEncoding.DecodeString(*part.Image.Base64Data)
			if derr != nil || len(b) == 0 {
				if a.logger != nil {
					a.logger.Warn("诊断图片 base64 解码失败，跳过图片", zap.Error(derr))
				}
				continue
			}
			if len(b) > diagnosisMaxImageCap {
				return "", nil, fmt.Errorf("图片单张不能超过 %dMB，请压缩后重试", diagnosisMaxImageCap/1024/1024)
			}
			mime := part.Image.MIMEType
			if mime == "" {
				mime = "image/jpeg"
			}
			images = append(images, "data:"+mime+";base64,"+base64.StdEncoding.EncodeToString(b))
		}
	}
	if text == "" {
		text = last.Content
	}
	if text == "" && len(images) > 0 {
		// 纯图片轮次：对齐包前端行为，默认走图片分析话术
		text = "请根据现场图片进行分析"
	}
	// v2 为单轮 RAG QA（契约 FAQ Q5：有意不支持多轮）：不折叠就真的没有上文。
	if len(history) > 0 && strings.TrimSpace(text) != "" {
		text = foldHistoryIntoQuery(history, text)
	}
	if received > len(images) {
		text += fmt.Sprintf("\n\n> 本次随问附带了 %d 张图片，仅前 %d 张送达诊断服务。", received, len(images))
	}
	return text, images, nil
}

// ---- 上下文折叠 ----

// 折叠预算：轮次上限（近 6 条消息）、单轮截断、总字符预算——控制 token 与检索污染。
const (
	foldMaxTurns      = 6
	foldMaxTurnRunes  = 300
	foldMaxTotalRunes = 1200
)

// diagnosisChatTurn 会话轮次（role: user|assistant）——v2 请求体里没有 chat_history 字段，
// 这里只作为 foldHistoryIntoQuery 的入参形状。
type diagnosisChatTurn struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// foldHistoryIntoQuery 把近几轮历史折进 query 文本：
//
//	[对话上下文]
//	用户：…
//	助手：…
//	[本轮问题]（请结合上文理解本轮追问；勿重复回答历史问题）
//	<query>
//
// 检索按整段 query 做向量匹配，历史中的故障码/车型关键词能拉回同一主题的文档；
// 生成模型也能从折叠文本里读出追问指代。预算封顶防长会话膨胀。
func foldHistoryIntoQuery(history []diagnosisChatTurn, query string) string {
	if len(history) == 0 {
		return query
	}
	start := 0
	if len(history) > foldMaxTurns {
		start = len(history) - foldMaxTurns
	}
	var b strings.Builder
	b.WriteString("[对话上下文]\n")
	total := 0
	for _, t := range history[start:] {
		role := "助手"
		if t.Role == "user" {
			role = "用户"
		}
		c := []rune(t.Content)
		if len(c) > foldMaxTurnRunes {
			c = c[:foldMaxTurnRunes]
		}
		if total+len(c) > foldMaxTotalRunes {
			break
		}
		total += len(c)
		b.WriteString(role)
		b.WriteString("：")
		b.WriteString(string(c))
		b.WriteString("\n")
	}
	b.WriteString("[本轮问题]（请结合上文理解本轮追问；勿重复回答历史问题）\n")
	b.WriteString(query)
	return b.String()
}

// ---- 图片表达归一（ADR-0063 + ADR-0091）----

// diagnosisImageRoute 归一后正文里图片的本站路径前缀，与 handler_diagnosis.go 注册的
// /api/ai-assistant/diagnosis/ossimg/*filepath 同源——改路由要同时改这里，
// 契约测试经真实路由命中该路径，会先红。
const diagnosisImageRoute = "/api/ai-assistant/diagnosis/ossimg/"

var (
	// 正文里的 markdown 图片一律过白名单判定。
	diagnosisMarkdownImgRe = regexp.MustCompile(`!\[([^\]]*)\]\(([^)\s]+)\)`)
	// diagnosisAltSanitizer 剥掉会破坏 markdown 图片语法的字符。
	diagnosisAltSanitizer = regexp.MustCompile("[\r\n\\[\\]`]")
)

// diagnosisFallbackAlt 图片降级成文字时使用的固定文案（与 ADR-0063 时期同词）。
const diagnosisFallbackAlt = "诊断配图"

// normalizeDiagnosisImages 归一正文里的图片：白名单内的第三方图改写成本站代理路径，
// 白名单外的降级成一行 alt 文字（ADR-0091 决策 5）。
//
// 入口是 done 的 answer（delta 只为观感，不归一）：v2 的图片以 OSS 公网链接出现在 markdown
// 里。白名单内→代理，于是学员浏览器不会直连第三方；白名单外→留字，宁少一张图也不让
// AI 生成的正文驱动学员浏览器去请求任意主机（ADR-0063 决策 3 的原始动机）。
func normalizeDiagnosisImages(ossBase, sop string, logger *zap.Logger) string {
	if sop == "" {
		return sop
	}
	return diagnosisMarkdownImgRe.ReplaceAllStringFunc(sop, func(m string) string {
		g := diagnosisMarkdownImgRe.FindStringSubmatch(m)
		if len(g) < 3 {
			return ""
		}
		// 已归一的本站代理路径原样放行（防同一正文被二次改写）。
		if strings.HasPrefix(g[2], diagnosisImageRoute) {
			return m
		}
		if path, ok := diagnosisOSSProxyPath(ossBase, g[2]); ok {
			return diagnosisMarkdownImage(g[1], path)
		}
		// 白名单外：同步降级为 alt 文字（决策 7 的一行日志，不打点、不进计量面）
		if logger != nil {
			logger.Info("诊断配图不在白名单内，降级为文字",
				zap.String("host", diagnosisHostOf(g[2])),
				zap.Bool("whitelisted", false),
				zap.String("url_head", truncateRunes(g[2], 120)))
		}
		return diagnosisAltText(g[1])
	})
}

// diagnosisMarkdownImage 拼一张本站代理图；alt 清洗后为空时给固定文案。
func diagnosisMarkdownImage(alt, path string) string {
	return "![" + diagnosisAltText(alt) + "](" + path + ")"
}

// diagnosisAltText 把 alt 清洗成可安全嵌进 markdown 的一行文字。
func diagnosisAltText(alt string) string {
	clean := strings.TrimSpace(diagnosisAltSanitizer.ReplaceAllString(alt, ""))
	if clean == "" {
		return diagnosisFallbackAlt
	}
	return clean
}

// truncateRunes 截断日志字段（按 rune，不留半个字）。
func truncateRunes(s string, max int) string {
	r := []rune(s)
	if len(r) <= max {
		return s
	}
	return string(r[:max])
}

// ---- routing adapter：按 FeatureKey 分发 ----

// routingAIModel 按功能键分发到两个生产 adapter（ADR-0029 T2 第二实现接入点）：
//
//	普通 AI（eino 解析管理端/双绑定凭证）   ——完整/流式一切既往
//	智能维修诊断（外部 RAG 助手，自持凭证）——仅流式消费
//
// 消耗面（Service.StreamChat 等）无感知：仍面对单一 ModelPort。
type routingAIModel struct {
	normal    ModelPort
	diagnosis ModelPort
}

// NewRoutingModel 构建分发端口。normal/diagnosis 均必须非 nil（构造期注入是不变量）。
func NewRoutingModel(normal, diagnosis ModelPort) ModelPort {
	return &routingAIModel{normal: normal, diagnosis: diagnosis}
}

var _ ModelPort = (*routingAIModel)(nil)

// Complete 走外部诊断适配器的功能不支持阻塞补全；其余走 normal（评分/解析/章节生成等）。
// 分发判据来自注册表 adapter 列（ADR-0047 §7），不再是功能键字符串比较。
func (r *routingAIModel) Complete(featureKey string, msgs []*schema.Message, opts CompleteOptions) (string, error) {
	if aiFeatureAdapterOf(featureKey) == aiAdapterDiagnosis {
		return r.diagnosis.Complete(featureKey, msgs, opts)
	}
	return r.normal.Complete(featureKey, msgs, opts)
}

// Stream 按注册表 adapter 分发：diagnosis 走外部诊断 RAG，其余走 eino（含未注册键回退路径）。
func (r *routingAIModel) Stream(ctx context.Context, sel ModelSelector, msgs []*schema.Message, onChunk func(string)) (string, *Usage, error) {
	if aiFeatureAdapterOf(sel.FeatureKey) == aiAdapterDiagnosis {
		return r.diagnosis.Stream(ctx, sel, msgs, onChunk)
	}
	return r.normal.Stream(ctx, sel, msgs, onChunk)
}
