// 管理域的 nonnil 行为例（ADR-0065 决策 5 判据 5）：跑真实出口、看发出的是 [] 还是 null。
//
// 为什么证据跟着域包走（ADR-0070）：判据 5 的表是「哪些域声明 nonnil、谁举证」的对应关系，
// 域包拆出去之后域的实现与它的举证住在同一个包里，改名 / 搬目录不会让两侧各自漂。
// 断言本体只有一份：testutil.AssertNonNilOutlets。
//
// 三格是波 4d 从 internal/service 的 nonnilOutletsPeople / nonnilOutletsStats 两张表搬来的
// （生产者 ListHrwaiUsers / GetTutors / GetStatistics 的方法体就在本包），键前缀随包名
// 从 service 改成 admin。
package admin

import (
	"testing"

	"go.uber.org/zap"

	"forklift-training/internal/testutil"
)

// nonnilOutletsAdmin 键 = 包名.类型名.json键，值 = 走真实出口取到的结果。
// 前缀 nonnilOutlets 是约定的证据表名：apitypes 的表态锁按它扫目录收键（见
// internal/apitypes/nullability_lock_test.go 的 nonNilEvidenceSources）。
var nonnilOutletsAdmin = map[string]func(t *testing.T) any{
	"admin.HrwaiUserPageResult.list":        outletHrwaiUserPageEmpty,
	"admin.TutorListDTO.tutors":             outletTutorListEmpty,
	"admin.AdminStatisticsDTO.course_stats": outletAdminStatisticsNoCourses,
	"admin.AdminCapabilitiesDTO.capabilities": outletAdminCapabilitiesUnknownAdmin,
}

// TestNonNilDeclaredOutletsNeverEmitNull 本域的举证入口（判据本体在 testutil）。
func TestNonNilDeclaredOutletsNeverEmitNull(t *testing.T) {
	testutil.AssertNonNilOutlets(t, []map[string]func(t *testing.T) any{nonnilOutletsAdmin})
}

// outletHrwaiUserPageEmpty 管理端用户列表：空库时 list 是空集。
func outletHrwaiUserPageEmpty(t *testing.T) any {
	t.Helper()
	svc := NewService(testutil.NewMemoryDB(t), nil, zap.NewNop())
	res, err := svc.ListHrwaiUsers(1, 20, "")
	if err != nil {
		t.Fatalf("用户列表失败: %v", err)
	}
	return res
}

// outletTutorListEmpty 导师列表：空库时 tutors 是空集。
func outletTutorListEmpty(t *testing.T) any {
	t.Helper()
	svc := NewService(testutil.NewMemoryDB(t), nil, zap.NewNop())
	res, err := svc.GetTutors(1, 20, "")
	if err != nil {
		t.Fatalf("导师列表失败: %v", err)
	}
	return res
}

// outletAdminStatisticsNoCourses 管理端统计看板：无课程时 course_stats 由 make([]CourseStatDTO,0,n) 起手。
func outletAdminStatisticsNoCourses(t *testing.T) any {
	t.Helper()
	return NewService(testutil.NewMemoryDB(t), nil, zap.NewNop()).GetStatistics()
}

// outletAdminCapabilitiesUnknownAdmin 能力集出口：账号不存在（未授权）时 capabilities 是**空数组**。
// 这是最容易发出 null 的那条分支（空集），也正是要举证的那一格。
func outletAdminCapabilitiesUnknownAdmin(t *testing.T) any {
	t.Helper()
	svc := NewService(testutil.NewMemoryDB(t), nil, zap.NewNop())
	res, err := svc.AdminCapabilitySet(999999)
	if err != nil {
		t.Fatalf("能力集出口失败: %v", err)
	}
	return res
}
