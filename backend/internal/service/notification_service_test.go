package service

import (
	"errors"
	"strings"
	"testing"
	"time"

	"go.uber.org/zap"
	"gorm.io/gorm"

	"forklift-training/internal/model"
	"forklift-training/internal/testutil"
)

func TestNotificationService_CreateAndList(t *testing.T) {
	db := testutil.NewMemoryDB(t)
	svc := NewNotificationService(db, zap.NewNop())
	student := testutil.SeedStudent(t, db, "notify_user", "x")
	uid := student.ID

	if err := svc.Create(uid, "profile_review", "资料审核通过", "您的昵称修改已生效", "", model.JSONB([]byte("{\"review_status\":\"approved\"}"))); err != nil {
		t.Fatalf("创建通知失败: %v", err)
	}
	if err := svc.Create(uid, "profile_review", "资料审核被驳回", "原因：不符合要求", "", model.JSONB([]byte("{\"review_status\":\"rejected\"}"))); err != nil {
		t.Fatalf("创建通知失败: %v", err)
	}

	result, err := svc.List(uid, 1, 10)
	if err != nil {
		t.Fatalf("查询通知失败: %v", err)
	}
	items := result.Items
	if len(items) != 2 {
		t.Fatalf("期望 2 条通知，得到 %v", items)
	}
	if items[0].Title != "资料审核被驳回" || items[0].IsRead {
		t.Errorf("最新通知应为未读的驳回通知: %+v", items[0])
	}
	if string(items[0].Payload) != "{\"review_status\":\"rejected\"}" {
		t.Errorf("驳回通知 payload 应携带 review_status=rejected: %s", items[0].Payload)
	}
	if string(items[1].Payload) != "{\"review_status\":\"approved\"}" {
		t.Errorf("通过通知 payload 应携带 review_status=approved: %s", items[1].Payload)
	}
	if result.UnreadCount != 2 {
		t.Errorf("期望未读数 2，得到 %v", result.UnreadCount)
	}
	if result.Pages != 1 {
		t.Errorf("2 条通知每页 10 条应 1 页，得到 %d", result.Pages)
	}

	count, err := svc.UnreadCount(uid)
	if err != nil || count != 2 {
		t.Fatalf("期望未读数 2，得到 %d, err=%v", count, err)
	}

	// 其他用户查询不到任何通知
	other := testutil.SeedStudent(t, db, "notify_other", "x")
	count, err = svc.UnreadCount(other.ID)
	if err != nil || count != 0 {
		t.Fatalf("其他用户未读数应为 0，得到 %d, err=%v", count, err)
	}
}

// TestNotificationService_ProfileReviewPayload 审核通知结构标记（Ticket #228）：
// ProfileReviewNotification 在标题不变的前提下，payload 携带结构化 review_status 判定。
func TestNotificationService_ProfileReviewPayload(t *testing.T) {
	svc := NewNotificationService(nil, zap.NewNop())

	req := &model.ProfileChangeRequest{FieldType: ProfileFieldNickname}
	typ, title, content, payload := svc.ProfileReviewNotification(req, ProfileStatusApproved, "")
	if typ != "profile_review" || title != "资料审核通过" {
		t.Errorf("审核通过通知 type/title 不符: type=%s title=%s", typ, title)
	}
	if !strings.Contains(content, "您的昵称修改已通过审核") {
		t.Errorf("通过通知正文不符: %s", content)
	}
	if string(payload) != "{\"review_status\":\"approved\"}" {
		t.Errorf("通过通知 payload 应为 approved，得到: %s", payload)
	}

	req.FieldType = ProfileFieldAvatar
	typ, title, content, payload = svc.ProfileReviewNotification(req, ProfileStatusRejected, "照片不清晰")
	if typ != "profile_review" || title != "资料审核被驳回" {
		t.Errorf("审核驳回通知 type/title 不符: type=%s title=%s", typ, title)
	}
	if !strings.Contains(content, "您的头像修改申请未通过审核，原因：照片不清晰。") {
		t.Errorf("驳回通知正文不符: %s", content)
	}
	if string(payload) != "{\"review_status\":\"rejected\"}" {
		t.Errorf("驳回通知 payload 应为 rejected，得到: %s", payload)
	}
}

func TestNotificationService_MarkRead(t *testing.T) {
	db := testutil.NewMemoryDB(t)
	svc := NewNotificationService(db, zap.NewNop())
	student := testutil.SeedStudent(t, db, "notify_mark", "x")
	uid := student.ID

	if err := svc.Create(uid, "system", "标题", "内容", "/training", nil); err != nil {
		t.Fatalf("创建通知失败: %v", err)
	}
	if err := svc.Create(uid, "system", "标题2", "内容2", "", nil); err != nil {
		t.Fatalf("创建通知失败: %v", err)
	}
	var ids []int64
	result, err := svc.List(uid, 1, 10)
	if err != nil {
		t.Fatalf("查询通知失败: %v", err)
	}
	for _, item := range result.Items {
		ids = append(ids, item.ID)
	}
	if len(ids) != 2 {
		t.Fatalf("期望 2 条通知")
	}

	if err := svc.MarkRead(uid, ids[0]); err != nil {
		t.Fatalf("标记已读失败: %v", err)
	}
	// 重复标记已读应幂等成功
	if err := svc.MarkRead(uid, ids[0]); err != nil {
		t.Fatalf("重复标记已读应成功: %v", err)
	}
	// 不存在的通知应报错
	if err := svc.MarkRead(uid, 99999); err == nil {
		t.Error("不存在的通知应报错")
	}

	count, _ := svc.UnreadCount(uid)
	if count != 1 {
		t.Fatalf("期望剩余未读数 1，得到 %d", count)
	}

	if err := svc.MarkAllRead(uid); err != nil {
		t.Fatalf("全部已读失败: %v", err)
	}
	count, _ = svc.UnreadCount(uid)
	if count != 0 {
		t.Fatalf("全部已读后未读数应为 0，得到 %d", count)
	}
}

func TestNotificationService_CreateWithTx_CommitAndRollback(t *testing.T) {
	db := testutil.NewMemoryDB(t)
	svc := NewNotificationService(db, zap.NewNop())
	student := testutil.SeedStudent(t, db, "notify_tx", "x")
	uid := student.ID

	// 事务内写入并提交 → 通知落库
	err := db.Transaction(func(tx *gorm.DB) error {
		return svc.CreateWithTx(tx, uid, "profile_review", "资料审核通过", "您的昵称修改已生效", "", nil, time.Now())
	})
	if err != nil {
		t.Fatalf("事务内创建通知失败: %v", err)
	}
	count, _ := svc.UnreadCount(uid)
	if count != 1 {
		t.Fatalf("提交后应落库 1 条，得到 %d", count)
	}

	// 事务内写入后回滚 → 通知不落库（与业务写同事务的原子性）
	err = db.Transaction(func(tx *gorm.DB) error {
		if err := svc.CreateWithTx(tx, uid, "profile_review", "资料审核被驳回", "原因：不符合要求", "", nil, time.Now()); err != nil {
			return err
		}
		return errors.New("模拟业务失败回滚")
	})
	if err == nil {
		t.Fatal("模拟失败应返回错误")
	}
	count, _ = svc.UnreadCount(uid)
	if count != 1 {
		t.Fatalf("回滚后不应新增通知，得到 %d", count)
	}
}
