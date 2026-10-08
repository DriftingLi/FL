// 本文件：App 端微信登录（#1482）的 HTTP 面。
// 单独成文件而不是往 handler_wechat.go 里挤：那条小程序链路（/auth/wx-login）与扫码占位
// 走的是另一对凭证、另一个微信端点，混在一个注册函数里最容易让人以为可以互相顶替。
package auth

import (
	"context"
	"net/http"

	"github.com/gin-gonic/gin"

	"forklift-training/pkg/httpx"
)

// RegisterWechatAppAuthRoutes 注册 App 端微信登录路由：
// POST /api/auth/app-wx-login {code}——开放平台「移动应用」的 code 换取登录态。
// App 端的 code 不得送进 /auth/wx-login（那条只认小程序 js_code、凭证也是小程序那对）；
// 移动端按端别分流那一半在 #1482 的后续 PR（改 api/auth.uts + stores/auth.uts），本仓尚未有该出口。
func RegisterWechatAppAuthRoutes(rg *gin.RouterGroup, svc *WechatAppService) {
	h := newWechatAppHandler(svc)
	rg.POST("/auth/app-wx-login", h.AppLogin)
}

// wechatAppHandler App 端微信登录 handler。
type wechatAppHandler struct {
	svc *WechatAppService
}

func newWechatAppHandler(svc *WechatAppService) *wechatAppHandler {
	return &wechatAppHandler{svc: svc}
}

// AppLogin App 端微信登录
// @Summary App 端微信登录（开放平台移动应用）
// @Description 移动应用 code → /sns/oauth2/access_token 换 openid(+unionid) → 定位或建号 → 签发双令牌；返回体与 /auth/wx-login 同构
// @Tags 学员端-认证
// @Accept json
// @Produce json
// @Param body body object true "code" example({"code":"app_wechat_code"})
// @Success 200 {object} response.R{data=WxLoginResult} "success"
// @Failure 400 {object} response.R "参数错误 / 未配置 / code 失效"
// @Router /auth/app-wx-login [post]
func (h *wechatAppHandler) AppLogin(c *gin.Context) {
	httpx.Endpoint[wechatLoginReq, WxLoginResult]{
		Parse: func(c *gin.Context) (*wechatLoginReq, error) {
			return httpx.BindJSON[wechatLoginReq](c)
		},
		Invoke: func(ctx context.Context, req *wechatLoginReq) (*WxLoginResult, error) {
			return h.svc.AppLogin(ctx, req.Code)
		},
	}.WithSuccess(httpx.OkMsg("登录成功"), http.StatusBadRequest).Handle(c)
}
