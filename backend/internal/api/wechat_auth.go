// Package api 实现 HTTP handlers。
// 本文件：微信登录。
// - 小程序登录 POST /api/auth/wx-login：uni.login code → code2session 换 openid → 查/建用户 → 签发双令牌。
// - 扫码登录（开放平台）：占位，授权信息待接入。
// 契约见 docs/docs/reference/微信小程序登录-文档说明.md。
package api

import (
	"context"

	"github.com/gin-gonic/gin"

	"forklift-training/internal/service"
	"forklift-training/pkg/response"
)

// WechatAuthHandler 微信登录 handler。
type WechatAuthHandler struct {
	svc *service.WechatAuthService
}

// NewWechatAuthHandler 创建微信登录 handler。
func NewWechatAuthHandler(svc *service.WechatAuthService) *WechatAuthHandler {
	return &WechatAuthHandler{svc: svc}
}

// RegisterWechatAuthRoutes 注册微信登录路由：
// - POST /api/auth/wx-login 小程序登录（前端 api/auth.uts mpWechatLoginApi 调用契约）
// - POST /api/auth/wechat/* 扫码登录占位（授权信息待接入）
func RegisterWechatAuthRoutes(rg *gin.RouterGroup, svc *service.WechatAuthService) {
	h := NewWechatAuthHandler(svc)

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
// @Success 200 {object} response.R "success"
// @Failure 400 {object} response.R "参数错误"
// @Router /auth/wx-login [post]
func (h *WechatAuthHandler) MiniProgramLogin(c *gin.Context) {
	Endpoint[wechatLoginReq, service.WxLoginResult]{
		Parse: func(c *gin.Context) (*wechatLoginReq, error) {
			return bindJSON[wechatLoginReq](c)
		},
		Invoke: func(ctx context.Context, req *wechatLoginReq) (*service.WxLoginResult, error) {
			return h.svc.MiniProgramLogin(ctx, req.Code)
		},
		Render: func(c *gin.Context, _ *wechatLoginReq, resp *service.WxLoginResult, err error) {
			if err != nil {
				response.BadRequest(c, err.Error())
				return
			}
			response.SuccessWithMsg(c, "登录成功", resp)
		},
	}.Handle(c)
}

// GetQRCodeInfo 获取扫码登录二维码
// @Summary 获取微信扫码信息
// @Description 占位实现，未配置授权时返回提示
// @Tags 学员端-认证
// @Accept json
// @Produce json
// @Success 200 {object} response.R "success"
// @Router /auth/wechat/qrcode [post]
func (h *WechatAuthHandler) GetQRCodeInfo(c *gin.Context) {
	Endpoint[struct{}, map[string]any]{
		Invoke: func(ctx context.Context, _ *struct{}) (*map[string]any, error) {
			result := h.svc.QRCodeInfo()
			return &result, nil
		},
		Render: func(c *gin.Context, _ *struct{}, resp *map[string]any, _ error) {
			response.Success(c, *resp)
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
// @Success 200 {object} response.R "success"
// @Failure 400 {object} response.R "未配置"
// @Router /auth/wechat/login [post]
func (h *WechatAuthHandler) LoginWithQRCode(c *gin.Context) {
	Endpoint[wechatLoginReq, service.LoginResult]{
		Parse: func(c *gin.Context) (*wechatLoginReq, error) {
			return bindJSON[wechatLoginReq](c)
		},
		Invoke: func(ctx context.Context, req *wechatLoginReq) (*service.LoginResult, error) {
			return h.svc.LoginWithQRCode(req.Code)
		},
		Render: func(c *gin.Context, _ *wechatLoginReq, _ *service.LoginResult, err error) {
			// 占位服务 err 一定非 nil，始终 BadRequest(err.Error())；成功（err==nil）无返回。
			if err != nil {
				response.BadRequest(c, err.Error())
			}
		},
	}.Handle(c)
}

// wechatLoginReq 微信登录请求体（小程序与扫码登录共用 {code}）。
type wechatLoginReq struct {
	Code string `json:"code"`
}
