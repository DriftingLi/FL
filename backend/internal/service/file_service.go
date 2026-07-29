// Package service 文件上传与 PPT 转换。
package service

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

// 文件扩展名白名单（文档仅允许 PDF，浏览器原生预览；其他 Office 格式无法内嵌渲染故移除）。
var allowedExtensions = map[string]map[string]bool{
	"document": {"pdf": true},
	"ppt":      {"ppt": true, "pptx": true},
	"video":    {"mp4": true, "webm": true},
	"image":    {"png": true, "jpg": true, "jpeg": true, "gif": true, "webp": true, "bmp": true, "svg": true},
}

// 文件大小限制。
var maxFileSizes = map[string]int64{
	"video":   200 * 1024 * 1024,
	"image":   20 * 1024 * 1024,
	"default": 50 * 1024 * 1024,
}

// FileService 文件服务。
type FileService struct {
	uploadFolder          string
	libreofficeSidecarURL string // LibreOffice sidecar HTTP 地址(如 http://libreoffice:8000);为空则降级到本地 exec
	httpClient            *http.Client
}

// NewFileService 创建文件服务实例。
// libreofficeSidecarURL 为空时降级到本地 LibreOffice exec 调用(向后兼容)。
func NewFileService(uploadFolder, libreofficeSidecarURL string) *FileService {
	return &FileService{
		uploadFolder:          uploadFolder,
		libreofficeSidecarURL: libreofficeSidecarURL,
		httpClient: &http.Client{
			Timeout: 180 * time.Second, // PPT 转换可能较慢
		},
	}
}

// GetContentType 获取文件内容类型。
func (s *FileService) GetContentType(filename string) string {
	ext := fileExtension(filename)
	for contentType, exts := range allowedExtensions {
		if exts[ext] {
			return contentType
		}
	}
	return ""
}

// AllowedFile 是否允许的文件格式。
func (s *FileService) AllowedFile(filename string) bool {
	return s.GetContentType(filename) != ""
}

// ValidateFileSize 校验文件大小。
func (s *FileService) ValidateFileSize(size int64, filename string) bool {
	contentType := s.GetContentType(filename)
	maxSize := maxFileSizes["default"]
	if m, ok := maxFileSizes[contentType]; ok {
		maxSize = m
	}
	return size <= maxSize
}

// ValidateImageFile 校验图片文件。
func (s *FileService) ValidateImageFile(filename string, size int64) (bool, string) {
	ext := fileExtension(filename)
	if !allowedExtensions["image"][ext] {
		allowedList := make([]string, 0)
		for k := range allowedExtensions["image"] {
			allowedList = append(allowedList, k)
		}
		return false, fmt.Sprintf("不支持的图片格式，允许格式：%s", strings.Join(allowedList, ", "))
	}
	if size > maxFileSizes["image"] {
		return false, fmt.Sprintf("图片大小超出限制，最大允许%dMB", maxFileSizes["image"]/(1024*1024))
	}
	return true, ""
}

// SaveFile 保存文件，返回 file_url 与 file_path。
func (s *FileService) SaveFile(content []byte, filename, subfolder string) (string, string) {
	saveDir := filepath.Join(s.uploadFolder, subfolder)
	_ = os.MkdirAll(saveDir, 0755)

	ext := filepath.Ext(filename)
	name := strings.TrimSuffix(filename, ext)
	timestamp := strconv.FormatInt(time.Now().UnixMilli(), 10)
	uniqueFilename := fmt.Sprintf("%s_%s%s", name, timestamp, ext)

	filePath := filepath.Join(saveDir, uniqueFilename)
	_ = os.WriteFile(filePath, content, 0644)

	fileURL := fmt.Sprintf("/static/uploads/%s/%s", subfolder, uniqueFilename)
	return fileURL, filePath
}

// DeleteFile 删除文件。
func (s *FileService) DeleteFile(fileURL string) {
	if fileURL == "" {
		return
	}
	relative := strings.TrimPrefix(fileURL, "/static/uploads/")
	filePath := filepath.Join(s.uploadFolder, relative)
	if _, err := os.Stat(filePath); err == nil {
		_ = os.Remove(filePath)
	}
}

// ConvertPPTToImages 将 PPT 转为图片。
// 转换流程：PPT → PDF（LibreOffice headless）→ PNG 图片。
// 优先调用 LibreOffice sidecar HTTP 服务；若未配置则降级到本地 exec。
// 失败时返回占位图片 URL 列表。
func (s *FileService) ConvertPPTToImages(pptPath, outputDir string) []string {
	if _, err := os.Stat(pptPath); err != nil {
		return nil
	}
	_ = os.MkdirAll(outputDir, 0755)

	// 已有图片直接返回
	existing := listSlideImages(outputDir)
	if len(existing) > 0 {
		urls := make([]string, 0, len(existing))
		baseName := filepath.Base(outputDir)
		for _, img := range existing {
			urls = append(urls, fmt.Sprintf("/static/uploads/slides/%s/%s", baseName, img))
		}
		return urls
	}

	// 优先:LibreOffice sidecar HTTP 调用
	if s.libreofficeSidecarURL != "" {
		if s.convertWithSidecar(pptPath, outputDir) {
			images := listSlideImages(outputDir)
			if len(images) > 0 {
				urls := make([]string, 0, len(images))
				baseName := filepath.Base(outputDir)
				for _, img := range images {
					urls = append(urls, fmt.Sprintf("/static/uploads/slides/%s/%s", baseName, img))
				}
				return urls
			}
		}
	} else {
		// 降级:本地 LibreOffice exec
		if s.convertWithLibreOffice(pptPath, outputDir) {
			images := listSlideImages(outputDir)
			if len(images) > 0 {
				urls := make([]string, 0, len(images))
				baseName := filepath.Base(outputDir)
				for _, img := range images {
					urls = append(urls, fmt.Sprintf("/static/uploads/slides/%s/%s", baseName, img))
				}
				return urls
			}
		}
	}

	// 占位图片
	s.createPlaceholderImages(outputDir)
	images := listSlideImages(outputDir)
	urls := make([]string, 0, len(images))
	baseName := filepath.Base(outputDir)
	for _, img := range images {
		urls = append(urls, fmt.Sprintf("/static/uploads/slides/%s/%s", baseName, img))
	}
	return urls
}

// convertWithSidecar 调用 LibreOffice sidecar HTTP 服务进行 PPT → 图片转换。
// backend 和 sidecar 共享 /data/uploads volume,因此路径必须用 sidecar 容器内路径。
// sidecarSidecarPathPrefix 为空时使用 uploadFolder 原值。
func (s *FileService) convertWithSidecar(pptPath, outputDir string) bool {
	sidecarInputPath := s.toSidecarPath(pptPath)
	sidecarOutputDir := s.toSidecarPath(outputDir)

	reqBody := struct {
		InputPath string `json:"input_path"`
		OutputDir string `json:"output_dir"`
	}{
		InputPath: sidecarInputPath,
		OutputDir: sidecarOutputDir,
	}
	bodyBytes, _ := json.Marshal(reqBody)

	url := strings.TrimSuffix(s.libreofficeSidecarURL, "/") + "/convert"
	req, err := http.NewRequestWithContext(context.Background(), "POST", url, bytes.NewReader(bodyBytes))
	if err != nil {
		log.Printf("[file_service] 构造 sidecar 请求失败: %v", err)
		return false
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := s.httpClient.Do(req)
	if err != nil {
		log.Printf("[file_service] 调用 LibreOffice sidecar 失败: %v", err)
		return false
	}
	defer resp.Body.Close()

	var result struct {
		Success bool     `json:"success"`
		Error   string   `json:"error"`
		Images  []string `json:"images"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		log.Printf("[file_service] 解析 sidecar 响应失败: %v", err)
		return false
	}
	if !result.Success {
		log.Printf("[file_service] sidecar 转换失败: %s", result.Error)
		return false
	}
	return len(result.Images) > 0
}

// toSidecarPath 把 backend 容器内的路径转换为 sidecar 容器内的路径。
// backend 容器: UPLOAD_FOLDER=/data/uploads (或本地 static/uploads)
// sidecar 容器: 挂载相同的 /data/uploads volume
// 约定:两边 volume 挂载点相同,因此路径直接透传;若不同则需通过环境变量 LIBREOFFICE_SIDECAR_DATA_PREFIX 配置前缀映射。
func (s *FileService) toSidecarPath(path string) string {
	// 当前实现:backend 与 sidecar 共享 /data/uploads,路径一致,直接返回。
	// 若未来挂载点不同,可在此处做前缀替换。
	return path
}

// convertWithLibreOffice 调用 LibreOffice headless 将 PPT 转 PDF，再转图片。
// 降级路径:仅当 libreofficeSidecarURL 为空时使用。
func (s *FileService) convertWithLibreOffice(pptPath, outputDir string) bool {
	soffice := findLibreOffice()
	if soffice == "" {
		log.Printf("[file_service] LibreOffice 未安装，跳过 PPT 转换")
		return false
	}

	cmd := exec.Command(soffice, "--headless", "--convert-to", "pdf", "--outdir", outputDir, pptPath)
	if err := cmd.Run(); err != nil {
		log.Printf("[file_service] LibreOffice 转换失败: %v", err)
		return false
	}

	baseName := strings.TrimSuffix(filepath.Base(pptPath), filepath.Ext(pptPath))
	pdfPath := filepath.Join(outputDir, baseName+".pdf")
	if _, err := os.Stat(pdfPath); err != nil {
		// 查找任何 PDF 文件
		entries, _ := os.ReadDir(outputDir)
		for _, e := range entries {
			if strings.HasSuffix(strings.ToLower(e.Name()), ".pdf") {
				pdfPath = filepath.Join(outputDir, e.Name())
				break
			}
		}
	}

	success := s.convertPDFToImages(pdfPath, outputDir)
	if success {
		_ = os.Remove(pdfPath)
	}
	return success
}

// convertPDFToImages 将 PDF 转为 PNG 图片。
// 使用 pdfcpu 或 pdftoppm（poppler-utils），若无则返回 false。
func (s *FileService) convertPDFToImages(pdfPath, outputDir string) bool {
	// 尝试 pdftoppm（poppler-utils）
	if pdftoppm := findExecutable("pdftoppm"); pdftoppm != "" {
		// pdftoppm -png -r 150 input.pdf slide
		prefix := filepath.Join(outputDir, "slide")
		cmd := exec.Command(pdftoppm, "-png", "-r", "150", pdfPath, prefix)
		if err := cmd.Run(); err != nil {
			log.Printf("[file_service] pdftoppm 转换失败: %v", err)
		} else {
			s.renamePDFImages(outputDir)
			return true
		}
	}

	log.Printf("[file_service] 无可用的 PDF 转图片工具（pdftoppm）")
	return false
}

// renamePDFImages 将 pdftoppm 输出（slide-1.png）重命名为 slide_001.png。
func (s *FileService) renamePDFImages(outputDir string) {
	entries, _ := os.ReadDir(outputDir)
	for _, e := range entries {
		name := e.Name()
		if !strings.HasSuffix(strings.ToLower(name), ".png") {
			continue
		}
		// slide-1.png → slide_001.png
		parts := strings.Split(name, "-")
		if len(parts) == 2 {
			num := strings.TrimSuffix(parts[1], ".png")
			n, err := strconv.Atoi(num)
			if err == nil {
				newName := fmt.Sprintf("slide_%03d.png", n)
				_ = os.Rename(filepath.Join(outputDir, name), filepath.Join(outputDir, newName))
			}
		}
	}
}

// createPlaceholderImages 创建占位图片。
func (s *FileService) createPlaceholderImages(outputDir string) {
	// 简单占位：创建一个 1x1 像素 PNG
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
	slidePath := filepath.Join(outputDir, "slide_001.png")
	_ = os.WriteFile(slidePath, placeholderPNG, 0644)
}

// ===== 辅助函数 =====

func fileExtension(filename string) string {
	idx := strings.LastIndex(filename, ".")
	if idx < 0 {
		return ""
	}
	return strings.ToLower(filename[idx+1:])
}

func findLibreOffice() string {
	// 查找 PATH 中的 soffice/libreoffice
	for _, name := range []string{"soffice", "libreoffice"} {
		if path, err := exec.LookPath(name); err == nil {
			return path
		}
	}
	// 常见安装路径
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
