package paging

import (
	"testing"

	"gorm.io/gorm"

	"forklift-training/internal/model"
	"forklift-training/internal/testutil"
)

func TestClamp(t *testing.T) {
	page, pageSize := Clamp(0, 0, 20)
	if page != 1 || pageSize != 20 {
		t.Errorf("Clamp(0,0,20) = %d,%d, want 1,20", page, pageSize)
	}
	page, pageSize = Clamp(3, 5, 20)
	if page != 3 || pageSize != 5 {
		t.Errorf("Clamp(3,5,20) = %d,%d, want 3,5", page, pageSize)
	}
}

func TestQuery(t *testing.T) {
	db := testutil.NewMemoryDB(t)
	for i := 0; i < 5; i++ {
		testutil.SeedQuestion(t, db, "single_choice", "题目", "A")
	}
	items, total, page, pageSize := Query[model.Question](db, 1, 2, 20, "id ASC", nil)
	if total != 5 {
		t.Fatalf("total = %d, want 5", total)
	}
	if len(items) != 2 {
		t.Fatalf("本页应 2 条, got %d", len(items))
	}
	if page != 1 || pageSize != 2 {
		t.Fatalf("page/pageSize = %d/%d", page, pageSize)
	}
	// 过滤 + 默认分页
	_, total2, page2, pageSize2 := Query[model.Question](db, 0, 0, 10, "", func(q *gorm.DB) *gorm.DB {
		return q.Where("status = ?", "draft")
	})
	if total2 != 0 {
		t.Fatalf("过滤后 total = %d, want 0", total2)
	}
	if page2 != 1 || pageSize2 != 10 {
		t.Fatalf("默认分页 = %d/%d, want 1/10", page2, pageSize2)
	}
}
