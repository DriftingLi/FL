// Package service 实现业务服务层。
// 本文件：论坛图片模块——上传（校验 + 命名）与悬空图片生命周期（孤儿清理）。
// 命名契约（文件名内嵌毫秒时间戳）由 FileStore.Save 写入；时间戳知识归位存储 seam，
// 悬空 TTL 判定不再解析文件名（ADR-0027 C2），见 orphan_sweep.go。
package service

import (
	"context"
	"fmt"
	"mime/multipart"
	"net/http"
	"time"

	"go.uber.org/zap"
	"gorm.io/gorm"

	"forklift-training/internal/model"
	"forklift-training/internal/storage"
)

// 论坛图片常量（模块契约，单一事实源）。前缀登记归附件归属 module（attachment.go）。
const (
	ForumImageOrphanTTL = 24 * time.Hour // 悬空图片清理门槛（超过该时长未被引用才删）
)

// ForumImageService 论坛图片模块：上传与悬空清理。
type ForumImageService struct {
	db      *gorm.DB
	fileSvc *FileStore

	logger *zap.Logger
}

// NewForumImageService 构造论坛图片服务。
func NewForumImageService(db *gorm.DB, fileSvc *FileStore, logger *zap.Logger) *ForumImageService {
	return &ForumImageService{db: db, fileSvc: fileSvc, logger: logger}
}

// ForumImageError 论坛图片上传失败错误：携带 HTTP 状态码与最终响应消息。
type ForumImageError struct {
	Status  int    // http.StatusBadRequest（客户端校验失败）/ http.StatusInternalServerError
	Message string // 可直接作为响应 message
}

func (e *ForumImageError) Error() string { return e.Message }

// Upload 上传论坛图片：读取 multipart 文件头内容、校验格式/大小，
// 经 FileStore.Save 保存到 images/forum/ 子目录（文件名 <name>_<毫秒时间戳>.<ext>），
// 返回完整可访问 URL。
func (s *ForumImageService) Upload(ctx context.Context, fileHeader *multipart.FileHeader) (string, error) {
	if fileHeader.Filename == "" {
		return "", &ForumImageError{Status: http.StatusBadRequest, Message: "未选择文件"}
	}
	if ok, msg := s.fileSvc.ValidateImage(fileHeader.Filename, fileHeader.Size); !ok {
		return "", &ForumImageError{Status: http.StatusBadRequest, Message: msg}
	}
	content, err := ReadMultipartFile(fileHeader)
	if err != nil {
		return "", &ForumImageError{Status: http.StatusInternalServerError, Message: "图片上传失败"}
	}
	url, err := s.fileSvc.Save(content, fileHeader.Filename, ForumImageDirPrefix)
	if err != nil {
		return "", &ForumImageError{Status: http.StatusInternalServerError, Message: "图片上传失败: " + err.Error()}
	}
	return url, nil
}

// CleanupOrphans 清理论坛悬空图片（薄配置壳，算法单点见 orphan_sweep.go / ADR-0027 C2）：
// ListWithInfo(images/forum/) 与全量引用集差集，仅删存储侧 LastModified 超过
// ForumImageOrphanTTL 且未被任何主题/回复引用的文件。
// 返回清理的文件数（存储错误不中断）；引用集查不动或为空时整轮不清理（ADR-0062 票5）；
// ctx 取消语义贯穿到存储调用。
func (s *ForumImageService) CleanupOrphans(ctx context.Context) int {
	if s.fileSvc == nil {
		return 0
	}
	return runOrphanSweep(ctx, orphanSweepConfig{
		domain: "forum_image",
		ttl:    ForumImageOrphanTTL,
		list: func(c context.Context) ([]storage.FileInfo, error) {
			return s.fileSvc.ListWithInfoWithContext(c, ForumImageDirPrefix)
		},
		referenced: s.collectReferencedImages,
		keyOf:      func(u string) string { return AttachmentKey(u, ForumImageDirPrefix) },
		deleteFile: s.fileSvc.DeleteWithContext,
		logger:     s.logger,
	})
}

// collectReferencedImages 收集全部主题与回复引用的图片 key 集合（归一化为 images/forum/...）。
// 任一半查不动即返回 error：引用集不完整时无从判断谁还在被引用，sweep 据此整轮放弃
// （ADR-0062 票5——「查不动」不得被读成「没人引用」）。
func (s *ForumImageService) collectReferencedImages() (map[string]bool, error) {
	ref := map[string]bool{}
	var rawList []string
	if err := s.db.Model(&model.ForumTopic{}).Pluck("images", &rawList).Error; err != nil {
		return nil, fmt.Errorf("收集主题图片引用失败: %w", err)
	}
	for _, raw := range rawList {
		for _, u := range parseImageURLs(raw) {
			if key := AttachmentKey(u, ForumImageDirPrefix); key != "" {
				ref[key] = true
			} else if u != "" {
				ref[u] = true
			}
		}
	}
	rawList = rawList[:0]
	if err := s.db.Model(&model.ForumReply{}).Pluck("images", &rawList).Error; err != nil {
		return nil, fmt.Errorf("收集回复图片引用失败: %w", err)
	}
	for _, raw := range rawList {
		for _, u := range parseImageURLs(raw) {
			if key := AttachmentKey(u, ForumImageDirPrefix); key != "" {
				ref[key] = true
			} else if u != "" {
				ref[u] = true
			}
		}
	}
	return ref, nil
}
