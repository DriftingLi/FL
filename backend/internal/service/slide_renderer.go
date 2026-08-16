// Package service PPT 转图 module：Render(ppt, chapter) 收敛 sidecar 与本地 LibreOffice
// 两个 adapter 以及占位图 fallback（ADR-0015）。
package service

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"mime/multipart"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"go.uber.org/zap"

	"forklift-training/internal/storage"
)

// SlideRenderer PPT 转图 module。
type SlideRenderer struct {
	storage               storage.Storage
	libreofficeSidecarURL string
	httpClient            *http.Client

	logger *zap.Logger
}

// NewSlideRenderer 创建 PPT 转图实例。
// libreofficeSidecarURL 为空时降级到本地 LibreOffice exec。
func NewSlideRenderer(libreofficeSidecarURL string, st storage.Storage, logger *zap.Logger) *SlideRenderer {
	return &SlideRenderer{
		storage:               st,
		libreofficeSidecarURL: libreofficeSidecarURL,
		httpClient:            &http.Client{Timeout: 180 * time.Second},
		logger:                logger,
	}
}

// Render 将 PPT bytes 转为图片并上传到存储后端，返回 slide URL 列表。
// 失败时返回占位图片 URL 列表（单张 1x1 像素 PNG）以避免阻塞业务流程。
func (s *SlideRenderer) Render(pptContent []byte, chapterID int) []string {
	if len(pptContent) == 0 {
		return nil
	}

	var (
		images []struct {
			Name string `json:"name"`
			Data string `json:"data"` // base64 编码
		}
		success bool
	)

	if s.libreofficeSidecarURL != "" {
		images, success = s.convertWithSidecar(pptContent, chapterID)
	} else {
		images, success = s.convertWithLibreOffice(pptContent, chapterID)
	}

	if !success || len(images) == 0 {
		placeholderURL := s.uploadPlaceholder(chapterID)
		if placeholderURL != "" {
			return []string{placeholderURL}
		}
		return nil
	}

	urls := make([]string, 0, len(images))
	for _, img := range images {
		imgData, err := base64Decode(img.Data)
		if err != nil {
			s.logger.Warn("[slide_renderer] base64 解码失败", zap.String("name", img.Name), zap.Error(err))
			continue
		}
		imgCT := "image/png"
		if strings.HasSuffix(strings.ToLower(img.Name), ".webp") {
			imgCT = "image/webp"
		}
		key := fmt.Sprintf("slides/%d/%s", chapterID, img.Name)
		ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
		url, err := s.storage.Save(ctx, key, imgData, imgCT)
		cancel()
		if err != nil {
			s.logger.Warn("[slide_renderer] 上传 slide 失败", zap.String("key", key), zap.Error(err))
			continue
		}
		urls = append(urls, url)
	}

	if len(urls) == 0 {
		placeholderURL := s.uploadPlaceholder(chapterID)
		if placeholderURL != "" {
			return []string{placeholderURL}
		}
	}
	return urls
}

// convertWithSidecar 调用 LibreOffice sidecar HTTP 服务进行 PPT → WebP 转换。
func (s *SlideRenderer) convertWithSidecar(pptContent []byte, chapterID int) ([]struct {
	Name string `json:"name"`
	Data string `json:"data"`
}, bool) {
	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	_ = writer.WriteField("chapter_id", strconv.Itoa(chapterID))
	part, err := writer.CreateFormFile("file", fmt.Sprintf("chapter_%d.pptx", chapterID))
	if err != nil {
		s.logger.Warn("[slide_renderer] 构造 multipart 失败", zap.Error(err))
		return nil, false
	}
	if _, err := part.Write(pptContent); err != nil {
		s.logger.Warn("[slide_renderer] 写入 multipart 失败", zap.Error(err))
		return nil, false
	}
	_ = writer.Close()

	url := strings.TrimSuffix(s.libreofficeSidecarURL, "/") + "/convert"
	req, err := http.NewRequestWithContext(context.Background(), "POST", url, body)
	if err != nil {
		s.logger.Warn("[slide_renderer] 构造 sidecar 请求失败", zap.Error(err))
		return nil, false
	}
	req.Header.Set("Content-Type", writer.FormDataContentType())

	resp, err := s.httpClient.Do(req)
	if err != nil {
		s.logger.Warn("[slide_renderer] 调用 LibreOffice sidecar 失败", zap.Error(err))
		return nil, false
	}
	defer resp.Body.Close()

	var result struct {
		Success bool   `json:"success"`
		Error   string `json:"error"`
		Images  []struct {
			Name string `json:"name"`
			Data string `json:"data"`
		} `json:"images"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		s.logger.Warn("[slide_renderer] 解析 sidecar 响应失败", zap.Error(err))
		return nil, false
	}
	if !result.Success {
		s.logger.Warn("[slide_renderer] sidecar 转换失败", zap.String("detail", result.Error))
		return nil, false
	}
	return result.Images, true
}

// convertWithLibreOffice 调用本地 LibreOffice headless 将 PPT 转 PDF，再转图片。
func (s *SlideRenderer) convertWithLibreOffice(pptContent []byte, chapterID int) ([]struct {
	Name string `json:"name"`
	Data string `json:"data"`
}, bool) {
	soffice := findLibreOffice()
	if soffice == "" {
		s.logger.Warn("[slide_renderer] LibreOffice 未安装，跳过 PPT 转换")
		return nil, false
	}

	tmpDir, err := os.MkdirTemp("", fmt.Sprintf("ppt_%d_*", chapterID))
	if err != nil {
		s.logger.Warn("[slide_renderer] 创建临时目录失败", zap.Error(err))
		return nil, false
	}
	defer os.RemoveAll(tmpDir)

	pptPath := filepath.Join(tmpDir, "input.pptx")
	if err := os.WriteFile(pptPath, pptContent, 0o644); err != nil {
		s.logger.Warn("[slide_renderer] 写入临时 PPT 失败", zap.Error(err))
		return nil, false
	}

	cmd := exec.Command(soffice, "--headless", "--convert-to", "pdf", "--outdir", tmpDir, pptPath)
	if err := cmd.Run(); err != nil {
		s.logger.Warn("[slide_renderer] LibreOffice 转换失败", zap.Error(err))
		return nil, false
	}

	pdfPath := filepath.Join(tmpDir, "input.pdf")
	if _, err := os.Stat(pdfPath); err != nil {
		entries, _ := os.ReadDir(tmpDir)
		for _, e := range entries {
			if strings.HasSuffix(strings.ToLower(e.Name()), ".pdf") {
				pdfPath = filepath.Join(tmpDir, e.Name())
				break
			}
		}
	}
	if _, err := os.Stat(pdfPath); err != nil {
		s.logger.Warn("[slide_renderer] 未找到转换后的 PDF")
		return nil, false
	}

	pdftoppm := findExecutable("pdftoppm")
	if pdftoppm == "" {
		s.logger.Warn("[slide_renderer] 无 pdftoppm 工具")
		return nil, false
	}
	prefix := filepath.Join(tmpDir, "slide")
	cmd2 := exec.Command(pdftoppm, "-png", "-r", "150", pdfPath, prefix)
	if err := cmd2.Run(); err != nil {
		s.logger.Warn("[slide_renderer] pdftoppm 转换失败", zap.Error(err))
		return nil, false
	}

	entries, _ := os.ReadDir(tmpDir)
	var images []struct {
		Name string `json:"name"`
		Data string `json:"data"`
	}
	for _, e := range entries {
		name := e.Name()
		if !strings.HasSuffix(strings.ToLower(name), ".png") {
			continue
		}
		finalName := name
		parts := strings.Split(name, "-")
		if len(parts) == 2 {
			num := strings.TrimSuffix(parts[1], ".png")
			if n, err := strconv.Atoi(num); err == nil {
				finalName = fmt.Sprintf("slide_%03d.png", n)
			}
		}
		imgPath := filepath.Join(tmpDir, name)
		data, err := os.ReadFile(imgPath)
		if err != nil {
			continue
		}
		images = append(images, struct {
			Name string `json:"name"`
			Data string `json:"data"`
		}{
			Name: finalName,
			Data: base64Encode(data),
		})
	}

	if len(images) == 0 {
		return nil, false
	}
	return images, true
}

// uploadPlaceholder 上传一张 1x1 像素占位图到存储后端，返回其 URL。
func (s *SlideRenderer) uploadPlaceholder(chapterID int) string {
	placeholderPNG := []byte{
		0x89, 0x50, 0x4E, 0x47, 0x0D, 0x0A, 0x1A, 0x0A, // PNG signature
		0x00, 0x00, 0x00, 0x0D, 0x49, 0x48, 0x44, 0x52,
		0x00, 0x00, 0x00, 0x01, 0x00, 0x00, 0x00, 0x01,
		0x08, 0x06, 0x00, 0x00, 0x00, 0x1F, 0x15, 0xC4,
		0x89, 0x00, 0x00, 0x00, 0x0D, 0x49, 0x44, 0x41,
		0x54, 0x78, 0x9C, 0x62, 0x00, 0x01, 0x00, 0x00,
		0x05, 0x00, 0x01, 0x0D, 0x0A, 0x2D, 0xB4, 0x00,
		0x00, 0x00, 0x00, 0x49, 0x45, 0x4E, 0x44, 0xAE,
		0x42, 0x60, 0x82,
	}

	key := fmt.Sprintf("slides/%d/slide_001.png", chapterID)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	url, err := s.storage.Save(ctx, key, placeholderPNG, "image/png")
	if err != nil {
		s.logger.Warn("[slide_renderer] 上传占位图失败", zap.Error(err))
		return ""
	}
	return url
}

func findLibreOffice() string {
	for _, name := range []string{"soffice", "libreoffice"} {
		if path, err := exec.LookPath(name); err == nil {
			return path
		}
	}
	commonPaths := []string{
		`C:\Program Files\LibreOffice\program\soffice.exe`,
		`C:\Program Files (x86)\LibreOffice\program\soffice.exe`,
		"/usr/bin/libreoffice",
		"/usr/bin/soffice",
		"/snap/bin/libreoffice",
		"/Applications/LibreOffice.app/Contents/MacOS/soffice",
	}
	for _, p := range commonPaths {
		if _, err := os.Stat(p); err == nil {
			return p
		}
	}
	return ""
}

func findExecutable(name string) string {
	path, err := exec.LookPath(name)
	if err != nil {
		return ""
	}
	return path
}
