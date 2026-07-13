// Package api 实现 HTTP handlers。
package api

import (
	"crypto/sha256"
	"encoding/hex"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
	"gorm.io/gorm"

	"forklift-training/internal/cache"
	"forklift-training/internal/middleware"
	"forklift-training/internal/model"
	"forklift-training/internal/service"
	"forklift-training/pkg/response"
)

// AuthHandler 认证相关 handler。
type AuthHandler struct {
	authSvc *service.AuthService
}

// NewAuthHandler 创建认证 handler。
func NewAuthHandler(authSvc *service.AuthService) *AuthHandler {
	return &AuthHandler{authSvc: authSvc}
}

// Login 学员登录 POST /api/auth/login
func (h *AuthHandler) Login(c *gin.Context) {
	var req struct {
		Username string `json:"username"`
		Password string `json:"password"`
		Role     string `json:"role"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "请求参数错误")
		return
	}
	if req.Username == "" || req.Password == "" {
		response.BadRequest(c, "用户名和密码不能为空")
		return
	}
	result, err := h.authSvc.StudentLogin(req.Username, req.Password)
	if err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	response.SuccessWithMsg(c, "登录成功", result)
}

// Register 学员注册 POST /api/auth/register
// 用户名由后端用手机号自动生成，前端无需提交 username。
func (h *AuthHandler) Register(c *gin.Context) {
	var req struct {
		Phone    string `json:"phone"`
		Password string `json:"password"`
		Name     string `json:"name"`
		Email    string `json:"email"`
		Company  string `json:"company"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "请求参数错误")
		return
	}
	if req.Phone == "" || req.Password == "" || req.Name == "" {
		response.BadRequest(c, "手机号、密码和姓名不能为空")
		return
	}
	result, err := h.authSvc.StudentRegister(req.Phone, req.Password, req.Name, req.Email, req.Company)
	if err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	response.Created(c, "注册成功", result)
}

// AdminLogin 管理员登录 POST /api/auth/admin-login
func (h *AuthHandler) AdminLogin(c *gin.Context) {
	var req struct {
		Username string `json:"username"`
		Password string `json:"password"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "请求参数错误")
		return
	}
	if req.Username == "" || req.Password == "" {
		response.BadRequest(c, "用户名和密码不能为空")
		return
	}
	result, err := h.authSvc.AdminLogin(req.Username, req.Password)
	if err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	response.SuccessWithMsg(c, "管理员登录成功", result)
}

// TutorLogin 导师登录 POST /api/auth/tutor-login
func (h *AuthHandler) TutorLogin(c *gin.Context) {
	var req struct {
		Username string `json:"username"`
		Password string `json:"password"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "请求参数错误")
		return
	}
	if req.Username == "" || req.Password == "" {
		response.BadRequest(c, "用户名和密码不能为空")
		return
	}
	result, err := h.authSvc.TutorLogin(req.Username, req.Password)
	if err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	response.SuccessWithMsg(c, "导师登录成功", result)
}

// Logout 登出 POST /api/auth/logout
// 将当前 token 写入 Redis 黑名单，TTL = token 剩余有效期，使其在后续请求中被 JWTAuth 中间件拒绝。
func (h *AuthHandler) Logout(c *gin.Context) {
	// 从 Authorization header 提取 Bearer token（与 JWTAuth 中间件提取逻辑一致）
	tokenStr := ""
	if auth := c.GetHeader("Authorization"); len(auth) > 7 && auth[:7] == "Bearer " {
		tokenStr = auth[7:]
	}
	if tokenStr != "" {
		// 已通过 JWTAuth 中间件校验，这里仅解析 claims 获取过期时间，不重复校验签名
		claims := &middleware.Claims{}
		if _, _, err := jwt.NewParser().ParseUnverified(tokenStr, claims); err == nil && claims.ExpiresAt != nil {
			tokenHash := sha256.Sum256([]byte(tokenStr))
			blacklistKey := "jwt:blacklist:" + hex.EncodeToString(tokenHash[:])
			ttl := time.Until(claims.ExpiresAt.Time)
			if ttl > 0 {
				_ = cache.Set(c.Request.Context(), blacklistKey, "1", ttl)
			}
		}
	}
	response.SuccessWithMsg(c, "已登出", nil)
}

// Me 获取当前用户 GET /api/auth/me
func (h *AuthHandler) Me(c *gin.Context) {
	userID, _ := c.Get(string(middleware.CtxUserID))
	role, _ := c.Get(string(middleware.CtxUserRole))
	username, _ := c.Get(string(middleware.CtxUsername))

	uid, _ := userID.(int)
	roleStr, _ := role.(string)
	uname, _ := username.(string)

	data := map[string]interface{}{
		"user_id":  uid,
		"username": uname,
		"role":     roleStr,
		"name":     "",
		"level":    "",
	}

	db := h.authSvc.DB()
	switch roleStr {
	case "student":
		var s model.Student
		if err := db.First(&s, uid).Error; err == nil {
			data["name"] = s.Name
			data["level"] = s.Level
			data["phone"] = s.Phone
			data["email"] = s.Email
			data["company"] = s.Company
		}
	case "tutor":
		var t model.Tutor
		if err := db.First(&t, uid).Error; err == nil {
			data["name"] = t.Name
		}
	case "admin":
		var a model.Admin
		if err := db.First(&a, uid).Error; err == nil {
			data["name"] = a.Name
		}
	}
	response.Success(c, data)
}

// 占位：避免未使用导入警告
var _ = gorm.ErrRecordNotFound
