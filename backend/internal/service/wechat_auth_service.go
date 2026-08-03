// Package service 实现业务服务层。
// 本文件：微信扫码登录框架占位（开放平台授权信息待接入）。
package service

import (
	"errors"

	"forklift-training/internal/config"
)

// WechatAuthService 微信扫码登录/注册占位服务。
type WechatAuthService struct {
	cfg config.WechatConfig
}

// NewWechatAuthService 构造微信服务（框架占位）。
func NewWechatAuthService(cfg config.WechatConfig) *WechatAuthService {
	return &WechatAuthService{cfg: cfg}
}

// QRCodeInfo 返回扫码登录占位信息：未配置授权时 enabled=false，前端展示占位二维码。
func (s *WechatAuthService) QRCodeInfo() map[string]any {
	if s.cfg.AppID == "" || s.cfg.AppSecret == "" {
		return map[string]any{
			"enabled": false,
			"qr_url":  "",
			"message": "微信授权暂未配置，请等待开放平台配置完成后使用",
		}
	}
	return map[string]any{
		"enabled": true,
		"qr_url":  "",
		"message": "微信扫码登录待接入（二维码生成接口占位）",
	}
}

// LoginWithQRCode 微信扫码登录占位：真实授权流程待接入。
func (s *WechatAuthService) LoginWithQRCode(code string) (*LoginResult, error) {
	return nil, errors.New("微信授权尚未配置，暂不支持扫码登录")
}
