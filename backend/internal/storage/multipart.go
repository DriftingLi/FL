// 本文件：分片上传 seam（#2494）——init/UploadPart/Complete/Abort 四段契约的抽象与本地模拟。
//
// 两环境语义一致由接口收敛：
//   - 生产 R2Storage 走 S3 原生 multipart（r2_multipart.go）；
//   - LocalStorage 以暂存目录模拟（.multipart/<uploadID>/<n>.part），
//     Complete 按分片序号合并到目标 key，暂存对 List 面不可见（悬空回收/列表不误伤残片）。
//
// 分片尺寸、片数、除末片等大等业务约束在 tutorfile 服务层校验；本 seam 只管搬运与合并。
package storage

import (
	"context"
	"crypto/md5"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strconv"
)

// Part 一个已上传分片的凭据（序号 + 存储侧 etag），Complete 时按 Number 升序合并。
type Part struct {
	Number int
	ETag   string
}

// MultipartStorage 分片上传能力。LocalStorage 与 R2Storage 均实现。
type MultipartStorage interface {
	// InitMultipart 开启一次分片上传会话，返回会话标识 uploadID。
	InitMultipart(ctx context.Context, key, contentType string) (uploadID string, err error)
	// UploadPart 上传第 partNumber 片（1 起）；重复上传同一序号幂等覆盖，返回该片 etag。
	UploadPart(ctx context.Context, key, uploadID string, partNumber int, content []byte) (etag string, err error)
	// CompleteMultipart 按 parts 的 Number 升序合并为最终对象，返回可访问 URL（与 Save 返回格式一致）。
	CompleteMultipart(ctx context.Context, key, uploadID string, parts []Part) (url string, err error)
	// AbortMultipart 中止会话并清理暂存；重复 abort 幂等。
	AbortMultipart(ctx context.Context, key, uploadID string) error
}

// UploadStore 聚合完整存储能力（普通读写 + 分片上传），供 tutorfile 等上传域依赖。
type UploadStore interface {
	Storage
	MultipartStorage
}

// stagingDirName 暂存目录名（位于 baseDir 下，ListWithInfo 整树过滤，残片不对列表面可见）。
const stagingDirName = ".multipart"

func newUploadID() (string, error) {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "", fmt.Errorf("生成上传会话标识失败: %w", err)
	}
	return hex.EncodeToString(b[:]), nil
}

// ---- LocalStorage 实现 ----

func (s *LocalStorage) stagingPath(uploadID string) string {
	return filepath.Join(s.baseDir, stagingDirName, uploadID)
}

// InitMultipart 建暂存目录，返回随机 uploadID。
func (s *LocalStorage) InitMultipart(ctx context.Context, key, contentType string) (string, error) {
	uploadID, err := newUploadID()
	if err != nil {
		return "", err
	}
	if err := os.MkdirAll(s.stagingPath(uploadID), 0o755); err != nil {
		return "", fmt.Errorf("创建分片暂存目录失败: %w", err)
	}
	return uploadID, nil
}

// UploadPart 把分片落到暂存目录 <n>.part；同序号重复写覆盖（幂等）。
func (s *LocalStorage) UploadPart(ctx context.Context, key, uploadID string, partNumber int, content []byte) (string, error) {
	if partNumber < 1 {
		return "", fmt.Errorf("分片序号无效: %d", partNumber)
	}
	dir := s.stagingPath(uploadID)
	if info, err := os.Stat(dir); err != nil || !info.IsDir() {
		return "", fmt.Errorf("上传会话不存在或已中止")
	}
	partPath := filepath.Join(dir, strconv.Itoa(partNumber)+".part")
	if err := os.WriteFile(partPath, content, 0o644); err != nil {
		return "", fmt.Errorf("写入分片失败: %w", err)
	}
	return contentETag(content), nil
}

// CompleteMultipart 按分片序号升序合并到目标 key，清理暂存并返回 URL。
func (s *LocalStorage) CompleteMultipart(ctx context.Context, key, uploadID string, parts []Part) (string, error) {
	dir := s.stagingPath(uploadID)
	if info, err := os.Stat(dir); err != nil || !info.IsDir() {
		return "", fmt.Errorf("上传会话不存在或已中止")
	}
	if len(parts) == 0 {
		return "", fmt.Errorf("分片清单为空")
	}
	nums := make([]int, 0, len(parts))
	for _, p := range parts {
		nums = append(nums, p.Number)
	}
	sort.Ints(nums)

	fullPath := filepath.Join(s.baseDir, filepath.FromSlash(key))
	if err := os.MkdirAll(filepath.Dir(fullPath), 0o755); err != nil {
		return "", fmt.Errorf("创建目录失败: %w", err)
	}
	out, err := os.Create(fullPath)
	if err != nil {
		return "", fmt.Errorf("创建目标文件失败: %w", err)
	}
	for _, n := range nums {
		f, err := os.Open(filepath.Join(dir, strconv.Itoa(n)+".part"))
		if err != nil {
			out.Close()
			os.Remove(fullPath)
			return "", fmt.Errorf("分片 %d 缺失: %w", n, err)
		}
		_, err = io.Copy(out, f)
		f.Close()
		if err != nil {
			out.Close()
			os.Remove(fullPath)
			return "", fmt.Errorf("合并分片 %d 失败: %w", n, err)
		}
	}
	if err := out.Close(); err != nil {
		os.Remove(fullPath)
		return "", fmt.Errorf("关闭目标文件失败: %w", err)
	}
	_ = os.RemoveAll(dir)
	return "/static/uploads/" + key, nil
}

// AbortMultipart 清理暂存目录；会话不存在时幂等返回 nil。
func (s *LocalStorage) AbortMultipart(ctx context.Context, key, uploadID string) error {
	if err := os.RemoveAll(s.stagingPath(uploadID)); err != nil {
		return fmt.Errorf("清理分片暂存失败: %w", err)
	}
	return nil
}

// contentETag 本地模拟 etag：内容 MD5 十六进制（与 S3 单段 etag 语义同形）。
func contentETag(content []byte) string {
	sum := md5.Sum(content)
	return hex.EncodeToString(sum[:])
}
