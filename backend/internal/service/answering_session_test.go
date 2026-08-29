// Package service 答题会话 module 测试：守卫、进度重建、答案三态初始化语义。
package service

import (
	"encoding/json"
	"testing"

	"forklift-training/internal/model"
	"forklift-training/internal/testutil"
)

func TestGuardOwnedInProgress(t *testing.T) {
	msgs := struct{ denied, notInProgress string }{"无权操作", "考试不在进行中"}

	if err := guardOwnedInProgress(1, "in_progress", 1, msgs.denied, msgs.notInProgress); err != nil {
		t.Errorf("本人+进行中应通过: %v", err)
	}
	if err := guardOwnedInProgress(2, "in_progress", 1, msgs.denied, msgs.notInProgress); err == nil {
		t.Error("他人会话应拒绝")
	}
	if err := guardOwnedInProgress(1, "submitted", 1, msgs.denied, msgs.notInProgress); err == nil {
		t.Error("非进行中应拒绝")
	}
}

func TestLoadOrderedQuestions_PreservesOrder(t *testing.T) {
	db := testutil.NewMemoryDB(t)
	q1 := testutil.SeedQuestion(t, db, "single_choice", "题1", "A")
	q2 := testutil.SeedQuestion(t, db, "single_choice", "题2", "A")
	q3 := testutil.SeedQuestion(t, db, "single_choice", "题3", "A")

	// 保存顺序与 id 顺序相反，返回必须按保存顺序
	ordered, qMap := loadOrderedQuestions(db, []int{q3.ID, q1.ID, q2.ID, 9999})
	if len(ordered) != 3 {
		t.Fatalf("应返回 3 题（缺失 id 跳过）, got %d", len(ordered))
	}
	if ordered[0].ID != q3.ID || ordered[1].ID != q1.ID || ordered[2].ID != q2.ID {
		t.Fatalf("顺序必须与保存顺序一致: %+v", ordered)
	}
	if _, ok := qMap[q1.ID]; !ok {
		t.Error("qMap 应可按下标取题")
	}
	if _, ok := qMap[9999]; ok {
		t.Error("qMap 不应包含缺失题")
	}
}

func TestAnswersMapRoundTrip(t *testing.T) {
	if got := answersMapRoundTrip(model.JSONB(nil)); len(got) != 0 {
		t.Errorf("nil JSONB 应归一为空 map, got %v", got)
	}
	if got := answersMapRoundTrip(model.JSONB([]byte("null"))); len(got) != 0 {
		t.Errorf("JSON null 应归一为空 map, got %v", got)
	}
	got := answersMapRoundTrip(model.JSONB([]byte(`{"5":"A"}`)))
	if got["5"] != "A" {
		t.Errorf("round-trip 失败: %v", got)
	}
}

// TestInitAnswersState 回归锁定 #142：显式 null/空/缺失三态一律落库为 {}，
// 不允许 JSONB 'null' 写库造成 SQL NULL。
func TestInitAnswersState(t *testing.T) {
	cases := []json.RawMessage{nil, {}, []byte("null"), []byte("{}"), []byte(`{"1":"A"}`)}
	for _, c := range cases {
		out := initAnswersState(c)
		var v any
		if err := json.Unmarshal(out, &v); err != nil {
			t.Fatalf("initAnswersState(%q) 输出非法 JSON: %v", c, err)
		}
		m, ok := v.(map[string]any)
		if !ok {
			t.Fatalf("initAnswersState(%q) 应为 JSON 对象, got %q", c, out)
		}
		if len(c) > 0 && string(c) == `{"1":"A"}` {
			if len(m) != 1 {
				t.Fatalf("有内容时不得清空: %q", out)
			}
		}
	}
}
