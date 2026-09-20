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

// errStatusEntry 域表条目：哨兵 → HTTP 状态码（+ 可选固定文案）。
// sentinel 为 nil = **无条件命中**，含 *ParseError 在内的一切错误都按本条渲染
// （票1b 用它表达「整条错误面只有一个固定码」的收编端点，逐字等价于旧闭包的写法）。
// message 非空 = 渲染这条固定文案，而不是 err.Error()。
type errStatusEntry struct {
	sentinel error
	status   int
	message  string
}

// errStatusTable 域级「哨兵 → HTTP 状态码」映射表。
// entries 按声明顺序 errors.Is 判定、先命中先用（与被收编的 if-chain 语义逐字等价）；
// fallback 为未命中兜底状态码，0 表示未设——未命中走 500 默认信封。
type errStatusTable struct {
	entries  []errStatusEntry
	fallback int
}

// errStatusAll 端点级单条目表：**一切错误**（含 *ParseError，不查域表）都渲染 status，
// 文案取 err.Error()。票1b 用它表达旧 Render 闭包「错误分支只有一个固定码」的写法
// （sentinel==nil 的无条件条目，见 errStatusEntry 与 renderError 的优先级注释）。
func errStatusAll(status int) *errStatusTable {
	return &errStatusTable{entries: []errStatusEntry{{sentinel: nil, status: status}}}
}

// errStatusAllMsg 同 errStatusAll，但响应文案固定为 msg（旧闭包的 `response.Xxx(c, "字面量")` 形态）。
func errStatusAllMsg(status int, msg string) *errStatusTable {
	return &errStatusTable{entries: []errStatusEntry{{sentinel: nil, status: status, message: msg}}}
}

// entryMsg 条目的响应文案：固定文案优先，否则错误自身文本。
func entryMsg(e errStatusEntry, err error) string {
	if e.message != "" {
		return e.message
	}
	return err.Error()
}

// renderError 渲染错误面（票1b 后是本端点错误渲染的唯一入口）。判定序：
//  1. entries **按声明顺序**单趟扫描——sentinel==nil 的条目无条件命中（含 *ParseError），
//     真哨兵以 errors.Is 命中；因此「哨兵 + 尾部无条件条目」的表逐字复现旧 if-chain，
//     而只有一条无条件项的表（errStatusAll / WithSuccess）自然抢在 *ParseError 规则之前；
//  2. *ParseError → 其自带状态码与文案（解析错误不属业务哨兵，且现有域表都不含无条件条目）；
//  3. fallback；4. 500 默认信封。
//
// 现有域表都不含无条件条目，故 2-4 与其逐字不变（域表快照锁 + 37 处论坛契约测试为证）。
// nil 表即纯默认信封（ADR-0024 C2）。
func (t *errStatusTable) renderError(c *gin.Context, err error) {
	if t != nil {
		for _, entry := range t.entries {
			if entry.sentinel == nil || errors.Is(err, entry.sentinel) {
				renderStatus(c, entry.status, entryMsg(entry, err))
				return
			}
		}
	}
	var pe *ParseError
	if asParseError(err, &pe) {
		renderStatus(c, pe.Status, pe.Message)
		return
	}
	if t != nil && t.fallback != 0 {
		renderStatus(c, t.fallback, err.Error())
		return
	}
	response.ServerError(c, err.Error())
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
// 错误面由 WithSuccess 挂的 ErrStatus 无条件条目承载（见该方法的注释）。
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

// WithSuccess 按「成功描述 + 错误状态码」装配端点，返回自身便于链式声明：
//
//	Endpoint[In, Out]{
//		Parse:  bindJSONMsgFunc[In]("请求数据无效"),
//		Invoke: invoke(h.svc.Create),
//	}.WithSuccess(created("XX创建成功"), http.StatusBadRequest).Handle(c)
//
// 错误面是**无条件**的单一状态码（errStatusAll：解析错误与业务错误在该域共用同一个错误状态码、
// 且不查域表）——与票1a 前 renderMsg 错误分支的写法逐字等价。
// 需要真正定制渲染的端点是少数（见本文件末尾的清单），它们继续显式设置 Render（只写成功面）。
func (e Endpoint[Req, Resp]) WithSuccess(ok *success, errStatus int) Endpoint[Req, Resp] {
	e.Render = successRenderer[Req, Resp](ok)
	e.ErrStatus = errStatusAll(errStatus)
	return e
}

// pathInt 解析路径参数为 int，失败返回 400 自定义文案。
func pathInt(c *gin.Context, key, failMsg string) (int, error) {
	v, err := strconv.Atoi(c.Param(key))
	if err != nil {
		return 0, badRequest(failMsg)
	}
	return v, nil
}

// pathInt64 解析路径参数为 int64，失败或 <=0 返回 400 自定义文案。
func pathInt64(c *gin.Context, key, failMsg string) (int64, error) {
	v, err := strconv.ParseInt(c.Param(key), 10, 64)
	if err != nil || v <= 0 {
		return 0, badRequest(failMsg)
	}
	return v, nil
}
