package deploy

// EdgeOne 回源鉴权的接线锁（票 #1746）。
//
// 背景：门户（EdgeOne Pages）的 /api 代理回源时，nginx 看到的对端是边缘节点；
// EdgeOne 会用 EO-Connecting-IP 带来真实客户端 IP，而 X-Forwarded-For / X-Real-IP 都只是
// 边缘链路。无条件采信该头会被直连伪造 ⇒ 需要门户携带回源 token，只有 token 命中才认。
//
// 本文件钉住三件外部事实：
//   1. www 的两个后端入口用 $xff_for_backend（而不是无条件追加头）；
//   2. 三个 map 都在（token 校验 / 客户端 IP 取值 / 后端 XFF 选择），且 token 来自部署变量；
//   3. token 为空时入口脚本拒绝启动（fail closed：空键会匹配空头，等于信任所有人）。

import (
	"strings"
	"testing"
)

// locationIn 取正文里起始行逐字等于 header 的 location 块（找不到即 Fatal）。
func locationIn(t *testing.T, body, header string) locBlock {
	t.Helper()
	for _, b := range parseLocationBlocks(body) {
		if strings.TrimSpace(b.header) == header {
			return b
		}
	}
	t.Fatalf("找不到 location：%s", header)
	return locBlock{}
}

// TestNginxWwwForwardsEdgeClientIP www 经 EdgeOne 回源时，后端必须拿到真实客户端 IP。
func TestNginxWwwForwardsEdgeClientIP(t *testing.T) {
	text := readNginxHostConf(t)
	www := blockByServerName(t, text, "www.${DOMAIN}")

	for _, header := range []string{"location /api/ {", "location ^~ /swagger {"} {
		b := locationIn(t, www.body, header)
		if !strings.Contains(b.body, "proxy_set_header X-Forwarded-For $xff_for_backend;") {
			t.Errorf("%s 没有用 $xff_for_backend：EdgeOne 回源的真实客户端 IP 传不进去", header)
		}
		if strings.Contains(b.body, "$proxy_add_x_forwarded_for") {
			t.Errorf("%s 仍在用 $proxy_add_x_forwarded_for：后端会把边缘节点 IP 当客户端", header)
		}
	}

	// 三个 map：token 校验 → 客户端 IP 取值 → 后端 XFF 选择。
	if !strings.Contains(text, "map $http_x_origin_token $edge_origin_token_ok {") {
		t.Error("缺 token 校验 map：无法区分 EdgeOne 回源与直连伪造")
	}
	if !strings.Contains(text, "$edge_origin_token_ok:$http_eo_connecting_ip") {
		t.Error("缺「token 命中才取 EO-Connecting-IP」的 map")
	}
	if !strings.Contains(text, "map $edge_client_ip $xff_for_backend {") {
		t.Error("缺后端 XFF 选择 map")
	}

	// token 必须来自部署变量（不是硬编码），且该 map 里有一条赋 1 的键。
	start := strings.Index(text, "map $http_x_origin_token $edge_origin_token_ok {")
	if start < 0 {
		t.Fatal("缺 token 校验 map")
	}
	end := strings.Index(text[start:], "\n}")
	if end < 0 {
		t.Fatal("token 校验 map 没有闭合")
	}
	tokenMap := text[start : start+end]
	if !strings.Contains(tokenMap, "${EDGE_ORIGIN_TOKEN}") || !strings.Contains(tokenMap, " 1;") {
		t.Errorf("token 校验 map 里没有引用部署变量 ${EDGE_ORIGIN_TOKEN} 或缺少赋 1 的键：%q", tokenMap)
	}

	// 业务子域（不经过 EdgeOne）保留「追加对端」的既有语义。
	if !strings.Contains(text, "$proxy_add_x_forwarded_for") {
		t.Error("业务子域的 XFF 追加语义被整体改掉了：非 EdgeOne 流量会丢客户端 IP")
	}

	// fail closed：模板启用鉴权时，入口脚本必须有 token 非空校验并退出。
	script := readEntrypoint(t)
	if !strings.Contains(script, "EDGE_ORIGIN_TOKEN") || !strings.Contains(script, "exit 1") {
		t.Error("docker-entrypoint.sh 缺 EDGE_ORIGIN_TOKEN 的 fail-closed 守卫（空 token 会退化成信任任意 EO-Connecting-IP）")
	}
}
