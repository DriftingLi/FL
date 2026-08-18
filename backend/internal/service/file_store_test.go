package service

import (
	"context"
	"io"
	"strings"
	"testing"

	"go.uber.org/zap"
)

// fileStoreMemStorage 内存 storage adapter：记录 Save/Delete/List 调用。
type fileStoreMemStorage struct {
	savedKeys []string
	savedURLs []string
	deleted   []string
	files     []string
}

func (m *fileStoreMemStorage) Save(_ context.Context, key string, _ []byte, _ string) (string, error) {
	m.savedKeys = append(m.savedKeys, key)
	url := "/static/uploads/" + key
	m.savedURLs = append(m.savedURLs, url)
	return url, nil
}

func (m *fileStoreMemStorage) Delete(_ context.Context, url string) error {
	m.deleted = append(m.deleted, url)
	return nil
}

func (m *fileStoreMemStorage) Exists(context.Context, string) (bool, error) { return true, nil }
func (m *fileStoreMemStorage) Get(context.Context, string) (io.ReadCloser, error) {
	return io.NopCloser(strings.NewReader("")), nil
}
func (m *fileStoreMemStorage) List(_ context.Context, _ string) ([]string, error) {
	return append([]string(nil), m.files...), nil
}

func TestFileStoreInterface(t *testing.T) {
	st := &fileStoreMemStorage{}
	store := NewFileStore("", st, zap.NewNop())

	url, err := store.Save([]byte("png"), "a.png", "images/questions")
	if err != nil {
		t.Fatalf("Save 失败: %v", err)
	}
	if !strings.Contains(url, "images/questions/") {
		t.Fatalf("URL 未包含 key: %q", url)
	}
	if len(st.savedKeys) != 1 {
		t.Fatalf("Save 调用次数 = %d", len(st.savedKeys))
	}

	if err := store.Delete(url); err != nil {
		t.Fatalf("Delete 失败: %v", err)
	}
	store.DeleteFiles([]string{url, ""})
	if len(st.deleted) != 2 {
		t.Fatalf("Delete 调用次数 = %d, 期望 2（DeleteFiles 忽略空 URL）", len(st.deleted))
	}

	st.files = []string{url}
	got := store.List("images/questions")
	if len(got) != 1 || got[0] != url {
		t.Fatalf("List 结果 = %v, 期望 [%s]", got, url)
	}
}

func TestFileStoreValidateImage(t *testing.T) {
	store := NewFileStore("", &fileStoreMemStorage{}, zap.NewNop())
	if ok, _ := store.ValidateImage("a.png", 1024); !ok {
		t.Error("png 应通过校验")
	}
	if ok, _ := store.ValidateImage("a.exe", 1024); ok {
		t.Error("exe 不应通过图片校验")
	}
	if ok, _ := store.ValidateImage("a.png", 21*1024*1024); ok {
		t.Error("超过图片大小上限应拒绝")
	}
}
