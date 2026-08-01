// Package service 实现业务服务层。
// 本文件：学员端论坛（综合讨论区 + 章节讨论区，支持回复别人的回复）。
package service

import (
	"errors"
	"strings"
	"time"
	"unicode/utf8"

	"gorm.io/gorm"

	"forklift-training/internal/model"
)

// 论坛范围常量。
const (
	ForumScopeAll     = "all"     // 全部（综合讨论区 + 章节讨论区）
	ForumScopeGeneral = "general" // 综合讨论区（chapter_id IS NULL）
	ForumScopeChapter = "chapter" // 指定章节讨论区
)

// ForumAuthor 论坛作者信息（昵称优先，其次姓名/用户名）。
type ForumAuthor struct {
	UserID    int    `json:"user_id"`
	Username  string `json:"username"`
	Name      string `json:"name"`
	Nickname  string `json:"nickname"`
	AvatarURL string `json:"avatar_url"`
}

// DisplayName 返回论坛展示名：昵称 > 姓名 > 用户名。
func (a ForumAuthor) DisplayName() string {
	if s := strings.TrimSpace(a.Nickname); s != "" {
		return s
	}
	if s := strings.TrimSpace(a.Name); s != "" {
		return s
	}
	return a.Username
}

// ForumTopicDTO 论坛主题列表/详情对象。
type ForumTopicDTO struct {
	ID           int64       `json:"id"`
	ChapterID    *int        `json:"chapter_id"`
	ChapterTitle string      `json:"chapter_title"`
	Title        string      `json:"title"`
	Content      string      `json:"content"`
	ViewCount    int         `json:"view_count"`
	ReplyCount   int         `json:"reply_count"`
	LastReplyAt  *string     `json:"last_reply_at"`
	CreatedAt    string      `json:"created_at"`
	Author       ForumAuthor `json:"author"`
	CanDelete    bool        `json:"can_delete"`
}

// ForumReplyDTO 论坛回复对象。
type ForumReplyDTO struct {
	ID         int64       `json:"id"`
	TopicID    int64       `json:"topic_id"`
	ParentID   *int64      `json:"parent_id,omitempty"`
	ParentName string      `json:"parent_name,omitempty"` // 被回复人的展示名
	Content    string      `json:"content"`
	CreatedAt  string      `json:"created_at"`
	Author     ForumAuthor `json:"author"`
	CanDelete  bool        `json:"can_delete"`
}

// ForumService 论坛服务。
type ForumService struct {
	db *gorm.DB
}

// NewForumService 构造论坛服务。
func NewForumService(db *gorm.DB) *ForumService {
	return &ForumService{db: db}
}

// topicRow 列表查询的扫描结构。
type topicRow struct {
	ID           int64
	ChapterID    *int
	ChapterTitle string
	Title        string
	Content      string
	ViewCount    int
	ReplyCount   int
	LastReplyAt  *time.Time
	CreatedAt    time.Time
	UserID       int
	Username     string
	Name         string
	Nickname     string
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
		ViewCount:    r.ViewCount,
		ReplyCount:   r.ReplyCount,
		LastReplyAt:  lastReplyAt,
		CreatedAt:    formatISO(r.CreatedAt),
		Author: ForumAuthor{
			UserID: r.UserID, Username: r.Username,
			Name: r.Name, Nickname: r.Nickname, AvatarURL: r.AvatarURL,
		},
		CanDelete: r.UserID == viewerID,
	}
}

// ListTopics 分页查询主题。
// scope: all（默认）/ general（综合讨论区）/ chapter（需配合 chapterID）。
func (s *ForumService) ListTopics(scope string, chapterID, page, pageSize int, keyword string) (map[string]any, error) {
	if scope == "" {
		scope = ForumScopeAll
	}
	if page <= 0 {
		page = 1
	}
	if pageSize <= 0 || pageSize > 100 {
		pageSize = 10
	}

	q := s.db.Table("forum_topics AS t").
		Select("t.id, t.chapter_id, t.title, t.content, t.view_count, t.reply_count, t.last_reply_at, t.created_at, " +
			"u.id AS user_id, u.username, u.name, u.nickname, u.avatar_url, " +
			"COALESCE(ch.title, '') AS chapter_title").
		Joins("JOIN hrwai_users AS u ON u.id = t.user_id").
		Joins("LEFT JOIN chapter AS ch ON ch.chapter_id = t.chapter_id")

	switch scope {
	case ForumScopeGeneral:
		q = q.Where("t.chapter_id IS NULL")
	case ForumScopeChapter:
		if chapterID <= 0 {
			return nil, errors.New("查询章节讨论区需要有效的 chapter_id")
		}
		q = q.Where("t.chapter_id = ?", chapterID)
	}
	if keyword = strings.TrimSpace(keyword); keyword != "" {
		like := "%" + keyword + "%"
		q = q.Where("(t.title ILIKE ? OR t.content ILIKE ?)", like, like)
	}

	var total int64
	if err := q.Count(&total).Error; err != nil {
		return nil, err
	}

	var rows []topicRow
	if err := q.Order("COALESCE(t.last_reply_at, t.created_at) DESC, t.id DESC").
		Offset((page - 1) * pageSize).Limit(pageSize).
		Scan(&rows).Error; err != nil {
		return nil, err
	}

	items := make([]ForumTopicDTO, 0, len(rows))
	for _, r := range rows {
		items = append(items, r.toDTO(0))
	}
	pages := int((total + int64(pageSize) - 1) / int64(pageSize))
	return map[string]any{
		"total":  total,
		"page":   page,
		"pages":  pages,
		"topics": items,
	}, nil
}

// GetTopic 主题详情（含回复，回复带被回复人信息），并累加浏览量。
func (s *ForumService) GetTopic(topicID int64, viewerID int) (map[string]any, error) {
	var row topicRow
	err := s.db.Table("forum_topics AS t").
		Select("t.id, t.chapter_id, t.title, t.content, t.view_count, t.reply_count, t.last_reply_at, t.created_at, "+
			"u.id AS user_id, u.username, u.name, u.nickname, u.avatar_url, "+
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
		CreatedAt  time.Time
		UserID     int
		Username   string
		Name       string
		Nickname   string
		AvatarURL  string
		ParentName string
	}
	if err := s.db.Table("forum_replies AS r").
		Select("r.id, r.topic_id, r.parent_id, r.content, r.created_at, "+
			"u.id AS user_id, u.username, u.name, u.nickname, u.avatar_url, "+
			"COALESCE(NULLIF(pu.nickname, ''), NULLIF(pu.name, ''), pu.username, '') AS parent_name").
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
			Content: r.Content, CreatedAt: formatISO(r.CreatedAt),
			Author: ForumAuthor{
				UserID: r.UserID, Username: r.Username,
				Name: r.Name, Nickname: r.Nickname, AvatarURL: r.AvatarURL,
			},
			CanDelete: r.UserID == viewerID,
		})
	}

	return map[string]any{
		"topic":   row.toDTO(viewerID),
		"replies": replyDTOs,
	}, nil
}

// CreateTopic 发帖。chapterID 为 nil/0 表示发到综合讨论区。
func (s *ForumService) CreateTopic(userID int, chapterID *int, title, content string) (*ForumTopicDTO, error) {
	title = strings.TrimSpace(title)
	content = strings.TrimSpace(content)
	if utf8.RuneCountInString(title) < 1 || utf8.RuneCountInString(title) > 100 {
		return nil, errors.New("标题长度需在 1-100 个字符之间")
	}
	if utf8.RuneCountInString(content) < 1 || utf8.RuneCountInString(content) > 10000 {
		return nil, errors.New("内容长度需在 1-10000 个字符之间")
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
		CreatedAt: formatISO(topic.CreatedAt),
		Author: ForumAuthor{
			UserID: u.ID, Username: u.Username,
			Name: u.Name, Nickname: u.Nickname, AvatarURL: u.AvatarURL,
		},
		CanDelete: true,
	}, nil
}

// ReplyTopic 回复主题或回复某条回复（parentReplyID 非空时）。
func (s *ForumService) ReplyTopic(userID int, topicID int64, content string, parentReplyID *int64) (*ForumReplyDTO, error) {
	content = strings.TrimSpace(content)
	if utf8.RuneCountInString(content) < 1 || utf8.RuneCountInString(content) > 5000 {
		return nil, errors.New("回复内容长度需在 1-5000 个字符之间")
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
				UserID: pu.ID, Username: pu.Username,
				Name: pu.Name, Nickname: pu.Nickname, AvatarURL: pu.AvatarURL,
			}.DisplayName()
		}
	}

	now := beijingNow()
	reply := model.ForumReply{
		TopicID:   topicID,
		UserID:    userID,
		ParentID:  parentReplyID,
		Content:   content,
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
		ParentName: parentName, Content: reply.Content, CreatedAt: formatISO(reply.CreatedAt),
		Author: ForumAuthor{
			UserID: u.ID, Username: u.Username,
			Name: u.Name, Nickname: u.Nickname, AvatarURL: u.AvatarURL,
		},
		CanDelete: true,
	}, nil
}

// DeleteTopic 删除主题（仅作者本人）。
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
	return s.db.Delete(&model.ForumTopic{}, topicID).Error
}

// DeleteReply 删除回复（仅作者本人；其下级回复随外键级联删除）。
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

	return s.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Delete(&model.ForumReply{}, replyID).Error; err != nil {
			return err
		}
		// 回扣回复数并刷新最后回复时间
		if err := tx.Model(&model.ForumTopic{}).Where("id = ?", reply.TopicID).
			UpdateColumn("reply_count", gorm.Expr("GREATEST(reply_count - 1, 0)")).Error; err != nil {
			return err
		}
		var last model.ForumReply
		if err := tx.Where("topic_id = ?", reply.TopicID).Order("created_at DESC, id DESC").
			Limit(1).Find(&last).Error; err != nil {
			return err
		}
		var lastAt *time.Time
		if last.ID > 0 {
			lastAt = &last.CreatedAt
		}
		return tx.Model(&model.ForumTopic{}).Where("id = ?", reply.TopicID).
			Update("last_reply_at", lastAt).Error
	})
}
