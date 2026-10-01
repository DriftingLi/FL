// #1098 静态扫描（ADR-0056 §4 锁）：api 层不得出现站内信文案/payload 构造。
//
// 站内信 title/content/link/payload 的口径只允许来自站内信域事件构造器
// （service/notification_events.go，ADR-0027 全域收编）；api 层只做「解析 → 调 service → 渲染」。
// 判据宿主在包内 Go 测试（读本包非测试源文件断言），不新增 CI 步骤、不动 scripts/。
package api

import (
	"strings"
	"testing"

	"forklift-training/internal/testutil"
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

// notificationHit 一处命中（判定面与报告共用，行号 1-based）。
type notificationHit struct {
	what   string
	needle string
	line   int
}

// scanNotificationComposition 判定面：返回源码里命中的禁止形态（空 = 通过）。
// 文件锁与下面的正负样本探针共用同一实现——探针若另写一份，锁本身仍可能空转。
func scanNotificationComposition(source string) []notificationHit {
	var out []notificationHit
	for _, f := range notificationCompositionForbidden {
		idx := strings.Index(source, f.needle)
		if idx < 0 {
			continue
		}
		out = append(out, notificationHit{what: f.what, needle: f.needle, line: 1 + strings.Count(source[:idx], "\n")})
	}
	return out
}

// TestAPILayerHasNoNotificationComposition 扫描 api 包源码：出现任一禁止形态即红。
func TestAPILayerHasNoNotificationComposition(t *testing.T) {
	t.Parallel()
	// 射程 = HTTP 面（testutil.HTTPSurface）：今天就是 internal/api 的非测试文件，拆包后跟着那
	// 一处定义走。**不能**宽成「全部生产代码」——站内信构造器本身住在 service 域，宽化会把实现处
	// 21 处命中当违规（实测）。
	scanned := 0
	for _, f := range testutil.ScanBackendCode(t) {
		if !testutil.HTTPSurface(f) {
			continue
		}
		scanned++
		for _, hit := range scanNotificationComposition(f.Src) {
			t.Errorf("%s:%d 出现%s（%q）——站内信口径只能来自 service/notification_events.go 的事件构造器",
				f.Path, hit.line, hit.what, hit.needle)
		}
	}
	if scanned == 0 {
		t.Fatal("未扫描到任何 HTTP 面非测试源文件（判据宿主失效）")
	}
}

// TestNotificationCompositionProbe 判定面的正负样本（合成源码）：
//   - 负样本：每个禁止形态各造一份含它的源码，必须都报出来（少一条 = 该形态的锁空转）；
//   - 正样本：收编后的干净形态（Endpoint + Render）不得报红；
//   - 行号也是判据：第 3 行的命中不得报成第 1 行。
func TestNotificationCompositionProbe(t *testing.T) {
	t.Parallel()
	for _, f := range notificationCompositionForbidden {
		hits := scanNotificationComposition("package api\nfunc probe() { _ = " + f.needle + " }\n")
		if len(hits) != 1 || hits[0].needle != f.needle {
			t.Fatalf("负样本必须报出形态 %q（%s），实得 %+v", f.needle, f.what, hits)
		}
	}
	clean := "package api\n\nfunc probe(c *gin.Context) {\n\tEndpoint[struct{}, DTO]{}.Handle(c)\n}\n"
	if hits := scanNotificationComposition(clean); len(hits) != 0 {
		t.Fatalf("正样本（收编后的端点骨架）不得报红，实得 %+v", hits)
	}
	if hits := scanNotificationComposition("package api\n// 注释\nvar _ = TryCreate\n"); len(hits) != 1 || hits[0].line != 3 {
		t.Fatalf("命中行号应为 3，实得 %+v", hits)
	}
}
