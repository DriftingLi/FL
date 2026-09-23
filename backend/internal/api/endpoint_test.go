// endpoint.go 骨架单元测试：parse 失败→400、invoke 错误→500、panic 恢复→500、成功→200。
package api

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"

	"forklift-training/pkg/response"
)

func init() {
	gin.SetMode(gin.TestMode)
}

// doEndpoint 构造 gin 引擎并命中单条路由，返回响应。
func doEndpoint(t *testing.T, e Endpoint[int, string]) *httptest.ResponseRecorder {
	t.Helper()
	r := gin.New()
	r.POST("/x", e.Handle)
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/x", nil)
	r.ServeHTTP(w, req)
	return w
}

// renderSuccessOnly 测试用显式 Render：只写成功面（票1b 后 err 不在签名上，错误面归骨架）。
// 用于对照断言「挂 Render 与否，错误面与默认信封逐字节一致」。
func renderSuccessOnly[Resp any](c *gin.Context, _ *int, resp *Resp) {
	response.Success(c, deref(resp))
}

// TestEndpoint_ParseFailure_BadRequest parse 返回 badRequest → 400 + 原文案。
func TestEndpoint_ParseFailure_BadRequest(t *testing.T) {
	e := Endpoint[int, string]{
		Parse: func(c *gin.Context) (*int, error) {
			return nil, badRequest("参数非法")
		},
		// Render 省略：走默认信封（ADR-0024 C2）
	}
	w := doEndpoint(t, e)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("状态码 = %d, 期望 400", w.Code)
	}
	if !strings.Contains(w.Body.String(), "参数非法") {
		t.Fatalf("文案不符: %s", w.Body.String())
	}
}

// TestEndpoint_InvokeServiceError_ServerError invoke 返回普通 error → 500 + err.Error()。
func TestEndpoint_InvokeServiceError_ServerError(t *testing.T) {
	e := Endpoint[int, string]{
		Invoke: func(ctx context.Context, req *int) (*string, error) {
			return nil, errors.New("服务崩了")
		},
		// Render 省略：invoke 错误 → 500
	}
	w := doEndpoint(t, e)
	if w.Code != http.StatusInternalServerError {
		t.Fatalf("状态码 = %d, 期望 500", w.Code)
	}
	// 5xx 不外发错误原文（ADR-0064 决策 9）：驱动/ORM 文本进 message 等于把实现细节交给外部，
	// 调用方在 5xx 上唯一需要的答案是「服务端失败、可重试」。真实错误经 c.Error 记进上下文。
	if msg := w.Body.String(); !strings.Contains(msg, "服务器内部错误") || strings.Contains(msg, "服务崩了") {
		t.Fatalf("5xx 文案应为固定「服务器内部错误」且不含错误原文: %s", msg)
	}
}

// TestEndpoint_ParseNotFound parse 返回 404 ParseError → 404。
func TestEndpoint_ParseNotFound(t *testing.T) {
	e := Endpoint[int, string]{
		Parse: func(c *gin.Context) (*int, error) {
			return nil, &ParseError{Status: http.StatusNotFound, Message: "不存在"}
		},
		// Render 省略：ParseError 404 → 404
	}
	w := doEndpoint(t, e)
	if w.Code != http.StatusNotFound {
		t.Fatalf("状态码 = %d, 期望 404", w.Code)
	}
}

// TestEndpoint_Success_200 invoke 成功 → 200 + data。
func TestEndpoint_Success_200(t *testing.T) {
	e := Endpoint[int, string]{
		Invoke: func(ctx context.Context, req *int) (*string, error) {
			v := "ok"
			return &v, nil
		},
		// Render 省略：成功 → 200 统一信封
	}
	w := doEndpoint(t, e)
	if w.Code != http.StatusOK {
		t.Fatalf("状态码 = %d, 期望 200", w.Code)
	}
	if !strings.Contains(w.Body.String(), "ok") {
		t.Fatalf("data 不符: %s", w.Body.String())
	}
}

// TestEndpoint_PanicRecovery_ServerError invoke panic → 恢复为 500 信封。
func TestEndpoint_PanicRecovery_ServerError(t *testing.T) {
	e := Endpoint[int, string]{
		Invoke: func(ctx context.Context, req *int) (*string, error) {
			panic("boom")
		},
		// Render 省略：panic 兜底 500
	}
	w := doEndpoint(t, e)
	if w.Code != http.StatusInternalServerError {
		t.Fatalf("状态码 = %d, 期望 500", w.Code)
	}
	if !strings.Contains(w.Body.String(), "服务器内部错误") {
		t.Fatalf("兜底文案不符: %s", w.Body.String())
	}
}

// TestEndpoint_NilParse_UsesZeroReq Parse 为 nil 时用零值 Req，invoke 正常。
func TestEndpoint_NilParse_UsesZeroReq(t *testing.T) {
	e := Endpoint[int, string]{
		Invoke: func(ctx context.Context, req *int) (*string, error) {
			if *req != 0 {
				t.Fatalf("零值 Req 期望 0, got %d", *req)
			}
			v := "zero"
			return &v, nil
		},
		// Render 省略：Parse nil 用零值 Req
	}
	w := doEndpoint(t, e)
	if w.Code != http.StatusOK {
		t.Fatalf("状态码 = %d, 期望 200", w.Code)
	}
}

// TestEndpoint_DefaultRender_ByteEquivalent 默认信封与显式成功面 Render 字节级等价（ADR-0024 C2 + 票1b）：
// 成功 → 200 统一信封由 Render 承载；*ParseError → 其状态码、其他错误 → 500 两条错误面**与挂没挂
// Render 无关**（票1b 把 err 从签名上拿掉后，错误面只有骨架一个作者）。
func TestEndpoint_DefaultRender_ByteEquivalent(t *testing.T) {
	// 成功路径
	okInvoke := func(ctx context.Context, req *int) (*string, error) {
		v := "ok"
		return &v, nil
	}
	explicit := Endpoint[int, string]{
		Invoke: okInvoke,
		Render: renderSuccessOnly[string],
	}
	implicit := Endpoint[int, string]{
		Invoke: okInvoke,
		// Render 省略 → 默认信封
	}
	if a, b := doEndpoint(t, explicit).Body.String(), doEndpoint(t, implicit).Body.String(); a != b {
		t.Fatalf("成功信封不一致:\n显式=%s\n默认=%s", a, b)
	}

	// 服务错误路径
	errInvoke := func(ctx context.Context, req *int) (*string, error) {
		return nil, errors.New("服务崩了")
	}
	explicit = Endpoint[int, string]{Invoke: errInvoke, Render: renderSuccessOnly[string]}
	implicit = Endpoint[int, string]{Invoke: errInvoke}
	if a, b := doEndpoint(t, explicit).Body.String(), doEndpoint(t, implicit).Body.String(); a != b {
		t.Fatalf("500 信封不一致:\n显式=%s\n默认=%s", a, b)
	}

	// 解析错误路径
	parseErr := func(c *gin.Context) (*int, error) {
		return nil, badRequest("参数非法")
	}
	explicit = Endpoint[int, string]{Parse: parseErr, Render: renderSuccessOnly[string]}
	implicit = Endpoint[int, string]{Parse: parseErr}
	if a, b := doEndpoint(t, explicit).Body.String(), doEndpoint(t, implicit).Body.String(); a != b {
		t.Fatalf("400 信封不一致:\n显式=%s\n默认=%s", a, b)
	}
}
