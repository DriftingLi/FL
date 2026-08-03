// Package api 实现 HTTP handlers。
// 本文件：邮箱注册/登录（验证码）。
package api

import (
	"context"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"forklift-training/internal/config"
	"forklift-training/internal/service"
	"forklift-training/pkg/response"
)

// RegisterEmailAuthRoutes 注册 /api/auth/email 蓝图（邮箱验证码注册/登录）。
func RegisterEmailAuthRoutes(rg *gin.RouterGroup, cfg *config.Config, db *gorm.DB, emailSvc *service.EmailAuthService) {
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
		switch service.EmailCodePurpose(req.Purpose) {
		case service.EmailCodeRegister:
			err = emailSvc.SendRegisterCode(ctx, req.Email)
		case service.EmailCodeLogin:
			err = emailSvc.SendLoginCode(ctx, req.Email)
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

	// POST /api/auth/email/register {email, code, name, company?} 验证码通过后注册并自动登录
	g.POST("/register", func(c *gin.Context) {
		var req struct {
			Email   string `json:"email"`
			Code    string `json:"code"`
			Name    string `json:"name"`
			Company string `json:"company"`
		}
		if err := c.ShouldBindJSON(&req); err != nil {
			response.BadRequest(c, "请求参数错误")
			return
		}
		ctx, cancel := context.WithTimeout(c.Request.Context(), 10*time.Second)
		defer cancel()

		result, err := emailSvc.RegisterWithCode(ctx, req.Email, req.Code, req.Name, req.Company)
		if err != nil {
			response.BadRequest(c, err.Error())
			return
		}
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

		result, err := emailSvc.LoginWithCode(ctx, req.Email, req.Code)
		if err != nil {
			response.BadRequest(c, err.Error())
			return
		}
		response.SuccessWithMsg(c, "登录成功", result)
	})
}
