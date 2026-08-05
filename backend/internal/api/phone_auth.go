// Package api 实现 HTTP handlers。
// 本文件：手机号注册/登录（验证码，走统一验证码 engine）。
package api

import (
	"context"
	"time"

	"github.com/gin-gonic/gin"

	"forklift-training/internal/config"
	"forklift-training/internal/service"
	"forklift-training/pkg/response"
)

// RegisterPhoneAuthRoutes 注册 /api/auth/phone 蓝图（手机号验证码注册/登录）。
func RegisterPhoneAuthRoutes(rg *gin.RouterGroup, cfg *config.Config, codeSvc *service.VerifyCodeService, phoneCh *service.SmsChannel) {
	g := rg.Group("/auth/phone")

	// POST /api/auth/phone/send-code {phone, purpose: register|login}
	g.POST("/send-code", func(c *gin.Context) {
		var req struct {
			Phone   string `json:"phone"`
			Purpose string `json:"purpose"`
		}
		if err := c.ShouldBindJSON(&req); err != nil {
			response.BadRequest(c, "请求参数错误")
			return
		}
		ctx, cancel := context.WithTimeout(c.Request.Context(), 10*time.Second)
		defer cancel()

		var err error
		switch service.CodePurpose(req.Purpose) {
		case service.CodePurposeRegister:
			err = codeSvc.Send(ctx, phoneCh, service.CodePurposeRegister, req.Phone)
		case service.CodePurposeLogin:
			err = codeSvc.Send(ctx, phoneCh, service.CodePurposeLogin, req.Phone)
		default:
			response.BadRequest(c, "purpose 必须为 register 或 login")
			return
		}
		if err != nil {
			response.BadRequest(c, err.Error())
			return
		}
		response.SuccessWithMsg(c, "验证码已发送，请查收手机短信", nil)
	})

	// POST /api/auth/phone/register {phone, code, nickname, company?, password}
	g.POST("/register", func(c *gin.Context) {
		var req struct {
			Phone    string `json:"phone"`
			Code     string `json:"code"`
			Nickname string `json:"nickname"`
			Company  string `json:"company"`
			Password string `json:"password"`
		}
		if err := c.ShouldBindJSON(&req); err != nil {
			response.BadRequest(c, "请求参数错误")
			return
		}
		ctx, cancel := context.WithTimeout(c.Request.Context(), 10*time.Second)
		defer cancel()
		result, err := codeSvc.RegisterWithCode(ctx, phoneCh, req.Phone, req.Code, req.Nickname, req.Company, req.Password)
		if err != nil {
			response.BadRequest(c, err.Error())
			return
		}
		setAuthCookie(c, cfg, result.Token)
		response.Created(c, "注册成功", result)
	})

	// POST /api/auth/phone/login {phone, code}
	g.POST("/login", func(c *gin.Context) {
		var req struct {
			Phone string `json:"phone"`
			Code  string `json:"code"`
		}
		if err := c.ShouldBindJSON(&req); err != nil {
			response.BadRequest(c, "请求参数错误")
			return
		}
		ctx, cancel := context.WithTimeout(c.Request.Context(), 10*time.Second)
		defer cancel()
		result, err := codeSvc.LoginWithCode(ctx, phoneCh, req.Phone, req.Code)
		if err != nil {
			response.BadRequest(c, err.Error())
			return
		}
		setAuthCookie(c, cfg, result.Token)
		response.SuccessWithMsg(c, "登录成功", result)
	})
}
