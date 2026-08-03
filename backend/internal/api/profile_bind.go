// Package api 实现 HTTP handlers。
// 本文件：个人信息页手机号/邮箱绑定修改（验证码校验 + 格式/唯一性校验）。
package api

import (
	"context"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"forklift-training/internal/config"
	"forklift-training/internal/middleware"
	"forklift-training/internal/service"
	"forklift-training/pkg/response"
)

// RegisterProfileBindRoutes 注册 /api/auth/profile 蓝图（登录后绑定/修改手机号、邮箱）。
func RegisterProfileBindRoutes(rg *gin.RouterGroup, cfg *config.Config, db *gorm.DB, authSvc *service.AuthService,
	emailSvc *service.EmailAuthService, phoneSvc *service.PhoneAuthService) {
	g := rg.Group("/auth/profile", middleware.JWTAuth(cfg))

	// POST /api/auth/profile/send-code {channel: email|phone, target}
	g.POST("/send-code", func(c *gin.Context) {
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
		userID := middleware.CurrentUserID(c)
		var err error
		switch req.Channel {
		case "email":
			err = emailSvc.SendBindCode(ctx, userID, req.Target)
		case "phone":
			err = phoneSvc.SendBindCode(ctx, userID, req.Target)
		default:
			response.BadRequest(c, "channel 必须为 email 或 phone")
			return
		}
		if err != nil {
			response.BadRequest(c, err.Error())
			return
		}
		response.SuccessWithMsg(c, "验证码已发送，请查收", nil)
	})

	// POST /api/auth/profile/email {email, code}
	g.POST("/email", func(c *gin.Context) {
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
		userID := middleware.CurrentUserID(c)
		if err := emailSvc.BindEmail(ctx, userID, req.Email, req.Code); err != nil {
			response.BadRequest(c, err.Error())
			return
		}
		response.SuccessWithMsg(c, "邮箱修改成功", nil)
	})

	// POST /api/auth/profile/phone {phone, code}
	g.POST("/phone", func(c *gin.Context) {
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
		userID := middleware.CurrentUserID(c)
		if err := phoneSvc.BindPhone(ctx, userID, req.Phone, req.Code); err != nil {
			response.BadRequest(c, err.Error())
			return
		}
		response.SuccessWithMsg(c, "手机号修改成功", nil)
	})

	// POST /api/auth/profile/password {password} 设置/修改密码（账号密码登录用）
	g.POST("/password", func(c *gin.Context) {
		var req struct {
			Password string `json:"password"`
		}
		if err := c.ShouldBindJSON(&req); err != nil {
			response.BadRequest(c, "请求参数错误")
			return
		}
		userID := middleware.CurrentUserID(c)
		if err := authSvc.UpdatePassword(userID, req.Password); err != nil {
			response.BadRequest(c, err.Error())
			return
		}
		response.SuccessWithMsg(c, "密码设置成功", nil)
	})
}
