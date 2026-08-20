// 论坛事件站内信通知测试（回复通知楼主/被回复人、举报处理、管理端删帖/删回复）。
package service

import (
	"strconv"
	"strings"
	"testing"
	"time"

	"gorm.io/gorm"

	"forklift-training/internal/model"
)

// seedNotificationTopic 播种一个帖子（作者 author）并返回。
func seedNotificationTopic(t *testing.T, db *gorm.DB, author *model.HrwaiUser, title string) *model.ForumTopic {
	t.Helper()
	now := time.Now()
	topic := &model.ForumTopic{UserID: author.ID, Title: title, Content: "内容", CreatedAt: now, UpdatedAt: now}
	if err := db.Create(topic).Error; err != nil {
		t.Fatalf("创建主题失败: %v", err)
	}
	return topic
}

// notificationsOf 查询某用户的全部通知。
func notificationsOf(t *testing.T, db *gorm.DB, userID int) []model.Notification {
	t.Helper()
	var ns []model.Notification
	if err := db.Where("user_id = ?", userID).Find(&ns).Error; err != nil {
		t.Fatalf("查询通知失败: %v", err)
	}
	return ns
}

func topicLink(topicID int64) string {
	return "/training/forum/" + strconv.FormatInt(topicID, 10)
}

// TestForumNotify_ReplyToOwnTopic 回复自己的帖子：不产生任何通知。
func TestForumNotify_ReplyToOwnTopic(t *testing.T) {
	svc, db, _ := newForumTestSvc(t)
	author := seedForumUser(t, db, "own-author")
	topic := seedNotificationTopic(t, db, author, "自学帖")

	if _, err := svc.ReplyTopic(author.ID, topic.ID, "自己顶一下", nil, nil); err != nil {
		t.Fatalf("回复失败: %v", err)
	}
	if ns := notificationsOf(t, db, author.ID); len(ns) != 0 {
		t.Fatalf("回复自己的帖子不应通知，得到 %d 条", len(ns))
	}
}

// TestForumNotify_ReplyToOthersTopic 他人回帖：楼主收到 1 条 forum_reply 通知（含标题与链接），回复人无通知。
func TestForumNotify_ReplyToOthersTopic(t *testing.T) {
	svc, db, _ := newForumTestSvc(t)
	author := seedForumUser(t, db, "topic-author")
	replier := seedForumUser(t, db, "replier-1")
	topic := seedNotificationTopic(t, db, author, "求助帖")

	if _, err := svc.ReplyTopic(replier.ID, topic.ID, "我来解答", nil, nil); err != nil {
		t.Fatalf("回复失败: %v", err)
	}
	ns := notificationsOf(t, db, author.ID)
	if len(ns) != 1 {
		t.Fatalf("楼主应收到 1 条通知，得到 %d", len(ns))
	}
	if ns[0].Type != "forum_reply" || ns[0].Title != "你的帖子有新回复" {
		t.Fatalf("通知类型/标题不符: %+v", ns[0])
	}
	if ns[0].Link != topicLink(topic.ID) {
		t.Fatalf("通知链接不符: %s", ns[0].Link)
	}
	if !strings.Contains(ns[0].Content, "求助帖") || !strings.Contains(ns[0].Content, "replier-1") {
		t.Fatalf("通知文案应包含帖子标题与回复人: %s", ns[0].Content)
	}
	if ns := notificationsOf(t, db, replier.ID); len(ns) != 0 {
		t.Fatalf("回复人不应收到通知，得到 %d 条", len(ns))
	}
}

// TestForumNotify_ReplyToReply 楼中楼去重：
// - 回复楼主自己的楼层回复 → 只按「楼主被回帖」通知 1 次，不重复；
// - 回复第三人的楼层回复 → 第三人收到「你的回复有新回复」。
func TestForumNotify_ReplyToReply(t *testing.T) {
	svc, db, _ := newForumTestSvc(t)
	author := seedForumUser(t, db, "lz")
	commenter := seedForumUser(t, db, "commenter")
	third := seedForumUser(t, db, "third")
	topic := seedNotificationTopic(t, db, author, "讨论帖")

	// 楼主自己发一条楼层回复（无通知）
	ar, err := svc.ReplyTopic(author.ID, topic.ID, "楼主补充", nil, nil)
	if err != nil {
		t.Fatalf("楼主回复失败: %v", err)
	}
	if ns := notificationsOf(t, db, author.ID); len(ns) != 0 {
		t.Fatalf("楼主自回不应通知，得到 %d", len(ns))
	}

	// commenter 楼中楼回复楼主的那条（被回复人=楼主 → 与楼主被回帖合并，只 1 条）
	if _, err := svc.ReplyTopic(commenter.ID, topic.ID, "追问", &ar.ID, nil); err != nil {
		t.Fatalf("楼中楼失败: %v", err)
	}
	if ns := notificationsOf(t, db, author.ID); len(ns) != 1 {
		t.Fatalf("被回复人=楼主时应只通知 1 条，得到 %d", len(ns))
	}

	// commenter 发自己的楼层回复，third 楼中楼回复它 → commenter 收到楼中楼通知
	cr, err := svc.ReplyTopic(commenter.ID, topic.ID, "我的楼层", nil, nil)
	if err != nil {
		t.Fatalf("commenter 回帖失败: %v", err)
	}
	if _, err := svc.ReplyTopic(third.ID, topic.ID, "回复你的楼层", &cr.ID, nil); err != nil {
		t.Fatalf("楼中楼失败: %v", err)
	}
	// commenter 的 cr（顶层回复 → 楼主第 2 条）+ third 的楼中楼同时是对楼主帖的新回复（楼主第 3 条）
	if ns := notificationsOf(t, db, author.ID); len(ns) != 3 {
		t.Fatalf("楼主应累计 3 条通知，得到 %d", len(ns))
	}
	cns := notificationsOf(t, db, commenter.ID)
	if len(cns) != 1 || cns[0].Title != "你的回复有新回复" {
		t.Fatalf("commenter 应收到 1 条楼中楼通知: %+v", cns)
	}
	if cns[0].Link != topicLink(topic.ID) {
		t.Fatalf("楼中楼通知链接不符: %s", cns[0].Link)
	}
}

// TestForumNotify_ReportHandled 举报标记已处理：举报人收到 forum_report 通知；重复标记不重复通知。
func TestForumNotify_ReportHandled(t *testing.T) {
	svc, db, _ := newForumTestSvc(t)
	reporter := seedForumUser(t, db, "reporter")
	author := seedForumUser(t, db, "r-author")
	topic := seedNotificationTopic(t, db, author, "被举报帖")

	report := &model.ForumReport{ReporterID: reporter.ID, TopicID: &topic.ID,
		Reason: "广告", Status: 0, CreatedAt: time.Now()}
	if err := db.Create(report).Error; err != nil {
		t.Fatalf("创建举报失败: %v", err)
	}

	if err := svc.HandleReport(report.ID, 1); err != nil {
		t.Fatalf("处理举报失败: %v", err)
	}
	ns := notificationsOf(t, db, reporter.ID)
	if len(ns) != 1 || ns[0].Type != "forum_report" || ns[0].Title != "举报已处理" {
		t.Fatalf("举报人应收到 1 条举报处理通知: %+v", ns)
	}
	if ns[0].Link != topicLink(topic.ID) {
		t.Fatalf("通知链接不符: %s", ns[0].Link)
	}

	// 重复标记已处理：不再通知
	if err := svc.HandleReport(report.ID, 1); err != nil {
		t.Fatalf("重复处理失败: %v", err)
	}
	if ns := notificationsOf(t, db, reporter.ID); len(ns) != 1 {
		t.Fatalf("重复标记不应重复通知，得到 %d", len(ns))
	}
}

// TestForumNotify_ReportHandledTopicDeleted 被举报主题已删除：降级文案（无链接）。
func TestForumNotify_ReportHandledTopicDeleted(t *testing.T) {
	svc, db, _ := newForumTestSvc(t)
	reporter := seedForumUser(t, db, "reporter2")
	deletedID := int64(999)
	report := &model.ForumReport{ReporterID: reporter.ID, TopicID: &deletedID,
		Reason: "违规", Status: 0, CreatedAt: time.Now()}
	if err := db.Create(report).Error; err != nil {
		t.Fatalf("创建举报失败: %v", err)
	}

	if err := svc.HandleReport(report.ID, 1); err != nil {
		t.Fatalf("处理举报失败: %v", err)
	}
	ns := notificationsOf(t, db, reporter.ID)
	if len(ns) != 1 || ns[0].Link != "" {
		t.Fatalf("主题已删应降级为无链接通知: %+v", ns)
	}
}

// TestForumNotify_AdminDeleteTopic 管理员删帖：作者收到删除通知（无链接）。
func TestForumNotify_AdminDeleteTopic(t *testing.T) {
	svc, db, _ := newForumTestSvc(t)
	author := seedForumUser(t, db, "del-author")
	topic := seedNotificationTopic(t, db, author, "待删帖")

	if err := svc.AdminDeleteTopic(topic.ID); err != nil {
		t.Fatalf("删帖失败: %v", err)
	}
	ns := notificationsOf(t, db, author.ID)
	if len(ns) != 1 || ns[0].Type != "forum_topic_deleted" || ns[0].Title != "你的帖子已被删除" {
		t.Fatalf("作者应收到删帖通知: %+v", ns)
	}
	if ns[0].Link != "" {
		t.Fatalf("已删帖子不应带链接: %s", ns[0].Link)
	}
}

// TestForumNotify_AdminDeleteReply 管理员删回复：回复作者收到通知（带帖子链接）。
func TestForumNotify_AdminDeleteReply(t *testing.T) {
	svc, db, _ := newForumTestSvc(t)
	author := seedForumUser(t, db, "reply-author")
	topic := seedNotificationTopic(t, db, seedForumUser(t, db, "lz2"), "有回复的帖")

	reply, err := svc.ReplyTopic(author.ID, topic.ID, "待删回复", nil, nil)
	if err != nil {
		t.Fatalf("回复失败: %v", err)
	}
	if err := svc.AdminDeleteReply(reply.ID); err != nil {
		t.Fatalf("删回复失败: %v", err)
	}
	ns := notificationsOf(t, db, author.ID)
	if len(ns) != 1 || ns[0].Type != "forum_reply_deleted" || ns[0].Title != "你的回复已被删除" {
		t.Fatalf("回复作者应收到删回复通知: %+v", ns)
	}
	if ns[0].Link != topicLink(topic.ID) {
		t.Fatalf("删回复通知应带帖子链接: %s", ns[0].Link)
	}
}
