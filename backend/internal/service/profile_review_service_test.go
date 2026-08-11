package service

import (
	"context"
	"testing"
	"time"

	"go.uber.org/zap"
	"gorm.io/gorm"

	"forklift-training/internal/model"
	"forklift-training/internal/testutil"
)

// memReviewStorage 内存文件存储（统计 Delete 调用，断言头像文件清理）。
type memReviewStorage struct {
	deleted []string
}

func (m *memReviewStorage) Save(_ context.Context, _ string, _ []byte, _ string) (string, error) {
	return "https://fake-cdn/uploaded.png", nil
}

func (m *memReviewStorage) Delete(_ context.Context, url string) error {
	m.deleted = append(m.deleted, url)
	return nil
}

func (m *memReviewStorage) Exists(context.Context, string) (bool, error) { return true, nil }

func (m *memReviewStorage) List(context.Context, string) ([]string, error) { return nil, nil }

func newReviewTestSvc(t *testing.T) (*ProfileReviewService, *NotificationService, *gorm.DB, *memReviewStorage) {
	t.Helper()
	db := testutil.NewMemoryDB(t)
	notifySvc := NewNotificationService(db, zap.NewNop())
	st := &memReviewStorage{}
	return NewProfileReviewService(db, notifySvc, st, zap.NewNop()), notifySvc, db, st
}

func seedHrwaiUser(t *testing.T, db *gorm.DB, phone, email string) *model.HrwaiUser {
	t.Helper()
	u := &model.HrwaiUser{
		UID:       800000000000000000 + int64(len(phone)),
		Account:   "hr_" + phone,
		Username:  "旧昵称",
		Password:  "x",
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
	svc, notifySvc, db, _ := newReviewTestSvc(t)
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
	if after.Username != "新昵称" {
		t.Errorf("昵称应已生效: %q", after.Username)
	}
}

func TestProfileReview_Reject_EmitsNotificationWithReason(t *testing.T) {
	svc, _, db, _ := newReviewTestSvc(t)
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

// TestProfileReview_ApproveAvatar_DeletesOldAvatar 审核通过头像修改时清理被替换的旧头像
// （孤儿文件 bug 修复的回归测试：approve 路径此前不清理）。
func TestProfileReview_ApproveAvatar_DeletesOldAvatar(t *testing.T) {
	svc, _, db, st := newReviewTestSvc(t)
	user := seedHrwaiUser(t, db, "13900002222", "u2@example.com")
	user.AvatarURL = "https://fake-cdn/old-avatar.png"
	if err := db.Save(user).Error; err != nil {
		t.Fatalf("设置旧头像失败: %v", err)
	}

	req, err := svc.CreateRequest(user.ID, ProfileFieldAvatar, "https://fake-cdn/new-avatar.png")
	if err != nil {
		t.Fatalf("提交审核失败: %v", err)
	}
	if _, err := svc.Approve(req.ID, 1); err != nil {
		t.Fatalf("审核通过失败: %v", err)
	}

	// 旧头像被清理，新头像保留
	if len(st.deleted) != 1 || st.deleted[0] != "https://fake-cdn/old-avatar.png" {
		t.Errorf("应仅清理旧头像, deleted=%v", st.deleted)
	}
}

// TestProfileReview_RejectAvatar_DeletesPendingFile 驳回头像修改时清理待审文件。
func TestProfileReview_RejectAvatar_DeletesPendingFile(t *testing.T) {
	svc, _, db, st := newReviewTestSvc(t)
	user := seedHrwaiUser(t, db, "13900003333", "u3@example.com")

	req, err := svc.CreateRequest(user.ID, ProfileFieldAvatar, "https://fake-cdn/pending-avatar.png")
	if err != nil {
		t.Fatalf("提交审核失败: %v", err)
	}
	if _, err := svc.Reject(req.ID, 1, "图片不清晰"); err != nil {
		t.Fatalf("驳回失败: %v", err)
	}

	if len(st.deleted) != 1 || st.deleted[0] != "https://fake-cdn/pending-avatar.png" {
		t.Errorf("应清理待审文件, deleted=%v", st.deleted)
	}
}

// TestProfileReview_NicknameReview_NoFileCleanup 昵称审核不触发文件清理。
func TestProfileReview_NicknameReview_NoFileCleanup(t *testing.T) {
	svc, _, db, st := newReviewTestSvc(t)
	user := seedHrwaiUser(t, db, "13900004444", "u4@example.com")

	req, err := svc.CreateRequest(user.ID, ProfileFieldNickname, "新昵称")
	if err != nil {
		t.Fatalf("提交审核失败: %v", err)
	}
	if _, err := svc.Approve(req.ID, 1); err != nil {
		t.Fatalf("审核通过失败: %v", err)
	}
	if len(st.deleted) != 0 {
		t.Errorf("昵称审核不应触发文件清理, deleted=%v", st.deleted)
	}
}
