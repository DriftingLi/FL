// 静态投递分档的端点契约（ADR-0066 决策 2/3）：走真实路由 + 临时上传目录。
//
// 判据：已知安全类型（pdf / 图片 / 视频）保持内联 —— PDF 内联预览是一等需求；
// 可执行类型与未识别扩展名一律 Content-Disposition: attachment + application/octet-stream。
package api

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"

	"forklift-training/internal/config"
	"forklift-training/internal/testutil"
)

// newStaticDeliveryEnv 建一个上传目录可控的路由环境（静态面不需要任何登录态）。
func newStaticDeliveryEnv(t *testing.T, files map[string]string) *gin.Engine {
	t.Helper()
	gin.SetMode(gin.TestMode)
	dir := t.TempDir()
	for name, content := range files {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0o644); err != nil {
			t.Fatalf("写测试文件 %s 失败: %v", name, err)
		}
	}
	cfg := &config.Config{JWTSecretKey: "static-delivery-secret", UploadFolder: dir}
	return NewRouter(newContractDeps(t, testutil.NewMemoryDB(t), cfg))
}

func TestStaticDeliveryClasses(t *testing.T) {
	r := newStaticDeliveryEnv(t, map[string]string{
		"ok.pdf":      "%PDF-1.4",
		"ok.png":      "not-a-real-png",
		"ok.mp4":      "x",
		"evil.svg":    `<svg xmlns="http://www.w3.org/2000/svg"><script>alert(1)</script></svg>`,
		"evil.html":   "<script>alert(1)</script>",
		"mystery.bin": "MZ",
		"noext":       "x",
	})
	cases := []struct {
		name           string
		path           string
		wantCT         string
		wantAttachment bool
	}{
		{"PDF 保持内联（章节预览依赖它）", "/static/uploads/ok.pdf", "application/pdf", false},
		{"PNG 保持内联", "/static/uploads/ok.png", "image/png", false},
		{"MP4 保持内联", "/static/uploads/ok.mp4", "video/mp4", false},
		{"SVG 强制下载 + octet-stream", "/static/uploads/evil.svg", "application/octet-stream", true},
		{"HTML 强制下载 + octet-stream", "/static/uploads/evil.html", "application/octet-stream", true},
		{"未识别扩展名强制下载", "/static/uploads/mystery.bin", "application/octet-stream", true},
		{"无扩展名强制下载", "/static/uploads/noext", "application/octet-stream", true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			w := httptest.NewRecorder()
			r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, c.path, nil))
			if w.Code != http.StatusOK {
				t.Fatalf("GET %s = %d，期望 200（body=%s）", c.path, w.Code, w.Body.String())
			}
			if got := w.Header().Get("Content-Type"); !strings.Contains(got, c.wantCT) {
				t.Errorf("Content-Type = %q，期望包含 %q", got, c.wantCT)
			}
			gotAtt := strings.Contains(strings.ToLower(w.Header().Get("Content-Disposition")), "attachment")
			if gotAtt != c.wantAttachment {
				t.Errorf("Content-Disposition = %q，期望 attachment = %v", w.Header().Get("Content-Disposition"), c.wantAttachment)
			}
		})
	}
}

// HEAD 也被前端用来探测文件存在性（DocumentViewer/ImageViewer），分档头必须同样生效。
func TestStaticDeliveryHeadKeepsClassification(t *testing.T) {
	r := newStaticDeliveryEnv(t, map[string]string{"evil.svg": "<svg/>", "ok.pdf": "%PDF-1.4"})
	for _, c := range []struct {
		path           string
		wantAttachment bool
	}{
		{"/static/uploads/evil.svg", true},
		{"/static/uploads/ok.pdf", false},
	} {
		w := httptest.NewRecorder()
		r.ServeHTTP(w, httptest.NewRequest(http.MethodHead, c.path, nil))
		if w.Code != http.StatusOK {
			t.Fatalf("HEAD %s = %d，期望 200", c.path, w.Code)
		}
		gotAtt := strings.Contains(strings.ToLower(w.Header().Get("Content-Disposition")), "attachment")
		if gotAtt != c.wantAttachment {
			t.Errorf("HEAD %s: attachment = %v，期望 %v", c.path, gotAtt, c.wantAttachment)
		}
	}
}
