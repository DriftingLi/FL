// 搜索域的 nonnil 行为例（ADR-0065 决策 5 判据 5）：跑真实出口、看发出的是 [] 还是 null。
//
// 为什么证据跟着域包走（ADR-0070）：判据 5 的表是「哪些域声明 nonnil、谁举证」的对应关系，
// 域包拆出去之后域的实现与它的举证住在同一个包里，改名/搬目录不会让两侧各自漂。
// 断言本体只有一份：testutil.AssertNonNilOutlets。
//
// 这一格是波 4b 从 internal/service 的 nonnilOutletsPeople 表搬来的（生产者 Search 的方法体
// 就在本包），键前缀随包名改成 search。
package search

import (
	"testing"

	"go.uber.org/zap"

	"forklift-training/internal/testutil"
)

// nonnilOutletsSearch 键 = 包名.类型名.json键，值 = 走真实出口取到的结果。
// 前缀 nonnilOutlets 是约定的证据表名：apitypes 的表态锁按它扫目录收键（见
// internal/apitypes/nullability_lock_test.go 的 nonNilEvidenceSources）。
var nonnilOutletsSearch = map[string]func(t *testing.T) any{
	"search.SearchSectionDTO.items": outletSearchSectionEmpty,
}

// TestNonNilDeclaredOutletsNeverEmitNull 本域的举证入口（判据本体在 testutil）。
func TestNonNilDeclaredOutletsNeverEmitNull(t *testing.T) {
	testutil.AssertNonNilOutlets(t, []map[string]func(t *testing.T) any{nonnilOutletsSearch})
}

// outletSearchSectionEmpty 聚合搜索的单个分区：SearchAllDTO 的五个分区字段共用这一条出口、
// 各自 marshal（同一处 make，一格一证）。
func outletSearchSectionEmpty(t *testing.T) any {
	t.Helper()
	svc := NewService(testutil.NewMemoryDB(t), zap.NewNop())
	res, err := svc.Search("液压泵压力不足", "", 1, 20, nil)
	if err != nil {
		t.Fatalf("聚合搜索失败: %v", err)
	}
	all, ok := res.(*SearchAllDTO)
	if !ok {
		t.Fatalf("聚合搜索返回了意料之外的类型 %T", res)
	}
	return all.Courses
}
