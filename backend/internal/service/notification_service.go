// Package service 实现业务服务层。
// 本文件：站内信通知（P0 通知基础设施，当前仅站内信渠道）。
package service

import (
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"go.uber.org/zap"
	"gorm.io/gorm"

	"forklift-training/internal/model"
	"forklift-training/pkg/paging"
	"forklift-training/pkg/response"
)

// NotificationDTO 站内信通知展示对象。
// Payload 为结构化业务标记（JSONB，加性字段，如 review_status），旧契约字段不变。
type NotificationDTO struct {
	ID        int64       `json:"id"`
	Type      string      `json:"type"`
	Title     string      `json:"title"`
	Content   string      `json:"content"`
	Link      string      `json:"link"`
	Payload   model.JSONB `json:"payload,omitempty"`
	IsRead    bool        `json:"is_read"`
	CreatedAt string      `json:"created_at"`
	ReadAt    *string     `json:"read_at,omitempty"`
}

// GormCreator 通知写入执行器（*gorm.DB 与 *gorm.Tx 均满足，事务内写入用）。
type GormCreator interface {
	Create(value interface{}) *gorm.DB
}

// NotificationService 站内信通知服务。
type NotificationService struct {
	db *gorm.DB

	logger *zap.Logger
}

// NewNotificationService 构造通知服务。
func NewNotificationService(db *gorm.DB, logger *zap.Logger) *NotificationService {
	return &NotificationService{db: db, logger: logger}
}

// Create 创建一条站内信通知（payload 为可选结构化标记，nil 表示无）。
func (s *NotificationService) Create(userID int, typ, title, content, link string, payload model.JSONB) error {
	return s.CreateWithTx(s.db, userID, typ, title, content, link, payload, time.Now())
}

// CreateWithTx 在指定事务/连接内创建站内信。
// 业务事件（如资料审核）与业务写同事务提交，避免通知丢失；createdAt 由调用方控制时区语义。
func (s *NotificationService) CreateWithTx(tx GormCreator, userID int, typ, title, content, link string, payload model.JSONB, createdAt time.Time) error {
	n := model.Notification{
		UserID:    userID,
		Type:      typ,
		Title:     title,
		Content:   content,
		Link:      link,
		Payload:   payload,
		CreatedAt: createdAt,
	}
	return tx.Create(&n).Error
}

// ProfileReviewNotification 构造资料审核结果站内信参数（type=profile_review）。
// 返回 (type, title, content, payload)：标题保持人读文案；payload 为结构化判定标记
// （如 {"review_status":"approved"|"rejected"}），供前端确定性消费，不依赖标题文案。
func (s *NotificationService) ProfileReviewNotification(req *model.ProfileChangeRequest, status, reason string) (typ, title, content string, payload model.JSONB) {
	fieldLabel := "昵称"
	if req.FieldType == ProfileFieldAvatar {
		fieldLabel = "头像"
	}
	title = "资料审核通过"
	content = "您的" + fieldLabel + "修改已通过审核，修改已生效。"
	payload = reviewStatusPayload(ProfileStatusApproved)
	if status == ProfileStatusRejected {
		title = "资料审核被驳回"
		content = "您的" + fieldLabel + "修改申请未通过审核"
		if reason != "" {
			content += "，原因：" + reason
		}
		content += "。"
		payload = reviewStatusPayload(ProfileStatusRejected)
	}
	return "profile_review", title, content, payload
}

// 论坛采纳通知类型（#369）。
const (
	NotifTypeForumAcceptAnswerer = "forum_accept_answerer" // 答主：你的回答被采纳 +40
	NotifTypeForumAcceptOwner    = "forum_accept_owner"    // 楼主：你采纳了答案 +5
)

// forumTopicPayload 构造论坛事件通知结构化标记（{"topic_id": N}），
// 供前端/移动端确定性定位帖子（不依赖 link 文案解析）。
func forumTopicPayload(topicID int64) model.JSONB {
	b, err := json.Marshal(struct {
		TopicID int64 `json:"topic_id"`
	}{TopicID: topicID})
	if err != nil {
		return nil
	}
	return model.JSONB(b)
}

// forumAcceptPayload 构造采纳事件结构化标记（topic_id + reply_id + points + reason），
// 加性扩展 forumTopicPayload，不依赖标题文案判定（#369）。
func forumAcceptPayload(topicID, replyID int64, points int, reason string) model.JSONB {
	b, err := json.Marshal(struct {
		TopicID int64  `json:"topic_id"`
		ReplyID int64  `json:"reply_id"`
		Points  int    `json:"points"`
		Reason  string `json:"reason"`
	}{TopicID: topicID, ReplyID: replyID, Points: points, Reason: reason})
	if err != nil {
		return nil
	}
	return model.JSONB(b)
}

// ForumAcceptEvent 问答采纳事件通知参数（ADR-0024 C3）：站内信域单点构造
// title/content/link/payload 口径，业务侧一行触发。
type ForumAcceptEvent struct {
	// UserID 收件人（答主或楼主）。
	UserID int
	// Type 通知类型：NotifTypeForumAcceptAnswerer / NotifTypeForumAcceptOwner。
	Type string
	// TopicTitle 问答标题（用于文案）。
	TopicTitle string
	// TopicID 主题 ID。
	TopicID int64
	// ReplyID 被采纳回复 ID。
	ReplyID int64
	// Points 到账分值（与实际入账一致）。
	Points int
	// Reason 流水原因（ReasonAcceptedBonus / ReasonAcceptAction）。
	Reason string
}

// answererAcceptTitle 答主被采纳标题。
const answererAcceptTitle = "你的回答被采纳"

// ownerAcceptTitle 楼主采纳动作标题。
const ownerAcceptTitle = "你采纳了答案"

// NewAnswererAcceptEvent 构造答主被采纳通知事件（+40 分到账，link 锚到回答）。
func NewAnswererAcceptEvent(userID int, topicTitle string, topicID, replyID int64, points int) ForumAcceptEvent {
	return ForumAcceptEvent{
		UserID:     userID,
		Type:       NotifTypeForumAcceptAnswerer,
		TopicTitle: topicTitle,
		TopicID:    topicID,
		ReplyID:    replyID,
		Points:     points,
		Reason:     ReasonAcceptedBonus,
	}
}

// NewOwnerAcceptEvent 构造楼主采纳动作通知事件（+5 分到账，link 锚到回答）。
func NewOwnerAcceptEvent(userID int, topicTitle string, topicID, replyID int64, points int) ForumAcceptEvent {
	return ForumAcceptEvent{
		UserID:     userID,
		Type:       NotifTypeForumAcceptOwner,
		TopicTitle: topicTitle,
		TopicID:    topicID,
		ReplyID:    replyID,
		Points:     points,
		Reason:     ReasonAcceptAction,
	}
}

// CreateForumAcceptEvent 在指定事务/连接内创建一条问答采纳事件站内信。
// 与积分入账同事务提交/回滚（ADR-0023）：通知与到账积分一致。
func (s *NotificationService) CreateForumAcceptEvent(tx GormCreator, ev ForumAcceptEvent, createdAt time.Time) error {
	link := fmt.Sprintf("/training/forum/%d#reply-%d", ev.TopicID, ev.ReplyID)
	title := answererAcceptTitle
	if ev.Type == NotifTypeForumAcceptOwner {
		title = ownerAcceptTitle
	}
	content := fmt.Sprintf("你在问答「%s」中的回答被采纳，+%d 分已到账", ev.TopicTitle, ev.Points)
	if ev.Type == NotifTypeForumAcceptOwner {
		content = fmt.Sprintf("你在问答「%s」中采纳了回答，+%d 分已到账", ev.TopicTitle, ev.Points)
	}
	return s.CreateWithTx(tx, ev.UserID, ev.Type, title, content, link, forumAcceptPayload(ev.TopicID, ev.ReplyID, ev.Points, ev.Reason), createdAt)
}

// reviewStatusPayload 构造审核状态结构化标记，如 {"review_status":"approved"}。
func reviewStatusPayload(reviewStatus string) model.JSONB {
	b, err := json.Marshal(struct {
		ReviewStatus string `json:"review_status"`
	}{ReviewStatus: reviewStatus})
	if err != nil {
		return nil
	}
	return model.JSONB(b)
}

// NotificationListPageResult 站内信分页结果（含未读数）。
type NotificationListPageResult struct {
	Items       []NotificationDTO `json:"items"`
	Page        int               `json:"page"`
	Pages       int               `json:"pages"`
	Total       int64             `json:"total"`
	UnreadCount int64             `json:"unread_count"`
}

// List 分页查询当前用户通知，并附带未读数（一次请求同时支撑列表与角标）。
func (s *NotificationService) List(userID int, page, pageSize int) (*NotificationListPageResult, error) {
	var unread int64
	if err := s.db.Model(&model.Notification{}).
		Where("user_id = ? AND is_read = ?", userID, false).Count(&unread).Error; err != nil {
		return nil, err
	}

	rows, total, page, pageSize := paging.QueryWithScan[model.Notification](s.db, page, pageSize, 10, 50,
		"id DESC",
		func(q *gorm.DB) *gorm.DB {
			return q.Model(&model.Notification{}).Where("user_id = ?", userID)
		})

	items := make([]NotificationDTO, 0, len(rows))
	for i := range rows {
		items = append(items, toNotificationDTO(&rows[i]))
	}
	return &NotificationListPageResult{
		Items:       items,
		Page:        page,
		Pages:       response.PageCount(total, pageSize),
		Total:       total,
		UnreadCount: unread,
	}, nil
}

// UnreadCount 查询当前用户未读通知数。
func (s *NotificationService) UnreadCount(userID int) (int64, error) {
	var count int64
	err := s.db.Model(&model.Notification{}).
		Where("user_id = ? AND is_read = ?", userID, false).
		Count(&count).Error
	return count, err
}

// MarkRead 将单条通知标记为已读（仅限本人；已读或不存在均幂等成功，非本人报错）。
func (s *NotificationService) MarkRead(userID int, id int64) error {
	res := s.db.Model(&model.Notification{}).
		Where("id = ? AND user_id = ? AND is_read = ?", id, userID, false).
		Updates(map[string]any{"is_read": true, "read_at": time.Now()})
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected > 0 {
		return nil
	}
	// 未更新：校验是否属于本人（不存在或已读都返回成功，保持幂等）
	var count int64
	if err := s.db.Model(&model.Notification{}).
		Where("id = ? AND user_id = ?", id, userID).Count(&count).Error; err != nil {
		return err
	}
	if count == 0 {
		return errors.New("通知不存在")
	}
	return nil
}

// MarkAllRead 将当前用户全部未读通知标记为已读。
func (s *NotificationService) MarkAllRead(userID int) error {
	return s.db.Model(&model.Notification{}).
		Where("user_id = ? AND is_read = ?", userID, false).
		Updates(map[string]any{"is_read": true, "read_at": time.Now()}).Error
}

// toNotificationDTO 组装展示对象。
func toNotificationDTO(n *model.Notification) NotificationDTO {
	return NotificationDTO{
		ID:        n.ID,
		Type:      n.Type,
		Title:     n.Title,
		Content:   n.Content,
		Link:      n.Link,
		Payload:   n.Payload,
		IsRead:    n.IsRead,
		CreatedAt: formatISO(n.CreatedAt),
		ReadAt:    formatTimePtr(n.ReadAt),
	}
}
