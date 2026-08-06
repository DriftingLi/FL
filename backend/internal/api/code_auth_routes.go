// Package api 实现 HTTP handlers。
// 本文件：验证码认证路由生成器——注册/登录/发送/绑定一份骨架，通道作为 adapter 注入
// （ADR-0001 的 CodeChannel seam 的自然收尾：handler 层不再按通道复制）。
package api

import (
	"context"
	"time"

	"github.com/gin-gonic/gin"

	"forklift-training/internal/config"
	"forklift-training/internal/middleware"
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

// registerCodeChannelAuthRoutes 注册 /auth/<prefix> 蓝图（验证码注册/登录，通道注入）。
// targetField: 请求体中的目标字段名（email / phone）；sentMsg: 发送成功提示文案。
func registerCodeChannelAuthRoutes(g *gin.RouterGroup, cfg *config.Config, codeSvc *service.VerifyCodeService,
	ch service.CodeChannel, targetField, sentMsg string) {
	// POST /auth/<prefix>/send-code {targetField, purpose: register|login}
	g.POST("/send-code", func(c *gin.Context) {
		body, ok := bindBody(c)
		if !ok {
			return
		}
		ctx, cancel := context.WithTimeout(c.Request.Context(), 10*time.Second)
		defer cancel()

		var err error
		switch service.CodePurpose(str(body, "purpose")) {
		case service.CodePurposeRegister:
			err = codeSvc.Send(ctx, ch, service.CodePurposeRegister, str(body, targetField))
		case service.CodePurposeLogin:
			err = codeSvc.Send(ctx, ch, service.CodePurposeLogin, str(body, targetField))
		default:
			response.BadRequest(c, "purpose 必须为 register 或 login")
			return
		}
		if err != nil {
			response.BadRequest(c, err.Error())
			return
		}
		response.SuccessWithMsg(c, sentMsg, nil)
	})

	// POST /auth/<prefix>/register {targetField, code, nickname, company?, password} 验证码通过后注册并自动登录
	g.POST("/register", func(c *gin.Context) {
		body, ok := bindBody(c)
		if !ok {
			return
		}
		ctx, cancel := context.WithTimeout(c.Request.Context(), 10*time.Second)
		defer cancel()

		result, err := codeSvc.RegisterWithCode(ctx, ch,
			str(body, targetField), str(body, "code"), str(body, "nickname"), str(body, "company"), str(body, "password"))
		if err != nil {
			response.BadRequest(c, err.Error())
			return
		}
		setAuthCookie(c, cfg, result.Token)
		response.Created(c, "注册成功", result)
	})

	// POST /auth/<prefix>/login {targetField, code} 验证码通过后登录
	g.POST("/login", func(c *gin.Context) {
		body, ok := bindBody(c)
		if !ok {
			return
		}
		ctx, cancel := context.WithTimeout(c.Request.Context(), 10*time.Second)
		defer cancel()

		result, err := codeSvc.LoginWithCode(ctx, ch, str(body, targetField), str(body, "code"))
		if err != nil {
			response.BadRequest(c, err.Error())
			return
		}
		setAuthCookie(c, cfg, result.Token)
		response.SuccessWithMsg(c, "登录成功", result)
	})
}

// registerCodeChannelBindRoutes 注册 /auth/profile/<targetField>（登录后绑定/修改目标字段，通道注入）。
func registerCodeChannelBindRoutes(g *gin.RouterGroup, codeSvc *service.VerifyCodeService,
	ch service.CodeChannel, targetField, successMsg string) {
	g.POST("/"+targetField, func(c *gin.Context) {
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
	})
}
