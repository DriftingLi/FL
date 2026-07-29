// Package handler 实现残值评估模块的 HTTP 处理器。
// 本文件：估值模块认证 handler（/api/valuation/auth/*）。
// 已统一到主体系 AuthService,本 handler 仅作为前端兼容入口。
package handler

import (
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"

	"forklift-training/internal/cache"
	"forklift-training/internal/middleware"
	vservice "forklift-training/internal/valuation/service"
)

// ValuationAuthHandler 估值模块认证处理器(薄包装,内部代理到主体系 AuthService)。
type ValuationAuthHandler struct {
	authSvc *vservice.ValuationAuthService
}

// NewValuationAuthHandler 构造估值认证处理器。
func NewValuationAuthHandler(authSvc *vservice.ValuationAuthService) *ValuationAuthHandler {
	return &ValuationAuthHandler{authSvc: authSvc}
}

// loginRequest 登录请求体。
type loginRequest struct {
	Account  string `json:"account"` // 用户名或手机号
	Password string `json:"password"`
}

// Login 处理 POST /api/valuation/auth/login
func (h *ValuationAuthHandler) Login(c *gin.Context) {
	var req loginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		Error(c, http.StatusBadRequest, CodeBadRequest, "请求参数错误")
		return
	}
	if req.Account == "" || req.Password == "" {
		Error(c, http.StatusBadRequest, CodeBadRequest, "用户名和密码不能为空")
		return
	}
	result, err := h.authSvc.Login(req.Account, req.Password)
	if err != nil {
		Fail(c, CodeBadRequest, err.Error())
		return
	}
	OK(c, result)
}

// registerRequest 注册请求体。
type registerRequest struct {
	Phone    string `json:"phone"`
	Password string `json:"password"`
	Name     string `json:"name"`
	Email    string `json:"email"`
	Company  string `json:"company"`
}

// Register 处理 POST /api/valuation/auth/register
// username 由后端用手机号自动生成，前端无需提交 username。
func (h *ValuationAuthHandler) Register(c *gin.Context) {
	var req registerRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		Error(c, http.StatusBadRequest, CodeBadRequest, "请求参数错误")
		return
	}
	if req.Phone == "" || req.Password == "" || req.Name == "" {
		Error(c, http.StatusBadRequest, CodeBadRequest, "手机号、密码和姓名不能为空")
		return
	}
	result, err := h.authSvc.Register(req.Phone, req.Password, req.Name, req.Email, req.Company)
	if err != nil {
		Fail(c, CodeBadRequest, err.Error())
		return
	}
	OK(c, result)
}

// Me 处理 GET /api/valuation/auth/me（需 middleware.JWTAuth）
func (h *ValuationAuthHandler) Me(c *gin.Context) {
	uid := middleware.CurrentUserID(c)
	if uid == 0 {
		Error(c, http.StatusUnauthorized, 40100, "Token无效或已过期，请重新登录")
		return
	}
	user, err := h.authSvc.GetByID(uid)
	if err != nil {
		Error(c, http.StatusNotFound, CodeNotFound, "用户不存在")
		return
	}
	OK(c, map[string]interface{}{
		"user_id":  user.ID,
		"username": user.Username,
		"name":     user.Name,
		"phone":    user.Phone,
		"email":    user.Email,
		"company":  user.Company,
		"role":     vservice.ValuationRole,
	})
}

// Logout 处理 POST /api/valuation/auth/logout（需 middleware.JWTAuth）
// 将当前 token 写入 Redis 黑名单(统一前缀 jwt:blacklist:),TTL = token 剩余有效期。
func (h *ValuationAuthHandler) Logout(c *gin.Context) {
	tokenStr := extractValuationBearerToken(c)
	if tokenStr == "" {
		OK(c, nil)
		return
	}
	// 已通过 middleware.JWTAuth 校验,这里用主体系 Claims 解析过期时间
	claims := &middleware.Claims{}
	if _, _, err := jwt.NewParser().ParseUnverified(tokenStr, claims); err == nil && claims.ExpiresAt != nil {
		tokenHash := sha256.Sum256([]byte(tokenStr))
		blacklistKey := valuationBlacklistPrefix + hex.EncodeToString(tokenHash[:])
		ttl := time.Until(claims.ExpiresAt.Time)
		if ttl > 0 {
			_ = cache.Set(c.Request.Context(), blacklistKey, "1", ttl)
		}
	}
	OK(c, nil)
}
