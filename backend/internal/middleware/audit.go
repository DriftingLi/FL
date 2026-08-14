package middleware

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"

	"forklift-training/internal/model"
	"forklift-training/internal/service"
)

// AuditLog 管理员/讲师写操作审计中间件。
// 挂载在 /api 路由组，不依赖中间件顺序：请求处理完成后读取 JWT 上下文，
// 仅记录 admin / tutor 角色的 POST/PUT/PATCH/DELETE 请求。
// 审计写（Write）与操作命名（DescribeAction）下沉到 service.AuditService 单点。
func AuditLog(svc *service.AuditService, logger *zap.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		// 测试装配（如估值模块 handler 测试）可能不注入审计服务：审计降级为跳过
		if svc == nil {
			c.Next()
			return
		}
		method := c.Request.Method
		if method != http.MethodPost && method != http.MethodPut &&
			method != http.MethodPatch && method != http.MethodDelete {
			c.Next()
			return
		}
		if !strings.HasPrefix(c.Request.URL.Path, "/api/") {
			c.Next()
			return
		}

		// 仅缓存小体积 JSON 请求体用于审计；大文件上传 / 未知长度请求体跳过，
		// 避免内存开销，也避免截断后破坏后续 handler 读取。
		var body string
		if strings.HasPrefix(c.Request.Header.Get("Content-Type"), "application/json") &&
			c.Request.ContentLength > 0 && c.Request.ContentLength <= 64*1024 {
			if data, err := io.ReadAll(c.Request.Body); err == nil {
				body = string(data)
				c.Request.Body = io.NopCloser(bytes.NewReader(data))
			}
		}

		start := time.Now()
		c.Next()

		role := CurrentRole(c)
		userID := CurrentUserID(c)
		if userID <= 0 || (role != "admin" && role != "tutor") {
			return
		}

		detail, _ := json.Marshal(map[string]any{
			"query":        c.Request.URL.RawQuery,
			"request_body": body,
			"status":       c.Writer.Status(),
			"duration_ms":  time.Since(start).Milliseconds(),
		})
		record := model.AuditLog{
			ActorID:   userID,
			ActorRole: role,
			ActorName: CurrentAccount(c),
			Action:    svc.DescribeAction(method, c.Request.URL.Path),
			Path:      c.Request.URL.Path,
			Method:    method,
			RequestID: c.GetString(string(CtxRequestID)),
			IP:        c.ClientIP(),
			Status:    c.Writer.Status(),
			Detail:    model.JSONB(detail),
			CreatedAt: time.Now(),
		}
		if err := svc.Write(record); err != nil {
			logger.Warn("写入审计日志失败", zap.Error(err), zap.String("path", c.Request.URL.Path))
		}
	}
}
