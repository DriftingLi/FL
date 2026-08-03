// Package api 实现 HTTP handlers。
// 本文件：登录态 Cookie 辅助函数（父域名共享登录）。
package api

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"forklift-training/internal/config"
)

// setAuthCookie 将 JWT 写入父域名 httpOnly Cookie，实现子域名间登录态共享。
func setAuthCookie(c *gin.Context, cfg *config.Config, token string) {
	http.SetCookie(c.Writer, &http.Cookie{
		Name:     cfg.AuthCookie.Name,
		Value:    token,
		Path:     "/",
		Domain:   cfg.AuthCookie.Domain,
		MaxAge:   int(cfg.JWTExpiry().Seconds()),
		HttpOnly: true,
		Secure:   cfg.AuthCookie.Secure,
		SameSite: http.SameSiteLaxMode,
	})
}

// clearAuthCookie 清除登录 Cookie（登出时调用）。
func clearAuthCookie(c *gin.Context, cfg *config.Config) {
	http.SetCookie(c.Writer, &http.Cookie{
		Name:     cfg.AuthCookie.Name,
		Value:    "",
		Path:     "/",
		Domain:   cfg.AuthCookie.Domain,
		MaxAge:   -1,
		HttpOnly: true,
		Secure:   cfg.AuthCookie.Secure,
		SameSite: http.SameSiteLaxMode,
	})
}
