// 本文件：跨域共享的身份词汇与口令原语（P2 波 3a 收口）。
// 这些符号同时被 auth 域（internal/auth）与留驻的 service 侧代码使用，所以留在这里，
// 由 auth 侧以 core.X 限定引用——反向（service → auth）会成环，见 ADR-0070。
package core

import (
	"errors"

	"golang.org/x/crypto/bcrypt"
)

// HrwaiRole 统一 HRWAI 账号角色名(替代原 "student" 和 "valuation_user")。
const HrwaiRole = "hrwai_user"

// RecruiterRole 企业招聘者角色名（第四角色，独立表 recruiter_users，邀约制）。
const RecruiterRole = "recruiter"

// TutorRole 讲师角色名。**与 HrwaiRole/RecruiterRole 同住一处**（ADR-0064 判据）：这个字符串
// 同时是 JWT 的角色 claim 与全会话吊销的命名空间键片段，此前只以字面量散在登录分派
// （今 internal/auth/service.go），吊销侧一用就得再抄一遍——同一个事实的两个住处。
// 注意与 authz.RoleTutor 不是一回事：那一层是能力角色名，这一层是凭证命名空间。
const TutorRole = "tutor"

// AdminRole 管理员角色名（#1640）。与另三个同住处、同口径：**这是凭证命名空间**，
// 不是能力角色名（authz.RoleAdmin 那一层是能力角色）—— 两者取值相同是事实，但它们是两个概念，
// 各自只有一个住处。管理员的 JWT 角色 claim 与全会话吊销的命名空间都用它。
const AdminRole = "admin"

// ErrAdminNotFound 「管理员账号不存在」这一事实的唯一载体（#1640，与 ErrRecruiterNotFound 同形）。
// 口令写面（core.ApplyAdminPassword）拿它把「账号不存在」与「查不动」分档。
var ErrAdminNotFound = errors.New("管理员不存在")

// ErrRecruiterNotFound 「招聘者账号不存在」这一事实的唯一载体（ADR-0064 决策 1/2）。
// 与吊销命名空间 RecruiterRole 同处一地，api 侧据此把它与「查不动」分档。
var ErrRecruiterNotFound = errors.New("招聘者不存在")

// MaskedPhone 隐藏占位手机号（邮箱注册 email_ / 微信建号 wxp_ / 注销哨兵 deleted__sentinel，
// IsPlaceholderPhone 单点判定），/auth/me 源头过滤不下发客户端——修复微信建号用户
// /auth/me 泄漏 wxp_ 串的问题。
func MaskedPhone(phone string) string {
	if IsPlaceholderPhone(phone) {
		return ""
	}
	return phone
}

// HashPassword 使用 bcrypt 加密密码。
func HashPassword(password string) (string, error) {
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return "", err
	}
	return string(hash), nil
}

// VerifyPassword 校验密码。
func VerifyPassword(password, hashed string) bool {
	return bcrypt.CompareHashAndPassword([]byte(hashed), []byte(password)) == nil
}
