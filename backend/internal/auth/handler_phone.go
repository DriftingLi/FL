// 本文件：手机号注册/登录（验证码，走统一验证码 engine；骨架由通道生成器提供）。
package auth

import (
	"github.com/gin-gonic/gin"

	"forklift-training/internal/captcha"
	"forklift-training/internal/security"
)

// RegisterPhoneAuthRoutes 注册 /api/auth/phone 蓝图（手机号验证码注册/登录）。
func RegisterPhoneAuthRoutes(rg *gin.RouterGroup, session *security.Session, codeSvc *VerifyCodeService, ch CodeChannel, captchaSvc *captcha.Service, captchaEnabled bool) {
	registerCodeChannelAuthRoutes(rg.Group("/auth/phone"), session, codeSvc, ch, "phone", "验证码已发送，请查收手机短信", captchaSvc, captchaEnabled)
}
