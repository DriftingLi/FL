// 复杂实体（#95）描述符写存储集成测试：真实 Postgres 下
// original_prices（宽行 upsert/update + updated_at 尾列）与
// coefficient_configs（按 key 更新 + RETURNING 整行）的往返正确性。
// CI 提供 postgres:15 服务（DATABASE_URL），本地未配置时跳过。
package dictcrud

import (
	"context"
	"errors"
	"testing"

	"github.com/jackc/pgx/v5"
)

func opValues() map[string]any {
	return map[string]any{
		"brand": "合力", "vehicle_type": "电动叉车", "series": "K系列",
		"tonnage": 3.0, "config_type": "标准", "mast_type": "标准门架",
		"mast_height_mm": 3000, "earliest_factory_year": 2015, "original_price": 100000.0,
	}
}

// TestStore_OriginalPriceRoundTrip 原价宽行走描述符核心的 DB 往返：
// 7 字段复合唯一 upsert（同组合冲突 → 更新非唯一列不新增行）、
// 全字段 update、delete 未命中 → pgx.ErrNoRows。
func TestStore_OriginalPriceRoundTrip(t *testing.T) {
	pool := integrationPool(t)
	ctx := context.Background()

	if _, err := pool.Exec(ctx, `TRUNCATE original_prices RESTART IDENTITY CASCADE`); err != nil {
		t.Fatalf("清空表失败: %v", err)
	}

	s := NewStore(pool)
	d := OriginalPriceDescriptor

	id, err := s.Create(ctx, d, opValues())
	if err != nil {
		t.Fatalf("create 失败: %v", err)
	}
	if id != 1 {
		t.Fatalf("自增 id = %d, 期望 1", id)
	}

	// upsert：同 7 字段组合冲突 → 更新 earliest_factory_year/original_price，不新增行
	upsertVals := opValues()
	upsertVals["original_price"] = 105000.0
	id2, err := s.Create(ctx, d, upsertVals)
	if err != nil {
		t.Fatalf("upsert 失败: %v", err)
	}
	if id2 != id {
		t.Fatalf("upsert 应复用原行 id, got %d want %d", id2, id)
	}
	var count int
	if err := pool.QueryRow(ctx, `SELECT COUNT(*) FROM original_prices`).Scan(&count); err != nil {
		t.Fatalf("计数失败: %v", err)
	}
	if count != 1 {
		t.Fatalf("upsert 后应只有 1 行, got %d", count)
	}
	var price float64
	if err := pool.QueryRow(ctx, `SELECT original_price FROM original_prices WHERE id = $1`, id).Scan(&price); err != nil {
		t.Fatalf("读取失败: %v", err)
	}
	if price != 105000.0 {
		t.Fatalf("upsert 未更新 original_price: %v", price)
	}

	// 全字段 update
	updVals := opValues()
	updVals["series"] = "K2系列"
	if err := s.Update(ctx, d, id, updVals); err != nil {
		t.Fatalf("update 失败: %v", err)
	}
	var series string
	if err := pool.QueryRow(ctx, `SELECT series FROM original_prices WHERE id = $1`, id).Scan(&series); err != nil {
		t.Fatalf("读取失败: %v", err)
	}
	if series != "K2系列" {
		t.Fatalf("update 后 series = %q, 期望 K2系列", series)
	}

	// update/delete 未命中 → pgx.ErrNoRows
	if err := s.Update(ctx, d, 999, opValues()); !errors.Is(err, pgx.ErrNoRows) {
		t.Fatalf("update 未命中应返回 pgx.ErrNoRows, got %v", err)
	}
	if err := s.Delete(ctx, d, 999); !errors.Is(err, pgx.ErrNoRows) {
		t.Fatalf("delete 未命中应返回 pgx.ErrNoRows, got %v", err)
	}

	// delete
	if err := s.Delete(ctx, d, id); err != nil {
		t.Fatalf("delete 失败: %v", err)
	}
	if err := pool.QueryRow(ctx, `SELECT COUNT(*) FROM original_prices`).Scan(&count); err != nil {
		t.Fatalf("计数失败: %v", err)
	}
	if count != 0 {
		t.Fatalf("delete 后应为 0 行, got %d", count)
	}
}

// TestStore_CoefficientByKeyRoundTrip 系数配置按 key 更新：
// RETURNING 整行（description 可空 → ""、updated_at 格式化）；未命中 → pgx.ErrNoRows。
func TestStore_CoefficientByKeyRoundTrip(t *testing.T) {
	pool := integrationPool(t)
	ctx := context.Background()

	if _, err := pool.Exec(ctx, `UPDATE coefficient_configs SET value = 0.10 WHERE key = 'lambda_electric'`); err != nil {
		t.Fatalf("重置失败: %v", err)
	}

	s := NewStore(pool)
	d := CoefficientConfigDescriptor

	row, err := s.UpdateByKey(ctx, d, "lambda_electric", map[string]any{"value": 0.15})
	if err != nil {
		t.Fatalf("update by key 失败: %v", err)
	}
	if row["key"] != "lambda_electric" || row["value"] != 0.15 {
		t.Fatalf("响应行错误: %v", row)
	}
	if row["description"] == nil || row["updated_at"] == nil {
		t.Fatalf("响应缺 description/updated_at: %v", row)
	}
	if _, ok := row["updated_at"].(string); !ok || row["updated_at"].(string) == "" {
		t.Fatalf("updated_at 未格式化: %v", row["updated_at"])
	}

	var value float64
	if err := pool.QueryRow(ctx, `SELECT value FROM coefficient_configs WHERE key = 'lambda_electric'`).Scan(&value); err != nil {
		t.Fatalf("读取失败: %v", err)
	}
	if value != 0.15 {
		t.Fatalf("update 后 value = %v, 期望 0.15", value)
	}

	// 未命中 → pgx.ErrNoRows
	if _, err := s.UpdateByKey(ctx, d, "no_such_key", map[string]any{"value": 0.5}); !errors.Is(err, pgx.ErrNoRows) {
		t.Fatalf("未命中应返回 pgx.ErrNoRows, got %v", err)
	}
}
