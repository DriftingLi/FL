// 处置动作的后果集必须齐全（ADR-0064 决策 4；CONTEXT.md「会话（session）」全会话吊销族）。
//
// seam：AdminService 与两条自助入口共用同一个 Session 单例。吊销标记**只有 RotateRefresh
// 的读侧会查**（ValidateRefresh 不查），故断言一律打在真轮换链路上。
//
// 修复前的形状：吊销 caller 只有 4 处，hrwai_user 命名空间覆盖「注销」（auth.go:334）与
// 「口令族」（password_write.go:61），**没有「被禁用」这一元**；招聘者侧
// ToggleRecruiterStatus 禁用了就吊销。于是 `issueLogin`（auth_service.go:211）会拒禁用账号
// **重新登录**，却放过他手上已有的会话链 ⇒ 精确的不对称是「禁用挡住进来，不挡住留下」。
//
// 同形缺口第二条：AdminService.ResetHrwaiUserPassword 仍自行哈希 + 落库、零吊销，是
// 「落新口令」唯一动作之外的一条捷径；长度规则也只住在 handler（admin.go:543）。
//
// 失败策略沿用既有口径：禁用与代重置都属「已生效动作之后的补救」⇒ 尽力而为，标记写失败
// 不回退处置本身（与注销族「先写标记、失败即整体不生效」有意不同）。
package api

import (
	"context"
	"errors"
	"testing"
	"time"

	"go.uber.org/zap"

	"gorm.io/gorm"

	"forklift-training/internal/model"
	"forklift-training/internal/security"
	"forklift-training/internal/service"
	"forklift-training/internal/testutil"
)

const (
	disposeAccount     = "disposeme"
	disposeOldPassword = "oldpass123"
	disposeNewPassword = "newpass123"
)

// newDispositionFixture 装配「管理面处置动作 + 双令牌轮换」的最小真链路（共用一个 Session）。
// 返回的学员处于启用态（status=1）——禁用态走不到签发，故令牌由 Session 直接签。
func newDispositionFixture(t *testing.T, bl security.BlacklistStore) (*service.AdminService, *security.Session, *gorm.DB, int) {
	t.Helper()
	db := testutil.NewMemoryDB(t)
	sess := security.NewSessionWithBlacklistAndRefresh("test-secret", time.Hour, 7*time.Hour,
		security.CookieConfig{Name: "hrwai_token"}, bl)

	hashed, err := service.HashPassword(disposeOldPassword)
	if err != nil {
		t.Fatalf("哈希种子口令失败: %v", err)
	}
	u := model.HrwaiUser{
		UID: 910002, Account: disposeAccount, Username: "待处置学员",
		Password: hashed, Status: 1,
	}
	if err := db.Create(&u).Error; err != nil {
		t.Fatalf("播种学员账号失败: %v", err)
	}
	return service.NewAdminService(db, sess, zap.NewNop()), sess, db, u.ID
}

// issueStudentRefresh 以学员角色命名空间签一枚 refresh（与登录签发的 claims.Role 同源）。
func issueStudentRefresh(t *testing.T, sess *security.Session, uid int) string {
	t.Helper()
	_, refresh, err := sess.IssuePair(uid, disposeAccount, service.HrwaiRole)
	if err != nil {
		t.Fatalf("签发学员令牌对失败: %v", err)
	}
	return refresh
}

// rotationAccepted 判定这枚 refresh 还能不能轮换出新的。
func rotationAccepted(sess *security.Session, refresh string) bool {
	_, _, err := sess.RotateRefresh(context.Background(), refresh)
	return err == nil
}

func storedPassword(t *testing.T, db *gorm.DB, uid int) string {
	t.Helper()
	var u model.HrwaiUser
	if err := db.First(&u, uid).Error; err != nil {
		t.Fatalf("取回学员失败: %v", err)
	}
	return u.Password
}

// mustRotateOK 先证明这枚 refresh 此刻**能**轮换（对照组：被拒的原因必须是吊销标记，
// 不是令牌无效或装配没接好），并交出轮换后的「手上那一枚」——轮换本身作废旧值，
// 所以处置之后的断言必须打在轮换后的令牌上，否则等于在断言一个已被消费的东西。
func mustRotateOK(t *testing.T, sess *security.Session, refresh string) string {
	t.Helper()
	_, rotated, err := sess.RotateRefresh(context.Background(), refresh)
	if err != nil {
		t.Fatalf("处置前应可轮换（令牌或装配有问题）: %v", err)
	}
	return rotated
}

// TestDisableStudentRevokesAllSessions 禁用学员 = 全会话吊销（与禁用招聘者同判）。
func TestDisableStudentRevokesAllSessions(t *testing.T) {
	adminSvc, sess, _, uid := newDispositionFixture(t, newValBlacklist())
	inHand := mustRotateOK(t, sess, issueStudentRefresh(t, sess, uid))

	next, err := adminSvc.ToggleHrwaiUserStatus(context.Background(), uid)
	if err != nil {
		t.Fatalf("禁用学员失败: %v", err)
	}
	if next != 0 {
		t.Fatalf("禁用后状态应为 0，实际 %d", next)
	}
	if rotationAccepted(sess, inHand) {
		t.Fatal("禁用后链上最新的 refresh 仍可轮换 ⇒ 禁用对本人不起作用（吊销缺失）")
	}
}

// TestEnableStudentDoesNotRevoke 只有「禁用」是处置动作；恢复启用不写吊销标记
// （它不剥夺任何既有凭证，写标记反而会把「解除即恢复原状」做成二次惩罚）。
func TestEnableStudentDoesNotRevoke(t *testing.T) {
	adminSvc, sess, db, uid := newDispositionFixture(t, newValBlacklist())
	if err := db.Model(&model.HrwaiUser{}).Where("id = ?", uid).Update("status", 0).Error; err != nil {
		t.Fatalf("预置禁用态失败: %v", err)
	}
	refresh := issueStudentRefresh(t, sess, uid)

	next, err := adminSvc.ToggleHrwaiUserStatus(context.Background(), uid)
	if err != nil {
		t.Fatalf("恢复启用失败: %v", err)
	}
	if next != 1 {
		t.Fatalf("恢复后状态应为 1，实际 %d", next)
	}
	if !rotationAccepted(sess, refresh) {
		t.Fatal("恢复启用却吊销了会话 ⇒ 吊销只应跟着「禁用」这一半")
	}
}

// TestAdminResetPasswordRevokesAllSessions 管理员代重置 = 与学员自助改密同一个动作 ⇒ 同样吊销。
func TestAdminResetPasswordRevokesAllSessions(t *testing.T) {
	adminSvc, sess, db, uid := newDispositionFixture(t, newValBlacklist())
	before := storedPassword(t, db, uid)
	inHand := mustRotateOK(t, sess, issueStudentRefresh(t, sess, uid))

	if err := adminSvc.ResetHrwaiUserPassword(context.Background(), uid, disposeNewPassword); err != nil {
		t.Fatalf("管理员代重置失败: %v", err)
	}
	if storedPassword(t, db, uid) == before {
		t.Fatal("代重置没改动口令")
	}
	if rotationAccepted(sess, inHand) {
		t.Fatal("代重置后旧 refresh 仍可轮换 ⇒ 管理员替用户自救，攻击者的链还活着")
	}
	if !service.VerifyPassword(disposeNewPassword, storedPassword(t, db, uid)) {
		t.Fatal("新口令验不过")
	}
}

// TestAdminResetPasswordSharesLengthRule 长度规则必须在**动作**里兜底，不只住在 handler：
// 否则任何绕过 handler 的 caller 都能落一个 3 位口令。非法口令不得留下任何副作用。
func TestAdminResetPasswordSharesLengthRule(t *testing.T) {
	adminSvc, _, db, uid := newDispositionFixture(t, newValBlacklist())
	before := storedPassword(t, db, uid)

	if err := adminSvc.ResetHrwaiUserPassword(context.Background(), uid, "123"); err == nil {
		t.Fatal("3 位口令被动作接受 ⇒ 唯一动作没带上共享的长度判据")
	}
	if storedPassword(t, db, uid) != before {
		t.Fatal("非法口令却改动了口令")
	}
}

// failOnSetBlacklist 吊销标记写不进、其余照常——用来钉「尽力而为」那一半。
type failOnSetBlacklist struct{ setCalls int }

func (failOnSetBlacklist) Get(_ context.Context, _ string) (string, error) {
	return "", errors.New("not found")
}
func (s *failOnSetBlacklist) Set(_ context.Context, _, _ string, _ time.Duration) error {
	s.setCalls++
	return errors.New("redis down")
}
func (failOnSetBlacklist) PutIfAbsent(_ context.Context, _, _ string, _ time.Duration) (bool, error) {
	return true, nil
}

// TestDispositionRevokeFailureDoesNotRollBackAction 吊销写失败不回退处置本身（禁用已落库），
// 但那条缺口必须仍然被走到（Set 真被调用过一次），否则「尽力而为」会退化成都没尝试。
func TestDispositionRevokeFailureDoesNotRollBackAction(t *testing.T) {
	bl := &failOnSetBlacklist{}
	adminSvc, _, db, uid := newDispositionFixture(t, bl)

	if _, err := adminSvc.ToggleHrwaiUserStatus(context.Background(), uid); err != nil {
		t.Fatalf("吊销写失败不得让禁用本身失败: %v", err)
	}
	var u model.HrwaiUser
	if err := db.First(&u, uid).Error; err != nil || u.Status != 0 {
		t.Fatalf("禁用应已落库（status=0），实际 err=%v status=%d", err, u.Status)
	}
	if bl.setCalls == 0 {
		t.Fatal("吊销标记一次都没尝试写 ⇒ 尽力而为退化成了不尝试")
	}
}
