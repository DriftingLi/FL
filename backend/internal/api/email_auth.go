// Package api 实现 HTTP handlers。
// 本文件：邮箱注册/登录（验证码，走统一验证码 engine）。
package api

import (
	"context"
	"time"

	"github.com/gin-gonic/gin"

	"forklift-training/internal/config"
	"forklift-training/internal/service"
	"forklift-training/pkg/response"
)

// RegisterEmailAuthRoutes 注册 /api/auth/email 蓝图（邮箱验证码注册/登录）。
func RegisterEmailAuthRoutes(rg *gin.RouterGroup, cfg *config.Config, codeSvc *service.VerifyCodeService, emailCh *service.EmailChannel) {
	g := rg.Group("/auth/email")

	// POST /api/auth/email/send-code {email, purpose: register|login}
	g.POST("/send-code", func(c *gin.Context) {
		var req struct {
			Email   string `json:"email"`
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
			err = codeSvc.Send(ctx, emailCh, service.CodePurposeRegister, req.Email)
		case service.CodePurposeLogin:
			err = codeSvc.Send(ctx, emailCh, service.CodePurposeLogin, req.Email)
		default:
			response.BadRequest(c, "purpose 必须为 register 或 login")
			return
		}
		if err != nil {
			response.BadRequest(c, err.Error())
			return
		}
		response.SuccessWithMsg(c, "验证码已发送，请查收邮箱", nil)
	})

	// POST /api/auth/email/register {email, code, nickname, company?, password} 验证码通过后注册并自动登录
	g.POST("/register", func(c *gin.Context) {
		var req struct {
			Email    string `json:"email"`
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

		result, err := codeSvc.RegisterWithCode(ctx, emailCh, req.Email, req.Code, req.Nickname, req.Company, req.Password)
		if err != nil {
			response.BadRequest(c, err.Error())
			return
		}
		setAuthCookie(c, cfg, result.Token)
		response.Created(c, "注册成功", result)
	})

	// POST /api/auth/email/login {email, code} 验证码通过后登录
	g.POST("/login", func(c *gin.Context) {
		var req struct {
			Email string `json:"email"`
			Code  string `json:"code"`
		}
		if err := c.ShouldBindJSON(&req); err != nil {
			response.BadRequest(c, "请求参数错误")
			return
		}
		ctx, cancel := context.WithTimeout(c.Request.Context(), 10*time.Second)
		defer cancel()

		result, err := codeSvc.LoginWithCode(ctx, emailCh, req.Email, req.Code)
		if err != nil {
			response.BadRequest(c, err.Error())
			return
		}
		setAuthCookie(c, cfg, result.Token)
		response.SuccessWithMsg(c, "登录成功", result)
	})
}
