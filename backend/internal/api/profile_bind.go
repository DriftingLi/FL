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

// RegisterProfileBindRoutes 注册 /api/auth/profile 蓝图（登录后绑定/修改手机号、邮箱）。
func RegisterProfileBindRoutes(rg *gin.RouterGroup, deps *Deps) {
	g := rg.Group("/auth/profile", middleware.JWTAuth(deps.Session))
	authSvc := deps.AuthSvc
	codeSvc := deps.CodeSvc

	// 通道映射表：send-code 的 channel 字段在此解析为 CodeChannel adapter（不再按通道写 switch）
	channels := map[string]service.CodeChannel{"email": deps.EmailCh, "phone": deps.PhoneCh}

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
		ch, ok := channels[req.Channel]
		if !ok {
			response.BadRequest(c, "channel 必须为 email 或 phone")
			return
		}
		if err := codeSvc.SendBind(ctx, ch, middleware.CurrentUserID(c), req.Target); err != nil {
			response.BadRequest(c, err.Error())
			return
		}
		response.SuccessWithMsg(c, "验证码已发送，请查收", nil)
	})

	// 绑定/修改端点按通道生成（一份骨架，通道作为 adapter 注入）
	registerCodeChannelBindRoutes(g, codeSvc, deps.EmailCh, "email", "邮箱修改成功")
	registerCodeChannelBindRoutes(g, codeSvc, deps.PhoneCh, "phone", "手机号修改成功")

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
