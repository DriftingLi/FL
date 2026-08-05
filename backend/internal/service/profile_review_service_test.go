package service

import (
	"testing"
	"time"

	"gorm.io/gorm"

	"forklift-training/internal/model"
	"forklift-training/internal/testutil"
)

func newReviewTestSvc(t *testing.T) (*ProfileReviewService, *NotificationService, *gorm.DB) {
	t.Helper()
	db := testutil.NewMemoryDB(t)
	notifySvc := NewNotificationService(db)
	return NewProfileReviewService(db, notifySvc), notifySvc, db
}

func seedHrwaiUser(t *testing.T, db *gorm.DB, phone, email string) *model.HrwaiUser {
	t.Helper()
	u := &model.HrwaiUser{
		Username:  "hr_" + phone,
		Password:  "x",
		Name:      "测试学员",
		Nickname:  "旧昵称",
		Phone:     phone,
		Email:     email,
		Status:    1,
		CreatedAt: time.Now(),
	}
	if err := db.Create(u).Error; err != nil {
		t.Fatalf("创建学员失败: %v", err)
	}
	return u
}

func TestProfileReview_Approve_EmitsNotification(t *testing.T) {
	svc, notifySvc, db := newReviewTestSvc(t)
	user := seedHrwaiUser(t, db, "13900001111", "u1@example.com")

	req, err := svc.CreateRequest(user.ID, ProfileFieldNickname, "新昵称")
	if err != nil {
		t.Fatalf("提交审核失败: %v", err)
	}
	if _, err := svc.Approve(req.ID, 1); err != nil {
		t.Fatalf("审核通过失败: %v", err)
	}

	var n model.Notification
	if err := db.Where("user_id = ?", user.ID).First(&n).Error; err != nil {
		t.Fatalf("审核通过后应产生站内信: %v", err)
	}
	if n.Type != "profile_review" || n.Title != "资料审核通过" {
		t.Errorf("通知内容异常: %+v", n)
	}
	if !notifySvc.hasNotification(user.ID, n.ID) {
		t.Error("通知应通过站内信模块落库")
	}
	// 审核通过后资料已生效
	var after model.HrwaiUser
	_ = db.First(&after, user.ID).Error
	if after.Nickname != "新昵称" {
		t.Errorf("昵称应已生效: %q", after.Nickname)
	}
}

func TestProfileReview_Reject_EmitsNotificationWithReason(t *testing.T) {
	svc, _, db := newReviewTestSvc(t)
	user := seedHrwaiUser(t, db, "13900002222", "u2@example.com")

	req, err := svc.CreateRequest(user.ID, ProfileFieldAvatar, "/static/uploads/avatar.webp")
	if err != nil {
		t.Fatalf("提交审核失败: %v", err)
	}
	if _, err := svc.Reject(req.ID, 1, "照片不清晰"); err != nil {
		t.Fatalf("审核驳回失败: %v", err)
	}

	var n model.Notification
	if err := db.Where("user_id = ?", user.ID).First(&n).Error; err != nil {
		t.Fatalf("审核驳回后应产生站内信: %v", err)
	}
	if n.Title != "资料审核被驳回" || n.Content != "您的头像修改申请未通过审核，原因：照片不清晰。" {
		t.Errorf("驳回通知内容异常: %+v", n)
	}
}

// hasNotification 校验通知存在（测试辅助）。
func (s *NotificationService) hasNotification(userID int, id int64) bool {
	var count int64
	s.db.Model(&model.Notification{}).Where("user_id = ? AND id = ?", userID, id).Count(&count)
	return count > 0
}
