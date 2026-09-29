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
	"go.uber.org/zap"

	"forklift-training/internal/config"
	"forklift-training/internal/storage"
	"forklift-training/internal/testutil"
)

// newUploadEnv 建一个带真实本地存储的路由环境（上传端点会写盘）。
func newUploadEnv(t *testing.T) *gin.Engine {
	t.Helper()
	setTestGinMode()
	cfg := &config.Config{JWTSecretKey: "upload-gate-secret", UploadFolder: t.TempDir()}
	deps := NewDeps(cfg, testutil.NewMemoryDB(t), storage.NewLocalStorage(t.TempDir()), zap.NewNop(), stubExportStore{})
	return NewRouter(deps)
}

// TestUploadEndpointRejectsSvg 上传侧的唯一闸门（ValidateImage，五个上传端点共用）必须拒掉 svg；
// 同时反证白名单没被误伤（png 仍可上传）。ADR-0066 决策 1。
func TestUploadEndpointRejectsSvg(t *testing.T) {
	t.Parallel()
	r := newUploadEnv(t)
	w := uploadAIImage(t, r, "evil.svg")
	if w.Code == http.StatusOK {
		t.Fatalf("上传 evil.svg 竟成功（body=%s）—— 存储型 XSS 的注入源仍在", w.Body.String())
	}
	if !strings.Contains(w.Body.String(), "不支持的图片格式") {
		t.Fatalf("拒绝文案未命中白名单提示：%s", w.Body.String())
	}
	if w2 := uploadAIImage(t, r, "ok.png"); w2.Code != http.StatusOK {
		t.Fatalf("上传 ok.png 失败（%d）：%s", w2.Code, w2.Body.String())
	}
}

// newStaticDeliveryEnv 建一个上传目录可控的路由环境（静态面不需要任何登录态）。
func newStaticDeliveryEnv(t *testing.T, files map[string]string) *gin.Engine {
	t.Helper()
	setTestGinMode()
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
	t.Parallel()
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
	t.Parallel()
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
