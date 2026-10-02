// 精选域的 nonnil 行为例（ADR-0065 决策 5 判据 5）：跑真实出口、看发出的是 [] 还是 null。
//
// 为什么证据跟着域包走（ADR-0070）：判据 5 的表是「哪些域声明 nonnil、谁举证」的对应关系，
// 域包拆出去之后域的实现与它的举证住在同一个包里，改名/搬目录不会让两侧各自漂。
// 断言本体只有一份：testutil.AssertNonNilOutlets。
package featured

import (
	"testing"

	"forklift-training/internal/testutil"
)

// nonnilOutletsFeatured 键 = 包名.类型名.json键，值 = 走真实出口取到的结果。
// 前缀 nonnilOutlets 是约定的证据表名：apitypes 的表态锁按它扫目录收键（见
// internal/apitypes/nullability_lock_test.go 的 nonNilEvidenceSources）。
var nonnilOutletsFeatured = map[string]func(t *testing.T) any{
	"featured.FeaturedContentPageResult.items":  outletFeaturedPageEmpty,
	"featured.FeaturedContentDetailDTO.related": outletFeaturedDetailNoRelated,
}

// TestNonNilDeclaredOutletsNeverEmitNull 本域的举证入口（判据本体在 testutil）。
func TestNonNilDeclaredOutletsNeverEmitNull(t *testing.T) {
	testutil.AssertNonNilOutlets(t, []map[string]func(t *testing.T) any{nonnilOutletsFeatured})
}

func outletFeaturedPageEmpty(t *testing.T) any {
	t.Helper()
	svc, _ := newFeaturedTestSvc(t)
	res, err := svc.GetPublicList(1, 20, "")
	if err != nil {
		t.Fatalf("精选公开列表失败: %v", err)
	}
	return res
}

// outletFeaturedDetailNoRelated related 的初值是 []FeaturedContentDTO{}，随后被
// `make(0,len(related))` 整格覆盖 ⇒ 同分类没有第二篇时发 `[]`（不是保留初值，故播一篇就够）。
func outletFeaturedDetailNoRelated(t *testing.T) any {
	t.Helper()
	svc, db := newFeaturedTestSvc(t)
	id := seedPublishedFeatured(t, db, "相关资讯为空", 0)
	res, err := svc.GetPublicDetail(id, false)
	if err != nil {
		t.Fatalf("精选公开详情失败: %v", err)
	}
	return res
}
