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

// renderAsDefault 测试用显式 Render：复刻既有默认渲染语义（err==nil → 200；
// *ParseError → 其状态码；其他 → 500）。原 defaultRender 删除后，测试改用显式 Render 断言同一行为。
func renderAsDefault[Resp any](c *gin.Context, _ *int, resp *Resp, err error) {
	var pe *ParseError
	switch {
	case err == nil:
		response.Success(c, deref(resp))
	case asParseError(err, &pe):
		renderStatus(c, pe.Status, pe.Message)
	default:
		response.ServerError(c, err.Error())
	}
}

// TestEndpoint_ParseFailure_BadRequest parse 返回 badRequest → 400 + 原文案。
func TestEndpoint_ParseFailure_BadRequest(t *testing.T) {
	e := Endpoint[int, string]{
		Parse: func(c *gin.Context) (*int, error) {
			return nil, badRequest("参数非法")
		},
		Render: renderAsDefault[string],
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
		Render: renderAsDefault[string],
	}
	w := doEndpoint(t, e)
	if w.Code != http.StatusInternalServerError {
		t.Fatalf("状态码 = %d, 期望 500", w.Code)
	}
	if !strings.Contains(w.Body.String(), "服务崩了") {
		t.Fatalf("文案不符: %s", w.Body.String())
	}
}

// TestEndpoint_ParseNotFound parse 返回 404 ParseError → 404。
func TestEndpoint_ParseNotFound(t *testing.T) {
	e := Endpoint[int, string]{
		Parse: func(c *gin.Context) (*int, error) {
			return nil, &ParseError{Status: http.StatusNotFound, Message: "不存在"}
		},
		Render: renderAsDefault[string],
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
		Render: renderAsDefault[string],
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
		Render: renderAsDefault[string],
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
		Render: renderAsDefault[string],
	}
	w := doEndpoint(t, e)
	if w.Code != http.StatusOK {
		t.Fatalf("状态码 = %d, 期望 200", w.Code)
	}
}
