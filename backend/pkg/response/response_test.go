package response

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
)

func init() {
	gin.SetMode(gin.TestMode)
}

func setupRouter(handler gin.HandlerFunc) *gin.Engine {
	r := gin.New()
	r.GET("/test", handler)
	return r
}

func assertResponse(t *testing.T, r R, expectedCode int, expectedMsg string, dataNil bool) {
	t.Helper()
	if r.Code != expectedCode {
		t.Errorf("Code = %d，期望 %d", r.Code, expectedCode)
	}
	if expectedMsg != "" && r.Message != expectedMsg {
		t.Errorf("Message = %q，期望 %q", r.Message, expectedMsg)
	}
	if dataNil && r.Data != nil {
		t.Errorf("Data 应为 nil，得到 %v", r.Data)
	}
}

func TestSuccess(t *testing.T) {
	r := setupRouter(func(c *gin.Context) { Success(c, "data") })
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/test", nil)
	r.ServeHTTP(w, req)

	if w.Code != 200 {
		t.Fatalf("HTTP 状态码 = %d，期望 200", w.Code)
	}
	var resp R
	_ = json.Unmarshal(w.Body.Bytes(), &resp)
	assertResponse(t, resp, 200, "success", false)
	if resp.Data != "data" {
		t.Errorf("Data = %v，期望 'data'", resp.Data)
	}
}

func TestSuccessWithMsg(t *testing.T) {
	r := setupRouter(func(c *gin.Context) { SuccessWithMsg(c, "自定义消息", 42) })
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/test", nil)
	r.ServeHTTP(w, req)

	var resp R
	_ = json.Unmarshal(w.Body.Bytes(), &resp)
	assertResponse(t, resp, 200, "自定义消息", false)
}

func TestCreated(t *testing.T) {
	r := setupRouter(func(c *gin.Context) { Created(c, "创建成功", nil) })
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/test", nil)
	r.ServeHTTP(w, req)

	if w.Code != 201 {
		t.Fatalf("HTTP 状态码 = %d，期望 201", w.Code)
	}
	var resp R
	_ = json.Unmarshal(w.Body.Bytes(), &resp)
	assertResponse(t, resp, 201, "创建成功", true)
}

func TestBadRequest(t *testing.T) {
	r := setupRouter(func(c *gin.Context) { BadRequest(c, "参数错误") })
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/test", nil)
	r.ServeHTTP(w, req)

	if w.Code != 400 {
		t.Fatalf("HTTP 状态码 = %d，期望 400", w.Code)
	}
	var resp R
	_ = json.Unmarshal(w.Body.Bytes(), &resp)
	assertResponse(t, resp, 400, "参数错误", true)
}

func TestUnauthorized(t *testing.T) {
	r := setupRouter(func(c *gin.Context) { Unauthorized(c, "未登录") })
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/test", nil)
	r.ServeHTTP(w, req)

	if w.Code != 401 {
		t.Fatalf("HTTP 状态码 = %d，期望 401", w.Code)
	}
	var resp R
	_ = json.Unmarshal(w.Body.Bytes(), &resp)
	assertResponse(t, resp, 401, "未登录", true)
}

func TestForbidden(t *testing.T) {
	r := setupRouter(func(c *gin.Context) { Forbidden(c, "无权限") })
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/test", nil)
	r.ServeHTTP(w, req)

	if w.Code != 403 {
		t.Fatalf("HTTP 状态码 = %d，期望 403", w.Code)
	}
	var resp R
	_ = json.Unmarshal(w.Body.Bytes(), &resp)
	assertResponse(t, resp, 403, "无权限", true)
}

func TestNotFound(t *testing.T) {
	r := setupRouter(func(c *gin.Context) { NotFound(c, "不存在") })
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/test", nil)
	r.ServeHTTP(w, req)

	if w.Code != 404 {
		t.Fatalf("HTTP 状态码 = %d，期望 404", w.Code)
	}
	var resp R
	_ = json.Unmarshal(w.Body.Bytes(), &resp)
	assertResponse(t, resp, 404, "不存在", true)
}

func TestServerError(t *testing.T) {
	r := setupRouter(func(c *gin.Context) { ServerError(c, "服务器错误") })
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/test", nil)
	r.ServeHTTP(w, req)

	if w.Code != 500 {
		t.Fatalf("HTTP 状态码 = %d，期望 500", w.Code)
	}
	var resp R
	_ = json.Unmarshal(w.Body.Bytes(), &resp)
	assertResponse(t, resp, 500, "服务器错误", true)
}

// --- 5xx 不外发驱动原文（ADR-0064 决策 9）---

// driverErr 驱动/ORM 原文的样本（与 internal/api 台账 leakyDriverText 认的那一族同形）。
var driverErr = errors.New("SQL logic error: no such table: recruiter_users (1)")

// TestClientErrorText 4xx 回原文、5xx 保留前缀丢尾巴 —— 这条规则的唯一实现。
func TestClientErrorText(t *testing.T) {
	cases := []struct {
		name   string
		status int
		prefix string
		want   string
	}{
		{"4xx 保留错误原文（给调用方看的领域说明）", 400, "证件不存在: ", "证件不存在: " + driverErr.Error()},
		{"4xx 无前缀", 404, "", driverErr.Error()},
		{"5xx 保留前缀、丢掉尾巴", 500, "导出失败: ", "导出失败"},
		{"5xx 前缀只有中文冒号", 500, "生成报告失败：", "生成报告失败"},
		{"5xx 前缀只有分隔符", 500, " : ", "服务器内部错误"},
		{"5xx 无前缀", 500, "", "服务器内部错误"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := ClientErrorText(tc.status, driverErr, tc.prefix)
			if got != tc.want {
				t.Errorf("ClientErrorText(%d, driverErr, %q) = %q，期望 %q", tc.status, tc.prefix, got, tc.want)
			}
			if tc.status >= 500 && strings.Contains(got, "no such table") {
				t.Errorf("5xx 文案漏出了驱动原文: %q", got)
			}
		})
	}
}

// TestServerErrorCause 5xx 一次做齐：响应体是固定文案（无驱动原文），原因经 c.Error 记账不丢。
func TestServerErrorCause(t *testing.T) {
	var recorded []*gin.Error
	r := setupRouter(func(c *gin.Context) {
		ServerErrorCause(c, "导出失败: ", driverErr)
		recorded = append(recorded, c.Errors...)
	})
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/test", nil)
	r.ServeHTTP(w, req)

	if w.Code != 500 {
		t.Fatalf("HTTP 状态码 = %d，期望 500", w.Code)
	}
	var resp R
	_ = json.Unmarshal(w.Body.Bytes(), &resp)
	assertResponse(t, resp, 500, "导出失败", true)
	if strings.Contains(w.Body.String(), "no such table") {
		t.Errorf("响应体漏出驱动原文: %s", w.Body.String())
	}
	// 记账：真实原因仍在 gin 上下文（日志面不丢）
	if len(recorded) != 1 {
		t.Fatalf("c.Error 记账条数 = %d，期望 1", len(recorded))
	}
	if !errors.Is(recorded[0].Err, driverErr) {
		t.Errorf("记账的错误 = %v，期望原错误", recorded[0].Err)
	}
}

// TestServerErrorCauseNilError err 为 nil 时不 panic 也不记账（前缀仍按 5xx 规则渲染）。
func TestServerErrorCauseNilError(t *testing.T) {
	var recorded []*gin.Error
	r := setupRouter(func(c *gin.Context) {
		ServerErrorCause(c, "", nil)
		recorded = append(recorded, c.Errors...)
	})
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/test", nil)
	r.ServeHTTP(w, req)

	var resp R
	_ = json.Unmarshal(w.Body.Bytes(), &resp)
	assertResponse(t, resp, 500, "服务器内部错误", true)
	if len(recorded) != 0 {
		t.Errorf("nil 错误不应记账，实得 %d 条", len(recorded))
	}
}

// --- PageResult 分页信封 ---

// TestPageResult_PagesComputation pages = ceil(total/pageSize) 的唯一实现，
// 期望值独立手算（不复用公式）。
func TestPageResult_PagesComputation(t *testing.T) {
	cases := []struct {
		name     string
		total    int64
		pageSize int
		want     int
	}{
		{"空列表（total=0）", 0, 10, 0},
		{"精确整除", 20, 10, 2},
		{"非精确整除-余1", 21, 10, 3},
		{"非精确整除-余9", 19, 10, 2},
		{"边界-恰一页", 10, 10, 1},
		{"边界-多一条", 11, 10, 2},
		{"total小于pageSize", 3, 10, 1},
		{"单条单页", 1, 1, 1},
		{"页大小1", 5, 1, 5},
		{"pageSize为0", 5, 0, 5},
		{"pageSize为负", 5, -3, 5},
		{"空列表且pageSize非法", 0, 0, 0},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := PageCount(tc.total, tc.pageSize); got != tc.want {
				t.Errorf("PageCount(%d, %d) = %d，期望 %d", tc.total, tc.pageSize, got, tc.want)
			}
		})
	}
}
