// 注销清理表 + 事务自证的 Postgres 契约（#1356 判据 1 / spec #1345 决策 10 / 真实缺陷 #6）。
//
// 为什么必须用 PG：缺陷 #6 的形态是「某条语句把 PG 事务打进 aborted 态 ⇒ 之后的语句与 COMMIT
// 全部空转（aborted 事务上的 COMMIT 等价 ROLLBACK 且不报错），接口照旧回 200，数据一行没少」。
// SQLite 的语句报错不废整笔事务，内存库测不到这一层（同 ADR-0044 / #1197 的口径）。
// 本机无 DATABASE_URL 时 testutil.NewPostgresDB 干净 skip ⇒ 这条的首跑在 CI（backend-test 带 PG 15）。
package api

import (
	"net/http"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"forklift-training/internal/middleware"
	"forklift-training/internal/model"
	"forklift-training/internal/testutil"
)

// newDeleteAccountPGRouter 装配一个带登录态的 DELETE /api/auth/account（PG 库，黑名单走内存实现）。
// seam 形态照 session_termination_contract_test.go 的 newDeleteAccountRouter，只把测试库换成真实迁移建起来的 PG。
// db 必须由调用方传入并复用：testutil.NewPostgresDB 每次建**独立随机 schema**，各建各的就变成
// 「播种在一个 schema、注销打在另一个 schema」，判据恒绿。
func newDeleteAccountPGRouter(t *testing.T, db *gorm.DB, uid int) *gin.Engine {
	t.Helper()
	gin.SetMode(gin.TestMode)
	deps := newContractDeps(t, db, nil)
	r := gin.New()
	g := r.Group("/api/auth", func(c *gin.Context) {
		c.Set(string(middleware.CtxUserID), uid)
		c.Next()
	})
	g.DELETE("/account", deps.AuthH.DeleteAccount)
	return r
}

// 判据 1：注入某一条删除失败 ⇒ 接口非 2xx，且主行仍在（整笔回滚，不是半删）。
func TestDeleteAccountOnPostgres_注入清理失败则非2xx且主行仍在(t *testing.T) {
	db := testutil.NewPostgresDB(t)
	student := testutil.SeedStudent(t, db, "del_pg_stu", "hash")
	now := testutil.Now()

	// 清理表靠前的一张表（favorite 在第 1 行、note 在第 16 行）：note 失败时它已被删过，
	// 它还在不在就是「整笔回滚 vs 半删」的判据。
	if err := db.Create(&model.Favorite{
		UserID: student.ID, TargetType: "topic", TargetID: 777, CreatedAt: now,
	}).Error; err != nil {
		t.Fatalf("播种收藏行失败: %v", err)
	}
	// 故障注入：把清理表里的一张依赖表从库里抹掉 ⇒ 那一行的 DELETE 必失败。
	if err := db.Exec("DROP TABLE IF EXISTS note CASCADE").Error; err != nil {
		t.Fatalf("DROP TABLE note 失败: %v", err)
	}

	r := newDeleteAccountPGRouter(t, db, student.ID)
	rec := performRequest(r, http.MethodDelete, "/api/auth/account")
	if rec.Code >= http.StatusOK && rec.Code < http.StatusMultipleChoices {
		t.Fatalf("清理语句失败时注销不得回 2xx，实际 %d，body=%s", rec.Code, rec.Body.String())
	}
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("注销失败应映射为 400，实际 %d，body=%s", rec.Code, rec.Body.String())
	}

	count := func(dst any, column string, value int) int64 {
		var n int64
		if err := db.Model(dst).Where(column+" = ?", value).Count(&n).Error; err != nil {
			t.Fatalf("查 %T.%s 失败: %v", dst, column, err)
		}
		return n
	}
	if n := count(&model.HrwaiUser{}, "id", student.ID); n != 1 {
		t.Errorf("注销未生效时主行应仍在，实际 %d 行", n)
	}
	if n := count(&model.Favorite{}, "user_id", student.ID); n != 1 {
		t.Errorf("靠前的删除步骤也应随事务回滚（favorite 应仍为 1 行），实际 %d 行 ⇒ 半删", n)
	}

	// 对外文案不泄露库的结构：表名、驱动原文、SQLSTATE 都不许出现在响应里。
	body := rec.Body.String()
	for _, leak := range []string{"note", "relation", "SQLSTATE", "hrwai", "user_id"} {
		if strings.Contains(body, leak) {
			t.Errorf("400 正文不应出现内部结构 %q，实际 body=%s", leak, body)
		}
	}
}

// 对照组（防恒绿）：没有故障注入时 PG 上注销必须真的生效——上面那条用例不能靠「注销在 PG 上
// 从来就失败」通过。判据是 200 + 主行确实不在 + 论坛内容已匿名化（占位用户接管）。
func TestDeleteAccountOnPostgres_无故障时注销真的生效(t *testing.T) {
	db := testutil.NewPostgresDB(t)
	student := testutil.SeedStudent(t, db, "del_pg_ok", "hash")
	now := testutil.Now()
	topic := model.ForumTopic{
		Category: "discussion", UserID: student.ID, Title: "注销前发的帖子", Content: "c",
		Images: model.JSONB("[]"), CreatedAt: now, UpdatedAt: now,
	}
	if err := db.Create(&topic).Error; err != nil {
		t.Fatalf("播种帖子失败: %v", err)
	}
	if err := db.Create(&model.Favorite{
		UserID: student.ID, TargetType: "topic", TargetID: 778, CreatedAt: now,
	}).Error; err != nil {
		t.Fatalf("播种收藏行失败: %v", err)
	}

	r := newDeleteAccountPGRouter(t, db, student.ID)
	if rec := performRequest(r, http.MethodDelete, "/api/auth/account"); rec.Code != http.StatusOK {
		t.Fatalf("无故障时注销应 200，实际 %d，body=%s", rec.Code, rec.Body.String())
	}

	var left int64
	if err := db.Model(&model.HrwaiUser{}).Where("id = ?", student.ID).Count(&left).Error; err != nil {
		t.Fatalf("查主行失败: %v", err)
	}
	if left != 0 {
		t.Fatalf("注销后主行应已不存在（PG 上静默回滚就是缺陷 #6），实际 %d 行", left)
	}
	var fav int64
	if err := db.Model(&model.Favorite{}).Where("user_id = ?", student.ID).Count(&fav).Error; err != nil {
		t.Fatalf("查收藏行失败: %v", err)
	}
	if fav != 0 {
		t.Errorf("注销后收藏行应清零，实际 %d 行", fav)
	}
	var owned int64
	if err := db.Model(&model.ForumTopic{}).Where("user_id = ?", student.ID).Count(&owned).Error; err != nil {
		t.Fatalf("查帖子归属失败: %v", err)
	}
	if owned != 0 {
		t.Errorf("注销后帖子应已匿名化（不再挂在本人名下），实际 %d 行", owned)
	}
	var sentinel model.HrwaiUser
	if err := db.Where("account = ?", "__deleted_user").First(&sentinel).Error; err != nil {
		t.Fatalf("匿名占位用户应存在: %v", err)
	}
	var anon int64
	if err := db.Model(&model.ForumTopic{}).Where("user_id = ?", sentinel.ID).Count(&anon).Error; err != nil {
		t.Fatalf("查匿名归属失败: %v", err)
	}
	if anon != 1 {
		t.Errorf("帖子应重分配给占位用户，实际 %d 行", anon)
	}
}
