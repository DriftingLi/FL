// Package tutorfile 导师个人文件库（#2494）：分片断点续传上传四段契约（init/chunk/complete/abort）
// 与文件行实体的落库、列表、删除。
//
// 分片约束来自 spec A2：单片 5–8 MiB（下限 = S3/R2 最小片语义，上限 = 服务端 60s 读窗）、
// 除末片外各片等大（= init 声明的 chunkSize）、分片总数 ≤10,000、单文件 ≤2GB（A4）。
// 重复分片幂等覆盖（A8，S3 UploadPart 语义）；abort 后 complete/补片一律拒绝。
// 存储命名沿 `<subfolder>/<name>_<毫秒时间戳>.<ext>` 契约（filestore.UniqueFileName 单一写入点）。
package tutorfile

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"gorm.io/gorm"

	"forklift-training/internal/filestore"
	"forklift-training/internal/model"
	"forklift-training/internal/storage"
)

const (
	// MaxFileSize 单文件上限 2GB（spec A4，专为录课长视频；独立于单发上传 maxFileSizes 档位）。
	MaxFileSize = 2 << 30
	// MinChunkSize / MaxChunkSize 单片档位（spec A2）。
	MinChunkSize = 5 << 20
	MaxChunkSize = 8 << 20
	// MaxChunks 分片总数上限（spec A2，与 R2/S3 同值）。
	MaxChunks = 10000

	statusActive    = "active"
	statusCompleted = "completed"
	statusAborted   = "aborted"

	// Subfolder 存储 key 分区前缀。
	Subfolder = "tutor-files"
)

var (
	// ErrNotFound 会话或文件行不存在。
	ErrNotFound = errors.New("上传会话或文件不存在")
	// ErrForbidden 资源归属他人。
	ErrForbidden = errors.New("无权操作该资源")
	// ErrInvalidState 会话状态不允许该操作（如 abort 后 complete）。
	ErrInvalidState = errors.New("上传会话状态不允许该操作")
	// ErrInvalidParam 参数越界（大小/类型/分片档/序号/长度）。
	ErrInvalidParam = errors.New("上传参数不合法")
)

// Service 导师文件库服务。
// 依赖 storage.Storage（与全域同型）；分片四段走 multipart 能力（UploadStore），
// 驱动不支持时 Init/Chunk/Complete/Abort 调用面报错，List/Delete 仍按普通存储走。
type Service struct {
	db *gorm.DB
	st storage.Storage
	mp storage.UploadStore
}

// NewService 创建文件库服务。st 为 nil 时仅当调用面走到存储操作才报错
// （契约测试装配允许 nil 存储，与 FileSvc 同构）。
func NewService(db *gorm.DB, st storage.Storage) *Service {
	s := &Service{db: db, st: st}
	s.mp, _ = st.(storage.UploadStore)
	return s
}

// needStore 普通存储操作前置守卫。
func (s *Service) needStore() error {
	if s.st == nil {
		return fmt.Errorf("%w: 存储未配置", ErrInvalidState)
	}
	return nil
}

// needMultipart 分片上传操作前置守卫。
func (s *Service) needMultipart() error {
	if err := s.needStore(); err != nil {
		return err
	}
	if s.mp == nil {
		return fmt.Errorf("%w: 存储驱动不支持分片上传", ErrInvalidState)
	}
	return nil
}

// InitUpload 创建上传会话（spec A1）。校验单文件 ≤2GB、类型白名单四档、分片档 5–8MiB、片数 ≤10,000。
func (s *Service) InitUpload(ctx context.Context, tutorID int, filename string, fileSize, chunkSize int64) (string, error) {
	if err := s.needMultipart(); err != nil {
		return "", err
	}
	if tutorID <= 0 {
		return "", fmt.Errorf("%w: 归属导师无效", ErrInvalidParam)
	}
	if fileSize <= 0 || fileSize > MaxFileSize {
		return "", fmt.Errorf("%w: 文件大小须在 1B–2GB 之间", ErrInvalidParam)
	}
	ext := filestore.FileExtension(filename)
	category := filestore.UploadCategoryOf(ext)
	if category == "" {
		return "", fmt.Errorf("%w: 不支持的文件类型 %q", ErrInvalidParam, ext)
	}
	if chunkSize < MinChunkSize || chunkSize > MaxChunkSize {
		return "", fmt.Errorf("%w: 分片大小须在 5–8MiB 之间", ErrInvalidParam)
	}
	total := totalChunksFor(fileSize, chunkSize)
	if total > MaxChunks {
		return "", fmt.Errorf("%w: 分片总数超过 %d", ErrInvalidParam, MaxChunks)
	}

	sessionID, err := newSessionID()
	if err != nil {
		return "", err
	}
	name := strings.TrimSuffix(filename, filepath.Ext(filename))
	storageKey := fmt.Sprintf("%s/%d/%s", Subfolder, tutorID, filestore.UniqueFileName(name, filepath.Ext(filename)))
	contentType := filestore.MimeTypeOf(ext)
	uploadID, err := s.mp.InitMultipart(ctx, storageKey, contentType)
	if err != nil {
		return "", fmt.Errorf("存储会话创建失败: %w", err)
	}

	row := &model.TutorUploadSession{
		SessionID:   sessionID,
		TutorID:     tutorID,
		FileName:    filename,
		FileSize:    fileSize,
		ChunkSize:   chunkSize,
		TotalChunks: total,
		ContentType: contentType,
		StorageKey:  storageKey,
		UploadID:    uploadID,
		Status:      statusActive,
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}
	if err := s.db.WithContext(ctx).Create(row).Error; err != nil {
		_ = s.mp.AbortMultipart(ctx, storageKey, uploadID)
		return "", fmt.Errorf("保存上传会话失败: %w", err)
	}
	return sessionID, nil
}

// UploadChunk 上传第 partNumber 片（spec A1/A8）。同序号重传幂等覆盖；
// 除末片外各片必须恰为 chunkSize，末片必须恰为剩余长度。
func (s *Service) UploadChunk(ctx context.Context, tutorID int, sessionID string, partNumber int, content []byte) error {
	if err := s.needMultipart(); err != nil {
		return err
	}
	row, err := s.loadSession(ctx, tutorID, sessionID)
	if err != nil {
		return err
	}
	if row.Status != statusActive {
		return fmt.Errorf("%w: 会话已 %s", ErrInvalidState, row.Status)
	}
	if partNumber < 1 || partNumber > row.TotalChunks {
		return fmt.Errorf("%w: 分片序号 %d 超出 1–%d", ErrInvalidParam, partNumber, row.TotalChunks)
	}
	expected := row.ChunkSize
	if partNumber == row.TotalChunks {
		expected = row.FileSize - row.ChunkSize*int64(row.TotalChunks-1)
	}
	if int64(len(content)) != expected {
		return fmt.Errorf("%w: 分片 %d 应为 %d 字节，收到 %d", ErrInvalidParam, partNumber, expected, len(content))
	}

	etag, err := s.mp.UploadPart(ctx, row.StorageKey, row.UploadID, partNumber, content)
	if err != nil {
		return fmt.Errorf("存储分片失败: %w", err)
	}
	part := &model.TutorUploadPart{
		SessionID:  sessionID,
		PartNumber: partNumber,
		ETag:       etag,
		Size:       int64(len(content)),
		CreatedAt:  time.Now(),
	}
	if err := s.db.WithContext(ctx).Save(part).Error; err != nil {
		return fmt.Errorf("保存分片凭据失败: %w", err)
	}
	return nil
}

// CompleteUpload 拼装落存储并产出文件行（spec A1/A5）。缺片、abort/已完成的会话一律拒绝。
func (s *Service) CompleteUpload(ctx context.Context, tutorID int, sessionID string) (*model.TutorFile, error) {
	if err := s.needMultipart(); err != nil {
		return nil, err
	}
	row, err := s.loadSession(ctx, tutorID, sessionID)
	if err != nil {
		return nil, err
	}
	if row.Status != statusActive {
		return nil, fmt.Errorf("%w: 会话已 %s", ErrInvalidState, row.Status)
	}

	var parts []model.TutorUploadPart
	if err := s.db.WithContext(ctx).
		Where("session_id = ?", sessionID).
		Order("part_number ASC").
		Find(&parts).Error; err != nil {
		return nil, fmt.Errorf("读取分片凭据失败: %w", err)
	}
	if len(parts) != row.TotalChunks {
		return nil, fmt.Errorf("%w: 分片不全（已收 %d/%d），请补齐后再完成", ErrInvalidParam, len(parts), row.TotalChunks)
	}
	storageParts := make([]storage.Part, 0, len(parts))
	for i, p := range parts {
		if p.PartNumber != i+1 {
			return nil, fmt.Errorf("%w: 分片序号不连续（缺 %d）", ErrInvalidParam, i+1)
		}
		storageParts = append(storageParts, storage.Part{Number: p.PartNumber, ETag: p.ETag})
	}

	url, err := s.mp.CompleteMultipart(ctx, row.StorageKey, row.UploadID, storageParts)
	if err != nil {
		return nil, fmt.Errorf("存储拼装失败: %w", err)
	}

	file := &model.TutorFile{
		TutorID:     row.TutorID,
		FileURL:     url,
		FileName:    row.FileName,
		ContentType: filestore.UploadCategoryOf(filestore.FileExtension(row.FileName)),
		FileSize:    row.FileSize,
		CreatedAt:   time.Now(),
	}
	err = s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(file).Error; err != nil {
			return fmt.Errorf("保存文件行失败: %w", err)
		}
		return tx.Model(&model.TutorUploadSession{}).
			Where("session_id = ?", sessionID).
			Updates(map[string]any{"status": statusCompleted, "updated_at": time.Now()}).Error
	})
	if err != nil {
		return nil, err
	}
	return file, nil
}

// AbortUpload 放弃会话并清理半成品（spec A1）。已 abort 的重复调用幂等。
func (s *Service) AbortUpload(ctx context.Context, tutorID int, sessionID string) error {
	if err := s.needMultipart(); err != nil {
		return err
	}
	row, err := s.loadSession(ctx, tutorID, sessionID)
	if err != nil {
		return err
	}
	if row.Status == statusAborted {
		return nil
	}
	if row.Status == statusCompleted {
		return fmt.Errorf("%w: 会话已完成，不能放弃", ErrInvalidState)
	}
	if err := s.mp.AbortMultipart(ctx, row.StorageKey, row.UploadID); err != nil {
		return err
	}
	return s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Model(&model.TutorUploadSession{}).
			Where("session_id = ?", sessionID).
			Updates(map[string]any{"status": statusAborted, "updated_at": time.Now()}).Error; err != nil {
			return err
		}
		return tx.Where("session_id = ?", sessionID).Delete(&model.TutorUploadPart{}).Error
	})
}

// ListFiles 列出导师自己的文件库行（新→旧）。
func (s *Service) ListFiles(ctx context.Context, tutorID int) ([]model.TutorFile, error) {
	var files []model.TutorFile
	if err := s.db.WithContext(ctx).
		Where("tutor_id = ?", tutorID).
		Order("file_id DESC").
		Find(&files).Error; err != nil {
		return nil, fmt.Errorf("读取文件库失败: %w", err)
	}
	return files, nil
}

// DeleteFile 删除文件行与存储对象（仅归属导师）。
func (s *Service) DeleteFile(ctx context.Context, tutorID, fileID int) error {
	if err := s.needStore(); err != nil {
		return err
	}
	var file model.TutorFile
	if err := s.db.WithContext(ctx).First(&file, "file_id = ?", fileID).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ErrNotFound
		}
		return fmt.Errorf("读取文件行失败: %w", err)
	}
	if file.TutorID != tutorID {
		return ErrForbidden
	}
	if err := s.st.Delete(ctx, file.FileURL); err != nil {
		return fmt.Errorf("删除存储对象失败: %w", err)
	}
	if err := s.db.WithContext(ctx).Delete(&model.TutorFile{}, "file_id = ?", fileID).Error; err != nil {
		return fmt.Errorf("删除文件行失败: %w", err)
	}
	return nil
}

// totalChunksFor 按「除末片等大、末片补余」口径计算分片总数（服务与 handler 共用单点）。
func totalChunksFor(fileSize, chunkSize int64) int {
	return int((fileSize + chunkSize - 1) / chunkSize)
}

// loadSession 载入会话并做归属校验。
func (s *Service) loadSession(ctx context.Context, tutorID int, sessionID string) (*model.TutorUploadSession, error) {
	var row model.TutorUploadSession
	if err := s.db.WithContext(ctx).First(&row, "session_id = ?", sessionID).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("读取上传会话失败: %w", err)
	}
	if row.TutorID != tutorID {
		return nil, ErrForbidden
	}
	return &row, nil
}

// newSessionID 生成随机会话 ID（十六进制 32 字符）。
func newSessionID() (string, error) {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("生成会话 ID 失败: %w", err)
	}
	return hex.EncodeToString(b), nil
}
