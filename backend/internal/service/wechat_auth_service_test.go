package service

import (
	"go.uber.org/zap"
	"testing"

	"forklift-training/internal/config"
)

func TestWechatQRCodeInfo_NotConfigured(t *testing.T) {
	svc := NewWechatAuthService(config.WechatConfig{}, zap.NewNop())
	info := svc.QRCodeInfo()
	if info["enabled"] != false {
		t.Errorf("未配置授权时应 enabled=false: %+v", info)
	}
}

func TestWechatLogin_NotImplemented(t *testing.T) {
	svc := NewWechatAuthService(config.WechatConfig{}, zap.NewNop())
	if _, err := svc.LoginWithQRCode("code"); err == nil {
		t.Error("未配置授权时登录应报错")
	}
}
