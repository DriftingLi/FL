// student 包测试共享辅助：DTO shape-lock 断言（原 internal/service/dto_shape_test_helpers_test.go 的
// 域内副本，波 4b 随 dto_shape_test.go 搬进本包；断言 JSON key 集合与转换前的 map 契约逐字一致）。
package student

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

// marshalJSON 序列化 v（shape-lock 字节断言辅助）。
func marshalJSON(t *testing.T, v any) ([]byte, error) {
	t.Helper()
	b, err := json.Marshal(v)
	if err != nil {
		t.Fatalf("序列化失败: %v", err)
	}
	return b, nil
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
