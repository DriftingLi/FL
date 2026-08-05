// Package middleware 提供 Gin 中间件：CORS、JWT 认证、请求日志、panic 恢复。
package middleware

import (
	"log/slog"
	"time"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"forklift-training/internal/config"
	"forklift-training/internal/security"
	"forklift-training/pkg/response"
)

// ContextKey 是 context 中存储用户信息的键。
type ContextKey string

const (
	// CtxUserID 用户ID
	CtxUserID ContextKey = "user_id"
	// CtxUsername 用户名
	CtxUsername ContextKey = "username"
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

// Logger 请求日志中间件。
func Logger() gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		c.Next()
		slog.Info("request",
			"method", c.Request.Method,
			"path", c.Request.URL.Path,
			"status", c.Writer.Status(),
			"duration_ms", time.Since(start).Milliseconds(),
			"ip", c.ClientIP(),
		)
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
func Recovery() gin.HandlerFunc {
	return gin.CustomRecovery(func(c *gin.Context, recovered interface{}) {
		slog.Error("panic recovered",
			"error", recovered,
			"path", c.Request.URL.Path,
		)
		response.ServerError(c, "服务器内部错误")
		c.Abort()
	})
}

// JWTAuth 强制 JWT 认证中间件。
func JWTAuth(cfg *config.Config) gin.HandlerFunc {
	sess := security.SessionFromConfig(cfg)
	return func(c *gin.Context) {
		tokenStr := sess.ExtractToken(c.GetHeader("Authorization"), authCookieValue(c, cfg.AuthCookie.Name))
		if tokenStr == "" {
			response.Unauthorized(c, "Token无效或已过期，请重新登录")
			c.Abort()
			return
		}

		claims, err := sess.Verify(tokenStr)
		if err != nil {
			response.Unauthorized(c, "Token无效或已过期，请重新登录")
			c.Abort()
			return
		}

		// 检查 token 黑名单（已登出 token 会被加入黑名单直到自然过期）
		if revoked, _ := sess.IsRevoked(c.Request.Context(), tokenStr); revoked {
			response.Unauthorized(c, "Token无效或已过期，请重新登录")
			c.Abort()
			return
		}

		c.Set(string(CtxUserID), claims.UserID)
		c.Set(string(CtxUsername), claims.Username)
		c.Set(string(CtxUserRole), claims.Role)
		c.Next()
	}
}

// OptionalAuth 可选 JWT 认证：有 token 则解析填充，无则放行。
func OptionalAuth(cfg *config.Config) gin.HandlerFunc {
	sess := security.SessionFromConfig(cfg)
	return func(c *gin.Context) {
		tokenStr := sess.ExtractToken(c.GetHeader("Authorization"), authCookieValue(c, cfg.AuthCookie.Name))
		if tokenStr == "" {
			c.Next()
			return
		}
		if claims, err := sess.Verify(tokenStr); err == nil {
			if revoked, _ := sess.IsRevoked(c.Request.Context(), tokenStr); !revoked {
				c.Set(string(CtxUserID), claims.UserID)
				c.Set(string(CtxUsername), claims.Username)
				c.Set(string(CtxUserRole), claims.Role)
			}
		}
		c.Next()
	}
}

// authCookieValue 读取父域名登录 Cookie（不存在时返回空串）。
func authCookieValue(c *gin.Context, cookieName string) string {
	tk, err := c.Cookie(cookieName)
	if err != nil {
		return ""
	}
	return tk
}

// RoleRequired 角色校验中间件。
// 必须在 JWTAuth 之后使用。
func RoleRequired(roles ...string) gin.HandlerFunc {
	allowed := make(map[string]struct{}, len(roles))
	for _, r := range roles {
		allowed[r] = struct{}{}
	}
	return func(c *gin.Context) {
		role, exists := c.Get(string(CtxUserRole))
		if !exists {
			response.Unauthorized(c, "Token无效或已过期，请重新登录")
			c.Abort()
			return
		}
		roleStr, _ := role.(string)
		if _, ok := allowed[roleStr]; !ok {
			response.Forbidden(c, "权限不足")
			c.Abort()
			return
		}
		c.Next()
	}
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

// CurrentUsername 从 gin.Context 读取当前登录用户名(未登录返回空串)。
func CurrentUsername(c *gin.Context) string {
	v, ok := c.Get(string(CtxUsername))
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
