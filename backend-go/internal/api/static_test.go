package api

import (
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"forklift-training/internal/config"
)

// newTestRouter 创建仅含静态路由的测试路由器，避免依赖数据库。
func newTestRouter(t *testing.T, cfg *config.Config) *gin.Engine {
	t.Helper()
	gin.SetMode(gin.TestMode)
	r := gin.New()
	registerStaticRoutes(r, cfg)
	return r
}

// TestStaticUploadsFromLocal 测试从本地 UploadFolder 提供 /static/uploads/*。
func TestStaticUploadsFromLocal(t *testing.T) {
	// 准备临时上传目录与测试文件
	tmpDir := t.TempDir()
	uploadDir := filepath.Join(tmpDir, "uploads")
	if err := os.MkdirAll(filepath.Join(uploadDir, "chapters"), 0o755); err != nil {
		t.Fatal(err)
	}
	testFile := filepath.Join(uploadDir, "chapters", "test.txt")
	if err := os.WriteFile(testFile, []byte("hello upload"), 0o644); err != nil {
		t.Fatal(err)
	}

	cfg := &config.Config{UploadFolder: uploadDir}
	r := newTestRouter(t, cfg)

	// 请求 /static/uploads/chapters/test.txt
	w := performRequest(r, "GET", "/static/uploads/chapters/test.txt")
	if w.Code != 200 {
		t.Fatalf("期望 200，得到 %d", w.Code)
	}
	body, _ := io.ReadAll(w.Body)
	if string(body) != "hello upload" {
		t.Fatalf("期望 'hello upload'，得到 %s", string(body))
	}
}

// TestStaticUploadsFromVolume 测试从 VOLUME_MOUNT_PATH 提供 /static/uploads/*。
func TestStaticUploadsFromVolume(t *testing.T) {
	volDir := t.TempDir()
	uploadDir := filepath.Join(volDir, "uploads")
	if err := os.MkdirAll(filepath.Join(uploadDir, "slides"), 0o755); err != nil {
		t.Fatal(err)
	}
	testFile := filepath.Join(uploadDir, "slides", "page1.png")
	if err := os.WriteFile(testFile, []byte("fake-png-data"), 0o644); err != nil {
		t.Fatal(err)
	}

	cfg := &config.Config{VolumeMountPath: volDir, UploadFolder: "should-not-use-this"}
	r := newTestRouter(t, cfg)

	w := performRequest(r, "GET", "/static/uploads/slides/page1.png")
	if w.Code != 200 {
		t.Fatalf("期望 200，得到 %d", w.Code)
	}
	body, _ := io.ReadAll(w.Body)
	if string(body) != "fake-png-data" {
		t.Fatalf("期望 'fake-png-data'，得到 %s", string(body))
	}
}

// TestStaticNotFound 测试不存在的文件返回 404。
func TestStaticNotFound(t *testing.T) {
	cfg := &config.Config{UploadFolder: t.TempDir()}
	r := newTestRouter(t, cfg)

	w := performRequest(r, "GET", "/static/uploads/nonexistent.txt")
	if w.Code != 404 {
		t.Fatalf("期望 404，得到 %d", w.Code)
	}
}

// TestStaticPathTraversal 测试路径穿越防护。
func TestStaticPathTraversal(t *testing.T) {
	cfg := &config.Config{UploadFolder: t.TempDir()}
	r := newTestRouter(t, cfg)

	w := performRequest(r, "GET", "/static/uploads/../../../etc/passwd")
	if w.Code != 404 {
		t.Fatalf("期望 404 防路径穿越，得到 %d", w.Code)
	}
}

// TestStaticOtherResource 测试 /static/* 其他静态资源从本地 static/ 提供。
func TestStaticOtherResource(t *testing.T) {
	// 在工作目录的 static/ 下创建测试文件
	staticDir := "static"
	_ = os.MkdirAll(staticDir, 0o755)
	testFile := filepath.Join(staticDir, "favicon.ico")
	if err := os.WriteFile(testFile, []byte("fake-icon"), 0o644); err != nil {
		t.Fatal(err)
	}
	defer os.Remove(testFile)

	cfg := &config.Config{UploadFolder: t.TempDir()}
	r := newTestRouter(t, cfg)

	w := performRequest(r, "GET", "/static/favicon.ico")
	if w.Code != 200 {
		t.Fatalf("期望 200，得到 %d", w.Code)
	}
	body, _ := io.ReadAll(w.Body)
	if string(body) != "fake-icon" {
		t.Fatalf("期望 'fake-icon'，得到 %s", string(body))
	}
}

// TestResolveUploadDir 测试上传目录解析逻辑。
func TestResolveUploadDir(t *testing.T) {
	// VOLUME_MOUNT_PATH 优先
	volDir := t.TempDir()
	cfg := &config.Config{VolumeMountPath: volDir, UploadFolder: "/some/local"}
	if dir := resolveUploadDir(cfg); dir != filepath.Join(volDir, "uploads") {
		t.Fatalf("VOLUME_MOUNT_PATH 优先失败: %s", dir)
	}

	// 无 VOLUME_MOUNT_PATH 时用 UploadFolder（resolveUploadDir 将其转为绝对路径）
	cfg = &config.Config{UploadFolder: "/custom/uploads"}
	if d := resolveUploadDir(cfg); filepath.Base(d) != "uploads" || !filepath.IsAbs(d) {
		t.Fatalf("UploadFolder 回退失败: %s", d)
	}

	// 均无时用默认（resolveUploadDir 返回绝对路径）
	cfg = &config.Config{}
	if d := resolveUploadDir(cfg); filepath.Base(d) != "uploads" {
		t.Fatalf("默认值失败: %s", d)
	}
}

// 占位：确保 gorm.DB 引用不报未使用
var _ *gorm.DB
