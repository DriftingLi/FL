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

	"forklift-training/internal/authz"
	"forklift-training/internal/model"
)

// AuditWriter 审计写入口：中间件只认这两个动作，不认 `*service.AuditService` 这个具体类型。
//
// 为什么接口声明在消费方而不是直接 import internal/service（ADR-0070）：域包要被 internal/service
// import（那 7 个事件构造器），域包的 handler*.go 又要 import 本包拿 JWTAuth / CapabilityRequired
// ——「middleware → service → 域包 → middleware」是个三角，而这条边编译器只在有人撞上时报，
// 报出来的还是无关的 cmd（第一次拆 notification 时就撞上了）。实现仍是单点：service.AuditService 的
// Write / DescribeAction 原样满足本接口，装配点传的就是它。
//
// 注意 typed nil：装配点必须先判空具体实现再传进来（internal/api/router.go 与
// internal/valuation/handler/router.go 都在注入前判 nil）——(*service.AuditService)(nil) 装进接口
// 不等于 nil 接口，下面的 svc == nil 拦不住它。
type AuditWriter interface {
	// Write 落一条审计记录。
	Write(record model.AuditLog) error
	// DescribeAction 把 method + path 命名成操作名（人话动作）。
	DescribeAction(method, path string) string
}

// AuditLog 管理员/讲师写操作审计中间件。
// 挂载在 /api 路由组，不依赖中间件顺序：请求处理完成后读取 JWT 上下文，
// 仅记录 admin / tutor 角色的 POST/PUT/PATCH/DELETE 请求。
// 审计写（Write）与操作命名（DescribeAction）由 svc 背后的 service.AuditService 单点实现（见 AuditWriter）。
func AuditLog(svc AuditWriter, logger *zap.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		// 兜底：装配点已按 nil 具体实现决定挂不挂（见 AuditWriter 的 typed nil 注释），
		// 这里只拦真正传进来的 nil 接口。
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
		if userID <= 0 || (role != string(authz.RoleAdmin) && role != string(authz.RoleTutor)) {
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
			IP:        ClientIP(c),
			Status:    c.Writer.Status(),
			Detail:    model.JSONB(detail),
			CreatedAt: time.Now(),
		}
		if err := svc.Write(record); err != nil {
			logger.Warn("写入审计日志失败", zap.Error(err), zap.String("path", c.Request.URL.Path))
		}
	}
}
