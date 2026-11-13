// 本文件：删章回库的数据库级契约（#2497 评审修订）——tutor_file.chapter_id 外键
// ON DELETE SET NULL（迁移 000053，沿 chapter_file 先例 000001:262）：章节删除后
// 挂接行自动回库（chapter_id 清空），不产生反向悬空引用，回库后可删（B5）。
// 真实外键只在 Postgres 迁移里成立（sqlite AutoMigrate 不建 FK）⇒ 契约载体 = PG，
// 无 DATABASE_URL 干净 skip、首跑在 CI（与 quota_resume_postgres_contract_test 同口径）。
package tutorfile

import (
	"context"
	"testing"

	"forklift-training/internal/model"
	"forklift-training/internal/storage"
	"forklift-training/internal/testutil"
)

func TestChapterDeleteReleasesAttachment(t *testing.T) {
	db := testutil.NewPostgresDB(t)
	svc := NewService(db, storage.NewLocalStorage(t.TempDir()), nil)
	ctx := context.Background()

	ch := model.Chapter{CourseID: 1, Title: "待删章", ContentType: "text"}
	if err := db.Create(&ch).Error; err != nil {
		t.Fatalf("造章节失败: %v", err)
	}
	f := &model.TutorFile{
		TutorID:     1,
		FileURL:     "/static/uploads/tutor-files/1/录课_1.mp4",
		FileName:    "录课.mp4",
		ContentType: "video",
		FileSize:    1024,
		ChapterID:   &ch.ChapterID,
	}
	if err := db.Create(f).Error; err != nil {
		t.Fatalf("造文件行失败: %v", err)
	}

	if err := db.Delete(&model.Chapter{}, "chapter_id = ?", ch.ChapterID).Error; err != nil {
		t.Fatalf("删章失败: %v", err)
	}

	var got model.TutorFile
	if err := db.First(&got, "file_id = ?", f.FileID).Error; err != nil {
		t.Fatalf("读文件行失败: %v", err)
	}
	if got.ChapterID != nil {
		t.Fatalf("删章后 chapter_id 应被 FK SET NULL 回库, got %v", *got.ChapterID)
	}
	// 回库后可删（不再被 ErrFileInUse 拦）
	if err := svc.DeleteFile(ctx, f.TutorID, f.FileID); err != nil {
		t.Fatalf("回库后删除失败: %v", err)
	}
}
