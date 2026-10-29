package api

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"

	"forklift-training/internal/testutil"
)

// /api/health 探的是**数据库**（#2109 起不再探 Redis）：
// 能 ping 通 → 200；句柄缺失或 ping 失败 → 503 degraded，降级字段是 database。
func TestHealthCheckProbesDatabase(t *testing.T) {
	gin.SetMode(gin.TestMode)

	t.Run("数据库可用", func(t *testing.T) {
		r := gin.New()
		db := testutil.NewMemoryDB(t)
		r.GET("/api/health", InjectProbe(func(ctx context.Context) error {
			sqlDB, err := db.DB()
			if err != nil {
				return err
			}
			return sqlDB.PingContext(ctx)
		}), HealthCheck)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/api/health", nil))
		if w.Code != http.StatusOK {
			t.Fatalf("健康应 200, got %d %s", w.Code, w.Body.String())
		}
		if !strings.Contains(w.Body.String(), "backend is running") {
			t.Fatalf("响应体形状变了: %s", w.Body.String())
		}
	})

	t.Run("句柄未注入", func(t *testing.T) {
		r := gin.New()
		r.GET("/api/health", HealthCheck) // 故意不挂 InjectDB
		w := httptest.NewRecorder()
		r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/api/health", nil))
		if w.Code != http.StatusServiceUnavailable {
			t.Fatalf("缺句柄应 503, got %d", w.Code)
		}
		if !strings.Contains(w.Body.String(), "\"database\":\"unreachable\"") {
			t.Fatalf("降级字段应为 database:unreachable（不再是 redis）: %s", w.Body.String())
		}
	})

	t.Run("数据库已关闭", func(t *testing.T) {
		db := testutil.NewMemoryDB(t)
		sqlDB, err := db.DB()
		if err != nil {
			t.Fatalf("取 sql.DB 失败: %v", err)
		}
		_ = sqlDB.Close()

		r := gin.New()
		r.GET("/api/health", InjectProbe(func(ctx context.Context) error {
			return sqlDB.PingContext(ctx)
		}), HealthCheck)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/api/health", nil))
		if w.Code != http.StatusServiceUnavailable {
			t.Fatalf("库不可用应 503, got %d %s", w.Code, w.Body.String())
		}
	})
}

// 存活探针不依赖任何外部组件：即使数据库挂了也必须 200（容器编排用它决定要不要重启）。
func TestHealthLiveIgnoresDatabase(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.GET("/api/health/live", HealthLive)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/api/health/live", nil))
	if w.Code != http.StatusOK {
		t.Fatalf("存活探针应恒 200, got %d", w.Code)
	}
}
