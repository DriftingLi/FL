// #1098 静态扫描（ADR-0056 §4 锁）：api 层不得出现站内信文案/payload 构造。
//
// 站内信 title/content/link/payload 的口径只允许来自站内信域事件构造器
// （service/notification_events.go，ADR-0027 全域收编）；api 层只做「解析 → 调 service → 渲染」。
// 判据宿主在包内 Go 测试（读本包非测试源文件断言），不新增 CI 步骤、不动 scripts/。
package api

import (
	"os"
	"strings"
	"testing"
)

// notificationCompositionForbidden api 层禁止出现的站内信构造形态。
var notificationCompositionForbidden = []struct {
	what   string
	needle string
}{
	{"站内信事务内写入调用", "CreateWithTx("},
	{"站内信写入调用", "notificationSvc.Create("},
	{"站内信写入调用（装配根字段）", "NotificationSvc.Create("},
	{"站内信尽力而为写入调用", "TryCreate"},
	{"站内信 payload 构造", "model.JSONB("},
	{"扣罚站内信标题字面量", "\"积分扣罚\""},
	{"扣罚站内信正文字面量", "您的积分因"},
}

// TestAPILayerHasNoNotificationComposition 扫描 api 包源码：出现任一禁止形态即红。
func TestAPILayerHasNoNotificationComposition(t *testing.T) {
	entries, err := os.ReadDir(".")
	if err != nil {
		t.Fatalf("读取 api 包目录失败: %v", err)
	}
	scanned := 0
	for _, e := range entries {
		name := e.Name()
		if e.IsDir() || !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
			continue
		}
		src, err := os.ReadFile(name)
		if err != nil {
			t.Fatalf("读取 %s 失败: %v", name, err)
		}
		scanned++
		text := string(src)
		for _, f := range notificationCompositionForbidden {
			idx := strings.Index(text, f.needle)
			if idx < 0 {
				continue
			}
			line := 1 + strings.Count(text[:idx], "\n")
			t.Errorf("%s:%d 出现%s（%q）——站内信口径只能来自 service/notification_events.go 的事件构造器",
				name, line, f.what, f.needle)
		}
	}
	if scanned == 0 {
		t.Fatal("未扫描到任何 api 非测试源文件（判据宿主失效）")
	}
}
