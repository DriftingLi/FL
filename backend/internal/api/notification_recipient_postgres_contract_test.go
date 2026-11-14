// 站内信收件人 CHECK 与章节归属的库层契约（ADR-0095 / #2537，Postgres adapter）。
//
// 为什么必须用 Postgres：`chk_notifications_recipient` 的 `CHECK (num_nonnulls(user_id, tutor_id) = 1)`
// 由迁移 000056 建出，SQLite 测试库走 AutoMigrate、不执行 migrations/，那条 CHECK 在
// SQLite 里物理上不存在。ADR-0095 收益①的全部内容是「『通知不会投给学员』从测试纪律
// 升级为 DB 约束」——若没有真 PG 判据，这条收益就是零（与 #2499 P0「sqlite 无外键 ⇒
// 投递测试假绿」同构，本票正是为修那个 P0 而生）。
package api

import (
	"testing"
	"time"

	"forklift-training/internal/core"
	"forklift-training/internal/model"
	"forklift-training/internal/testutil"
)

// TestNotificationRecipientCheckOnPostgres 迁移 000056 的 CHECK 在真库里存在且两面都拦。
func TestNotificationRecipientCheckOnPostgres(t *testing.T) {
	db := testutil.NewPostgresDB(t)

	// 1. 约束确实存在（按 schema 收窄：go test ./... 并发跑多包共用测试库、各建随机 schema，
	//    pg_constraint 是全库视图，不收窄会数到别的包那份同名对象）。
	var chkCount int64
	if err := db.Raw(`SELECT count(*) FROM pg_constraint c
		JOIN pg_class t ON t.oid = c.conrelid
		JOIN pg_namespace n ON n.oid = t.relnamespace
		WHERE n.nspname = current_schema() AND c.conname = 'chk_notifications_recipient'`).
		Scan(&chkCount).Error; err != nil {
		t.Fatalf("查 CHECK: %v", err)
	}
	if chkCount != 1 {
		t.Fatalf("迁移 000056 的 CHECK 在本 schema 不存在（%d 行）——「通知不投给错的人」退回测试纪律", chkCount)
	}

	// 2. 两列同时非空 → 拒（一条站内信只发给一个人）。
	if err := db.Exec(`INSERT INTO notifications (user_id, tutor_id, type, title, content, link, is_read, created_at)
		VALUES (1, 1, 'system', '双收件人', '内容', '', FALSE, NOW())`).Error; err == nil {
		t.Fatal("CHECK 未拦住「两列同时非空」——收件人身份可以同时是学员和讲师")
	}

	// 3. 两列同时为空 → 拒（不存在「发给没人」的通知）。
	if err := db.Exec(`INSERT INTO notifications (user_id, tutor_id, type, title, content, link, is_read, created_at)
		VALUES (NULL, NULL, 'system', '无收件人', '内容', '', FALSE, NOW())`).Error; err == nil {
		t.Fatal("CHECK 未拦住「两列同时为空」——通知可以发给没人")
	}

	// 4. 只落学员列 → 合法（存量 11 种通知的形状）。
	student := seedStudent(t, db, "chk_stu", "x")
	if err := db.Exec(`INSERT INTO notifications (user_id, tutor_id, type, title, content, link, is_read, created_at)
		VALUES (?, NULL, 'system', '学员收件', '内容', '', FALSE, NOW())`, student.ID).Error; err != nil {
		t.Fatalf("只落 user_id 应合法: %v", err)
	}

	// 5. 只落讲师列 → 合法（#2537 新形状）。
	tutor := testutil.SeedTutor(t, db, "chk_tutor", "x")
	if err := db.Exec(`INSERT INTO notifications (user_id, tutor_id, type, title, content, link, is_read, created_at)
		VALUES (NULL, ?, 'chapter_review', '讲师收件', '内容', '', FALSE, NOW())`, tutor.TutorID).Error; err != nil {
		t.Fatalf("只落 tutor_id 应合法: %v", err)
	}
}

// TestChapterTutorOwnershipOnPostgres 章节归属的外键语义（ADR-0095 决策二/八）：
//   - chapter.tutor_id 可空（存量章节作者未知）；
//   - 删讲师 → SET NULL（内容资产不随身份删除消失），不是级联删除；
//   - 指向不存在的讲师 → 外键违约。
func TestChapterTutorOwnershipOnPostgres(t *testing.T) {
	db := testutil.NewPostgresDB(t)

	spec := model.Specialty{Code: "own-pg", Name: "归属方向", SortOrder: 1, Status: 1}
	if err := db.Create(&spec).Error; err != nil {
		t.Fatalf("创建方向失败: %v", err)
	}
	lv := model.CourseLevel{Code: "own-pg-lv", Name: "入门", SortOrder: 1, Status: 1}
	if err := db.Create(&lv).Error; err != nil {
		t.Fatalf("创建等级失败: %v", err)
	}
	c := model.Course{Name: "归属课程", Status: 1, SpecialtyID: &spec.SpecialtyID,
		LevelID: &lv.LevelID, CreatedAt: time.Now()}
	if err := db.Create(&c).Error; err != nil {
		t.Fatalf("创建课程失败: %v", err)
	}

	tutor := testutil.SeedTutor(t, db, "own_tutor", "x")
	ch := model.Chapter{CourseID: c.CourseID, Title: "有归属的章", ContentType: "text",
		OrderNum: 1, TutorID: &tutor.TutorID, CreatedAt: time.Now()}
	if err := db.Create(&ch).Error; err != nil {
		t.Fatalf("创建带归属章节失败: %v", err)
	}

	// 无归属章节可写（存量形状）。
	orphan := model.Chapter{CourseID: c.CourseID, Title: "无归属的章", ContentType: "text",
		OrderNum: 2, CreatedAt: time.Now()}
	if err := db.Create(&orphan).Error; err != nil {
		t.Fatalf("无归属章节应可写（存量作者未知）: %v", err)
	}

	// 指向不存在的讲师 → 外键违约。
	if err := db.Exec(`UPDATE chapter SET tutor_id = 999999 WHERE chapter_id = ?`, orphan.ChapterID).Error; err == nil {
		t.Fatal("外键未拦住指向不存在讲师的归属")
	}

	// 删讲师 → 归属置空，章节本身还在（内容资产不随身份消失）。
	if err := db.Exec(`DELETE FROM tutor WHERE tutor_id = ?`, tutor.TutorID).Error; err != nil {
		t.Fatalf("删除讲师失败: %v", err)
	}
	var after model.Chapter
	if err := db.First(&after, ch.ChapterID).Error; err != nil {
		t.Fatalf("讲师删除后章节不应消失（ON DELETE SET NULL，不是级联）: %v", err)
	}
	if after.TutorID != nil {
		t.Fatalf("讲师删除后章节归属应置空, got %d", *after.TutorID)
	}
}

// TestTutorNotificationDeleteCascadeOnPostgres 删讲师 → 他的站内信级联删除
// （与学员侧 notifications.user_id ON DELETE CASCADE 同语义）。
func TestTutorNotificationDeleteCascadeOnPostgres(t *testing.T) {
	db := testutil.NewPostgresDB(t)
	tutor := testutil.SeedTutor(t, db, "cascade_tutor", "x")

	if err := db.Exec(`INSERT INTO notifications (user_id, tutor_id, type, title, content, link, is_read, created_at)
		VALUES (NULL, ?, 'chapter_review', '将被级联删', '内容', '', FALSE, NOW())`, tutor.TutorID).Error; err != nil {
		t.Fatalf("写讲师通知失败: %v", err)
	}
	var before int64
	if err := db.Model(&model.Notification{}).Where("tutor_id = ?", tutor.TutorID).Count(&before).Error; err != nil {
		t.Fatalf("查通知数失败: %v", err)
	}
	if before != 1 {
		t.Fatalf("写入后应有 1 条讲师通知, got %d", before)
	}

	if err := db.Exec(`DELETE FROM tutor WHERE tutor_id = ?`, tutor.TutorID).Error; err != nil {
		t.Fatalf("删除讲师失败: %v", err)
	}
	var after int64
	if err := db.Model(&model.Notification{}).Where("tutor_id = ?", tutor.TutorID).Count(&after).Error; err != nil {
		t.Fatalf("查通知数失败: %v", err)
	}
	if after != 0 {
		t.Fatalf("讲师删除后其站内信应级联删除, got %d", after)
	}
	_ = core.TutorRole // 角色常量引用占位：本文件只验库层语义，不签发 token
}
