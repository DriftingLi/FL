// Package handler 实现残值评估模块的 HTTP 处理器。
// 本文件：估值模块认证 handler（/api/valuation/auth/*）。
// 已统一到主体系 AuthService 与 security 会话模块,本 handler 仅作为前端兼容入口。
package handler

import (
	"github.com/gin-gonic/gin"

	"forklift-training/internal/middleware"
	vmain "forklift-training/internal/service"
	"forklift-training/pkg/response"
)

// ValuationAuthHandler 估值模块认证处理器（消费 ValuationAuth 窄接口，主体系 AuthService 直接满足）。
type ValuationAuthHandler struct {
	authSvc ValuationAuth
}

// NewValuationAuthHandler 构造估值认证处理器。
func NewValuationAuthHandler(authSvc ValuationAuth) *ValuationAuthHandler {
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
		response.BadRequest(c, "请求参数错误")
		return
	}
	if req.Account == "" || req.Password == "" {
		response.BadRequest(c, "用户名和密码不能为空")
		return
	}
	result, err := h.authSvc.HrwaiLogin(req.Account, req.Password)
	if err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	response.Success(c, result)
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
		response.BadRequest(c, "请求参数错误")
		return
	}
	if req.Phone == "" || req.Password == "" || req.Name == "" {
		response.BadRequest(c, "手机号、密码和姓名不能为空")
		return
	}
	result, err := h.authSvc.HrwaiRegister(req.Phone, req.Password, req.Name, req.Email, req.Company)
	if err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	response.Success(c, result)
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
		"username": user.Username,
		"name":     user.Name,
		"phone":    user.Phone,
		"email":    user.Email,
		"company":  user.Company,
		"role":     vmain.HrwaiRole,
	})
}

// Logout 处理 POST /api/valuation/auth/logout（需 middleware.JWTAuth）
// 将当前 token 写入黑名单（统一前缀 jwt:blacklist:，由会话模块实现），TTL = token 剩余有效期。
func (h *ValuationAuthHandler) Logout(c *gin.Context) {
	// 与旧行为一致：仅从 Authorization 头提取 Bearer token
	tokenStr := h.authSvc.ExtractToken(c.GetHeader("Authorization"), "")
	if tokenStr == "" {
		response.Success(c, nil)
		return
	}
	// 已通过 middleware.JWTAuth 校验，直接吊销（无效 token 静默忽略）
	_ = h.authSvc.RevokeToken(c.Request.Context(), tokenStr)
	response.Success(c, nil)
}
