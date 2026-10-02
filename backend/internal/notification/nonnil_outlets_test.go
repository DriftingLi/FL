// 站内信通知域的 nonnil 行为例（ADR-0065 决策 5 判据 5）：跑真实出口、看发出的是 [] 还是 null。
//
// 为什么证据跟着域包走（ADR-0070）：判据 5 的表是「哪些域声明 nonnil、谁举证」的对应关系，
// 域包拆出去之后域的实现与它的举证住在同一个包里，改名/搬目录不会让两侧各自漂。
// 断言本体只有一份：testutil.AssertNonNilOutlets。
package notification

import (
	"testing"

	"go.uber.org/zap"

	"forklift-training/internal/testutil"
)

// nonnilOutletsNotification 键 = 包名.类型名.json键，值 = 走真实出口取到的结果。
// 前缀 nonnilOutlets 是约定的证据表名：apitypes 的表态锁按它扫目录收键（见
// internal/apitypes/nullability_lock_test.go 的 nonNilEvidenceSources）。
var nonnilOutletsNotification = map[string]func(t *testing.T) any{
	"notification.NotificationListPageResult.items": outletNotificationListEmpty,
}

// TestNonNilDeclaredOutletsNeverEmitNull 本域的举证入口（判据本体在 testutil）。
func TestNonNilDeclaredOutletsNeverEmitNull(t *testing.T) {
	testutil.AssertNonNilOutlets(t, []map[string]func(t *testing.T) any{nonnilOutletsNotification})
}

// outletNotificationListEmpty 站内信列表：零消息时 items 是空集。
func outletNotificationListEmpty(t *testing.T) any {
	t.Helper()
	res, err := NewService(testutil.NewMemoryDB(t), zap.NewNop()).List(1, 1, 20)
	if err != nil {
		t.Fatalf("站内信列表失败: %v", err)
	}
	return res
}
