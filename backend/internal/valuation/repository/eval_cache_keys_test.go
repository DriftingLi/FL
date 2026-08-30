// 评估缓存 key 契约 static 测试：读 key ⊆ 写路径失效 pattern 集（含 per-user 形状）。
// prior art：dict_cache_keys_test.go（keyMatchesPattern 同包复用）。
package repository

import (
	"os"
	"strings"
	"testing"
)

// TestEvalCacheContract_ReadPrefixesCovered 评估域全部读 key 前缀 ⊆ 写路径失效 pattern 集的并集。
// 新增读 key 形状而失效集未跟上时，本测试即失败（与 dict 域同款不变式）。
func TestEvalCacheContract_ReadPrefixesCovered(t *testing.T) {
	patterns := map[string]bool{}
	for _, ps := range evalWriteInvalidations {
		for _, p := range ps {
			patterns[p] = true
		}
	}
	for _, f := range evalCacheFamilies {
		for _, prefix := range f.ReadKeyPrefixes {
			covered := false
			for pattern := range patterns {
				if keyMatchesPattern(prefix, pattern) {
					covered = true
					break
				}
			}
			if !covered {
				t.Errorf("家族 %s 读前缀 %q 未被任何写路径失效集覆盖: %v", f.Name, prefix, patterns)
			}
		}
	}
}

// TestEvalCacheContract_DetailWriteCoversBothShapes 报告回写/建议回填必须同时覆盖公开与
// per-user 两种详情读 key 形状（#399 回归：per-user key 从不失效导致学生端 stale 10 分钟）。
func TestEvalCacheContract_DetailWriteCoversBothShapes(t *testing.T) {
	shapes := []string{cachePrefixEvalGet, cachePrefixEvalGetUser}
	for _, op := range []string{evalWriteReportPath, evalWriteSuggestions} {
		for _, shape := range shapes {
			covered := false
			for _, pattern := range evalWriteInvalidations[op] {
				if keyMatchesPattern(shape, pattern) {
					covered = true
					break
				}
			}
			if !covered {
				t.Errorf("写操作 %q 的失效集 %v 未覆盖详情读 key 形状 %q",
					op, evalWriteInvalidations[op], shape)
			}
		}
	}
}

// TestEvalCacheContract_CreateCoversListAndCount 新建评估的失效集必须覆盖列表与统计读前缀。
func TestEvalCacheContract_CreateCoversListAndCount(t *testing.T) {
	for _, prefix := range []string{cachePrefixEvalList, cachePrefixEvalCount} {
		covered := false
		for _, pattern := range evalWriteInvalidations[evalWriteCreate] {
			if keyMatchesPattern(prefix, pattern) {
				covered = true
				break
			}
		}
		if !covered {
			t.Errorf("写操作 %q 的失效集 %v 未覆盖读前缀 %q",
				evalWriteCreate, evalWriteInvalidations[evalWriteCreate], prefix)
		}
	}
}

// TestEvalCacheContract_KeyShapes 读 key 构造器形状：per-user key 起于公开详情前缀之内
// （一种 pattern 覆盖两种形状）；key 随 uid/id/用户维度区分，不跨用户串缓存。
func TestEvalCacheContract_KeyShapes(t *testing.T) {
	if got, want := evalGetKey(42), "eval:get:42"; got != want {
		t.Errorf("evalGetKey(42) = %q, want %q", got, want)
	}
	if got, want := evalGetUserKey(7, 42), "eval:get:user:7:42"; got != want {
		t.Errorf("evalGetUserKey(7, 42) = %q, want %q", got, want)
	}
	// per-user 形状嵌套于公开前缀：eval:get:* 一种 pattern 失效两种形状
	if !strings.HasPrefix(evalGetUserKey(7, 42), cachePrefixEvalGet+":") {
		t.Errorf("per-user key %q 应起于公开详情前缀 %q 之内", evalGetUserKey(7, 42), cachePrefixEvalGet)
	}
	if evalGetUserKey(7, 42) == evalGetUserKey(8, 42) {
		t.Error("不同用户的 per-user 详情 key 不应相同（跨用户串缓存）")
	}
	if evalListKey("", 7, 20, 0) == evalListKey("", 8, 20, 0) {
		t.Error("不同用户的列表 key 不应相同（跨用户串缓存）")
	}
	if evalCountKey("", 7) == evalCountKey("", 8) {
		t.Error("不同用户的统计 key 不应相同（跨用户串缓存）")
	}
}

// TestEvalCacheContract_NoEvalLiteralKeys 静态扫描本包全部生产 .go 源文件，禁止任何
// eval:* 缓存 key/pattern 字面量散落在读方法与写路径里：所有 key 必须经契约构造器
// （eval_cache_keys.go），失效必须经 invalidateEvalWrite。新增读方法若漏声明进
// evalCacheFamilies 或直接书写字面量即测试失败。
//
// 契约 source-of-truth 是 eval_cache_keys.go 本文件内声明的常量，该文件允许出现
// eval:*（常量定义点），与 *_test.go（集成测试的清场 flush 需要字面量）一并排除。
func TestEvalCacheContract_NoEvalLiteralKeys(t *testing.T) {
	entries, err := os.ReadDir(".")
	if err != nil {
		t.Fatalf("读取包目录失败: %v", err)
	}
	for _, e := range entries {
		name := e.Name()
		if e.IsDir() || !strings.HasSuffix(name, ".go") ||
			strings.HasSuffix(name, "_test.go") || name == "eval_cache_keys.go" {
			continue
		}
		data, err := os.ReadFile(name)
		if err != nil {
			t.Fatalf("读取 %s 失败: %v", name, err)
		}
		for i, line := range strings.Split(string(data), "\n") {
			if strings.Contains(line, `"eval:`) || strings.Contains(line, "`eval:") ||
				strings.Contains(line, `"eval"`) || strings.Contains(line, "`eval`") {
				t.Errorf("%s:%d 含游离的 eval:* 缓存 key/pattern 字面量，应经契约构造器与 invalidateEvalWrite: %s",
					name, i+1, strings.TrimSpace(line))
			}
		}
	}
}
