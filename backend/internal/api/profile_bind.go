// Package api 实现 HTTP handlers。
// 本文件：个人信息页手机号/邮箱绑定修改（验证码校验 + 格式/唯一性校验，走统一验证码 engine）。
package api

import (
	"context"
	"time"

	"github.com/gin-gonic/gin"

	"forklift-training/internal/middleware"
	"forklift-training/internal/service"
	"forklift-training/pkg/response"
)

// ProfileBindHandler 个人信息绑定修改 handler。
type ProfileBindHandler struct {
	authSvc *service.AuthService
	codeSvc *service.VerifyCodeService
	emailCh service.CodeChannel
	phoneCh service.CodeChannel
}

// NewProfileBindHandler 创建个人信息绑定修改 handler。
func NewProfileBindHandler(authSvc *service.AuthService, codeSvc *service.VerifyCodeService, emailCh, phoneCh service.CodeChannel) *ProfileBindHandler {
	return &ProfileBindHandler{authSvc: authSvc, codeSvc: codeSvc, emailCh: emailCh, phoneCh: phoneCh}
}

// RegisterProfileBindRoutes 注册 /api/auth/profile 蓝图（登录后绑定/修改手机号、邮箱）
// 与 /api/auth/account 蓝图（短信验证码确认修改登录账号）。
func RegisterProfileBindRoutes(rg *gin.RouterGroup, rd RouterDeps, authSvc *service.AuthService, codeSvc *service.VerifyCodeService, emailCh, phoneCh service.CodeChannel) {
	h := NewProfileBindHandler(authSvc, codeSvc, emailCh, phoneCh)

	g := rg.Group("/auth/profile", middleware.JWTAuth(rd.Session))

	// POST /api/auth/profile/send-code {channel: email|phone, target}
	g.POST("/send-code", h.SendCode)
	// 绑定/修改端点按通道生成（一份骨架，通道作为 adapter 注入）
	g.POST("/email", h.bindEmail)
	g.POST("/phone", h.bindPhone)
	// POST /api/auth/profile/password {password} 设置/修改密码（账号密码登录用）
	g.POST("/password", h.UpdatePassword)

	// 修改登录账号（短信验证码确认）：PUT /api/auth/account、POST /api/auth/account/send-code
	acct := rg.Group("/auth/account", middleware.JWTAuth(rd.Session))
	acct.PUT("", h.UpdateAccount)
	acct.POST("/send-code", h.SendAccountChangeCode)
}

// SendCode 发送绑定验证码 POST /api/auth/profile/send-code {channel: email|phone, target}
func (h *ProfileBindHandler) SendCode(c *gin.Context) {
	var req struct {
		Channel string `json:"channel"`
		Target  string `json:"target"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "请求参数错误")
		return
	}
	ctx, cancel := context.WithTimeout(c.Request.Context(), 10*time.Second)
	defer cancel()
	// 通道映射表：send-code 的 channel 字段在此解析为 CodeChannel adapter（不再按通道写 switch）
	channels := map[string]service.CodeChannel{"email": h.emailCh, "phone": h.phoneCh}
	ch, ok := channels[req.Channel]
	if !ok {
		response.BadRequest(c, "channel 必须为 email 或 phone")
		return
	}
	if err := h.codeSvc.SendBind(ctx, ch, middleware.CurrentUserID(c), req.Target); err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	response.SuccessWithMsg(c, "验证码已发送，请查收", nil)
}

// bindEmail 绑定/修改邮箱 POST /api/auth/profile/email
func (h *ProfileBindHandler) bindEmail(c *gin.Context) {
	handleCodeChannelBind(c, h.codeSvc, h.emailCh, "email", "邮箱修改成功")
}

// bindPhone 绑定/修改手机号 POST /api/auth/profile/phone
func (h *ProfileBindHandler) bindPhone(c *gin.Context) {
	handleCodeChannelBind(c, h.codeSvc, h.phoneCh, "phone", "手机号修改成功")
}

// UpdatePassword 设置/修改密码 POST /api/auth/profile/password
func (h *ProfileBindHandler) UpdatePassword(c *gin.Context) {
	var req struct {
		Password string `json:"password"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "请求参数错误")
		return
	}
	userID := middleware.CurrentUserID(c)
	if err := h.authSvc.UpdatePassword(userID, req.Password); err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	response.SuccessWithMsg(c, "密码设置成功", nil)
}

// SendAccountChangeCode 发送修改登录账号验证码 POST /api/auth/account/send-code
// 验证码发送到当前用户已绑定手机号（复用短信通道，生产未接入时开发日志降级）。
func (h *ProfileBindHandler) SendAccountChangeCode(c *gin.Context) {
	ctx, cancel := context.WithTimeout(c.Request.Context(), 10*time.Second)
	defer cancel()
	if err := h.codeSvc.SendAccountChange(ctx, h.phoneCh, middleware.CurrentUserID(c)); err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	response.SuccessWithMsg(c, "验证码已发送，请查收", nil)
}

// UpdateAccount 修改登录账号 PUT /api/auth/account {account, code}
// 短信验证码确认 + 格式 4~20 位字母/数字/下划线 + 唯一性校验（复用 profile_bind 验证码模式）。
func (h *ProfileBindHandler) UpdateAccount(c *gin.Context) {
	var req struct {
		Account string `json:"account"`
		Code    string `json:"code"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "请求参数错误")
		return
	}
	ctx, cancel := context.WithTimeout(c.Request.Context(), 10*time.Second)
	defer cancel()
	result, err := h.codeSvc.ChangeAccount(ctx, h.phoneCh, middleware.CurrentUserID(c), req.Account, req.Code)
	if err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	// 响应携带新签发的 token（前端替换本地登录态，JWT claim 随新账号同步）
	response.SuccessWithMsg(c, "账号修改成功", result)
}

// handleCodeChannelBind 绑定/修改目标字段的公共实现（通道注入）。
func handleCodeChannelBind(c *gin.Context, codeSvc *service.VerifyCodeService, ch service.CodeChannel, targetField, successMsg string) {
	body, ok := bindBody(c)
	if !ok {
		return
	}
	ctx, cancel := context.WithTimeout(c.Request.Context(), 10*time.Second)
	defer cancel()
	userID := middleware.CurrentUserID(c)
	if err := codeSvc.Bind(ctx, ch, userID, str(body, targetField), str(body, "code")); err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	response.SuccessWithMsg(c, successMsg, nil)
}
