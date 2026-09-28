package logger

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"

	"forklift-training/internal/middleware"
)

func testLogger(t *testing.T, buf *strings.Builder) *zap.Logger {
	t.Helper()
	enc, _ := buildEncoder("json")
	core := zapcore.NewCore(enc, zapcore.AddSync(buf), zapcore.InfoLevel)
	return zap.New(core)
}

func setupRouter(t *testing.T, logger *zap.Logger) *gin.Engine {
	t.Helper()
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(middleware.RequestID())
	r.Use(AccessLog(logger))
	r.GET("/api/ping", func(c *gin.Context) {
		c.Set(string(middleware.CtxUserID), 42)
		c.Set(string(middleware.CtxUserRole), "admin")
		c.JSON(http.StatusOK, gin.H{"ok": true})
	})
	r.GET("/api/health/live", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"ok": true})
	})
	return r
}

func TestAccessLog_Fields(t *testing.T) {
	var buf strings.Builder
	r := setupRouter(t, testLogger(t, &buf))

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/ping", nil)
	req.Header.Set("X-Request-ID", "rid-123")
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d", w.Code)
	}
	lines := strings.Split(strings.TrimSpace(buf.String()), "\n")
	if len(lines) != 1 {
		t.Fatalf("应有 1 条访问日志, got %d: %s", len(lines), buf.String())
	}
	var entry map[string]any
	if err := json.Unmarshal([]byte(lines[0]), &entry); err != nil {
		t.Fatalf("日志应为 JSON: %v", err)
	}
	for _, k := range []string{"method", "path", "status", "duration_ms", "ip", "user_id", "user_role", "request_id"} {
		if _, ok := entry[k]; !ok {
			t.Errorf("访问日志缺少字段 %q: %v", k, entry)
		}
	}
	if entry["user_id"] != float64(42) {
		t.Errorf("user_id 应为 42, got %v", entry["user_id"])
	}
	if entry["user_role"] != "admin" {
		t.Errorf("user_role 应为 admin, got %v", entry["user_role"])
	}
	if rid, _ := entry["request_id"].(string); rid == "" || rid == "rid-123" {
		// 请求身份由服务端铸造（ADR-0062 票1）：调用方供给的 X-Request-ID 不再进日志，
		// 否则外部可控 ID 既污染排查面，又是 AI 计量「固定头即免扣费」通道的来源。
		t.Errorf("request_id 应为服务端铸造值（非空且不等于调用方供给的头）, got %v", entry["request_id"])
	}
	if entry["path"] != "/api/ping" {
		t.Errorf("path 应为 /api/ping, got %v", entry["path"])
	}
}

// lastLogEntry 解析缓冲区里最后一条访问日志（测试内共用）。
func lastLogEntry(t *testing.T, buf *strings.Builder) map[string]any {
	t.Helper()
	lines := strings.Split(strings.TrimSpace(buf.String()), "\n")
	if len(lines) == 0 || lines[len(lines)-1] == "" {
		t.Fatalf("没有访问日志: %q", buf.String())
	}
	var entry map[string]any
	if err := json.Unmarshal([]byte(lines[len(lines)-1]), &entry); err != nil {
		t.Fatalf("日志应为 JSON: %v", err)
	}
	return entry
}

// TestAccessLog_ErrorField c.Error 记账有消费面（ADR-0064 决策 9 的配套）：
// 5xx 不外发驱动原文之后，原因的唯一去处是 gin 上下文 —— 没人读它就等于丢了。
// 本中间件是请求收尾处唯一的读者：有记账 ⇒ error 字段落地且经脱敏；无记账 ⇒ 不多一个字段。
func TestAccessLog_ErrorField(t *testing.T) {
	var buf strings.Builder
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(middleware.RequestID())
	r.Use(AccessLog(testLogger(t, &buf)))
	r.GET("/api/boom", func(c *gin.Context) {
		_ = c.Error(errors.New("dial tcp postgres://app:secret@db:5432: connect: refused"))
		c.JSON(http.StatusInternalServerError, gin.H{"message": "服务器内部错误"})
	})
	r.GET("/api/ok", func(c *gin.Context) { c.JSON(http.StatusOK, gin.H{"ok": true}) })

	r.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/api/boom", nil))
	entry := lastLogEntry(t, &buf)
	msg, _ := entry["error"].(string)
	if msg == "" {
		t.Fatalf("有 c.Error 记账时访问日志应带 error 字段: %v", entry)
	}
	if strings.Contains(msg, "app:secret@") {
		t.Errorf("error 字段未脱敏（连接串凭证进日志）: %q", msg)
	}
	if !strings.Contains(msg, "connect: refused") {
		t.Errorf("error 字段应保留真实原因: %q", msg)
	}

	buf.Reset()
	r.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/api/ok", nil))
	entry = lastLogEntry(t, &buf)
	if _, ok := entry["error"]; ok {
		t.Errorf("无记账时不应出现 error 字段: %v", entry)
	}
}

func TestAccessLog_HealthSkipped(t *testing.T) {
	var buf strings.Builder
	r := setupRouter(t, testLogger(t, &buf))

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/health/live", nil)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d", w.Code)
	}
	if buf.Len() != 0 {
		t.Errorf("health 路径不应产生访问日志: %s", buf.String())
	}
}
