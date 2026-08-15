// Package service 实现业务服务层。
// 本文件：邮箱注册账号的 phone 占位值（sentinel）的单点定义。
// 邮箱注册时 phone 列 NOT NULL，但真实手机号未知，故写入 "email_"+sha256(email) 占位值。
// "email_" 前缀是该 sentinel 的唯一实现点，禁止在业务代码中散落字面量（PlaceholderPhonePrefix）。
package service

import "strings"

// PlaceholderPhonePrefix 邮箱注册账号的 phone 占位值前缀，标识非真实手机号。
// 散落字面量会被 placeholder_phone_gate_test.go 的静态扫描拦截。
const PlaceholderPhonePrefix = "email_"

// IsPlaceholderPhone 判断 phone 是否为邮箱注册生成的占位值（PlaceholderPhonePrefix 前缀）。
// MaskedPhone / currentUserPhone 共用，避免三处各自书写 HasPrefix("email_")。
func IsPlaceholderPhone(phone string) bool {
	return strings.HasPrefix(phone, PlaceholderPhonePrefix)
}
