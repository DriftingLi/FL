// Package service 实现业务服务层。
// 本文件：学员端论坛（综合讨论区 + 章节讨论区，支持回复别人的回复，图文分离发图）。
package service

import (
	"encoding/json"
	"errors"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"go.uber.org/zap"
	"gorm.io/gorm"

	"forklift-training/internal/model"
	"forklift-training/pkg/paging"
	"forklift-training/pkg/response"
)

// 论坛范围常量。
const (
	ForumScopeAll     = "all"     // 全部（综合讨论区 + 章节讨论区）
	ForumScopeGeneral = "general" // 综合讨论区（chapter_id IS NULL）
	ForumScopeChapter = "chapter" // 指定章节讨论区
)

// 论坛发图限制。
const (
	ForumTopicMaxImages = 9 // 主题最多图片数
	ForumReplyMaxImages = 3 // 回复最多图片数
)

// ForumAuthor 论坛作者信息（展示名为昵称）。
type ForumAuthor struct {
	UserID    int    `json:"user_id"`
	Username  string `json:"username"`
	AvatarURL string `json:"avatar_url"`
}

// DisplayName 返回论坛展示名（昵称）。
func (a ForumAuthor) DisplayName() string {
	return a.Username
}

// ForumTopicDTO 论坛主题列表/详情对象。
type ForumTopicDTO struct {
	ID           int64       `json:"id"`
	ChapterID    *int        `json:"chapter_id"`
	ChapterTitle string      `json:"chapter_title"`
	Title        string      `json:"title"`
	Content      string      `json:"content"`
	Images       []string    `json:"images"`
	ViewCount    int         `json:"view_count"`
	ReplyCount   int         `json:"reply_count"`
	LastReplyAt  *string     `json:"last_reply_at"`
	CreatedAt    string      `json:"created_at"`
	Author       ForumAuthor `json:"author"`
	CanDelete    bool        `json:"can_delete"`
	LikesCount   int64       `json:"likes_count"`
	LikedByMe    bool        `json:"liked_by_me"`
}

// ForumReplyDTO 论坛回复对象。
type ForumReplyDTO struct {
	ID         int64       `json:"id"`
	TopicID    int64       `json:"topic_id"`
	ParentID   *int64      `json:"parent_id,omitempty"`
	ParentName string      `json:"parent_name,omitempty"` // 被回复人的展示名
	Content    string      `json:"content"`
	Images     []string    `json:"images"`
	CreatedAt  string      `json:"created_at"`
	Author     ForumAuthor `json:"author"`
	CanDelete  bool        `json:"can_delete"`
}

// ForumService 论坛服务。
type ForumService struct {
	db      *gorm.DB
	fileSvc *FileStore

	logger *zap.Logger
}

// NewForumService 构造论坛服务。
// fileSvc 用于删除帖子/回复时清理图片存储（可 nil，nil 时跳过清理）。
func NewForumService(db *gorm.DB, fileSvc *FileStore, logger *zap.Logger) *ForumService {
	return &ForumService{db: db, fileSvc: fileSvc, logger: logger}
}

// topicRow 列表查询的扫描结构。
type topicRow struct {
	ID           int64
	ChapterID    *int
	ChapterTitle string
	Title        string
	Content      string
	Images       string
	ViewCount    int
	ReplyCount   int
	LastReplyAt  *time.Time
	CreatedAt    time.Time
	UserID       int
	Username     string
	AvatarURL    string
}

func (r topicRow) toDTO(viewerID int) ForumTopicDTO {
	var lastReplyAt *string
	if r.LastReplyAt != nil {
		s := formatISO(*r.LastReplyAt)
		lastReplyAt = &s
	}
	return ForumTopicDTO{
		ID:           r.ID,
		ChapterID:    r.ChapterID,
		ChapterTitle: r.ChapterTitle,
		Title:        r.Title,
		Content:      r.Content,
		Images:       parseImageURLs(r.Images),
		ViewCount:    r.ViewCount,
		ReplyCount:   r.ReplyCount,
		LastReplyAt:  lastReplyAt,
		CreatedAt:    formatISO(r.CreatedAt),
		Author: ForumAuthor{
			UserID: r.UserID, Username: r.Username, AvatarURL: r.AvatarURL,
		},
		CanDelete: r.UserID == viewerID,
	}
}

// ForumTopicPageResult 论坛主题分页结果。
type ForumTopicPageResult struct {
	Page   int             `json:"page"`
	Pages  int             `json:"pages"`
	Topics []ForumTopicDTO `json:"topics"`
	Total  int64           `json:"total"`
}

// ListTopics 分页查询主题。
// scope: all（默认）/ general（综合讨论区）/ chapter（需配合 chapterID）。
func (s *ForumService) ListTopics(scope string, chapterID, page, pageSize int, keyword string) (*ForumTopicPageResult, error) {
	if scope == "" {
		scope = ForumScopeAll
	}
	if scope == ForumScopeChapter && chapterID <= 0 {
		return nil, errors.New("查询章节讨论区需要有效的 chapter_id")
	}

	rows, total, page, pageSize := paging.QueryWithScan[topicRow](s.db, page, pageSize, 10, 100,
		"COALESCE(t.last_reply_at, t.created_at) DESC, t.id DESC",
		func(q *gorm.DB) *gorm.DB {
			q = q.Table("forum_topics AS t").
				Select("t.id, t.chapter_id, t.title, t.content, t.images, t.view_count, t.reply_count, t.last_reply_at, t.created_at, " +
					"u.id AS user_id, u.username, u.avatar_url, " +
					"COALESCE(ch.title, '') AS chapter_title").
				Joins("JOIN hrwai_users AS u ON u.id = t.user_id").
				Joins("LEFT JOIN chapter AS ch ON ch.chapter_id = t.chapter_id")
			switch scope {
			case ForumScopeGeneral:
				q = q.Where("t.chapter_id IS NULL")
			case ForumScopeChapter:
				q = q.Where("t.chapter_id = ?", chapterID)
			}
			if keyword = strings.TrimSpace(keyword); keyword != "" {
				like := "%" + keyword + "%"
				q = q.Where("(t.title ILIKE ? OR t.content ILIKE ?)", like, like)
			}
			return q
		})

	items := make([]ForumTopicDTO, 0, len(rows))
	for _, r := range rows {
		items = append(items, r.toDTO(0))
	}
	return &ForumTopicPageResult{
		Page:   page,
		Pages:  response.PageCount(total, pageSize),
		Topics: items,
		Total:  total,
	}, nil
}

// GetTopic 主题详情（含回复，回复带被回复人信息），并累加浏览量。
func (s *ForumService) GetTopic(topicID int64, viewerID int) (map[string]any, error) {
	var row topicRow
	err := s.db.Table("forum_topics AS t").
		Select("t.id, t.chapter_id, t.title, t.content, t.images, t.view_count, t.reply_count, t.last_reply_at, t.created_at, "+
			"u.id AS user_id, u.username, u.avatar_url, "+
			"COALESCE(ch.title, '') AS chapter_title").
		Joins("JOIN hrwai_users AS u ON u.id = t.user_id").
		Joins("LEFT JOIN chapter AS ch ON ch.chapter_id = t.chapter_id").
		Where("t.id = ?", topicID).
		Scan(&row).Error
	if err != nil {
		return nil, err
	}
	if row.ID == 0 {
		return nil, gorm.ErrRecordNotFound
	}

	// 浏览量 +1（失败不影响主流程）
	_ = s.db.Model(&model.ForumTopic{}).Where("id = ?", topicID).
		UpdateColumn("view_count", gorm.Expr("view_count + 1")).Error
	row.ViewCount++

	// 回复列表（含被回复人展示名）
	var replies []struct {
		ID         int64
		TopicID    int64
		ParentID   *int64
		Content    string
		Images     string
		CreatedAt  time.Time
		UserID     int
		Username   string
		AvatarURL  string
		ParentName string
	}
	if err := s.db.Table("forum_replies AS r").
		Select("r.id, r.topic_id, r.parent_id, r.content, r.images, r.created_at, "+
			"u.id AS user_id, u.username, u.avatar_url, "+
			"COALESCE(pu.username, '') AS parent_name").
		Joins("JOIN hrwai_users AS u ON u.id = r.user_id").
		Joins("LEFT JOIN forum_replies AS pr ON pr.id = r.parent_id").
		Joins("LEFT JOIN hrwai_users AS pu ON pu.id = pr.user_id").
		Where("r.topic_id = ?", topicID).
		Order("r.created_at ASC, r.id ASC").
		Scan(&replies).Error; err != nil {
		return nil, err
	}

	replyDTOs := make([]ForumReplyDTO, 0, len(replies))
	for _, r := range replies {
		replyDTOs = append(replyDTOs, ForumReplyDTO{
			ID: r.ID, TopicID: r.TopicID, ParentID: r.ParentID, ParentName: r.ParentName,
			Content: r.Content, Images: parseImageURLs(r.Images), CreatedAt: formatISO(r.CreatedAt),
			Author: ForumAuthor{
				UserID: r.UserID, Username: r.Username, AvatarURL: r.AvatarURL,
			},
			CanDelete: r.UserID == viewerID,
		})
	}

	// 点赞状态（ADR-0018）：详情返回计数与当前用户是否已赞。
	topicDTO := row.toDTO(viewerID)
	topicDTO.LikesCount = s.topicLikesCount(topicID)
	topicDTO.LikedByMe = viewerID > 0 && s.hasLiked(viewerID, topicID)

	return map[string]any{
		"topic":   topicDTO,
		"replies": replyDTOs,
	}, nil
}

// CreateTopic 发帖。chapterID 为 nil/0 表示发到综合讨论区。
// images 为主题图片 URL 列表（最多 ForumTopicMaxImages 张，仅接受本站 images/forum/ 前缀）。
func (s *ForumService) CreateTopic(userID int, chapterID *int, title, content string, images []string) (*ForumTopicDTO, error) {
	title = strings.TrimSpace(title)
	content = strings.TrimSpace(content)
	if utf8.RuneCountInString(title) < 1 || utf8.RuneCountInString(title) > 100 {
		return nil, errors.New("标题长度需在 1-100 个字符之间")
	}
	if utf8.RuneCountInString(content) < 1 || utf8.RuneCountInString(content) > 10000 {
		return nil, errors.New("内容长度需在 1-10000 个字符之间")
	}
	if err := validateForumImages(images, ForumTopicMaxImages); err != nil {
		return nil, err
	}

	var cid *int
	if chapterID != nil && *chapterID > 0 {
		var cnt int64
		if err := s.db.Model(&model.Chapter{}).Where("chapter_id = ?", *chapterID).Count(&cnt).Error; err != nil {
			return nil, err
		}
		if cnt == 0 {
			return nil, errors.New("章节不存在")
		}
		cid = chapterID
	}

	now := beijingNow()
	topic := model.ForumTopic{
		ChapterID: cid,
		UserID:    userID,
		Title:     title,
		Content:   content,
		Images:    marshalImageURLs(images),
		CreatedAt: now,
		UpdatedAt: now,
	}
	if err := s.db.Create(&topic).Error; err != nil {
		return nil, err
	}

	var u model.HrwaiUser
	if err := s.db.First(&u, userID).Error; err != nil {
		return nil, err
	}
	return &ForumTopicDTO{
		ID:        topic.ID,
		ChapterID: topic.ChapterID,
		Title:     topic.Title,
		Content:   topic.Content,
		Images:    images,
		CreatedAt: formatISO(topic.CreatedAt),
		Author: ForumAuthor{
			UserID: u.ID, Username: u.Username, AvatarURL: u.AvatarURL,
		},
		CanDelete: true,
	}, nil
}

// ReplyTopic 回复主题或回复某条回复（parentReplyID 非空时）。
// images 为回复图片 URL 列表（最多 ForumReplyMaxImages 张，仅接受本站 images/forum/ 前缀）。
func (s *ForumService) ReplyTopic(userID int, topicID int64, content string, parentReplyID *int64, images []string) (*ForumReplyDTO, error) {
	content = strings.TrimSpace(content)
	if utf8.RuneCountInString(content) < 1 || utf8.RuneCountInString(content) > 5000 {
		return nil, errors.New("回复内容长度需在 1-5000 个字符之间")
	}
	if err := validateForumImages(images, ForumReplyMaxImages); err != nil {
		return nil, err
	}

	var topic model.ForumTopic
	if err := s.db.First(&topic, topicID).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, errors.New("主题不存在")
		}
		return nil, err
	}

	// 校验被回复的回复存在且属于同一主题
	var parentName string
	if parentReplyID != nil && *parentReplyID > 0 {
		var parent model.ForumReply
		if err := s.db.First(&parent, *parentReplyID).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return nil, errors.New("被回复的回复不存在")
			}
			return nil, err
		}
		if parent.TopicID != topicID {
			return nil, errors.New("被回复的回复不属于该主题")
		}
		var pu model.HrwaiUser
		if err := s.db.First(&pu, parent.UserID).Error; err == nil {
			parentName = ForumAuthor{
				UserID: pu.ID, Username: pu.Username, AvatarURL: pu.AvatarURL,
			}.DisplayName()
		}
	}

	now := beijingNow()
	reply := model.ForumReply{
		TopicID:   topicID,
		UserID:    userID,
		ParentID:  parentReplyID,
		Content:   content,
		Images:    marshalImageURLs(images),
		CreatedAt: now,
	}
	err := s.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(&reply).Error; err != nil {
			return err
		}
		return tx.Model(&model.ForumTopic{}).Where("id = ?", topicID).
			Updates(map[string]any{
				"reply_count":   gorm.Expr("reply_count + 1"),
				"last_reply_at": now,
				"updated_at":    now,
			}).Error
	})
	if err != nil {
		return nil, err
	}

	var u model.HrwaiUser
	if err := s.db.First(&u, userID).Error; err != nil {
		return nil, err
	}
	return &ForumReplyDTO{
		ID: reply.ID, TopicID: reply.TopicID, ParentID: reply.ParentID,
		ParentName: parentName, Content: reply.Content, Images: images, CreatedAt: formatISO(reply.CreatedAt),
		Author: ForumAuthor{
			UserID: u.ID, Username: u.Username, AvatarURL: u.AvatarURL,
		},
		CanDelete: true,
	}, nil
}

// DeleteTopic 删除主题（仅作者本人）。主题与全部回复（含子回复）的图片一并清理。
func (s *ForumService) DeleteTopic(userID int, topicID int64) error {
	var topic model.ForumTopic
	if err := s.db.First(&topic, topicID).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return errors.New("主题不存在")
		}
		return err
	}
	if topic.UserID != userID {
		return errors.New("只能删除自己发布的主题")
	}
	return s.deleteTopicWithImages(topicID)
}

// AdminDeleteTopic 管理员删除任意主题（不校验作者）。图片一并清理。
func (s *ForumService) AdminDeleteTopic(topicID int64) error {
	var topic model.ForumTopic
	if err := s.db.First(&topic, topicID).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return errors.New("主题不存在")
		}
		return err
	}
	return s.deleteTopicWithImages(topicID)
}

// deleteTopicWithImages 删除主题前收集主题 + 全部回复（含子回复）的图片并清理存储。
func (s *ForumService) deleteTopicWithImages(topicID int64) error {
	var topic model.ForumTopic
	if err := s.db.First(&topic, topicID).Error; err != nil {
		return err
	}
	urls := parseImageURLs(string(topic.Images))
	var replyImages []string
	if err := s.db.Model(&model.ForumReply{}).
		Where("topic_id = ?", topicID).
		Pluck("images", &replyImages).Error; err != nil {
		return err
	}
	for _, raw := range replyImages {
		urls = append(urls, parseImageURLs(raw)...)
	}
	if err := s.db.Delete(&model.ForumTopic{}, topicID).Error; err != nil {
		return err
	}
	s.deleteImages(urls)
	return nil
}

// DeleteReply 删除回复（仅作者本人；其下级回复随外键级联删除）。
// 本回复与全部下级回复（parent_id 链条）的图片一并清理。
func (s *ForumService) DeleteReply(userID int, replyID int64) error {
	var reply model.ForumReply
	if err := s.db.First(&reply, replyID).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return errors.New("回复不存在")
		}
		return err
	}
	if reply.UserID != userID {
		return errors.New("只能删除自己发布的回复")
	}
	return s.deleteReplyWithImages(replyID, reply.TopicID)
}

// AdminDeleteReply 管理员删除任意回复（不校验作者；其下级回复随外键级联删除）。图片一并清理。
func (s *ForumService) AdminDeleteReply(replyID int64) error {
	var reply model.ForumReply
	if err := s.db.First(&reply, replyID).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return errors.New("回复不存在")
		}
		return err
	}
	return s.deleteReplyWithImages(replyID, reply.TopicID)
}

// deleteReplyWithImages 删除回复前收集本回复 + 全部下级回复的图片并清理存储。
// 下级回复通过 parent_id 递归收集（单表递归 CTE 或逐层查询）。
func (s *ForumService) deleteReplyWithImages(replyID, topicID int64) error {
	urls, err := s.collectReplyImages(replyID)
	if err != nil {
		return err
	}
	if err := s.deleteReplyByID(replyID, topicID); err != nil {
		return err
	}
	s.deleteImages(urls)
	return nil
}

// collectReplyImages 收集回复及其全部下级回复（parent_id 链条）的图片 URL。
func (s *ForumService) collectReplyImages(replyID int64) ([]string, error) {
	var urls []string

	var self model.ForumReply
	if err := s.db.First(&self, replyID).Error; err != nil {
		return nil, err
	}
	urls = append(urls, parseImageURLs(string(self.Images))...)

	// BFS 收集下级回复
	level := []int64{replyID}
	for len(level) > 0 {
		var children []model.ForumReply
		if err := s.db.Where("parent_id IN ?", level).Find(&children).Error; err != nil {
			return nil, err
		}
		if len(children) == 0 {
			break
		}
		level = level[:0]
		for _, ch := range children {
			urls = append(urls, parseImageURLs(string(ch.Images))...)
			level = append(level, ch.ID)
		}
	}
	return urls, nil
}

// deleteImages 清理图片存储文件（fileSvc 为 nil 时跳过，尽力而为）。
func (s *ForumService) deleteImages(urls []string) {
	if s.fileSvc == nil || len(urls) == 0 {
		return
	}
	s.fileSvc.DeleteFiles(urls)
}

// deleteReplyByID 删除回复并回扣主题回复数、刷新最后回复时间。
func (s *ForumService) deleteReplyByID(replyID, topicID int64) error {
	return s.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Delete(&model.ForumReply{}, replyID).Error; err != nil {
			return err
		}
		// 回扣回复数（下限 0，避免并发下负数）
		var topic model.ForumTopic
		if err := tx.First(&topic, topicID).Error; err != nil {
			return err
		}
		newCount := topic.ReplyCount - 1
		if newCount < 0 {
			newCount = 0
		}
		if err := tx.Model(&model.ForumTopic{}).Where("id = ?", topicID).
			UpdateColumn("reply_count", newCount).Error; err != nil {
			return err
		}
		var last model.ForumReply
		if err := tx.Where("topic_id = ?", topicID).Order("created_at DESC, id DESC").
			Limit(1).Find(&last).Error; err != nil {
			return err
		}
		var lastAt *time.Time
		if last.ID > 0 {
			lastAt = &last.CreatedAt
		}
		return tx.Model(&model.ForumTopic{}).Where("id = ?", topicID).
			Update("last_reply_at", lastAt).Error
	})
}

// ===== 图片工具 =====

// validateForumImages 校验图片 URL 列表：数量上限 + 来源（仅接受本站 images/forum/ 前缀）。
// 允许 local（/static/uploads/images/forum/...）与 R2（https://.../images/forum/...）两种形式。
func validateForumImages(images []string, max int) error {
	if len(images) == 0 {
		return nil
	}
	if len(images) > max {
		return errors.New("图片数量超出限制（最多 " + strconv.Itoa(max) + " 张）")
	}
	for _, u := range images {
		if !isForumImageURL(u) {
			return errors.New("图片地址无效（仅支持本站上传的论坛图片）")
		}
	}
	return nil
}

// parseImageURLs 将 JSONB 图片数组字符串解析为 URL 列表（无效 JSON 返回空列表）。
func parseImageURLs(raw string) []string {
	raw = strings.TrimSpace(raw)
	if raw == "" || raw == "null" {
		return nil
	}
	var urls []string
	if err := json.Unmarshal([]byte(raw), &urls); err != nil {
		return nil
	}
	return urls
}

// marshalImageURLs 将 URL 列表序列化为 JSONB 字节。
func marshalImageURLs(urls []string) model.JSONB {
	if len(urls) == 0 {
		return model.JSONB([]byte("[]"))
	}
	b, _ := json.Marshal(urls)
	return model.JSONB(b)
}

// ===== 论坛互动（ADR-0018：点赞 / 举报 / 我的帖子 / 我的回复）=====

// LikeTopic 点赞主题（幂等：重复点赞不报错、不重复计数）。
func (s *ForumService) LikeTopic(userID int, topicID int64) (int64, error) {
	var cnt int64
	if err := s.db.Model(&model.ForumTopic{}).Where("id = ?", topicID).Count(&cnt).Error; err != nil {
		return 0, err
	}
	if cnt == 0 {
		return 0, errors.New("主题不存在")
	}
	var existing model.ForumTopicLike
	if err := s.db.Where("topic_id = ? AND user_id = ?", topicID, userID).
		Limit(1).Find(&existing).Error; err != nil {
		return 0, err
	}
	if existing.ID == 0 {
		if err := s.db.Create(&model.ForumTopicLike{
			TopicID: topicID, UserID: userID, CreatedAt: beijingNow(),
		}).Error; err != nil {
			return 0, err
		}
	}
	return s.topicLikesCount(topicID), nil
}

// UnlikeTopic 取消点赞（幂等：未点赞时直接返回当前计数）。
func (s *ForumService) UnlikeTopic(userID int, topicID int64) (int64, error) {
	if err := s.db.Where("topic_id = ? AND user_id = ?", topicID, userID).
		Delete(&model.ForumTopicLike{}).Error; err != nil {
		return 0, err
	}
	return s.topicLikesCount(topicID), nil
}

// topicLikesCount 主题点赞数。
func (s *ForumService) topicLikesCount(topicID int64) int64 {
	var n int64
	s.db.Model(&model.ForumTopicLike{}).Where("topic_id = ?", topicID).Count(&n)
	return n
}

// hasLiked 当前用户是否已点赞。
func (s *ForumService) hasLiked(userID int, topicID int64) bool {
	var n int64
	s.db.Model(&model.ForumTopicLike{}).Where("topic_id = ? AND user_id = ?", topicID, userID).Count(&n)
	return n > 0
}

// CreateReport 举报主题或回复（topicID/replyID 二选一，由调用方保证）。
func (s *ForumService) CreateReport(userID int, topicID, replyID *int64, reason string) error {
	reason = strings.TrimSpace(reason)
	if utf8.RuneCountInString(reason) < 1 || utf8.RuneCountInString(reason) > 500 {
		return errors.New("举报理由长度需在 1-500 个字符之间")
	}
	if (topicID == nil) == (replyID == nil) {
		return errors.New("举报对象必须为主题或回复之一")
	}
	if topicID != nil {
		var cnt int64
		s.db.Model(&model.ForumTopic{}).Where("id = ?", *topicID).Count(&cnt)
		if cnt == 0 {
			return errors.New("主题不存在")
		}
	}
	if replyID != nil {
		var cnt int64
		s.db.Model(&model.ForumReply{}).Where("id = ?", *replyID).Count(&cnt)
		if cnt == 0 {
			return errors.New("回复不存在")
		}
	}
	return s.db.Create(&model.ForumReport{
		ReporterID: userID, TopicID: topicID, ReplyID: replyID,
		Reason: reason, Status: 0, CreatedAt: beijingNow(),
	}).Error
}

// ForumReportDTO 管理端举报条目。
type ForumReportDTO struct {
	ID         int64  `json:"id"`
	ReporterID int    `json:"reporter_id"`
	Reporter   string `json:"reporter"`
	TopicID    *int64 `json:"topic_id,omitempty"`
	TopicTitle string `json:"topic_title"`
	ReplyID    *int64 `json:"reply_id,omitempty"`
	Reason     string `json:"reason"`
	Status     int16  `json:"status"`
	CreatedAt  string `json:"created_at"`
}

// ForumReportPageResult 举报分页结果。
type ForumReportPageResult struct {
	Page    int              `json:"page"`
	Pages   int              `json:"pages"`
	Total   int64            `json:"total"`
	Reports []ForumReportDTO `json:"reports"`
}

// ListReports 管理端举报列表（status: nil 全部 / 0 待处理 / 1 已处理）。
func (s *ForumService) ListReports(page, pageSize int, status *int16) (*ForumReportPageResult, error) {
	type reportRow struct {
		ID         int64
		ReporterID int
		Reporter   string
		TopicID    *int64
		TopicTitle string
		ReplyID    *int64
		Reason     string
		Status     int16
		CreatedAt  time.Time
	}
	rows, total, page, pageSize := paging.QueryWithScan[reportRow](s.db, page, pageSize, 20, 100,
		"r.created_at DESC, r.id DESC",
		func(q *gorm.DB) *gorm.DB {
			q = q.Table("forum_report AS r").
				Select("r.id, r.reporter_id, r.topic_id, r.reply_id, r.reason, r.status, r.created_at, " +
					"COALESCE(u.username, '') AS reporter, COALESCE(t.title, '') AS topic_title").
				Joins("LEFT JOIN hrwai_users AS u ON u.id = r.reporter_id").
				Joins("LEFT JOIN forum_topics AS t ON t.id = r.topic_id")
			if status != nil {
				q = q.Where("r.status = ?", *status)
			}
			return q
		})
	items := make([]ForumReportDTO, 0, len(rows))
	for _, r := range rows {
		items = append(items, ForumReportDTO{
			ID: r.ID, ReporterID: r.ReporterID, Reporter: r.Reporter,
			TopicID: r.TopicID, TopicTitle: r.TopicTitle, ReplyID: r.ReplyID,
			Reason: r.Reason, Status: r.Status, CreatedAt: formatISO(r.CreatedAt),
		})
	}
	return &ForumReportPageResult{
		Page: page, Pages: response.PageCount(total, pageSize),
		Total: total, Reports: items,
	}, nil
}

// HandleReport 管理端处理举报（status: 0 待处理 / 1 已处理）。
func (s *ForumService) HandleReport(reportID int64, status int16) error {
	if status != 0 && status != 1 {
		return errors.New("状态仅支持 0（待处理）/ 1（已处理）")
	}
	res := s.db.Model(&model.ForumReport{}).Where("id = ?", reportID).
		Update("status", status)
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return errors.New("举报不存在")
	}
	return nil
}

// MyTopics 我的帖子（复用主题列表行装配，按最后活跃倒序）。
func (s *ForumService) MyTopics(userID, page, pageSize int) (*ForumTopicPageResult, error) {
	rows, total, page, pageSize := paging.QueryWithScan[topicRow](s.db, page, pageSize, 10, 100,
		"COALESCE(t.last_reply_at, t.created_at) DESC, t.id DESC",
		func(q *gorm.DB) *gorm.DB {
			return q.Table("forum_topics AS t").
				Select("t.id, t.chapter_id, t.title, t.content, t.images, t.view_count, t.reply_count, t.last_reply_at, t.created_at, "+
					"u.id AS user_id, u.username, u.avatar_url, COALESCE(ch.title, '') AS chapter_title").
				Joins("JOIN hrwai_users AS u ON u.id = t.user_id").
				Joins("LEFT JOIN chapter AS ch ON ch.chapter_id = t.chapter_id").
				Where("t.user_id = ?", userID)
		})
	items := make([]ForumTopicDTO, 0, len(rows))
	for _, r := range rows {
		items = append(items, r.toDTO(userID))
	}
	return &ForumTopicPageResult{
		Page: page, Pages: response.PageCount(total, pageSize),
		Topics: items, Total: total,
	}, nil
}

// MyReplyDTO 我的回复条目（带主题标题回填）。
type MyReplyDTO struct {
	ID         int64       `json:"id"`
	TopicID    int64       `json:"topic_id"`
	TopicTitle string      `json:"topic_title"`
	ParentID   *int64      `json:"parent_id,omitempty"`
	Content    string      `json:"content"`
	Images     []string    `json:"images"`
	CreatedAt  string      `json:"created_at"`
	Author     ForumAuthor `json:"author"`
}

// MyReplyPageResult 我的回复分页结果。
type MyReplyPageResult struct {
	Page    int          `json:"page"`
	Pages   int          `json:"pages"`
	Total   int64        `json:"total"`
	Replies []MyReplyDTO `json:"replies"`
}

// MyReplies 我的回复（主题被删时标题为空串，条目保留）。
func (s *ForumService) MyReplies(userID, page, pageSize int) (*MyReplyPageResult, error) {
	type myReplyRow struct {
		ID         int64
		TopicID    int64
		TopicTitle string
		ParentID   *int64
		Content    string
		Images     string
		CreatedAt  time.Time
		UserID     int
		Username   string
		AvatarURL  string
	}
	rows, total, page, pageSize := paging.QueryWithScan[myReplyRow](s.db, page, pageSize, 10, 100,
		"r.created_at DESC, r.id DESC",
		func(q *gorm.DB) *gorm.DB {
			return q.Table("forum_replies AS r").
				Select("r.id, r.topic_id, r.parent_id, r.content, r.images, r.created_at, "+
					"u.id AS user_id, u.username, u.avatar_url, COALESCE(t.title, '') AS topic_title").
				Joins("JOIN hrwai_users AS u ON u.id = r.user_id").
				Joins("LEFT JOIN forum_topics AS t ON t.id = r.topic_id").
				Where("r.user_id = ?", userID)
		})
	items := make([]MyReplyDTO, 0, len(rows))
	for _, r := range rows {
		items = append(items, MyReplyDTO{
			ID: r.ID, TopicID: r.TopicID, TopicTitle: r.TopicTitle, ParentID: r.ParentID,
			Content: r.Content, Images: parseImageURLs(r.Images), CreatedAt: formatISO(r.CreatedAt),
			Author: ForumAuthor{UserID: r.UserID, Username: r.Username, AvatarURL: r.AvatarURL},
		})
	}
	return &MyReplyPageResult{
		Page: page, Pages: response.PageCount(total, pageSize),
		Total: total, Replies: items,
	}, nil
}
