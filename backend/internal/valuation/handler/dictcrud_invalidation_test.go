// 缓存失效聚焦验证：迁移后区域系数写操作经描述符核心失效的 pattern 集
// 必须与迁移前 RegionCoefficientInvalidationPatterns() + resultInvalidation 组合完全一致
// （契约单点 repository.PatternsOf("region_coefficients") + ResultCachePattern）。
package handler

import (
	"strings"
	"testing"

	"forklift-training/internal/valuation/dictcrud"
	"forklift-training/internal/valuation/repository"
)

// keyMatches 读 key 前缀是否被失效 pattern 覆盖（与 repository 契约测试同语义）。
func keyMatches(key, pattern string) bool {
	if strings.HasSuffix(pattern, "*") {
		base := strings.TrimSuffix(pattern, "*")
		return strings.HasPrefix(key, base) || strings.HasPrefix(key+":", base)
	}
	return key == pattern
}

func TestDictInvalidationPatterns_Region(t *testing.T) {
	got := dictInvalidationPatterns(dictcrud.RegionCoefficientDescriptor)
	want := []string{"dict:region:*", repository.ResultCachePattern}

	if len(got) != len(want) {
		t.Fatalf("失效 pattern 集漂移: got %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("失效 pattern 漂移: got %v, want %v", got, want)
		}
	}
}

// TestDictInvalidationPatterns_NonResult 未标记 InvalidateResult 的实体
// 不追加评估结果 pattern（series 现状；防描述符误开）。
func TestDictInvalidationPatterns_NonResult(t *testing.T) {
	d := dictcrud.SeriesDescriptor
	got := dictInvalidationPatterns(d)
	for _, p := range got {
		if p == repository.ResultCachePattern {
			t.Fatalf("非 result 实体不应追加评估结果 pattern: %v", got)
		}
	}
	if len(got) != 3 {
		t.Fatalf("series 契约应有 3 个 pattern: %v", got)
	}
}

// TestDictInvalidationPatterns_AllDescriptors 全部描述符的失效集 = 契约 pattern +
// （除 series 外）ResultCachePattern；结果集内每个 pattern 必须覆盖该实体读前缀
// （沿用 repository 契约 static 测试的不变式；此处在 handler 侧以描述符验证）。
func TestDictInvalidationPatterns_AllDescriptors(t *testing.T) {
	descriptors := []dictcrud.Descriptor{
		dictcrud.BrandDescriptor, dictcrud.VehicleTypeDescriptor, dictcrud.SeriesDescriptor,
		dictcrud.TonnageDescriptor, dictcrud.MastTypeDescriptor, dictcrud.MastHeightDescriptor,
		dictcrud.BatteryTypeDescriptor, dictcrud.TransmissionTypeDescriptor, dictcrud.EngineTypeDescriptor,
		dictcrud.ConditionRatingDescriptor, dictcrud.RegionCoefficientDescriptor,
		dictcrud.CoefficientConfigDescriptor, dictcrud.OriginalPriceDescriptor,
	}
	for _, d := range descriptors {
		got := dictInvalidationPatterns(d)
		contract := repository.PatternsOf(d.Name)
		if len(contract) == 0 {
			t.Fatalf("%s 无缓存契约", d.Name)
		}
		wantCount := len(contract)
		if d.InvalidateResult {
			wantCount++
		}
		if len(got) != wantCount {
			t.Fatalf("%s 失效 pattern 数 = %d, 期望 %d: %v", d.Name, len(got), wantCount, got)
		}
		for i, p := range contract {
			if got[i] != p {
				t.Fatalf("%s 失效 pattern 漂移: got %v, want %v", d.Name, got, contract)
			}
		}
	}
}

// TestRegionDescriptor_ReadPrefixesCovered 区域系数的读 key 前缀必须被描述符实体的
// 失效集覆盖（沿用 repository 契约 static 测试的不变式；此处在 handler 侧以描述符验证）。
func TestRegionDescriptor_ReadPrefixesCovered(t *testing.T) {
	patterns := dictInvalidationPatterns(dictcrud.RegionCoefficientDescriptor)
	readPrefixes := []string{
		repository.CachePrefixRegionList,
		repository.CachePrefixRegionCities,
		repository.CachePrefixRegionGet,
	}
	for _, prefix := range readPrefixes {
		covered := false
		for _, p := range patterns {
			if keyMatches(prefix, p) {
				covered = true
				break
			}
		}
		if !covered {
			t.Errorf("区域系数读前缀 %q 未被失效集覆盖: %v", prefix, patterns)
		}
	}
}
