// 本文件：#2495 的 PG 契约面（spec「契约测试」载体要求：同构既有 PG 契约测试纪律）。
// 只在真实 PG 上跑（无 DATABASE_URL 干净 skip，首跑在 CI）：要证明的是**聚合与排序的 SQL 形态**
// 在 PG 上成立——配额 SUM 双臂（COALESCE(SUM)）与续传清单的 ORDER BY part_number。
// 行为面断言（幂等/状态机/差集）在 service_test.go（MemoryDB），两处不重复。
package tutorfile

import (
	"context"
	"errors"
	"strconv"
	"strings"
	"testing"
	"time"

	"forklift-training/internal/model"
	"forklift-training/internal/storage"
	"forklift-training/internal/testutil"
)

// errorContainsUsage 断言：错误是目标哨兵、且文案含当前用量（B3「拒绝信息含当前用量」）。
func errorContainsUsage(err error, sentinel error, usage string) bool {
	return errors.Is(err, sentinel) && strings.Contains(err.Error(), usage)
}

// TestQuotaArmsSQLShapeOnPostgres 配额双臂的聚合查询在 PG 上独立拦截（SUM 空集为 NULL、COALESCE 兜 0）。
func TestQuotaArmsSQLShapeOnPostgres(t *testing.T) {
	db := testutil.NewPostgresDB(t)
	st := storage.NewLocalStorage(t.TempDir())
	svc := NewService(db, st, nil)
	ctx := context.Background()

	// 数量臂：300 行小文件（Size=1），容量远未到 ⇒ 只有数量臂拦
	rows := make([]model.TutorFile, 0, MaxFilesPerTutor)
	for i := 0; i < MaxFilesPerTutor; i++ {
		rows = append(rows, model.TutorFile{
			TutorID:     7,
			FileURL:     "/static/uploads/tutor-files/seed/seed_" + strconv.Itoa(i) + ".mp4",
			FileName:    "seed_" + strconv.Itoa(i) + ".mp4",
			ContentType: "video",
			FileSize:    1,
			CreatedAt:   time.Now(),
		})
	}
	if err := db.Create(&rows).Error; err != nil {
		t.Fatalf("seed 失败: %v", err)
	}
	if _, err := svc.InitUpload(ctx, 7, "n.mp4", 8, testChunkSz); err == nil ||
		!errorContainsUsage(err, ErrQuotaFileCount, "300/300") {
		t.Fatalf("数量臂应拦截且含用量, got %v", err)
	}
	// 他人不受影响（配额按导师分区）
	if _, err := svc.InitUpload(ctx, 8, "n.mp4", 8, testChunkSz); err != nil {
		t.Fatalf("他人配额独立，不应拦截: %v", err)
	}

	// 容量臂：清空换 2×50GiB 行（数量 2/300 远未到）⇒ 只有容量臂拦
	if err := db.Where("tutor_id = ?", 7).Delete(&model.TutorFile{}).Error; err != nil {
		t.Fatalf("清行失败: %v", err)
	}
	big := []model.TutorFile{
		{TutorID: 7, FileURL: "/static/uploads/tutor-files/seed/big_a.mp4", FileName: "big_a.mp4", ContentType: "video", FileSize: 50 << 30, CreatedAt: time.Now()},
		{TutorID: 7, FileURL: "/static/uploads/tutor-files/seed/big_b.mp4", FileName: "big_b.mp4", ContentType: "video", FileSize: 50 << 30, CreatedAt: time.Now()},
	}
	if err := db.Create(&big).Error; err != nil {
		t.Fatalf("seed 失败: %v", err)
	}
	if _, err := svc.InitUpload(ctx, 7, "big.mp4", 8, testChunkSz); err == nil ||
		!errorContainsUsage(err, ErrQuotaTotalSize, "100.0/100") {
		t.Fatalf("容量臂应拦截且含用量, got %v", err)
	}
}

// TestResumeManifestSQLShapeOnPostgres 续传清单：分片序号升序（ORDER BY part_number）在 PG 上成立。
func TestResumeManifestSQLShapeOnPostgres(t *testing.T) {
	db := testutil.NewPostgresDB(t)
	st := storage.NewLocalStorage(t.TempDir())
	svc := NewService(db, st, nil)
	ctx := context.Background()

	sid, err := svc.InitUpload(ctx, 7, "a.mp4", 2*testChunkSz+4, testChunkSz)
	if err != nil {
		t.Fatalf("init 失败: %v", err)
	}
	// 乱序传 3、1
	if err := svc.UploadChunk(ctx, 7, sid, 3, []byte("DDDD")); err != nil {
		t.Fatalf("chunk 3 失败: %v", err)
	}
	if err := svc.UploadChunk(ctx, 7, sid, 1, bytesOf('A', int(testChunkSz))); err != nil {
		t.Fatalf("chunk 1 失败: %v", err)
	}
	row, parts, err := svc.GetUploadSession(ctx, 7, sid)
	if err != nil {
		t.Fatalf("GetUploadSession 失败: %v", err)
	}
	if row.Status != statusActive {
		t.Fatalf("状态 = %s, want active", row.Status)
	}
	if len(parts) != 2 || parts[0] != 1 || parts[1] != 3 {
		t.Fatalf("已收分片 = %v, want [1 3]（升序）", parts)
	}
}
