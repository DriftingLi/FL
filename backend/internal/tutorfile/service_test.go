// 本文件：导师文件库服务契约（#2494）——四段上传契约的外部行为：
// init/分片/complete/abort + 文件行落库可见可删。只经公开方法断言，不摸内部。
package tutorfile

import (
	"context"
	"path/filepath"
	"strings"
	"testing"

	"forklift-training/internal/storage"
	"forklift-training/internal/testutil"
)

const (
	mib         = int64(1 << 20)
	testChunkSz = 5 * mib // A2 契约最小片
)

func newSvc(t *testing.T) (*Service, *storage.LocalStorage) {
	t.Helper()
	db := testutil.NewMemoryDB(t)
	st := storage.NewLocalStorage(t.TempDir())
	return NewService(db, st), st
}

func initSession(t *testing.T, svc *Service, tutorID int, filename string, fileSize, chunkSize int64) string {
	t.Helper()
	sid, err := svc.InitUpload(context.Background(), tutorID, filename, fileSize, chunkSize)
	if err != nil {
		t.Fatalf("InitUpload 失败: %v", err)
	}
	return sid
}

func bytesOf(b byte, n int) []byte {
	out := make([]byte, n)
	for i := range out {
		out[i] = b
	}
	return out
}

func TestUploadCompleteCreatesFileRowAndListVisible(t *testing.T) {
	svc, _ := newSvc(t)
	ctx := context.Background()
	// 3 片：5MiB + 5MiB + 4B（末片无下限）
	total := 2*testChunkSz + 4
	sid := initSession(t, svc, 1, "录课.mp4", total, testChunkSz)

	// 乱序上传：3、1、2
	if err := svc.UploadChunk(ctx, 1, sid, 3, []byte("DDDD")); err != nil {
		t.Fatalf("UploadChunk 3 失败: %v", err)
	}
	if err := svc.UploadChunk(ctx, 1, sid, 1, bytesOf('A', int(testChunkSz))); err != nil {
		t.Fatalf("UploadChunk 1 失败: %v", err)
	}
	if err := svc.UploadChunk(ctx, 1, sid, 2, bytesOf('B', int(testChunkSz))); err != nil {
		t.Fatalf("UploadChunk 2 失败: %v", err)
	}

	file, err := svc.CompleteUpload(ctx, 1, sid)
	if err != nil {
		t.Fatalf("CompleteUpload 失败: %v", err)
	}
	if file.FileSize != total {
		t.Fatalf("文件大小 = %d, want %d", file.FileSize, total)
	}
	if file.FileName != "录课.mp4" {
		t.Fatalf("文件名 = %q, want 录课.mp4", file.FileName)
	}
	// 命名契约：<name>_<毫秒时间戳>.<ext>
	base := filepath.Base(strings.TrimPrefix(file.FileURL, "/static/uploads/"))
	if !strings.HasPrefix(base, "录课_") || !strings.HasSuffix(base, ".mp4") {
		t.Fatalf("存储命名不符 <name>_<毫秒时间戳>.<ext> 契约: %q", base)
	}

	files, err := svc.ListFiles(ctx, 1)
	if err != nil {
		t.Fatalf("ListFiles 失败: %v", err)
	}
	if len(files) != 1 || files[0].FileID != file.FileID {
		t.Fatalf("列表应含新文件行: %+v", files)
	}

	// 别的导师看不见
	other, err := svc.ListFiles(ctx, 2)
	if err != nil || len(other) != 0 {
		t.Fatalf("他人列表应为空: %v %v", other, err)
	}
}

func TestUploadChunkIdempotentOverwrite(t *testing.T) {
	svc, st := newSvc(t)
	ctx := context.Background()
	sid := initSession(t, svc, 1, "a.mp4", 8, testChunkSz)

	if err := svc.UploadChunk(ctx, 1, sid, 1, []byte("first123")); err != nil {
		t.Fatalf("首次上传失败: %v", err)
	}
	if err := svc.UploadChunk(ctx, 1, sid, 1, []byte("second22")); err != nil {
		t.Fatalf("重复上传失败: %v", err)
	}
	file, err := svc.CompleteUpload(ctx, 1, sid)
	if err != nil {
		t.Fatalf("CompleteUpload 失败: %v", err)
	}
	rc, err := st.Get(ctx, file.FileURL)
	if err != nil {
		t.Fatalf("读回文件失败: %v", err)
	}
	defer rc.Close()
	buf := make([]byte, 16)
	n, _ := rc.Read(buf)
	if got := string(buf[:n]); got != "second22" {
		t.Fatalf("重复分片应幂等覆盖为最新内容, got %q", got)
	}
}

func TestCompleteAfterAbortRejected(t *testing.T) {
	svc, _ := newSvc(t)
	ctx := context.Background()
	sid := initSession(t, svc, 1, "a.mp4", 8, testChunkSz)
	if err := svc.UploadChunk(ctx, 1, sid, 1, []byte("AAAAAAAA")); err != nil {
		t.Fatalf("UploadChunk 失败: %v", err)
	}
	if err := svc.AbortUpload(ctx, 1, sid); err != nil {
		t.Fatalf("AbortUpload 失败: %v", err)
	}
	if _, err := svc.CompleteUpload(ctx, 1, sid); err == nil {
		t.Fatal("abort 后 complete 应拒绝")
	}
	// abort 后补分片也应拒绝
	if err := svc.UploadChunk(ctx, 1, sid, 1, []byte("BBBBBBBB")); err == nil {
		t.Fatal("abort 后 UploadChunk 应拒绝")
	}
	// 重复 abort 幂等
	if err := svc.AbortUpload(ctx, 1, sid); err != nil {
		t.Fatalf("重复 abort 应幂等: %v", err)
	}
}

func TestInitRejectsOversizeBadTypeAndChunkBand(t *testing.T) {
	svc, _ := newSvc(t)
	ctx := context.Background()

	if _, err := svc.InitUpload(ctx, 1, "big.mp4", MaxFileSize+1, testChunkSz); err == nil {
		t.Fatal("超过 2GB 应拒绝")
	}
	if _, err := svc.InitUpload(ctx, 1, "evil.exe", mib, testChunkSz); err == nil {
		t.Fatal("类型越界（.exe 不在四档白名单）应拒绝")
	}
	if _, err := svc.InitUpload(ctx, 1, "script.svg", mib, testChunkSz); err == nil {
		t.Fatal("类型越界（.svg unsafe）应拒绝")
	}
	if _, err := svc.InitUpload(ctx, 1, "tiny.mp4", mib, 1*mib); err == nil {
		t.Fatal("分片档低于 5MiB 下限应拒绝（S3 最小片语义）")
	}
	if _, err := svc.InitUpload(ctx, 1, "huge.mp4", mib, 16*mib); err == nil {
		t.Fatal("分片档超出 8MiB 上限应拒绝（60s 读窗）")
	}
	if _, err := svc.InitUpload(ctx, 1, "ok.pdf", mib, testChunkSz); err != nil {
		t.Fatalf("pdf 在白名单（document 档）应放行: %v", err)
	}
}

func TestUploadChunkRejectsWrongSizeAndOutOfRange(t *testing.T) {
	svc, _ := newSvc(t)
	ctx := context.Background()
	sid := initSession(t, svc, 1, "a.mp4", 2*testChunkSz+4, testChunkSz)

	// 非末片不足 chunkSize → 拒绝
	if err := svc.UploadChunk(ctx, 1, sid, 1, bytesOf('x', 5)); err == nil {
		t.Fatal("非末片大小不足 chunkSize 应拒绝")
	}
	// 序号越界 → 拒绝
	if err := svc.UploadChunk(ctx, 1, sid, 4, bytesOf('x', 5)); err == nil {
		t.Fatal("超出 total_chunks 的序号应拒绝")
	}
	// 末片超出应有长度（应为 4 字节）→ 拒绝
	if err := svc.UploadChunk(ctx, 1, sid, 3, bytesOf('x', 5)); err == nil {
		t.Fatal("末片超出应有长度应拒绝")
	}
}

func TestChunkPlanBandAndMaxChunks(t *testing.T) {
	// 分片档位与片数上限的纯函数面（公开 API 上 2GB/5MiB 到不了 10,000 片，此处直测档位）。
	if _, err := chunkPlan(mib, 1*mib); err == nil {
		t.Fatal("chunkSize 低于 5MiB 下限应拒绝")
	}
	if _, err := chunkPlan(mib, 16*mib); err == nil {
		t.Fatal("chunkSize 超 8MiB 上限应拒绝")
	}
	if _, err := chunkPlan(0, testChunkSz); err == nil {
		t.Fatal("空文件应拒绝")
	}
	if _, err := chunkPlan(MaxFileSize+1, testChunkSz); err == nil {
		t.Fatal("超 2GB 应拒绝")
	}
	// 5MiB 档、虚构体量越过片数上限——上限本身是契约位（A2 ≤10,000），保持可达判定。
	total := totalChunksFor(50_001*mib, testChunkSz)
	if total <= MaxChunks {
		t.Fatalf("该组合应可触发片数上限，total=%d", total)
	}
	if _, err := chunkPlan(50_001*mib, testChunkSz); err == nil {
		t.Fatal("片数超 10,000 应拒绝")
	}
}

func TestCompleteRejectsMissingChunks(t *testing.T) {
	svc, _ := newSvc(t)
	ctx := context.Background()
	sid := initSession(t, svc, 1, "a.mp4", 2*testChunkSz+4, testChunkSz)
	if err := svc.UploadChunk(ctx, 1, sid, 1, bytesOf('A', int(testChunkSz))); err != nil {
		t.Fatalf("UploadChunk 失败: %v", err)
	}
	if _, err := svc.CompleteUpload(ctx, 1, sid); err == nil {
		t.Fatal("缺片时 complete 应拒绝")
	}
}

func TestDeleteFileRemovesRowAndStorage(t *testing.T) {
	svc, st := newSvc(t)
	ctx := context.Background()
	sid := initSession(t, svc, 1, "a.mp4", 8, testChunkSz)
	if err := svc.UploadChunk(ctx, 1, sid, 1, []byte("AAAAAAAA")); err != nil {
		t.Fatalf("UploadChunk 失败: %v", err)
	}
	file, err := svc.CompleteUpload(ctx, 1, sid)
	if err != nil {
		t.Fatalf("CompleteUpload 失败: %v", err)
	}
	if err := svc.DeleteFile(ctx, 1, file.FileID); err != nil {
		t.Fatalf("DeleteFile 失败: %v", err)
	}
	files, _ := svc.ListFiles(ctx, 1)
	if len(files) != 0 {
		t.Fatalf("删除后列表应为空: %+v", files)
	}
	if ok, _ := st.Exists(ctx, file.FileURL); ok {
		t.Fatal("删除后存储对象不应残留")
	}
	// 别的导师删不了
	sid2 := initSession(t, svc, 1, "b.mp4", 8, testChunkSz)
	_ = svc.UploadChunk(ctx, 1, sid2, 1, []byte("BBBBBBBB"))
	file2, _ := svc.CompleteUpload(ctx, 1, sid2)
	if err := svc.DeleteFile(ctx, 2, file2.FileID); err == nil {
		t.Fatal("他人删除应拒绝")
	}
}
