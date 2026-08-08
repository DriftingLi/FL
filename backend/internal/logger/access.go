package logger

import (
	"time"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"

	"forklift-training/internal/middleware"
)

// healthPaths 健康检查探活路径：不出现在访问日志中，避免刷屏淹没真实请求。
var healthPaths = map[string]struct{}{
	"/api/health":      {},
	"/api/health/live": {},
}

// AccessLog 请求访问日志中间件。
// 记录 method/path/status/duration/ip/user_id/user_role/request_id；
// 不记录请求体与 query（避免 PII 与凭证进入日志流）。
func AccessLog(l *zap.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		if _, skip := healthPaths[c.Request.URL.Path]; skip {
			c.Next()
			return
		}

		start := time.Now()
		c.Next()

		l.Info("request",
			zap.String("method", c.Request.Method),
			zap.String("path", c.Request.URL.Path),
			zap.Int("status", c.Writer.Status()),
			zap.Float64("duration_ms", float64(time.Since(start).Microseconds())/1000),
			zap.String("ip", c.ClientIP()),
			zap.Int64("user_id", int64(middleware.CurrentUserID(c))),
			zap.String("user_role", middleware.CurrentRole(c)),
			zap.String("request_id", c.GetString(string(middleware.CtxRequestID))),
		)
	}
}
