// Package service 文件存储 module：上传、校验、删除与列表。
// 图片 WebP 压缩保留在 Save 的 implementation 内（ADR-0015）。
package service

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"go.uber.org/zap"

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

// FileStore 文件存储 module：Save / Delete / DeleteFiles / List / ValidateImage。
type FileStore struct {
	storage               storage.Storage
	libreofficeSidecarURL string
	httpClient            *http.Client

	logger *zap.Logger
}

// NewFileStore 创建文件存储实例。
// libreofficeSidecarURL 用于图片 WebP 压缩；为空时保留原始格式上传。
func NewFileStore(libreofficeSidecarURL string, st storage.Storage, logger *zap.Logger) *FileStore {
	return &FileStore{
		storage:               st,
		libreofficeSidecarURL: libreofficeSidecarURL,
		httpClient:            &http.Client{Timeout: 180 * time.Second},
		logger:                logger,
	}
}

// Save 保存文件到存储后端，返回可访问 URL。
// key 设计为 <subfolder>/<name>_<毫秒时间戳>.<ext>，保证唯一性。
// 时间戳命名契约由本 module 的 Save 写入；解析方自行遵守该契约。
// 可压缩图片（jpg/png/bmp 等，不含 svg/gif）会经 sidecar 转 WebP 后上传。
func (s *FileStore) Save(content []byte, filename, subfolder string) (string, error) {
	ext := filepath.Ext(filename)
	name := strings.TrimSuffix(filename, ext)
	timestamp := strconv.FormatInt(time.Now().UnixMilli(), 10)

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

// Delete 删除文件。URL 为空时直接返回。
func (s *FileStore) Delete(fileURL string) error {
	if fileURL == "" {
		return nil
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	return s.storage.Delete(ctx, fileURL)
}

// DeleteFiles 批量删除文件（忽略空 URL，单个失败不阻断其余删除）。
func (s *FileStore) DeleteFiles(urls []string) {
	for _, u := range urls {
		if u == "" {
			continue
		}
		_ = s.Delete(u)
	}
}

// DeleteWithContext 按 URL 删除文件，ctx 取消时及时返回。
func (s *FileStore) DeleteWithContext(ctx context.Context, url string) error {
	if url == "" {
		return nil
	}
	// 继承 caller ctx，叠加超时以避免永久阻塞
	ctx2, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	return s.storage.Delete(ctx2, url)
}

// List 按 key 前缀列出文件 URL（如 "images/forum"、"slides/12"）。
func (s *FileStore) List(prefix string) []string {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	urls, err := s.storage.List(ctx, prefix)
	if err != nil {
		s.logger.Warn("[file_store] List 失败", zap.String("prefix", prefix), zap.Error(err))
		return nil
	}
	return urls
}

// ListWithContext 按 key 前缀列出文件 URL，ctx 取消语义贯穿到 storage 调用。
func (s *FileStore) ListWithContext(ctx context.Context, prefix string) ([]string, error) {
	ctx2, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	return s.storage.List(ctx2, prefix)
}

// ValidateImage 校验图片文件格式与大小。
func (s *FileStore) ValidateImage(filename string, size int64) (bool, string) {
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

// Read 按 URL 读回文件内容（本地/对象存储统一），返回内容与从扩展名推断的 MIME 类型。
func (s *FileStore) Read(fileURL string) ([]byte, string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	reader, err := s.storage.Get(ctx, fileURL)
	if err != nil {
		return nil, "", err
	}
	defer reader.Close()
	content, err := io.ReadAll(reader)
	if err != nil {
		return nil, "", err
	}
	return content, mimeTypeFromExt(fileExtension(fileURL)), nil
}

// ===== 章节文件校验（package-private：仅导师文件上传路径使用）=====

func fileContentType(filename string) string {
	ext := fileExtension(filename)
	for contentType, exts := range allowedExtensions {
		if exts[ext] {
			return contentType
		}
	}
	return ""
}

func allowedFile(filename string) bool {
	return fileContentType(filename) != ""
}

func validateFileSize(size int64, filename string) bool {
	contentType := fileContentType(filename)
	maxSize := maxFileSizes["default"]
	if m, ok := maxFileSizes[contentType]; ok {
		maxSize = m
	}
	return size <= maxSize
}

// ===== 图片压缩 implementation =====

// compressImageViaSidecar 调用 sidecar 的 /convert-image 接口将图片转为 WebP。
// 返回 (webpBytes, ok)；sidecar 未配置、转换失败或返回原始数据时 ok=false。
// 调用方应在 ok=false 时保留原始字节上传。
func (s *FileStore) compressImageViaSidecar(content []byte) ([]byte, bool) {
	if s.libreofficeSidecarURL == "" {
		return nil, false
	}
	if len(content) == 0 {
		return nil, false
	}

	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	part, err := writer.CreateFormFile("file", "image.bin")
	if err != nil {
		s.logger.Warn("[file_store] 构造图片压缩 multipart 失败", zap.Error(err))
		return nil, false
	}
	if _, err := part.Write(content); err != nil {
		s.logger.Warn("[file_store] 写入图片压缩 multipart 失败", zap.Error(err))
		return nil, false
	}
	_ = writer.Close()

	url := strings.TrimSuffix(s.libreofficeSidecarURL, "/") + "/convert-image"
	req, err := http.NewRequestWithContext(context.Background(), "POST", url, body)
	if err != nil {
		s.logger.Warn("[file_store] 构造图片压缩请求失败", zap.Error(err))
		return nil, false
	}
	req.Header.Set("Content-Type", writer.FormDataContentType())

	resp, err := s.httpClient.Do(req)
	if err != nil {
		s.logger.Warn("[file_store] 调用 sidecar 图片压缩失败", zap.Error(err))
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
		s.logger.Warn("[file_store] 解析图片压缩响应失败", zap.Error(err))
		return nil, false
	}
	if !result.Success {
		s.logger.Warn("[file_store] sidecar 图片压缩失败", zap.String("detail", result.Error))
		return nil, false
	}

	imgData, err := base64Decode(result.Data)
	if err != nil {
		s.logger.Warn("[file_store] sidecar 图片压缩响应 base64 解码失败", zap.Error(err))
		return nil, false
	}
	if result.Status == "WEBP" {
		return imgData, true
	}
	return nil, false
}

// ===== 辅助函数 =====

func shouldCompressImage(contentType string) bool {
	switch contentType {
	case "image/jpeg", "image/jpg", "image/png", "image/bmp":
		return true
	case "image/webp", "image/gif", "image/svg+xml":
		return false
	}
	return false
}

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

func base64Encode(data []byte) string {
	return base64.StdEncoding.EncodeToString(data)
}

func base64Decode(s string) ([]byte, error) {
	return base64.StdEncoding.DecodeString(s)
}
