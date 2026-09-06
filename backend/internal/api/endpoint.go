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

// InvokeFunc 调用 service：Req → Resp。err 交给 Render 决定状态码与文案。
type InvokeFunc[Req, Resp any] func(ctx context.Context, req *Req) (*Resp, error)

// RenderFunc 将 (Req, Resp, error) 渲染为响应。Render 全权负责写响应。
type RenderFunc[Req, Resp any] func(c *gin.Context, req *Req, resp *Resp, err error)

// Endpoint 泛型端点骨架：parse → invoke → render 三段式守卫链。
// Req 为 typed 请求（query/路径/body 字段），Resp 为 service 返回的 typed DTO。
type Endpoint[Req, Resp any] struct {
	// Parse 解析请求。为 nil 时使用零值 Req（无请求参数的端点）。
	Parse ParseFunc[Req]
	// Invoke 调用 service。为 nil 时跳过调用（Resp 保持 nil）。
	Invoke InvokeFunc[Req, Resp]
	// Render 渲染响应，全权负责写响应（含 err→状态码/信封）。省略时走内置默认信封（ADR-0024 C2）：
	// 成功 → 200 统一信封；错误路径经 ErrStatus 域表（未关联表时 ParseError → 其状态码、其余 500）。
	// 自定义 Render 优先级高于 ErrStatus：设置 Render 后域表对该端点不再生效，
	// 仍需查表的定制端点在 Render 内显式调用域表 renderError。
	Render RenderFunc[Req, Resp]
	// ErrStatus 域级哨兵→状态码表（#610/#611）：Render 省略时错误路径查表兜底——
	// errors.Is 命中 → 表内状态码；未命中 → 表 fallback（未设 → 500）。
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
	if e.Render == nil {
		// 默认信封（ADR-0024 C2 + #610/#611 域表兜底）：成功统一信封；错误路径走域表
		//（未关联表时即纯默认：ParseError → 其状态码，其余 500）。
		if err == nil {
			response.Success(c, deref(resp))
			return
		}
		e.ErrStatus.renderError(c, err)
		return
	}
	e.Render(c, req, resp, err)
}

// deref 解引用指针；nil 返回 nil（保持 JSON "data": null 语义）。
func deref[T any](p *T) any {
	if p == nil {
		return nil
	}
	return *p
}

// renderStatus 按状态码输出信封（收敛到 response 单点）。
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

// ===== 域级哨兵→状态码表（#610/#611） =====
//
// 每域一张「哨兵 → HTTP 状态码」表（pointsErrStatus / contributionErrStatus 等，写在各域文件内）：
// HTTP 语义归 api 侧（ADR-0024「handler 以 errors.Is 映射状态码」的投影位置），同域端点共用；
// 不做全仓中央表——同一哨兵跨域可归属不同状态码（如 ErrJobNotFound 同时进 job / application /
// recruiterApplication 三张表）。
//
// 收编后残留的手写映射链仅限表无法表达的真实渲染定制（哨兵→固定文案或定制 500 文案）：
//   - forum.go GetTopic/AdminGetTopic：gorm.ErrRecordNotFound → 404「主题不存在」（固定文案）
//   - job_card.go GetJobCard：gorm.ErrRecordNotFound → 404「简历不存在」（固定文案）
//
// Render 闭包之外的手写映射（raw handler，非本骨架管辖）不在收编范围：auth.go RotateRefresh、
// ai_assistant.go SSE 扣分事件、contact.go GetContact、forum.go AcceptReply/CancelAccept、
// recruit.go ResumeCard、resume_pdf.go 两处、settings.go TestConfig。

// errStatusEntry 域表条目：哨兵 → HTTP 状态码。
type errStatusEntry struct {
	sentinel error
	status   int
}

// errStatusTable 域级「哨兵 → HTTP 状态码」映射表。
// entries 按声明顺序 errors.Is 判定、先命中先用（与被收编的 if-chain 语义逐字等价）；
// fallback 为未命中兜底状态码，0 表示未设——未命中走 500 默认信封。
type errStatusTable struct {
	entries  []errStatusEntry
	fallback int
}

// renderError 渲染 invoke 错误：*ParseError 优先（解析错误不属业务哨兵）→ 表内命中 →
// fallback（未设 → 500）。nil 表即纯默认信封（ADR-0024 C2）。
// 定制端点的 Render 需要复用域表时也直接调用本方法（如 contributionErrStatus.renderError）。
func (t *errStatusTable) renderError(c *gin.Context, err error) {
	var pe *ParseError
	if asParseError(err, &pe) {
		renderStatus(c, pe.Status, pe.Message)
		return
	}
	if t != nil {
		for _, entry := range t.entries {
			if errors.Is(err, entry.sentinel) {
				renderStatus(c, entry.status, err.Error())
				return
			}
		}
		if t.fallback != 0 {
			renderStatus(c, t.fallback, err.Error())
			return
		}
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
