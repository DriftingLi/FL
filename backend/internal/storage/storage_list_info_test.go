package storage

import (
	"context"
	"os"
	"path/filepath"
	"sort"
	"testing"
	"time"
)

// TestLocalStorageListWithInfo ListWithInfo 返回存储侧原生 LastModified（文件 ModTime），
// 时间戳真实性来自文件系统元数据而非文件名（ADR-0027 C2 存储时间戳归位）。
func TestLocalStorageListWithInfo(t *testing.T) {
	baseDir := t.TempDir()
	s := NewLocalStorage(baseDir)
	ctx := context.Background()

	writeFile := func(rel string, modTime time.Time) {
		full := filepath.Join(baseDir, rel)
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(full, []byte("x"), 0o644); err != nil {
			t.Fatal(err)
		}
		if err := os.Chtimes(full, modTime, modTime); err != nil {
			t.Fatal(err)
		}
	}
	old := time.Date(2020, 1, 2, 3, 4, 5, 0, time.UTC)
	recent := time.Date(2025, 6, 7, 8, 9, 10, 0, time.UTC)
	writeFile("images/forum/a.webp", old)
	writeFile("images/forum/b.webp", recent)
	writeFile("images/chapters/c.pdf", recent)

	infos, err := s.ListWithInfo(ctx, "images/forum")
	if err != nil {
		t.Fatalf("ListWithInfo 失败: %v", err)
	}
	if len(infos) != 2 {
		t.Fatalf("前缀 images/forum 应返回 2 个文件，得到 %d: %+v", len(infos), infos)
	}
	sort.Slice(infos, func(i, j int) bool { return infos[i].URL < infos[j].URL })

	got := map[string]time.Time{}
	for _, f := range infos {
		got[f.URL] = f.LastModified
		if f.LastModified.IsZero() {
			t.Fatalf("LastModified 不应为零值: %+v", f)
		}
	}
	if !got["/static/uploads/images/forum/a.webp"].Equal(old) {
		t.Errorf("a.webp LastModified 应为 %v，得到 %v", old, got["/static/uploads/images/forum/a.webp"])
	}
	if !got["/static/uploads/images/forum/b.webp"].Equal(recent) {
		t.Errorf("b.webp LastModified 应为 %v，得到 %v", recent, got["/static/uploads/images/forum/b.webp"])
	}
}

// TestLocalStorageListWithInfoMissingPrefix 前缀目录不存在返回空列表（与 List 语义一致）。
func TestLocalStorageListWithInfoMissingPrefix(t *testing.T) {
	s := NewLocalStorage(t.TempDir())
	infos, err := s.ListWithInfo(context.Background(), "images/forum")
	if err != nil {
		t.Fatalf("ListWithInfo 失败: %v", err)
	}
	if infos == nil || len(infos) != 0 {
		t.Fatalf("不存在的目录应返回空列表，得到 %+v", infos)
	}
}
