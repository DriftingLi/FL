// Package middleware 提供 Gin 中间件：CORS、JWT 认证、请求日志、panic 恢复。
package middleware

import (
	"time"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"go.uber.org/zap"

	"forklift-training/internal/authz"
	"forklift-training/internal/security"
	"forklift-training/pkg/response"
)

// ContextKey 是 context 中存储用户信息的键。
type ContextKey string

const (
	// CtxUserID 用户ID
	CtxUserID ContextKey = "user_id"
	// CtxAccount 登录账号
	CtxAccount ContextKey = "account"
	// CtxUserRole 用户角色
	CtxUserRole ContextKey = "role"
	// CtxRequestID 请求ID
	CtxRequestID ContextKey = "request_id"
)

// Claims JWT 声明（统一由 security 会话模块持有）。
type Claims = security.Claims

// RequestID 为每个请求注入唯一 ID。
func RequestID() gin.HandlerFunc {
	return func(c *gin.Context) {
		rid := c.GetHeader("X-Request-ID")
		if rid == "" {
			rid = uuid.NewString()
		}
		c.Set(string(CtxRequestID), rid)
		c.Header("X-Request-ID", rid)
		c.Next()
	}
}

// CORS 跨域中间件。
// 开发环境放开全部来源（本地前端可能运行在任意端口/子域名，避免改端口后被拦截）；
// 生产环境仍按配置白名单校验。
// 在闭包外构造一次 cors.Handler，避免每个请求重复创建（行为保持一致）。
func CORS(origins []string, isProd bool) gin.HandlerFunc {
	config := cors.Config{
		AllowMethods:     []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
		AllowHeaders:     []string{"Content-Type", "Authorization", "X-Silent", "Accept"},
		AllowCredentials: true,
		MaxAge:           12 * time.Hour,
	}
	if isProd {
		config.AllowOrigins = origins
	} else {
		config.AllowAllOrigins = true
	}
	handler := cors.New(config)
	return func(c *gin.Context) {
		handler(c)
	}
}

// Recovery panic 恢复中间件。
func Recovery(logger *zap.Logger) gin.HandlerFunc {
	return gin.CustomRecovery(func(c *gin.Context, recovered interface{}) {
		logger.Error("panic recovered",
			zap.Any("error", recovered),
			zap.String("path", c.Request.URL.Path),
		)
		response.ServerError(c, "服务器内部错误")
		c.Abort()
	})
}

// HealthPaths 健康检查探活路径：不出现在访问日志中（避免探活刷屏），
// 也不受限流拦截（容器编排探活不应被限流挡掉）。
var HealthPaths = map[string]struct{}{
	"/api/health":      {},
	"/api/health/live": {},
}

// JWTAuth 强制 JWT 认证中间件。
// sess 由装配根构建一次注入，避免每处路由注册重复构造会话模块。
func JWTAuth(sess *security.Session) gin.HandlerFunc {
	return func(c *gin.Context) {
		if !resolveClaims(c, sess) {
			response.Unauthorized(c, "Token无效或已过期，请重新登录")
			c.Abort()
			return
		}
		c.Next()
	}
}

// OptionalAuth 可选 JWT 认证：有 token 则解析填充，无则放行。
func OptionalAuth(sess *security.Session) gin.HandlerFunc {
	return func(c *gin.Context) {
		resolveClaims(c, sess)
		c.Next()
	}
}

// resolveClaims 提取 token → 校验（仅 access 类型）→ 写 context，是
// JWTAuth 与 OptionalAuth 共享的解析核心。
// 返回是否通过认证：JWTAuth 据此 401 中止，OptionalAuth 忽略结果静默放行。
// 双令牌会话（ADR-0012）：access 短生命周期不入黑名单，refresh 传入鉴权端点被 VerifyAccess 拒绝；
// 登出撤销的是 refresh（由 /logout 处理器处理），access 自然过期。
func resolveClaims(c *gin.Context, sess *security.Session) bool {
	tokenStr := sess.ExtractToken(c.GetHeader("Authorization"), authCookieValue(c, sess))
	if tokenStr == "" {
		return false
	}
	if claims, err := sess.VerifyAccess(tokenStr); err == nil {
		c.Set(string(CtxUserID), claims.UserID)
		c.Set(string(CtxAccount), claims.Account)
		c.Set(string(CtxUserRole), claims.Role)
		return true
	}
	return false
}

// authCookieValue 读取登录 Cookie（依次尝试 hrwai_token 与 recruiter_token，不存在时返回空串）。
// 兼容保持单参调用：旧签名 authCookieValue(c, name) 已收敛为按 Session CookieNames 遍历，
// 保留同名 helper 供测试或外部调用时走多 cookie 逻辑。
func authCookieValue(c *gin.Context, sess *security.Session) string {
	for _, name := range sess.CookieNames() {
		if tk, err := c.Cookie(name); err == nil && tk != "" {
			return tk
		}
	}
	return ""
}

// CapabilityRequired 能力守卫（ADR-0047 §1）：判据是 authz 能力，不是角色字面量。
// 这是逐域迁移的目标形态——端点声明「需要什么能力」，角色可达面由 authz 能力表回答。
// 必须在 JWTAuth 之后使用。
func CapabilityRequired(capability authz.Capability) gin.HandlerFunc {
	return func(c *gin.Context) {
		role, exists := c.Get(string(CtxUserRole))
		if !exists {
			response.Unauthorized(c, "Token无效或已过期，请重新登录")
			c.Abort()
			return
		}
		roleStr, _ := role.(string)
		if !authz.Has(authz.Role(roleStr), capability) {
			response.Forbidden(c, "权限不足")
			c.Abort()
			return
		}
		c.Next()
	}
}

// HasCapability 读当前请求角色的能力判定（JWTAuth 未应用或未登录时一律 false）。
// 与 CapabilityRequired 同源（都走 authz 能力表），供 handler 在**同一端点内分流**读路径
// 时使用（如题目 by-id：作者/审核者走编辑面，学员走题库池口径）。
func HasCapability(c *gin.Context, capability authz.Capability) bool {
	role, exists := c.Get(string(CtxUserRole))
	if !exists {
		return false
	}
	roleStr, _ := role.(string)
	return authz.Has(authz.Role(roleStr), capability)
}

// CurrentUserID 从 gin.Context 读取当前登录用户 ID(未登录返回 0)。
// 统一供主体系与估值模块使用,替代原 vhandler.CurrentValuationUserID。
func CurrentUserID(c *gin.Context) int {
	v, ok := c.Get(string(CtxUserID))
	if !ok {
		return 0
	}
	uid, _ := v.(int)
	return uid
}

// CurrentAccount 从 gin.Context 读取当前登录账号(未登录返回空串)。
func CurrentAccount(c *gin.Context) string {
	v, ok := c.Get(string(CtxAccount))
	if !ok {
		return ""
	}
	uid, _ := v.(string)
	return uid
}

// CurrentRole 从 gin.Context 读取当前登录用户角色(未登录返回空串)。
func CurrentRole(c *gin.Context) string {
	v, ok := c.Get(string(CtxUserRole))
	if !ok {
		return ""
	}
	uid, _ := v.(string)
	return uid
}
