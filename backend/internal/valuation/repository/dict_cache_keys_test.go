// 缓存 key 契约 static 测试：读 key 前缀必须被失效 pattern 覆盖。
package repository

import (
	"strings"
	"testing"
)

// keyMatchesPattern key 空间（前缀）是否匹配失效 pattern：
// pattern 以 * 结尾为前缀匹配（key 代表一个 key 空间：key 或 key+":" 任一起于 base 即被覆盖），否则精确匹配。
func keyMatchesPattern(key, pattern string) bool {
	if strings.HasSuffix(pattern, "*") {
		base := strings.TrimSuffix(pattern, "*")
		return strings.HasPrefix(key, base) || strings.HasPrefix(key+":", base)
	}
	return key == pattern
}

// TestCacheContracts_ReadPrefixesCovered 每实体的读 key 前缀 ⊆ 该实体失效 pattern 集。
// 这是"级联读 key 永不被失效"缺陷的回归测试：tonnages/mast_types/mast_heights/battery_types
// 的级联读前缀必须出现在各自失效集中。
func TestCacheContracts_ReadPrefixesCovered(t *testing.T) {
	for _, c := range dictCacheContracts {
		for _, prefix := range c.ReadKeyPrefixes {
			covered := false
			for _, pattern := range c.InvalidatePatterns {
				if keyMatchesPattern(prefix, pattern) {
					covered = true
					break
				}
			}
			if !covered {
				t.Errorf("实体 %s 读前缀 %q 未被自身失效集覆盖: %v", c.Name, prefix, c.InvalidatePatterns)
			}
		}
	}
}

// TestCacheContracts_GlobalCoverage 系统内全部读前缀至少被一个实体失效集覆盖。
// original_prices 的兜底 pattern "dict:*" 可覆盖任何前缀，但各实体的具体失效集
// 不能依赖兜底（一旦兜底被收窄，级联数据会静默陈旧）。
func TestCacheContracts_GlobalCoverage(t *testing.T) {
	seen := map[string]bool{}
	for _, c := range dictCacheContracts {
		for _, pattern := range c.InvalidatePatterns {
			seen[pattern] = true
		}
	}
	for _, c := range dictCacheContracts {
		for _, prefix := range c.ReadKeyPrefixes {
			if prefix == "dict:*" {
				continue
			}
			covered := false
			for pattern := range seen {
				if keyMatchesPattern(prefix, pattern) {
					covered = true
					break
				}
			}
			if !covered {
				t.Errorf("读前缀 %q（实体 %s）未被任何实体失效集覆盖", prefix, c.Name)
			}
		}
	}
}

// TestCacheContracts_CascadePrefixesInvalidated 级联读前缀必须由非兜底的具体 pattern 失效
// （回归：dict:specs:* 与 dict:*:cascade:* 两套命名互不包含的缺陷）。
func TestCacheContracts_CascadePrefixesInvalidated(t *testing.T) {
	cascades := []struct {
		name     string
		prefix   string
		expected []string
	}{
		{"tonnages", CachePrefixTonnagesCascade, []string{"dict:tonnages:cascade:*"}},
		{"mast_types", CachePrefixMastTypesCascade, []string{"dict:mast_types:cascade:*"}},
		{"mast_heights", CachePrefixMastHeightsCascade, []string{"dict:mast_heights:cascade:*"}},
		{"battery_types", CachePrefixBatteryCascade, []string{"dict:battery:cascade:*"}},
	}
	for _, cc := range cascades {
		found := false
		for _, c := range dictCacheContracts {
			if c.Name == cc.name {
				for _, pattern := range c.InvalidatePatterns {
					if pattern == cc.expected[0] {
						found = true
					}
				}
			}
		}
		if !found {
			t.Errorf("实体 %s 的失效集缺少级联 pattern %v", cc.name, cc.expected)
		}
	}
}

// TestPatternsOf_AllEntities 每个契约都能通过 PatternsOf 取到失效集（防拼写漂移）。
func TestPatternsOf_AllEntities(t *testing.T) {
	for _, c := range dictCacheContracts {
		if got := PatternsOf(c.Name); got == nil {
			t.Errorf("PatternsOf(%q) 返回 nil", c.Name)
		}
	}
}
