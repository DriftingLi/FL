// 关键唯一索引对账的单测（#1362，无 PG 也能红的那半）。
//
// 本文件是 #1362 判据 2「列对账在删掉任一索引时能判红」的本机承载：check-columns 连库那半
// 只能在 CI 的 postgres 服务上真跑，而这里三条不连库的锁覆盖同一件事——
//  1. 登记表 ⇔ migrations/*.up.sql 逐字相等（删/改任一侧的任一句即红）；
//  2. 逐条删除测试：对登记表里**每一条**索引做一次「从迁移侧抹掉」，逐个都必须判红
//     （防恒绿：整表断言只在一条上生效时，删别的索引照样绿）；
//  3. 存在性对账的单向口径与 SQL 形状（schema 收窄，见 checks.md 两条纪律）。
package migrate

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/glebarez/sqlite"
	"go.uber.org/zap"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// migrationsDir 复用生产的路径解析，避免测试与实现各写一套目录上溯逻辑。
func migrationsDir(t *testing.T) string {
	t.Helper()
	dir, err := resolveMigrationsDir(zap.NewNop())
	if err != nil {
		t.Fatalf("定位 migrations 目录失败: %v", err)
	}
	return dir
}

// TestCriticalUniqueIndexesMatchMigrations 判据 1：登记表与迁移逐字相等（双向）。
// 任一侧漂移都在这里红：迁移删了一句、改了一个谓词、新加了一条唯一索引没登记。
func TestCriticalUniqueIndexesMatchMigrations(t *testing.T) {
	fromMigrations, err := UniqueIndexesFromMigrationsDir(migrationsDir(t))
	if err != nil {
		t.Fatalf("解析 migrations 失败: %v", err)
	}
	registry := CriticalUniqueIndexes()
	if len(registry) == 0 {
		t.Fatal("登记表为空：对账会在「0 条」上假绿")
	}
	if diff := DiffUniqueIndexRegistry(registry, fromMigrations); !diff.OK() {
		t.Fatalf("登记表与迁移逐字对账失败：\n%s", strings.Join(
			append(append(append([]string{}, diff.MissingInMigrations...), diff.MissingInRegistry...), diff.Mismatched...), "\n"))
	}
	t.Logf("逐字对账通过：登记表 %d 条 = 迁移解析 %d 条", len(registry), len(fromMigrations))
}

// TestCriticalUniqueIndexesCoverPartialIndexes 覆盖面：登记表必须把迁移里**全部**
// CREATE UNIQUE INDEX 收进来，且偏索引（带 WHERE）一条不漏——票面点名的几族逐个点名，
// 免得「登记表非空」被当成判据。
func TestCriticalUniqueIndexesCoverPartialIndexes(t *testing.T) {
	dir := migrationsDir(t)
	all, err := UniqueIndexesFromMigrationsDir(dir)
	if err != nil {
		t.Fatalf("解析 migrations 失败: %v", err)
	}
	names := map[string]bool{}
	partial := map[string]bool{}
	for _, s := range all {
		names[s.Name] = true
		if strings.Contains(strings.ToUpper(s.DDL), " WHERE ") {
			partial[s.Name] = true
		}
	}
	// 票面清单 + 自己 grep 出的全量核对结果（12 条，其中偏索引 9 条）。
	wantPartial := []string{
		"idx_hrwai_users_email_unique", "idx_hrwai_users_wechat_openid_unique", "idx_hrwai_users_phone_unique",
		"uq_points_task_claim_daily", "uq_points_task_claim_ref",
		"idx_contact_requests_pending_unique", "idx_job_applications_applied_unique",
		"uq_practice_progress_cred", "uq_practice_progress_nocred",
	}
	for _, n := range wantPartial {
		if !partial[n] {
			t.Fatalf("偏唯一索引 %s 没在迁移里解析到 WHERE（解析器失守或迁移被改）", n)
		}
	}
	if len(partial) != len(wantPartial) {
		t.Fatalf("迁移里的偏唯一索引数 = %d，want %d（新增偏索引须同步登记表与本清单）：%v",
			len(partial), len(wantPartial), keysOf(partial))
	}
	if len(all) != len(CriticalUniqueIndexes()) {
		t.Fatalf("迁移解析 %d 条 ≠ 登记表 %d 条", len(all), len(CriticalUniqueIndexes()))
	}
}

func keysOf(m map[string]bool) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	return out
}

// TestUniqueIndexesFromSQLShape 解析器自身的正负样本：跨行语句、注释里的假语句、
// 非唯一索引、IF NOT EXISTS 四种形状都要判对。
// 这条锁的存在理由：登记表 ⇔ 迁移的对账如果拿一个失灵的正则去比，两侧会「一起是空」而判绿。
func TestUniqueIndexesFromSQLShape(t *testing.T) {
	sql := `
-- CREATE UNIQUE INDEX commented_out ON fake_table (col);
CREATE UNIQUE INDEX idx_a
    ON tbl_a (email)
    WHERE email <> '';
CREATE UNIQUE INDEX IF NOT EXISTS idx_b ON tbl_b(x, y) WHERE x IS NOT NULL;
CREATE INDEX idx_not_unique ON tbl_c (z);
CREATE UNIQUE INDEX idx_c ON tbl_c (z);
`
	got := UniqueIndexesFromSQL(sql)
	var names []string
	for _, g := range got {
		names = append(names, g.Name)
	}
	want := "idx_a,idx_b,idx_c"
	if strings.Join(names, ",") != want {
		t.Fatalf("解析到的索引名 = %s，want %s（注释里的假语句不得被数进来，非唯一索引同理）",
			strings.Join(names, ","), want)
	}
	byName := map[string]UniqueIndexSpec{}
	for _, g := range got {
		byName[g.Name] = g
	}
	if byName["idx_a"].DDL != "CREATE UNIQUE INDEX idx_a ON tbl_a (email) WHERE email <> ''" {
		t.Fatalf("跨行语句归一化结果不符：%q", byName["idx_a"].DDL)
	}
	if byName["idx_a"].Table != "tbl_a" || byName["idx_b"].Table != "tbl_b" {
		t.Fatalf("宿主表解析错：%q / %q", byName["idx_a"].Table, byName["idx_b"].Table)
	}
	if !strings.Contains(byName["idx_b"].DDL, "IF NOT EXISTS") {
		t.Fatalf("IF NOT EXISTS 被归一化吃掉了：%q", byName["idx_b"].DDL)
	}
}

// TestUniqueIndexesFromMigrationsDirFailClosed 空目录 / 没有任何唯一索引的目录必须报错，
// 不得让对账在「0 条」上判通过。
func TestUniqueIndexesFromMigrationsDirFailClosed(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "000001_x.up.sql"),
		[]byte("CREATE TABLE t (id INT);\nCREATE INDEX t_id ON t (id);\n"), 0o600); err != nil {
		t.Fatalf("写夹具失败: %v", err)
	}
	if _, err := UniqueIndexesFromMigrationsDir(dir); err == nil {
		t.Fatal("目录里没有任何 CREATE UNIQUE INDEX 必须报错（防空转）")
	}
	if _, err := UniqueIndexesFromMigrationsDir(filepath.Join(dir, "nope")); err == nil {
		t.Fatal("目录不存在必须报错")
	}
}

// TestDeletingAnyIndexTurnsReconciliationRed 判据 2 的本机承载：逐条删除、逐条必须红。
// 两个方向各跑一遍（迁移侧删 / 登记表侧删），并打印命中的条数——命中数不等于登记表条数
// 就说明循环没真的跑满，别拿「全绿」当结论。
func TestDeletingAnyIndexTurnsReconciliationRed(t *testing.T) {
	all, err := UniqueIndexesFromMigrationsDir(migrationsDir(t))
	if err != nil {
		t.Fatalf("解析 migrations 失败: %v", err)
	}
	registry := CriticalUniqueIndexes()

	migRed, registryRed := 0, 0
	for i := range all {
		// 方向一：从「迁移」里抹掉这一条 ⇒ 登记表要求、迁移没有 ⇒ 必须红。
		without := append(append([]UniqueIndexSpec{}, all[:i]...), all[i+1:]...)
		if d := DiffUniqueIndexRegistry(registry, without); d.OK() {
			t.Fatalf("删掉迁移侧的 %s 之后对账仍判绿", all[i].Name)
		}
		migRed++
		// 方向二：从「登记表」里抹掉同一条 ⇒ 迁移有、登记表没有 ⇒ 也必须红。
		j := indexOfSpec(registry, all[i].Name)
		if j < 0 {
			t.Fatalf("登记表里没有 %s，前面的逐字对账不可能通过", all[i].Name)
		}
		regWithout := append(append([]UniqueIndexSpec{}, registry[:j]...), registry[j+1:]...)
		if d := DiffUniqueIndexRegistry(regWithout, all); d.OK() {
			t.Fatalf("删掉登记表侧的 %s 之后对账仍判绿", all[i].Name)
		}
		registryRed++
	}
	if migRed != len(all) || registryRed != len(all) {
		t.Fatalf("逐条删除命中数 = 迁移侧 %d / 登记表侧 %d，应各等于 %d", migRed, registryRed, len(all))
	}
	t.Logf("逐条删除判红命中 %d/%d（两侧各 %d 条）", migRed, len(all), registryRed)
}

// TestIndexPredicateDriftTurnsReconciliationRed 改了谓词（把 WHERE 换成另一个条件）
// 必须红——这是「两处漂移」里最贵的一种：约束看起来还在，语义已经变了。
func TestIndexPredicateDriftTurnsReconciliationRed(t *testing.T) {
	all, err := UniqueIndexesFromMigrationsDir(migrationsDir(t))
	if err != nil {
		t.Fatalf("解析 migrations 失败: %v", err)
	}
	drifted := append([]UniqueIndexSpec{}, all...)
	drifted[0].DDL = strings.Replace(drifted[0].DDL, "WHERE", "WHERE 1=1 AND", 1)
	if d := DiffUniqueIndexRegistry(CriticalUniqueIndexes(), drifted); d.OK() {
		t.Fatal("改掉了 WHERE 谓词后对账仍判绿")
	}
	// 宿主表写错也要红（存在性对账按「表.索引名」比，登记表侧的表名同样得逐字）。
	wrongTable := append([]UniqueIndexSpec{}, all...)
	wrongTable[0].Table = "some_other_table"
	if d := DiffUniqueIndexRegistry(CriticalUniqueIndexes(), wrongTable); d.OK() {
		t.Fatal("迁移侧宿主表被改名后对账仍判绿")
	}
}

func indexOfSpec(registry []UniqueIndexSpec, name string) int {
	for i, r := range registry {
		if r.Name == name {
			return i
		}
	}
	return -1
}

// TestDiffUniqueIndexesOneWay 存在性对账的单向口径：库里多出来的索引不算错
// （非唯一索引、历史索引都在 pg_indexes 里），只有登记表要求的缺失才判红。
func TestDiffUniqueIndexesOneWay(t *testing.T) {
	expected := []UniqueIndexSpec{{Name: "uq_a", Table: "ta"}, {Name: "uq_b", Table: "tb"}}
	actual := map[string]struct{}{"ta.uq_a": {}, "tb.extra_idx": {}, "tc.uq_unrelated": {}}
	missing := DiffUniqueIndexes(expected, actual)
	if len(missing) != 1 || missing[0] != "tb.uq_b" {
		t.Fatalf("缺失项 = %v，want [tb.uq_b]（不得把库里多出来的索引算进红色）", missing)
	}
	// 反例：两条都在 ⇒ 空。
	if got := DiffUniqueIndexes(expected, map[string]struct{}{"ta.uq_a": {}, "tb.uq_b": {}}); len(got) != 0 {
		t.Fatalf("两条都在却报缺：%v", got)
	}
}

// TestPgIndexesSQLShape 生产口径 SQL 的形状锁：pg_indexes 是全库视图，
// 不收窄到 current_schema() 就会数到别的包那份同名对象（checks.md 第二条纪律）。
func TestPgIndexesSQLShape(t *testing.T) {
	flat := strings.Join(strings.Fields(pgIndexesSQL), " ")
	lower := strings.ToLower(flat)
	if !strings.HasPrefix(lower, "select tablename, indexname from pg_indexes") {
		t.Fatalf("SQL 的 SELECT 形态变了（Scan 顺序绑定 tablename, indexname）：%s", flat)
	}
	if !strings.Contains(lower, "where schemaname = current_schema()") {
		t.Fatalf("SQL 必须按 current_schema() 收窄：%s", flat)
	}
	if n := strings.Count(flat, ","); n != 1 {
		t.Fatalf("SQL 只允许取 tablename / indexname 两列（逗号数 = %d）：%s", n, flat)
	}
}

// TestActualUniqueIndexesAgainstRealDatabase 用真实数据库（内存 SQLite 建表 + 补同句 DDL）
// 跑完整链路：登记表 DDL 可执行 → 自省得到索引 → 存在性对账通过；删掉一句 DDL 即红。
// 这条锁住的是 testutil 建库路径的形状（Exec 得动、名字落得下），不是伪造的 map。
func TestActualUniqueIndexesAgainstRealDatabase(t *testing.T) {
	gdb, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	if err != nil {
		t.Fatalf("打开内存库失败: %v", err)
	}
	db, err := gdb.DB()
	if err != nil {
		t.Fatalf("取 *sql.DB 失败: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })

	ctx := context.Background()
	if _, err := db.ExecContext(ctx, `CREATE TABLE practice_progress (
		id INTEGER PRIMARY KEY, student_id INT, practice_mode TEXT, credential_id INT)`); err != nil {
		t.Fatalf("建表失败: %v", err)
	}
	// 与 000013 逐字同句的两条偏索引（登记表里的原文）。
	targets := []UniqueIndexSpec{}
	for _, s := range CriticalUniqueIndexes() {
		if s.Table == "practice_progress" {
			targets = append(targets, s)
		}
	}
	if len(targets) != 2 {
		t.Fatalf("登记表里 practice_progress 的偏索引 = %d 条，want 2", len(targets))
	}
	for _, s := range targets {
		if _, err := db.ExecContext(ctx, s.DDL); err != nil {
			t.Fatalf("执行登记表 DDL 失败（%s）: %v\n%s", s.Name, err, s.DDL)
		}
	}

	actual, err := actualIndexes(ctx, db, sqliteIndexesSQL)
	if err != nil {
		t.Fatalf("自省索引失败: %v", err)
	}
	if missing := DiffUniqueIndexes(targets, actual); len(missing) != 0 {
		t.Fatalf("补跑 DDL 后仍报缺索引：%v", missing)
	}

	// 负样本：只建其中一条时，另一条必须被报出来（存在性对账真的在看库）。
	onlyOne := map[string]struct{}{targets[0].Table + "." + targets[0].Name: {}}
	if missing := DiffUniqueIndexes(targets, onlyOne); len(missing) != 1 {
		t.Fatalf("少建一条索引时报缺 %d 条，want 1：%v", len(missing), missing)
	}
}

// sqliteIndexesSQL 是本单测口径的「实际索引」查询，与生产的 pgIndexesSQL 同构
// （换成 SQLite 的 sqlite_master，列序保持 tablename, indexname 与 actualIndexes 的 Scan 一致）。
const sqliteIndexesSQL = `SELECT tbl_name, name FROM sqlite_master WHERE type = 'index'`
