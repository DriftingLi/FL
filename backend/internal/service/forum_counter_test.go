// Package service forumCounter 单测与集成测试（spec #297）：
// isDuplicateError 双方言谓词、注销点赞回扣、删楼中楼 reply_count 级联少减 N。
package service

import (
	"errors"
	"testing"
	"time"

	"go.uber.org/zap"
	"gorm.io/gorm"

	"forklift-training/internal/model"
	"forklift-training/internal/testutil"
)

// --- isDuplicateError：PG / SQLite 双言文案 ---

func TestIsDuplicateError_PostgresDialect(t *testing.T) {
	err := errors.New(`ERROR: duplicate key value violates unique constraint "uq_forum_topic_like" (SQLSTATE 23505)`)
	if !isDuplicateError(err) {
		t.Fatal("PG duplicate key 文案应判为唯一冲突")
	}
}

func TestIsDuplicateError_SQLiteDialect(t *testing.T) {
	err := errors.New("UNIQUE constraint failed: forum_checkin.user_id, forum_checkin.check_date")
	if !isDuplicateError(err) {
		t.Fatal("SQLite UNIQUE constraint 文案应判为唯一冲突")
	}
}

func TestIsDuplicateError_ConstraintNamePrefixes(t *testing.T) {
	for _, msg := range []string{
		"constraint uq_forum_reply_like violated",
		"pk_forum_checkin 冲突",
	} {
		if !isDuplicateError(errors.New(msg)) {
			t.Fatalf("约束名前缀文案应判为唯一冲突: %s", msg)
		}
	}
}

func TestIsDuplicateError_Negative(t *testing.T) {
	for _, err := range []error{
		nil,
		errors.New("record not found"),
		errors.New("connection refused"),
	} {
		if isDuplicateError(err) {
			t.Fatalf("非唯一冲突错误不应命中: %v", err)
		}
	}
}

// --- ForumCounter 护栏语义 ---

func TestForumCounter_GuardDoesNotGoNegative(t *testing.T) {
	db := testutil.NewMemoryDB(t)
	cnt := NewForumCounter()
	u := seedForumUser(t, db, "guard")
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

	forumSvc := NewForumService(db, nil, NewNotificationService(db, zap.NewNop()), NewForumCounter(), zap.NewNop())
	if _, err := forumSvc.LikeTopic(liker.ID, topic.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := forumSvc.LikeTopic(bystander.ID, topic.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := forumSvc.LikeReply(liker.ID, reply.ID); err != nil {
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

// --- 删楼中楼：reply_count -= N（子树大小，含自身）---

func TestDeleteNestedReply_DecrementsReplyCountBySubtreeSize(t *testing.T) {
	svc, db, _ := newForumTestSvc(t)
	user := seedForumUser(t, db, "nest")

	topic, err := svc.CreateTopic(CreateTopicInput{UserID: user.ID, Title: "标题", Content: "内容"})
	if err != nil {
		t.Fatal(err)
	}
	// 结构：r0（顶层幸存者）/ r1（删除根）← r2 ← r3，reply_count = 4。
	if _, err := svc.ReplyTopic(user.ID, topic.ID, "顶层幸存者", nil, nil); err != nil {
		t.Fatal(err)
	}
	r1, err := svc.ReplyTopic(user.ID, topic.ID, "一楼", nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	r2, err := svc.ReplyTopic(user.ID, topic.ID, "楼中楼", &r1.ID, nil)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := svc.ReplyTopic(user.ID, topic.ID, "楼中楼的楼中楼", &r2.ID, nil); err != nil {
		t.Fatal(err)
	}
	assertForumInt(t, db, "forum_topics", topic.ID, "reply_count", 4)

	// 删除 r1：子树 {r1, r2, r3} 大小 N=3（生产端 ON DELETE CASCADE 连带删下级回复），
	// reply_count 应 -3 剩 1，而非旧逻辑固定 -1 剩 3。
	if err := svc.DeleteReply(user.ID, r1.ID); err != nil {
		t.Fatalf("删除回复失败: %v", err)
	}
	assertForumInt(t, db, "forum_topics", topic.ID, "reply_count", 1)

	var survivor model.ForumReply
	if err := db.Where("topic_id = ? AND content = ?", topic.ID, "顶层幸存者").First(&survivor).Error; err != nil {
		t.Fatalf("无关回复不应受影响: %v", err)
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
