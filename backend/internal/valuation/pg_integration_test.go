// 域内共享的真实 Postgres 集成夹具：CI 提供 postgres:15 服务（DATABASE_URL），本地未配置时干净跳过。
// 本文件是**唯一**定义处（#1514 波 7 并包前，dictcrud 与 repository 两个子包各有一份逐字相同的副本）。
package valuation

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"go.uber.org/zap"

	migratedb "forklift-training/internal/migrate"
)

// integrationPool 跑完迁移并返回连接池（未配 DATABASE_URL 即跳过）。
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
