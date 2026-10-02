// 证件删除的投稿阻塞（#1360 / 真实缺陷 #12 / CONTEXT.md「证件删除的阻塞项」）。
//
// 本文件锁服务层那半（SQLite 面）：预检算出条数、句子带条数、哨兵可 errors.Is、挡住时不删证件。
// 「练习分区随证件删除」那一半是库层 ON DELETE CASCADE，SQLite 测试库不建外键（模型侧映射不出
// REFERENCES 的动作），只有 PG 契约测试与迁移文本锁能回答 —— 见
// internal/api/credential_delete_postgres_contract_test.go 与 internal/migrate/credential_fk_test.go。
package training

import (
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"go.uber.org/zap"
	"gorm.io/gorm"

	"forklift-training/internal/contribution"
	"forklift-training/internal/model"
	"forklift-training/internal/testutil"
)

func newCredDeleteSvc(t *testing.T) (*Service, *gorm.DB) {
	t.Helper()
	db := testutil.NewMemoryDB(t)
	return NewService(db, zap.NewNop()), db
}

// seedCredForDelete 建一枚证件（编码唯一，逐个错开）。
func seedCredForDelete(t *testing.T, db *gorm.DB, code string) *model.Credential {
	t.Helper()
	c := &model.Credential{Code: code, Name: "证件-" + code, Category: "special_operation", Status: 1}
	if err := db.Create(c).Error; err != nil {
		t.Fatalf("建证件失败: %v", err)
	}
	return c
}

// seedProgressForDelete 给某学员在某证件下建一行练习进度。
func seedProgressForDelete(t *testing.T, db *gorm.DB, studentID, credID int, mode string) {
	t.Helper()
	cid := credID
	p := model.PracticeProgress{
		StudentID: studentID, PracticeMode: mode, CredentialID: &cid,
		QuestionIDs: model.JSONB("[]"), AnswersState: model.JSONB("{}"), UpdatedAt: time.Now(),
	}
	if err := db.Create(&p).Error; err != nil {
		t.Fatalf("建练习进度失败: %v", err)
	}
}

// seedContributionForDelete 在某证件下建 n 篇投稿（status 可指空串用默认 pending）。
func seedContributionForDelete(t *testing.T, db *gorm.DB, userID, credID int, n int, status string) {
	t.Helper()
	for i := 0; i < n; i++ {
		st := status
		if st == "" {
			st = contribution.ContributionStatusPending
		}
		row := model.UserContribution{
			UserID: userID, CredentialID: credID, Title: fmt.Sprintf("投稿-%d-%s", credID, st),
			Intro: "夹具", Status: st, CreatedAt: time.Now(), UpdatedAt: time.Now(),
		}
		if err := db.Create(&row).Error; err != nil {
			t.Fatalf("建投稿失败: %v", err)
		}
	}
}

// TestDeleteCredentialBlockedByContributions 判据 2：有投稿时删被拒，且文案带条数。
func TestDeleteCredentialBlockedByContributions(t *testing.T) {
	svc, db := newCredDeleteSvc(t)
	student := testutil.SeedStudent(t, db, "cred_del_blocker", "x")
	cred := seedCredForDelete(t, db, "N1_block")
	other := seedCredForDelete(t, db, "N1_other")
	seedContributionForDelete(t, db, student.ID, cred.ID, 2, "")
	seedContributionForDelete(t, db, student.ID, other.ID, 3, "")

	err := svc.DeleteCredential(cred.ID)
	if err == nil {
		t.Fatal("证件下仍有投稿时删除必须被拒（投稿是内容资产，不做静默级联删投稿）")
	}
	if !errors.Is(err, ErrCredentialHasContributions) {
		t.Fatalf("错误应可被哨兵 ErrCredentialHasContributions 命中（api 侧据此落 400），实际: %v", err)
	}
	// 条数必须是**该证件**的条数（2），不是全库投稿数（5）。
	if !strings.Contains(err.Error(), "2 篇投稿") {
		t.Fatalf("文案必须带该证件下的投稿条数 2，实际: %q", err.Error())
	}
	if strings.Contains(err.Error(), "5 篇") {
		t.Fatalf("文案把别的证件的投稿算进来了: %q", err.Error())
	}
	if !strings.Contains(err.Error(), "请先迁移或下架") {
		t.Fatalf("文案要给出路（迁移或下架），实际: %q", err.Error())
	}

	// 挡住时证件还在（不是「先删了再报错」）。
	if _, err := svc.UpdateCredential(cred.ID, CredentialInput{Name: "仍在"}); err != nil {
		t.Fatalf("被拒的删除不得动到证件本身: %v", err)
	}
}

// TestDeleteCredentialBlockerCountsEveryStatus 外键不看状态：withdrawn / rejected 行同样挡住删除，
// 预检少算一档就等于放行后被数据库拿外键冲突报 500（那正是本票要消掉的形状）。
func TestDeleteCredentialBlockerCountsEveryStatus(t *testing.T) {
	svc, db := newCredDeleteSvc(t)
	student := testutil.SeedStudent(t, db, "cred_del_states", "x")
	cred := seedCredForDelete(t, db, "N1_states")
	for _, st := range []string{
		contribution.ContributionStatusPending, contribution.ContributionStatusApproved,
		contribution.ContributionStatusRejected, contribution.ContributionStatusWithdrawn, contribution.ContributionStatusArchived,
	} {
		seedContributionForDelete(t, db, student.ID, cred.ID, 1, st)
	}

	err := svc.DeleteCredential(cred.ID)
	if err == nil {
		t.Fatal("五种状态的投稿行都会挡住删除（user_contribution.credential_id 保持 NO ACTION）")
	}
	if !strings.Contains(err.Error(), "5 篇投稿") {
		t.Fatalf("计数必须覆盖全部状态，want「5 篇投稿」，实际: %q", err.Error())
	}
}

// TestDeleteCredentialWithPracticeProgressSucceeds 判据 1 的服务层一半：只有练习进度时
// 删除不报错（分区随证件消失由库层 CASCADE 负责，见 PG 契约测试）。
func TestDeleteCredentialWithPracticeProgressSucceeds(t *testing.T) {
	svc, db := newCredDeleteSvc(t)
	student := testutil.SeedStudent(t, db, "cred_del_progress", "x")
	cred := seedCredForDelete(t, db, "N1_progress")
	seedProgressForDelete(t, db, student.ID, cred.ID, "sequential")
	seedProgressForDelete(t, db, student.ID, cred.ID, "random")

	if err := svc.DeleteCredential(cred.ID); err != nil {
		t.Fatalf("只有练习进度时删证件应成功（分区随证件删除）: %v", err)
	}
	var cnt int64
	if err := db.Model(&model.Credential{}).Where("id = ?", cred.ID).Count(&cnt).Error; err != nil {
		t.Fatalf("计数失败: %v", err)
	}
	if cnt != 0 {
		t.Fatalf("证件应已删除，实际仍在 %d 行", cnt)
	}
}

// TestDeleteCredentialNotFoundIsNotMasked 预检不得把「证件不存在」吞成 200 或 400：
// 计数对不存在的证件恒为 0，删除仍走 catalogDelete 的 NotFound 分支（目录档位台账锁的同一档）。
func TestDeleteCredentialNotFoundIsNotMasked(t *testing.T) {
	svc, _ := newCredDeleteSvc(t)
	err := svc.DeleteCredential(999999)
	if !errors.Is(err, ErrCredentialNotFound) {
		t.Fatalf("不存在的证件应回 ErrCredentialNotFound（404），实际: %v", err)
	}
}
