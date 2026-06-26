// Package middleware 提供 Gin 中间件：CORS、JWT 认证、请求日志、panic 恢复。
package middleware

import (
	"log/slog"
	"time"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"

	"forklift-training/internal/config"
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

// Claims JWT 声明，与原 Python 版 additional_claims 结构一致。
type Claims struct {
	UserID   int    `json:"user_id"`
	Username string `json:"username"`
	Role     string `json:"role"`
	jwt.RegisteredClaims
}

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
func CORS(origins []string) gin.HandlerFunc {
	return func(c *gin.Context) {
		cors.New(cors.Config{
			AllowOrigins:     origins,
			AllowMethods:     []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
			AllowHeaders:     []string{"Content-Type", "Authorization", "X-Silent"},
			AllowCredentials: true,
			MaxAge:           12 * time.Hour,
		})(c)
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

// JWTAuth 强制 JWT 认证中间件，对应原 Python 版 jwt_required_custom()。
func JWTAuth(cfg *config.Config) gin.HandlerFunc {
	return func(c *gin.Context) {
		tokenStr := extractToken(c)
		if tokenStr == "" {
			response.Unauthorized(c, "Token无效或已过期，请重新登录")
			c.Abort()
			return
		}

		claims, err := parseToken(cfg.JWTSecretKey, tokenStr)
		if err != nil {
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
	return func(c *gin.Context) {
		tokenStr := extractToken(c)
		if tokenStr == "" {
			c.Next()
			return
		}
		if claims, err := parseToken(cfg.JWTSecretKey, tokenStr); err == nil {
			c.Set(string(CtxUserID), claims.UserID)
			c.Set(string(CtxUsername), claims.Username)
			c.Set(string(CtxUserRole), claims.Role)
		}
		c.Next()
	}
}

// RoleRequired 角色校验中间件，对应原 Python 版 role_required()。
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

// extractToken 从 Authorization 头提取 Bearer token。
func extractToken(c *gin.Context) string {
	auth := c.GetHeader("Authorization")
	if len(auth) > 7 && auth[:7] == "Bearer " {
		return auth[7:]
	}
	return ""
}

// parseToken 解析并校验 JWT。
func parseToken(secret, tokenStr string) (*Claims, error) {
	claims := &Claims{}
	_, err := jwt.ParseWithClaims(tokenStr, claims, func(t *jwt.Token) (interface{}, error) {
		return []byte(secret), nil
	})
	if err != nil {
		return nil, err
	}
	return claims, nil
}
