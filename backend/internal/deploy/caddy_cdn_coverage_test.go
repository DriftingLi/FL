// 本文件：cdn 直出面的**覆盖完整性**锁（#2541 / #2542）。
//
// caddy_topology_test.go 锁的是 cdn 块的**形状**（正向匹配、禁 not path、写方法拦截在 handle 内），
// 但形状对了不等于清单全了 —— 2026-11-14 线上实测：`tutor-files` 分区（#2494 新增）没进
// @object_store 清单，RGW 里对象存在、cdn 却一律 404。形状锁当时全绿，没人发现。
//
// 本文件补两类判据：
//  1. 前缀覆盖：filestore.AttachmentDirPrefixes 的每一项都必须在 @object_store 清单里（同源，不手写第二份）；
//  2. 跨域读：@object_store 的 handle 块必须回 Access-Control-Allow-Origin。
package deploy

import (
	"strings"
	"testing"

	"forklift-training/internal/filestore"
)

// objectStoreMatcherLine 取 cdn 块里 @object_store 的 path 匹配行（已去注释）。
func objectStoreMatcherLine(t *testing.T, cdn string) string {
	t.Helper()
	for _, line := range strings.Split(cdn, "\n") {
		if strings.Contains(line, "@object_store path") {
			return line
		}
	}
	t.Fatal("cdn 块里找不到 @object_store path 匹配行")
	return ""
}

// objectStoreHandleBlock 取 @object_store 的 handle 块正文（按大括号配平）。
func objectStoreHandleBlock(t *testing.T, cdn string) string {
	t.Helper()
	idx := strings.Index(cdn, "handle @object_store {")
	if idx < 0 {
		t.Fatal("cdn 块里找不到 handle @object_store")
	}
	depth := 0
	for i := idx; i < len(cdn); i++ {
		switch cdn[i] {
		case '{':
			depth++
		case '}':
			depth--
			if depth == 0 {
				return cdn[idx : i+1]
			}
		}
	}
	t.Fatal("handle @object_store 的大括号不配平")
	return ""
}

// TestCaddyCDNCoversAllAttachmentPrefixes 前缀覆盖锁（#2541）：
// filestore 前缀登记表的每一项都必须被 cdn 的 @object_store 清单**实际匹配到**。
//
// 判据与登记表同源 —— 新增上传子目录时只在 filestore 登记一次，漏进 Caddyfile 即判红，
// 不会再出现「对象在 RGW 里、cdn 却 404」的静默漏配。
//
// 注意 Caddy 的 `*` 跨越 `/`（实测：`/resumes/*` 匹配 `/resumes/images/b.webp`），
// 所以嵌套前缀（如 `resumes/images`）由父前缀 `/resumes/*` 覆盖即算通过 ——
// 判据按「是否被某个已登记的通配项匹配」来判，而不是要求逐字出现，否则会出假阳性。
func TestCaddyCDNCoversAllAttachmentPrefixes(t *testing.T) {
	src := stripComments(readRepoFile(t, "frontend/Caddyfile"))
	cdn := blockAt(src, "cdn.{$DOMAIN}")
	if cdn == "" {
		t.Fatal("找不到 cdn 站点块")
	}
	line := objectStoreMatcherLine(t, cdn)

	patterns := objectStorePatterns(line)
	if len(patterns) == 0 {
		t.Fatalf("cdn 的 @object_store 清单解析出 0 个路径模式（行形态变了？本判据需要跟着改）：%s", line)
	}

	if len(filestore.AttachmentDirPrefixes) == 0 {
		t.Fatal("filestore.AttachmentDirPrefixes 为空：前缀登记表失效，本判据会空转")
	}
	for _, prefix := range filestore.AttachmentDirPrefixes {
		if prefix == "" {
			t.Error("filestore.AttachmentDirPrefixes 里有空前缀")
			continue
		}
		if !matchedByAnyPattern("/"+prefix+"/probe", patterns) {
			t.Errorf("cdn 的 @object_store 清单覆盖不到前缀 %q（在 filestore 登记了却没进白名单）——"+
				"该分区的文件会在浏览器里一律 404，而对象其实在 RGW 里。清单：%v", prefix, patterns)
		}
	}
}

// objectStorePatterns 从 `@object_store path /a/* /b/* ...` 行里取出路径模式（忽略前导标记）。
func objectStorePatterns(line string) []string {
	var out []string
	for _, tok := range strings.Fields(line) {
		if strings.HasPrefix(tok, "/") {
			out = append(out, tok)
		}
	}
	return out
}

// matchedByAnyPattern 判断某路径是否被任一 Caddy path 模式匹配。
//
// Caddy 的 path 匹配语义：`*` 匹配任意字符序列（含 `/`），其余字符字面量比较。
// 这里只实现本项目用到的形态（`/prefix/*` 与精确路径），未覆盖的形态会让判据落空并报错。
func matchedByAnyPattern(path string, patterns []string) bool {
	for _, p := range patterns {
		if matchCaddyPath(path, p) {
			return true
		}
	}
	return false
}

// matchCaddyPath 单条 Caddy path 模式的匹配（`*` 通配，其余字面量）。
func matchCaddyPath(path, pattern string) bool {
	if !strings.Contains(pattern, "*") {
		return path == pattern
	}
	parts := strings.Split(pattern, "*")
	// 首段必须从行首匹配
	if !strings.HasPrefix(path, parts[0]) {
		return false
	}
	rest := path[len(parts[0]):]
	for i := 1; i < len(parts); i++ {
		seg := parts[i]
		if seg == "" {
			// 尾随 `*`：吃掉剩余全部
			if i == len(parts)-1 {
				return true
			}
			continue
		}
		idx := strings.Index(rest, seg)
		if idx < 0 {
			return false
		}
		rest = rest[idx+len(seg):]
	}
	return rest == "" || strings.HasSuffix(pattern, "*")
}

// TestCaddyCDNAllowsCrossOriginRead 跨域读锁（#2542）：
// @object_store 的 handle 块必须回 Access-Control-Allow-Origin。
//
// 前端三查看器（DocumentViewer / ImageViewer / VideoPlayer）都用 fetch HEAD 探测文件存在性，
// cdn 域 → 页面域必然跨域；不回该头，探测走 catch，页面报「无法连接到文件服务器」——
// 即使对象存在、前缀也对。HEAD 属 CORS simple method，无需预检，一个头够用。
func TestCaddyCDNAllowsCrossOriginRead(t *testing.T) {
	src := stripComments(readRepoFile(t, "frontend/Caddyfile"))
	cdn := blockAt(src, "cdn.{$DOMAIN}")
	if cdn == "" {
		t.Fatal("找不到 cdn 站点块")
	}
	handle := objectStoreHandleBlock(t, cdn)
	if !strings.Contains(handle, "Access-Control-Allow-Origin") {
		t.Error("cdn 的 @object_store handle 块缺少 Access-Control-Allow-Origin：" +
			"前端 HEAD 存在性探测会被浏览器 CORS 拦掉，预览报「无法连接到文件服务器」")
	}
}
