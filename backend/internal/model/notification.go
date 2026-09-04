// 通知与审计域（CONTEXT.md「通知与审计」）。
package model

import "time"

// ===== 26. 站内信通知 =====

// Notification 站内信通知（P0 通知基础设施，当前仅站内信渠道）。
// Payload 为结构化业务标记（JSONB，如资料审核 {"review_status":"approved"}），加性扩展，不参与人读文案。
type Notification struct {
	ID        int64      `gorm:"column:id;primaryKey;autoIncrement" json:"id"`
	UserID    int        `gorm:"column:user_id" json:"user_id"`
	Type      string     `gorm:"column:type;default:system" json:"type"`
	Title     string     `gorm:"column:title" json:"title"`
	Content   string     `gorm:"column:content" json:"content"`
	Link      string     `gorm:"column:link" json:"link"`
	Payload   JSONB      `gorm:"column:payload;type:jsonb" json:"payload,omitempty"`
	IsRead    bool       `gorm:"column:is_read;default:false" json:"is_read"`
	CreatedAt time.Time  `gorm:"column:created_at" json:"created_at"`
	ReadAt    *time.Time `gorm:"column:read_at" json:"read_at,omitempty"`
}

func (Notification) TableName() string { return "notifications" }

// ===== 27. 审计日志 =====

// AuditLog 管理员/讲师关键操作审计日志。
type AuditLog struct {
	ID        int64     `gorm:"column:id;primaryKey;autoIncrement" json:"id"`
	ActorID   int       `gorm:"column:actor_id" json:"actor_id"`
	ActorRole string    `gorm:"column:actor_role" json:"actor_role"`
	ActorName string    `gorm:"column:actor_name" json:"actor_name"`
	Action    string    `gorm:"column:action" json:"action"`
	Path      string    `gorm:"column:path" json:"path"`
	Method    string    `gorm:"column:method" json:"method"`
	RequestID string    `gorm:"column:request_id" json:"request_id"`
	IP        string    `gorm:"column:ip" json:"ip"`
	Status    int       `gorm:"column:status" json:"status"`
	Detail    JSONB     `gorm:"column:detail;type:jsonb" json:"detail,omitempty"`
	CreatedAt time.Time `gorm:"column:created_at" json:"created_at"`
}

func (AuditLog) TableName() string { return "audit_logs" }
