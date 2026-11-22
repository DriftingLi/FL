// 认证与账号域的 nonnil 行为例（ADR-0065 决策 5 判据 5）：跑真实出口、看发出的是 [] 还是 null。
//
// 为什么证据跟着域包走（ADR-0070）：判据 5 的表是「哪些域声明 nonnil、谁举证」的对应关系，
// 域包拆出去之后域的实现与它的举证住在同一个包里，改名/搬目录不会让两侧各自漂。
// 断言本体只有一份：testutil.AssertNonNilOutlets。
package auth

import (
	"testing"

	"go.uber.org/zap"

	"forklift-training/internal/core"
	"forklift-training/internal/testutil"
)

// nonnilOutletsAuth 键 = 包名.类型名.json键，值 = 走真实出口取到的结果。
// 前缀 nonnilOutlets 是约定的证据表名：apitypes 的表态锁按它扫目录收键（见
// internal/apitypes/nullability_lock_test.go 的 nonNilEvidenceSources）。
var nonnilOutletsAuth = map[string]func(t *testing.T) any{
	"auth.RecruiterListResult.items":               outletRecruiterListEmpty,
	"auth.ProfileChangeRequestPageResult.requests": outletProfileChangeRequestPageEmpty,
	"auth.ProfileDTO.pending_fields":               outletProfileDTOPendingFieldsEmpty,
}

// TestNonNilDeclaredOutletsNeverEmitNull 本域的举证入口（判据本体在 testutil）。
func TestNonNilDeclaredOutletsNeverEmitNull(t *testing.T) {
	testutil.AssertNonNilOutlets(t, []map[string]func(t *testing.T) any{nonnilOutletsAuth})
}

// outletRecruiterListEmpty 招聘者列表（#416 那条「硬编码空数组桩」的真实现）：空库发 `[]`。
// P2 波 3a（ADR-0070）：原住 internal/service/nonnil_outlets_people_test.go，随域搬来。
func outletRecruiterListEmpty(t *testing.T) any {
	t.Helper()
	svc := NewService(testutil.NewMemoryDB(t), nil, nil, "", "", "", zap.NewNop())
	res, err := svc.ListRecruiters(1, 20, "")
	if err != nil {
		t.Fatalf("招聘者列表失败: %v", err)
	}
	return res
}

// outletProfileDTOPendingFieldsEmpty /auth/me（#2712）：无待审时 pending_fields 是空数组，不是 null。
// 走真实出口而不是零值 DTO —— 零值切片必然是 nil，那不构成反例。
func outletProfileDTOPendingFieldsEmpty(t *testing.T) any {
	t.Helper()
	svc, tdb := newGetProfileSvc(t)
	u := testutil.SeedStudent(t, tdb, "nopending", "")
	return svc.GetProfile(u.ID, core.HrwaiRole, u.Account)
}

// outletProfileChangeRequestPageEmpty 资料审核列表：零申请时 requests 是空集。
// P2 波 3a（ADR-0070）：原住 internal/service/nonnil_outlets_people_test.go，随域搬来。
func outletProfileChangeRequestPageEmpty(t *testing.T) any {
	t.Helper()
	svc := NewProfileReviewService(testutil.NewMemoryDB(t), nil, nil, zap.NewNop())
	res, err := svc.ListRequests("", 1, 20)
	if err != nil {
		t.Fatalf("资料审核列表失败: %v", err)
	}
	return res
}
