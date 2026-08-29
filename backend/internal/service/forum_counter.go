// Package service 实现业务服务层。
// 本文件：forumCounter——论坛反范式计数列（likes_count / reply_count）增减的唯一写入口（spec #297）。
// ForumService 与 AuthService 经构造注入同一接口，保证「点赞行数 ↔ 计数列」同步与下限护栏语义一致。
package service

import (
	"fmt"
	"strings"

	"gorm.io/gorm"

	"forklift-training/internal/model"
)

// ForumCounter 论坛计数唯一写入口。exec 为执行器：调用方传服务自身的 s.db 或事务 tx，
// 使计数调整与外层写入同事务提交/回滚。负向调整自带「列 > 0」护栏，并发下不出现负数。
type ForumCounter interface {
	// AdjustLikes 调整主题点赞数（delta<0 时仅当 likes_count > 0 才递减）。
	AdjustLikes(exec *gorm.DB, topicID int64, delta int) error
	// AdjustReplyCounts 调整主题回复数（delta<0 时仅当 reply_count > 0 才递减）。
	AdjustReplyCounts(exec *gorm.DB, topicID int64, delta int) error
	// AdjustReplyLikes 调整回复点赞数（delta<0 时仅当 likes_count > 0 才递减）。
	AdjustReplyLikes(exec *gorm.DB, replyID int64, delta int) error
}

// forumCounter 生产实现：经 gorm 直写计数列。
type forumCounter struct{}

// NewForumCounter 构造生产 DB 计数器（装配根唯一实例，注入 ForumService 与 AuthService）。
func NewForumCounter() ForumCounter { return forumCounter{} }

// AdjustLikes 实现 ForumCounter：主题点赞数增减。
func (forumCounter) AdjustLikes(exec *gorm.DB, topicID int64, delta int) error {
	return adjustForumCount(exec, &model.ForumTopic{}, "id", topicID, "likes_count", delta)
}

// AdjustReplyCounts 实现 ForumCounter：主题回复数增减。
func (forumCounter) AdjustReplyCounts(exec *gorm.DB, topicID int64, delta int) error {
	return adjustForumCount(exec, &model.ForumTopic{}, "id", topicID, "reply_count", delta)
}

// AdjustReplyLikes 实现 ForumCounter：回复点赞数增减。
func (forumCounter) AdjustReplyLikes(exec *gorm.DB, replyID int64, delta int) error {
	return adjustForumCount(exec, &model.ForumReply{}, "id", replyID, "likes_count", delta)
}

// adjustForumCount 计数列统一写入：delta>0 直接加；delta<0 带「列 > 0」护栏，
// 并以 CASE 表达式把结果钳在下限 0（等价原「读出取 max(0, n+N) 再写回」语义且原子，
// 防止 |delta| 大于当前值时穿透为负数）。双方言兼容（PG/SQLite 均支持 CASE）。
func adjustForumCount(exec *gorm.DB, dst any, idCol string, id int64, col string, delta int) error {
	if delta == 0 {
		return nil
	}
	q := exec.Model(dst).Where(idCol+" = ?", id)
	if delta < 0 {
		q = q.Where(col + " > 0")
	}
	return q.UpdateColumn(col, gorm.Expr(
		fmt.Sprintf("CASE WHEN %s + ? < 0 THEN 0 ELSE %s + ? END", col, col),
		delta, delta,
	)).Error
}

// isDuplicateError 判断数据库错误是否为唯一约束冲突——幂等写入点共享谓词，
// 收敛原多处字符串匹配复制；小写归一后兼容 PG（duplicate key / uq_ 约束名）
// 与 SQLite（UNIQUE constraint failed）双方言。
func isDuplicateError(err error) bool {
	if err == nil {
		return false
	}
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "duplicate") ||
		strings.Contains(msg, "unique") ||
		strings.Contains(msg, "uq_") ||
		strings.Contains(msg, "pk_")
}
