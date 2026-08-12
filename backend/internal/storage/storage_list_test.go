package storage

import (
	"context"
	"os"
	"path/filepath"
	"sort"
	"testing"
)

// TestLocalStorageList 测试 LocalStorage.List：按前缀列出文件并返回 /static/uploads/<key> 形式的 URL。
func TestLocalStorageList(t *testing.T) {
	baseDir := t.TempDir()
	s := NewLocalStorage(baseDir)
	ctx := context.Background()

	writeFile := func(rel string) {
		full := filepath.Join(baseDir, rel)
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(full, []byte("x"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	writeFile("images/forum/a_1.webp")
	writeFile("images/forum/b_2.webp")
	writeFile("images/chapters/c_3.webp")
	writeFile("chapters/d.pdf")

	got, err := s.List(ctx, "images/forum")
	if err != nil {
		t.Fatalf("List 失败: %v", err)
	}
	sort.Strings(got)
	want := []string{
		"/static/uploads/images/forum/a_1.webp",
		"/static/uploads/images/forum/b_2.webp",
	}
	if len(got) != len(want) {
		t.Fatalf("前缀 images/forum 应返回 %d 个文件，得到 %d: %v", len(want), len(got), got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("第 %d 项: 期望 %q 得到 %q", i, want[i], got[i])
		}
	}
}

// TestLocalStorageListEmptyPrefix 测试 List 空前缀返回全部文件。
func TestLocalStorageListEmptyPrefix(t *testing.T) {
	baseDir := t.TempDir()
	s := NewLocalStorage(baseDir)
	ctx := context.Background()

	for _, rel := range []string{"images/forum/a.webp", "chapters/d.pdf"} {
		full := filepath.Join(baseDir, rel)
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(full, []byte("x"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	got, err := s.List(ctx, "")
	if err != nil {
		t.Fatalf("List 失败: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("空前缀应返回全部 2 个文件，得到 %d: %v", len(got), got)
	}
}

// TestLocalStorageListMissingPrefix 测试前缀目录不存在时返回空列表而非错误。
func TestLocalStorageListMissingPrefix(t *testing.T) {
	s := NewLocalStorage(t.TempDir())
	got, err := s.List(context.Background(), "images/forum")
	if err != nil {
		t.Fatalf("List 失败: %v", err)
	}
	if len(got) != 0 {
		t.Fatalf("不存在的目录应返回空列表，得到 %v", got)
	}
}
