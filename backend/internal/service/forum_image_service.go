// Package service 实现业务服务层。
// 本文件：论坛图片模块——上传（校验 + 命名）与悬空图片生命周期（孤儿清理）。
// 命名契约（文件名内嵌毫秒时间戳）由 FileService 单一拥有：SaveFile 负责写入，
// ExtractTimestamp 负责提取，本模块不再自行解析时间戳。
package service

import (
	"context"
	"io"
	"mime/multipart"
	"net/http"
	"strings"
	"time"

	"go.uber.org/zap"
	"gorm.io/gorm"

	"forklift-training/internal/model"
)

// 论坛图片常量（模块契约，单一事实源）。
const (
	ForumImageDirPrefix = "images/forum"
	ForumImageOrphanTTL = 24 * time.Hour // 悬空图片清理门槛（超过该时长未被引用才删）
)

// ForumImageService 论坛图片模块：上传与悬空清理。
type ForumImageService struct {
	db      *gorm.DB
	fileSvc *FileService

	logger *zap.Logger
}

// NewForumImageService 构造论坛图片服务。
func NewForumImageService(db *gorm.DB, fileSvc *FileService, logger *zap.Logger) *ForumImageService {
	return &ForumImageService{db: db, fileSvc: fileSvc, logger: logger}
}

// ForumImageError 论坛图片上传失败错误：携带 HTTP 状态码与最终响应消息。
type ForumImageError struct {
	Status  int    // http.StatusBadRequest（客户端校验失败）/ http.StatusInternalServerError
	Message string // 可直接作为响应 message
}

func (e *ForumImageError) Error() string { return e.Message }

// Upload 上传论坛图片：读取 multipart 文件头内容、校验格式/大小，
// 经 FileService.SaveFile 保存到 images/forum/ 子目录（文件名 <name>_<毫秒时间戳>.<ext>），
// 返回完整可访问 URL。
func (s *ForumImageService) Upload(ctx context.Context, fileHeader *multipart.FileHeader) (string, error) {
	if fileHeader.Filename == "" {
		return "", &ForumImageError{Status: http.StatusBadRequest, Message: "未选择文件"}
	}
	if ok, msg := s.fileSvc.ValidateImageFile(fileHeader.Filename, fileHeader.Size); !ok {
		return "", &ForumImageError{Status: http.StatusBadRequest, Message: msg}
	}
	src, err := fileHeader.Open()
	if err != nil {
		return "", &ForumImageError{Status: http.StatusInternalServerError, Message: "图片上传失败"}
	}
	defer src.Close()
	content, err := io.ReadAll(src)
	if err != nil {
		return "", &ForumImageError{Status: http.StatusInternalServerError, Message: "图片上传失败"}
	}
	url, err := s.fileSvc.SaveFile(content, fileHeader.Filename, ForumImageDirPrefix)
	if err != nil {
		return "", &ForumImageError{Status: http.StatusInternalServerError, Message: "图片上传失败: " + err.Error()}
	}
	return url, nil
}

// CleanupOrphans 清理论坛悬空图片：List(images/forum/) 与全量引用集差集，
// 仅删除文件名时间戳超过 ForumImageOrphanTTL 且未被任何主题/回复引用的文件。
// 返回清理的文件数（尽力而为，存储错误不中断）。
func (s *ForumImageService) CleanupOrphans(_ context.Context) int {
	if s.fileSvc == nil {
		return 0
	}
	stored := s.fileSvc.ListFiles(ForumImageDirPrefix)
	if len(stored) == 0 {
		return 0
	}

	referenced := s.collectReferencedImages()
	cleaned := 0
	cutoff := time.Now().Add(-ForumImageOrphanTTL)
	for _, u := range stored {
		if referenced[u] {
			continue
		}
		if !isForumImageURL(u) {
			continue
		}
		if ms, ok := s.fileSvc.ExtractTimestamp(u); ok && time.UnixMilli(ms).Before(cutoff) {
			if err := s.fileSvc.DeleteFile(u); err == nil {
				cleaned++
			}
		}
	}
	return cleaned
}

// collectReferencedImages 收集全部主题与回复引用的图片 URL 集合。
func (s *ForumImageService) collectReferencedImages() map[string]bool {
	ref := map[string]bool{}
	var rawList []string
	if err := s.db.Model(&model.ForumTopic{}).Pluck("images", &rawList).Error; err == nil {
		for _, raw := range rawList {
			for _, u := range parseImageURLs(raw) {
				ref[u] = true
			}
		}
	}
	rawList = rawList[:0]
	if err := s.db.Model(&model.ForumReply{}).Pluck("images", &rawList).Error; err == nil {
		for _, raw := range rawList {
			for _, u := range parseImageURLs(raw) {
				ref[u] = true
			}
		}
	}
	return ref
}

// isForumImageURL 判断 URL 是否指向本站 images/forum/ 子目录。
func isForumImageURL(u string) bool {
	u = strings.TrimSpace(u)
	if u == "" {
		return false
	}
	// local：/static/uploads/images/forum/xxx
	if strings.HasPrefix(u, "/static/uploads/images/forum/") {
		return true
	}
	// R2：https://<任意域名>/images/forum/xxx
	idx := strings.Index(u, "/images/forum/")
	return idx > 0 && (strings.HasPrefix(u, "http://") || strings.HasPrefix(u, "https://"))
}
