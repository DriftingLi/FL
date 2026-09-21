// Package service 实现业务服务层。
// 本文件：学员口令写面的唯一动作（ADR-0062 决策 7）。
//
// 全会话吊销（identity revoke）有两族终止语义（CONTEXT.md「会话（session）」），两族的
// **失败策略不同且不许互相顶替**，故本文件只提供口令族那一条：
//
//   - 口令族（本文件的 SetNewPassword）：改密与验证码重置口令。口令一落库即不可回退，
//     吊销标记写失败**不阻断**口令，只记日志暴露缺口（尽力而为）。
//   - 注销族（不在本文件）：先写吊销标记、标记失败即整体不生效——那里没有任何已生效的
//     动作值得牺牲凭证失效（见 AuthHandler.DeleteAccount）。
//
// 共享的是动作，声明权留在调用方：SetNewPassword 把吊销的成败**如实返回**（照 Session.RevokeIdentity
// 的既有约定「失败策略由调用方决定，本动作只如实返回」），由每条入口各自记日志。
// 新增口令写面时走 SetNewPassword 即同时拿到落库与吊销，拿不到「只落哈希不吊销」的捷径：
// 学员侧两条自助入口（登录态改密 AuthService.UpdatePassword、验证码重置口令
// VerifyCodeService.ResetPasswordWithCode）都已收在此处。
// 已知例外：AdminService.ResetHrwaiUserPassword（管理员代重置）仍自己哈希+落库、不吊销，
// 属本动作之外的一条同形缺口，接它时改调 SetNewPassword，别再抄一遍哈希。
package service

import (
	"context"
	"errors"

	"forklift-training/internal/model"
)

// validatePasswordLength 口令长度规则（6-20 位，包内唯一实现；学员与招聘者的口令写面共用）。
// 入口的前置校验与 SetNewPassword 共用它：前置那一次是为了**在消费验证码之前**拒掉非法口令
// （验证码是一次性资源，不该被一个填错的口令烧掉），落库前那一次是兜底。
func validatePasswordLength(password string) error {
	if len(password) < 6 || len(password) > 20 {
		return errors.New("密码长度需为 6-20 位")
	}
	return nil
}

// SetNewPassword 落新口令（学员口令写面的唯一动作）：长度校验 → bcrypt 哈希 → 落库 →
// 全会话吊销（RevokeIdentity，身份命名空间 hrwai_user）。
//
// 返回 (revokeErr, err)：
//   - err != nil：口令**没有**落库（长度非法 / 哈希失败 / 写库失败），此时 revokeErr 恒为 nil。
//   - err == nil：口令已生效且不可回退；revokeErr 如实交出吊销标记的写入结果，
//     失败策略由调用方声明——本动作不替调用方决定。两条既有入口都属尽力而为族
//     （记 zap 日志、不因吊销失败而拒绝口令），注销族的「先写标记、失败即整体不生效」
//     刻意不在这里出现。
func (s *AuthService) SetNewPassword(ctx context.Context, userID int, password string) (revokeErr error, err error) {
	if err := validatePasswordLength(password); err != nil {
		return nil, err
	}
	hashed, err := HashPassword(password)
	if err != nil {
		return nil, err
	}
	if err := s.db.WithContext(ctx).Model(&model.HrwaiUser{}).
		Where("id = ?", userID).Update("password", hashed).Error; err != nil {
		return nil, err
	}
	// 落库后一律尝试吊销（#622 → ADR-0060 票2 → ADR-0062 票7）：只写哈希不吊销就是漏洞——
	// RotateRefresh 只看令牌与吊销标记，攻击者手上的 refresh 链最长 7 天仍可静默续登。
	return s.session.RevokeIdentity(ctx, "hrwai_user", userID), nil
}
