// Store 集成测试：真实 Postgres 下描述符生成 SQL 的往返正确性。
// CI 提供 postgres:15 服务（DATABASE_URL），本地未配置时跳过。
package dictcrud

import (
	"context"
	"errors"
	"os"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"go.uber.org/zap"

	migratedb "forklift-training/internal/migrate"
)

func integrationPool(t *testing.T) *pgxpool.Pool {
	t.Helper()
	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		t.Skip("DATABASE_URL 未配置，跳过集成测试")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	if err := migratedb.RunMigrations(dsn, "up", zap.NewNop()); err != nil {
		t.Fatalf("迁移失败: %v", err)
	}
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		t.Fatalf("连接失败: %v", err)
	}
	t.Cleanup(pool.Close)
	return pool
}

// TestStore_RegionCoefficientRoundTrip 区域系数走描述符核心的 DB 往返：
// upsert 语义（同 (province, city) 冲突 → 更新 coefficient 不新增行）、
// update 单字段、delete 未命中 → pgx.ErrNoRows。
func TestStore_RegionCoefficientRoundTrip(t *testing.T) {
	pool := integrationPool(t)
	ctx := context.Background()

	if _, err := pool.Exec(ctx, `TRUNCATE region_coefficients RESTART IDENTITY CASCADE`); err != nil {
		t.Fatalf("清空表失败: %v", err)
	}

	s := NewStore(pool)
	d := RegionCoefficientDescriptor

	id, err := s.Create(ctx, d, map[string]any{"province": "江苏", "city": "苏州", "coefficient": 1.02})
	if err != nil {
		t.Fatalf("create 失败: %v", err)
	}
	if id != 1 {
		t.Fatalf("自增 id = %d, 期望 1", id)
	}

	// upsert：同 (province, city) 冲突 → 更新 coefficient，不新增行
	id2, err := s.Create(ctx, d, map[string]any{"province": "江苏", "city": "苏州", "coefficient": 1.05})
	if err != nil {
		t.Fatalf("upsert 失败: %v", err)
	}
	if id2 != id {
		t.Fatalf("upsert 应复用原行 id, got %d want %d", id2, id)
	}
	var count int
	if err := pool.QueryRow(ctx, `SELECT COUNT(*) FROM region_coefficients`).Scan(&count); err != nil {
		t.Fatalf("计数失败: %v", err)
	}
	if count != 1 {
		t.Fatalf("upsert 后应只有 1 行, got %d", count)
	}
	var coefficient float64
	if err := pool.QueryRow(ctx, `SELECT coefficient FROM region_coefficients WHERE id = $1`, id).Scan(&coefficient); err != nil {
		t.Fatalf("读取失败: %v", err)
	}
	if coefficient != 1.05 {
		t.Fatalf("upsert 未更新 coefficient: %v", coefficient)
	}

	// update 单字段
	if err := s.Update(ctx, d, id, map[string]any{"coefficient": 1.10}); err != nil {
		t.Fatalf("update 失败: %v", err)
	}
	if err := pool.QueryRow(ctx, `SELECT coefficient FROM region_coefficients WHERE id = $1`, id).Scan(&coefficient); err != nil {
		t.Fatalf("读取失败: %v", err)
	}
	if coefficient != 1.10 {
		t.Fatalf("update 后 coefficient = %v, 期望 1.10", coefficient)
	}

	// update/delete 未命中 → pgx.ErrNoRows
	if err := s.Update(ctx, d, 999, map[string]any{"coefficient": 1.0}); !errors.Is(err, pgx.ErrNoRows) {
		t.Fatalf("update 未命中应返回 pgx.ErrNoRows, got %v", err)
	}
	if err := s.Delete(ctx, d, 999); !errors.Is(err, pgx.ErrNoRows) {
		t.Fatalf("delete 未命中应返回 pgx.ErrNoRows, got %v", err)
	}

	// delete
	if err := s.Delete(ctx, d, id); err != nil {
		t.Fatalf("delete 失败: %v", err)
	}
	if err := pool.QueryRow(ctx, `SELECT COUNT(*) FROM region_coefficients`).Scan(&count); err != nil {
		t.Fatalf("计数失败: %v", err)
	}
	if count != 0 {
		t.Fatalf("delete 后应为 0 行, got %d", count)
	}
}
