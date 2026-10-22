// 本文件：#1364 的三条锁 —— 生成链锁（改表不改生成物即红）、逐档一致锁、接线锁。
//
// 形状沿用同包的 topology_test.go：
//   - 同步契约走 codegen.AssertInSync（字节级全等，行尾按 LF 语义比对）；
//   - 「生成物有真实消费者」由 TestCaddyfileWiresDeliveryMatchers 钉住
//     —— 否则某次重构能把引用点摘掉，留下一段没人用的匹配器。
//
// ⚠️ 跑这批锁要带 `-count=1`：产物在 Go 模块外（frontend/），**不进测试缓存的输入集**，
// 只改产物时 `go test` 会回 `ok (cached)`。CI 侧因此把新鲜度锁做成
// 「`go run ./cmd/gen-deploy -only caddy` + git diff 必须为空」，并在同一步骤里
// 用 `go test -run TestCaddy -count=1` 把这批锁在 frontend 面上重跑一遍。
package deploy

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"forklift-training/internal/codegen"
	"forklift-training/internal/filestore"
)

// matcherPat 解析生成区里的一条命名匹配器：`@delivery_inline path_regexp (?i).(bmp|doc)$`。
var matcherPat = regexp.MustCompile(`^@([a-z_]+) path_regexp \(\?i\)\\\.\(([a-z0-9|]+)\)\$$`)

// matcherNameByClass 三档 → 生成区里的匹配器名（改名字要同步本表与 Caddyfile 的引用点）。
var matcherNameByClass = map[filestore.FileTypeClass]string{
	filestore.FileTypeSafe:    "delivery_inline",
	filestore.FileTypeUnsafe:  "delivery_unsafe",
	filestore.FileTypeUnknown: "delivery_unknown",
}

// TestCaddyDeliveryInSync 生成链锁（#1364 判据 ①）：Caddyfile 的生成区必须与
// 「由 fileTypeTable 现算的结果」字节级全等。改表不重生成、或手改生成区，本测试即红。
func TestCaddyDeliveryInSync(t *testing.T) {
	codegen.AssertInSync(t, CaddyDeliveryGen)
}

// TestCaddyDeliveryCoversFileTypeTable 逐档一致锁：生成物里 safe / unsafe / unknown 三档
// 必须与类型表**逐条**对上，且双向都不许多、不少、不改档。
//
// 为什么不能只靠上面那条全等锁：全等锁比的是「现算 vs 磁盘」，如果现算本身漏了表里一行
// （解析形态漂移、某一档被静默摘掉），两条锁会一起绿 —— 那正是 ADR-0066 决策 6 要防的
// 「新增类型没被逼着表态」。这里独立取一次表（AST + 运行期两侧对账），再与生成物逐条比。
func TestCaddyDeliveryCoversFileTypeTable(t *testing.T) {
	rows, err := fileTypeTableRows()
	if err != nil {
		t.Fatalf("读类型表失败: %v", err)
	}
	region, err := RenderCaddyDeliveryRegion()
	if err != nil {
		t.Fatalf("渲染生成区失败: %v", err)
	}

	got := map[string]string{}
	for _, line := range strings.Split(region, "\n") {
		m := matcherPat.FindStringSubmatch(strings.TrimSpace(line))
		if m == nil {
			continue
		}
		name := m[1]
		for _, ext := range strings.Split(m[2], "|") {
			if prev, dup := got[ext]; dup {
				t.Fatalf("扩展名 %q 在生成区里出现两次（匹配器 %s 与 %s）：多条交替项会让档位判定不可读", ext, prev, name)
			}
			got[ext] = name
		}
	}
	if len(got) == 0 {
		t.Fatal("生成区里一条匹配器都没解析到：生成的形态变了，本锁已失效")
	}

	// 正向：表里每一行都必须在生成区里，且落在正确的匹配器上。
	for _, r := range rows {
		want, ok := matcherNameByClass[r.class]
		if !ok {
			t.Fatalf("表里出现未知分档 %v（本锁需要跟着扩展）", r.class)
		}
		name, found := got[r.ext]
		if !found {
			t.Errorf("生成区缺扩展名 %q（表里是 %v 档 ⇒ 应在 @%s）：漏一行就等于让它静默落到兜底档", r.ext, r.class, want)
			continue
		}
		if name != want {
			t.Errorf("生成区 %q 落在 @%s，表里是 %v 档 ⇒ 应在 @%s", r.ext, name, r.class, want)
		}
		delete(got, r.ext)
	}
	// 反向：生成区不得有表里没有的键 —— 那就是在反代配置里手抄了第二份规则。
	for ext, name := range got {
		t.Errorf("生成区多出扩展名 %q（@%s），类型表里没有它", ext, name)
	}
}

// TestCaddyDeliveryMatchersAreCaseInsensitive 语义保持锁：nginx 原用 `~*`（不区分大小写），
// Caddy 的 path 匹配**区分**大小写 ⇒ 每条匹配器都必须以 `(?i)` 开头。
// 丢了它，`A.JPG` 会从 inline 掉到 attachment（下载语义），而 ADR-0066 锁②依赖的是「扩展名同档」。
func TestCaddyDeliveryMatchersAreCaseInsensitive(t *testing.T) {
	region, err := RenderCaddyDeliveryRegion()
	if err != nil {
		t.Fatalf("渲染生成区失败: %v", err)
	}
	var n int
	for _, line := range strings.Split(region, "\n") {
		trimmed := strings.TrimSpace(line)
		if !strings.HasPrefix(trimmed, "@") {
			continue
		}
		n++
		if !strings.Contains(trimmed, "path_regexp (?i)") {
			t.Errorf("匹配器缺少 (?i) 前缀（大小写语义会变）：%s", trimmed)
		}
	}
	if n == 0 {
		t.Fatal("生成区里一条匹配器都没解析到")
	}
}

// TestCaddyfileWiresDeliveryMatchers 接线锁：生成区里的匹配器必须真的被 cdn 块消费。
// 判据是产物里的两行响应头 —— inline 覆写与 nosniff —— 且**不得**出现 Content-Type 覆写
// （#1364 决策 3）。
func TestCaddyfileWiresDeliveryMatchers(t *testing.T) {
	root, err := repoRoot()
	if err != nil {
		t.Fatalf("定位仓库根失败: %v", err)
	}
	raw, err := os.ReadFile(filepath.Join(root, "frontend", "Caddyfile"))
	if err != nil {
		t.Fatalf("读 Caddyfile 失败: %v", err)
	}
	full := strings.ReplaceAll(string(raw), "\r\n", "\n")

	for _, want := range []string{
		"header @delivery_inline Content-Disposition \"inline\"",
		"header X-Content-Type-Options \"nosniff\"",
	} {
		if n := strings.Count(full, want); n == 0 {
			t.Errorf("Caddyfile 里找不到接线行：%s（生成区有匹配器但没人消费）", want)
		}
	}
	// 兜底档：未命中 inline 的一律 attachment。生成区只声明匹配器，默认值在引用点。
	if n := strings.Count(full, "header Content-Disposition \"attachment\""); n == 0 {
		t.Error("Caddyfile 里没有 attachment 兜底行（ADR-0066 决策 6 锁②：未登记扩展名必须强制下载）")
	}
	// 只认「真的设了 Content-Type 响应头」的指令行：X-Content-Type-Options 属另一码事，
	// 生成区的注释里提到 Content-Type 也不算。
	contentTypeOverride := regexp.MustCompile(`^\s*header\s+(?:@[a-z_]+\s+)?Content-Type\b`)
	for i, line := range strings.Split(full, "\n") {
		if contentTypeOverride.MatchString(line) {
			t.Errorf("Caddyfile:%d 覆写了 Content-Type（#1364 决策 3 明确不做）：%s", i+1, strings.TrimSpace(line))
		}
	}
}
