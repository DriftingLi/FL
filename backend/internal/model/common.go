// Package model 定义全部 GORM 数据模型，与 migrations/000001_init.up.sql 一一对应。
// 按领域词汇表（CONTEXT.md）分文件：common（共用件）/ account / training /
// question（题库练习）/ forum / recruiting / points / contribution / ai / notification（ADR-0027 C3）。
package model

import (
	"database/sql/driver"
	"encoding/json"
	"errors"
	"time"
)

// JSONB 是 PostgreSQL JSONB 字段的 Go 映射，支持任意 JSON 值。
type JSONB json.RawMessage

// Scan 实现 sql.Scanner。
func (j *JSONB) Scan(value any) error {
	if value == nil {
		*j = nil
		return nil
	}
	switch v := value.(type) {
	case []byte:
		*j = append((*j)[:0], v...)
		return nil
	case string:
		*j = JSONB(v)
		return nil
	}
	return errors.New("JSONB.Scan: 不支持的类型")
}

// Value 实现 driver.Valuer。
func (j JSONB) Value() (driver.Value, error) {
	if len(j) == 0 {
		return nil, nil
	}
	return []byte(j), nil
}

// MarshalJSON 实现 json.Marshaler。
func (j JSONB) MarshalJSON() ([]byte, error) {
	if len(j) == 0 {
		return []byte("null"), nil
	}
	return []byte(j), nil
}

// UnmarshalJSON 实现 json.Unmarshaler。
func (j *JSONB) UnmarshalJSON(data []byte) error {
	if j == nil {
		return errors.New("JSONB.UnmarshalJSON: 指针为空")
	}
	*j = append((*j)[:0], data...)
	return nil
}

// ===== 19. 异步任务 =====

type AsyncTask struct {
	ID        int       `gorm:"column:id;primaryKey" json:"id"`
	TaskType  string    `gorm:"column:task_type" json:"task_type"`
	Status    string    `gorm:"column:status;default:pending" json:"status"`
	Payload   JSONB     `gorm:"column:payload;type:jsonb" json:"payload,omitempty"`
	Result    JSONB     `gorm:"column:result;type:jsonb" json:"result,omitempty"`
	Error     string    `gorm:"column:error" json:"error"`
	CreatedAt time.Time `gorm:"column:created_at" json:"created_at"`
	UpdatedAt time.Time `gorm:"column:updated_at" json:"updated_at"`
}

func (AsyncTask) TableName() string { return "async_task" }

// ===== 20. 内容精选（公司动态 / 行业新闻 等） =====

type FeaturedContent struct {
	ContentID   int        `gorm:"column:content_id;primaryKey" json:"content_id"`
	Title       string     `gorm:"column:title" json:"title"`
	Summary     string     `gorm:"column:summary" json:"summary"`
	CoverImage  string     `gorm:"column:cover_image" json:"cover_image"`
	Content     string     `gorm:"column:content" json:"content"`
	Category    string     `gorm:"column:category;default:industry" json:"category"`
	Source      string     `gorm:"column:source" json:"source"`
	Status      int16      `gorm:"column:status;default:0" json:"status"`
	ViewCount   int        `gorm:"column:view_count;default:0" json:"view_count"`
	SortOrder   int        `gorm:"column:sort_order;default:0" json:"sort_order"`
	PublishedAt *time.Time `gorm:"column:published_at" json:"published_at,omitempty"`
	CreatedAt   time.Time  `gorm:"column:created_at" json:"created_at"`
	UpdatedAt   time.Time  `gorm:"column:updated_at" json:"updated_at"`
}

func (FeaturedContent) TableName() string { return "featured_content" }

// ===== 21. 系统设置 =====

// SystemSetting 系统设置表（key-value 结构，承载 AI 等模块的动态配置）。
type SystemSetting struct {
	Key         string    `gorm:"column:key;primaryKey" json:"key"`
	Value       string    `gorm:"column:value" json:"value"`
	Description string    `gorm:"column:description" json:"description"`
	UpdatedAt   time.Time `gorm:"column:updated_at" json:"updated_at"`
}

func (SystemSetting) TableName() string { return "system_settings" }

// ===== 通用收藏（ADR-0018）=====

// Favorite 通用收藏：target_type + target_id 多态定位
// （course/chapter/question/featured/topic，ADR-0018；user+type+id 唯一）。
type Favorite struct {
	FavoriteID int64     `gorm:"column:favorite_id;primaryKey;autoIncrement" json:"favorite_id"`
	UserID     int       `gorm:"column:user_id" json:"user_id"`
	TargetType string    `gorm:"column:target_type;size:20" json:"target_type"`
	TargetID   int       `gorm:"column:target_id" json:"target_id"`
	CreatedAt  time.Time `gorm:"column:created_at" json:"created_at"`
}

func (Favorite) TableName() string { return "favorite" }
