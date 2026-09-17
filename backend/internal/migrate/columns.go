// 单向列对账（#1099 / ADR-0056 §6）。
//
// 为什么需要：生产 schema 的事实源是 migrations/*.up.sql，测试与运行时 schema 的事实源是
// GORM AutoMigrate(model.AllModels())，两者此前零对账——「模型加了字段、忘了写迁移」在部署
// 前无人能答（CI 的 migration-check 只数过文件个数）。本文件把这条判据前移：
//
//	GORM 模型期望的列集合 ⊆ information_schema 的实际列集合
//
// 三个有意为之的口径（ADR-0056 §6「被否备选」）：
//   - **单向**：实际库多出来的列（迁移里有、模型刻意不映射，或索引/历史列）**不**算错，
//     否则每次刻意不映射都会假红；
//   - **只读 information_schema.columns**：实际列的唯一来源，不反问模型、不猜 DDL；
//   - **指名到「表.列」**：逐条打印缺失项，不报个数（37 对迁移里报个数没有定位价值）。
//
// 用法（CLI 面复用 cmd/migrate 既有的 direction 分派，不改 CLI 入口）：
//
//	DATABASE_URL=postgres://... go run ./cmd/migrate check-columns
//
// 缺失任一列/表即非零退出，并逐条打印「表.列」。CI 的 migration-check job 在
// 「migrate up 之后」与「migrate down 之后（期望报红）」各跑一次。
package migrate

import (
	"context"
	"database/sql"
	"fmt"
	"sort"
	"sync"
	"time"

	_ "github.com/jackc/pgx/v5/stdlib" // database/sql 驱动名 "pgx"：对账只读 information_schema
	"go.uber.org/zap"
	"gorm.io/gorm/schema"

	"forklift-training/internal/model"
)

// informationSchemaColumnsSQL 是生产口径的实际列查询：只读元数据，不触碰业务表数据。
// table_schema = current_schema() 让连接串上的 search_path 决定对账范围（默认 public）。
const informationSchemaColumnsSQL = `SELECT table_name, column_name
FROM information_schema.columns
WHERE table_schema = current_schema()`

// ExpectedColumns 从 GORM 模型推导「表 → 期望列集合」（纯内存，不连库）。
// 列的取法与 AutoMigrate 同源：schema.DBNames + Field.IgnoreMigration 过滤
// （gorm:"-:migration" 是刻意不建列的字段，要求它存在是假红）。
func ExpectedColumns(models []any) (map[string][]string, error) {
	var cache sync.Map
	out := make(map[string][]string, len(models))
	for _, m := range models {
		s, err := schema.Parse(m, &cache, schema.NamingStrategy{})
		if err != nil {
			return nil, fmt.Errorf("解析 GORM 模型 %T 失败: %w", m, err)
		}
		cols := make([]string, 0, len(s.DBNames))
		for _, name := range s.DBNames {
			if f, ok := s.FieldsByDBName[name]; ok && f.IgnoreMigration {
				continue
			}
			cols = append(cols, name)
		}
		sort.Strings(cols)
		out[s.Table] = cols
	}
	// 兜底（fail-closed）：期望集合为空时不得进入对账——否则 AllModels() 变空 / 传参写错会让
	// DiffColumns 报出「0 表 0 列」的通过（CheckColumns 随即打印「对账通过 表=0 列=0」）。
	if len(out) == 0 {
		return nil, fmt.Errorf("期望列集合为空（传入模型 %d 个）：拒绝在空集合上判「对账通过」", len(models))
	}
	return out, nil
}

// ActualColumns 读取实际库的列集合（表 → 列集），来源是 information_schema。
func ActualColumns(ctx context.Context, db *sql.DB) (map[string]map[string]struct{}, error) {
	return queryColumns(ctx, db, informationSchemaColumnsSQL)
}

// queryColumns 是 ActualColumns 的查询面：SQL 可替换，便于单测用内存库跑真实建表/加列
// （对账的「实际」侧仍来自数据库自省，而不是测试伪造的 map）。
func queryColumns(ctx context.Context, db *sql.DB, query string) (map[string]map[string]struct{}, error) {
	rows, err := db.QueryContext(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("查询实际列失败: %w", err)
	}
	defer func() { _ = rows.Close() }()

	out := map[string]map[string]struct{}{}
	for rows.Next() {
		var table, column string
		if err := rows.Scan(&table, &column); err != nil {
			return nil, fmt.Errorf("读取实际列失败: %w", err)
		}
		if out[table] == nil {
			out[table] = map[string]struct{}{}
		}
		out[table][column] = struct{}{}
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("遍历实际列失败: %w", err)
	}
	return out, nil
}

// ColumnDiff 是单向对账结果。缺表与缺列都判红，但分开报：整表缺失时逐列刷屏没有意义。
type ColumnDiff struct {
	MissingTables  []string // 模型有、实际库没有的表
	MissingColumns []string // "表.列"：表在、该列不在
	CheckedTables  int      // 实际比对过的表数（整表缺失的不计）
	CheckedColumns int      // 实际比对过的列数
}

// OK 报告对账是否通过。实际库里多出来的列不影响结果（单向）。
func (d ColumnDiff) OK() bool {
	return len(d.MissingTables) == 0 && len(d.MissingColumns) == 0
}

// DiffColumns 单向对账：只判 expected ⊆ actual。
func DiffColumns(expected map[string][]string, actual map[string]map[string]struct{}) ColumnDiff {
	var diff ColumnDiff
	tables := make([]string, 0, len(expected))
	for t := range expected {
		tables = append(tables, t)
	}
	sort.Strings(tables)

	for _, table := range tables {
		actualCols, ok := actual[table]
		if !ok {
			diff.MissingTables = append(diff.MissingTables, table)
			continue
		}
		diff.CheckedTables++
		for _, col := range expected[table] {
			diff.CheckedColumns++
			if _, ok := actualCols[col]; !ok {
				diff.MissingColumns = append(diff.MissingColumns, table+"."+col)
			}
		}
	}
	sort.Strings(diff.MissingColumns)
	return diff
}

// CheckColumns 连库真跑单向列对账（CI 的 migration-check 与本地排查共用同一实现）。
func CheckColumns(dsn string, logger *zap.Logger) error {
	expected, err := ExpectedColumns(model.AllModels())
	if err != nil {
		return err
	}

	db, err := sql.Open("pgx", dsn)
	if err != nil {
		return fmt.Errorf("打开数据库连接失败: %w", err)
	}
	defer func() { _ = db.Close() }()

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	actual, err := ActualColumns(ctx, db)
	if err != nil {
		return err
	}

	diff := DiffColumns(expected, actual)
	if !diff.OK() {
		for _, table := range diff.MissingTables {
			logger.Error("列对账失败：整表缺失（模型有、迁移没有）", zap.String("表", table))
		}
		for _, col := range diff.MissingColumns {
			logger.Error("列对账失败：缺列（模型有、迁移没有）", zap.String("表.列", col))
		}
		return fmt.Errorf(
			"单向列对账失败：缺 %d 张表、%d 列（GORM 模型期望的列必须全部由 migrations/*.up.sql 建出来）",
			len(diff.MissingTables), len(diff.MissingColumns))
	}

	logger.Info("单向列对账通过",
		zap.Int("表", diff.CheckedTables), zap.Int("列", diff.CheckedColumns))
	return nil
}
