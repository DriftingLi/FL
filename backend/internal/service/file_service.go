// Package service 文件上传与 PPT 转换。
//
// 重构说明：存储行为抽象为 storage.Storage 接口，FileService 持有该接口，
// 实际存储后端由配置决定（local 本地磁盘 / r2 Cloudflare R2 对象存储）。
// SaveFile 返回完整可访问 URL（local 模式 /static/uploads/...，r2 模式 https://...）。
// ConvertPPTToImages 改为接收 PPT 二进制内容，通过 sidecar multipart 接口完成转换后，
// sidecar 直接返回 WebP 字节，由 FileService 上传到存储后端并返回 URL 列表。
//
// 图片压缩：所有可压缩图片（jpg/png/bmp/webp/tiff）统一通过 sidecar 的
// /convert-image 接口转为 WebP（质量 85）。sidecar 未配置或转换失败时保留原格式。
// svg/gif 不压缩（svg 矢量图 gzip 更优，gif 转换会丢动图帧）。
package service

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"log"
	"mime/multipart"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"forklift-training/internal/storage"
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
	storage               storage.Storage
	libreofficeSidecarURL string // LibreOffice sidecar HTTP 地址(如 http://libreoffice:8000);为空则降级到本地 exec
	httpClient            *http.Client
}

// NewFileService 创建文件服务实例。
// libreofficeSidecarURL 为空时降级到本地 LibreOffice exec 调用(向后兼容)。
func NewFileService(libreofficeSidecarURL string, st storage.Storage) *FileService {
	return &FileService{
		storage:               st,
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

// SaveFile 保存文件到存储后端，返回可访问 URL。
// key 设计为 <subfolder>/<name>_<毫秒时间戳>.<ext>，保证唯一性。
// 可压缩图片（jpg/png/bmp 等，不含 svg/gif）会自动转 WebP 后再上传：
//   - sidecar 已配置：调用 /convert-image 转换
//   - sidecar 未配置或转换失败：保留原格式上传
func (s *FileService) SaveFile(content []byte, filename, subfolder string) (string, error) {
	ext := filepath.Ext(filename)
	name := strings.TrimSuffix(filename, ext)
	timestamp := strconv.FormatInt(time.Now().UnixMilli(), 10)

	// 图片转 WebP 压缩
	contentType := mimeTypeFromExt(fileExtension(filename))
	finalExt := ext
	if shouldCompressImage(contentType) {
		if webpData, ok := s.compressImageViaSidecar(content); ok {
			content = webpData
			contentType = "image/webp"
			finalExt = ".webp"
		}
	}

	uniqueFilename := fmt.Sprintf("%s_%s%s", name, timestamp, finalExt)
	key := fmt.Sprintf("%s/%s", subfolder, uniqueFilename)

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	return s.storage.Save(ctx, key, content, contentType)
}

// DeleteFile 删除文件。URL 为空时直接返回。
func (s *FileService) DeleteFile(fileURL string) error {
	if fileURL == "" {
		return nil
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	return s.storage.Delete(ctx, fileURL)
}

// DeleteFiles 批量删除文件（忽略空 URL，单个失败不阻断其余删除）。
func (s *FileService) DeleteFiles(urls []string) {
	for _, u := range urls {
		if u == "" {
			continue
		}
		_ = s.DeleteFile(u)
	}
}

// ListFiles 按 key 前缀列出文件 URL（如 "images/forum"、"slides/12"）。
func (s *FileService) ListFiles(prefix string) []string {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	urls, err := s.storage.List(ctx, prefix)
	if err != nil {
		log.Printf("[file_service] List 失败 prefix=%s: %v", prefix, err)
		return nil
	}
	return urls
}

// ConvertPPTToImages 将 PPT 二进制内容转为图片并上传到存储后端。
// 返回幻灯片图片 URL 列表（按 slide_001.webp、slide_002.webp... 顺序）。
//
// 转换流程：PPT bytes → sidecar(multipart) → WebP bytes(base64) → storage.Save。
// sidecar 在 PPT → PDF → PNG 后直接转 WebP 返回，后端无需再压缩。
// 若 sidecar 未配置则降级到本地 LibreOffice exec（写临时文件转换后上传 PNG，不转 WebP）。
// 失败时返回占位图片 URL 列表（单张 1x1 像素 PNG）以避免阻塞业务流程。
func (s *FileService) ConvertPPTToImages(pptContent []byte, chapterID int) []string {
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

	// 优先:LibreOffice sidecar HTTP 调用(multipart 上传 PPT bytes)
	if s.libreofficeSidecarURL != "" {
		images, success = s.convertWithSidecar(pptContent, chapterID)
	} else {
		// 降级:本地 LibreOffice exec(返回 PNG,不转 WebP)
		images, success = s.convertWithLibreOffice(pptContent, chapterID)
	}

	if !success || len(images) == 0 {
		// 占位图片:生成单张 1x1 像素 PNG 上传到存储后端
		placeholderURL := s.uploadPlaceholder(chapterID)
		if placeholderURL != "" {
			return []string{placeholderURL}
		}
		return nil
	}

	// 逐张上传 slide 到存储后端
	// sidecar 路径下 name 已是 .webp，CT 为 image/webp；本地降级路径下 name 为 .png，CT 为 image/png
	urls := make([]string, 0, len(images))
	for _, img := range images {
		imgData, err := base64Decode(img.Data)
		if err != nil {
			log.Printf("[file_service] base64 解码失败 name=%s: %v", img.Name, err)
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
			log.Printf("[file_service] 上传 slide 失败 key=%s: %v", key, err)
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
// 通过 multipart 上传 PPT bytes，接收 JSON+base64 响应（图片已是 WebP）。
func (s *FileService) convertWithSidecar(pptContent []byte, chapterID int) ([]struct {
	Name string `json:"name"`
	Data string `json:"data"`
}, bool) {
	// 构造 multipart body
	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	_ = writer.WriteField("chapter_id", strconv.Itoa(chapterID))
	part, err := writer.CreateFormFile("file", fmt.Sprintf("chapter_%d.pptx", chapterID))
	if err != nil {
		log.Printf("[file_service] 构造 multipart 失败: %v", err)
		return nil, false
	}
	if _, err := part.Write(pptContent); err != nil {
		log.Printf("[file_service] 写入 multipart 失败: %v", err)
		return nil, false
	}
	_ = writer.Close()

	url := strings.TrimSuffix(s.libreofficeSidecarURL, "/") + "/convert"
	req, err := http.NewRequestWithContext(context.Background(), "POST", url, body)
	if err != nil {
		log.Printf("[file_service] 构造 sidecar 请求失败: %v", err)
		return nil, false
	}
	req.Header.Set("Content-Type", writer.FormDataContentType())

	resp, err := s.httpClient.Do(req)
	if err != nil {
		log.Printf("[file_service] 调用 LibreOffice sidecar 失败: %v", err)
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
		log.Printf("[file_service] 解析 sidecar 响应失败: %v", err)
		return nil, false
	}
	if !result.Success {
		log.Printf("[file_service] sidecar 转换失败: %s", result.Error)
		return nil, false
	}
	return result.Images, true
}

// convertWithLibreOffice 调用本地 LibreOffice headless 将 PPT 转 PDF，再转图片。
// 降级路径:仅当 libreofficeSidecarURL 为空时使用。
// 接收 PPT bytes，写临时文件转换后读取 PNG bytes 返回（不转 WebP）。
func (s *FileService) convertWithLibreOffice(pptContent []byte, chapterID int) ([]struct {
	Name string `json:"name"`
	Data string `json:"data"`
}, bool) {
	soffice := findLibreOffice()
	if soffice == "" {
		log.Printf("[file_service] LibreOffice 未安装，跳过 PPT 转换")
		return nil, false
	}

	tmpDir, err := os.MkdirTemp("", fmt.Sprintf("ppt_%d_*", chapterID))
	if err != nil {
		log.Printf("[file_service] 创建临时目录失败: %v", err)
		return nil, false
	}
	defer os.RemoveAll(tmpDir)

	pptPath := filepath.Join(tmpDir, "input.pptx")
	if err := os.WriteFile(pptPath, pptContent, 0o644); err != nil {
		log.Printf("[file_service] 写入临时 PPT 失败: %v", err)
		return nil, false
	}

	// PPT → PDF
	cmd := exec.Command(soffice, "--headless", "--convert-to", "pdf", "--outdir", tmpDir, pptPath)
	if err := cmd.Run(); err != nil {
		log.Printf("[file_service] LibreOffice 转换失败: %v", err)
		return nil, false
	}

	// 查找生成的 PDF
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
		log.Printf("[file_service] 未找到转换后的 PDF")
		return nil, false
	}

	// PDF → PNG (pdftoppm)
	pdftoppm := findExecutable("pdftoppm")
	if pdftoppm == "" {
		log.Printf("[file_service] 无 pdftoppm 工具")
		return nil, false
	}
	prefix := filepath.Join(tmpDir, "slide")
	cmd2 := exec.Command(pdftoppm, "-png", "-r", "150", pdfPath, prefix)
	if err := cmd2.Run(); err != nil {
		log.Printf("[file_service] pdftoppm 转换失败: %v", err)
		return nil, false
	}

	// 读取 PNG 文件，重命名为 slide_001.png 并 base64 编码
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
		// slide-1.png → slide_001.png
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
// 占位图为 PNG（体积已极小，无需转 WebP）。
func (s *FileService) uploadPlaceholder(chapterID int) string {
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
		log.Printf("[file_service] 上传占位图失败: %v", err)
		return ""
	}
	return url
}

// compressImageViaSidecar 调用 sidecar 的 /convert-image 接口将图片转为 WebP。
// 返回 (webpBytes, ok)；sidecar 未配置、转换失败或返回原始数据时 ok=false。
// 调用方应在 ok=false 时保留原始字节上传。
func (s *FileService) compressImageViaSidecar(content []byte) ([]byte, bool) {
	if s.libreofficeSidecarURL == "" {
		// 本地降级模式：无 sidecar，跳过压缩
		return nil, false
	}
	if len(content) == 0 {
		return nil, false
	}

	// 构造 multipart body
	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	part, err := writer.CreateFormFile("file", "image.bin")
	if err != nil {
		log.Printf("[file_service] 构造图片压缩 multipart 失败: %v", err)
		return nil, false
	}
	if _, err := part.Write(content); err != nil {
		log.Printf("[file_service] 写入图片压缩 multipart 失败: %v", err)
		return nil, false
	}
	_ = writer.Close()

	url := strings.TrimSuffix(s.libreofficeSidecarURL, "/") + "/convert-image"
	req, err := http.NewRequestWithContext(context.Background(), "POST", url, body)
	if err != nil {
		log.Printf("[file_service] 构造图片压缩请求失败: %v", err)
		return nil, false
	}
	req.Header.Set("Content-Type", writer.FormDataContentType())

	resp, err := s.httpClient.Do(req)
	if err != nil {
		log.Printf("[file_service] 调用 sidecar 图片压缩失败: %v", err)
		return nil, false
	}
	defer resp.Body.Close()

	var result struct {
		Success bool   `json:"success"`
		Status  string `json:"status"` // WEBP / ORIGINAL / SKIP
		Data    string `json:"data"`   // base64 编码
		Error   string `json:"error"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		log.Printf("[file_service] 解析 sidecar 图片压缩响应失败: %v", err)
		return nil, false
	}
	if !result.Success {
		log.Printf("[file_service] sidecar 图片压缩失败: %s", result.Error)
		return nil, false
	}

	imgData, err := base64Decode(result.Data)
	if err != nil {
		log.Printf("[file_service] sidecar 图片压缩响应 base64 解码失败: %v", err)
		return nil, false
	}

	// status=WEBP 时才用转换后的数据；ORIGINAL/SKIP 时保留原始字节
	if result.Status == "WEBP" {
		return imgData, true
	}
	return nil, false
}

// ===== 辅助函数 =====

// shouldCompressImage 判断该 MIME 类型是否需要转 WebP 压缩。
// 排除：svg（矢量图，gzip 更优）、gif（动图会丢帧）、已经是 webp。
func shouldCompressImage(contentType string) bool {
	switch contentType {
	case "image/jpeg", "image/jpg", "image/png", "image/bmp":
		return true
	case "image/webp", "image/gif", "image/svg+xml":
		return false
	}
	return false
}

// mimeTypeFromExt 根据文件扩展名返回 MIME 类型。
func mimeTypeFromExt(ext string) string {
	switch ext {
	case "pdf":
		return "application/pdf"
	case "ppt":
		return "application/vnd.ms-powerpoint"
	case "pptx":
		return "application/vnd.openxmlformats-officedocument.presentationml.presentation"
	case "mp4":
		return "video/mp4"
	case "webm":
		return "video/webm"
	case "png":
		return "image/png"
	case "jpg", "jpeg":
		return "image/jpeg"
	case "gif":
		return "image/gif"
	case "webp":
		return "image/webp"
	case "bmp":
		return "image/bmp"
	case "svg":
		return "image/svg+xml"
	default:
		return "application/octet-stream"
	}
}

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

// base64Encode / base64Decode 包装 encoding/base64，集中管理编解码逻辑。
func base64Encode(data []byte) string {
	return base64.StdEncoding.EncodeToString(data)
}

func base64Decode(s string) ([]byte, error) {
	return base64.StdEncoding.DecodeString(s)
}
