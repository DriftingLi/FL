// 本文件：微信登录。
// - 小程序登录 POST /api/auth/wx-login：uni.login code → code2session 换 openid → 查/建用户 → 签发双令牌。
// - 扫码登录（开放平台）：占位，授权信息待接入。
// 契约见 docs/docs/reference/微信小程序登录-文档说明.md。
package auth

import (
	"context"
	"net/http"

	"github.com/gin-gonic/gin"

	"forklift-training/pkg/httpx"
)

// wechatHandler 微信登录 handler。
type wechatHandler struct {
	svc *WechatService
}

// newWechatHandler 创建微信登录 handler。
func newWechatHandler(svc *WechatService) *wechatHandler {
	return &wechatHandler{svc: svc}
}

// RegisterWechatAuthRoutes 注册微信登录路由：
// - POST /api/auth/wx-login 小程序登录（前端 api/auth.uts mpWechatLoginApi 调用契约）
// - POST /api/auth/wechat/* 扫码登录占位（授权信息待接入）
func RegisterWechatAuthRoutes(rg *gin.RouterGroup, svc *WechatService) {
	h := newWechatHandler(svc)

	// POST /api/auth/wx-login {code} 微信小程序登录
	rg.POST("/auth/wx-login", h.MiniProgramLogin)

	g := rg.Group("/auth/wechat")

	// POST /api/auth/wechat/qrcode 获取扫码登录占位信息
	g.POST("/qrcode", h.GetQRCodeInfo)
	// POST /api/auth/wechat/login {code} 扫码登录占位（未配置授权时明确报错）
	g.POST("/login", h.LoginWithQRCode)
}

// MiniProgramLogin 微信小程序登录
// @Summary 微信小程序登录
// @Description uni.login code → code2session 换 openid → 查/建用户 → 签发双令牌，平铺返回 token 等
// @Tags 学员端-认证
// @Accept json
// @Produce json
// @Param body body object true "code" example({"code":"wx_code"})
// @Success 200 {object} response.R{data=WxLoginResult} "success"
// @Failure 400 {object} response.R "参数错误"
// @Router /auth/wx-login [post]
func (h *wechatHandler) MiniProgramLogin(c *gin.Context) {
	httpx.Endpoint[wechatLoginReq, WxLoginResult]{
		Parse: func(c *gin.Context) (*wechatLoginReq, error) {
			return httpx.BindJSON[wechatLoginReq](c)
		},
		Invoke: func(ctx context.Context, req *wechatLoginReq) (*WxLoginResult, error) {
			return h.svc.MiniProgramLogin(ctx, req.Code)
		},
	}.WithSuccess(httpx.OkMsg("登录成功"), http.StatusBadRequest).Handle(c)
}

// GetQRCodeInfo 获取扫码登录二维码
// @Summary 获取微信扫码信息
// @Description 占位实现，未配置授权时返回提示
// @Tags 学员端-认证
// @Accept json
// @Produce json
// @Success 200 {object} response.R{data=WechatQRCodeInfoDTO} "success"
// @Router /auth/wechat/qrcode [post]
func (h *wechatHandler) GetQRCodeInfo(c *gin.Context) {
	httpx.Endpoint[struct{}, WechatQRCodeInfoDTO]{
		Invoke: func(ctx context.Context, _ *struct{}) (*WechatQRCodeInfoDTO, error) {
			return h.svc.QRCodeInfo(), nil
		},
	}.Handle(c)
}

// LoginWithQRCode 微信扫码登录
// @Summary 微信扫码登录（占位）
// @Description 扫码登录占位，未配置授权时明确报错
// @Tags 学员端-认证
// @Accept json
// @Produce json
// @Param body body object true "code" example({"code":"qr_code"})
// @Success 200 {object} response.R{data=LoginResult} "success"
// @Failure 400 {object} response.R "未配置"
// @Router /auth/wechat/login [post]
func (h *wechatHandler) LoginWithQRCode(c *gin.Context) {
	httpx.Endpoint[wechatLoginReq, LoginResult]{
		Parse: func(c *gin.Context) (*wechatLoginReq, error) {
			return httpx.BindJSON[wechatLoginReq](c)
		},
		Invoke: func(ctx context.Context, req *wechatLoginReq) (*LoginResult, error) {
			return h.svc.LoginWithQRCode(req.Code)
		},
		// 占位服务：错误恒非 nil，一律 400 + err.Error()（成功面暂无返回内容）。
		ErrStatus: httpx.ErrStatusAll(http.StatusBadRequest),
		Render: func(c *gin.Context, _ *wechatLoginReq, _ *LoginResult) {
		},
	}.Handle(c)
}

// wechatLoginReq 微信登录请求体（小程序、扫码与 App 端 #1482 三条链路的入参都只有一个 code）。
type wechatLoginReq struct {
	Code string `json:"code"`
}
