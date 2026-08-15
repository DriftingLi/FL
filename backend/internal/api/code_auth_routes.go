// Package api 实现 HTTP handlers。
// 本文件：验证码认证路由生成器——注册/登录/发送一份骨架，通道作为 adapter 注入
// （ADR-0001 的 CodeChannel seam 的自然收尾：handler 层不再按通道复制）。
package api

import (
	"context"
	"time"

	"github.com/gin-gonic/gin"

	"forklift-training/internal/captcha"
	"forklift-training/internal/security"
	"forklift-training/internal/service"
	"forklift-training/pkg/response"
)

// bindBody 绑定请求体为 map；targetField 是通道相关的动态字段名（email / phone），
// 由调用方传入，骨架不感知具体通道。
func bindBody(c *gin.Context) (map[string]interface{}, bool) {
	var body map[string]interface{}
	if err := c.ShouldBindJSON(&body); err != nil {
		response.BadRequest(c, "请求参数错误")
		return nil, false
	}
	return body, true
}

// str 从 map 取字符串字段。
func str(body map[string]interface{}, key string) string {
	v, _ := body[key].(string)
	return v
}

// CodeChannelAuthHandler 验证码注册/登录 handler（通道注入，一份骨架两通道共用）。
type CodeChannelAuthHandler struct {
	sess           *security.Session
	codeSvc        *service.VerifyCodeService
	ch             service.CodeChannel
	targetField    string // 请求体中的目标字段名（email / phone）
	sentMsg        string // 发送成功提示文案
	captchaSvc     *captcha.Service
	captchaEnabled bool
}

// NewCodeChannelAuthHandler 创建验证码注册/登录 handler。
func NewCodeChannelAuthHandler(sess *security.Session, codeSvc *service.VerifyCodeService, ch service.CodeChannel, targetField, sentMsg string, captchaSvc *captcha.Service, captchaEnabled bool) *CodeChannelAuthHandler {
	return &CodeChannelAuthHandler{sess: sess, codeSvc: codeSvc, ch: ch, targetField: targetField, sentMsg: sentMsg, captchaSvc: captchaSvc, captchaEnabled: captchaEnabled}
}

// registerCodeChannelAuthRoutes 注册 /auth/<prefix> 蓝图（验证码注册/登录，通道注入）。
func registerCodeChannelAuthRoutes(g *gin.RouterGroup, sess *security.Session, codeSvc *service.VerifyCodeService,
	ch service.CodeChannel, targetField, sentMsg string, captchaSvc *captcha.Service, captchaEnabled bool) {
	h := NewCodeChannelAuthHandler(sess, codeSvc, ch, targetField, sentMsg, captchaSvc, captchaEnabled)

	// POST /auth/<prefix>/send-code {targetField, purpose: register|login}
	g.POST("/send-code", h.SendCode)
	// POST /auth/<prefix>/register {targetField, code, nickname, company?, password} 验证码通过后注册并自动登录
	g.POST("/register", h.Register)
	// POST /auth/<prefix>/login {targetField, code} 验证码通过后登录
	g.POST("/login", h.Login)
	// POST /auth/<prefix>/reset-password {targetField, code, password} 忘记密码：验证码通过后重置密码
	g.POST("/reset-password", h.ResetPassword)
}

// SendCode 发送验证码 POST /auth/<prefix>/send-code
// body: {targetField, purpose, captcha_id, captcha_value}；开启人机验证时先校验图形验证码。
func (h *CodeChannelAuthHandler) SendCode(c *gin.Context) {
	body, ok := bindBody(c)
	if !ok {
		return
	}
	ctx, cancel := context.WithTimeout(c.Request.Context(), 10*time.Second)
	defer cancel()

	if h.captchaEnabled {
		if !h.captchaSvc.Verify(ctx, str(body, "captcha_id"), str(body, "captcha_value")) {
			response.BadRequest(c, "图形验证码错误或已过期")
			return
		}
	}

	var err error
	switch service.CodePurpose(str(body, "purpose")) {
	case service.CodePurposeRegister:
		err = h.codeSvc.Send(ctx, h.ch, service.CodePurposeRegister, str(body, h.targetField))
	case service.CodePurposeLogin:
		err = h.codeSvc.Send(ctx, h.ch, service.CodePurposeLogin, str(body, h.targetField))
	case service.CodePurposeResetPassword:
		err = h.codeSvc.Send(ctx, h.ch, service.CodePurposeResetPassword, str(body, h.targetField))
	default:
		response.BadRequest(c, "purpose 必须为 register、login 或 reset_password")
		return
	}
	if err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	response.SuccessWithMsg(c, h.sentMsg, nil)
}

// Register 验证码通过后注册并自动登录 POST /auth/<prefix>/register
func (h *CodeChannelAuthHandler) Register(c *gin.Context) {
	body, ok := bindBody(c)
	if !ok {
		return
	}
	ctx, cancel := context.WithTimeout(c.Request.Context(), 10*time.Second)
	defer cancel()

	result, err := h.codeSvc.RegisterWithCode(ctx, h.ch,
		str(body, h.targetField), str(body, "code"), str(body, "nickname"), str(body, "company"), str(body, "password"))
	if err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	setAuthCookie(c, h.sess, result.Token)
	response.Created(c, "注册成功", result)
}

// Login 验证码通过后登录 POST /auth/<prefix>/login
func (h *CodeChannelAuthHandler) Login(c *gin.Context) {
	body, ok := bindBody(c)
	if !ok {
		return
	}
	ctx, cancel := context.WithTimeout(c.Request.Context(), 10*time.Second)
	defer cancel()

	result, err := h.codeSvc.LoginWithCode(ctx, h.ch, str(body, h.targetField), str(body, "code"))
	if err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	setAuthCookie(c, h.sess, result.Token)
	response.SuccessWithMsg(c, "登录成功", result)
}

// ResetPassword 忘记密码：验证码校验通过后重置密码（不自动登录）。
func (h *CodeChannelAuthHandler) ResetPassword(c *gin.Context) {
	body, ok := bindBody(c)
	if !ok {
		return
	}
	ctx, cancel := context.WithTimeout(c.Request.Context(), 10*time.Second)
	defer cancel()

	if err := h.codeSvc.ResetPasswordWithCode(ctx, h.ch, str(body, h.targetField), str(body, "code"), str(body, "password")); err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	response.SuccessWithMsg(c, "密码已重置，请使用新密码登录", nil)
}
