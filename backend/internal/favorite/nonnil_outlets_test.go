// 收藏域的 nonnil 行为例（ADR-0065 决策 5 判据 5）：跑真实出口、看发出的是 [] 还是 null。
//
// 这一格原先住在 internal/api/contact_list_items_shape_test.go 的路由级表里（键 service.
// FavoritePageResult.favorites），理由是「那一格是 handler 拼出来的」——波 4b 核对后确认该理由
// 不成立：favorites 由本包 List 的 make([]FavoriteDTO, 0, len(rows)) 组装、handler 原样透传，
// 所以证据按「谁的组装点谁举证」搬回本域（键前缀随包名改成 favorite）。
//
// 断言本体只有一份：testutil.AssertNonNilOutlets。
package favorite

import (
	"testing"

	"go.uber.org/zap"

	"forklift-training/internal/testutil"
)

// nonnilOutletsFavorite 键 = 包名.类型名.json键，值 = 走真实出口取到的结果。
// 前缀 nonnilOutlets 是约定的证据表名：apitypes 的表态锁按它扫目录收键（见
// internal/apitypes/nullability_lock_test.go 的 nonNilEvidenceSources）。
var nonnilOutletsFavorite = map[string]func(t *testing.T) any{
	"favorite.FavoritePageResult.favorites": outletFavoritePageEmpty,
}

// TestNonNilDeclaredOutletsNeverEmitNull 本域的举证入口（判据本体在 testutil）。
func TestNonNilDeclaredOutletsNeverEmitNull(t *testing.T) {
	testutil.AssertNonNilOutlets(t, []map[string]func(t *testing.T) any{nonnilOutletsFavorite})
}

// outletFavoritePageEmpty 零收藏的学员拉列表：favorites 由 make([]FavoriteDTO, 0, len(rows))
// 起手 ⇒ 空集也是 `[]`。
func outletFavoritePageEmpty(t *testing.T) any {
	t.Helper()
	db := testutil.NewMemoryDB(t)
	student := testutil.SeedStudent(t, db, "收藏空列表学员", "x")
	res, err := NewService(db, zap.NewNop()).List(student.ID, "", 1, 10, nil)
	if err != nil {
		t.Fatalf("空收藏列表失败: %v", err)
	}
	return res
}
