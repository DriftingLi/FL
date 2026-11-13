// 本文件：storage multipart seam（#2494 分片断点续传上传）——
// 只测本地模拟实现的外部行为：init/UploadPart/Complete/Abort 四段语义与两环境一致的返回约定。
package storage

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestLocalStorageMultipartCompleteAssemblesPartsInOrder(t *testing.T) {
	s := NewLocalStorage(t.TempDir())
	ctx := context.Background()

	uploadID, err := s.InitMultipart(ctx, "tutor-files/1/clip_1.mp4", "video/mp4")
	if err != nil {
		t.Fatalf("InitMultipart 失败: %v", err)
	}
	// 乱序上传 3 片：3、1、2
	if _, err := s.UploadPart(ctx, "tutor-files/1/clip_1.mp4", uploadID, 3, []byte("C")); err != nil {
		t.Fatalf("UploadPart 3 失败: %v", err)
	}
	if _, err := s.UploadPart(ctx, "tutor-files/1/clip_1.mp4", uploadID, 1, []byte("A")); err != nil {
		t.Fatalf("UploadPart 1 失败: %v", err)
	}
	etag, err := s.UploadPart(ctx, "tutor-files/1/clip_1.mp4", uploadID, 2, []byte("B"))
	if err != nil {
		t.Fatalf("UploadPart 2 失败: %v", err)
	}
	if etag == "" {
		t.Fatal("UploadPart 应返回非空 etag（R2 侧 CompleteMultipart 需要）")
	}

	url, err := s.CompleteMultipart(ctx, "tutor-files/1/clip_1.mp4", uploadID, []Part{
		{Number: 1, ETag: "e1"}, {Number: 2, ETag: etag}, {Number: 3, ETag: "e3"},
	})
	if err != nil {
		t.Fatalf("CompleteMultipart 失败: %v", err)
	}
	if want := "/static/uploads/tutor-files/1/clip_1.mp4"; url != want {
		t.Fatalf("Complete 返回 URL = %q，want %q（与 Save 返回格式一致）", url, want)
	}

	rc, err := s.Get(ctx, url)
	if err != nil {
		t.Fatalf("Get 已合并文件失败: %v", err)
	}
	defer rc.Close()
	buf := make([]byte, 8)
	n, _ := rc.Read(buf)
	if got := string(buf[:n]); got != "ABC" {
		t.Fatalf("合并内容 = %q，want %q（按分片序号合并，不按上传顺序）", got, "ABC")
	}
}

func TestLocalStorageMultipartUploadPartIdempotentOverwrite(t *testing.T) {
	s := NewLocalStorage(t.TempDir())
	ctx := context.Background()
	key := "tutor-files/1/doc_2.pdf"

	uploadID, err := s.InitMultipart(ctx, key, "application/octet-stream")
	if err != nil {
		t.Fatalf("InitMultipart 失败: %v", err)
	}
	if _, err := s.UploadPart(ctx, key, uploadID, 1, []byte("first")); err != nil {
		t.Fatalf("UploadPart 首次失败: %v", err)
	}
	if _, err := s.UploadPart(ctx, key, uploadID, 1, []byte("second")); err != nil {
		t.Fatalf("UploadPart 重复覆盖失败: %v", err)
	}

	if _, err := s.CompleteMultipart(ctx, key, uploadID, []Part{{Number: 1, ETag: "e"}}); err != nil {
		t.Fatalf("CompleteMultipart 失败: %v", err)
	}
	rc, err := s.Get(ctx, "/static/uploads/"+key)
	if err != nil {
		t.Fatalf("Get 失败: %v", err)
	}
	defer rc.Close()
	buf := make([]byte, 16)
	n, _ := rc.Read(buf)
	if got := string(buf[:n]); got != "second" {
		t.Fatalf("重复分片应覆盖为最新内容，got %q", got)
	}
}

func TestLocalStorageMultipartAbortCleansStaging(t *testing.T) {
	s := NewLocalStorage(t.TempDir())
	ctx := context.Background()
	key := "tutor-files/1/x_3.png"

	uploadID, err := s.InitMultipart(ctx, key, "application/octet-stream")
	if err != nil {
		t.Fatalf("InitMultipart 失败: %v", err)
	}
	if _, err := s.UploadPart(ctx, key, uploadID, 1, []byte("data")); err != nil {
		t.Fatalf("UploadPart 失败: %v", err)
	}
	if err := s.AbortMultipart(ctx, key, uploadID); err != nil {
		t.Fatalf("AbortMultipart 失败: %v", err)
	}
	// abort 后 staging 不残留，且目标 key 不应出现
	if _, err := s.Get(ctx, "/static/uploads/"+key); err == nil {
		t.Fatal("abort 后目标文件不应存在")
	}
	if _, err := s.CompleteMultipart(ctx, key, uploadID, []Part{{Number: 1, ETag: "e"}}); err == nil {
		t.Fatal("abort 后 complete 应失败")
	}
	if err := s.AbortMultipart(ctx, key, uploadID); err != nil {
		t.Fatalf("重复 abort 应幂等: %v", err)
	}
}

func TestLocalStorageMultipartStagingNotVisibleUnderUploads(t *testing.T) {
	base := t.TempDir()
	s := NewLocalStorage(base)
	ctx := context.Background()
	key := "tutor-files/1/vis_4.mp4"

	uploadID, err := s.InitMultipart(ctx, key, "application/octet-stream")
	if err != nil {
		t.Fatalf("InitMultipart 失败: %v", err)
	}
	if _, err := s.UploadPart(ctx, key, uploadID, 1, []byte("partial")); err != nil {
		t.Fatalf("UploadPart 失败: %v", err)
	}
	// 未 complete 前，List 面（悬空回收/列表依赖）不应看到暂存残片
	urls, err := s.List(ctx, "tutor-files")
	if err != nil {
		t.Fatalf("List 失败: %v", err)
	}
	if len(urls) != 0 {
		t.Fatalf("未 complete 的分片不应出现在 List 中: %v", urls)
	}
	if _, err := os.Stat(filepath.Join(base, "tutor-files", "1", "vis_4.mp4")); !os.IsNotExist(err) {
		t.Fatalf("目标 key 在 complete 前不应存在, stat err = %v", err)
	}
}
