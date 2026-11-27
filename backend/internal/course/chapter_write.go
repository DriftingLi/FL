package course

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"

	"go.uber.org/zap"
	"gorm.io/gorm"

	"forklift-training/internal/clock"
	"forklift-training/internal/filestore"
	"forklift-training/internal/model"
)

var (
	// ErrChapterWriteFailed 章节已落盘但后续数据库写入失败。
	ErrChapterWriteFailed = errors.New("章节写入失败")
	// ErrChapterFileNotFound 章节附件行不存在。
	ErrChapterFileNotFound = errors.New("文件不存在")
	// ErrChapterWriteFileNotFound 文件库文件行不存在。
	ErrChapterWriteFileNotFound = errors.New("上传会话或文件不存在")
	// ErrChapterWriteFileForbidden 文件库文件归属他人。
	ErrChapterWriteFileForbidden = errors.New("无权操作该资源")
	// ErrChapterWriteFileInvalidState 附件挂接状态不允许该操作。
	ErrChapterWriteFileInvalidState = errors.New("上传会话状态不允许该操作")
	// ErrChapterWriteFileInvalidParam 附件写入参数不合法。
	ErrChapterWriteFileInvalidParam = errors.New("上传参数不合法")
)

// ChapterWriter 是讲师章节写入 seam：认领、归属、草稿与审核快照规则由课程域单点承载。
type ChapterWriter struct {
	db     *gorm.DB
	files  *filestore.FileStore
	slides *SlideRenderer
	logger *zap.Logger
}

// NewChapterWriter 创建章节写入 module。
func NewChapterWriter(db *gorm.DB, files *filestore.FileStore, slides *SlideRenderer, logger *zap.Logger) *ChapterWriter {
	return &ChapterWriter{db: db, files: files, slides: slides, logger: logger}
}

// Update 更新章节工作副本；正文变化回草稿，纯元数据不改变审核状态。
func (w *ChapterWriter) Update(ctx context.Context, chapterID, tutorID int, in *ChapterInput) (*ChapterDTO, error) {
	var chapter model.Chapter
	if err := fetchRow(w.db.WithContext(ctx).Where("chapter_id = ?", chapterID), &chapter, ErrChapterNotFound); err != nil {
		return nil, err
	}
	if err := RequireChapterClaim(w.db, tutorID, chapterID); err != nil {
		return nil, err
	}
	if in == nil {
		in = &ChapterInput{}
	}
	updates := make(map[string]any)
	if in.Title != nil && *in.Title != "" {
		chapter.Title = *in.Title
		updates["title"] = chapter.Title
	}
	if in.Content != nil {
		chapter.Content = *in.Content
		updates["content"] = chapter.Content
	}
	if in.Duration != nil {
		chapter.Duration = *in.Duration
		updates["duration"] = chapter.Duration
	}
	if in.OrderNum != nil {
		chapter.OrderNum = *in.OrderNum
		updates["order_num"] = chapter.OrderNum
	}
	if in.Description != nil {
		chapter.Description = *in.Description
		updates["description"] = chapter.Description
	}
	if err := w.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := RequireChapterClaim(tx, tutorID, chapterID); err != nil {
			return err
		}
		if len(updates) > 0 {
			if err := tx.Model(&model.Chapter{}).Where("chapter_id = ?", chapterID).Updates(updates).Error; err != nil {
				return err
			}
		}
		if err := ClaimChapterOwnership(tx, chapterID, tutorID); err != nil {
			return err
		}
		if in.Content != nil {
			return MarkChapterDraft(tx, chapterID)
		}
		return nil
	}); err != nil {
		return nil, err
	}
	dto := ChapterToDTO(&chapter)
	return &dto, nil
}

// SubmitForReview 提交整章审核；认领与在审快照由同一写入 seam 承载。
func (w *ChapterWriter) SubmitForReview(ctx context.Context, chapterID, tutorID int) (*ChapterDTO, error) {
	if err := w.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := RequireChapterClaim(tx, tutorID, chapterID); err != nil {
			return err
		}
		return submitChapterForReviewInTx(tx, chapterID, tutorID)
	}); err != nil {
		return nil, err
	}
	var chapter model.Chapter
	if err := fetchRow(w.db.WithContext(ctx).Where("chapter_id = ?", chapterID), &chapter, ErrChapterNotFound); err != nil {
		return nil, err
	}
	dto := ChapterToDTO(&chapter)
	return &dto, nil
}

// DeleteFile 删除章节附件并更新所属章节的草稿与内容形态。
func (w *ChapterWriter) DeleteFile(ctx context.Context, fileID, tutorID int) error {
	var file model.ChapterFile
	if err := fetchRow(w.db.WithContext(ctx).Where("file_id = ?", fileID), &file, ErrChapterFileNotFound); err != nil {
		return err
	}
	if file.ChapterID != nil {
		if err := RequireChapterClaim(w.db, tutorID, *file.ChapterID); err != nil {
			return err
		}
	}
	if w.files != nil {
		_ = w.files.Delete(file.FileURL)
	}
	return w.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Delete(&file).Error; err != nil {
			return err
		}
		if file.ChapterID == nil {
			return nil
		}
		if err := MarkChapterDraft(tx, *file.ChapterID); err != nil {
			return err
		}
		if err := ClaimChapterOwnership(tx, *file.ChapterID, tutorID); err != nil {
			return err
		}
		return RecomputeChapterContentTypeAfterFileChange(tx, *file.ChapterID)
	})
}

// BatchDeleteFiles 批量删除附件，保留既有「失败项进入 failed_ids」的部分成功语义。
func (w *ChapterWriter) BatchDeleteFiles(ctx context.Context, fileIDs []int, tutorID int) (int, []int, error) {
	failedIDs := make([]int, 0)
	successCount := 0
	for _, fileID := range fileIDs {
		if err := w.DeleteFile(ctx, fileID, tutorID); err != nil {
			if errors.Is(err, ErrCourseClaimRequired) || errors.Is(err, ErrChapterFileNotFound) {
				failedIDs = append(failedIDs, fileID)
				continue
			}
			return successCount, failedIDs, err
		}
		successCount++
	}
	return successCount, failedIDs, nil
}

// AttachFile 把文件库文件单挂到章节，并更新章节归属与草稿。
func (w *ChapterWriter) AttachFile(ctx context.Context, tutorID, fileID, chapterID int) error {
	file, err := w.loadTutorFile(ctx, tutorID, fileID)
	if err != nil {
		return err
	}
	if err := RequireChapterClaim(w.db, tutorID, chapterID); err != nil {
		return err
	}
	if file.ChapterID != nil {
		if *file.ChapterID == chapterID {
			return nil
		}
		return fmt.Errorf("%w: 文件已挂接到其他章节（#%d），先解挂再挂接", ErrChapterWriteFileInvalidState, *file.ChapterID)
	}
	return w.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := RequireChapterClaim(tx, tutorID, chapterID); err != nil {
			return err
		}
		if err := tx.Model(&model.TutorFile{}).Where("file_id = ?", fileID).Update("chapter_id", chapterID).Error; err != nil {
			return err
		}
		if err := MarkChapterDraft(tx, chapterID); err != nil {
			return err
		}
		return ClaimChapterOwnership(tx, chapterID, tutorID)
	})
}

// DetachFile 取消文件挂接；未挂接时是幂等空操作。
func (w *ChapterWriter) DetachFile(ctx context.Context, tutorID, fileID int) error {
	file, err := w.loadTutorFile(ctx, tutorID, fileID)
	if err != nil {
		return err
	}
	if file.ChapterID == nil {
		return nil
	}
	chapterID := *file.ChapterID
	if err := RequireChapterClaim(w.db, tutorID, chapterID); err != nil {
		return err
	}
	return w.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := RequireChapterClaim(tx, tutorID, chapterID); err != nil {
			return err
		}
		if err := tx.Model(&model.TutorFile{}).Where("file_id = ?", fileID).Update("chapter_id", nil).Error; err != nil {
			return err
		}
		if err := MarkChapterDraft(tx, chapterID); err != nil {
			return err
		}
		return ClaimChapterOwnership(tx, chapterID, tutorID)
	})
}

// SwapChapterFile 原子换挂新旧文件，并更新章节归属与草稿。
func (w *ChapterWriter) SwapChapterFile(ctx context.Context, tutorID, chapterID, oldFileID, newFileID int) error {
	if oldFileID == newFileID {
		return fmt.Errorf("%w: 新旧文件不能相同", ErrChapterWriteFileInvalidParam)
	}
	if err := RequireChapterClaim(w.db, tutorID, chapterID); err != nil {
		return err
	}
	old, err := w.loadTutorFile(ctx, tutorID, oldFileID)
	if err != nil {
		return err
	}
	fresh, err := w.loadTutorFile(ctx, tutorID, newFileID)
	if err != nil {
		return err
	}
	if old.ChapterID == nil || *old.ChapterID != chapterID {
		return fmt.Errorf("%w: 旧文件未挂在该章节，无法换挂", ErrChapterWriteFileInvalidState)
	}
	if fresh.ChapterID != nil {
		return fmt.Errorf("%w: 新文件已挂接到其他章节（#%d），先解挂再换挂", ErrChapterWriteFileInvalidState, *fresh.ChapterID)
	}
	return w.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := RequireChapterClaim(tx, tutorID, chapterID); err != nil {
			return err
		}
		if err := tx.Model(&model.TutorFile{}).Where("file_id = ?", newFileID).Update("chapter_id", chapterID).Error; err != nil {
			return err
		}
		if err := tx.Model(&model.TutorFile{}).Where("file_id = ?", oldFileID).Update("chapter_id", nil).Error; err != nil {
			return err
		}
		if err := MarkChapterDraft(tx, chapterID); err != nil {
			return err
		}
		return ClaimChapterOwnership(tx, chapterID, tutorID)
	})
}

func (w *ChapterWriter) loadTutorFile(ctx context.Context, tutorID, fileID int) (*model.TutorFile, error) {
	var file model.TutorFile
	if err := fetchRow(w.db.WithContext(ctx).Where("file_id = ?", fileID), &file, ErrChapterWriteFileNotFound); err != nil {
		return nil, err
	}
	if file.TutorID != tutorID {
		return nil, ErrChapterWriteFileForbidden
	}
	return &file, nil
}

// UploadFile 上传章节附件，并把附件清单、归属与草稿状态放进同一数据库事务。
func (w *ChapterWriter) UploadFile(ctx context.Context, chapterID, tutorID int, filename string, content []byte) (*ChapterFileDTO, error) {
	var chapter model.Chapter
	if err := fetchRow(w.db.Where("chapter_id = ?", chapterID), &chapter, ErrChapterNotFound); err != nil {
		return nil, err
	}
	if err := RequireChapterClaim(w.db, tutorID, chapterID); err != nil {
		return nil, err
	}
	if filename == "" {
		return nil, errors.New("文件名不能为空")
	}
	if w.files == nil {
		return nil, errors.New("文件服务不可用")
	}
	if !filestore.AllowedFile(filename) {
		return nil, errors.New("不支持的文件格式")
	}
	if !filestore.ValidateFileSize(int64(len(content)), filename) {
		return nil, errors.New("文件大小超出限制")
	}
	contentType := filestore.FileContentType(filename)
	if err := RequireChapterContentNature(w.db, chapterID, contentType); err != nil {
		return nil, err
	}
	fileURL, err := w.files.Save(content, filename, filestore.ChapterFileDirPrefix)
	if err != nil {
		return nil, fmt.Errorf("保存文件失败: %w", err)
	}
	storedURLs := []string{fileURL}
	if contentType == "ppt" && w.slides != nil {
		storedURLs = append(storedURLs, w.slides.Render(content, chapterID)...)
	}

	chapterFile := model.ChapterFile{
		ChapterID: &chapterID, FileURL: fileURL, FileName: filename,
		ContentType: contentType, FileSize: int64(len(content)), CreatedAt: clock.Now(),
	}
	if err := w.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := RequireChapterClaim(tx, tutorID, chapterID); err != nil {
			return err
		}
		if err := tx.Create(&chapterFile).Error; err != nil {
			return err
		}
		if chapter.ContentType == "" || chapter.ContentType == "text" {
			if err := tx.Model(&model.Chapter{}).Where("chapter_id = ?", chapterID).
				Updates(map[string]any{"content_type": contentType, "file_url": fileURL}).Error; err != nil {
				return err
			}
		}
		if len(storedURLs) > 1 {
			slideURLsJSON, err := json.Marshal(storedURLs[1:])
			if err != nil {
				return err
			}
			if err := tx.Model(&model.Chapter{}).Where("chapter_id = ?", chapterID).
				Update("slide_urls", string(slideURLsJSON)).Error; err != nil {
				return err
			}
		}
		if err := MarkChapterDraft(tx, chapterID); err != nil {
			return err
		}
		return ClaimChapterOwnership(tx, chapterID, tutorID)
	}); err != nil {
		return nil, w.compensateStoredWrite(storedURLs, err)
	}
	dto := ChapterFileToDTO(&chapterFile)
	return &dto, nil
}

// RequireWritable 在读取上传文件前回答章节是否存在、当前讲师是否认领。
func (w *ChapterWriter) RequireWritable(chapterID, tutorID int) error {
	if chapterID <= 0 {
		return errors.New("章节ID无效")
	}
	return RequireChapterClaim(w.db, tutorID, chapterID)
}

// UploadImage 写入章节内嵌图片。文件格式由 HTTP 上传适配器校验，本 module 只接收已解析内容。
func (w *ChapterWriter) UploadImage(ctx context.Context, chapterID, tutorID int, content []byte, filename string) (string, error) {
	if err := w.RequireWritable(chapterID, tutorID); err != nil {
		return "", err
	}
	if w.files == nil {
		return "", errors.New("文件服务不可用")
	}
	subfolder := filestore.ChapterImageDirPrefix + "/" + strconv.Itoa(chapterID)
	fileURL, err := w.files.Save(content, filename, subfolder)
	if err != nil {
		return "", err
	}
	if err := w.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := RequireChapterClaim(tx, tutorID, chapterID); err != nil {
			return err
		}
		return ClaimChapterOwnership(tx, chapterID, tutorID)
	}); err != nil {
		return "", w.compensateStoredWrite([]string{fileURL}, err)
	}
	return fileURL, nil
}

func (w *ChapterWriter) compensateStoredWrite(urls []string, cause error) error {
	writeErr := fmt.Errorf("%w: 数据库写入失败: %w", ErrChapterWriteFailed, cause)
	var cleanupErrs []error
	for _, url := range urls {
		if err := w.files.Delete(url); err != nil {
			cleanupErrs = append(cleanupErrs, fmt.Errorf("落盘对象 %s 补偿失败: %w", url, err))
		}
	}
	if len(cleanupErrs) == 0 {
		return writeErr
	}
	combined := errors.Join(writeErr, errors.Join(cleanupErrs...))
	if w.logger != nil {
		w.logger.Error("[course] 章节写入落盘补偿失败", zap.Error(combined))
	}
	return combined
}
