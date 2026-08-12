package logger

import (
	"encoding/json"
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
	if entry["request_id"] != "rid-123" {
		t.Errorf("request_id 应为 rid-123, got %v", entry["request_id"])
	}
	if entry["path"] != "/api/ping" {
		t.Errorf("path 应为 /api/ping, got %v", entry["path"])
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
