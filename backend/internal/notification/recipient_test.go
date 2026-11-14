package notification

import (
	"context"
	"errors"
	"testing"
	"time"

	"go.uber.org/zap"

	"forklift-training/internal/model"
	"forklift-training/internal/testutil"
)

// timeNow 测试用时间源（与 svc.Create 内部 time.Now 同源）。
func timeNow() time.Time { return time.Now() }

// studentRecipient 测试便捷构造：学员收件人。
func studentRecipient(id int) Recipient {
	return Recipient{Role: RecipientStudent, ID: id}
}

// tutorRecipient 测试便捷构造：讲师收件人。
func tutorRecipient(id int) Recipient {
	return Recipient{Role: RecipientTutor, ID: id}
}

// TestRecipientForRole 角色 → 收件人列的映射唯一入口（ADR-0095 决策三）。
// 学员/讲师各自分派；admin / recruiter 没有信箱 ⇒ ErrNoMailbox（不是空信箱）。
// 与 core 常量的一致性由外部测试包 recipient_vocab_test.go 钉住（包内 import core 会成环）。
func TestRecipientForRole(t *testing.T) {
	cases := []struct {
		role    string
		id      int
		want    RecipientRole
		wantErr error
	}{
		{"hrwai_user", 7, RecipientStudent, nil},
		{"tutor", 7, RecipientTutor, nil},
		{"admin", 1, "", ErrNoMailbox},
		{"recruiter", 1, "", ErrNoMailbox},
		{"", 1, "", ErrNoMailbox},
	}
	for _, c := range cases {
		got, err := RecipientForRole(c.role, c.id)
		if c.wantErr != nil {
			if !errors.Is(err, c.wantErr) {
				t.Errorf("role=%q 应拒 ErrNoMailbox，得到 err=%v", c.role, err)
			}
			continue
		}
		if err != nil {
			t.Errorf("role=%q 不应报错: %v", c.role, err)
			continue
		}
		if got.Role != c.want || got.ID != c.id {
			t.Errorf("role=%q 期望 %+v，得到 %+v", c.role, Recipient{Role: c.want, ID: c.id}, got)
		}
	}
}

// TestTutorMailboxIsolatedFromStudent 两个 id 空间不可互读（ADR-0095）。
// tutor_id=5 与 user_id=5 是两个不同的人：给讲师 5 发的信，学员 5 一条都读不到，
// 反之亦然——任何跨列兜底都会在这里判红。
func TestTutorMailboxIsolatedFromStudent(t *testing.T) {
	db := testutil.NewMemoryDB(t)
	svc := NewService(db, zap.NewNop())

	const id = 5 // 故意撞号：同一个数字在两个身份表里各指一个人

	if err := svc.CreateForTutorWithTx(db, id, NotifTypeChapterReview, "章节审核通过", "给讲师 5 的信", "/training/tutor/course/1/chapter/2", nil, timeNow()); err != nil {
		t.Fatalf("讲师通知写入失败: %v", err)
	}
	if err := svc.CreateWithTx(db, id, "system", "给学员 5 的信", "学员侧", "", nil, timeNow()); err != nil {
		t.Fatalf("学员通知写入失败: %v", err)
	}

	tutorRes, err := svc.List(context.Background(), tutorRecipient(id), 1, 10)
	if err != nil {
		t.Fatalf("讲师信箱查询失败: %v", err)
	}
	if len(tutorRes.Items) != 1 || tutorRes.Items[0].Title != "章节审核通过" {
		t.Fatalf("讲师信箱应只有讲师那一封，得到 %+v", tutorRes.Items)
	}

	studentRes, err := svc.List(context.Background(), studentRecipient(id), 1, 10)
	if err != nil {
		t.Fatalf("学员信箱查询失败: %v", err)
	}
	if len(studentRes.Items) != 1 || studentRes.Items[0].Title != "给学员 5 的信" {
		t.Fatalf("学员信箱应只有学员那一封，得到 %+v", studentRes.Items)
	}
}

// TestTutorMarkReadDoesNotCrossColumns 已读动作也不跨列：讲师不能把学员的通知标成已读。
func TestTutorMarkReadDoesNotCrossColumns(t *testing.T) {
	db := testutil.NewMemoryDB(t)
	svc := NewService(db, zap.NewNop())

	const id = 9
	if err := svc.CreateWithTx(db, id, "system", "学员的信", "内容", "", nil, timeNow()); err != nil {
		t.Fatalf("学员通知写入失败: %v", err)
	}
	var n model.Notification
	if err := db.Where("user_id = ?", id).First(&n).Error; err != nil {
		t.Fatalf("查询通知失败: %v", err)
	}

	// 讲师 9 尝试标记学员 9 的通知：按「不存在」处理（同哨兵，不泄漏存在性）
	if err := svc.MarkRead(context.Background(), tutorRecipient(id), n.ID); err == nil {
		t.Fatal("讲师不应能标记学员的通知为已读")
	}
	// 学员 9 自己可以
	if err := svc.MarkRead(context.Background(), studentRecipient(id), n.ID); err != nil {
		t.Fatalf("学员标记自己的通知应成功: %v", err)
	}
}
