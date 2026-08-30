// 评估记录（evaluations）缓存 key 契约：读 key 与失效 pattern 单点定义。
//
// 不变式（static 测试锁定，见 eval_cache_keys_test.go）：
// 评估域全部读 key 前缀 ⊆ 写路径失效 pattern 集的并集，且详情写路径
// （报告回写/建议回填）必须同时覆盖公开与 per-user 两种详情读 key 形状。
// 历史上 per-user 读 key（eval:get:user:<uid>:<id>）从不被任何写路径失效，
// 报告回写后学生端详情最长 stale 10 分钟——本文件让契约只有一个来源。
//
// 读 key 前缀 ⊆ 失效 pattern 集（与 dict_cache_keys.go 同款不变式）；
// per-user 详情前缀（eval:get:user）天然起于公开详情前缀（eval:get）之内，
// 一种失效 pattern（eval:get:*）即覆盖两种详情形状。
package repository

import (
	"context"
	"strconv"

	"forklift-training/internal/cache"
)

// 读 key 前缀常量（cacheKey 构造 key 的公共前缀）。
const (
	cachePrefixEvalGet     = "eval:get"      // 公开详情读前缀（eval:get:<id>，报告生成/下载等公开场景）
	cachePrefixEvalGetUser = "eval:get:user" // per-user 详情读前缀（eval:get:user:<uid>:<id>，归属校验详情）
	cachePrefixEvalList    = "eval:list"     // 历史列表读前缀
	cachePrefixEvalCount   = "eval:count"    // 历史统计读前缀
)

// 失效 pattern 常量（仓储写路径经 invalidateEvalWrite 引用，不书写字面量）。
const (
	cachePatternEvalGet   = cachePrefixEvalGet + ":*"
	cachePatternEvalList  = cachePrefixEvalList + ":*"
	cachePatternEvalCount = cachePrefixEvalCount + ":*"
)

// evalCacheFamily 一个评估缓存家族的读面。
type evalCacheFamily struct {
	// Name 家族名（static 测试报错定位用）。
	Name string
	// ReadKeyPrefixes 该家族全部读 key 的公共前缀。
	ReadKeyPrefixes []string
}

// evalCacheFamilies 评估域全部缓存家族。全部读前缀必须被写路径失效集并集覆盖
// （新增读 key 形状而失效集未跟上时，static 测试即失败）。
var evalCacheFamilies = []evalCacheFamily{
	{Name: "detail", ReadKeyPrefixes: []string{cachePrefixEvalGet, cachePrefixEvalGetUser}},
	{Name: "list", ReadKeyPrefixes: []string{cachePrefixEvalList}},
	{Name: "count", ReadKeyPrefixes: []string{cachePrefixEvalCount}},
}

// 写操作名（仓储写方法按名调用 invalidateEvalWrite）。
const (
	evalWriteCreate      = "create"      // 新建评估记录：列表与统计结果变化
	evalWriteReportPath  = "report_path" // 报告回写：详情变化（公开 + per-user 两种形状）
	evalWriteSuggestions = "suggestions" // 建议回填：详情变化（公开 + per-user 两种形状）
)

// evalWriteInvalidations 写操作 → 必须失效的 pattern 集（失效单一来源）。
// 报告/建议回写失效整个详情家族：per-user key 带 uid 无法精确枚举，
// pattern 失效同时覆盖公开与 per-user 两种形状（修复 per-user 从不失效的缺陷）。
var evalWriteInvalidations = map[string][]string{
	evalWriteCreate:      {cachePatternEvalCount, cachePatternEvalList},
	evalWriteReportPath:  {cachePatternEvalGet},
	evalWriteSuggestions: {cachePatternEvalGet},
}

// invalidateEvalWrite 按写操作名失效其契约 pattern 集（仓储写路径唯一失效入口）。
func invalidateEvalWrite(ctx context.Context, op string) {
	for _, pattern := range evalWriteInvalidations[op] {
		_ = cache.InvalidatePattern(ctx, pattern)
	}
}

// evalGetKey 公开详情读 key（eval:get:<id>）。
func evalGetKey(id int64) string {
	return cacheKey(cachePrefixEvalGet, strconv.FormatInt(id, 10))
}

// evalGetUserKey per-user 详情读 key（eval:get:user:<uid>:<id>）。
func evalGetUserKey(userID int, id int64) string {
	return cacheKey(cachePrefixEvalGetUser, strconv.Itoa(userID), strconv.FormatInt(id, 10))
}

// evalListKey 历史列表读 key（brand/userID/limit/offset 参与查询结果，必须进 key）。
func evalListKey(brand string, userID, limit, offset int) string {
	return cacheKey(cachePrefixEvalList, brand, "u"+strconv.Itoa(userID),
		strconv.Itoa(limit), strconv.Itoa(offset))
}

// evalCountKey 历史统计读 key。
func evalCountKey(brand string, userID int) string {
	return cacheKey(cachePrefixEvalCount, brand, "u"+strconv.Itoa(userID))
}
