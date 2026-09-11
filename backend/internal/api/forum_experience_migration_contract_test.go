// 契约测试 ADR-0040（Postgres adapter）：迁移 000027 落地的约束与副作用。
//
// 为什么必须用 Postgres：SQLite 测试库走 AutoMigrate 建表、**不执行 migrations/**，
// 所以库层 CHECK 在 SQLite 契约测试里物理上不存在（迁移 000024 的注释早已写明这一点）。
// 而 CI 的 migration-check job **只统计 up/down 文件个数、并不执行迁移**——
// 在本次之前，论坛的 CHECK 约束实际上没有任何测试真正验证过。
// 本文件与 forum_designation_contract_test.go（行为层）合起来才覆盖「行为 + 库」两面。
//
// 覆盖范围与**已知不覆盖**：
//   - 覆盖：类别值域收窄、经验蕴含精选、列存在与默认值、任务配置行删除、策展线索写入。
//   - 不覆盖：**存量 experience 行的降级 UPDATE 本身**。原因是迁移在测试库建成时
//     已从零跑完（migrate 只支持 up/down/force，无法「迁到 000026 停住」去造迁移前数据）。
//     该 UPDATE 的正确性由「迁移在真实库上成功执行」间接保证——顺序若写反
//     （先加 CHECK 再 UPDATE），生产库含存量 experience 行时迁移会直接失败。
package api

import (
	"testing"

	"github.com/gin-gonic/gin"

	"forklift-training/internal/model"
	"forklift-training/internal/testutil"
)

func TestForumExperienceDesignationOnPostgres(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db := testutil.NewPostgresDB(t)
	if db == nil {
		t.Skip("DATABASE_URL 未设置")
	}

	author := testutil.SeedStudent(t, db, "desig_pg_author", "x")

	// 1. 列存在且默认 false
	var colExists bool
	if err := db.Raw(`SELECT EXISTS (SELECT 1 FROM information_schema.columns
		WHERE table_name = 'forum_topics' AND column_name = 'is_experience')`).Scan(&colExists).Error; err != nil {
		t.Fatalf("查询列存在性失败: %v", err)
	}
	if !colExists {
		t.Fatalf("forum_topics.is_experience 列不存在——迁移 000027 未生效")
	}

	// 2. 常规行（discussion + 非经验）可写：确认收窄后的值域不影响正常路径
	normal := model.ForumTopic{
		Category: "discussion", UserID: author.ID, Title: "普通讨论帖", Content: "x",
		Images: model.JSONB("[]"), CreatedAt: testutil.Now(), UpdatedAt: testutil.Now(),
	}
	if err := db.Create(&normal).Error; err != nil {
		t.Fatalf("普通讨论帖应可写入: %v", err)
	}
	if normal.IsExperience {
		t.Fatalf("is_experience 默认值应为 false，实际 true")
	}

	// 3. ★ 类别值域已收窄：experience 不再是合法意图，库层直接拒绝。
	//    这一条 SQLite 契约测试**物理上测不到**（那边没有 CHECK）。
	legacy := model.ForumTopic{
		Category: "experience", UserID: author.ID, Title: "自称考经", Content: "x",
		Images: model.JSONB("[]"), CreatedAt: testutil.Now(), UpdatedAt: testutil.Now(),
	}
	if err := db.Create(&legacy).Error; err == nil {
		t.Fatalf("写入 category='experience' 应被 CHECK 拒绝（ADR-0040 收窄），实际成功")
	}

	// 4. ★ 经验蕴含精选由 CHECK 兜底：写「经验但非精选」必须被拒。
	//    这是行为层 service 之外的最后一道防线（并发与直接 SQL 都绕不过它）。
	if err := db.Exec(`INSERT INTO forum_topics (category, user_id, title, content, images, is_featured, is_experience, created_at, updated_at)
		VALUES ('discussion', ?, '经验但非精选', 'x', '[]', false, true, NOW(), NOW())`, author.ID).Error; err == nil {
		t.Fatalf("写入「经验但非精选」应被 CHECK 拒绝（经验蕴含精选），实际成功")
	}

	// 5. 反证：经验 + 精选的合法组合可写（确认上一条拒的是组合而非列本身）
	okExp := model.ForumTopic{
		Category: "discussion", UserID: author.ID, Title: "合法经验帖", Content: "x",
		Images: model.JSONB("[]"), IsFeatured: true, IsExperience: true,
		CreatedAt: testutil.Now(), UpdatedAt: testutil.Now(),
	}
	if err := db.Create(&okExp).Error; err != nil {
		t.Fatalf("经验+精选应可写入: %v", err)
	}

	// 6. 任务配置行已删除（growth_first_experience 退役，认定奖励与加精合并）
	var taskCnt int64
	if err := db.Model(&model.PointsTaskConfig{}).Where("code = ?", "growth_first_experience").Count(&taskCnt).Error; err != nil {
		t.Fatalf("查询任务配置失败: %v", err)
	}
	if taskCnt != 0 {
		t.Fatalf("growth_first_experience 配置行应已删除，实际 %d 行", taskCnt)
	}

	// 7. 策展线索已写入（迁移的 INSERT 跑过；无存量行时为空串，键仍存在）
	var clue model.SystemSetting
	if err := db.Where("key = ?", "forum_legacy_experience_ids").First(&clue).Error; err != nil {
		t.Fatalf("策展线索键应存在（迁移 INSERT 未执行？）: %v", err)
	}

	// 8. ★ 「经验帖不可被采纳」由 CHECK 兜底（迁移 000028）。
	//    行为层由 service 双向守卫（AcceptReply / DesignateExperience），
	//    但并发与直接 SQL 绕不过库层这条——它是该不变式的最后一道防线。
	acceptedReply := model.ForumReply{TopicID: normal.ID, UserID: author.ID, Content: "回答", Images: model.JSONB("[]"), CreatedAt: testutil.Now()}
	if err := db.Create(&acceptedReply).Error; err != nil {
		t.Fatalf("建回复失败: %v", err)
	}
	if err := db.Exec("UPDATE forum_topics SET accepted_reply_id = ?, solved_at = NOW() WHERE id = ?", acceptedReply.ID, normal.ID).Error; err != nil {
		t.Fatalf("置采纳状态失败: %v", err)
	}
	if err := db.Exec("UPDATE forum_topics SET is_experience = true, is_featured = true WHERE id = ?", normal.ID).Error; err == nil {
		t.Fatalf("「已采纳 + 认定为经验」应被 CHECK 拒绝（迁移 000028），实际成功")
	}
	// 反证：清掉采纳指针后，同一行可以正常被认定（拒绝的是组合而非列本身）
	if err := db.Exec("UPDATE forum_topics SET accepted_reply_id = NULL, solved_at = NULL WHERE id = ?", normal.ID).Error; err != nil {
		t.Fatalf("清采纳状态失败: %v", err)
	}
	if err := db.Exec("UPDATE forum_topics SET is_experience = true WHERE id = ?", normal.ID).Error; err != nil {
		t.Fatalf("无采纳指针时认定应可写入: %v", err)
	}

	t.Log("迁移 000027/000028 契约通过：值域收窄、经验蕴含精选、经验不可被采纳、任务退役、策展线索均落地")
}
