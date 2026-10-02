// Package admin 测试共享辅助：DTO shape-lock 断言（逐字搬自 internal/service/dto_shape_test_helpers_test.go，
// P2 波 4d 随管理域 DTO 搬进本域包；域包测试不得 import internal/service 的测试文件）。
// 断言 JSON key 集合与转换前的 map 契约逐字一致——前端契约零改动是最高优先级约束，
// 任何 DTO 字段增删都会在这里暴露，需同步评估前端影响。
package admin

import (
	"encoding/json"
	"testing"
)

// topLevelKeys 返回 v 序列化后的顶层 key 集合。
func topLevelKeys(t *testing.T, v any) map[string]bool {
	t.Helper()
	b, err := json.Marshal(v)
	if err != nil {
		t.Fatalf("序列化失败: %v", err)
	}
	var m map[string]json.RawMessage
	if err := json.Unmarshal(b, &m); err != nil {
		t.Fatalf("反序列化失败: %v", err)
	}
	keys := map[string]bool{}
	for k := range m {
		keys[k] = true
	}
	return keys
}

// assertShapeLock 断言 key 集合与期望完全一致（不多不少）。
func assertShapeLock(t *testing.T, v any, want ...string) {
	t.Helper()
	got := topLevelKeys(t, v)
	if len(got) != len(want) {
		t.Errorf("key 数量 = %d, 期望 %d\n实际: %v\n期望: %v", len(got), len(want), got, want)
	}
	for _, k := range want {
		if !got[k] {
			t.Errorf("缺少 key: %s", k)
		}
	}
}
