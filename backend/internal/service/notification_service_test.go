package service

import (
	"errors"
	"testing"
	"time"

	"go.uber.org/zap"
	"gorm.io/gorm"

	"forklift-training/internal/testutil"
)

func TestNotificationService_CreateAndList(t *testing.T) {
	db := testutil.NewMemoryDB(t)
	svc := NewNotificationService(db, zap.NewNop())
	student := testutil.SeedStudent(t, db, "notify_user", "x")
	uid := student.StudentID

	if err := svc.Create(uid, "profile_review", "资料审核通过", "您的昵称修改已生效", ""); err != nil {
		t.Fatalf("创建通知失败: %v", err)
	}
	if err := svc.Create(uid, "profile_review", "资料审核被驳回", "原因：不符合要求", ""); err != nil {
		t.Fatalf("创建通知失败: %v", err)
	}

	result, err := svc.List(uid, 1, 10)
	if err != nil {
		t.Fatalf("查询通知失败: %v", err)
	}
	items, ok := result["items"].([]NotificationDTO)
	if !ok || len(items) != 2 {
		t.Fatalf("期望 2 条通知，得到 %v", items)
	}
	if items[0].Title != "资料审核被驳回" || items[0].IsRead {
		t.Errorf("最新通知应为未读的驳回通知: %+v", items[0])
	}
	if unread, ok := result["unread_count"].(int64); !ok || unread != 2 {
		t.Errorf("期望未读数 2，得到 %v", result["unread_count"])
	}

	count, err := svc.UnreadCount(uid)
	if err != nil || count != 2 {
		t.Fatalf("期望未读数 2，得到 %d, err=%v", count, err)
	}

	// 其他用户查询不到任何通知
	other := testutil.SeedStudent(t, db, "notify_other", "x")
	count, err = svc.UnreadCount(other.StudentID)
	if err != nil || count != 0 {
		t.Fatalf("其他用户未读数应为 0，得到 %d, err=%v", count, err)
	}
}

func TestNotificationService_MarkRead(t *testing.T) {
	db := testutil.NewMemoryDB(t)
	svc := NewNotificationService(db, zap.NewNop())
	student := testutil.SeedStudent(t, db, "notify_mark", "x")
	uid := student.StudentID

	if err := svc.Create(uid, "system", "标题", "内容", "/training"); err != nil {
		t.Fatalf("创建通知失败: %v", err)
	}
	if err := svc.Create(uid, "system", "标题2", "内容2", ""); err != nil {
		t.Fatalf("创建通知失败: %v", err)
	}
	var ids []int64
	result, err := svc.List(uid, 1, 10)
	if err != nil {
		t.Fatalf("查询通知失败: %v", err)
	}
	for _, item := range result["items"].([]NotificationDTO) {
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
	uid := student.StudentID

	// 事务内写入并提交 → 通知落库
	err := db.Transaction(func(tx *gorm.DB) error {
		return svc.CreateWithTx(tx, uid, "profile_review", "资料审核通过", "您的昵称修改已生效", "", time.Now())
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
		if err := svc.CreateWithTx(tx, uid, "profile_review", "资料审核被驳回", "原因：不符合要求", "", time.Now()); err != nil {
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
