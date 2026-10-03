// Package core 承载跨域共享的业务助手（#1445 P3 起由 internal/service 更名而来）。
// 本文件：随机登录账号生成（#1445 P2 波 3a 自 auth 域留驻并导出）。
//
// 为什么留在这里：两个调用方各在一边 —— internal/auth 的验证码注册
// （auth/code_service.go）与管理域的 AdminService 建号（波 4d 起在 internal/admin/service.go:157）。
// 把它搬进域包就会逼 internal/core import internal/auth，与「域包 → internal/core
// 单向边」构成 import cycle（同 mailer.go 的落点理由）。
package core

import (
	"crypto/rand"
	"encoding/hex"
)

// GenerateRandomAccount 生成随机登录账号（如 hr1a2b3c4d5e6f78）。
func GenerateRandomAccount() (string, error) {
	b := make([]byte, 9)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return "hr" + hex.EncodeToString(b), nil
}
