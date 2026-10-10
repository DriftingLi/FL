// 管理员账号的新建与删除（#1632）。
//
// 收敛前这两件事没有端点（只有 GET 列表与 PUT 改挂角色），界面上的「新建/删除」点不动。
//
// 为什么「删成功」这条打在这一层而不是 HTTP 面：删除要先写**吊销标记**（注销族语义，
// ADR-0064 决策 4），而契约装配里服务持有的会话是 Redis 版 —— NewDeps 的 coreSingletons 用
// SessionFromConfig 建会话，契约 helper 只换掉了 d.Session（middleware 与蓝图注册那一份），
// 这一层没有 Redis。于是：
//   - 本文件锁「删掉了 + 旧 refresh 当场失效」（用真轮换链路断言，seam 同 disposition_revoke_contract_test.go）；
//   - internal/api/admin_authz_contract_test.go 锁四条 HTTP 判据（自删 / 最后一个超管 / 不存在 / 参数）。
package admin

import (
	"context"
	"errors"
	"testing"
	"time"

	"go.uber.org/zap"

	"forklift-training/internal/authz"
	"forklift-training/internal/core"
	"forklift-training/internal/model"
	"forklift-training/internal/security"
	"forklift-training/internal/testutil"
)

// newAdminAccountFixture 装配「管理员账号写面 + 会话单例」的最小真链路。
func newAdminAccountFixture(t *testing.T) (*Service, *security.Session, *model.Admin, *model.AdminRole) {
	t.Helper()
	db := testutil.NewMemoryDB(t)
	sess := security.NewSessionWithBlacklistAndRefresh("admin-account-secret", time.Hour, 7*time.Hour,
		security.CookieConfig{Name: "hrwai_token"}, testutil.NewValueBlacklist())
	svc := NewService(db, sess, zap.NewNop())
	super := testutil.SeedAdmin(t, db, "acct_super", "x")
	role := model.AdminRole{Name: "客服"}
	if err := db.Create(&role).Error; err != nil {
		t.Fatalf("播种普通角色失败: %v", err)
	}
	return svc, sess, super, &role
}

func TestAdminAccountCreateAndDelete(t *testing.T) {
	t.Parallel()
	svc, sess, super, role := newAdminAccountFixture(t)
	db := svc.db

	// ① 新建：落库 + 所挂角色 + 口令是 bcrypt（登录路径的校验认得出）
	created, err := svc.CreateAdminAccount("acct_new", "新管理员", "newpass123", role.RoleID)
	if err != nil {
		t.Fatalf("新建管理员失败: %v", err)
	}
	if created.AdminID == 0 || created.RoleID != role.RoleID || created.RoleName != "客服" || created.Protected {
		t.Fatalf("新建管理员形状不对: %+v", created)
	}
	var stored model.Admin
	if err := db.Where("admin_id = ?", created.AdminID).First(&stored).Error; err != nil {
		t.Fatalf("新建的管理员没落库: %v", err)
	}
	if stored.Password == "newpass123" || !core.VerifyPassword("newpass123", stored.Password) {
		t.Fatalf("口令不是 bcrypt 哈希: %q", stored.Password)
	}

	// ② 新建的三条拒绝：重名 / 口令不合规（长度规则在动作里兜底）/ 角色不存在
	if _, err := svc.CreateAdminAccount("acct_new", "重名", "newpass123", 0); !errors.Is(err, ErrAdminUsernameTaken) {
		t.Fatalf("账号名重复应 ErrAdminUsernameTaken, got %v", err)
	}
	if _, err := svc.CreateAdminAccount("acct_tiny", "短口令", "123", 0); err == nil {
		t.Fatal("口令过短应被动作层拒绝")
	}
	if _, err := svc.CreateAdminAccount("acct_badrole", "坏角色", "newpass123", 999999); !errors.Is(err, ErrRoleNotFound) {
		t.Fatalf("角色不存在应 ErrRoleNotFound, got %v", err)
	}
	// 姓名留空回落为账号名（展示字段不是判据）
	nameless, err := svc.CreateAdminAccount("acct_nameless", "  ", "newpass123", 0)
	if err != nil || nameless.Name != "acct_nameless" {
		t.Fatalf("姓名留空应回落为账号名: %+v err=%v", nameless, err)
	}

	// ③ 删除：先吊销后落库 —— 旧 refresh 当场失效，账号行消失
	_, refresh, err := sess.IssuePair(created.AdminID, "acct_new", string(authz.RoleAdmin))
	if err != nil {
		t.Fatalf("签发管理员令牌对失败: %v", err)
	}
	if _, _, err := sess.RotateRefresh(context.Background(), refresh); err != nil {
		t.Fatalf("前置：新签的 refresh 应该能轮换: %v", err)
	}
	if err := svc.DeleteAdminAccount(context.Background(), super.AdminID, created.AdminID); err != nil {
		t.Fatalf("删除管理员失败: %v", err)
	}
	var left int64
	if err := db.Model(&model.Admin{}).Where("admin_id = ?", created.AdminID).Count(&left).Error; err != nil {
		t.Fatalf("查删除结果失败: %v", err)
	}
	if left != 0 {
		t.Fatalf("删除后账号仍在库里: %d 行", left)
	}
	// 吊销命名空间必须与 JWT 的 role claim 同源，否则标记写得进、读不出
	if _, _, err := sess.RotateRefresh(context.Background(), refresh); err == nil {
		t.Fatal("被删账号的 refresh 仍能轮换（吊销命名空间与 role claim 不同源？）")
	}

	// ④ 三条拒绝：自删 / 最后一个超管 / 账号不存在
	if err := svc.DeleteAdminAccount(context.Background(), super.AdminID, super.AdminID); !errors.Is(err, ErrSelfDelete) {
		t.Fatalf("删自己应 ErrSelfDelete, got %v", err)
	}
	other, err := svc.CreateAdminAccount("acct_other", "另一个", "newpass123", 0)
	if err != nil {
		t.Fatalf("建第二个管理员失败: %v", err)
	}
	if err := svc.DeleteAdminAccount(context.Background(), other.AdminID, super.AdminID); !errors.Is(err, ErrLastSuperAdmin) {
		t.Fatalf("删最后一个超管应 ErrLastSuperAdmin, got %v", err)
	}
	if err := svc.DeleteAdminAccount(context.Background(), super.AdminID, 999999); !errors.Is(err, ErrAdminNotFound) {
		t.Fatalf("删不存在的账号应 ErrAdminNotFound, got %v", err)
	}
	if err := svc.DeleteAdminAccount(context.Background(), super.AdminID, 0); !errors.Is(err, ErrInvalidAdminID) {
		t.Fatalf("id 非正数应 ErrInvalidAdminID, got %v", err)
	}
	// 有两个超管后，删其中一个放行（判据是「最后一个」，不是「超管」）
	second := testutil.SeedAdmin(t, db, "acct_super2", "x")
	if err := svc.DeleteAdminAccount(context.Background(), super.AdminID, second.AdminID); err != nil {
		t.Fatalf("有第二个超管时应可删, got %v", err)
	}
}

func TestAdminResetPassword(t *testing.T) {
	t.Parallel()
	svc, sess, super, role := newAdminAccountFixture(t)
	db := svc.db

	created, err := svc.CreateAdminAccount("acct_pwd", "待重置", "oldpass123", role.RoleID)
	if err != nil {
		t.Fatalf("建号失败: %v", err)
	}
	_, refresh, err := sess.IssuePair(created.AdminID, "acct_pwd", core.AdminRole)
	if err != nil {
		t.Fatalf("签发管理员令牌对失败: %v", err)
	}
	if _, _, err := sess.RotateRefresh(context.Background(), refresh); err != nil {
		t.Fatalf("前置：新签的 refresh 应该能轮换: %v", err)
	}

	// ① 代重置：新口令落库（bcrypt 可校验），旧 refresh 链当场失效
	if err := svc.ResetAdminPassword(context.Background(), created.AdminID, "newpass456"); err != nil {
		t.Fatalf("代重置口令失败: %v", err)
	}
	var stored model.Admin
	if err := db.Where("admin_id = ?", created.AdminID).First(&stored).Error; err != nil {
		t.Fatalf("取回管理员失败: %v", err)
	}
	if !core.VerifyPassword("newpass456", stored.Password) || core.VerifyPassword("oldpass123", stored.Password) {
		t.Fatal("口令没有被换成新值（或旧值仍可校验）")
	}
	if _, _, err := sess.RotateRefresh(context.Background(), refresh); err == nil {
		t.Fatal("代重置后旧 refresh 仍能轮换（吊销命名空间与 role claim 不同源？）")
	}

	// ② 三条拒绝：长度不合规（动作层兜底）/ 账号不存在 / id 非正数
	if err := svc.ResetAdminPassword(context.Background(), created.AdminID, "123"); err == nil {
		t.Fatal("口令过短应被动作层拒绝")
	}
	if err := svc.ResetAdminPassword(context.Background(), 999999, "newpass456"); !errors.Is(err, ErrAdminNotFound) {
		t.Fatalf("账号不存在应 ErrAdminNotFound, got %v", err)
	}
	if err := svc.ResetAdminPassword(context.Background(), 0, "newpass456"); !errors.Is(err, ErrInvalidAdminID) {
		t.Fatalf("id 非正数应 ErrInvalidAdminID, got %v", err)
	}

	// ③ 受保护角色的账号也可以重置口令（改口令不是降权，不触发防自锁第二层）
	if err := svc.ResetAdminPassword(context.Background(), super.AdminID, "superpass789"); err != nil {
		t.Fatalf("超管账号应可重置口令, got %v", err)
	}
}
