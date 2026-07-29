// Package handler 实现残值评估模块的 HTTP 处理器。
// 本文件:保留估值模块兼容性 helper,中间件已统一改用主体系 middleware.JWTAuth/OptionalAuth。
package handler

import (
	"github.com/gin-gonic/gin"

	"forklift-training/internal/middleware"
)

// 估值模块上下文键(与主体系 middleware.Ctx* 等价,保留以兼容旧代码引用)
const (
	CtxValuationUserID   = middleware.CtxUserID
	CtxValuationUsername = middleware.CtxUsername
	CtxValuationRole     = middleware.CtxUserRole
)

// 估值模块独立黑名单 key 前缀(deprecated,统一用主体系 "jwt:blacklist:")
const valuationBlacklistPrefix = "jwt:blacklist:"

// CurrentValuationUserID 从 gin.Context 读取当前登录用户 ID(未登录返回 0)。
// deprecated: 请改用 middleware.CurrentUserID
func CurrentValuationUserID(c *gin.Context) int {
	return middleware.CurrentUserID(c)
}

// extractValuationBearerToken 从 Authorization 头提取 Bearer token。
// 与 middleware 中同名 helper 等价,保留给 Logout handler 复用。
func extractValuationBearerToken(c *gin.Context) string {
	auth := c.GetHeader("Authorization")
	if len(auth) > 7 && auth[:7] == "Bearer " {
		return auth[7:]
	}
	return ""
}
