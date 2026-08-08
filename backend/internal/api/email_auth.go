// Package api 实现 HTTP handlers。
// 本文件：邮箱注册/登录（验证码，走统一验证码 engine；骨架由通道生成器提供）。
package api

import (
	"github.com/gin-gonic/gin"
)

// RegisterEmailAuthRoutes 注册 /api/auth/email 蓝图（邮箱验证码注册/登录）。
func RegisterEmailAuthRoutes(rg *gin.RouterGroup, deps *Deps) {
	registerCodeChannelAuthRoutes(rg.Group("/auth/email"), deps.Cfg, deps.CodeSvc, deps.EmailCh, "email", "验证码已发送，请查收邮箱")
}
