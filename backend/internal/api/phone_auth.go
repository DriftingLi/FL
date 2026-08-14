// Package api 实现 HTTP handlers。
// 本文件：手机号注册/登录（验证码，走统一验证码 engine；骨架由通道生成器提供）。
package api

import (
	"github.com/gin-gonic/gin"

	"forklift-training/internal/service"
)

// RegisterPhoneAuthRoutes 注册 /api/auth/phone 蓝图（手机号验证码注册/登录）。
func RegisterPhoneAuthRoutes(rg *gin.RouterGroup, rd RouterDeps, codeSvc *service.VerifyCodeService, ch service.CodeChannel) {
	registerCodeChannelAuthRoutes(rg.Group("/auth/phone"), rd.Session, codeSvc, ch, "phone", "验证码已发送，请查收手机短信")
}
