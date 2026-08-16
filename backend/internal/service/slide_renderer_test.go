package service

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"go.uber.org/zap"
)

func TestSlideRendererRenderEmpty(t *testing.T) {
	st := &fileStoreMemStorage{}
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

	st := &fileStoreMemStorage{}
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

	st := &fileStoreMemStorage{}
	renderer := NewSlideRenderer(server.URL, st, zap.NewNop())
	urls := renderer.Render([]byte("bad-ppt"), 3)

	if len(urls) != 1 || len(st.savedKeys) != 1 {
		t.Fatalf("urls=%v savedKeys=%v", urls, st.savedKeys)
	}
	if want := "/static/uploads/slides/3/slide_001.png"; urls[0] != want {
		t.Fatalf("占位图 URL = %q, 期望 %q", urls[0], want)
	}
}
