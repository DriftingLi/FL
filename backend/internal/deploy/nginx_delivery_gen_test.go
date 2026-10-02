// 本文件：#1364 的三条锁 —— 生成链锁（改表不改生成物即红）、逐档一致锁、接线锁。
//
// 形状沿用同包的 topology_test.go：
//   - 同步契约走 codegen.AssertInSync（字节级全等，行尾按 LF 语义比对）；
//   - 「生成物有真实消费者」由 TestNginxDeliveryWiring 钉住（先例 TestEnvDefaultsIsConsumed）
//     —— 否则某次重构能把 include 点摘掉，留下一段没人用的 map。
//
// ⚠️ 跑这批锁要带 `-count=1`：产物在 Go 模块外（frontend/），**不进测试缓存的输入集**，
// 只改产物时 `go test` 会回 `ok (cached)`（本机实测：删掉生成区一行后不加 -count=1 照绿）。
// CI 侧因此把新鲜度锁做成「`go run ./cmd/gen-deploy -only nginx` + git diff 必须为空」，
// 并在同一步骤里用 `go test -run TestNginx -count=1` 把这六条锁在 frontend 面上重跑一遍
// （.github/workflows/ci.yml 的 frontend-check）—— 现算路径不吃缓存；而接线锁判的两行
// add_header 在生成区之外，git diff 看不见，只改那两行的 PR 又只命中 frontend/**。
// 改表本身会动依赖图，backend-test 那条全等锁必然重跑。两侧合起来才是「改表不改生成物即红」的完整判据。
package deploy

import (
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"testing"

	"forklift-training/internal/codegen"
	"forklift-training/internal/filestore"
)

// deliveryLinePat 解析生成区里的一行映射：`~*\.ext$  inline;` 与 `default  attachment;`。
var deliveryLinePat = regexp.MustCompile(`^\s*(?:~\*\\.([a-z0-9]+)\$|default)\s+(\w+);$`)

// TestNginxDeliveryMapInSync 生成链锁（#1364 判据 ①）：nginx-host.conf 的生成区必须与
// 「由 fileTypeTable 现算的结果」字节级全等。改表不重生成、或手改生成区，本测试即红。
func TestNginxDeliveryMapInSync(t *testing.T) {
	codegen.AssertInSync(t, NginxDeliveryMapGen)
}

// TestNginxDeliveryMapCoversFileTypeTable 逐档一致锁：生成物里 safe/unsafe/unknown 三档
// 必须与类型表**逐条**对上，且双向都不许多、不少、不改档。
//
// 为什么不能只靠上面那条全等锁：全等锁比的是「现算 vs 磁盘」，如果现算本身漏了表里一行
// （解析形态漂移、某一档被静默摘掉），两条锁会一起绿 —— 那正是 ADR-0066 决策 6 要防的
// 「新增类型没被逼着表态」。这里独立取一次表（AST + 运行期两侧对账），再与生成物逐条比。
func TestNginxDeliveryMapCoversFileTypeTable(t *testing.T) {
	rows, err := fileTypeTableRows()
	if err != nil {
		t.Fatalf("读类型表失败: %v", err)
	}
	region, err := RenderNginxDeliveryRegion()
	if err != nil {
		t.Fatalf("渲染生成区失败: %v", err)
	}

	got := map[string]string{}
	for _, line := range strings.Split(region, "\n") {
		m := deliveryLinePat.FindStringSubmatch(line)
		if m == nil {
			continue
		}
		key := m[1]
		if key == "" {
			key = "default" // map 的兜底档，不对应表里任何一行
		}
		if _, dup := got[key]; dup {
			t.Fatalf("生成区里 %q 出现两次（nginx 会判 duplicate map）", key)
		}
		got[key] = m[2]
	}
	if got["default"] != "attachment" {
		t.Fatalf("map 的 default 必须是 attachment（未登记扩展名 = unknown = 强制下载，ADR-0066 决策 6 锁②），实得 %q", got["default"])
	}
	delete(got, "default")

	// 正向：表里每一行都必须在生成区里，且档位正确。
	wantByClass := map[filestore.FileTypeClass]int{}
	for _, r := range rows {
		wantByClass[r.class]++
		want := deliveryOf(r.class)
		line, ok := got[r.ext]
		if !ok {
			t.Errorf("生成区缺扩展名 %q（表里是 %v 档 ⇒ 应为 %q）：漏一行就等于让它静默落到 default", r.ext, r.class, want)
			continue
		}
		if line != want {
			t.Errorf("生成区 %q = %q，表里是 %v 档 ⇒ 应为 %q", r.ext, line, r.class, want)
		}
		delete(got, r.ext)
	}
	// 反向：生成区不得有表里没有的键 —— 那就是在 nginx 手抄了第二份规则。
	for ext, v := range got {
		t.Errorf("生成区多出扩展名 %q（值 %q），类型表里没有这一行：规则必须只在表里写一次", ext, v)
	}

	// 三档都要有内容：某档解析成 0 行说明解析失配，不许静默产出一份缺档的 map。
	t.Logf("投递分档：safe %d 项 / unsafe %d 项 / unknown 显式登记 %d 项（另有 default 兜底）",
		wantByClass[filestore.FileTypeSafe], wantByClass[filestore.FileTypeUnsafe], wantByClass[filestore.FileTypeUnknown])
	for cls, name := range map[filestore.FileTypeClass]string{
		filestore.FileTypeSafe:   "safe",
		filestore.FileTypeUnsafe: "unsafe",
	} {
		if wantByClass[cls] == 0 {
			t.Errorf("类型表里 %s 档解析到 0 行：生成器与表已不同形（本锁需要跟着改，不是放宽）", name)
		}
	}
	if n := strings.Count(region, "~*\\."); n != len(rows) {
		t.Fatalf("生成区的映射行数 %d ≠ 类型表行数 %d", n, len(rows))
	}
}

// TestNginxDeliveryMapPropagation 传播方向锁：表里新增/改档必须体现在产物里。
//
// 这条不是形式主义 —— 上面两条锁都比的是「同一棵表现算两次」，要证明它们真的能因**表的变化**
// 而红，得喂一份合成行集合（不许去动只读的类型表）。断言三件事：新增 safe 行 → 出现 inline；
// 新增 unsafe 行 → 出现 attachment；空档不输出抬头（不留悬空注释）。
func TestNginxDeliveryMapPropagation(t *testing.T) {
	region, err := renderDeliveryRegion([]deliveryRow{
		{"tif", filestore.FileTypeSafe},
		{"exe", filestore.FileTypeUnsafe},
		{"bin", filestore.FileTypeUnknown},
	})
	if err != nil {
		t.Fatalf("渲染合成表失败: %v", err)
	}
	for _, want := range []string{"~*\\.tif$            inline;", "~*\\.exe$            attachment;", "~*\\.bin$            attachment;"} {
		if !strings.Contains(region, want) {
			t.Errorf("合成表新增行没进生成物（缺 %q）：说明生成器不认识这一档\n--- 生成物 ---\n%s", want, region)
		}
	}
	if !strings.Contains(region, "safe 档：1 项") || !strings.Contains(region, "unsafe 档：1 项") || !strings.Contains(region, "unknown 档：1 项") {
		t.Errorf("三档抬头计数与合成表不符:\n%s", region)
	}
	// 确定性：同一份表两次渲染必须逐字节相同（否则 CI 的新鲜度比对会随机红）。
	again, err := renderDeliveryRegion([]deliveryRow{
		{"exe", filestore.FileTypeUnsafe},
		{"bin", filestore.FileTypeUnknown},
		{"tif", filestore.FileTypeSafe},
	})
	if err != nil {
		t.Fatalf("二次渲染失败: %v", err)
	}
	if again != region {
		t.Error("渲染不确定：入参顺序改变就产出了不同文本（生成物会每次再生成都产生 diff）")
	}

	// fail-closed：非法扩展名键（正则元字符）必须拒绝生成，不能原样拼进 nginx 正则。
	if _, err := renderDeliveryRegion([]deliveryRow{{"tar.gz", filestore.FileTypeSafe}}); err == nil {
		t.Error("含正则元字符的扩展名键没被拒绝：会被原样拼进 nginx 正则键")
	}
}

// rgwDeliveryLocations 必须带投递分档头的 location 起始行（票 #1364 指定的两个 RGW 出口）。
// 匹配用**整行字面量**：改了 location 的正则（例如新增顶层前缀）就要同步改这里，
// 而「新增前缀忘了登记」正是 #517 那次的复发形态（见本文件开头注释与 nginx-host.conf 注 3）。
var rgwDeliveryLocations = []string{
	`    location ~ ^/(featured|avatars|contributions|resumes|images/(forum|questions|chapters|ai-assistant)|chapters|slides|uploads)/ {`,
	`    location ~ ^/reports/ {`,
}

// rgwKnownUncovered 已实测存在、但**不在本票射程**的 RGW 出口（键 = location 起始行，值 = 理由）。
// 登记而非忽略：新增第三个直出 RGW 的 location 时本锁判红，必须要么纳入分档、要么在此登记理由。
var rgwKnownUncovered = map[string]string{
	"    location /kbimg/ {": "dify.gccsmile.com 的知识库图片反代（同一 bucket，但票 #1364 的口径是 www 的两个 RGW location；" +
		"纳入需另立票并补它自己的真实 curl 判据）",
}

// TestNginxDeliveryWiring 接线锁：生成区必须在 http 上下文（所有 server 块之前），
// 且每个 RGW 出口都真的用上 $delivery_disposition + nosniff。
//
// 摘掉消费者而留下 map 是「看起来做了、其实没生效」的形态（先例：deploy/env.defaults 曾零消费者），
// 所以这条锁独立于全等锁存在。
func TestNginxDeliveryWiring(t *testing.T) {
	text := readNginxHostConf(t)

	start := strings.Index(text, nginxDeliveryStart)
	firstServer := indexOfFirstServerBlock(t, text)
	if start < 0 {
		t.Fatalf("生成区标记缺失：nginx-host.conf 里找不到起标记")
	}
	if start > firstServer {
		t.Errorf("生成区落在第一个 server 块之后（%d > %d）：nginx 的 map 只能在 http 上下文声明，起在 server 里 nginx -t 直接判失败",
			start, firstServer)
	}

	dispositions := locationsWith(t, text, "add_header Content-Disposition $delivery_disposition always;")
	nosniffs := locationsWith(t, text, `add_header X-Content-Type-Options "nosniff" always;`)
	for _, want := range rgwDeliveryLocations {
		if !slices.Contains(dispositions, want) {
			t.Errorf("RGW 出口缺 Content-Disposition：%s", want)
		}
		if !slices.Contains(nosniffs, want) {
			t.Errorf("RGW 出口缺 X-Content-Type-Options: nosniff：%s", want)
		}
	}
	// 分档头不许出现在别处：写死的第二份 disposition（不来自 map）就是手抄规则。
	for _, loc := range dispositions {
		if !slices.Contains(rgwDeliveryLocations, loc) {
			t.Errorf("location %q 里出现 Content-Disposition 但不在登记的两个 RGW 出口内：分档值只能由 map 给出", loc)
		}
	}
	// 决策 3：Content-Type 刻意不覆写。
	for i, line := range strings.Split(text, "\n") {
		if strings.Contains(strings.TrimSpace(line), "proxy_hide_header Content-Type") {
			t.Errorf("nginx-host.conf:%d 覆写了 Content-Type（#1364 决策 3 明确不做）：%s", i+1, strings.TrimSpace(line))
		}
	}

	// 每一个反代到 RGW 的 location：要么已纳入分档，要么在登记表里写明理由。
	blocks := parseLocationBlocks(text)
	if len(blocks) == 0 {
		t.Fatal("一个 location 都没解析到：nginx-host.conf 的结构变了，本锁已失效")
	}
	covered := 0
	for _, b := range blocks {
		if !strings.Contains(b.header, "172.17.1.42:8088") && !strings.Contains(b.body, "172.17.1.42:8088") {
			continue
		}
		if slices.Contains(rgwDeliveryLocations, b.header) {
			covered++
			continue
		}
		if reason, ok := rgwKnownUncovered[b.header]; ok {
			t.Logf("已登记的未纳入 RGW 出口：%s —— %s", strings.TrimSpace(b.header), reason)
			continue
		}
		t.Errorf("新增的 RGW 直出出口既没纳入投递分档、也没在 rgwKnownUncovered 登记理由：%s", strings.TrimSpace(b.header))
	}
	if covered != len(rgwDeliveryLocations) {
		t.Fatalf("登记的两个 RGW 出口里只解析到 %d 个（location 起始行改了？两个都要更新）", covered)
	}
}

// TestNginxHostConfStructure 结构核对 —— 本机无 nginx 时 `nginx -t` 的静态替身。
//
// 只做三件机器能判的事：大括号配平、生成区每行的语法形态、map 块的键唯一。
// **这不等于跑过 nginx -t**：指令级合法性（unknown variable、duplicate directive 之类）
// 只能在容器里由 frontend/docker-entrypoint.sh 的 `nginx -t` 判，见票面待验证项。
func TestNginxHostConfStructure(t *testing.T) {
	text := readNginxHostConf(t)
	body := stripConfComments(text)
	if n := strings.Count(body, "{") - strings.Count(body, "}"); n != 0 {
		t.Errorf("大括号不配平（多 %d 个 {）：nginx 会报 unexpected end of file", n)
	}

	region := mustRegion(t, text)
	inMap := false
	seen := map[string]bool{}
	for i, line := range strings.Split(region, "\n") {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			continue
		}
		if trimmed == "map $uri $delivery_disposition {" {
			inMap = true
			continue
		}
		if trimmed == "}" {
			if !inMap {
				t.Errorf("生成区第 %d 行出现孤立 }", i+1)
			}
			inMap = false
			continue
		}
		if !inMap {
			t.Errorf("生成区第 %d 行既不在 map 块内也不是 map 块本身：%q", i+1, trimmed)
			continue
		}
		m := deliveryLinePat.FindStringSubmatch(trimmed)
		if m == nil {
			t.Errorf("map 条目语法不符（应形如 `~*\\.ext$  inline;` 或 `default  attachment;`）：%q", trimmed)
			continue
		}
		if m[2] != "inline" && m[2] != "attachment" {
			t.Errorf("map 取值只允许 inline / attachment，实得 %q（%q）", m[2], trimmed)
		}
		key := m[1] + "|" + m[2]
		if seen[trimmed] {
			t.Errorf("map 条目重复（nginx 判 duplicate map）：%q（键 %s）", trimmed, key)
		}
		seen[trimmed] = true
	}
	if inMap {
		t.Error("map 块没有闭合 }")
	}
}

// readNginxHostConf 读产物文本（LF 归一，与 Render 同口径）。
// 定位从测试工作目录（backend/internal/deploy）逐级向上，判据与生成器同一个
// locateNginxHostConf —— 不在测试里手写 `../../../` 这类相对路径（ADR-0053 §9）。
func readNginxHostConf(t *testing.T) string {
	t.Helper()
	dir := mustCwd(t)
	var path string
	for {
		if p, ok := locateNginxHostConf(dir); ok {
			path = p
			break
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatalf("从 %s 逐级向上没找到 %s", mustCwd(t), nginxHostConfRel)
		}
		dir = parent
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("读取 %s 失败: %v", path, err)
	}
	return strings.ReplaceAll(string(raw), "\r\n", "\n")
}

// mustCwd 当前工作目录。
func mustCwd(t *testing.T) string {
	t.Helper()
	dir, err := os.Getwd()
	if err != nil {
		t.Fatalf("取当前目录失败: %v", err)
	}
	return dir
}

// mustRegion 取出生成区正文（两行标记之间）。
func mustRegion(t *testing.T, text string) string {
	t.Helper()
	i := strings.Index(text, nginxDeliveryStart)
	j := strings.Index(text, nginxDeliveryEnd)
	if i < 0 || j < 0 || j < i {
		t.Fatalf("生成区标记缺失或顺序颠倒（起 %d / 止 %d）", i, j)
	}
	return text[i+len(nginxDeliveryStart) : j]
}

// locBlock 一个 location 块：起始行（含缩进的原样文本）与花括号内的正文。
type locBlock struct {
	header string
	body   string
}

// parseLocationBlocks 按行扫 location 块并做花括号配对。
//
// 只认「行首（可缩进）是 location …{」的形态，够用且 fail-visible：
// 解析不到任何块时调用方判红，不会静默通过。
func parseLocationBlocks(text string) []locBlock {
	var out []locBlock
	lines := strings.Split(text, "\n")
	for i, raw := range lines {
		trimmed := strings.TrimSpace(raw)
		if !strings.HasPrefix(trimmed, "location ") || !strings.HasSuffix(trimmed, "{") {
			continue
		}
		depth := 0
		var body []string
		for j := i; j < len(lines); j++ {
			line := stripConfComment(lines[j])
			depth += strings.Count(line, "{") - strings.Count(line, "}")
			if j > i {
				body = append(body, line)
			}
			if depth <= 0 {
				out = append(out, locBlock{header: raw, body: strings.Join(body, "\n")})
				break
			}
		}
	}
	return out
}

// locationsWith 返回正文里包含 want 的所有 location 起始行。
func locationsWith(t *testing.T, text, want string) []string {
	t.Helper()
	var hits []string
	for _, b := range parseLocationBlocks(text) {
		if strings.Contains(b.body, want) {
			hits = append(hits, b.header)
		}
	}
	return hits
}

// indexOfFirstServerBlock 第一个 server 块的起始位置（字节偏移）。
func indexOfFirstServerBlock(t *testing.T, text string) int {
	t.Helper()
	i := strings.Index(text, "\nserver {\n")
	if i < 0 {
		t.Fatal("找不到 server 块：nginx-host.conf 的结构变了，本锁已失效")
	}
	return i
}

// stripConfComments 去掉整行注释（用于括号配平统计）。
func stripConfComments(text string) string {
	var out []string
	for _, line := range strings.Split(text, "\n") {
		out = append(out, stripConfComment(line))
	}
	return strings.Join(out, "\n")
}

// stripConfComment 行内注释只在「行首是 #」时剥离：本文件没有行尾注释，
// 而 `#` 出现在别处（如注释里的「#1364」）时不该被当成指令的一部分。
func stripConfComment(line string) string {
	if strings.HasPrefix(strings.TrimSpace(line), "#") {
		return ""
	}
	return line
}

// 生成区标记的字面量守卫：标记文本同时被 spliceRegion 与配置里的注释引用，
// 一旦其中一处漂移，全等锁会红；这里提前给出人话版错误。
func TestNginxDeliveryMarkers(t *testing.T) {
	text := readNginxHostConf(t)
	for _, m := range []string{nginxDeliveryStart, nginxDeliveryEnd} {
		if n := strings.Count(text, m); n != 1 {
			t.Errorf("标记在产物里出现 %d 次（必须恰好 1 次）：%s", n, m)
		}
	}
	if head := strings.TrimPrefix(mustRegion(t, text), "\n"); !strings.HasPrefix(head, "# ===== RGW 直出的投递分档") {
		t.Errorf("生成区正文抬头变了：请同步 RenderNginxDeliveryRegion（实得 %q）", firstLineOf(head))
	}
}

// firstLineOf 文本首行（报错只打一行，别把整段生成物灌进测试输出）。
func firstLineOf(s string) string {
	if i := strings.IndexByte(s, '\n'); i >= 0 {
		return s[:i]
	}
	return s
}
