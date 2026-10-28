// 反馈域（ADR-0089）：学员面向平台的单向消息。
package model

import "time"

// 反馈两态（ADR-0089 决策 1）：没有第三态、没有流转历史、没有工单号。
const (
	FeedbackStatusPending = "pending"
	FeedbackStatusHandled = "handled"
)

// Feedback 学员提交给平台的一条反馈（ADR-0089）。
//
// 与相邻四面的边界：不是公开内容（论坛）、不是下发通道（站内信）、不是编辑内容（内容精选）、
// 不进治理队列（举报）。图片与论坛图片同前缀 images/forum/（ADR-0066），故本表里的 URL
// 必须被论坛悬空图片清理算进引用集——见 internal/forum/image_service.go 的 collectReferencedImages。
type Feedback struct {
	ID      int64  `gorm:"column:id;primaryKey;autoIncrement" json:"id"`
	UserID  int    `gorm:"column:user_id;index" json:"user_id"`
	Content string `gorm:"column:content" json:"content"`
	// Images 图片 URL 数组（0–3 张，仅本站 images/forum/ 前缀）；空集落 '[]'，读出即 []。
	Images  JSONB  `gorm:"column:images;type:jsonb;default:'[]'" json:"images"`
	Contact string `gorm:"column:contact;default:''" json:"contact"`
	Status  string `gorm:"column:status;default:pending" json:"status"`
	// HandledAt / HandledBy 处置留痕：离开 pending 的那一刻写一次，之后不再变（标记是幂等的）。
	HandledAt *time.Time `gorm:"column:handled_at" json:"handled_at,omitempty"`
	HandledBy *int       `gorm:"column:handled_by" json:"handled_by,omitempty"`
	CreatedAt time.Time  `gorm:"column:created_at" json:"created_at"`
	UpdatedAt time.Time  `gorm:"column:updated_at" json:"updated_at"`
}

func (Feedback) TableName() string { return "feedback" }
