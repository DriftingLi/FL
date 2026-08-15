// Package api 实现 HTTP handlers。
// 本文件：微信扫码登录/注册占位（授权信息待接入）。
package api

import (
	"context"

	"github.com/gin-gonic/gin"

	"forklift-training/internal/service"
	"forklift-training/pkg/response"
)

// WechatAuthHandler 微信登录 handler（框架占位）。
type WechatAuthHandler struct {
	svc *service.WechatAuthService
}

// NewWechatAuthHandler 创建微信登录 handler。
func NewWechatAuthHandler(svc *service.WechatAuthService) *WechatAuthHandler {
	return &WechatAuthHandler{svc: svc}
}

// RegisterWechatAuthRoutes 注册 /api/auth/wechat 蓝图（框架占位）。
func RegisterWechatAuthRoutes(rg *gin.RouterGroup, svc *service.WechatAuthService) {
	h := NewWechatAuthHandler(svc)

	g := rg.Group("/auth/wechat")

	// POST /api/auth/wechat/qrcode 获取扫码登录占位信息
	g.POST("/qrcode", h.GetQRCodeInfo)
	// POST /api/auth/wechat/login {code} 扫码登录占位（未配置授权时明确报错）
	g.POST("/login", h.LoginWithQRCode)
}

// GetQRCodeInfo 获取扫码登录占位信息 POST /api/auth/wechat/qrcode
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

// LoginWithQRCode 扫码登录占位 POST /api/auth/wechat/login（未配置授权时明确报错）
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

// wechatLoginReq 扫码登录请求体。
type wechatLoginReq struct {
	Code string `json:"code"`
}
