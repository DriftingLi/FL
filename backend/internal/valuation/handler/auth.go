// Package handler 实现残值评估模块的 HTTP 处理器。
// 本文件：估值模块认证 handler（/api/valuation/auth/*）。
// 已统一到主体系 AuthService 与 security 会话模块,本 handler 仅作为前端兼容入口。
package handler

import (
	"github.com/gin-gonic/gin"

	"forklift-training/internal/middleware"
	"forklift-training/internal/security"
	vmain "forklift-training/internal/service"
	"forklift-training/pkg/response"
)

// ValuationAuthHandler 估值模块认证处理器（消费 ValuationAuth 窄接口，主体系 AuthService 直接满足）。
type ValuationAuthHandler struct {
	authSvc ValuationAuth
	sess    *security.Session
}

// NewValuationAuthHandler 构造估值认证处理器。sess 为装配根注入的唯一会话实例。
func NewValuationAuthHandler(authSvc ValuationAuth, sess *security.Session) *ValuationAuthHandler {
	return &ValuationAuthHandler{authSvc: authSvc, sess: sess}
}

// Me 处理 GET /api/valuation/auth/me（需 middleware.JWTAuth）
func (h *ValuationAuthHandler) Me(c *gin.Context) {
	uid := middleware.CurrentUserID(c)
	if uid == 0 {
		response.Unauthorized(c, "Token无效或已过期，请重新登录")
		return
	}
	user, err := h.authSvc.GetHrwaiUserByID(uid)
	if err != nil {
		response.NotFound(c, "用户不存在")
		return
	}
	response.Success(c, map[string]interface{}{
		"user_id":  user.ID,
		"uid":      vmain.FormatUID(user.UID),
		"account":  user.Account,
		"username": user.Username,
		"phone":    vmain.MaskedPhone(user.Phone),
		"email":    user.Email,
		"company":  user.Company,
		"role":     vmain.HrwaiRole,
	})
}

// Logout 处理 POST /api/valuation/auth/logout（需 middleware.JWTAuth）
// 将当前 token 写入黑名单（统一前缀 jwt:blacklist:，由会话模块实现），TTL = token 剩余有效期。
func (h *ValuationAuthHandler) Logout(c *gin.Context) {
	// 与旧行为一致：仅从 Authorization 头提取 Bearer token
	tokenStr := h.sess.ExtractToken(c.GetHeader("Authorization"), "")
	if tokenStr == "" {
		response.Success(c, nil)
		return
	}
	// 已通过 middleware.JWTAuth 校验，直接吊销（无效 token 静默忽略）
	_ = h.sess.Revoke(c.Request.Context(), tokenStr)
	response.Success(c, nil)
}
