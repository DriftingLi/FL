// 本文件：邮箱注册/登录（验证码，走统一验证码 engine；骨架由通道生成器提供）。
package auth

import (
	"github.com/gin-gonic/gin"

	"forklift-training/internal/captcha"
	"forklift-training/internal/security"
)

// RegisterEmailAuthRoutes 注册 /api/auth/email 蓝图（邮箱验证码注册/登录）。
func RegisterEmailAuthRoutes(rg *gin.RouterGroup, session *security.Session, codeSvc *VerifyCodeService, ch CodeChannel, captchaSvc *captcha.Service, captchaEnabled bool) {
	registerCodeChannelAuthRoutes(rg.Group("/auth/email"), session, codeSvc, ch, "email", "验证码已发送，请查收邮箱", captchaSvc, captchaEnabled)
}
