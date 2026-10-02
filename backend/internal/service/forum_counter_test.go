// Package service forumCounter 单测与集成测试（spec #297）：
// 注销点赞回扣、删楼中楼 reply_count 级联少减 N。
package service

import (
	"testing"
	"time"

	"gorm.io/gorm"

	"forklift-training/internal/model"
	"forklift-training/internal/testutil"
)

// --- ForumCounter 护栏语义 ---

func TestForumCounter_GuardDoesNotGoNegative(t *testing.T) {
	db := testutil.NewMemoryDB(t)
	cnt := NewForumCounter()
	u := testutil.SeedStudent(t, db, "guard", "hash")
	now := time.Now()
	topic := model.ForumTopic{UserID: u.ID, Title: "t", Content: "c", LikesCount: 1, ReplyCount: 1, CreatedAt: now, UpdatedAt: now}
	if err := db.Create(&topic).Error; err != nil {
		t.Fatal(err)
	}

	// 连续 -1 两次：第二次因 likes_count > 0 护栏不再递减，保持 0。
	if err := cnt.AdjustLikes(db, topic.ID, -1); err != nil {
		t.Fatal(err)
	}
	if err := cnt.AdjustLikes(db, topic.ID, -1); err != nil {
		t.Fatal(err)
	}
	var got model.ForumTopic
	if err := db.First(&got, topic.ID).Error; err != nil {
		t.Fatal(err)
	}
	if got.LikesCount != 0 || got.ReplyCount != 1 {
		t.Fatalf("护栏后 likes_count 应为 0, got %d", got.LikesCount)
	}

	if err := cnt.AdjustReplyCounts(db, topic.ID, -5); err != nil {
		t.Fatal(err)
	}
	if err := db.First(&got, topic.ID).Error; err != nil {
		t.Fatal(err)
	}
	if got.ReplyCount != 0 {
		t.Fatalf("reply_count 下限应为 0, got %d", got.ReplyCount)
	}
}

// --- 注销回扣：点赞 → 注销 → likes_count 归位 ---

func TestDeleteAccount_RefundsForumLikeCounts(t *testing.T) {
	authSvc, db := newAuthSvc(t)
	author := testutil.SeedStudent(t, db, "refund_author", "hash")
	liker := testutil.SeedStudent(t, db, "refund_liker", "hash")
	bystander := testutil.SeedStudent(t, db, "refund_bystander", "hash")

	now := time.Now()
	topic := model.ForumTopic{UserID: author.ID, Title: "回扣主题", Content: "内容", CreatedAt: now, UpdatedAt: now}
	if err := db.Create(&topic).Error; err != nil {
		t.Fatal(err)
	}
	reply := model.ForumReply{TopicID: topic.ID, UserID: author.ID, Content: "被赞回复", CreatedAt: now}
	if err := db.Create(&reply).Error; err != nil {
		t.Fatal(err)
	}

	// 点赞行直插 + 计数走 ForumCounter：本用例判的是「注销时按行数回扣计数」，与论坛域无关；
	// 域包不得被留驻测试反向 import（ADR-0070 波 2b-2），故不经 NewForumService 的点赞入口。
	cnt := NewForumCounter()
	if err := db.Create(&model.ForumTopicLike{TopicID: topic.ID, UserID: liker.ID, CreatedAt: now}).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&model.ForumTopicLike{TopicID: topic.ID, UserID: bystander.ID, CreatedAt: now}).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&model.ForumReplyLike{ReplyID: reply.ID, UserID: liker.ID, CreatedAt: now}).Error; err != nil {
		t.Fatal(err)
	}
	if err := cnt.AdjustLikes(db, topic.ID, 2); err != nil {
		t.Fatal(err)
	}
	if err := cnt.AdjustReplyLikes(db, reply.ID, 1); err != nil {
		t.Fatal(err)
	}
	assertForumInt(t, db, "forum_topics", topic.ID, "likes_count", 2)
	assertForumInt(t, db, "forum_replies", reply.ID, "likes_count", 1)
	// 注销 liker：其主题/回复点赞行删除并按行数回扣计数
	if err := authSvc.DeleteAccount(liker.ID); err != nil {
		t.Fatalf("注销失败: %v", err)
	}
	assertForumInt(t, db, "forum_topics", topic.ID, "likes_count", 1)  // bystander 的赞保留
	assertForumInt(t, db, "forum_replies", reply.ID, "likes_count", 0) // 归位

	var likeRows int64
	db.Model(&model.ForumTopicLike{}).Where("user_id = ?", liker.ID).Count(&likeRows)
	db.Model(&model.ForumReplyLike{}).Where("user_id = ?", liker.ID).Count(&likeRows)
	if likeRows != 0 {
		t.Fatalf("liker 点赞行应清零, got %d", likeRows)
	}
	var n int64
	db.Model(&model.HrwaiUser{}).Where("id = ?", liker.ID).Count(&n)
	if n != 0 {
		t.Fatal("liker 账号应已硬删除")
	}
}

// assertForumInt 断言表内某行某整型列的值（列名白名单由调用方保证为常量字面量）。
func assertForumInt(t *testing.T, db *gorm.DB, table string, id int64, col string, want int64) {
	t.Helper()
	var got int64
	if err := db.Table(table).Select(col).Where("id = ?", id).Scan(&got).Error; err != nil {
		t.Fatalf("读取 %s.%s 失败: %v", table, col, err)
	}
	if got != want {
		t.Fatalf("%s.%s = %d, want %d", table, col, got, want)
	}
}
