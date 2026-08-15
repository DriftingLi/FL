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
func TestQueryWithMax(t *testing.T) {
	db := testutil.NewMemoryDB(t)
	for i := 0; i < 5; i++ {
		testutil.SeedQuestion(t, db, "single_choice", "题目", "A")
	}
	// 页大小超过上限 maxPageSize=2 时回退默认 defaultPageSize=3（不截断到 2）
	items, total, page, pageSize := QueryWithMax[model.Question](db, 1, 100, 3, 2, "id ASC", nil)
	if total != 5 {
		t.Fatalf("total = %d, want 5", total)
	}
	if pageSize != 3 {
		t.Fatalf("pageSize = %d, want 默认 3 (ClampMax 超上限回退默认)", pageSize)
	}
	if len(items) != 3 {
		t.Fatalf("本页应返回 3 条, got %d", len(items))
	}
	if page != 1 {
		t.Fatalf("page = %d, want 1", page)
	}
	// 合法页大小不被钳制
	_, _, _, pageSize2 := QueryWithMax[model.Question](db, 1, 2, 3, 2, "id ASC", nil)
	if pageSize2 != 2 {
		t.Fatalf("pageSize2 = %d, want 2 (合法页大小不被钳制)", pageSize2)
	}
}
