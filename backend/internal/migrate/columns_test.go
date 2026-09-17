package migrate

import (
	"context"
	"database/sql"
	"reflect"
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
