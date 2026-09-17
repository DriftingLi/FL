package paging

import (
	"encoding/json"
	"strings"
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
	items, total, page, pageSize, err := Query[model.Question](db, 1, 2, 20, "id ASC", nil)
	if err != nil {
		t.Fatalf("Query 失败: %v", err)
	}
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
	_, total2, page2, pageSize2, err := Query[model.Question](db, 0, 0, 10, "", func(q *gorm.DB) *gorm.DB {
		return q.Where("status = ?", "draft")
	})
	if err != nil {
		t.Fatalf("Query 失败: %v", err)
	}
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
	items, total, page, pageSize, err := QueryWithMax[model.Question](db, 1, 100, 3, 2, "id ASC", nil)
	if err != nil {
		t.Fatalf("QueryWithMax 失败: %v", err)
	}
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
	_, _, _, pageSize2, err := QueryWithMax[model.Question](db, 1, 2, 3, 2, "id ASC", nil)
	if err != nil {
		t.Fatalf("QueryWithMax 失败: %v", err)
	}
	if pageSize2 != 2 {
		t.Fatalf("pageSize2 = %d, want 2 (合法页大小不被钳制)", pageSize2)
	}
}

// TestItemsPageBytes ItemsPage 的字节锁（#1095）：键序必须与 api 层既有
// gin.H{"items","page","page_size","total"} 的输出逐字节一致（encoding/json 对 map 按 key 排序）。
func TestItemsPageBytes(t *testing.T) {
	raw, err := json.Marshal(ItemsPage[model.Question]{})
	if err != nil {
		t.Fatalf("marshal 失败: %v", err)
	}
	want := `{"items":null,"page":0,"page_size":0,"total":0}`
	if string(raw) != want {
		t.Fatalf("ItemsPage 字节序漂移: got %s, want %s", raw, want)
	}
}

// closePool 关闭 gorm 底层 *sql.DB 连接池：此后任何查询返回 "sql: database is closed"。
// 这是 paging 层能拿到的最真实的 DB 故障注入（内存库可用，无需外部依赖）。
func closePool(t *testing.T, db *gorm.DB) {
	t.Helper()
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatalf("取底层连接池失败: %v", err)
	}
	if err := sqlDB.Close(); err != nil {
		t.Fatalf("关闭连接池失败: %v", err)
	}
}

// TestQueryPropagatesDBError 失败注入（ADR-0056 §1 / issue #1095 验收判据 1）：
// Count/Find 的 DB 错误必须在 interface 上出现并上抛 —— 收口前这里返回 (nil, 0, ...) 无错误，
// DB 故障被渲染成 HTTP 200 + items:[] + total:0（19 个 importer 继承的 fail-open）。
func TestQueryPropagatesDBError(t *testing.T) {
	db := testutil.NewMemoryDB(t)
	testutil.SeedQuestion(t, db, "single_choice", "题目", "A")
	closePool(t, db)

	items, total, _, _, err := Query[model.Question](db, 1, 10, 20, "id ASC", nil)
	if err == nil {
		t.Fatalf("Query 应上抛 DB 故障，实际 err=nil（fail-open：items=%v total=%d）", items, total)
	}

	items, total, _, _, err = QueryWithMax[model.Question](db, 1, 10, 20, 100, "id ASC", nil)
	if err == nil {
		t.Fatalf("QueryWithMax 应上抛 DB 故障，实际 err=nil（fail-open：items=%v total=%d）", items, total)
	}
}

// TestQueryWithScanPropagatesDBError QueryWithScan 的同一失败注入：Count 阶段的错误同样上抛
// （用「不存在的表」造 SQL 错误，验证不止「连接被关」这一种失败形态）。
func TestQueryWithScanPropagatesDBError(t *testing.T) {
	db := testutil.NewMemoryDB(t)
	rows, total, _, _, err := QueryWithScan[model.Question](db, 1, 10, 20, 100, "id ASC", func(q *gorm.DB) *gorm.DB {
		return q.Table("table_that_does_not_exist")
	})
	if err == nil {
		t.Fatalf("QueryWithScan 应上抛 DB 故障，实际 err=nil（fail-open：rows=%v total=%d）", rows, total)
	}
	if !strings.Contains(err.Error(), "table_that_does_not_exist") {
		t.Fatalf("错误应来自真实 SQL 失败（提及缺失的表），got %v", err)
	}
}
