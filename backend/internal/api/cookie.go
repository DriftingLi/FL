package api

import (
	"github.com/gin-gonic/gin"

	"forklift-training/internal/config"
	"forklift-training/internal/security"
)

// setAuthCookie 将 JWT 写入父域名 httpOnly Cookie（子域名间共享登录）。
// 登录态 Cookie 的构造/写清除统一由 security 会话模块实现。
func setAuthCookie(c *gin.Context, cfg *config.Config, token string) {
	security.SessionFromConfig(cfg).SetCookie(c.Writer, token)
}
