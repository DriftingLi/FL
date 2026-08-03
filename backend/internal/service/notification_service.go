// Package service 实现业务服务层。
// 本文件：站内信通知（P0 通知基础设施，当前仅站内信渠道）。
package service

import (
	"errors"
	"time"

	"gorm.io/gorm"

	"forklift-training/internal/model"
)

// NotificationDTO 站内信通知展示对象。
type NotificationDTO struct {
	ID        int64   `json:"id"`
	Type      string  `json:"type"`
	Title     string  `json:"title"`
	Content   string  `json:"content"`
	Link      string  `json:"link"`
	IsRead    bool    `json:"is_read"`
	CreatedAt string  `json:"created_at"`
	ReadAt    *string `json:"read_at,omitempty"`
}

// NotificationService 站内信通知服务。
type NotificationService struct {
	db *gorm.DB
}

// NewNotificationService 构造通知服务。
func NewNotificationService(db *gorm.DB) *NotificationService {
	return &NotificationService{db: db}
}

// Create 创建一条站内信通知。
func (s *NotificationService) Create(userID int, typ, title, content, link string) error {
	n := model.Notification{
		UserID:    userID,
		Type:      typ,
		Title:     title,
		Content:   content,
		Link:      link,
		CreatedAt: time.Now(),
	}
	return s.db.Create(&n).Error
}

// List 分页查询当前用户通知，并附带未读数（一次请求同时支撑列表与角标）。
func (s *NotificationService) List(userID int, page, pageSize int) (map[string]any, error) {
	if page <= 0 {
		page = 1
	}
	if pageSize <= 0 || pageSize > 50 {
		pageSize = 10
	}

	var total int64
	if err := s.db.Model(&model.Notification{}).
		Where("user_id = ?", userID).Count(&total).Error; err != nil {
		return nil, err
	}
	var unread int64
	if err := s.db.Model(&model.Notification{}).
		Where("user_id = ? AND is_read = ?", userID, false).Count(&unread).Error; err != nil {
		return nil, err
	}

	var rows []model.Notification
	if err := s.db.Where("user_id = ?", userID).
		Order("id DESC").
		Offset((page - 1) * pageSize).
		Limit(pageSize).
		Find(&rows).Error; err != nil {
		return nil, err
	}

	items := make([]NotificationDTO, 0, len(rows))
	for i := range rows {
		items = append(items, toNotificationDTO(&rows[i]))
	}
	pages := int((total + int64(pageSize) - 1) / int64(pageSize))
	return map[string]any{
		"total":        total,
		"page":         page,
		"pages":        pages,
		"unread_count": unread,
		"items":        items,
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
		IsRead:    n.IsRead,
		CreatedAt: formatISO(n.CreatedAt),
		ReadAt:    formatTimePtr(n.ReadAt),
	}
}
