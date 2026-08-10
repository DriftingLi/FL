// Package api 实现 HTTP handlers。
// 本文件：手机号注册/登录（验证码，走统一验证码 engine；骨架由通道生成器提供）。
package api

import (
	"github.com/gin-gonic/gin"
)

// RegisterPhoneAuthRoutes 注册 /api/auth/phone 蓝图（手机号验证码注册/登录）。
func RegisterPhoneAuthRoutes(rg *gin.RouterGroup, deps *Deps) {
	registerCodeChannelAuthRoutes(rg.Group("/auth/phone"), deps.Session, deps.CodeSvc, deps.PhoneCh, "phone", "验证码已发送，请查收手机短信")
}
