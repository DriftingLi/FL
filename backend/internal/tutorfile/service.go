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

	"go.uber.org/zap"
	"gorm.io/gorm"

	"forklift-training/internal/course"
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

	// MaxFilesPerTutor / MaxBytesPerTutor 配额双臂（spec B3）：≤300 个 / ≤100GB 每导师，
	// 各自独立拦截（数字是软值，调前须重估——#2486 grilling 决策）。
	MaxFilesPerTutor = 300
	MaxBytesPerTutor = 100 << 30

	// SessionTTL 未完成会话的存活期（spec A3）：超过 7 天自动 abort（R2 lifecycle 同口径）。
	SessionTTL = 7 * 24 * time.Hour
	// OrphanTTL 真孤儿（对象在、文件行没有）的回收门槛（spec B4）：沿 orphan_sweep 24h 既有口径。
	OrphanTTL = 24 * time.Hour

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
	// ErrQuotaFileCount 配额数量臂（spec B3，400 沿投稿配额先例）。
	ErrQuotaFileCount = errors.New("文件数量已达上限")
	// ErrQuotaTotalSize 配额容量臂（spec B3，400 沿投稿配额先例）。
	ErrQuotaTotalSize = errors.New("文件总容量已达上限")
	// ErrFileInUse 文件被章节引用，库侧不可删（spec B5 删除随引用走）。
	ErrFileInUse = errors.New("文件已被章节引用")
)

// FileFilter 管理面筛选（spec B7）：类型 / 挂接状态 / 创建时间区间。
// 零值 = 不过滤；区间闭区间（含端点）。
type FileFilter struct {
	ContentType string
	Attached    *bool
	CreatedFrom time.Time
	CreatedTo   time.Time
}

// Service 导师文件库服务。
// 依赖 storage.Storage（与全域同型）；分片四段走 multipart 能力（UploadStore），
// 驱动不支持时 Init/Chunk/Complete/Abort 调用面报错，List/Delete 仍按普通存储走。
type Service struct {
	db     *gorm.DB
	st     storage.Storage
	mp     storage.UploadStore
	logger *zap.Logger
}

// NewService 创建文件库服务。st 为 nil 时仅当调用面走到存储操作才报错
// （契约测试装配允许 nil 存储，与 FileSvc 同构）；logger 可为 nil（尽力而为日志）。
func NewService(db *gorm.DB, st storage.Storage, logger *zap.Logger) *Service {
	s := &Service{db: db, st: st, logger: logger}
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
	total, err := chunkPlan(fileSize, chunkSize)
	if err != nil {
		return "", err
	}
	if err := s.checkQuota(ctx, tutorID, fileSize); err != nil {
		return "", err
	}
	ext := filestore.FileExtension(filename)
	category := filestore.UploadCategoryOf(ext)
	if category == "" {
		return "", fmt.Errorf("%w: 不支持的文件类型 %q", ErrInvalidParam, ext)
	}

	sessionID, err := newSessionID()
	if err != nil {
		return "", err
	}
	name := strings.TrimSuffix(filename, filepath.Ext(filename))
	// 命名契约 `<name>_<毫秒时间戳>.<ext>`（filestore.UniqueFileName）：multipart 的 key 在
	// init 即绑定（S3 CreateMultipartUpload 语义），故时间戳取会话创建时刻而非 complete 时刻；
	// 契约的语义是「唯一命名」，格式不变。
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
	return s.abortSessionRow(ctx, row)
}

// abortSessionRow 放弃一个 active 会话的共用落点（显式 abort 与 7 天过期清扫同用）：
// 清存储半成品 + 行落 aborted + 删分片凭据。
func (s *Service) abortSessionRow(ctx context.Context, row *model.TutorUploadSession) error {
	if err := s.mp.AbortMultipart(ctx, row.StorageKey, row.UploadID); err != nil {
		return err
	}
	return s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Model(&model.TutorUploadSession{}).
			Where("session_id = ?", row.SessionID).
			Updates(map[string]any{"status": statusAborted, "updated_at": time.Now()}).Error; err != nil {
			return err
		}
		return tx.Where("session_id = ?", row.SessionID).Delete(&model.TutorUploadPart{}).Error
	})
}

// GetUploadSession 断点续传查询口（spec A8，= S3 ListParts 语义）：
// 返回会话行与已收分片序号清单，客户端据此只补缺片；服务端以会话状态为准。
func (s *Service) GetUploadSession(ctx context.Context, tutorID int, sessionID string) (*model.TutorUploadSession, []int, error) {
	row, err := s.loadSession(ctx, tutorID, sessionID)
	if err != nil {
		return nil, nil, err
	}
	var parts []model.TutorUploadPart
	if err := s.db.WithContext(ctx).
		Where("session_id = ?", sessionID).
		Order("part_number ASC").
		Find(&parts).Error; err != nil {
		return nil, nil, fmt.Errorf("读取分片凭据失败: %w", err)
	}
	nums := make([]int, 0, len(parts))
	for _, p := range parts {
		nums = append(nums, p.PartNumber)
	}
	return row, nums, nil
}

// SweepStaleSessions 会话 7 天自动 abort（spec A3）：updated_at 早于 now-SessionTTL 的
// active 会话全部放弃（存储半成品 + 行态 + 分片凭据）。返回清理数；now 由调用方注入（可测）。
func (s *Service) SweepStaleSessions(ctx context.Context, now time.Time) (int, error) {
	if err := s.needMultipart(); err != nil {
		return 0, err
	}
	var sessions []model.TutorUploadSession
	if err := s.db.WithContext(ctx).
		Where("status = ? AND updated_at < ?", statusActive, now.Add(-SessionTTL)).
		Find(&sessions).Error; err != nil {
		return 0, fmt.Errorf("查询过期会话失败: %w", err)
	}
	cleaned := 0
	for i := range sessions {
		if err := s.abortSessionRow(ctx, &sessions[i]); err != nil {
			if s.logger != nil {
				s.logger.Warn("[tutorfile] 过期会话清理失败", zap.String("session_id", sessions[i].SessionID), zap.Error(err))
			}
			continue
		}
		cleaned++
	}
	return cleaned, nil
}

// CleanupOrphanFiles 真孤儿回收（spec B4，薄配置壳，算法单点见 internal/filestore/orphan_sweep.go /
// ADR-0027 C2）：ListWithInfo(tutor-files/) 与全量引用集（tutor_file.file_url）差集，
// 仅删存储侧 LastModified 超过 OrphanTTL 且未被任何文件行引用的对象。
// 「库内有行文件永不静默删」由差集天然保证：引用集里有它的 key 就不动。
// 引用集查不动或为空时整轮不清理（ADR-0062 票5）；ctx 取消语义贯穿。
func (s *Service) CleanupOrphanFiles(ctx context.Context) int {
	if s.st == nil {
		return 0
	}
	return filestore.RunOrphanSweep(ctx, filestore.OrphanSweepConfig{
		Domain: "tutor_file",
		TTL:    OrphanTTL,
		List: func(c context.Context) ([]storage.FileInfo, error) {
			return s.st.ListWithInfo(c, Subfolder)
		},
		Referenced: s.collectReferencedKeys,
		KeyOf:      storageKeyOf,
		DeleteFile: s.st.Delete,
		Logger:     s.logger,
	})
}

// collectReferencedKeys 全量引用集：tutor_file 行的 file_url → 存储 key（与 KeyOf 同归一化口径）。
// 任一非空 URL 提取不出 key 就整体报错——「不知道谁在引用」不得当成「没人引用」
// （orphan_sweep 引用集 fail-safe 同构，B4「有行永不静默删」的最后一道闸）。
func (s *Service) collectReferencedKeys() (map[string]bool, error) {
	var urls []string
	if err := s.db.Model(&model.TutorFile{}).Pluck("file_url", &urls).Error; err != nil {
		return nil, fmt.Errorf("读取文件行引用集失败: %w", err)
	}
	refs := make(map[string]bool, len(urls))
	for _, u := range urls {
		key := storageKeyOf(u)
		if key == "" && u != "" {
			return nil, fmt.Errorf("文件行 URL 提取不出存储 key（格式漂移?）: %q", u)
		}
		if key != "" {
			refs[key] = true
		}
	}
	return refs, nil
}

// storageKeyOf URL → 存储对象 key：先剥 query/fragment（签名串不属 key），
// 再走 filestore.AttachmentKey 的前缀收窄（local/R2 两形态同口径，提取失败返回空串）。
func storageKeyOf(url string) string {
	if i := strings.IndexAny(url, "?#"); i >= 0 {
		url = url[:i]
	}
	return filestore.AttachmentKey(url, Subfolder)
}

// checkQuota 配额双臂（spec B3）：数量与容量各自独立判定、拒绝信息含当前用量。
// 只在 init 闸（最早拒、最好体验）；并发会话竞态在软值口径内可接受（#2486 grilling）。
func (s *Service) checkQuota(ctx context.Context, tutorID int, fileSize int64) error {
	var count int64
	if err := s.db.WithContext(ctx).Model(&model.TutorFile{}).
		Where("tutor_id = ?", tutorID).Count(&count).Error; err != nil {
		return fmt.Errorf("查询文件数量失败: %w", err)
	}
	if count+1 > int64(MaxFilesPerTutor) {
		return fmt.Errorf("%w: 文件数量已达上限（%d/%d 个），请先清理后再上传", ErrQuotaFileCount, count, MaxFilesPerTutor)
	}
	var used int64
	if err := s.db.WithContext(ctx).Model(&model.TutorFile{}).
		Where("tutor_id = ?", tutorID).
		Select("COALESCE(SUM(file_size), 0)").Scan(&used).Error; err != nil {
		return fmt.Errorf("查询文件总容量失败: %w", err)
	}
	if used+fileSize > MaxBytesPerTutor {
		return fmt.Errorf("%w: 文件总容量已达上限（%.1f/%d GB），请先清理后再上传",
			ErrQuotaTotalSize, float64(used)/float64(1<<30), MaxBytesPerTutor>>30)
	}
	return nil
}

// ListFiles 列出导师自己的文件库行（新→旧）。filter 零值 = 不过滤（spec B7 管理面筛选）。
func (s *Service) ListFiles(ctx context.Context, tutorID int, filter FileFilter) ([]model.TutorFile, error) {
	q := s.db.WithContext(ctx).Where("tutor_id = ?", tutorID)
	if filter.ContentType != "" {
		q = q.Where("content_type = ?", filter.ContentType)
	}
	if filter.Attached != nil {
		if *filter.Attached {
			q = q.Where("chapter_id IS NOT NULL")
		} else {
			q = q.Where("chapter_id IS NULL")
		}
	}
	if !filter.CreatedFrom.IsZero() {
		q = q.Where("created_at >= ?", filter.CreatedFrom)
	}
	if !filter.CreatedTo.IsZero() {
		q = q.Where("created_at <= ?", filter.CreatedTo)
	}
	var files []model.TutorFile
	if err := q.Order("file_id DESC").Find(&files).Error; err != nil {
		return nil, fmt.Errorf("读取文件库失败: %w", err)
	}
	return files, nil
}

// AttachFile 单挂挂接（spec B2）：文件行带章节归属，空=在库未挂。单挂互斥——已挂其他章
// 拒绝并明示（先解挂再挂）；挂到同章幂等。挂接后随该章节送审（块 C 的文件挂接清单引用它）。
func (s *Service) AttachFile(ctx context.Context, tutorID, fileID, chapterID int) error {
	file, err := s.loadFile(ctx, tutorID, fileID)
	if err != nil {
		return err
	}
	if file.ChapterID != nil {
		if *file.ChapterID == chapterID {
			return nil
		}
		return fmt.Errorf("%w: 文件已挂接到其他章节（#%d），先解挂再挂接", ErrInvalidState, *file.ChapterID)
	}
	var ch model.Chapter
	if err := s.db.WithContext(ctx).First(&ch, "chapter_id = ?", chapterID).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return course.ErrChapterNotFound
		}
		return fmt.Errorf("读取章节失败: %w", err) // 查不动不得被读成「不存在」（ADR-0064 决策 1）
	}
	if err := s.db.WithContext(ctx).Model(&model.TutorFile{}).
		Where("file_id = ?", fileID).Update("chapter_id", chapterID).Error; err != nil {
		return err
	}
	// 挂接清单是审核单元（#2498 C3）：变更把章节工作副本标回草稿（C2）
	return course.MarkChapterDraft(s.db, chapterID)
}

// DetachFile 取消挂接回库（spec B2）：chapter_id 清空回到未挂状态。重复解挂幂等。
func (s *Service) DetachFile(ctx context.Context, tutorID, fileID int) error {
	file, err := s.loadFile(ctx, tutorID, fileID)
	if err != nil {
		return err
	}
	if err := s.db.WithContext(ctx).Model(&model.TutorFile{}).
		Where("file_id = ?", fileID).Update("chapter_id", nil).Error; err != nil {
		return err
	}
	if file.ChapterID != nil {
		return course.MarkChapterDraft(s.db, *file.ChapterID)
	}
	return nil
}

// SwapChapterFile 章节侧换挂（spec B6）：挂新 + 旧回库一步原子——「换文件」是章节侧动作，
// 库侧不提供替换。旧文件必须挂在本章、新文件必须在库未挂（单挂互斥同样成立）。
func (s *Service) SwapChapterFile(ctx context.Context, tutorID, chapterID, oldFileID, newFileID int) error {
	if oldFileID == newFileID {
		return fmt.Errorf("%w: 新旧文件不能相同", ErrInvalidParam)
	}
	old, err := s.loadFile(ctx, tutorID, oldFileID)
	if err != nil {
		return err
	}
	fresh, err := s.loadFile(ctx, tutorID, newFileID)
	if err != nil {
		return err
	}
	if old.ChapterID == nil || *old.ChapterID != chapterID {
		return fmt.Errorf("%w: 旧文件未挂在该章节，无法换挂", ErrInvalidState)
	}
	if fresh.ChapterID != nil {
		return fmt.Errorf("%w: 新文件已挂接到其他章节（#%d），先解挂再换挂", ErrInvalidState, *fresh.ChapterID)
	}
	return s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Model(&model.TutorFile{}).
			Where("file_id = ?", newFileID).Update("chapter_id", chapterID).Error; err != nil {
			return err
		}
		if err := tx.Model(&model.TutorFile{}).
			Where("file_id = ?", oldFileID).Update("chapter_id", nil).Error; err != nil {
			return err
		}
		return course.MarkChapterDraft(tx, chapterID)
	})
}

// RenameFile 改名（spec B7）：只改显示名 FileName，存储 key 不动（命名契约 A5 在落库时已定）。
func (s *Service) RenameFile(ctx context.Context, tutorID, fileID int, newName string) error {
	file, err := s.loadFile(ctx, tutorID, fileID)
	if err != nil {
		return err
	}
	newName = strings.TrimSpace(newName)
	if newName == "" {
		return fmt.Errorf("%w: 文件名不能为空", ErrInvalidParam)
	}
	if newName == file.FileName {
		return nil
	}
	return s.db.WithContext(ctx).Model(&model.TutorFile{}).
		Where("file_id = ?", fileID).Update("file_name", newName).Error
}

// DeleteFile 删除文件行与存储对象（仅归属导师）。被章节引用（含在审快照、已发布槽引用的
// 视同挂接中——B5，引用面扩展由 #2498 接入）时拒绝并明示原因，绝不产生章节侧悬空引用。
func (s *Service) DeleteFile(ctx context.Context, tutorID, fileID int) error {
	if err := s.needStore(); err != nil {
		return err
	}
	file, err := s.loadFile(ctx, tutorID, fileID)
	if err != nil {
		return err
	}
	if file.ChapterID != nil {
		return fmt.Errorf("%w: 文件已挂接到章节（#%d），解挂后可删除", ErrFileInUse, *file.ChapterID)
	}
	// C13（#2498 B5 扩展）：在审快照与已发布槽引用的文件视同挂接中——即使已解挂，
	// 被冻结清单引用期间库侧不可删，否则版本槽引用会悬空。
	var slotRefs int64
	if err := s.db.WithContext(ctx).Model(&model.ChapterFileSlot{}).
		Where("source = ? AND file_id = ?", course.FileSourceTutor, fileID).
		Count(&slotRefs).Error; err != nil {
		return fmt.Errorf("查询版本槽引用失败: %w", err)
	}
	if slotRefs > 0 {
		return fmt.Errorf("%w: 文件仍被章节的在审/已发布版本引用，版本更新或解绑后可删除", ErrFileInUse)
	}
	if err := s.st.Delete(ctx, file.FileURL); err != nil {
		return fmt.Errorf("删除存储对象失败: %w", err)
	}
	if err := s.db.WithContext(ctx).Delete(&model.TutorFile{}, "file_id = ?", fileID).Error; err != nil {
		return fmt.Errorf("删除文件行失败: %w", err)
	}
	return nil
}

// ChapterTitles 为挂接中的文件行批量解析章节标题（管理面展示用，一次 IN 查询）。
func (s *Service) ChapterTitles(ctx context.Context, files []model.TutorFile) map[int]string {
	ids := make([]int, 0, len(files))
	seen := make(map[int]bool, len(files))
	for _, f := range files {
		if f.ChapterID != nil && !seen[*f.ChapterID] {
			seen[*f.ChapterID] = true
			ids = append(ids, *f.ChapterID)
		}
	}
	if len(ids) == 0 {
		return nil
	}
	titles := make(map[int]string, len(ids))
	var rows []model.Chapter
	if err := s.db.WithContext(ctx).Where("chapter_id IN ?", ids).Find(&rows).Error; err != nil {
		// 查不动只降级展示（UI 回落「章节 #id」），不整页失败；不静默。
		if s.logger != nil {
			s.logger.Warn("[tutorfile] 解析挂接章标题失败，降级显示章节编号", zap.Error(err))
		}
		return titles
	}
	for _, ch := range rows {
		titles[ch.ChapterID] = ch.Title
	}
	return titles
}

// loadFile 载入文件行并做归属校验（管理面动作的共用前置）。
func (s *Service) loadFile(ctx context.Context, tutorID, fileID int) (*model.TutorFile, error) {
	var file model.TutorFile
	if err := s.db.WithContext(ctx).First(&file, "file_id = ?", fileID).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("读取文件行失败: %w", err)
	}
	if file.TutorID != tutorID {
		return nil, ErrForbidden
	}
	return &file, nil
}

// totalChunksFor 按「除末片等大、末片补余」口径计算分片总数（服务与 handler 共用单点）。
func totalChunksFor(fileSize, chunkSize int64) int {
	return int((fileSize + chunkSize - 1) / chunkSize)
}

// chunkPlan 校验分片方案并返回总片数（spec A2：单片 5–8MiB、除末片等大、≤10,000 片）。
// 检查顺序刻意让片数上限先于 2GB 档判定：2GB/5MiB 上限组合下片数上限在公开 API 上不可达，
// 先判片数这条契约位（A2）才测得到（TestChunkPlanBandAndMaxChunks）。
func chunkPlan(fileSize, chunkSize int64) (int, error) {
	if chunkSize < MinChunkSize || chunkSize > MaxChunkSize {
		return 0, fmt.Errorf("%w: 分片大小须在 5–8MiB 之间", ErrInvalidParam)
	}
	total := totalChunksFor(fileSize, chunkSize)
	if total > MaxChunks {
		return 0, fmt.Errorf("%w: 分片总数超过 %d", ErrInvalidParam, MaxChunks)
	}
	if fileSize <= 0 || fileSize > MaxFileSize {
		return 0, fmt.Errorf("%w: 文件大小须在 1B–2GB 之间", ErrInvalidParam)
	}
	return total, nil
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
