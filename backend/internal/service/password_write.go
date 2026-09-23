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
// 新增口令写面时走 SetNewPassword / applyNewPassword 即同时拿到落库与吊销，拿不到
// 「只落哈希不吊销」的捷径：学员侧两条自助入口（登录态改密 AuthService.UpdatePassword、
// 验证码重置口令 VerifyCodeService.ResetPasswordWithCode）与管理员侧一条代重置
// （AdminService.ResetHrwaiUserPassword，ADR-0064 决策 4 接进来）都收在此处。
package service

import (
	"context"
	"errors"

	"gorm.io/gorm"

	"forklift-training/internal/model"
	"forklift-training/internal/security"
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

// passwordSubject 「落新口令」动作的主体参数：更新哪张表的哪一列、吊销时落在哪个命名空间、
// 以及**该主体不存在时交出哪个哨兵**。role 与 JWT 的角色 claim 同源（同一身份的两个名字会
// 直接导致标记「写得进、读不出」）。
// 该类型**不导出、只在包内以两个常量实例出现**，故 key 不是可注入面：新增主体请在
// 本文件加实例，不要从 caller 收列名。
type passwordSubject struct {
	dest     interface{}
	key      string
	role     string
	notFound error
}

var (
	hrwaiPasswordSubject = passwordSubject{dest: &model.HrwaiUser{}, key: "id", role: HrwaiRole, notFound: ErrHrwaiUserNotFound}
	tutorPasswordSubject = passwordSubject{dest: &model.Tutor{}, key: "tutor_id", role: TutorRole, notFound: ErrTutorNotFound}
)

// ErrHrwaiUserNotFound / ErrTutorNotFound 是这两类账号**真不存在**这一件事的唯一载体
// （ADR-0064 决策 1/2：一个事实一个哨兵，api 侧据此才能把它与「查不动」分档）。
var (
	ErrHrwaiUserNotFound = errors.New("用户不存在")
	ErrTutorNotFound     = errors.New("讲师不存在")
)

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
	return applyNewPassword(ctx, s.db, s.session, hrwaiPasswordSubject, userID, password)
}

// applyNewPassword 是「落新口令」这一动作的实现体。四个入口共用（ADR-0064 决策 4 把它从
// 「学员专属」扩成「按主体参数化」）：学员自助改密与验证码重置（经 SetNewPassword）、
// 管理员代重置学员口令、管理员代重置讲师口令。
//
// 之所以是包内函数而不是某个服务的方法：它唯一的两个依赖（db、session）由 caller 各自持有，
// 做成方法就会逼 AdminService 依赖 AuthService —— 那是两个服务之间的横向耦合，
// 而这里要的只是同一条动作。subject 承载的正是「同一条动作、不同主体」这一维。
func applyNewPassword(ctx context.Context, db *gorm.DB, session *security.Session,
	subject passwordSubject, id int, password string) (revokeErr error, err error) {
	if err := validatePasswordLength(password); err != nil {
		return nil, err
	}
	hashed, err := HashPassword(password)
	if err != nil {
		return nil, err
	}
	res := db.WithContext(ctx).Model(subject.dest).Where(subject.key+" = ?", id).Update("password", hashed)
	if res.Error != nil {
		return nil, res.Error
	}
	if res.RowsAffected == 0 {
		// 一行都没改动 ⇒ 这个主体不存在。此前它静默返回成功（讲师侧旧实现有一句 First 兜着，
		// 抽成共用动作后丢了），于是「代重置了一个不存在的账号」在界面上是成功的。
		// bcrypt 每次加盐，故同一口令重复落库也必然产生 1 行变更，不会误判成「不存在」。
		return nil, subject.notFound
	}
	// 落库后一律尝试吊销（#622 → ADR-0060 票2 → ADR-0062 票7 → ADR-0064 决策 4）：
	// 只写哈希不吊销就是漏洞——RotateRefresh 只看令牌与吊销标记，攻击者手上的 refresh 链
	// 最长 7 天仍可静默续登。代重置同判：自救动作由谁发起不改变「旧链该死」。
	return session.RevokeIdentity(ctx, subject.role, id), nil
}
