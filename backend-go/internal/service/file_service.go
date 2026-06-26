// Package service 文件上传与 PPT 转换，对应 Python file_service。
package service

import (
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

// 文件扩展名白名单，与 Python ALLOWED_EXTENSIONS 一致。
var allowedExtensions = map[string]map[string]bool{
	"document": {"pdf": true, "doc": true, "docx": true, "xls": true, "xlsx": true, "csv": true},
	"ppt":      {"ppt": true, "pptx": true},
	"video":    {"mp4": true, "webm": true},
	"image":    {"png": true, "jpg": true, "jpeg": true, "gif": true, "webp": true, "bmp": true, "svg": true},
}

// 文件大小限制，与 Python MAX_FILE_SIZES 一致。
var maxFileSizes = map[string]int64{
	"video":   200 * 1024 * 1024,
	"image":   20 * 1024 * 1024,
	"default": 50 * 1024 * 1024,
}

// FileService 文件服务。
type FileService struct {
	uploadFolder string
}

// NewFileService 创建文件服务实例。
func NewFileService(uploadFolder string) *FileService {
	return &FileService{uploadFolder: uploadFolder}
}

// GetContentType 获取文件内容类型，对应 Python get_content_type。
func (s *FileService) GetContentType(filename string) string {
	ext := fileExtension(filename)
	for contentType, exts := range allowedExtensions {
		if exts[ext] {
			return contentType
		}
	}
	return ""
}

// AllowedFile 是否允许的文件格式，对应 Python allowed_file。
func (s *FileService) AllowedFile(filename string) bool {
	return s.GetContentType(filename) != ""
}

// ValidateFileSize 校验文件大小，对应 Python validate_file_size。
func (s *FileService) ValidateFileSize(size int64, filename string) bool {
	contentType := s.GetContentType(filename)
	maxSize := maxFileSizes["default"]
	if m, ok := maxFileSizes[contentType]; ok {
		maxSize = m
	}
	return size <= maxSize
}

// ValidateImageFile 校验图片文件，对应 Python validate_image_file。
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

// SaveFile 保存文件，返回 file_url 与 file_path，对应 Python save_file。
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

// DeleteFile 删除文件，对应 Python delete_file。
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

// ConvertPPTToImages 将 PPT 转为图片，对应 Python convert_ppt_to_images。
// 转换流程：PPT → PDF（LibreOffice headless）→ PNG 图片。
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

	// 尝试 LibreOffice 转换
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

// convertWithLibreOffice 调用 LibreOffice headless 将 PPT 转 PDF，再转图片。
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
