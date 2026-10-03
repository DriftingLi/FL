// Package forum 论坛计数用例：删楼中楼 reply_count 级联少减 N（spec #297）。
//
// 本用例原在 internal/service/forum_counter_test.go，随域包搬来（ADR-0070 波 2b-2）；
// 它用到的 assertForumInt 在本文件尾部有私有副本 —— 域包不得 import internal/core 的测试文件。
package forum

import (
	"testing"

	"gorm.io/gorm"

	"forklift-training/internal/model"
)

// --- 删楼中楼：reply_count -= N（子树大小，含自身）---

func TestDeleteNestedReply_DecrementsReplyCountBySubtreeSize(t *testing.T) {
	svc, db, _ := newForumTestSvc(t)
	user := seedForumUser(t, db, "nest")

	topic, err := svc.CreateTopic(CreateTopicInput{UserID: user.ID, Title: "标题", Content: "内容"})
	if err != nil {
		t.Fatal(err)
	}
	// 结构：r0（顶层幸存者）/ r1（删除根）← r2 ← r3，reply_count = 4。
	if _, err := svc.ReplyTopic(ReplyTopicInput{UserID: user.ID, TopicID: topic.ID, Content: "顶层幸存者", ParentReplyID: nil, Images: nil}); err != nil {
		t.Fatal(err)
	}
	r1, err := svc.ReplyTopic(ReplyTopicInput{UserID: user.ID, TopicID: topic.ID, Content: "一楼", ParentReplyID: nil, Images: nil})
	if err != nil {
		t.Fatal(err)
	}
	r2, err := svc.ReplyTopic(ReplyTopicInput{UserID: user.ID, TopicID: topic.ID, Content: "楼中楼", ParentReplyID: &r1.ID, Images: nil})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := svc.ReplyTopic(ReplyTopicInput{UserID: user.ID, TopicID: topic.ID, Content: "楼中楼的楼中楼", ParentReplyID: &r2.ID, Images: nil}); err != nil {
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
