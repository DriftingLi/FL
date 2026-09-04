// 论坛域（CONTEXT.md「论坛」）。
package model

import "time"

// ===== 24. 论坛模块 =====

// ForumTopic 论坛主题。
//
// 两个正交维度中，chapter_id 只服务讨论帖：
//   - category=discussion + chapter_id IS NULL = 综合讨论区
//   - category=discussion + chapter_id 非空    = 章节讨论区
//   - category=question   + chapter_id IS NULL = 全局问答
//   - category=question   + chapter_id 非空    = 非法组合
//
// ⚠️ 判类别看 category，判区域看 chapter_id，两者不可互相替代：scope=general 的定义
// 就是 chapter_id IS NULL，而问答帖的 chapter_id 同样为 NULL，故列表查询必须让
// category 与 scope 共存在同一条 WHERE 里，否则问答帖会整片灌进讨论 Tab。
//
// ⚠️ 上述非法组合在数据库层由 CHECK 兜底，但这些约束**只存在于迁移 SQL（000004）**：
// 测试库由 AutoMigrate 建表、不执行 migrations/，因此两条 CHECK（值域 chk_forum_topics_category
// 与非法组合 chk_forum_topics_question_no_chapter）契约测试都覆盖不到，别误以为测试守住了它们。
// 行为层由 service 的校验守住：非法类别 400、问答帖带 chapter_id>0 返回 400。
type ForumTopic struct {
	ID              int64      `gorm:"column:id;primaryKey;autoIncrement" json:"id"`
	ChapterID       *int       `gorm:"column:chapter_id" json:"chapter_id,omitempty"`
	Category        string     `gorm:"column:category;not null;default:discussion" json:"category"` // 'discussion' | 'question'
	UserID          int        `gorm:"column:user_id" json:"user_id"`
	Title           string     `gorm:"column:title" json:"title"`
	Content         string     `gorm:"column:content" json:"content"`
	Images          JSONB      `gorm:"column:images;type:jsonb" json:"images"`
	ViewCount       int        `gorm:"column:view_count;default:0" json:"view_count"`
	ReplyCount      int        `gorm:"column:reply_count;default:0" json:"reply_count"`
	LikesCount      int        `gorm:"column:likes_count;default:0" json:"likes_count"`
	AcceptedReplyID *int64     `gorm:"column:accepted_reply_id" json:"accepted_reply_id,omitempty"`
	SolvedAt        *time.Time `gorm:"column:solved_at" json:"solved_at,omitempty"`
	LastReplyAt     *time.Time `gorm:"column:last_reply_at" json:"last_reply_at"`
	CreatedAt       time.Time  `gorm:"column:created_at" json:"created_at"`
	UpdatedAt       time.Time  `gorm:"column:updated_at" json:"updated_at"`
}

func (ForumTopic) TableName() string { return "forum_topics" }

// ForumReply 论坛回复。
type ForumReply struct {
	ID         int64     `gorm:"column:id;primaryKey;autoIncrement" json:"id"`
	TopicID    int64     `gorm:"column:topic_id" json:"topic_id"`
	UserID     int       `gorm:"column:user_id" json:"user_id"`
	ParentID   *int64    `gorm:"column:parent_id" json:"parent_id,omitempty"`
	Content    string    `gorm:"column:content" json:"content"`
	Images     JSONB     `gorm:"column:images;type:jsonb" json:"images"`
	LikesCount int       `gorm:"column:likes_count;default:0" json:"likes_count"`
	CreatedAt  time.Time `gorm:"column:created_at" json:"created_at"`
}

func (ForumReply) TableName() string { return "forum_replies" }

// ForumTopicLike 论坛主题点赞（topic_id+user_id 唯一约束保证幂等，ADR-0018）。
type ForumTopicLike struct {
	ID        int64     `gorm:"column:id;primaryKey;autoIncrement" json:"id"`
	TopicID   int64     `gorm:"column:topic_id" json:"topic_id"`
	UserID    int       `gorm:"column:user_id" json:"user_id"`
	CreatedAt time.Time `gorm:"column:created_at" json:"created_at"`
}

func (ForumTopicLike) TableName() string { return "forum_topic_like" }

// ForumCheckIn 每日打卡记录（user_id + check_date 唯一，Asia/Shanghai 自然日，spec #268）。
type ForumCheckIn struct {
	UserID    int       `gorm:"column:user_id;primaryKey" json:"user_id"`
	CheckDate time.Time `gorm:"column:check_date;primaryKey;type:date" json:"check_date"`
	CreatedAt time.Time `gorm:"column:created_at" json:"created_at"`
}

func (ForumCheckIn) TableName() string { return "forum_checkin" }

// ForumReplyLike 评论点赞（reply_id+user_id 唯一，与 ForumTopicLike 同构，spec #268）。
type ForumReplyLike struct {
	ID        int64     `gorm:"column:id;primaryKey;autoIncrement" json:"id"`
	ReplyID   int64     `gorm:"column:reply_id" json:"reply_id"`
	UserID    int       `gorm:"column:user_id" json:"user_id"`
	CreatedAt time.Time `gorm:"column:created_at" json:"created_at"`
}

func (ForumReplyLike) TableName() string { return "forum_reply_like" }

// ForumTopicView 用户帖子浏览去重（每日每帖一次，排除自帖，用于 daily_browse）
type ForumTopicView struct {
	UserID   int       `gorm:"column:user_id;primaryKey" json:"user_id"`
	TopicID  int64     `gorm:"column:topic_id;primaryKey" json:"topic_id"`
	ViewedAt time.Time `gorm:"column:viewed_at" json:"viewed_at"`
	ViewDate string    `gorm:"column:view_date;primaryKey;type:date" json:"view_date"`
}

func (ForumTopicView) TableName() string { return "forum_topic_views" }

// ForumReport 论坛举报：topic_id 与 reply_id 二选一；status 0 待处理 / 1 已处理（ADR-0018）。
type ForumReport struct {
	ID         int64     `gorm:"column:id;primaryKey;autoIncrement" json:"id"`
	ReporterID int       `gorm:"column:reporter_id" json:"reporter_id"`
	TopicID    *int64    `gorm:"column:topic_id" json:"topic_id,omitempty"`
	ReplyID    *int64    `gorm:"column:reply_id" json:"reply_id,omitempty"`
	Reason     string    `gorm:"column:reason" json:"reason"`
	Status     int16     `gorm:"column:status;default:0" json:"status"`
	CreatedAt  time.Time `gorm:"column:created_at" json:"created_at"`
}

func (ForumReport) TableName() string { return "forum_report" }
