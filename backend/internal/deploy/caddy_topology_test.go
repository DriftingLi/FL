// 本文件：反代层（Caddy）的结构锁 —— 它是 `caddy validate` 的静态替身 + 部署面接线锁。
//
// 判据读的都是**真实产物**（frontend/Caddyfile、frontend/docker-entrypoint.sh、
// docker-compose.prod.yml、.github/workflows/cd.yml），不读生成器中间值：
// 结构锁要防的正是「产物被改坏了而生成器还很开心」。
//
// 这批锁从 nginx 时代原样搬过来（nginx_topology_test.go / nginx_edgeone_test.go），
// 保持同一组判据，只换语法。
package deploy

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// readRepoFile 读仓库内文件并归一成 LF。
func readRepoFile(t *testing.T, rel string) string {
	t.Helper()
	root, err := repoRoot()
	if err != nil {
		t.Fatalf("定位仓库根失败: %v", err)
	}
	raw, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(rel)))
	if err != nil {
		t.Fatalf("读 %s 失败: %v", rel, err)
	}
	return strings.ReplaceAll(string(raw), "\r\n", "\n")
}

// stripComments 去掉整行注释与行尾注释（注释里的域名/占位符不是配置事实，
// 例如「本块将来 301 到 https://valuation.hrwai.cn」这样的说明不该被锁判成引用）。
func stripComments(src string) string {
	var out []string
	for _, line := range strings.Split(src, "\n") {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "#") {
			continue
		}
		if i := strings.Index(line, " #"); i >= 0 {
			line = line[:i]
		}
		out = append(out, line)
	}
	return strings.Join(out, "\n")
}

// blockAt 取出以 `addr {` 开头的那一段（按花括号配对，含首尾行）。
// 找不到时返回空串，由调用方判红。
func blockAt(src, addr string) string {
	lines := strings.Split(src, "\n")
	start := -1
	for i, line := range lines {
		if strings.HasPrefix(strings.TrimSpace(line), addr+" {") {
			start = i
			break
		}
	}
	if start < 0 {
		return ""
	}
	depth := 0
	for i := start; i < len(lines); i++ {
		depth += strings.Count(lines[i], "{") - strings.Count(lines[i], "}")
		if depth == 0 {
			return strings.Join(lines[start:i+1], "\n")
		}
	}
	return ""
}

// TestCaddyfileHostMatrixLock 主机矩阵锁：终态该有的站点一个不少、一个不多。
// 少一个 ⇒ 那个子域落到「无匹配站点」返回 404（而且是上线后才发现）；
// 多一个 ⇒ 某个已退役的子域还在服务。
func TestCaddyfileHostMatrixLock(t *testing.T) {
	src := stripComments(readRepoFile(t, "frontend/Caddyfile"))
	want := []string{
		"api.{$DOMAIN}",
		"training.{$DOMAIN}",
		"mentor.{$DOMAIN}",
		"manage.{$DOMAIN}",
		"recruit.{$DOMAIN}",
		"valuation.{$DOMAIN}",
		"cdn.{$DOMAIN}",
		"dify.{$DOMAIN}",
	}
	for _, addr := range want {
		if got := blockAt(src, addr); got == "" {
			t.Errorf("Caddyfile 缺少站点块：%s", addr)
		}
	}
	// 根域与 www 用逗号合并写法，单独断言。
	if !strings.Contains(src, "gdyidao.cn, www.gdyidao.cn {") {
		t.Error("Caddyfile 缺少「根域 + www」的合并站点块")
	}
	// 旧域只在**过渡块**（门户回源兼容）里允许出现，且该块必须只服务 API。
	legacy := blockAt(src, "www.gccsmile.com")
	if legacy == "" {
		t.Log("未发现过渡期的门户回源兼容块（EdgeOne 已切走 ⇒ 正常）")
	} else {
		for _, forbidden := range []string{"file_server", "try_files", "redir"} {
			if strings.Contains(legacy, forbidden) {
				t.Errorf("门户回源兼容块里出现了 %s：它只该服务 /api、/swagger、/health", forbidden)
			}
		}
		if !strings.Contains(legacy, "@legacy_api path /api/* /swagger*") {
			t.Error("门户回源兼容块的放行集合被改过：必须恰好是 /api/* 与 /swagger*（/health 由独立 handle 返回 ok）")
		}
	}
	// 除过渡块外，旧域与和润域都不得出现（R2 硬切：切完即删）。
	rest := strings.Replace(src, legacy, "", 1)
	for _, retired := range []string{"gccsmile", "hrwai.cn"} {
		if strings.Contains(rest, retired) {
			t.Errorf("Caddyfile 里还有已退役域名 %q 的引用（hrwai.cn 已由 EdgeOne 承载，与本机无关）", retired)
		}
	}
}

// TestCaddyfilePlaceholderLock 占位符集合锁：允许出现的环境变量只有六个。
// 多一个 = 有人往配置里塞了一个没人 export 的变量（展开成空串，故障静默）；
// 少一个则无所谓（本锁只禁「未登记的名字」）。
func TestCaddyfilePlaceholderLock(t *testing.T) {
	src := stripComments(readRepoFile(t, "frontend/Caddyfile"))
	allowed := map[string]bool{
		"DOMAIN":            true,
		"BACKEND_HOST_PORT": true,
		"EDGE_ORIGIN_TOKEN": true,
		"CADDY_GLOBAL":      true,
		"CADDY_TLS":         true,
		"CADDY_ACME_LINE":   true,
	}
	re := regexp.MustCompile(`\{\$([A-Z0-9_]+)\}`)
	seen := map[string]bool{}
	for _, m := range re.FindAllStringSubmatch(src, -1) {
		seen[m[1]] = true
	}
	for name := range seen {
		if !allowed[name] {
			t.Errorf("Caddyfile 引用了未登记的环境变量 {$%s}：要么登记到本锁的白名单，要么改用已有的变量", name)
		}
	}
	// 反向：五个核心变量必须真的被用到，否则说明某段配置被整块删掉了。
	for _, must := range []string{"DOMAIN", "BACKEND_HOST_PORT", "CADDY_TLS"} {
		if !seen[must] {
			t.Errorf("Caddyfile 里找不到 {$%s}：站点地址或后端端口没有跟着环境走", must)
		}
	}
}

// TestCaddyEdgeOriginTokenLock 回源鉴权锁（票 #1746 的语义，nginx 时代原样搬来）：
//  1. 命中 token 的请求走单独一支，X-Forwarded-For **只给 EO-Connecting-IP**（不追加对端）——
//     追加会让后端把边缘节点当成客户端 IP（限流键聚合、访问日志失真）；
//  2. 未命中走默认反代（Caddy 自带的「追加对端」语义）；
//  3. token 为空时入口脚本必须拒绝启动（fail closed），而不是退化成「空头匹配空 token」。
func TestCaddyEdgeOriginTokenLock(t *testing.T) {
	src := readRepoFile(t, "frontend/Caddyfile")
	if n := strings.Count(src, "@from_edge header X-Origin-Token {$EDGE_ORIGIN_TOKEN}"); n != 1 {
		t.Errorf("回源鉴权的匹配器必须恰好出现 1 次，实得 %d 次", n)
	}
	if n := strings.Count(src, "header_up X-Forwarded-For {http.request.header.EO-Connecting-IP}"); n != 1 {
		t.Errorf("EO-Connecting-IP 必须恰好写一次且只给这一个值（不追加对端），实得 %d 次", n)
	}
	// 反向断言：不得出现「追加式」的 XFF 覆盖写法——那是本条锁要防的回归形态。
	if strings.Contains(src, "header_up X-Forwarded-For {http.request.header.X-Forwarded-For}") {
		t.Error("出现了显式透传 X-Forwarded-For 的写法：会绕过 Caddy 的追加语义，客户端可伪造前缀")
	}

	entry := readRepoFile(t, "frontend/docker-entrypoint.sh")
	if !strings.Contains(entry, "X-Origin-Token") {
		t.Error("entrypoint 没有检查 Caddyfile 是否启用了回源鉴权")
	}
	if !strings.Contains(entry, "EDGE_ORIGIN_TOKEN 为空") || !strings.Contains(entry, "exit 1") {
		t.Error("entrypoint 缺少「token 为空 ⇒ 拒绝启动」的 fail-closed 分支（静默降级就是伪造 IP 的入口）")
	}
}

// TestCaddyCDNScopeLock 资源域边界锁：cdn 只直出、不吐 SPA、未知路径 404。
// 判据落在真实产物上：cdn 块里不得出现 file_server / try_files（那是 SPA 的形态）。
func TestCaddyCDNScopeLock(t *testing.T) {
	src := stripComments(readRepoFile(t, "frontend/Caddyfile"))
	cdn := blockAt(src, "cdn.{$DOMAIN}")
	if cdn == "" {
		t.Fatal("找不到 cdn 站点块")
	}
	for _, forbidden := range []string{"file_server", "try_files"} {
		if strings.Contains(cdn, forbidden) {
			t.Errorf("cdn 块里出现了 %s：资源域不得吐 SPA（漏配的前缀必须立刻 404，而不是伪装成 200）", forbidden)
		}
	}
	if !strings.Contains(cdn, "404") || !strings.Contains(cdn, "respond") {
		t.Error("cdn 块缺少未知路径的 404 兜底")
	}
	// 2026-10-22 线上事故：`not path ...` 形式的反向匹配对所有路径都成立，
	// 结果所有请求被本站 404 拦掉，而 RGW 里对象其实存在（学员头像全废）。
	// 判据锁「正向匹配 @object_store + 独立兜底」这个形状本身。
	if !strings.Contains(cdn, "@object_store path") {
		t.Error("cdn 块必须用正向匹配 @object_store，而不是反向 not path（后者实测会把一切拦成 404）")
	}
	if strings.Contains(cdn, "not path") {
		t.Error("cdn 块里出现了 not path：反向匹配在本项目踩过线上事故，改用正向匹配 + 独立兜底")
	}
	// 写方法拦截必须落在 handle 内（链外会被互斥路由吞掉，#1759）。
	if !strings.Contains(cdn, "handle @write_methods {") {
		t.Error("写方法拦截必须在 handle 内：链外的 respond 会被后续 handle 吞掉，POST 会返回 404 而非 403")
	}
	if !strings.Contains(cdn, "@write_methods method POST PUT PATCH DELETE") {
		t.Error("cdn 块没有挡住写方法（资源域只允许 GET/HEAD）")
	}
}

// TestDeployChainUsesCaddy 接线锁：部署链必须真的用上这些产物，且旧证书机制不得复活。
// 少了这条，改坏 docker-compose 的挂载点、或有人把 SSL Secrets 加回来，上面所有锁都照绿。
func TestDeployChainUsesCaddy(t *testing.T) {
	compose := readRepoFile(t, "docker-compose.prod.yml")
	if !strings.Contains(compose, "./frontend/Caddyfile:/etc/caddy/Caddyfile:ro") {
		t.Error("compose 没有挂载 frontend/Caddyfile（容器里跑的还是镜像自带的旧配置）")
	}
	if !strings.Contains(compose, "caddy-data:/data") {
		t.Error("compose 没有挂 caddy-data 卷：证书会在容器重建时丢失、每次部署都重新签发并撞 ACME 配额")
	}
	if !strings.Contains(compose, "TLS_MODE:") {
		t.Error("compose 没有透传 TLS_MODE：测试环境会按生产策略去申请公共证书")
	}
	for _, dead := range []string{"nginx-host.conf", "SSL_CERT_DIR", "/etc/nginx/"} {
		if strings.Contains(compose, dead) {
			t.Errorf("compose 里还有 nginx 时代的残留：%s", dead)
		}
	}

	cd := readRepoFile(t, ".github/workflows/cd.yml")
	if !strings.Contains(cd, "frontend/Caddyfile") {
		t.Error("cd.yml 的打包清单没有带上 Caddyfile（容器里挂载点会指向不存在的文件）")
	}
	for _, dead := range []string{"SSL_FULLCHAIN", "SSL_PRIVKEY"} {
		if strings.Contains(cd, dead) {
			t.Errorf("cd.yml 里还有 %s：证书已由 Caddy 自签，这个 secret 不该再注入", dead)
		}
	}
	// TLS_MODE 必须同时出现在**两处写盘**里：cd.yml 生成 /tmp/deploy-env.sh，
	// deploy-remote.sh 把它写进 $DEPLOY_PATH/.env（compose 只读后者）。
	// 少一处的实测后果：testing 回落到 auto，去申请生产域名的公共证书 ⇒ 容器起不来、
	// 健康检查红、回滚也救不回来（2026-10-22 两次冒烟）。
	if !strings.Contains(cd, "TLS_MODE") {
		t.Error("cd.yml 没有把 TLS_MODE 写进远端环境变量文件：compose 会回落到 auto")
	}
	remote := readRepoFile(t, "scripts/deploy-remote.sh")
	if !strings.Contains(remote, "TLS_MODE=") {
		t.Error("deploy-remote.sh 没有把 TLS_MODE 写进 .env：远程 compose 读不到它")
	}
}
