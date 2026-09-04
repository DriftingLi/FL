// Package service 实现业务服务层。
// 本文件：论坛图片模块——上传（校验 + 命名）与悬空图片生命周期（孤儿清理）。
// 命名契约（文件名内嵌毫秒时间戳）由 FileStore.Save 写入；时间戳知识归位存储 seam，
// 悬空 TTL 判定不再解析文件名（ADR-0027 C2），见 orphan_sweep.go。
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
	"forklift-training/internal/storage"
)

// 论坛图片常量（模块契约，单一事实源）。
const (
	ForumImageDirPrefix = "images/forum"
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
	src, err := fileHeader.Open()
	if err != nil {
		return "", &ForumImageError{Status: http.StatusInternalServerError, Message: "图片上传失败"}
	}
	defer src.Close()
	content, err := io.ReadAll(src)
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
// 返回清理的文件数（尽力而为，存储错误不中断）；ctx 取消语义贯穿到存储调用。
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
		keyOf:      forumImageKey,
		deleteFile: s.fileSvc.DeleteWithContext,
		logger:     s.logger,
	})
}

// forumImageKey 提取 images/forum/ 后的对象 key（兼容 local 与 R2 两种 URL 形态）
// 例：/static/uploads/images/forum/a_1.webp → images/forum/a_1.webp
//
//	https://cdn.example.com/images/forum/a_1.webp → images/forum/a_1.webp
func forumImageKey(u string) string {
	idx := strings.Index(u, "images/forum/")
	if idx < 0 {
		return ""
	}
	return u[idx:]
}

// collectReferencedImages 收集全部主题与回复引用的图片 key 集合（归一化为 images/forum/...）。
func (s *ForumImageService) collectReferencedImages() map[string]bool {
	ref := map[string]bool{}
	var rawList []string
	if err := s.db.Model(&model.ForumTopic{}).Pluck("images", &rawList).Error; err == nil {
		for _, raw := range rawList {
			for _, u := range parseImageURLs(raw) {
				if key := forumImageKey(u); key != "" {
					ref[key] = true
				} else if u != "" {
					ref[u] = true
				}
			}
		}
	}
	rawList = rawList[:0]
	if err := s.db.Model(&model.ForumReply{}).Pluck("images", &rawList).Error; err == nil {
		for _, raw := range rawList {
			for _, u := range parseImageURLs(raw) {
				if key := forumImageKey(u); key != "" {
					ref[key] = true
				} else if u != "" {
					ref[u] = true
				}
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
