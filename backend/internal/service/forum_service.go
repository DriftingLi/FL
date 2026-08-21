// Package service 实现业务服务层。
// 本文件：学员端论坛（综合讨论区 + 章节讨论区，支持回复别人的回复，图文分离发图）。
package service

import (
	"encoding/json"
	"errors"
	"fmt"
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
	LikesCount int64       `json:"likes_count"`
	LikedByMe  bool        `json:"liked_by_me"`
}

// ForumService 论坛服务。
type ForumService struct {
	db              *gorm.DB
	fileSvc         *FileStore
	notificationSvc *NotificationService

	logger *zap.Logger
}

// NewForumService 构造论坛服务。
// fileSvc 用于删除帖子/回复时清理图片存储（可 nil，nil 时跳过清理）；
// notificationSvc 用于论坛事件站内信（回复/举报处理/管理端删帖，见各触发点）。
func NewForumService(db *gorm.DB, fileSvc *FileStore, notificationSvc *NotificationService, logger *zap.Logger) *ForumService {
	return &ForumService{db: db, fileSvc: fileSvc, notificationSvc: notificationSvc, logger: logger}
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
	if len(items) > 0 {
		ids := make([]int64, 0, len(items))
		for _, it := range items {
			ids = append(ids, it.ID)
		}
		var counts []struct {
			TopicID int64 `gorm:"column:topic_id"`
			Cnt     int64 `gorm:"column:cnt"`
		}
		if err := s.db.Table("forum_topic_like").Select("topic_id, COUNT(*) as cnt").Where("topic_id IN ?", ids).Group("topic_id").Scan(&counts).Error; err == nil {
			m := make(map[int64]int64, len(counts))
			for _, c := range counts {
				m[c.TopicID] = c.Cnt
			}
			for i := range items {
				if v, ok := m[items[i].ID]; ok {
					items[i].LikesCount = v
				}
			}
		}
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
	// 批量回填回复点赞数与当前用户是否已赞
	if len(replyDTOs) > 0 {
		ids := make([]int64, 0, len(replyDTOs))
		for _, rd := range replyDTOs {
			ids = append(ids, rd.ID)
		}
		var cnts []struct {
			ReplyID int64 `gorm:"column:reply_id"`
			Cnt     int64 `gorm:"column:cnt"`
		}
		if err := s.db.Table("forum_reply_like").Select("reply_id, COUNT(*) as cnt").Where("reply_id IN ?", ids).Group("reply_id").Scan(&cnts).Error; err == nil {
			m := make(map[int64]int64, len(cnts))
			for _, c := range cnts {
				m[c.ReplyID] = c.Cnt
			}
			for i := range replyDTOs {
				if v, ok := m[replyDTOs[i].ID]; ok {
					replyDTOs[i].LikesCount = v
				}
			}
		}
		if viewerID > 0 {
			var liked []int64
			if err := s.db.Model(&model.ForumReplyLike{}).Where("user_id = ? AND reply_id IN ?", viewerID, ids).Pluck("reply_id", &liked).Error; err == nil {
				lm := make(map[int64]bool, len(liked))
				for _, id := range liked {
					lm[id] = true
				}
				for i := range replyDTOs {
					replyDTOs[i].LikedByMe = lm[replyDTOs[i].ID]
				}
			}
		}
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
	var parentAuthorID int // 被回复人（楼中楼通知用）
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
		parentAuthorID = parent.UserID
		var pu model.HrwaiUser
		if err := s.db.First(&pu, parent.UserID).Error; err == nil {
			parentName = ForumAuthor{
				UserID: pu.ID, Username: pu.Username, AvatarURL: pu.AvatarURL,
			}.DisplayName()
		}
	}

	// 回复人展示名（通知文案用；查询失败回退空串，不阻断回复）
	var replier model.HrwaiUser
	_ = s.db.Select("username").First(&replier, userID).Error
	replierName := replier.Username

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
		if err := tx.Model(&model.ForumTopic{}).Where("id = ?", topicID).
			Updates(map[string]any{
				"reply_count":   gorm.Expr("reply_count + 1"),
				"last_reply_at": now,
				"updated_at":    now,
			}).Error; err != nil {
			return err
		}
		// 站内信通知（与回复同事务提交，避免通知丢失；与资料审核同模式）：
		// 1) 楼主被回复（回复人是楼主本人时不通知）
		// 2) 楼中楼被回复人（非自己、非楼主——楼主已由 1) 覆盖，避免重复通知）
		link := fmt.Sprintf("/training/forum/%d", topicID)
		payload := forumTopicPayload(topicID)
		if topic.UserID != userID {
			if err := s.notificationSvc.CreateWithTx(tx, topic.UserID, "forum_reply",
				"你的帖子有新回复",
				fmt.Sprintf("%s 回复了你的帖子「%s」", replierName, topic.Title),
				link, payload, now); err != nil {
				return err
			}
		}
		if parentAuthorID != 0 && parentAuthorID != userID && parentAuthorID != topic.UserID {
			if err := s.notificationSvc.CreateWithTx(tx, parentAuthorID, "forum_reply",
				"你的回复有新回复",
				fmt.Sprintf("%s 在帖子「%s」中回复了你", replierName, topic.Title),
				link, payload, now); err != nil {
				return err
			}
		}
		return nil
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

// AdminDeleteTopic 管理员删除任意主题（不校验作者）。图片一并清理；站内信通知作者。
func (s *ForumService) AdminDeleteTopic(topicID int64) error {
	var topic model.ForumTopic
	if err := s.db.First(&topic, topicID).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return errors.New("主题不存在")
		}
		return err
	}
	if err := s.deleteTopicWithImages(topicID); err != nil {
		return err
	}
	// 通知作者（尽力而为：内容已删，通知失败不回滚，仅记日志）
	if err := s.notificationSvc.Create(topic.UserID, "forum_topic_deleted",
		"你的帖子已被删除",
		"管理员删除了你的帖子「"+topic.Title+"」。",
		"", nil); err != nil {
		s.logger.Warn("删帖通知发送失败", zap.Int64("topic_id", topicID), zap.Error(err))
	}
	return nil
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

// AdminDeleteReply 管理员删除任意回复（不校验作者；其下级回复随外键级联删除）。图片一并清理；站内信通知回复作者。
func (s *ForumService) AdminDeleteReply(replyID int64) error {
	var reply model.ForumReply
	if err := s.db.First(&reply, replyID).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return errors.New("回复不存在")
		}
		return err
	}
	topicTitle := ""
	var topic model.ForumTopic
	if err := s.db.Select("title").First(&topic, reply.TopicID).Error; err == nil {
		topicTitle = topic.Title
	}
	if err := s.deleteReplyWithImages(replyID, reply.TopicID); err != nil {
		return err
	}
	// 通知回复作者（尽力而为：内容已删，通知失败不回滚，仅记日志）
	if err := s.notificationSvc.Create(reply.UserID, "forum_reply_deleted",
		"你的回复已被删除",
		"管理员删除了你在帖子「"+topicTitle+"」中的回复。",
		fmt.Sprintf("/training/forum/%d", reply.TopicID),
		forumTopicPayload(reply.TopicID)); err != nil {
		s.logger.Warn("删回复通知发送失败", zap.Int64("reply_id", replyID), zap.Error(err))
	}
	return nil
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

// HandleReport 管理端处理举报（status: 0 待处理 / 1 已处理）；标记已处理时站内信通知举报人。
func (s *ForumService) HandleReport(reportID int64, status int16) error {
	if status != 0 && status != 1 {
		return errors.New("状态仅支持 0（待处理）/ 1（已处理）")
	}
	var report model.ForumReport
	if err := s.db.First(&report, reportID).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return errors.New("举报不存在")
		}
		return err
	}
	if err := s.db.Model(&model.ForumReport{}).Where("id = ?", reportID).
		Update("status", status).Error; err != nil {
		return err
	}
	// 待处理 → 已处理时通知举报人（重复标记不重复通知；尽力而为，失败仅记日志）
	if status == 1 && report.Status != 1 {
		s.notifyReportHandled(&report)
	}
	return nil
}

// notifyReportHandled 举报处理完成站内信。举报对象可能已被删除：
// 主题已删时降级文案（不带标题与链接）。
func (s *ForumService) notifyReportHandled(report *model.ForumReport) {
	target := "帖子"
	if report.ReplyID != nil {
		target = "回复"
	}
	content := "你举报的" + target + "已处理完毕。"
	link := ""
	var payload model.JSONB
	if report.TopicID != nil {
		var topic model.ForumTopic
		if err := s.db.Select("title").First(&topic, *report.TopicID).Error; err == nil {
			content = fmt.Sprintf("你举报的%s「%s」已处理完毕。", target, topic.Title)
			link = fmt.Sprintf("/training/forum/%d", *report.TopicID)
			payload = forumTopicPayload(*report.TopicID)
		}
	}
	if err := s.notificationSvc.Create(report.ReporterID, "forum_report", "举报已处理", content, link, payload); err != nil {
		s.logger.Warn("举报处理通知发送失败", zap.Int64("report_id", report.ID), zap.Error(err))
	}
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
	if len(items) > 0 {
		ids := make([]int64, 0, len(items))
		for _, it := range items {
			ids = append(ids, it.ID)
		}
		var counts []struct {
			TopicID int64 `gorm:"column:topic_id"`
			Cnt     int64 `gorm:"column:cnt"`
		}
		if err := s.db.Table("forum_topic_like").Select("topic_id, COUNT(*) as cnt").Where("topic_id IN ?", ids).Group("topic_id").Scan(&counts).Error; err == nil {
			m := make(map[int64]int64, len(counts))
			for _, c := range counts {
				m[c.TopicID] = c.Cnt
			}
			for i := range items {
				if v, ok := m[items[i].ID]; ok {
					items[i].LikesCount = v
				}
			}
		}
		// 批量回填当前用户是否已赞，避免 N+1
		var likedIDs []int64
		if err := s.db.Model(&model.ForumTopicLike{}).Where("user_id = ? AND topic_id IN ?", userID, ids).Pluck("topic_id", &likedIDs).Error; err == nil {
			lm := make(map[int64]bool, len(likedIDs))
			for _, id := range likedIDs {
				lm[id] = true
			}
			for i := range items {
				items[i].LikedByMe = lm[items[i].ID]
			}
		}
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

// ===== 每日打卡（spec #268）=====

// CheckInResult 打卡结果。
type CheckInResult struct {
	Checked      bool `json:"checked"`
	Streak       int  `json:"streak"`
	Total        int  `json:"total"`
	TodayChecked bool `json:"today_checked"`
}

// CheckInCalendarResult 日历结果。
type CheckInCalendarResult struct {
	Dates        []string `json:"dates"`
	Streak       int      `json:"streak"`
	Total        int      `json:"total"`
	TodayChecked bool     `json:"today_checked"`
}

// CheckInRankItem 排行榜条目。
type CheckInRankItem struct {
	Rank         int         `json:"rank"`
	User         ForumAuthor `json:"user"`
	Total        int         `json:"total"`
	Streak       int         `json:"streak"`
	TodayChecked bool        `json:"today_checked"`
}

// CheckInRankResult 排行榜分页结果。
type CheckInRankResult struct {
	Items []CheckInRankItem `json:"items"`
	Total int64             `json:"total"`
	Page  int               `json:"page"`
	Pages int               `json:"pages"`
	Me    *CheckInRankItem  `json:"me"`
}

func beijingToday() time.Time {
	now := beijingNow()
	return time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
}

func (s *ForumService) checkInStreakAndTotal(userID int) (streak, total int, todayChecked bool) {
	var dates []time.Time
	if err := s.db.Model(&model.ForumCheckIn{}).Where("user_id = ?", userID).Order("check_date ASC").Pluck("check_date", &dates).Error; err != nil {
		return 0, 0, false
	}
	total = len(dates)
	if total == 0 {
		return 0, 0, false
	}
	set := make(map[string]bool, total)
	for _, d := range dates {
		// Normalize to Asia/Shanghai date string to avoid UTC shift.
		set[d.In(beijingNow().Location()).Format("2006-01-02")] = true
	}
	todayStr := beijingToday().Format("2006-01-02")
	todayChecked = set[todayStr]
	// 计算连续天数：从今天（若已签）或昨天（若今日未签）往前连续计数；若最近一天与起点间隔>1则 streak=0
	var cur time.Time
	if todayChecked {
		cur = beijingToday()
	} else {
		cur = beijingToday().AddDate(0, 0, -1)
		if !set[cur.Format("2006-01-02")] {
			return 0, total, false
		}
	}
	for {
		if set[cur.Format("2006-01-02")] {
			streak++
			cur = cur.AddDate(0, 0, -1)
		} else {
			break
		}
	}
	return streak, total, todayChecked
}

// CheckIn 每日打卡（幂等，Asia/Shanghai 自然日）。
func (s *ForumService) CheckIn(userID int) (*CheckInResult, error) {
	today := beijingToday()
	// 使用 DATE 类型比较时传入 today（00:00）
	var exists int64
	if err := s.db.Model(&model.ForumCheckIn{}).Where("user_id = ? AND check_date = ?", userID, today).Count(&exists).Error; err != nil {
		return nil, err
	}
	if exists == 0 {
		if err := s.db.Create(&model.ForumCheckIn{UserID: userID, CheckDate: today, CreatedAt: beijingNow()}).Error; err != nil {
			// 唯一冲突视为已签（并发幂等）
			if strings.Contains(err.Error(), "duplicate") || strings.Contains(err.Error(), "uq_") || strings.Contains(err.Error(), "pk_forum_checkin") || strings.Contains(err.Error(), "UNIQUE") {
				// ignore
			} else {
				return nil, err
			}
		}
	}
	streak, total, todayChecked := s.checkInStreakAndTotal(userID)
	return &CheckInResult{Checked: true, Streak: streak, Total: total, TodayChecked: todayChecked}, nil
}

// GetCheckInCalendar 获取某月已签日期及统计。
func (s *ForumService) GetCheckInCalendar(userID, year, month int) (*CheckInCalendarResult, error) {
	if year < 2000 || year > 2100 {
		return nil, errors.New("年份无效")
	}
	if month < 1 || month > 12 {
		return nil, errors.New("月份无效")
	}
	loc := beijingNow().Location()
	first := time.Date(year, time.Month(month), 1, 0, 0, 0, 0, loc)
	// last day: first of next month -1 day
	last := first.AddDate(0, 1, -1)
	var dates []time.Time
	if err := s.db.Model(&model.ForumCheckIn{}).Where("user_id = ? AND check_date BETWEEN ? AND ?", userID, first, last).Order("check_date ASC").Pluck("check_date", &dates).Error; err != nil {
		return nil, err
	}
	strs := make([]string, 0, len(dates))
	for _, d := range dates {
		strs = append(strs, d.In(loc).Format("2006-01-02"))
	}
	streak, total, todayChecked := s.checkInStreakAndTotal(userID)
	return &CheckInCalendarResult{Dates: strs, Streak: streak, Total: total, TodayChecked: todayChecked}, nil
}

// GetCheckInRank 排行榜（累计总榜，total DESC, last_date ASC）。
func (s *ForumService) GetCheckInRank(requesterID, page, pageSize int) (*CheckInRankResult, error) {
	if page <= 0 {
		page = 1
	}
	if pageSize <= 0 || pageSize > 100 {
		pageSize = 20
	}
	type aggRow struct {
		UserID   int       `gorm:"column:user_id"`
		Total    int       `gorm:"column:total"`
		LastDate time.Time `gorm:"column:last_date"`
	}
	var totalUsers int64
	if err := s.db.Table("(SELECT user_id FROM forum_checkin GROUP BY user_id) AS t").Count(&totalUsers).Error; err != nil {
		return nil, err
	}
	pages := response.PageCount(totalUsers, pageSize)
	if page > pages && pages > 0 {
		page = pages
	}
	offset := (page - 1) * pageSize
	var rows []aggRow
	if err := s.db.Table("forum_checkin").Select("user_id, COUNT(*) as total, MAX(check_date) as last_date").Group("user_id").Order("total DESC, last_date ASC, user_id ASC").Limit(pageSize).Offset(offset).Scan(&rows).Error; err != nil {
		return nil, err
	}
	// 批量取用户信息
	userIDs := make([]int, 0, len(rows))
	for _, r := range rows {
		userIDs = append(userIDs, r.UserID)
	}
	userMap := make(map[int]model.HrwaiUser)
	if len(userIDs) > 0 {
		var users []model.HrwaiUser
		if err := s.db.Where("id IN ?", userIDs).Find(&users).Error; err == nil {
			for _, u := range users {
				userMap[u.ID] = u
			}
		}
	}
	// 为每行计算 streak / today_checked（小批量，每行单独查 checkin 集合，pageSize 20 可接受）
	items := make([]CheckInRankItem, 0, len(rows))
	for i, r := range rows {
		u := userMap[r.UserID]
		streak, _, todayChecked := s.checkInStreakAndTotal(r.UserID)
		items = append(items, CheckInRankItem{
			Rank:         offset + i + 1,
			User:         ForumAuthor{UserID: r.UserID, Username: u.Username, AvatarURL: u.AvatarURL},
			Total:        r.Total,
			Streak:       streak,
			TodayChecked: todayChecked,
		})
	}
	// 当前用户排名（若不在当页仍需计算）
	var me *CheckInRankItem
	if requesterID > 0 {
		var myTotal int64
		s.db.Model(&model.ForumCheckIn{}).Where("user_id = ?", requesterID).Count(&myTotal)
		if myTotal > 0 {
			var myLast time.Time
			s.db.Model(&model.ForumCheckIn{}).Where("user_id = ?", requesterID).Select("MAX(check_date)").Scan(&myLast)
			// 计算排名：total 更高者数量 + 同 total 但 last_date 更早 + 同 total 同 last_date 但 user_id 更小 +1
			var rank int64
			// 先 count users with total > myTotal
			var higher int64
			s.db.Table("(SELECT user_id, COUNT(*) as total, MAX(check_date) as last_date FROM forum_checkin GROUP BY user_id) AS t").Where("total > ?", myTotal).Count(&higher)
			// 再 count users with total == myTotal and (last_date < myLast OR (last_date == myLast AND user_id < requesterID))
			var sameRank int64
			s.db.Table("(SELECT user_id, COUNT(*) as total, MAX(check_date) as last_date FROM forum_checkin GROUP BY user_id) AS t").Where("total = ? AND (last_date < ? OR (last_date = ? AND user_id < ?))", myTotal, myLast, myLast, requesterID).Count(&sameRank)
			rank = higher + sameRank + 1
			var mu model.HrwaiUser
			_ = s.db.First(&mu, requesterID).Error
			streak, _, todayChecked := s.checkInStreakAndTotal(requesterID)
			me = &CheckInRankItem{
				Rank:         int(rank),
				User:         ForumAuthor{UserID: requesterID, Username: mu.Username, AvatarURL: mu.AvatarURL},
				Total:        int(myTotal),
				Streak:       streak,
				TodayChecked: todayChecked,
			}
		}
	}
	return &CheckInRankResult{Items: items, Total: totalUsers, Page: page, Pages: pages, Me: me}, nil
}

// LikeReply 点赞评论（幂等）。
func (s *ForumService) LikeReply(userID int, replyID int64) (int64, error) {
	var cnt int64
	if err := s.db.Model(&model.ForumReply{}).Where("id = ?", replyID).Count(&cnt).Error; err != nil {
		return 0, err
	}
	if cnt == 0 {
		return 0, errors.New("回复不存在")
	}
	var existing model.ForumReplyLike
	if err := s.db.Where("reply_id = ? AND user_id = ?", replyID, userID).Limit(1).Find(&existing).Error; err != nil {
		return 0, err
	}
	if existing.ID == 0 {
		if err := s.db.Create(&model.ForumReplyLike{ReplyID: replyID, UserID: userID, CreatedAt: beijingNow()}).Error; err != nil {
			if strings.Contains(err.Error(), "duplicate") || strings.Contains(err.Error(), "UNIQUE") || strings.Contains(err.Error(), "uq_forum_reply_like") {
				// 并发幂等：已被其他请求创建
			} else {
				return 0, err
			}
		}
	}
	return s.replyLikesCount(replyID), nil
}

// UnlikeReply 取消点赞评论（幂等）。
func (s *ForumService) UnlikeReply(userID int, replyID int64) (int64, error) {
	if err := s.db.Where("reply_id = ? AND user_id = ?", replyID, userID).Delete(&model.ForumReplyLike{}).Error; err != nil {
		return 0, err
	}
	return s.replyLikesCount(replyID), nil
}

func (s *ForumService) replyLikesCount(replyID int64) int64 {
	var n int64
	_ = s.db.Model(&model.ForumReplyLike{}).Where("reply_id = ?", replyID).Count(&n).Error
	return n
}
