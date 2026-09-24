// Package api 实现 HTTP handlers。
// 本文件：Endpoint 骨架（ADR-0013 §9）——在 struct 方法之上再收一层
// Endpoint(parse, invoke, render) 抽象，吸收「参数解析 - BindJSON - 响应信封 - 超时」公共守卫链。
//
// struct 方法仍保留为 handler 声明形式（ADR-0009），方法体改为配置端点的三个阶段后调用 Handle：
//
//	func (h *XxxHandler) List(c *gin.Context) {
//		Endpoint[ListReq, ListResp]{
//			Parse:  parseListReq,
//			Invoke: h.svc.List,
//			Render: renderList,
//		}.Handle(c)
//	}
//
// 不重新引入闭包注册：路由装配形态（Register*Routes + RouterDeps）不变。
// 「id>0 守卫」等 query 解析单点仍收敛于 helpers.go（atoiDefault/queryIntPtr/queryIDPtr），本骨架复用。
//
// 三面分工（票1b，ADR-0060 §1）：Parse 管参数、Invoke 管业务、**错误面归骨架**（查 ErrStatus 域表），
// Render 只写成功面。
package api

import (
	"context"
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"forklift-training/pkg/response"
)

// endpointTimeout 统一超时（保留既有各 handler 的 10s 语义）。
const endpointTimeout = 10 * time.Second

// ParseError 请求解析失败的哨兵错误：携带 HTTP 状态码与用户可见文案。
type ParseError struct {
	Status  int
	Message string
}

func (e *ParseError) Error() string { return e.Message }

// badRequest 构造 400 解析错误（endpoint 骨架内的参数错误统一出口）。
func badRequest(msg string) *ParseError {
	return &ParseError{Status: http.StatusBadRequest, Message: msg}
}

// ParseFunc 解析请求为 typed Req。返回 *ParseError 表示参数错误（渲染对应 4xx 信封）；
// 返回其他 error 视为服务器内部错误（渲染 500 信封）。
type ParseFunc[Req any] func(c *gin.Context) (*Req, error)

// InvokeFunc 调用 service：Req → Resp。错误交给骨架按 ErrStatus 域表渲染（票1b，ADR-0060 §1）。
type InvokeFunc[Req, Resp any] func(ctx context.Context, req *Req) (*Resp, error)

// RenderFunc 把成功的 Resp 渲染为响应。**签名上没有 err**（ADR-0060 §1 票1b）：
// 错误面归骨架无条件渲染，Render 只在 Invoke 成功时被调用一次。
// 「记得自己查域表」从注释约束升级为类型约束——闭包里既拿不到 err，也就写不出漏查表的分支。
type RenderFunc[Req, Resp any] func(c *gin.Context, req *Req, resp *Resp)

// Endpoint 泛型端点骨架：parse → invoke → render 三段式守卫链。
// Req 为 typed 请求（query/路径/body 字段），Resp 为 service 返回的 typed DTO。
type Endpoint[Req, Resp any] struct {
	// Parse 解析请求。为 nil 时使用零值 Req（无请求参数的端点）。
	Parse ParseFunc[Req]
	// Invoke 调用 service。为 nil 时跳过调用（Resp 保持 nil）。
	Invoke InvokeFunc[Req, Resp]
	// Render 渲染**成功面**。省略时走内置默认信封（ADR-0024 C2）：成功 → 200 统一信封。
	// 自定义 Render 不参与错误渲染（见 RenderFunc）。
	Render RenderFunc[Req, Resp]
	// ErrStatus 域级「哨兵 → 状态码（+ 可选固定文案）」表：本端点的错误面**唯一**由此字段渲染——
	// errors.Is 命中 → 表内状态码；未命中 → 表 fallback（未设 → 500）。
	// 表内任何条目都命中不了 *ParseError：解析错误恒优先（票8 / ADR-0062 决策 8）。
	// 省略即纯默认：*ParseError → 其状态码、其余 500。
	ErrStatus *errStatusTable
}

// Handle 执行端点全链条：10s 超时 → parse → invoke → render，并兜底 panic（保证 500 信封）。
func (e Endpoint[Req, Resp]) Handle(c *gin.Context) {
	ctx, cancel := context.WithTimeout(c.Request.Context(), endpointTimeout)
	defer cancel()

	// panic 兜底：endpoint 自身保证 500 信封，不依赖 gin 全局 Recovery。
	defer func() {
		if r := recover(); r != nil {
			response.ServerError(c, "服务器内部错误")
		}
	}()

	req, err := e.parse(c)
	if err != nil {
		e.render(c, req, nil, err)
		return
	}

	var resp *Resp
	if e.Invoke != nil {
		resp, err = e.Invoke(ctx, req)
	}
	e.render(c, req, resp, err)
}

func (e Endpoint[Req, Resp]) parse(c *gin.Context) (*Req, error) {
	if e.Parse == nil {
		var zero Req
		return &zero, nil
	}
	return e.Parse(c)
}

func (e Endpoint[Req, Resp]) render(c *gin.Context, req *Req, resp *Resp, err error) {
	if err != nil {
		// 错误面无条件归骨架（ADR-0060 §1 票1b）：查 ErrStatus 域表，Render 不参与。
		// 未挂表时即纯默认信封（*ParseError → 其状态码、其余 500）。
		e.ErrStatus.renderError(c, err)
		return
	}
	if e.Render == nil {
		response.Success(c, deref(resp))
		return
	}
	e.Render(c, req, resp)
}

// deref 解引用指针；nil 返回 nil（保持 JSON "data": null 语义）。
func deref[T any](p *T) any {
	if p == nil {
		return nil
	}
	return *p
}

// renderStatus 按状态码输出信封（收敛到 response 单点）。
// 注意：本函数是所有域表状态码的单一咽喉——新增状态码（如 409）必须
// 在此补 case，否则 default 会把该语义静默渲染成 500。
func renderStatus(c *gin.Context, status int, msg string) {
	switch status {
	case http.StatusBadRequest:
		response.BadRequest(c, msg)
	case http.StatusUnauthorized:
		response.Unauthorized(c, msg)
	case http.StatusForbidden:
		response.Forbidden(c, msg)
	case http.StatusNotFound:
		response.NotFound(c, msg)
	case http.StatusCreated:
		response.Created(c, msg, nil)
	default:
		response.ServerError(c, msg)
	}
}

// ===== 域级「哨兵 → 状态码」表（#610/#611；票1b 起为错误面唯一出口） =====
//
// 每域一张表（pointsErrStatus / contributionErrStatus 等，写在各域文件内）：
// HTTP 语义归 api 侧（ADR-0024「handler 以 errors.Is 映射状态码」的投影位置），同域端点共用；
// 不做全仓中央表——同一哨兵跨域可归属不同状态码（如 ErrJobNotFound 同时进 job / application /
// recruiterApplication 三张表）。
//
// 票1b（ADR-0060 §1）后不再有「Render 闭包自查域表」这回事：错误渲染整体归骨架，
// 手写例外清单（旧版本段列的 forum.go GetTopic / job_card.go 两处固定文案）已收编为
// 带 message 的表条目。需要非信封错误形状的面（SSE / 文件流 / 裸字节）**没有逃生口**，
// 走 raw handler：auth.go RotateRefresh、ai_assistant.go SSE 扣分事件、contact.go GetContact、
// recruit.go ResumeCard、resume_pdf.go 两处、settings.go TestConfig。

// errStatusEntry 域表条目：哨兵 → HTTP 状态码（+ 可选固定文案 / 可选人读前缀）。
// sentinel 为 nil = **无条件命中**：本端点除解析错误之外的一切错误都按本条渲染
// （票1b 用它表达「整条错误面只有一个固定码」的收编端点，逐字等价于旧闭包的写法；
// 票8/ADR-0062 起 `*ParseError` 不再归它兜——参数错误恒回自己的码与自己的文案，见 renderError）。
// message 非空 = 渲染这条固定文案，而不是 err.Error()。
// errPrefix 非空 = 渲染「前缀 + err.Error()」——旧闭包 `response.Xxx(c, "查询失败: "+err.Error())`
// 那一族；票1b 实测出这是第四种定制之外的**第五种**形态（ADR-0060 实施回记有账），
// 没有这一格时收编会把人读前缀静默丢掉，即一处未登记的行为变更。
type errStatusEntry struct {
	sentinel  error
	status    int
	message   string
	errPrefix string
}

// errStatusTable 域级「哨兵 → HTTP 状态码」映射表。
// entries 按声明顺序 errors.Is 判定、先命中先用（与被收编的 if-chain 语义逐字等价）；
// fallback 为未命中兜底状态码，0 表示未设——未命中走 500 默认信封。
type errStatusTable struct {
	entries  []errStatusEntry
	fallback int
}

// errStatusAll 端点级单条目表：**除解析错误外**的一切错误都渲染 status，文案取 err.Error()。
// 票1b 用它表达旧 Render 闭包「错误分支只有一个固定码」的写法（sentinel==nil 的无条件条目，
// 见 errStatusEntry 与 renderError 的优先级注释）；票8（ADR-0062）把 *ParseError 让给它自己。
func errStatusAll(status int) *errStatusTable {
	return &errStatusTable{entries: []errStatusEntry{{sentinel: nil, status: status}}}
}

// errStatusAllMsg 同 errStatusAll，但响应文案固定为 msg（旧闭包的 `response.Xxx(c, "字面量")` 形态）。
func errStatusAllMsg(status int, msg string) *errStatusTable {
	return &errStatusTable{entries: []errStatusEntry{{sentinel: nil, status: status, message: msg}}}
}

// errStatusAllPrefix 同 errStatusAll，但文案是「前缀 + err.Error()」——
// 旧闭包 `response.ServerError(c, "查询失败: "+err.Error())` 那一族的等价收编（见 errStatusEntry.errPrefix）。
// 前缀只加在**业务/DB 错误**上：解析错误走自己的文案，不再被前缀包成「更新失败: 请求参数错误: …」。
func errStatusAllPrefix(status int, prefix string) *errStatusTable {
	return &errStatusTable{entries: []errStatusEntry{{sentinel: nil, status: status, errPrefix: prefix}}}
}

// entryMsg 条目的响应文案。固定文案优先；其余走 clientErrorText（ADR-0064 决策 9）。
func entryMsg(e errStatusEntry, err error) string {
	if e.message != "" {
		return e.message
	}
	return clientErrorText(e.status, err, e.errPrefix)
}

// clientErrorText 是「5xx 不外发驱动原文」这条规则的唯一落点（ADR-0064 决策 9）：
//
//	4xx —— 照旧回「前缀 + 错误自身文本」。4xx 的文案本来就是给调用方看的领域说明
//	        （「证件不存在」「专业方向编码已存在」），收掉它会直接伤可用性。
//	5xx —— 一律不回 err.Error()。驱动/ORM 原文（record not found、no such table、
//	        SQL logic error、pq: …、dial tcp …）原样进 message 等于把实现细节交给外部，
//	        而调用方在 5xx 上唯一需要的信息是「服务端失败了、可重试」这一个事实。
//	        真实错误仍经 c.Error 记进 gin 上下文，日志面不丢；要给用户看原因的 5xx
//	        必须改为抛**具名领域错误**（4xx 档）或显式声明固定文案（errStatusAllMsg），
//	        即「说什么」是一次显式决定，而不是 err.Error() 的默认漏出。
//	        有前缀时保留前缀本身（「更新进度失败: 」→「更新进度失败」），丢掉的是尾巴。
func clientErrorText(status int, err error, prefix string) string {
	if status < http.StatusInternalServerError {
		return prefix + err.Error()
	}
	if trimmed := strings.TrimRight(prefix, " :："); trimmed != "" {
		return trimmed
	}
	return "服务器内部错误"
}

// renderError 渲染错误面（票1b 后是本端点错误渲染的唯一入口）。判定序（票8 / ADR-0062 决策 8 翻转）：
//  1. **`*ParseError` 恒优先**——解析错误回自带的状态码与文案。参数错误是 Parse 面的事实，
//     不是「本端点错误面只有一个码」那种业务判断，故任何条目（含 sentinel==nil 的无条件项）
//     都不得把它改写成自己的码：那正是 20 处 `WithSuccess(…, 500)` 把 400 答成 500 的成因。
//     翻转前被钉住的「无条件条目抢在解析错误之前」只为复刻旧闭包的字节形状，代价是错的码；
//     旧闭包对参数错误本来就该回 400（Parse 与 Invoke 两半不同源），故此处不按旧形状保全。
//  2. entries **按声明顺序**单趟扫描——真哨兵以 errors.Is 命中，sentinel==nil 的条目无条件命中
//     **其余**错误；「哨兵 + 尾部无条件条目」的表仍逐字复现旧 if-chain（业务错误那一半形状不变）；
//  3. fallback；4. 500 默认信封。
//
// 具名条目对解析错误命中不了（*ParseError 不包装任何哨兵），故规则 1 提到最前对域表逐字等价
// （锁：TestEndpointErrStatus_ParseError_PrecedesSentinelTableEntries）。
// 现有域表都不含无条件条目，故 2-4 与其逐字不变（域表快照锁 + 37 处论坛契约测试为证）。
// nil 表即纯默认信封（ADR-0024 C2）。
func (t *errStatusTable) renderError(c *gin.Context, err error) {
	var pe *ParseError
	if asParseError(err, &pe) {
		renderStatus(c, pe.Status, pe.Message)
		return
	}
	if t != nil {
		for _, entry := range t.entries {
			if entry.sentinel == nil || errors.Is(err, entry.sentinel) {
				renderStatus(c, entry.status, entryMsg(entry, err))
				return
			}
		}
	}
	if t != nil && t.fallback != 0 {
		renderStatus(c, t.fallback, clientErrorText(t.fallback, err, ""))
		return
	}
	// 未挂表的端点：真实错误记进 gin 上下文（日志面不丢），对外只给「服务器内部错误」。
	c.Error(err) //nolint:errcheck // gin 的 Error 只记账，返回值是链式用的
	response.ServerError(c, "服务器内部错误")
}

// ===== 常用解析器（吸收既有 handler 手写解析链） =====

// bindJSON 绑定请求体到 typed struct，失败返回 400「请求参数错误」。
func bindJSON[T any](c *gin.Context) (*T, error) {
	var req T
	if err := c.ShouldBindJSON(&req); err != nil {
		return nil, badRequest("请求参数错误")
	}
	return &req, nil
}

// bindJSONMsg 绑定请求体，自定义失败文案（如「请求数据无效」）。
func bindJSONMsg[T any](c *gin.Context, failMsg string) (*T, error) {
	var req T
	if err := c.ShouldBindJSON(&req); err != nil {
		return nil, badRequest(failMsg)
	}
	return &req, nil
}

// bindJSONMsgFunc 返回「绑定请求体 + 固定失败文案」的解析函数。
// 供 Parse 字段直接引用，避免每个 handler 写一层只转调的闭包。
func bindJSONMsgFunc[T any](failMsg string) ParseFunc[T] {
	return func(c *gin.Context) (*T, error) { return bindJSONMsg[T](c, failMsg) }
}

// ===== 域中立端点适配器（ADR-0053 §1） =====
//
// 目录域六类实体 × CRUD 是同构复制：api 层每个 handler 原本手写「解析 → 取结构体 →
// 返回指针 → 判空 → 按文案/状态码渲染」约 20 行，差异只有类型参数、成功文案与状态码。
// 下面两个适配器把这两段样板抹平，且都是域中立的（不含任何目录域词汇）：

// invoke 把「取结构体、返回 (结果, error)」的 service 方法适配成端点调用签名，
// 吸收取地址与判空。
//
// 两种用法（都让 Invoke 退化为一行）：
//
//	Invoke: invoke(h.svc.CreateSpecialty)                                  // 方法值，Req 即方法入参
//	Invoke: invoke(func(req *updateReq) (Dict, error) { ... })             // Req 是复合请求体时自行拆参
func invoke[Req, Resp any](fn func(Req) (Resp, error)) InvokeFunc[Req, Resp] {
	return func(_ context.Context, req *Req) (*Resp, error) {
		result, err := fn(*req)
		if err != nil {
			return nil, err
		}
		return &result, nil
	}
}

// success 成功路径的渲染描述：状态码 + 文案 + 是否带载荷。
type success struct {
	// Status 成功状态码；0 表示 200。
	Status int
	// Msg 成功文案。
	Msg string
	// NoData 成功响应不带载荷（老 API 的 data:null 语义）。
	NoData bool
}

// created 成功描述：201 + 文案 + 载荷（对齐 response.Created）。
func created(msg string) *success {
	return &success{Status: http.StatusCreated, Msg: msg}
}

// okMsg 成功描述：200 + 文案 + 载荷（对齐 response.SuccessWithMsg(msg, deref(resp))）。
func okMsg(msg string) *success { return &success{Msg: msg} }

// okMsgNoData 成功描述：200 + 文案 + 无载荷（对齐 response.SuccessWithMsg(msg, nil)）。
func okMsgNoData(msg string) *success { return &success{Msg: msg, NoData: true} }

// successRenderer 返回只写成功面的 Render（按 success 描述）。
// 错误面由 WithSuccess 挂的 ErrStatus 无条件条目承载（解析错误除外，见该方法注释）。
func successRenderer[Req, Resp any](ok *success) RenderFunc[Req, Resp] {
	return func(c *gin.Context, _ *Req, resp *Resp) {
		if ok.NoData {
			response.SuccessWithMsg(c, ok.Msg, nil)
			return
		}
		if ok.Status == http.StatusCreated {
			response.Created(c, ok.Msg, deref(resp))
			return
		}
		response.SuccessWithMsg(c, ok.Msg, deref(resp))
	}
}

// WithSentinel 在已装配的错误面上**前置**一条具名哨兵分档，返回自身便于链式声明：
//
//	}.WithSuccess(okMsg("success"), http.StatusInternalServerError).
//		WithSentinel(service.ErrGenTaskNotFound, http.StatusNotFound).Handle(c)
//
// 存在的理由就是本仓的主判据（ADR-0064 决策 1）：service 层把「不存在」「不可读」「查不动」
// 分开成具名事实之后，api 层要能把它们**分别**落码，而呈现层若仍要统一（例如未兑换与
// 真不存在都答 404、不泄漏存在性）必须是一次显式调用，而不是只有一格可填。
// 默认错误面（WithSuccess 第二参 / sentinel 为 nil 的条目）永远排在哨兵之后。
func (e Endpoint[Req, Resp]) WithSentinel(sentinel error, status int) Endpoint[Req, Resp] {
	if e.ErrStatus == nil {
		e.ErrStatus = &errStatusTable{}
	}
	e.ErrStatus.entries = append([]errStatusEntry{{sentinel: sentinel, status: status}}, e.ErrStatus.entries...)
	return e
}

// WithSentinelsMsg 一次前置多条具名哨兵、共用同一个状态码与**同一句对外文案**
// （形状同 WithSentinel，用于「一组事实在呈现层落同一档」）。
//
// 文案是参数，不是哨兵自己的 Error()：同一件「读不到」的事实在课程面与章节面上要说出
// 不同的对象名，把统一的句子写进 service 层的哨兵里，就等于让「被哪个端点消费」决定
// 「它叫什么」——那是把呈现决定沉到判据层，违反本波不变式（ADR-0064：统一只允许发生在
// 呈现层，且必须是显式决定）。逐条抄 WithSentinel 同样会抹掉「这是一组」这一层信息。
func (e Endpoint[Req, Resp]) WithSentinelsMsg(status int, message string, sentinels ...error) Endpoint[Req, Resp] {
	if e.ErrStatus == nil {
		e.ErrStatus = &errStatusTable{}
	}
	entries := make([]errStatusEntry, 0, len(sentinels))
	for _, sent := range sentinels {
		entries = append(entries, errStatusEntry{sentinel: sent, status: status, message: message})
	}
	e.ErrStatus.entries = append(entries, e.ErrStatus.entries...)
	return e
}

// WithSuccess 按「成功描述 + 默认错误面」装配端点，返回自身便于链式声明：
//
//	Endpoint[In, Out]{
//		Parse:  bindJSONMsgFunc[In]("请求数据无效"),
//		Invoke: invoke(h.svc.Create),
//	}.WithSuccess(created("XX创建成功"), http.StatusBadRequest).Handle(c)
//
// 第二参 errStatus 是**该端点的默认错误面**（errStatusAll：除解析外的一切错误共用这一个码、
// 且不查域表）——业务错误与 DB 故障那一半与票1a 前 renderMsg 错误分支的写法逐字等价。
// 解析面不归它（票8 / ADR-0062 决策 8）：`*ParseError` 恒回自己的 4xx 与自己的文案，
// 所以 `WithSuccess(okMsg(…), 500)` 的端点参数错误天然是 400，不必逐端点改表。
// 需要真正定制渲染的端点是少数，它们继续显式设置 Render（只写成功面）。
func (e Endpoint[Req, Resp]) WithSuccess(ok *success, errStatus int) Endpoint[Req, Resp] {
	e.Render = successRenderer[Req, Resp](ok)
	e.ErrStatus = errStatusAll(errStatus)
	return e
}

// pathInt 解析路径参数为正整数 id；非数字、0 与负数一律 400（带调用方给的那句文案）。
//
// 「路径上的整数 id 不是正整数」是一件**解析层**事实，与「这个资源不存在」无关：改之前这里只看
// `strconv.Atoi` 的 err ⇒ 0 与负数被放行到 service，于是 `GET /course/0` 对外答 404「课程不存在」
// （拿一个不存在的 id 冒充一个不存在的资源），而用户/讲师面因 service 有 `id <= 0` guard 答 400，
// 且用的是另一句文案（「用户 ID 非法」）——同一件输入错误在三个地方说出三种话（ADR-0065 决策 1）。
// `pathInt64` 一直是这里的形状，本函数向它对齐。
//
// service 层那 5 处 `id <= 0` guard **保留**：HTTP 面现在轮不到它触发，但 service 的契约不能依赖
// 「调用方一定是这个 handler」（同一 guard 也管着来自 body 的 id）。被否备选见 ADR-0065。
func pathInt(c *gin.Context, key, failMsg string) (int, error) {
	v, err := strconv.Atoi(c.Param(key))
	if err != nil || v <= 0 {
		return 0, badRequest(failMsg)
	}
	return v, nil
}

// pathInt64 解析路径参数为正整数 id（int64 版），判定与 pathInt 逐字相同。
// 两枚 helper 是**仅有的**两处路径整数解析点；由 ⑤b 把散在 9 个文件里的裸 `strconv.*(c.Param(...))`
// 收进来，之后由 parse_point_drift_lock_test.go 钉住「不许再出现第三处」。
func pathInt64(c *gin.Context, key, failMsg string) (int64, error) {
	v, err := strconv.ParseInt(c.Param(key), 10, 64)
	if err != nil || v <= 0 {
		return 0, badRequest(failMsg)
	}
	return v, nil
}
