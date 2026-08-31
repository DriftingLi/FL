package api

import (
	"github.com/gin-gonic/gin"

	"forklift-training/internal/security"
)

// setAuthCookie 将 JWT 写入父域名 httpOnly Cookie（子域名间共享登录）。
// 登录态 Cookie 的构造/写清除统一由 security 会话模块实现。
func setAuthCookie(c *gin.Context, sess *security.Session, token string) {
	sess.SetCookie(c.Writer, token)
}

// setRecruiterCookie 将招聘者 JWT 写入 host-only Cookie（不设 Domain）。
func setRecruiterCookie(c *gin.Context, sess *security.Session, token string) {
	sess.SetRecruiterCookie(c.Writer, token)
}
