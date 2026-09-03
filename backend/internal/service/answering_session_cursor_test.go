// Package service #504 回归：SaveSet 进度保存流游标「只进不退」守卫。
package service

import (
	"encoding/json"
	"strings"
	"testing"

	"forklift-training/internal/model"
	"forklift-training/internal/testutil"
)

// TestSaveSetCursorMonotonic 进度保存流（ids=nil）里新游标小于存量时不落游标
// （prev 后重做旧题 / 并发乱序不拉回断点），answers_state 照常更新。
func TestSaveSetCursorMonotonic(t *testing.T) {
	db := testutil.NewMemoryDB(t)

	// 推进游标到 3（进度保存流）
	if err := SaveSet(db, 1, "sequential", nil, nil, 3, 10, json.RawMessage(`{"1":{"user_answer":"A","is_correct":true}}`)); err != nil {
		t.Fatalf("保存进度失败: %v", err)
	}
	// prev 后重做旧题：保存小游标 1（携带新增 answers_state 题目 2）
	if err := SaveSet(db, 1, "sequential", nil, nil, 1, 10, json.RawMessage(`{"1":{"user_answer":"A","is_correct":true},"2":{"user_answer":"B","is_correct":false}}`)); err != nil {
		t.Fatalf("保存旧题进度失败: %v", err)
	}
	var prog model.PracticeProgress
	if err := db.Where("student_id = ? AND practice_mode = ?", 1, "sequential").First(&prog).Error; err != nil {
		t.Fatalf("读进度失败: %v", err)
	}
	if prog.CurrentIndex != 3 {
		t.Fatalf("小游标不应拉回断点: got %d, want 3", prog.CurrentIndex)
	}
	// answers_state 仍照常更新（含第二次保存新增的题目 2）
	if !strings.Contains(string(prog.AnswersState), "\"2\"") {
		t.Fatalf("answers_state 应照常更新, got %s", string(prog.AnswersState))
	}
	// 正常推进（游标 5）不受影响
	if err := SaveSet(db, 1, "sequential", nil, nil, 5, 10, nil); err != nil {
		t.Fatalf("推进游标失败: %v", err)
	}
	db.Where("student_id = ? AND practice_mode = ?", 1, "sequential").First(&prog)
	if prog.CurrentIndex != 5 {
		t.Fatalf("大游标应正常推进: got %d, want 5", prog.CurrentIndex)
	}
}
