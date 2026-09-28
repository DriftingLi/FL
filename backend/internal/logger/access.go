package logger

import (
	"time"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"

	"forklift-training/internal/middleware"
)

// AccessLog 请求访问日志中间件。
// 记录 method/path/status/duration/ip/user_id/user_role/request_id + **c.Errors 里的原因**；
// 不记录请求体与 query（避免 PII 与凭证进入日志流）。
// 健康检查探活路径（middleware.HealthPaths）跳过，避免刷屏淹没真实请求。
//
// 为什么带上 c.Errors（ADR-0064 决策 9 的配套）：5xx 一律**不外发**驱动原文，真实原因改为
// 经 c.Error 记进 gin 上下文 —— 那个上下文在请求结束时随对象一起丢掉，若无人读它就等于
// 把原因从响应搬到了一个没人看的地方（「日志面不丢」这句话不成立）。本中间件是唯一在读
// 请求收尾的地方，故它是这条记账的消费面（无错误时不加字段，正常请求零额外字段）。
// 文本经 RedactError 洗一遍：错误里可能带凭证 URL（口径同 logger 其余出口）。
func AccessLog(l *zap.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		if _, skip := middleware.HealthPaths[c.Request.URL.Path]; skip {
			c.Next()
			return
		}

		start := time.Now()
		c.Next()

		fields := []zap.Field{
			zap.String("method", c.Request.Method),
			zap.String("path", c.Request.URL.Path),
			zap.Int("status", c.Writer.Status()),
			zap.Float64("duration_ms", float64(time.Since(start).Microseconds())/1000),
			zap.String("ip", middleware.ClientIP(c)),
			zap.Int64("user_id", int64(middleware.CurrentUserID(c))),
			zap.String("user_role", middleware.CurrentRole(c)),
			zap.String("request_id", c.GetString(string(middleware.CtxRequestID))),
		}
		if len(c.Errors) > 0 {
			fields = append(fields, zap.String("error", RedactText(c.Errors.String())))
		}
		l.Info("request", fields...)
	}
}
