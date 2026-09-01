// Package service 站内信问答采纳事件构造器测试（ADR-0024 C3）：
// title/content/link/payload 口径内聚站内信域，事务内使用形状验证，与积分入账一致。
package service

import (
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	"go.uber.org/zap"
	"gorm.io/gorm"

	"forklift-training/internal/model"
	"forklift-training/internal/testutil"
)

// TestForumAcceptEventConstructors 构造器输出与既有文案/链接/payload 契约逐字一致。
func TestForumAcceptEventConstructors(t *testing.T) {
	db := testutil.NewMemoryDB(t)
	svc := NewNotificationService(db, zap.NewNop())

	answerer := NewAnswererAcceptEvent(1, "如何更换叉车轮胎", 42, 7, 40)
	owner := NewOwnerAcceptEvent(2, "如何更换叉车轮胎", 42, 7, 5)

	if answerer.Type != NotifTypeForumAcceptAnswerer || answerer.Reason != ReasonAcceptedBonus {
		t.Fatalf("答主事件字段不符: %+v", answerer)
	}
	if owner.Type != NotifTypeForumAcceptOwner || owner.Reason != ReasonAcceptAction {
		t.Fatalf("楼主事件字段不符: %+v", owner)
	}

	// 事务内创建（与积分入账同事务形状）
	now := time.Now()
	err := db.Transaction(func(tx *gorm.DB) error {
		if err := svc.CreateForumAcceptEvent(tx, answerer, now); err != nil {
			return err
		}
		return svc.CreateForumAcceptEvent(tx, owner, now)
	})
	if err != nil {
		t.Fatalf("事务内创建失败: %v", err)
	}

	var ans, own model.Notification
	if err := db.Where("user_id = ?", 1).First(&ans).Error; err != nil {
		t.Fatalf("查询答主通知失败: %v", err)
	}
	if err := db.Where("user_id = ?", 2).First(&own).Error; err != nil {
		t.Fatalf("查询楼主通知失败: %v", err)
	}

	// 答主
	if ans.Type != "forum_accept_answerer" || ans.Title != "你的回答被采纳" {
		t.Fatalf("答主通知类型/标题不符: %+v", ans)
	}
	if !strings.Contains(ans.Content, "如何更换叉车轮胎") || !strings.Contains(ans.Content, "+40 分已到账") {
		t.Fatalf("答主通知文案应与到账积分一致: %s", ans.Content)
	}
	if ans.Link != "/training/forum/42#reply-7" {
		t.Fatalf("答主通知链接不符: %s", ans.Link)
	}
	// 楼主
	if own.Type != "forum_accept_owner" || own.Title != "你采纳了答案" {
		t.Fatalf("楼主通知类型/标题不符: %+v", own)
	}
	if !strings.Contains(own.Content, "如何更换叉车轮胎") || !strings.Contains(own.Content, "+5 分已到账") {
		t.Fatalf("楼主通知文案应与到账积分一致: %s", own.Content)
	}
	if own.Link != "/training/forum/42#reply-7" {
		t.Fatalf("楼主通知链接不符: %s", own.Link)
	}

	// payload 带实际分值
	var ansPayload map[string]any
	if err := json.Unmarshal([]byte(ans.Payload), &ansPayload); err != nil {
		t.Fatalf("答主 payload 解析失败: %v", err)
	}
	if int(ansPayload["points"].(float64)) != 40 || ansPayload["reason"] != "accepted_bonus" {
		t.Fatalf("答主 payload 分值/reason 不符: %v", ansPayload)
	}
	if int(ansPayload["topic_id"].(float64)) != 42 || int(ansPayload["reply_id"].(float64)) != 7 {
		t.Fatalf("答主 payload topic/reply 不符: %v", ansPayload)
	}
	var ownPayload map[string]any
	if err := json.Unmarshal([]byte(own.Payload), &ownPayload); err != nil {
		t.Fatalf("楼主 payload 解析失败: %v", err)
	}
	if int(ownPayload["points"].(float64)) != 5 || ownPayload["reason"] != "accept_action" {
		t.Fatalf("楼主 payload 分值/reason 不符: %v", ownPayload)
	}
}

// TestForumAcceptEventTxRollback 构造器在事务回滚时不落库（与积分入账同事务提交/回滚）。
func TestForumAcceptEventTxRollback(t *testing.T) {
	db := testutil.NewMemoryDB(t)
	svc := NewNotificationService(db, zap.NewNop())

	ev := NewAnswererAcceptEvent(1, "回滚验证", 99, 1, 40)
	err := db.Transaction(func(tx *gorm.DB) error {
		if err := svc.CreateForumAcceptEvent(tx, ev, time.Now()); err != nil {
			return err
		}
		return errors.New("模拟积分入账失败触发回滚")
	})
	if err == nil {
		t.Fatal("事务应回滚")
	}
	var cnt int64
	if err := db.Model(&model.Notification{}).Count(&cnt).Error; err != nil {
		t.Fatalf("count failed: %v", err)
	}
	if cnt != 0 {
		t.Fatalf("回滚后不应有通知, got %d", cnt)
	}
}
