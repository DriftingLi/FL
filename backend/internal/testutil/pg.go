// Package testutil Postgres 适配器（#408 spec / #409 ticket）：后端 HTTP 契约测试可跑在
// 由真实 SQL 迁移建表的 Postgres 上，使「SQLite 测试通过、生产 Postgres 出错」的引擎差异
// 缺陷在 PR 阶段即变红。
//
// 关键约束（spec「测试基础设施」决策）：
//   - 建表走 backend/migrations 的真实迁移，**不用模型 AutoMigrate**——自动迁移会让列类型
//     跟着错误的字段形态一起错，缺陷在测试环境物理上不可见。
//   - 每个用例创建独立 schema（search_path 切换），用例间互不污染；结束时 DROP SCHEMA CASCADE。
//   - 缺少 DATABASE_URL 连接串时 t.Skip（本地与纯前端 CI 不受影响）；沿用 CI 既有的
//     DATABASE_URL / Postgres service，无需新增 CI 配置。
//   - 建库后断言「关键唯一索引都在」（#1362），与 SQLite 面补跑的是同一张登记表
//     （internal/migrate.CriticalUniqueIndexes）⇒ 两个测试引擎的约束面同源，不再各测各的。
package testutil

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"os"
	"strings"
	"testing"

	"go.uber.org/zap"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"forklift-training/internal/migrate"
)

// NewPostgresDB 打开一个由真实迁移建表的 Postgres 测试库（独立 schema）。
// DATABASE_URL 未设置时跳过当前测试。
func NewPostgresDB(t *testing.T) *gorm.DB {
	t.Helper()
	baseDSN := os.Getenv("DATABASE_URL")
	if baseDSN == "" {
		t.Skip("DATABASE_URL 未设置，跳过 Postgres 契约测试（本地 / 纯前端 CI 不受影响）")
	}
	schema := "tp_" + randHex(8)
	sep := "?"
	if strings.Contains(baseDSN, "?") {
		sep = "&"
	}
	dsn := baseDSN + sep + "search_path=" + schema

	admin, err := gorm.Open(postgres.Open(baseDSN), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	if err != nil {
		t.Fatalf("连接 Postgres 失败: %v", err)
	}
	if err := admin.Exec("CREATE SCHEMA " + schema).Error; err != nil {
		t.Fatalf("创建测试 schema 失败: %v", err)
	}
	t.Cleanup(func() {
		if err := admin.Exec("DROP SCHEMA " + schema + " CASCADE").Error; err != nil {
			t.Logf("清理测试 schema 失败(可忽略): %v", err)
		}
	})

	// 真实 SQL 迁移建表（禁用模型自动迁移）
	if err := migrate.RunMigrations(dsn, "up", zap.NewNop()); err != nil {
		t.Fatalf("Postgres 迁移失败: %v", err)
	}
	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	if err != nil {
		t.Fatalf("打开迁移后的连接失败: %v", err)
	}
	// 关键唯一索引存在性（#1362）：SQLite 面由 applyCriticalUniqueIndexes 补同句 DDL，
	// PG 面靠真迁移建出来——这里把两侧对齐到同一张登记表，任一侧少了索引就在契约测试里红。
	// 查询按 current_schema() 收窄（pg_indexes 是全库视图，见 checks.md 的两条纪律）。
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatalf("取 *sql.DB 失败: %v", err)
	}
	actual, err := migrate.ActualUniqueIndexes(context.Background(), sqlDB)
	if err != nil {
		t.Fatalf("查询测试 schema 的唯一索引失败: %v", err)
	}
	if missing := migrate.DiffUniqueIndexes(migrate.CriticalUniqueIndexes(), actual); len(missing) > 0 {
		t.Fatalf("迁移后的测试 schema 缺关键唯一索引 %d 条（首个：%s）——migrations 与 internal/migrate 登记表已漂移",
			len(missing), missing[0])
	}
	return db
}

func randHex(n int) string {
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		return "00000000"
	}
	return hex.EncodeToString(b)
}
