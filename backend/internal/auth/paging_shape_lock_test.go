package auth

import (
	"encoding/json"
	"testing"
)

// paging_shape_lock_test 锁定 profile_review ListRequests 的分页信封 shape：
// 字段名零漂移（前端契约是最高优先级约束）。本用例随域包搬来（ADR-0070 波 3a），
// 同一把锁的 notification List 分支留在 internal/core/paging_shape_lock_test.go。

func TestPagingResultShapeLock(t *testing.T) {
	assertShapeLock(t, &ProfileChangeRequestPageResult{}, "page", "pages", "requests", "total")
}

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
