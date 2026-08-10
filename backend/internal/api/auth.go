// Package api 实现 HTTP handlers。
package api

import (
	"io"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"

	"forklift-training/internal/middleware"
	"forklift-training/internal/security"
	"forklift-training/internal/service"
	"forklift-training/internal/storage"
	"forklift-training/pkg/response"
)

// AuthHandler 认证相关 handler。
type AuthHandler struct {
	authSvc   *service.AuthService
	fileSvc   *service.FileService
	storage   storage.Storage
	reviewSvc *service.ProfileReviewService
	session   *security.Session
}

// NewAuthHandler 创建认证 handler。session 由装配根构建一次注入。
func NewAuthHandler(sess *security.Session, authSvc *service.AuthService, fileSvc *service.FileService, st storage.Storage, reviewSvc *service.ProfileReviewService, logger *zap.Logger) *AuthHandler {
	return &AuthHandler{
		authSvc: authSvc, fileSvc: fileSvc, storage: st, reviewSvc: reviewSvc,
		session: sess,
	}
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
	result, err := h.authSvc.HrwaiLogin(req.Username, req.Password)
	if err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	setAuthCookie(c, h.session, result.Token)
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
	result, err := h.authSvc.HrwaiRegister(req.Phone, req.Password, req.Name, req.Email, req.Company)
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
	setAuthCookie(c, h.session, result.Token)
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
	setAuthCookie(c, h.session, result.Token)
	response.SuccessWithMsg(c, "导师登录成功", result)
}

// Logout 登出 POST /api/auth/logout
// 将当前 token 写入 Redis 黑名单，TTL = token 剩余有效期，使其在后续请求中被 JWTAuth 中间件拒绝。
func (h *AuthHandler) Logout(c *gin.Context) {
	// 与 JWTAuth 中间件同一套提取逻辑（session 模块统一实现）：Bearer 头优先，其次 Cookie
	tokenStr := h.session.ExtractToken(c.GetHeader("Authorization"), authCookieFromReq(c, h.session))
	if tokenStr != "" {
		// 已通过 JWTAuth 中间件校验，这里仅吊销（无效 token 静默忽略）
		_ = h.session.Revoke(c.Request.Context(), tokenStr)
	}
	h.session.ClearCookie(c.Writer)
	response.SuccessWithMsg(c, "已登出", nil)
}

// authCookieFromReq 读取父域名登录 Cookie。
func authCookieFromReq(c *gin.Context, sess *security.Session) string {
	tk, err := c.Cookie(sess.CookieName())
	if err != nil {
		return ""
	}
	return tk
}

// Me 获取当前用户 GET /api/auth/me
// 资料组装收编在 AuthService.GetProfile（响应形状由契约测试锁定）。
func (h *AuthHandler) Me(c *gin.Context) {
	userID, _ := c.Get(string(middleware.CtxUserID))
	role, _ := c.Get(string(middleware.CtxUserRole))
	username, _ := c.Get(string(middleware.CtxUsername))

	uid, _ := userID.(int)
	roleStr, _ := role.(string)
	uname, _ := username.(string)

	response.Success(c, h.authSvc.GetProfile(uid, roleStr, uname))
}

// UpdateProfile 提交个人资料（昵称）修改审核 POST /api/auth/profile
func (h *AuthHandler) UpdateProfile(c *gin.Context) {
	userID, _ := c.Get(string(middleware.CtxUserID))
	uid, _ := userID.(int)
	if uid <= 0 {
		response.Unauthorized(c, "请先登录")
		return
	}
	var req struct {
		Nickname string `json:"nickname"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "请求参数错误")
		return
	}
	reqDTO, err := h.reviewSvc.CreateRequest(uid, service.ProfileFieldNickname, req.Nickname)
	if err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	response.SuccessWithMsg(c, "昵称修改已提交，审核通过后生效", reqDTO)
}

// UploadAvatar 上传头像并提交审核 POST /api/auth/avatar（multipart，图片自动压缩为 WebP 后存入 local/R2）
func (h *AuthHandler) UploadAvatar(c *gin.Context) {
	userID, _ := c.Get(string(middleware.CtxUserID))
	uid, _ := userID.(int)
	if uid <= 0 {
		response.Unauthorized(c, "请先登录")
		return
	}
	file, err := c.FormFile("file")
	if err != nil {
		response.BadRequest(c, "未找到上传文件")
		return
	}
	if ok, msg := h.fileSvc.ValidateImageFile(file.Filename, file.Size); !ok {
		response.BadRequest(c, msg)
		return
	}
	src, err := file.Open()
	if err != nil {
		response.ServerError(c, "文件上传失败")
		return
	}
	defer src.Close()
	content, err := io.ReadAll(src)
	if err != nil {
		response.ServerError(c, "文件上传失败")
		return
	}

	url, err := h.fileSvc.SaveFile(content, file.Filename, "avatars")
	if err != nil {
		response.ServerError(c, "头像保存失败: "+err.Error())
		return
	}
	reqDTO, err := h.reviewSvc.CreateRequest(uid, service.ProfileFieldAvatar, url)
	if err != nil {
		// 提交审核失败时清理已上传的文件（尽力而为）
		_ = h.storage.Delete(c.Request.Context(), url)
		response.BadRequest(c, "头像提交失败: "+err.Error())
		return
	}
	response.SuccessWithMsg(c, "头像修改已提交，审核通过后生效", reqDTO)
}
