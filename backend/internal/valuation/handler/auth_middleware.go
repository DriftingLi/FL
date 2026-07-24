// Package handler 实现残值评估模块的 HTTP 处理器。
// 本文件：估值模块独立 JWT 认证中间件（与主体系 middleware.JWTAuth 完全隔离）。
package handler

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"

	"forklift-training/internal/cache"
	vservice "forklift-training/internal/valuation/service"
)

// 估值模块上下文键（与主体系 middleware.Ctx* 区分）
const (
	CtxValuationUserID   = "valuation_user_id"
	CtxValuationUsername = "valuation_username"
	CtxValuationRole     = "valuation_role"
)

// 估值模块独立黑名单 key 前缀（与主体系 "jwt:blacklist:" 区分）
const valuationBlacklistPrefix = "jwt:valuation:blacklist:"

// ValuationJWTAuth 估值模块独立 JWT 认证中间件。
// 使用独立 secret 签发与校验，与主体系 token 互不兼容；
// 独立黑名单 key 前缀避免与培训登出 token 冲突。
func ValuationJWTAuth(secret string) gin.HandlerFunc {
	return func(c *gin.Context) {
		tokenStr := extractValuationBearerToken(c)
		if tokenStr == "" {
			Error(c, http.StatusUnauthorized, 40100, "Token无效或已过期，请重新登录")
			c.Abort()
			return
		}
		claims, err := parseValuationToken(secret, tokenStr)
		if err != nil {
			Error(c, http.StatusUnauthorized, 40100, "Token无效或已过期，请重新登录")
			c.Abort()
			return
		}
		// 检查独立黑名单
		tokenHash := sha256.Sum256([]byte(tokenStr))
		blacklistKey := valuationBlacklistPrefix + hex.EncodeToString(tokenHash[:])
		if _, err := cache.Get(c.Request.Context(), blacklistKey); err == nil {
			Error(c, http.StatusUnauthorized, 40100, "Token无效或已过期，请重新登录")
			c.Abort()
			return
		}
		c.Set(CtxValuationUserID, claims.UserID)
		c.Set(CtxValuationUsername, claims.Username)
		c.Set(CtxValuationRole, claims.Role)
		c.Next()
	}
}

// extractValuationBearerToken 从 Authorization 头提取 Bearer token。
func extractValuationBearerToken(c *gin.Context) string {
	auth := c.GetHeader("Authorization")
	if len(auth) > 7 && auth[:7] == "Bearer " {
		return auth[7:]
	}
	return ""
}

// ValuationOptionalJWTAuth 估值模块"可选认证"中间件。
// 与 ValuationJWTAuth 的区别：无 token 或 token 无效时不拒绝请求，仅不设置 user_id；
// 有合法 token 时设置 CtxValuationUserID 等上下文，便于匿名也可用的接口（如评估提交）
// 在登录情况下记录归属。
func ValuationOptionalJWTAuth(secret string) gin.HandlerFunc {
	return func(c *gin.Context) {
		tokenStr := extractValuationBearerToken(c)
		if tokenStr == "" {
			c.Next()
			return
		}
		claims, err := parseValuationToken(secret, tokenStr)
		if err != nil {
			// 无效 token 视作匿名，不设置上下文
			c.Next()
			return
		}
		// 检查独立黑名单（已登出的 token 即使签名有效也不认）
		tokenHash := sha256.Sum256([]byte(tokenStr))
		blacklistKey := valuationBlacklistPrefix + hex.EncodeToString(tokenHash[:])
		if _, err := cache.Get(c.Request.Context(), blacklistKey); err == nil {
			c.Next()
			return
		}
		c.Set(CtxValuationUserID, claims.UserID)
		c.Set(CtxValuationUsername, claims.Username)
		c.Set(CtxValuationRole, claims.Role)
		c.Next()
	}
}

// currentValuationUserID 从 gin.Context 读取估值用户 ID（未登录返回 0）
func currentValuationUserID(c *gin.Context) int {
	v, ok := c.Get(CtxValuationUserID)
	if !ok {
		return 0
	}
	uid, _ := v.(int)
	return uid
}

// parseValuationToken 解析并校验估值 JWT。
func parseValuationToken(secret, tokenStr string) (*vservice.ValuationClaims, error) {
	claims := &vservice.ValuationClaims{}
	_, err := jwt.ParseWithClaims(tokenStr, claims, func(t *jwt.Token) (interface{}, error) {
		// 显式校验签名算法，拒绝非 HMAC 算法（防止 alg=none 攻击）
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", t.Header["alg"])
		}
		return []byte(secret), nil
	})
	if err != nil {
		return nil, err
	}
	return claims, nil
}
