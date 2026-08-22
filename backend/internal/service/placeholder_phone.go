// Package service 实现业务服务层。
// 本文件：phone 占位值（sentinel）的单点定义与判定。
// 三类占位：邮箱注册（"email_"+sha256(email)，email 列 NOT NULL 而真实手机号未知）、
// 微信自动建号（"wxp_"+openID，满足 phone 唯一约束）、注销哨兵用户（deleted__sentinel）。
// 判定统一走 IsPlaceholderPhone，禁止在业务代码中散落字面量
// （placeholder_phone_gate_test.go 静态扫描拦截裸 "email_"）。
package service

import "strings"

// PlaceholderPhonePrefix 邮箱注册账号的 phone 占位值前缀，标识非真实手机号。
const PlaceholderPhonePrefix = "email_"

// PlaceholderWechatPhonePrefix 微信自动建号的 phone 占位值前缀（openID 派生，唯一约束用）。
const PlaceholderWechatPhonePrefix = "wxp_"

// PlaceholderDeletedSentinel 注销账号哨兵用户（__deleted_user）的 phone 占位值（精确匹配）。
const PlaceholderDeletedSentinel = "deleted__sentinel"

// IsPlaceholderPhone 判断 phone 是否为占位值（非真实手机号）：三类 sentinel 的唯一判定点。
// MaskedPhone / currentUserPhone 共用；微信建号用户的 wxp_ 串与注销哨兵同样不得下发客户端
// 或当作可发短信的手机号。
func IsPlaceholderPhone(phone string) bool {
	if strings.HasPrefix(phone, PlaceholderPhonePrefix) || strings.HasPrefix(phone, PlaceholderWechatPhonePrefix) {
		return true
	}
	return phone == PlaceholderDeletedSentinel
}
