// Package service 站内信事件构造器契约测试（ADR-0027 C1）：
// 全域收编后 title/content/link/payload 口径在站内信域单点锁定，
// 业务侧一行触发；本文件锁「逐字零漂移」契约（含 link 查询参数两种既有变体）。
package service

import (
	"encoding/json"
	"testing"
	"time"

	"go.uber.org/zap"
	"gorm.io/gorm"

	"forklift-training/internal/model"
	"forklift-training/internal/testutil"
)

// TestForumReplyEventConstructors 楼主被回复 / 楼中楼被回复两条口径逐字锁定。
func TestForumReplyEventConstructors(t *testing.T) {
	db := testutil.NewMemoryDB(t)
	svc := NewNotificationService(db, zap.NewNop())
	now := time.Now()

	owner := NewTopicReplierEvent(1, "小明", "如何更换叉车轮胎", 42)
	replyee := NewReplyReplierEvent(2, "小明", "如何更换叉车轮胎", 42)

	if owner.IsReplyToReply {
		t.Fatalf("楼主被回复事件应为帖子变体: %+v", owner)
	}
	if !replyee.IsReplyToReply {
		t.Fatalf("楼中楼被回复事件应为回复变体: %+v", replyee)
	}

	if err := db.Transaction(func(tx *gorm.DB) error {
		if err := svc.CreateForumReplyEvent(tx, owner, now); err != nil {
			return err
		}
		return svc.CreateForumReplyEvent(tx, replyee, now)
	}); err != nil {
		t.Fatalf("事务内创建失败: %v", err)
	}

	var ownerN, replyeeN model.Notification
	if err := db.Where("user_id = ?", 1).First(&ownerN).Error; err != nil {
		t.Fatalf("查询楼主通知失败: %v", err)
	}
	if err := db.Where("user_id = ?", 2).First(&replyeeN).Error; err != nil {
		t.Fatalf("查询楼中楼通知失败: %v", err)
	}

	// 楼主被回复
	if ownerN.Type != "forum_reply" || ownerN.Title != "你的帖子有新回复" {
		t.Fatalf("楼主通知类型/标题不符: %+v", ownerN)
	}
	if ownerN.Content != "小明 回复了你的帖子「如何更换叉车轮胎」" {
		t.Fatalf("楼主通知文案不符: %s", ownerN.Content)
	}
	if ownerN.Link != "/training/forum/42" {
		t.Fatalf("楼主通知链接不符: %s", ownerN.Link)
	}
	assertTopicPayload(t, ownerN.Payload, 42)

	// 楼中楼被回复
	if replyeeN.Type != "forum_reply" || replyeeN.Title != "你的回复有新回复" {
		t.Fatalf("楼中楼通知类型/标题不符: %+v", replyeeN)
	}
	if replyeeN.Content != "小明 在帖子「如何更换叉车轮胎」中回复了你" {
		t.Fatalf("楼中楼通知文案不符: %s", replyeeN.Content)
	}
	if replyeeN.Link != "/training/forum/42" {
		t.Fatalf("楼中楼通知链接不符: %s", replyeeN.Link)
	}
	assertTopicPayload(t, replyeeN.Payload, 42)
}

// assertTopicPayload 断言 forum 事件 payload 为 {"topic_id": N}。
func assertTopicPayload(t *testing.T, payload model.JSONB, topicID int64) {
	t.Helper()
	var m map[string]int64
	if err := json.Unmarshal([]byte(payload), &m); err != nil {
		t.Fatalf("payload 解析失败: %v", err)
	}
	if m["topic_id"] != topicID {
		t.Fatalf("payload topic_id 不符: %v", m)
	}
}

// TestForumDeletionEventsContract 管理端删帖 / 删回复口径逐字锁定（尽力而为族落库验证）。
func TestForumDeletionEventsContract(t *testing.T) {
	db := testutil.NewMemoryDB(t)
	svc := NewNotificationService(db, zap.NewNop())

	svc.TryCreateForumTopicDeletedEvent(NewForumTopicDeletedEvent(1, "违规帖"))
	svc.TryCreateForumReplyDeletedEvent(NewForumReplyDeletedEvent(2, "有回复的帖", 88))

	var topicDel, replyDel model.Notification
	if err := db.Where("user_id = ?", 1).First(&topicDel).Error; err != nil {
		t.Fatalf("查询删帖通知失败: %v", err)
	}
	if topicDel.Type != "forum_topic_deleted" || topicDel.Title != "你的帖子已被删除" {
		t.Fatalf("删帖通知类型/标题不符: %+v", topicDel)
	}
	if topicDel.Content != "管理员删除了你的帖子「违规帖」。" {
		t.Fatalf("删帖通知文案不符: %s", topicDel.Content)
	}
	if topicDel.Link != "" || topicDel.Payload != nil {
		t.Fatalf("删帖通知不应带链接与 payload: link=%s payload=%s", topicDel.Link, topicDel.Payload)
	}

	if err := db.Where("user_id = ?", 2).First(&replyDel).Error; err != nil {
		t.Fatalf("查询删回复通知失败: %v", err)
	}
	if replyDel.Type != "forum_reply_deleted" || replyDel.Title != "你的回复已被删除" {
		t.Fatalf("删回复通知类型/标题不符: %+v", replyDel)
	}
	if replyDel.Content != "管理员删除了你在帖子「有回复的帖」中的回复。" {
		t.Fatalf("删回复通知文案不符: %s", replyDel.Content)
	}
	if replyDel.Link != "/training/forum/88" {
		t.Fatalf("删回复通知链接不符: %s", replyDel.Link)
	}
	assertTopicPayload(t, replyDel.Payload, 88)
}

// TestForumReportHandledEventContract 举报处理两种口径：主题存在（带标题+链接）与已删（降级无链接）。
func TestForumReportHandledEventContract(t *testing.T) {
	db := testutil.NewMemoryDB(t)
	svc := NewNotificationService(db, zap.NewNop())

	// 帖子举报，主题存在
	topicID := int64(77)
	svc.TryCreateForumReportHandledEvent(NewForumReportHandledEvent(1, false, &topicID, "被举报帖"))
	// 回复举报，主题已删（topicTitle 空 → 降级）
	deletedID := int64(999)
	svc.TryCreateForumReportHandledEvent(NewForumReportHandledEvent(2, true, &deletedID, ""))

	var ok, degraded model.Notification
	if err := db.Where("user_id = ?", 1).First(&ok).Error; err != nil {
		t.Fatalf("查询举报处理通知失败: %v", err)
	}
	if ok.Type != "forum_report" || ok.Title != "举报已处理" {
		t.Fatalf("举报通知类型/标题不符: %+v", ok)
	}
	if ok.Content != "你举报的帖子「被举报帖」已处理完毕。" {
		t.Fatalf("举报通知文案不符: %s", ok.Content)
	}
	if ok.Link != "/training/forum/77" {
		t.Fatalf("举报通知链接不符: %s", ok.Link)
	}
	assertTopicPayload(t, ok.Payload, 77)

	if err := db.Where("user_id = ?", 2).First(&degraded).Error; err != nil {
		t.Fatalf("查询降级举报通知失败: %v", err)
	}
	if degraded.Content != "你举报的回复已处理完毕。" {
		t.Fatalf("降级举报通知文案不符: %s", degraded.Content)
	}
	if degraded.Link != "" || degraded.Payload != nil {
		t.Fatalf("主题已删应降级为无链接无 payload: link=%s payload=%s", degraded.Link, degraded.Payload)
	}
}

// TestContributionEventsContract 投稿全生命周期四条口径逐字锁定（含两种 link 变体与追回文案）。
func TestContributionEventsContract(t *testing.T) {
	db := testutil.NewMemoryDB(t)
	svc := NewNotificationService(db, zap.NewNop())
	now := time.Now()

	// 过审（+50）与达阶（+30）同事务
	if err := db.Transaction(func(tx *gorm.DB) error {
		if err := svc.CreateContributionApprovedEvent(tx, NewContributionApprovedEvent(1, "叉车保养手册", 101, 50), now); err != nil {
			return err
		}
		return svc.CreateContributionTierEvent(tx, NewContributionTierEvent(2, "叉车保养手册", 101, 10, 30), now)
	}); err != nil {
		t.Fatalf("事务内创建失败: %v", err)
	}
	// 驳回（非事务，db 直连）与下架（含追回）各自触发
	if err := svc.CreateContributionRejectedEvent(db, NewContributionRejectedEvent(3, "叉车保养手册", 101, "内容错误"), now); err != nil {
		t.Fatalf("创建驳回通知失败: %v", err)
	}
	if err := db.Transaction(func(tx *gorm.DB) error {
		return svc.CreateContributionArchivedEvent(tx, NewContributionArchivedEvent(4, "叉车保养手册", 101, "违规", 80), now)
	}); err != nil {
		t.Fatalf("创建下架通知失败: %v", err)
	}

	var approved, rejected, tier, archived model.Notification
	if err := db.Where("user_id = ?", 1).First(&approved).Error; err != nil {
		t.Fatalf("查询过审通知失败: %v", err)
	}
	if err := db.Where("user_id = ?", 3).First(&rejected).Error; err != nil {
		t.Fatalf("查询驳回通知失败: %v", err)
	}
	if err := db.Where("user_id = ?", 2).First(&tier).Error; err != nil {
		t.Fatalf("查询达阶通知失败: %v", err)
	}
	if err := db.Where("user_id = ?", 4).First(&archived).Error; err != nil {
		t.Fatalf("查询下架通知失败: %v", err)
	}

	// 过审：link 变体一（?tab=contribution）
	if approved.Type != "contribution_approved" || approved.Title != "资料投稿通过审核" {
		t.Fatalf("过审通知类型/标题不符: %+v", approved)
	}
	if approved.Content != "你的投稿「叉车保养手册」已通过审核，+50 分已到账" {
		t.Fatalf("过审通知文案不符: %s", approved.Content)
	}
	if approved.Link != "/training/materials?tab=contribution" {
		t.Fatalf("过审通知链接不符: %s", approved.Link)
	}
	assertContributionPayload(t, approved.Payload, 101, 50, "contribution_approved")

	// 达阶：link 变体一
	if tier.Type != "contribution_tier" || tier.Title != "资料投稿下载量达阶" {
		t.Fatalf("达阶通知类型/标题不符: %+v", tier)
	}
	if tier.Content != "你的投稿「叉车保养手册」下载量达 10 次，+30 分已到账" {
		t.Fatalf("达阶通知文案不符: %s", tier.Content)
	}
	if tier.Link != "/training/materials?tab=contribution" {
		t.Fatalf("达阶通知链接不符: %s", tier.Link)
	}
	assertContributionPayload(t, tier.Payload, 101, 30, "contribution_tier")

	// 驳回：link 变体二（?tab=contribution&view=mine），payload reason 空
	if rejected.Type != "contribution_rejected" || rejected.Title != "资料投稿被驳回" {
		t.Fatalf("驳回通知类型/标题不符: %+v", rejected)
	}
	if rejected.Content != "你的投稿「叉车保养手册」未通过审核：内容错误" {
		t.Fatalf("驳回通知文案不符: %s", rejected.Content)
	}
	if rejected.Link != "/training/materials?tab=contribution&view=mine" {
		t.Fatalf("驳回通知链接不符: %s", rejected.Link)
	}
	assertContributionPayload(t, rejected.Payload, 101, 0, "")

	// 下架：追回文案 + payload points 为负数
	if archived.Type != "contribution_archived" || archived.Title != "资料投稿已下架" {
		t.Fatalf("下架通知类型/标题不符: %+v", archived)
	}
	if archived.Content != "你的投稿「叉车保养手册」已下架：违规（已回收该稿奖励 80 分）" {
		t.Fatalf("下架通知文案不符: %s", archived.Content)
	}
	if archived.Link != "/training/materials?tab=contribution&view=mine" {
		t.Fatalf("下架通知链接不符: %s", archived.Link)
	}
	assertContributionPayload(t, archived.Payload, 101, -80, "rollback")
}

// assertContributionPayload 断言投稿 payload 口径。
func assertContributionPayload(t *testing.T, payload model.JSONB, contributionID int64, points int, reason string) {
	t.Helper()
	var m map[string]any
	if err := json.Unmarshal([]byte(payload), &m); err != nil {
		t.Fatalf("payload 解析失败: %v", err)
	}
	if int(m["contribution_id"].(float64)) != int(contributionID) {
		t.Fatalf("payload contribution_id 不符: %v", m)
	}
	if int(m["points"].(float64)) != points {
		t.Fatalf("payload points 不符: %v", m)
	}
	if m["reason"] != reason {
		t.Fatalf("payload reason 不符: %v", m)
	}
	if m["title"] != "叉车保养手册" {
		t.Fatalf("payload title 不符: %v", m)
	}
}

// TestContactRequestEventContract 联系方式交换申请通知口径逐字锁定（payload 双字段）。
func TestContactRequestEventContract(t *testing.T) {
	db := testutil.NewMemoryDB(t)
	svc := NewNotificationService(db, zap.NewNop())

	svc.TryCreateContactRequestEvent(NewContactRequestEvent(1, "华晨叉车", "王经理", "想约面试", 55, 9))

	var n model.Notification
	if err := db.Where("user_id = ?", 1).First(&n).Error; err != nil {
		t.Fatalf("查询申请通知失败: %v", err)
	}
	if n.Type != "contact_request" || n.Title != "收到联系方式交换申请" {
		t.Fatalf("申请通知类型/标题不符: %+v", n)
	}
	if n.Content != "企业「华晨叉车」联系人 王经理 申请查看你的联系方式，附言：想约面试" {
		t.Fatalf("申请通知文案不符: %s", n.Content)
	}
	if n.Link != "/training/resume" {
		t.Fatalf("申请通知链接不符: %s", n.Link)
	}
	var m map[string]int64
	if err := json.Unmarshal([]byte(n.Payload), &m); err != nil {
		t.Fatalf("payload 解析失败: %v", err)
	}
	if m["contact_request_id"] != 55 || m["recruiter_id"] != 9 {
		t.Fatalf("申请通知 payload 不符: %v", m)
	}
}

// TestTryCreateNilReceiver 尽力而为族对 nil receiver 不 panic（旧 contact 调用的 nil 守卫语义收编）。
func TestTryCreateNilReceiver(t *testing.T) {
	var svc *NotificationService
	svc.TryCreateForumTopicDeletedEvent(NewForumTopicDeletedEvent(1, "t"))
	svc.TryCreateForumReplyDeletedEvent(NewForumReplyDeletedEvent(1, "t", 1))
	svc.TryCreateForumReportHandledEvent(NewForumReportHandledEvent(1, false, nil, ""))
	svc.TryCreateContactRequestEvent(NewContactRequestEvent(1, "c", "n", "m", 1, 1))
}
