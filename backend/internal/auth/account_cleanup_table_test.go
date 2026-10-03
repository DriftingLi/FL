// 注销清理表与事务自证的单测（#1356 / spec #1345 决策 10 / 真实缺陷 #6）。
//
// seam：Service.DeleteAccount 的整笔事务（判据 1 与 2 的失败注入都落在「事务里的一条语句」
// 这一层，用 GORM 回调与 DROP TABLE 注入，不改生产代码形状）；观察点只有两个——返回的错误与
// 库里的行。PG 那一半的契约面在同目录的 account_deletion_postgres_contract_test.go。
package auth

import (
	"sort"
	"strings"
	"testing"

	"gorm.io/gorm"

	"forklift-training/internal/model"
	"forklift-training/internal/testutil"
)

// recordDeletedTables 注册一条 Delete 回调，记录本次事务里**真正执行过**的表名（测试侧观察点，
// 用来把「声明表」与「删除序列」钉成同一份事实）。
func recordDeletedTables(t *testing.T, db *gorm.DB) *[]string {
	t.Helper()
	seen := &[]string{}
	if err := db.Callback().Delete().After("gorm:delete").Register("1356:record-table", func(tx *gorm.DB) {
		*seen = append(*seen, tx.Statement.Table)
	}); err != nil {
		t.Fatalf("注册删除记录回调失败: %v", err)
	}
	t.Cleanup(func() {
		_ = db.Callback().Delete().After("gorm:delete").Remove("1356:record-table")
	})
	return seen
}

// 判据 3 的形状锁①：声明表自洽——表名不重复、字段齐全，且每行的 table/条件列都是所指模型的
// 真实表名与真实列。列名写错在旧实现里会被咽掉（PG 上整笔静默回滚、接口却回 200），现在必须有名字可指。
func TestAccountCleanupSteps_声明表自洽(t *testing.T) {
	_, db := newAuthSvc(t)
	seenTable := map[string]bool{}
	for i, step := range accountCleanupSteps {
		if step.dest == nil {
			t.Fatalf("清理表第 %d 行没有 dest 模型（删除序列无从执行）", i+1)
		}
		if step.column == "" || step.table == "" {
			t.Fatalf("清理表第 %d 行缺 table 或 column 字段", i+1)
		}
		if seenTable[step.table] {
			t.Errorf("表 %s 在清理表里出现两次（第 %d 行与更早的行重复）——删除序列会把它删两遍", step.table, i+1)
		}
		seenTable[step.table] = true

		stmt := &gorm.Statement{DB: db}
		if err := stmt.Parse(step.dest()); err != nil {
			t.Fatalf("解析清理表第 %d 行的模型失败: %v", i+1, err)
		}
		if stmt.Schema == nil || stmt.Schema.Table != step.table {
			t.Errorf("清理表第 %d 行的 table=%q 与模型的 TableName 不符（实际 %q）——报错与日志指认的是假现场",
				i+1, step.table, stmt.Schema.Table)
		}
		if stmt.Schema.LookUpField(step.column) == nil {
			t.Errorf("清理表第 %d 行声明的条件列 %q 在表 %s 上不存在（PG 上 DELETE 必报错）",
				i+1, step.column, step.table)
		}
		if step.likesTarget != "" && stmt.Schema.LookUpField(step.likesTarget) == nil {
			t.Errorf("清理表第 %d 行声明的回扣目标列 %q 在表 %s 上不存在", i+1, step.likesTarget, step.table)
		}
	}
}

// 判据 3 的形状锁②：删除序列执行的就是声明表里那些行，一行不多、一行不少。
// 有人绕过表手写一条 DELETE ⇒ 这里多出「表里没有」的判红；表里加了行却没被执行 ⇒ 反向判红。
func TestDeleteAccount_清理表是删除序列的唯一事实源(t *testing.T) {
	svc, db := newAuthSvc(t)
	stu := testutil.SeedStudent(t, db, "sole_source", "hash")
	seen := recordDeletedTables(t, db)

	if err := svc.DeleteAccount(stu.ID); err != nil {
		t.Fatalf("注销失败: %v", err)
	}

	// 主行不在清理表里（它后面紧跟着事务内自证，见 DeleteAccount），期望集 = 声明表 ∪ {主行}。
	want := map[string]bool{model.HrwaiUser{}.TableName(): true}
	for _, step := range accountCleanupSteps {
		want[step.table] = true
	}
	got := map[string]int{}
	for _, tbl := range *seen {
		got[tbl]++
	}
	var extra, missing []string
	for tbl, n := range got {
		if !want[tbl] {
			extra = append(extra, tbl)
		}
		if n > 1 {
			t.Errorf("表 %s 被删除了 %d 次（同一条事实不该有两个删除动作）", tbl, n)
		}
	}
	for tbl := range want {
		if got[tbl] == 0 {
			missing = append(missing, tbl)
		}
	}
	sort.Strings(extra)
	sort.Strings(missing)
	if len(extra) > 0 {
		t.Errorf("删除序列执行了清理表之外的表：%v（清单被抄成了第二份）", extra)
	}
	if len(missing) > 0 {
		t.Errorf("清理表声明的表没有被删除序列执行：%v（表加了行、序列没跑）", missing)
	}
	if len(*seen) != len(want) {
		t.Fatalf("执行的 DELETE 语句 %d 条，期望 %d 条", len(*seen), len(want))
	}
}

// 判据 1 的本机孪生（PG 版在 internal/api 的 *_postgres_contract_test.go，本机无 PG 时干净 skip）：
// 注入某一条删除失败 ⇒ DeleteAccount 报错，且整笔事务回滚（主行与靠前已执行的依赖行都还在）。
func TestDeleteAccount_某条清理失败则整笔不生效(t *testing.T) {
	svc, db := newAuthSvc(t)
	stu := testutil.SeedStudent(t, db, "cleanup_fail", "hash")
	now := testutil.Now()
	if err := db.Create(&model.Favorite{UserID: stu.ID, TargetType: "topic", TargetID: 777, CreatedAt: now}).Error; err != nil {
		t.Fatalf("播种收藏行失败: %v", err)
	}
	topic := model.ForumTopic{UserID: stu.ID, Title: "注销前的帖子", Content: "c", CreatedAt: now, UpdatedAt: now}
	if err := db.Create(&topic).Error; err != nil {
		t.Fatalf("播种帖子失败: %v", err)
	}

	// 故障注入：清理表里靠后的一张表在库里不存在 ⇒ 那一行的 DELETE 必失败。
	// favorite 那张表在它之前已被删除，正好用来验「不是半删」。
	if err := db.Exec("DROP TABLE note").Error; err != nil {
		t.Fatalf("_DROP TABLE note 失败: %v", err)
	}

	err := svc.DeleteAccount(stu.ID)
	if err == nil {
		t.Fatal("清理语句失败时注销应报错，实际返回 nil（正是缺陷 #6 的静默形态）")
	}
	if !strings.Contains(err.Error(), "note") {
		t.Errorf("错误应点名失败的那张表，实际: %q", err.Error())
	}
	assertRowCount(t, db, "hrwai_users", "id", stu.ID, 1, "清理失败后主行应仍在（整笔回滚）")
	assertRowCount(t, db, "favorite", "user_id", stu.ID, 1, "已执行过的靠前删除也应被回滚（不许半删）")
	assertRowCount(t, db, "forum_topics", "user_id", stu.ID, 1, "匿名化 UPDATE 也应被回滚")
}

// 判据 2：自证失败路径——清理与主删除全部「成功」、主行却仍在（删除打到了一张同名结构的空表上）
// ⇒ 事务内自证必须判失败并整笔回滚，绝不允许接口报成功。
func TestDeleteAccount_自证失败则整笔回滚(t *testing.T) {
	svc, db := newAuthSvc(t)
	stu := testutil.SeedStudent(t, db, "self_proof", "hash")
	now := testutil.Now()
	if err := db.Create(&model.Favorite{UserID: stu.ID, TargetType: "topic", TargetID: 888, CreatedAt: now}).Error; err != nil {
		t.Fatalf("播种收藏行失败: %v", err)
	}
	// 同名结构的空表：主行删除被改打到这里，语句成功、0 行受影响，主行原封不动。
	if err := db.Exec("CREATE TABLE hrwai_users_ghost AS SELECT * FROM hrwai_users WHERE 0").Error; err != nil {
		t.Fatalf("建影子表失败: %v", err)
	}
	if err := db.Callback().Delete().Before("gorm:delete").Register("1356:divert-main-delete", func(tx *gorm.DB) {
		if tx.Statement.Table == "hrwai_users" {
			tx.Statement.Table = "hrwai_users_ghost"
		}
	}); err != nil {
		t.Fatalf("注册改道回调失败: %v", err)
	}
	t.Cleanup(func() {
		_ = db.Callback().Delete().Before("gorm:delete").Remove("1356:divert-main-delete")
	})

	err := svc.DeleteAccount(stu.ID)
	if err == nil {
		t.Fatal("主行仍在时注销必须报错，实际返回 nil（静默半删）")
	}
	if !strings.Contains(err.Error(), "主行仍在") {
		t.Errorf("应命中自证失败这一支，实际: %q", err.Error())
	}
	assertRowCount(t, db, "hrwai_users", "id", stu.ID, 1, "自证失败应整笔回滚，主行仍在")
	assertRowCount(t, db, "favorite", "user_id", stu.ID, 1, "自证失败应整笔回滚，依赖行也仍在")
	assertRowCount(t, db, "hrwai_users_ghost", "id", stu.ID, 0, "回滚后影子表也应是空的")
}

// 判据 2 的另一半：自证出口本身（提交后那一遍走的是同一个函数，只是换一条连接读）。
func TestProveAccountGone_主行在与不在(t *testing.T) {
	svc, db := newAuthSvc(t)
	stu := testutil.SeedStudent(t, db, "proof_exit", "hash")

	if err := svc.proveAccountGone(db, stu.ID); err == nil || !strings.Contains(err.Error(), "主行仍在") {
		t.Fatalf("主行仍在时自证应报「主行仍在」，实际: %v", err)
	}
	if err := db.Where("id = ?", stu.ID).Delete(&model.HrwaiUser{}).Error; err != nil {
		t.Fatalf("删除主行失败: %v", err)
	}
	if err := svc.proveAccountGone(db, stu.ID); err != nil {
		t.Fatalf("主行已不存在时自证应通过，实际: %v", err)
	}
}

// assertRowCount 按「表 + 列 = 值」数行（测试侧观察点，不碰生产代码）。
func assertRowCount(t *testing.T, db *gorm.DB, table, column string, value, want int, msg string) {
	t.Helper()
	var got int64
	if err := db.Table(table).Where(column+" = ?", value).Count(&got).Error; err != nil {
		t.Fatalf("查 %s.%s 失败: %v", table, column, err)
	}
	if got != int64(want) {
		t.Errorf("%s：期望 %d 行，实际 %d 行", msg, want, got)
	}
}
