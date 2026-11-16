// 迁移 000060（存量题归属回填）的 PG 契约测试（#2598）。
//
// 为什么这条必须有 PG 版本：迁移是 SQL，SQLite 测试库走 AutoMigrate、**根本不跑 migrations/**
// （见 internal/migrate/unique_index.go 的说明），所以「迁移文件写错了」在内存库里是绿的。
// 本机无 DATABASE_URL 时 testutil.NewPostgresDB 会干净 skip —— 首跑在 CI。
package api

import (
	"testing"

	"gorm.io/gorm"

	"forklift-training/internal/model"
	"forklift-training/internal/testutil"
)

// applyMigration000060 执行回填迁移的 SQL 正文（与 migrations/000060_*.up.sql 同句）。
//
// 不读文件而是内联同一句：迁移文件是 .sql、Go 测试读它要处理路径上溯与 CRLF，
// 而这里要验的是**那条 SQL 的语义**。若两处漂移，migration-check 与人工审查会拦
// （迁移正文已在文件里，此处是它的可执行副本）。
func applyMigration000060(t *testing.T, db *gorm.DB) {
	t.Helper()
	const stmt = `
UPDATE question
SET created_by      = (SELECT tutor_id FROM tutor ORDER BY tutor_id ASC LIMIT 1),
    created_by_type = 'tutor'
WHERE created_by IS NULL
  AND EXISTS (SELECT 1 FROM tutor)`
	if err := db.Exec(stmt).Error; err != nil {
		t.Fatalf("执行回填迁移失败: %v", err)
	}
}

func TestQuestionCreatedByBackfillOnPostgres(t *testing.T) {
	db := testutil.NewPostgresDB(t) // 本机无 DATABASE_URL 时干净 skip

	// 造两道「作者未知」的存量题 + 两位讲师（回填应只认最早那位）
	orphanA := model.Question{Type: "single_choice", Content: "存量题一", Answer: "A", Status: "published",
		CreatedBy: nil, CreatedByType: "admin", CreatedAt: testutil.Now(), UpdatedAt: testutil.Now()}
	orphanB := model.Question{Type: "single_choice", Content: "存量题二", Answer: "A", Status: "published",
		CreatedBy: nil, CreatedByType: "admin", CreatedAt: testutil.Now(), UpdatedAt: testutil.Now()}
	for _, q := range []*model.Question{&orphanA, &orphanB} {
		if err := db.Create(q).Error; err != nil {
			t.Fatalf("建存量题失败: %v", err)
		}
	}
	// 一道已有作者的题：回填不得碰它（幂等的另一半）
	existingOwner := 999999
	owned := model.Question{Type: "single_choice", Content: "已有作者题", Answer: "A", Status: "published",
		CreatedBy: &existingOwner, CreatedByType: "tutor", CreatedAt: testutil.Now(), UpdatedAt: testutil.Now()}
	if err := db.Create(&owned).Error; err != nil {
		t.Fatalf("建有主题失败: %v", err)
	}

	earliest := testutil.SeedTutor(t, db, "backfill_earliest", "x")
	_ = testutil.SeedTutor(t, db, "backfill_later", "x")

	applyMigration000060(t, db)

	// 两道空创建者题都归最早那位讲师，且 created_by_type 同步为 tutor
	for _, want := range []int{orphanA.ID, orphanB.ID} {
		var got model.Question
		if err := db.First(&got, want).Error; err != nil {
			t.Fatalf("回读题 %d 失败: %v", want, err)
		}
		if got.CreatedBy == nil || *got.CreatedBy != earliest.TutorID {
			t.Fatalf("题 %d 应归最早讲师 %d, got %v", want, earliest.TutorID, got.CreatedBy)
		}
		if got.CreatedByType != "tutor" {
			t.Fatalf("题 %d 的 created_by_type 应同步为 tutor, got %q", want, got.CreatedByType)
		}
	}

	// 已有作者的题原样不动
	var kept model.Question
	if err := db.First(&kept, owned.ID).Error; err != nil {
		t.Fatalf("回读有主题失败: %v", err)
	}
	if kept.CreatedBy == nil || *kept.CreatedBy != existingOwner {
		t.Fatalf("已有作者的题不得被回填覆盖, got %v", kept.CreatedBy)
	}

	// 幂等：重跑一遍，结果不变（第二次命中 0 行）
	applyMigration000060(t, db)
	var again model.Question
	if err := db.First(&again, orphanA.ID).Error; err != nil {
		t.Fatalf("二次回填后回读失败: %v", err)
	}
	if again.CreatedBy == nil || *again.CreatedBy != earliest.TutorID {
		t.Fatalf("二次回填应幂等, got %v", again.CreatedBy)
	}
}

// TestQuestionCreatedByBackfillNoTutor 无讲师时**不回填**：保持 NULL（仅管理员可写）。
//
// 这条是刻意的行为而不是遗漏：硬塞一个不存在的 id 会让谓词永远判不中，
// 而 NULL 至少语义诚实（作者未知）。管理员仍可经管理端维护。
func TestQuestionCreatedByBackfillNoTutor(t *testing.T) {
	db := testutil.NewPostgresDB(t)

	orphan := model.Question{Type: "single_choice", Content: "无讲师库里的存量题", Answer: "A", Status: "published",
		CreatedBy: nil, CreatedByType: "admin", CreatedAt: testutil.Now(), UpdatedAt: testutil.Now()}
	if err := db.Create(&orphan).Error; err != nil {
		t.Fatalf("建存量题失败: %v", err)
	}
	var tutorCount int64
	if err := db.Model(&model.Tutor{}).Count(&tutorCount).Error; err != nil {
		t.Fatalf("查讲师数失败: %v", err)
	}
	if tutorCount != 0 {
		t.Skipf("该库已有 %d 位讲师，跳过「无讲师」分支（PG 测试库可能是共享的）", tutorCount)
	}

	applyMigration000060(t, db)

	var got model.Question
	if err := db.First(&got, orphan.ID).Error; err != nil {
		t.Fatalf("回读失败: %v", err)
	}
	if got.CreatedBy != nil {
		t.Fatalf("无讲师时不得回填, got %v", *got.CreatedBy)
	}
}
