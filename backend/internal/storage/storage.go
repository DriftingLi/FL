// Package storage 提供文件存储抽象层，支持本地磁盘与 Cloudflare R2（S3 兼容）。
//
// 设计目标：将原 FileService 中直接 os.WriteFile 的存储行为抽象为接口，
// 便于在本地磁盘（开发/回退）与 R2 对象存储（生产）之间切换。
// 切换由环境变量 STORAGE_DRIVER 控制（local / r2）。
package storage

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

// Storage 文件存储接口。
//
// 约定：
//   - key 为相对路径，如 "chapters/foo_1234567890.pdf" 或 "slides/12/slide_001.png"
//   - Save 返回的 url 在 local 模式下为 "/static/uploads/<key>"，
//     在 r2 模式下为 "https://<publicDomain>/<key>"
//   - Delete 与 Exists 接收完整 url（即 Save 返回值），内部解析回 key
type Storage interface {
	// Save 上传文件内容并返回可访问的 URL。
	Save(ctx context.Context, key string, content []byte, contentType string) (url string, err error)
	// Delete 按 URL 删除文件，URL 为空时直接返回 nil。
	Delete(ctx context.Context, url string) error
	// Exists 检查 URL 对应的文件是否存在。
	Exists(ctx context.Context, url string) (bool, error)
	// List 按 key 前缀列出所有文件，返回可访问的 URL 列表（与 Save 返回格式一致）。
	// 前缀为空时列出全部文件；prefix 与 key 分隔符（/）对齐，如 "images/forum" 匹配 "images/forum/xxx.webp"。
	List(ctx context.Context, prefix string) ([]string, error)
	// Get 按 URL 读取文件内容（流式），调用方负责关闭返回的 ReadCloser。
	// 用于代理下载等需要后端中转内容的场景（绕开对象存储的浏览器跨域限制）。
	Get(ctx context.Context, url string) (io.ReadCloser, error)
}

// LocalStorage 本地磁盘存储（本地开发与回退模式）。
//
// 行为与原 FileService 直接 os.WriteFile 一致：
// 文件写入 baseDir/<key>，URL 返回 "/static/uploads/<key>"。
type LocalStorage struct {
	baseDir string
}

// NewLocalStorage 创建本地存储实例。
// baseDir 通常为 cfg.UploadFolder（如 "static/uploads" 或 "/data/uploads"）。
func NewLocalStorage(baseDir string) *LocalStorage {
	return &LocalStorage{baseDir: baseDir}
}

// Save 写入本地磁盘，返回 /static/uploads/<key> 形式的相对 URL。
func (s *LocalStorage) Save(ctx context.Context, key string, content []byte, contentType string) (string, error) {
	fullPath := filepath.Join(s.baseDir, key)
	if err := os.MkdirAll(filepath.Dir(fullPath), 0o755); err != nil {
		return "", fmt.Errorf("创建目录失败: %w", err)
	}
	if err := os.WriteFile(fullPath, content, 0o644); err != nil {
		return "", fmt.Errorf("写入文件失败: %w", err)
	}
	return "/static/uploads/" + key, nil
}

// Delete 从 URL 解析出 key 后删除本地文件，文件不存在不报错。
func (s *LocalStorage) Delete(ctx context.Context, url string) error {
	if url == "" {
		return nil
	}
	key := localURLToKey(url)
	fullPath := filepath.Join(s.baseDir, key)
	if err := os.Remove(fullPath); err != nil && !os.IsNotExist(err) {
		return err
	}
	return nil
}

// Exists 检查本地文件是否存在且非目录。
func (s *LocalStorage) Exists(ctx context.Context, url string) (bool, error) {
	if url == "" {
		return false, nil
	}
	key := localURLToKey(url)
	fullPath := filepath.Join(s.baseDir, key)
	info, err := os.Stat(fullPath)
	if err != nil {
		if os.IsNotExist(err) {
			return false, nil
		}
		return false, err
	}
	return !info.IsDir(), nil
}

// localURLToKey 从 /static/uploads/<key> 形式的 URL 中提取 key。
// 兼容直接传入 key（无前缀）的情况。
func localURLToKey(url string) string {
	return strings.TrimPrefix(url, "/static/uploads/")
}

// Get 打开本地文件流式读取。
func (s *LocalStorage) Get(ctx context.Context, url string) (io.ReadCloser, error) {
	if url == "" {
		return nil, fmt.Errorf("url 为空")
	}
	key := localURLToKey(url)
	return os.Open(filepath.Join(s.baseDir, key))
}

// List 按 key 前缀列出本地文件，返回 /static/uploads/<key> 形式的 URL 列表。
// 前缀为空时列出 baseDir 下全部文件；返回的 URL 与 Save 返回格式一致。
func (s *LocalStorage) List(ctx context.Context, prefix string) ([]string, error) {
	root := s.baseDir
	relPrefix := strings.Trim(prefix, "/")
	if relPrefix != "" {
		root = filepath.Join(s.baseDir, relPrefix)
	}
	if _, err := os.Stat(root); err != nil {
		if os.IsNotExist(err) {
			return []string{}, nil
		}
		return nil, err
	}
	var urls []string
	err := filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		rel, err := filepath.Rel(s.baseDir, path)
		if err != nil {
			return err
		}
		urls = append(urls, "/static/uploads/"+filepath.ToSlash(rel))
		return nil
	})
	if err != nil {
		return nil, err
	}
	return urls, nil
}
