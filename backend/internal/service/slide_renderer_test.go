package service

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"go.uber.org/zap"

	"forklift-training/internal/storage"
)

// slideRenderStorage 内存 storage adapter：本文件的用例只断言「往哪个 key 存了」。
// （fileStoreMemStorage 随 file_store_test.go 搬进了 internal/filestore，跨包不可见，故自备一份。）
type slideRenderStorage struct{ savedKeys []string }

func (m *slideRenderStorage) Save(_ context.Context, key string, _ []byte, _ string) (string, error) {
	m.savedKeys = append(m.savedKeys, key)
	return "/static/uploads/" + key, nil
}

func (m *slideRenderStorage) Delete(context.Context, string) error { return nil }

func (m *slideRenderStorage) Exists(context.Context, string) (bool, error) { return true, nil }

func (m *slideRenderStorage) Get(context.Context, string) (io.ReadCloser, error) {
	return io.NopCloser(strings.NewReader("")), nil
}

func (m *slideRenderStorage) List(context.Context, string) ([]string, error) { return nil, nil }

func (m *slideRenderStorage) ListWithInfo(context.Context, string) ([]storage.FileInfo, error) {
	return nil, nil
}

func TestSlideRendererRenderEmpty(t *testing.T) {
	st := &slideRenderStorage{}
	renderer := NewSlideRenderer("", st, zap.NewNop())
	if got := renderer.Render(nil, 1); got != nil {
		t.Fatalf("空 PPT 应返回 nil，得到 %v", got)
	}
	if len(st.savedKeys) != 0 {
		t.Fatalf("空 PPT 不应上传，得到 %v", st.savedKeys)
	}
}

func TestSlideRendererSidecarAdapter(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/convert" {
			t.Errorf("sidecar path = %s", r.URL.Path)
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"success": true,
			"images": []map[string]any{
				{"name": "slide_001.webp", "data": "aGVsbG8="}, // "hello"
			},
		})
	}))
	defer server.Close()

	st := &slideRenderStorage{}
	renderer := NewSlideRenderer(server.URL, st, zap.NewNop())
	urls := renderer.Render([]byte("ppt-bytes"), 12)

	if len(urls) != 1 || len(st.savedKeys) != 1 {
		t.Fatalf("urls=%v savedKeys=%v", urls, st.savedKeys)
	}
	if want := "/static/uploads/slides/12/slide_001.webp"; urls[0] != want {
		t.Fatalf("slide URL = %q, 期望 %q", urls[0], want)
	}
}

func TestSlideRendererFallbackPlaceholder(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{"success": false, "error": "boom"})
	}))
	defer server.Close()

	st := &slideRenderStorage{}
	renderer := NewSlideRenderer(server.URL, st, zap.NewNop())
	urls := renderer.Render([]byte("bad-ppt"), 3)

	if len(urls) != 1 || len(st.savedKeys) != 1 {
		t.Fatalf("urls=%v savedKeys=%v", urls, st.savedKeys)
	}
	if want := "/static/uploads/slides/3/slide_001.png"; urls[0] != want {
		t.Fatalf("占位图 URL = %q, 期望 %q", urls[0], want)
	}
}
