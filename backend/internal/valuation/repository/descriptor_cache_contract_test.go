package repository

import (
	"testing"

	"forklift-training/internal/valuation/dictcrud"
)

// TestDescriptorNamesMatchCacheContract 描述符驱动的写面实体名必须命中缓存失效契约：
// 描述符 Name 与 dictCacheContracts 的键一一对应，名称漂移会静默禁用写后失效（缓存永不更新）。
// 描述符列表来自 AllDescriptors（路由注册与测试同源）。
func TestDescriptorNamesMatchCacheContract(t *testing.T) {
	reg := dictcrud.NewRegistry(dictcrud.AllDescriptors()...)
	descriptors := reg.All()
	if len(descriptors) == 0 {
		t.Fatal("AllDescriptors() 不应为空")
	}
	seen := map[string]bool{}
	for _, d := range descriptors {
		if d.Name == "" {
			t.Fatal("存在空 Name 的描述符")
		}
		if seen[d.Name] {
			t.Fatalf("描述符重名: %s", d.Name)
		}
		seen[d.Name] = true

		patterns := PatternsOf(d.Name)
		if len(patterns) == 0 {
			t.Fatalf("描述符 %q 未命中缓存失效契约（PatternsOf 返回空）——写操作将不再失效任何缓存", d.Name)
		}
	}
}
