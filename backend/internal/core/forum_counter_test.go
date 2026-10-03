// Package core forumCounter 护栏语义单测（spec #297）。
// 注销点赞回扣用例（TestDeleteAccount_RefundsForumLikeCounts）随注销动作搬去 internal/auth/（ADR-0070 波 3a）。
package core

import (
	"testing"
	"time"

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
