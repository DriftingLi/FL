// Package service 笔记域测试（ADR-0055）：题目笔记与独立笔记共用一张 note 表。
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

// studentScope 测试用学员题目读 scope：未选证件（nil）= 池不分区、看全部。
// 夹具题（testutil.SeedQuestion）是 published 且无证件、无源标记 ⇒ 恒在池内。
func studentScope() QuestionReadScope { return NewQuestionReadScope(nil) }

func newNoteSvc(t *testing.T) (*NoteService, *gorm.DB) {
	t.Helper()
	db := testutil.NewMemoryDB(t)
	return NewNoteService(db, zap.NewNop()), db
}

// TestNoteStandaloneCanBeMultiple 独立笔记可多条（question_id 为空时唯一约束不生效——
// Postgres 唯一索引里 NULL 互不冲突，这条正是「每题一条」与「独立笔记多条」能同时成立的依据）。
func TestNoteStandaloneCanBeMultiple(t *testing.T) {
	svc, _ := newNoteSvc(t)
	for _, c := range []string{"第一条", "第二条", "第三条"} {
		if _, err := svc.Create(1, c); err != nil {
			t.Fatalf("建独立笔记失败(%s): %v", c, err)
		}
	}
	page, err := svc.List(1, NoteScopeStandalone, 1, 20, studentScope())
	if err != nil {
		t.Fatalf("列独立笔记失败: %v", err)
	}
	if page.Total != 3 || len(page.Items) != 3 {
		t.Fatalf("独立笔记应 3 条, got total=%d len=%d", page.Total, len(page.Items))
	}
	for _, it := range page.Items {
		if it.QuestionID != nil {
			t.Fatalf("独立笔记 question_id 应为 null, got %v", *it.QuestionID)
		}
	}
}

// TestNoteQuestionNoteStaysSingle 题目笔记仍是「每人每题一条」：重复 Upsert 是更新不是新增。
func TestNoteQuestionNoteStaysSingle(t *testing.T) {
	svc, db := newNoteSvc(t)
	q := testutil.SeedQuestion(t, db, "single_choice", "题干", "A")
	if _, err := svc.UpsertForQuestion(q.ID, 1, "第一版", studentScope()); err != nil {
		t.Fatalf("首次保存失败: %v", err)
	}
	if _, err := svc.UpsertForQuestion(q.ID, 1, "第二版", studentScope()); err != nil {
		t.Fatalf("二次保存失败: %v", err)
	}
	var cnt int64
	db.Model(&model.Note{}).Where("user_id = ? AND question_id = ?", 1, q.ID).Count(&cnt)
	if cnt != 1 {
		t.Fatalf("每题应恰一条, got %d", cnt)
	}
	got, err := svc.GetForQuestion(q.ID, 1, studentScope())
	if err != nil || got == nil {
		t.Fatalf("读取失败: %v", err)
	}
	if got.Content != "第二版" {
		t.Fatalf("应为最新正文, got %q", got.Content)
	}
}

// TestNoteListScopeAndOrder 「我的笔记」列表：scope 筛选 + 按 updated_at 倒序 +
// 题目笔记带回题干摘要（一次 JOIN，禁 N+1）。
func TestNoteListScopeAndOrder(t *testing.T) {
	svc, db := newNoteSvc(t)
	q := testutil.SeedQuestion(t, db, "single_choice", "液压泵异响的判断", "A")
	if _, err := svc.Create(1, "独立笔记较早"); err != nil {
		t.Fatal(err)
	}
	// 拉开时间：Create 取 beijingNow()，这里直接改库里的 updated_at 以稳定断言
	if err := db.Model(&model.Note{}).Where("user_id = ? AND question_id IS NULL", 1).
		UpdateColumn("updated_at", time.Now().Add(-2*time.Hour)).Error; err != nil {
		t.Fatal(err)
	}
	if _, err := svc.UpsertForQuestion(q.ID, 1, "题目笔记较新", studentScope()); err != nil {
		t.Fatal(err)
	}

	all, err := svc.List(1, NoteScopeAll, 1, 20, studentScope())
	if err != nil {
		t.Fatalf("列全部失败: %v", err)
	}
	if all.Total != 2 {
		t.Fatalf("全部应 2 条, got %d", all.Total)
	}
	if all.Items[0].QuestionID == nil {
		t.Fatalf("倒序首条应为较新的题目笔记, got %+v", all.Items[0])
	}
	if all.Items[0].QuestionContent != "液压泵异响的判断" {
		t.Fatalf("题目笔记应带回题干摘要, got %q", all.Items[0].QuestionContent)
	}

	qs, _ := svc.List(1, NoteScopeQuestion, 1, 20, studentScope())
	if qs.Total != 1 || qs.Items[0].QuestionID == nil {
		t.Fatalf("scope=question 应只 1 条题目笔记, got total=%d", qs.Total)
	}
	ss, _ := svc.List(1, NoteScopeStandalone, 1, 20, studentScope())
	if ss.Total != 1 || ss.Items[0].QuestionID != nil {
		t.Fatalf("scope=standalone 应只 1 条独立笔记, got total=%d", ss.Total)
	}
}

// TestNoteOwnershipIsolation 越权隔离：他人笔记按「不存在」处理，不泄漏存在性。
func TestNoteOwnershipIsolation(t *testing.T) {
	svc, db := newNoteSvc(t)
	n, err := svc.Create(1, "甲的笔记")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := svc.Update(n.ID, 2, "乙来改"); !errors.Is(err, ErrNoteNotFound) {
		t.Fatalf("改他人笔记应报 ErrNoteNotFound, got %v", err)
	}
	if err := svc.Delete(n.ID, 2); !errors.Is(err, ErrNoteNotFound) {
		t.Fatalf("删他人笔记应报 ErrNoteNotFound, got %v", err)
	}
	// 甲的笔记原封不动
	var got model.Note
	if err := db.First(&got, n.ID).Error; err != nil {
		t.Fatalf("甲的笔记不应被删: %v", err)
	}
	if got.Content != "甲的笔记" {
		t.Fatalf("甲的笔记不应被改, got %q", got.Content)
	}
	// 列表也只看得到自己的
	page, _ := svc.List(2, NoteScopeAll, 1, 20, studentScope())
	if page.Total != 0 {
		t.Fatalf("乙应看不到甲的笔记, got total=%d", page.Total)
	}
}

// TestNoteContentValidation 正文校验：空 / 超长拒绝（沿用既有字节口径）。
func TestNoteContentValidation(t *testing.T) {
	svc, _ := newNoteSvc(t)
	if _, err := svc.Create(1, "   "); err == nil {
		t.Fatal("空白正文应被拒绝")
	}
	long := make([]byte, 2001)
	for i := range long {
		long[i] = 'a'
	}
	if _, err := svc.Create(1, string(long)); err == nil {
		t.Fatal("超长正文应被拒绝")
	}
}
