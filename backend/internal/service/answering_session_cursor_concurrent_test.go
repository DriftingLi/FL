// 并发回归（#504 原子守卫）：两个并发进度保存写不同游标，最终游标必须是较大者。
package service

import (
	"sync"
	"testing"

	"forklift-training/internal/testutil"
)

func TestSaveSetCursorAtomicConcurrent(t *testing.T) {
	db := testutil.NewFileDB(t)
	if err := SaveSet(db, 1, "sequential", nil, nil, 0, 10, nil); err != nil {
		t.Fatalf("初始进度失败: %v", err)
	}
	var wg sync.WaitGroup
	// 并发保存游标 3 与 8：最终必须 8（大者胜，小者不覆盖）
	for _, idx := range []int{8, 3, 6} {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			_ = SaveSet(db, 1, "sequential", nil, nil, i, 10, nil)
		}(idx)
	}
	wg.Wait()
	var got int
	if err := db.Raw("SELECT current_index FROM practice_progress WHERE student_id = 1 AND practice_mode = 'sequential'").Scan(&got).Error; err != nil {
		t.Fatalf("读游标失败: %v", err)
	}
	if got != 8 {
		t.Fatalf("并发保存后游标应为最大值 8, got %d", got)
	}
}
