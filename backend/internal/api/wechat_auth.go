// Package api 实现 HTTP handlers。
// 本文件：微信扫码登录/注册占位（授权信息待接入）。
package api

import (
	"github.com/gin-gonic/gin"

	"forklift-training/pkg/response"
)

// RegisterWechatAuthRoutes 注册 /api/auth/wechat 蓝图（框架占位）。
func RegisterWechatAuthRoutes(rg *gin.RouterGroup, deps *Deps) {
	g := rg.Group("/auth/wechat")
	wechatSvc := deps.WechatAuthSvc

	// POST /api/auth/wechat/qrcode 获取扫码登录占位信息
	g.POST("/qrcode", func(c *gin.Context) {
		response.Success(c, wechatSvc.QRCodeInfo())
	})

	// POST /api/auth/wechat/login {code} 扫码登录占位（未配置授权时明确报错）
	g.POST("/login", func(c *gin.Context) {
		var req struct {
			Code string `json:"code"`
		}
		if err := c.ShouldBindJSON(&req); err != nil {
			response.BadRequest(c, "请求参数错误")
			return
		}
		_, err := wechatSvc.LoginWithQRCode(req.Code)
		response.BadRequest(c, err.Error())
	})
}
