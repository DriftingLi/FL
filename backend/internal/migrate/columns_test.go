package migrate

import (
	"context"
	"database/sql"
	"reflect"
	"strings"
	"testing"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// 夹具模型：覆盖普通列、显式列名与 gorm:"-:migration"（刻意不建列）。
type colFixture struct {
	ID      uint   `gorm:"column:id;primaryKey"`
	Name    string `gorm:"column:name"`
	Ignored string `gorm:"column:ignored;-:migration"`
}

func (colFixture) TableName() string { return "col_fixtures" }

// colDriftFixture 模拟「模型加了字段、迁移忘了建列」。
type colDriftFixture struct {
	ID    uint   `gorm:"column:id;primaryKey"`
	Name  string `gorm:"column:name"`
	Phone string `gorm:"column:phone"`
}

func (colDriftFixture) TableName() string { return "col_drift" }

// colAbsentFixture 模拟「模型有表、迁移整表没有」。
type colAbsentFixture struct {
	ID uint `gorm:"column:id;primaryKey"`
}

func (colAbsentFixture) TableName() string { return "col_absent" }

// sqliteColumnsSQL 是单测口径的「实际列」查询：只列数据库里真实存在的表与列，
// 与生产的 information_schema 查询同构（换成 SQLite 的自省表而已）。
const sqliteColumnsSQL = `SELECT m.name AS table_name, p.name AS column_name
FROM sqlite_master m JOIN pragma_table_info(m.name) p
WHERE m.type = 'table'`

func TestExpectedColumnsFromModels(t *testing.T) {
	exp, err := ExpectedColumns([]any{&colFixture{}})
	if err != nil {
		t.Fatalf("推导期望列失败: %v", err)
	}
	// Ignored 带 -:migration：AutoMigrate 不建它，对账要求它存在就是假红。
	want := []string{"id", "name"}
	if !reflect.DeepEqual(exp["col_fixtures"], want) {
		t.Fatalf("期望列 = %v，want %v", exp["col_fixtures"], want)
	}
}

// TestDiffColumnsIsOneWay 钉住单向口径：实际库多出来的列不算错（迁移里有、模型没映射，
// 或索引/历史列），只有「模型期望而实际没有」才判红。
func TestDiffColumnsIsOneWay(t *testing.T) {
	expected := map[string][]string{"t": {"id", "name"}}
	actual := map[string]map[string]struct{}{
		"t": {"id": {}, "name": {}, "legacy_extra": {}, "unmapped_index_col": {}},
	}
	diff := DiffColumns(expected, actual)
	if !diff.OK() {
		t.Fatalf("实际库多列不应判红，却报: 缺表 %v / 缺列 %v", diff.MissingTables, diff.MissingColumns)
	}
	if diff.CheckedTables != 1 || diff.CheckedColumns != 2 {
		t.Fatalf("对账计数 = %d 表 / %d 列，want 1 / 2", diff.CheckedTables, diff.CheckedColumns)
	}
}

// TestDiffColumnsAgainstRealDatabase 用真实数据库（内存 SQLite，建表语句手写以模拟
// 「迁移漏建列/漏建表」）跑完整链路：模型推导 → 数据库自省 → 单向对账。
func TestDiffColumnsAgainstRealDatabase(t *testing.T) {
	ctx := context.Background()
	db := openSQLiteForColumns(t)

	// 迁移 1：col_fixtures 建对了，并且库里多一列（负样本）。
	if _, err := db.ExecContext(ctx, `CREATE TABLE col_fixtures (id INTEGER PRIMARY KEY, name TEXT NOT NULL)`); err != nil {
		t.Fatalf("建表失败: %v", err)
	}
	if _, err := db.ExecContext(ctx, `ALTER TABLE col_fixtures ADD COLUMN legacy_extra TEXT`); err != nil {
		t.Fatalf("加列失败: %v", err)
	}
	// 迁移 2：col_drift 漏了模型上的 phone 列（正样本：必须报出来）。
	if _, err := db.ExecContext(ctx, `CREATE TABLE col_drift (id INTEGER PRIMARY KEY, name TEXT NOT NULL)`); err != nil {
		t.Fatalf("建表失败: %v", err)
	}
	// col_absent 整表没建（正样本：必须报缺表）。

	expected, err := ExpectedColumns([]any{&colFixture{}, &colDriftFixture{}, &colAbsentFixture{}})
	if err != nil {
		t.Fatalf("推导期望列失败: %v", err)
	}
	actual, err := queryColumns(ctx, db, sqliteColumnsSQL)
	if err != nil {
		t.Fatalf("读取实际列失败: %v", err)
	}

	diff := DiffColumns(expected, actual)
	if diff.OK() {
		t.Fatal("缺列/缺表必须判红，却报了通过")
	}
	if want := []string{"col_drift.phone"}; !reflect.DeepEqual(diff.MissingColumns, want) {
		t.Fatalf("缺列 = %v，want %v（必须指名「表.列」，且不得把实际库多出来的 legacy_extra 算进来）",
			diff.MissingColumns, want)
	}
	if want := []string{"col_absent"}; !reflect.DeepEqual(diff.MissingTables, want) {
		t.Fatalf("缺表 = %v，want %v", diff.MissingTables, want)
	}
	if diff.CheckedTables != 2 || diff.CheckedColumns != 5 {
		t.Fatalf("对账计数 = %d 表 / %d 列，want 2 / 5", diff.CheckedTables, diff.CheckedColumns)
	}
}

// TestExpectedColumnsRejectsEmptyModelSet 兜底（fail-closed）：期望列集合为空必须直接报错，
// 不得让 CheckColumns 落到「对账通过 表=0 列=0」（AllModels() 变空 / 传参写错时的假绿面）。
func TestExpectedColumnsRejectsEmptyModelSet(t *testing.T) {
	if got, err := ExpectedColumns(nil); err == nil {
		t.Fatalf("空模型列表必须报错，实际返回 %v", got)
	}
	if got, err := ExpectedColumns([]any{}); err == nil {
		t.Fatalf("空模型切片必须报错，实际返回 %v", got)
	}
	// 反例：非空模型仍必须走通（兜底不得误伤正常路径）。
	if _, err := ExpectedColumns([]any{&colFixture{}}); err != nil {
		t.Fatalf("非空模型不得被兜底拦下: %v", err)
	}
}

// TestInformationSchemaColumnsSQLShape 生产口径 SQL 的形状锁（#1099 的 L 面）：
// 单测跑的是 SQLite 自省（sqliteColumnsSQL），CI 真正执行的却是 informationSchemaColumnsSQL ——
// 这条断言把它钉在「只读 information_schema.columns、按 current_schema() 限定、只取两列且列序
// 与 queryColumns 的 Scan 顺序一致」上；改成别的表/多取列/换列序都必须在这里变红。
func TestInformationSchemaColumnsSQLShape(t *testing.T) {
	flat := strings.Join(strings.Fields(informationSchemaColumnsSQL), " ")
	lower := strings.ToLower(flat)
	if !strings.HasPrefix(lower, "select table_name, column_name from information_schema.columns") {
		t.Fatalf("SQL 的 SELECT 形态变了（Scan 顺序绑定 table_name, column_name）：%s", flat)
	}
	if !strings.Contains(lower, "where table_schema = current_schema()") {
		t.Fatalf("SQL 必须按 current_schema() 限定对账范围：%s", flat)
	}
	if n := strings.Count(flat, ","); n != 1 {
		t.Fatalf("SQL 只允许取 table_name / column_name 两列（逗号数 = %d）：%s", n, flat)
	}
}

func openSQLiteForColumns(t *testing.T) *sql.DB {
	t.Helper()
	gdb, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	if err != nil {
		t.Fatalf("打开内存库失败: %v", err)
	}
	db, err := gdb.DB()
	if err != nil {
		t.Fatalf("取 *sql.DB 失败: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	return db
}
