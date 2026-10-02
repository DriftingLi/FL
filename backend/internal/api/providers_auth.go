package api

// provideAuth 认证与账号域：通道、验证码、微信登录、资料审核与登录 handler。
func provideAuth(c *coreSingletons, d *Deps) {
	d.AuthSvc = c.authSvc
	d.CodeSvc = c.codeSvc
	d.CaptchaSvc = c.captchaSvc
	d.EmailCh = c.emailCh
	d.PhoneCh = c.phoneCh
	d.WechatAuthSvc = c.wechatAuthSvc
	d.ReviewSvc = c.reviewSvc
}
