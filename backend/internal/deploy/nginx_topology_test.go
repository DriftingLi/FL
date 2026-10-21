package deploy

// 主机职责锁（票 #1742）：官网门户迁到 EdgeOne 后，www 只承担「页面 301 + API」，
// 对象存储直出搬到独立资源域 cdn。本文件把那次搬迁的三条外部事实钉住：
//   1. cdn 块只出对象存储、只读、未知路径 404（不是一个能写、能吐 SPA 的新入口）；
//   2. www 块把页面与资源分别 301 到门户与 cdn，且不再指向已撤走的门户容器；
//   3. 模板里的 ${VAR} 占位符集合与容器 entrypoint 的 envsubst 清单严格一致
//      —— 「删变量时漏改一处」正是这次要根治的事故类型。
//
// 判据全部读**真实产物**（frontend/nginx-host.conf 与其 entrypoint），不读生成器中间值：
// 与同包 nginx_delivery_gen_test.go 的口径一致（结构锁是 nginx -t 的静态替身）。

import (
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"testing"
)

const (
	// portalOrigin www 上页面路径 301 的目标（新站固定主域，不随 DOMAIN 派生：
	// 旧域只服务旧主域，新站是外部事实）。
	portalOrigin = "https://www.hrwai.cn"
	// assetsHostTemplate 资源域在模板里的写法（与 training.${DOMAIN} 等既有写法同构）。
	assetsHostTemplate = "cdn.${DOMAIN}"
)

// serverBlock 一个 server 块：起始行原样 + 花括号内正文。
type serverBlock struct {
	header string
	body   string
}

// parseServerBlocks 按行扫 server 块并做花括号配对（与 parseLocationBlocks 同构）。
func parseServerBlocks(text string) []serverBlock {
	var out []serverBlock
	lines := strings.Split(text, "\n")
	for i, raw := range lines {
		if !strings.HasPrefix(strings.TrimSpace(raw), "server {") {
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
				out = append(out, serverBlock{header: raw, body: strings.Join(body, "\n")})
				break
			}
		}
	}
	return out
}

// blockByServerName 取 server_name 命中 want 的 server 块（模板占位符原样匹配）。
func blockByServerName(t *testing.T, text, want string) serverBlock {
	t.Helper()
	var hits []serverBlock
	for _, b := range parseServerBlocks(text) {
		for _, line := range strings.Split(b.body, "\n") {
			trimmed := strings.TrimSpace(line)
			if !strings.HasPrefix(trimmed, "server_name ") || !strings.HasSuffix(trimmed, ";") {
				continue
			}
			// 一个块可以挂多个名字（如 www 同时收 EdgeOne 回源 Host）：
			// 按 token 比而不是整行比，否则想加别名时锁会先红。
			names := strings.Fields(strings.TrimSuffix(strings.TrimPrefix(trimmed, "server_name "), ";"))
			for _, n := range names {
				if n == want {
					hits = append(hits, b)
					break
				}
			}
		}
	}
	if len(hits) == 0 {
		t.Fatalf("找不到 server_name %s 的 server 块", want)
	}
	if len(hits) > 1 {
		t.Fatalf("server_name %s 出现 %d 次（同一职责只能有一个块）", want, len(hits))
	}
	return hits[0]
}

// TestNginxCdnHostIsAssetOnly 资源域的形态锁：只出对象存储、只读、其余 404。
func TestNginxCdnHostIsAssetOnly(t *testing.T) {
	text := readNginxHostConf(t)
	cdn := blockByServerName(t, text, assetsHostTemplate)

	for _, want := range rgwDeliveryLocations {
		if !strings.Contains(cdn.body, want) {
			t.Errorf("cdn 块里没有对象存储出口：%s", want)
		}
	}
	if !strings.Contains(cdn.body, "location ~ ^/reports/ {") {
		t.Error("cdn 块里没有 /reports/ 出口：报告 PDF 会 404")
	}

	// 只读：cdn 块的每个 location（两个直出 + 404 兜底）都要带 limit_except。
	// 不数总数是为了让「将来给兜底加限制」或「新增一个直出」都不会被硬计数误判。
	cdnLocs := parseLocationBlocks(cdn.body)
	if len(cdnLocs) != 3 {
		t.Fatalf("cdn 块的 location 数量 = %d，期望 3（两个直出 + 404 兜底）", len(cdnLocs))
	}
	for _, b := range cdnLocs {
		if !strings.Contains(b.body, "limit_except GET HEAD { deny all; }") {
			t.Errorf("cdn 的 location 缺少只读限制（非 GET/HEAD 应被拒）：%s", strings.TrimSpace(b.header))
		}
	}
	// 未知路径 404，而不是落到默认块的 SPA（返回 200 会伪装成「配好了」）。
	if !strings.Contains(cdn.body, "return 404;") {
		t.Error("cdn 块没有 return 404 兜底：漏配的前缀会伪装成 200")
	}
	// 资源域不得成为第二个 API 入口。
	if strings.Contains(cdn.body, "proxy_pass http://127.0.0.1") {
		t.Error("cdn 块里出现了到后端的反代：资源域不该承担 API 职责")
	}
	if strings.Contains(cdn.body, "try_files") {
		t.Error("cdn 块里出现了 try_files：资源域不该吐 SPA 首页")
	}
}

// TestNginxWwwBlockIsRedirectsAndAPI www 的职责锁：页面 301 到门户、资源 301 到 cdn、
// API 与探活原地不动、不再指向已撤走的门户容器。
func TestNginxWwwBlockIsRedirectsAndAPI(t *testing.T) {
	text := readNginxHostConf(t)
	www := blockByServerName(t, text, "www.${DOMAIN}")

	if strings.Contains(text, "PORTAL_HOST_PORT") {
		t.Error("全文仍引用 PORTAL_HOST_PORT：门户容器迁走后这是空指向")
	}

	// 页面兜底：301 到新站同一路径（查询串由 $request_uri 带过去）。
	if !strings.Contains(www.body, "return 301 "+portalOrigin+"$request_uri;") {
		t.Errorf("www 的页面兜底不是 301 到 %s$request_uri", portalOrigin)
	}
	// 资源前缀：301 到资源域，老外链不碎。
	if !strings.Contains(www.body, "location ~ ^/(featured|avatars|") {
		t.Error("www 块里没有对象存储前缀的 301 出口：旧图片外链会落进页面兜底")
	}
	if !strings.Contains(www.body, "return 301 https://"+assetsHostTemplate+"$request_uri;") {
		t.Errorf("www 的对象存储前缀没有 301 到 %s", assetsHostTemplate)
	}
	// 报告同样跟着搬。
	if !strings.Contains(www.body, "location ~ ^/reports/ {") {
		t.Error("www 块里没有 /reports/ 的出口（301 或直出）")
	}

	// 保留面：API、Swagger、探活、老 AI 助手链接。
	if !strings.Contains(www.body, "location /api/ {") ||
		!strings.Contains(www.body, "proxy_pass http://127.0.0.1:${BACKEND_HOST_PORT};") {
		t.Error("www 块丢了 /api/ 反代：新站经 EdgeOne 回源就靠它")
	}
	if !strings.Contains(www.body, "location ^~ /swagger {") {
		t.Error("www 块丢了 /swagger 反代")
	}
	if !strings.Contains(www.body, "location = /health {") {
		t.Error("www 块缺 /health 探活：页面兜底会把外部监控的探活变成 301")
	}
	if !strings.Contains(www.body, "location = /ai-assistant {") {
		t.Error("www 块缺 /ai-assistant 的精确匹配：老链接会被页面兜底带去新站")
	}
	// EdgeOne 回源若带着新站主域做 Host，也要落进本块（否则掉进 default_server 的 SPA，
	// 返回 200 + 首页）；而本块对页面路径的兜底是「301 到新站」，同域回源必须改成 404 防自跳成环。
	if !strings.Contains(www.body, "www.hrwai.cn;") {
		t.Error("www 块的 server_name 少了 EdgeOne 回源 Host（www.hrwai.cn）")
	}
	if !strings.Contains(www.body, "if ($host = www.hrwai.cn)") || !strings.Contains(www.body, "return 404;") {
		t.Error("www 的页面兜底缺「回源 Host = 新站主域时直接 404」的护栏：会 301 到自己成环")
	}
	// 301 必须在 location 里：server 级 return 会在 rewrite 阶段短路所有 location
	// （历史 issue #496，且会把 /api/ 与资源出口一起打掉）。
	// 按花括号深度判「server 级」：depth 0 的 return 会短路所有 location（issue #496）。
	depth := 0
	for _, line := range strings.Split(www.body, "\n") {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "#") {
			continue
		}
		if depth == 0 && strings.HasPrefix(trimmed, "return 301 "+portalOrigin) {
			t.Error("www 块在 server 级出现 301：会短路 /api/ 与资源出口（issue #496）")
		}
		depth += strings.Count(line, "{") - strings.Count(line, "}")
	}
}

// TestNginxAssetPrefixesMatchOnBothHosts 同一批存储前缀在 www（301）与 cdn（直出）里必须逐字同串。
// 新增前缀只改一处时，另一侧会把它落进页面兜底或 404 —— 链接看着「能用」但图打不开，
// 这类漏登记正是 #517 的复发形态。
func TestNginxAssetPrefixesMatchOnBothHosts(t *testing.T) {
	text := readNginxHostConf(t)
	cdn := blockByServerName(t, text, assetsHostTemplate)
	www := blockByServerName(t, text, "www.${DOMAIN}")

	assetHeaders := func(body string) []string {
		var out []string
		for _, b := range parseLocationBlocks(body) {
			trimmed := strings.TrimSpace(b.header)
			if strings.Contains(trimmed, "featured|avatars") || strings.HasPrefix(trimmed, "location ~ ^/reports/") {
				out = append(out, b.header)
			}
		}
		return out
	}
	cdnHeaders, wwwHeaders := assetHeaders(cdn.body), assetHeaders(www.body)
	if len(cdnHeaders) != 2 || len(wwwHeaders) != 2 {
		t.Fatalf("存储前缀 location 数量不对：cdn %d / www %d（期望各 2：对象存储前缀 + reports）",
			len(cdnHeaders), len(wwwHeaders))
	}
	for i := range cdnHeaders {
		if cdnHeaders[i] != wwwHeaders[i] {
			t.Errorf("同一前缀在两台的写法不一致（新增前缀时只改一处会静默漏登记）：\n  cdn = %q\n  www = %q",
				cdnHeaders[i], wwwHeaders[i])
		}
	}
}

// TestNginxRootDomainRedirectsToPortal 裸域一跳直达新站（不再经 www 二次跳）。
func TestNginxRootDomainRedirectsToPortal(t *testing.T) {
	text := readNginxHostConf(t)
	root := blockByServerName(t, text, "${DOMAIN}")
	if !strings.Contains(root.body, "return 301 "+portalOrigin+"$request_uri;") {
		t.Errorf("裸域没有直接 301 到 %s", portalOrigin)
	}
}

// TestNginxPlaceholdersMatchEnvsubst 占位符契约：模板里出现的 ${VAR} 集合必须与
// entrypoint 的 envsubst 清单严格相等。少替换一处 = 容器里 nginx -t 失败或线上空指向。
func TestNginxPlaceholdersMatchEnvsubst(t *testing.T) {
	template := stripConfComments(readNginxHostConf(t))
	used := placeholderNames(template)

	script := readEntrypoint(t)
	want := envsubstNames(t, script)

	if strings.Join(used, ",") != strings.Join(want, ",") {
		t.Fatalf("模板占位符与 envsubst 清单不一致：\n  模板  = %v\n  envsubst = %v\n"+
			"（模板多出来的会在容器里留成字面量，清单多出来的只是噪音；两边必须同源）", used, want)
	}
	// 门户端口随容器一起撤走，脚本里不该再提。
	if strings.Contains(script, "PORTAL_HOST_PORT") {
		t.Error("docker-entrypoint.sh 仍处理 PORTAL_HOST_PORT")
	}
}

// placeholderNames 模板里 ${VAR} 形式的占位符（去重升序）。
func placeholderNames(text string) []string {
	re := regexp.MustCompile(`\$\{[A-Z_][A-Z0-9_]*\}`)
	seen := map[string]bool{}
	var out []string
	for _, m := range re.FindAllString(text, -1) {
		name := strings.TrimSuffix(strings.TrimPrefix(m, "${"), "}")
		if !seen[name] {
			seen[name] = true
			out = append(out, name)
		}
	}
	sort.Strings(out)
	return out
}

// envsubstNames entrypoint 里 envsubst 单引号参数中的变量名（去重升序）。
func envsubstNames(t *testing.T, script string) []string {
	t.Helper()
	re := regexp.MustCompile(`envsubst '([^']*)'`)
	m := re.FindStringSubmatch(script)
	if m == nil {
		t.Fatal("entrypoint 里找不到 envsubst '...' 调用：替换清单的判据变了，本锁需要跟着改")
	}
	seen := map[string]bool{}
	var out []string
	for _, name := range placeholderNames(m[1]) {
		if !seen[name] {
			seen[name] = true
			out = append(out, name)
		}
	}
	sort.Strings(out)
	return out
}

// readEntrypoint 读容器 entrypoint（从测试工作目录逐级向上定位，不手写相对层级）。
func readEntrypoint(t *testing.T) string {
	t.Helper()
	dir := mustCwd(t)
	for {
		candidate := filepath.Join(dir, "frontend", "docker-entrypoint.sh")
		if raw, err := os.ReadFile(candidate); err == nil {
			return strings.ReplaceAll(string(raw), "\r\n", "\n")
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatalf("从 %s 逐级向上没找到 frontend/docker-entrypoint.sh", mustCwd(t))
		}
		dir = parent
	}
}
