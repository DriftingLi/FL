// Package tutorcap 测试：解析器的四类判定（四模板能力集 / 未挂角色空集 / 缓存语义 / 故障不入缓存）。
//
// 这些用例锁的是**语义**而不是实现：能力集从哪张表来是内部细节，但「未授权是空集」
// 「granted=false 也缓存」「故障不缓存」是规格 E2/E3 写下的对外承诺，改实现不该改它们。
package tutorcap

import (
	"testing"

	"gorm.io/gorm"

	"forklift-training/internal/authz"
	"forklift-training/internal/migrate"
	"forklift-training/internal/model"
	"forklift-training/internal/testutil"
)

// seedRole 建一个非 protected 角色并落能力行，返回 role_id。
func seedRole(t *testing.T, db *gorm.DB, name string, caps ...authz.Capability) int {
	t.Helper()
	role := model.TutorRole{Name: name}
	if err := db.Create(&role).Error; err != nil {
		t.Fatalf("建角色 %s 失败: %v", name, err)
	}
	for _, c := range caps {
		if err := db.Create(&model.TutorRoleCapability{RoleID: role.RoleID, Capability: string(c)}).Error; err != nil {
			t.Fatalf("落能力行失败: %v", err)
		}
	}
	return role.RoleID
}

// seedTutorWithRole 建讲师并挂角色（roleID=0 表示不挂）。
func seedTutorWithRole(t *testing.T, db *gorm.DB, username string, roleID int) *model.Tutor {
	t.Helper()
	tutor := testutil.SeedUnassignedTutor(t, db, username, "x")
	if roleID > 0 {
		if err := db.Model(&model.Tutor{}).Where("tutor_id = ?", tutor.TutorID).
			Update("role_id", roleID).Error; err != nil {
			t.Fatalf("挂角色失败: %v", err)
		}
		tutor.RoleID = &roleID
	}
	return tutor
}

// TestTutorCapabilities_FourTemplates 四个预置角色的能力集必须与 authz.TutorTemplateRoles() 逐键相等
// （规格 E6）。全权讲师走 protected 分支、由代码回答；其余三个走数据行。
func TestTutorCapabilities_FourTemplates(t *testing.T) {
	db := testutil.NewMemoryDB(t)
	if err := migrate.EnsureTutorRoleChain(db); err != nil {
		t.Fatalf("建讲师角色链失败: %v", err)
	}
	resolver := New(db, nil)

	var roles []model.TutorRole
	if err := db.Find(&roles).Error; err != nil {
		t.Fatalf("查角色失败: %v", err)
	}
	byName := map[string]model.TutorRole{}
	for _, r := range roles {
		byName[r.Name] = r
	}

	for _, tpl := range authz.TutorTemplateRoles() {
		role, ok := byName[tpl.Name]
		if !ok {
			t.Fatalf("预置角色 %q 未建出来", tpl.Name)
		}
		tutor := seedTutorWithRole(t, db, "tpl_"+tpl.Name, role.RoleID)
		got, granted, err := resolver.TutorCapabilities(tutor.TutorID)
		if err != nil {
			t.Fatalf("角色 %q 取能力集失败: %v", tpl.Name, err)
		}
		if !granted {
			t.Fatalf("角色 %q 已挂角色，granted 应为 true", tpl.Name)
		}
		want := tpl.Capabilities
		if tpl.Protected {
			want = authz.ProtectedTutorCapabilities()
		}
		if len(got) != len(want) {
			t.Fatalf("角色 %q 能力集 = %v（%d 个），want %v（%d 个）", tpl.Name, got, len(got), want, len(want))
		}
		for _, c := range want {
			if _, ok := got[c]; !ok {
				t.Fatalf("角色 %q 缺能力 %q", tpl.Name, c)
			}
		}
	}
}

// TestTutorCapabilities_UnassignedIsEmptyAndNotGranted 新建讲师默认不挂角色 ⇒ 空集 + granted=false
// （规格 E2 fail closed）。
func TestTutorCapabilities_UnassignedIsEmptyAndNotGranted(t *testing.T) {
	db := testutil.NewMemoryDB(t)
	tutor := testutil.SeedUnassignedTutor(t, db, "unassigned", "x")

	caps, granted, err := New(db, nil).TutorCapabilities(tutor.TutorID)
	if err != nil {
		t.Fatalf("取能力集失败: %v", err)
	}
	if granted {
		t.Fatal("未挂角色时 granted 应为 false")
	}
	if len(caps) != 0 {
		t.Fatalf("未挂角色时应为空集，得到 %v", caps)
	}
}

// TestTutorCapabilities_EmptyRoleIsGrantedButEmpty 空能力角色与未挂角色都是空集，但 granted 不同 ——
// 前端据此区分「还没分配角色」与「角色里什么都没勾」。
func TestTutorCapabilities_EmptyRoleIsGrantedButEmpty(t *testing.T) {
	db := testutil.NewMemoryDB(t)
	roleID := seedRole(t, db, "空能力讲师")
	tutor := seedTutorWithRole(t, db, "empty_role", roleID)

	caps, granted, err := New(db, nil).TutorCapabilities(tutor.TutorID)
	if err != nil {
		t.Fatalf("取能力集失败: %v", err)
	}
	if !granted {
		t.Fatal("挂了角色（即便空能力）granted 应为 true")
	}
	if len(caps) != 0 {
		t.Fatalf("空能力角色应得空集，得到 %v", caps)
	}
}

// TestTutorCapabilities_CachesEmptyGrant TTL 内 granted=false 也走缓存（规格 E3）。
//
// 判据：先取一次（缓存空集），随后在库里挂上角色，再取一次 —— 若仍为未授权，说明读的是缓存。
// 这条不是性能洁癖：未授权讲师恰恰是最可能被反复尝试的那类请求，不缓存就会每次打库。
func TestTutorCapabilities_CachesEmptyGrant(t *testing.T) {
	db := testutil.NewMemoryDB(t)
	tutor := testutil.SeedUnassignedTutor(t, db, "cache_empty", "x")
	resolver := New(db, nil)

	if _, granted, err := resolver.TutorCapabilities(tutor.TutorID); err != nil || granted {
		t.Fatalf("首次应为未授权且无故障，得到 granted=%v err=%v", granted, err)
	}

	roleID := seedRole(t, db, "后挂的角色", authz.CapTutorAccess)
	if err := db.Model(&model.Tutor{}).Where("tutor_id = ?", tutor.TutorID).
		Update("role_id", roleID).Error; err != nil {
		t.Fatalf("挂角色失败: %v", err)
	}

	_, granted, err := resolver.TutorCapabilities(tutor.TutorID)
	if err != nil {
		t.Fatalf("第二次取能力集失败: %v", err)
	}
	if granted {
		t.Fatal("TTL 内应读缓存（仍为未授权）；读到新角色说明缓存没生效")
	}
}

// TestTutorCapabilities_FailureNotCached 查询故障**不入缓存**（规格 E3）。
//
// 判据：把表删掉 → 取一次必失败；恢复表并把角色挂上 → 再取一次应成功。
// 若故障被缓存了，第二次仍会失败（一次抖动让该讲师在 TTL 内持续被判无权限）。
func TestTutorCapabilities_FailureNotCached(t *testing.T) {
	db := testutil.NewMemoryDB(t)
	roleID := seedRole(t, db, "故障后要挂的角色", authz.CapTutorAccess)
	tutor := seedTutorWithRole(t, db, "cache_fail", roleID)
	resolver := New(db, nil)

	// 先让 load 走一次真故障：讲师已挂角色、角色表被删 ⇒ 查询必报错。
	if err := db.Migrator().DropTable(&model.TutorRole{}, &model.TutorRoleCapability{}); err != nil {
		t.Fatalf("删角色表失败: %v", err)
	}
	if _, _, err := resolver.TutorCapabilities(tutor.TutorID); err == nil {
		t.Fatal("表不存在时应返回故障，而不是「未授权」")
	}

	// 恢复表与角色行（DropTable 连带删了角色，重建后重挂）。
	if err := db.AutoMigrate(&model.TutorRole{}, &model.TutorRoleCapability{}); err != nil {
		t.Fatalf("重建角色表失败: %v", err)
	}
	restored := model.TutorRole{Name: "故障后要挂的角色"}
	if err := db.Create(&restored).Error; err != nil {
		t.Fatalf("重建角色失败: %v", err)
	}
	if err := db.Create(&model.TutorRoleCapability{RoleID: restored.RoleID, Capability: string(authz.CapTutorAccess)}).Error; err != nil {
		t.Fatalf("重建能力行失败: %v", err)
	}
	if err := db.Model(&model.Tutor{}).Where("tutor_id = ?", tutor.TutorID).
		Update("role_id", restored.RoleID).Error; err != nil {
		t.Fatalf("挂角色失败: %v", err)
	}

	caps, granted, err := resolver.TutorCapabilities(tutor.TutorID)
	if err != nil {
		t.Fatalf("故障恢复后应能正常解析，得到 %v", err)
	}
	if !granted {
		t.Fatal("故障恢复后 granted 应为 true（说明上一次故障被缓存了）")
	}
	if _, ok := caps[authz.CapTutorAccess]; !ok {
		t.Fatalf("能力集缺 tutor.access: %v", caps)
	}
}

// TestTutorCapabilities_InvalidID 非法 / 不存在的讲师 id 一律「未授权 + 无故障」。
func TestTutorCapabilities_InvalidID(t *testing.T) {
	db := testutil.NewMemoryDB(t)
	resolver := New(db, nil)

	for _, id := range []int{0, -1} {
		caps, granted, err := resolver.TutorCapabilities(id)
		if err != nil || granted || len(caps) != 0 {
			t.Fatalf("id=%d 应为空集未授权无故障，得到 caps=%v granted=%v err=%v", id, caps, granted, err)
		}
	}
	// 账号不存在：令牌指向了已删除的讲师，也是「未授权」而不是故障。
	caps, granted, err := resolver.TutorCapabilities(999999)
	if err != nil || granted || len(caps) != 0 {
		t.Fatalf("不存在的讲师应为空集未授权无故障，得到 caps=%v granted=%v err=%v", caps, granted, err)
	}
}

// TestTutorCapabilities_RoleDeletedIsUnassigned 角色被删而讲师还挂着它 ⇒ 按未授权处理，不当故障。
func TestTutorCapabilities_RoleDeletedIsUnassigned(t *testing.T) {
	db := testutil.NewMemoryDB(t)
	roleID := seedRole(t, db, "将被删除的角色", authz.CapTutorAccess)
	tutor := seedTutorWithRole(t, db, "role_deleted", roleID)

	if err := db.Where("role_id = ?", roleID).Delete(&model.TutorRole{}).Error; err != nil {
		t.Fatalf("删角色失败: %v", err)
	}

	caps, granted, err := New(db, nil).TutorCapabilities(tutor.TutorID)
	if err != nil {
		t.Fatalf("角色缺失不应报故障: %v", err)
	}
	if granted || len(caps) != 0 {
		t.Fatalf("角色缺失应按未授权处理，得到 caps=%v granted=%v", caps, granted)
	}
}

// TestTutorCapabilities_PredicateIgnoresAdminRole 负向锁（规格硬约束 3 的延伸）：
// 讲师能力只读 tutor_role，管理员角色表的内容不得渗进来。
//
// 造一个「管理员角色里有 tutor.access、讲师角色里没有」的局面：讲师必须拿不到该键。
func TestTutorCapabilities_PredicateIgnoresAdminRole(t *testing.T) {
	db := testutil.NewMemoryDB(t)
	if err := migrate.EnsureTutorRoleChain(db); err != nil {
		t.Fatalf("建讲师角色链失败: %v", err)
	}
	// 管理员侧角色塞满讲师侧能力（模拟两张表被顺手混用的误实现）。
	adminRole := model.AdminRole{Name: "越界的角色"}
	if err := db.Create(&adminRole).Error; err != nil {
		t.Fatalf("建管理员角色失败: %v", err)
	}
	for _, c := range authz.ProtectedTutorCapabilities() {
		if err := db.Create(&model.AdminRoleCapability{RoleID: adminRole.RoleID, Capability: string(c)}).Error; err != nil {
			t.Fatalf("落管理员能力行失败: %v", err)
		}
	}

	tutor := testutil.SeedUnassignedTutor(t, db, "admin_leak", "x")
	caps, granted, err := New(db, nil).TutorCapabilities(tutor.TutorID)
	if err != nil {
		t.Fatalf("取能力集失败: %v", err)
	}
	if granted || len(caps) != 0 {
		t.Fatalf("未挂讲师角色的讲师不应从管理员角色表拿到能力，得到 caps=%v granted=%v", caps, granted)
	}
}
