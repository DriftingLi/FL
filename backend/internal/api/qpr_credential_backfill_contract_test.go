// 契约测试 #1007 / ADR-0051：练习记录的证件分区回填（pg 双跑）。
//
// Postgres 适配器由真实 SQL 迁移建表（含 000033 的 backfill_qpr_credential 函数），
// 迁移 up 成功 = 函数语法正确；本测试插入「迁移前形状」（credential_id 全为 NULL）的样本后重放函数，
// 验证回填语义：按题目反推落证件、题目未分区的记录保持 NULL、重放幂等。
package api

import (
	"testing"
	"time"

	"github.com/gin-gonic/gin"

	"forklift-training/internal/model"
	"forklift-training/internal/testutil"
)

func TestQPRCredentialBackfillOnPostgres(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db := testutil.NewPostgresDB(t)
	if db == nil {
		t.Skip("DATABASE_URL 未设置")
	}

	credA := model.Credential{Code: "qprBfA", Name: "证A", Category: "special_operation", Status: 1}
	credB := model.Credential{Code: "qprBfB", Name: "证B", Category: "special_operation", Status: 1}
	for _, c := range []*model.Credential{&credA, &credB} {
		if err := db.Create(c).Error; err != nil {
			t.Fatalf("建证件失败: %v", err)
		}
	}

	qA := testutil.SeedQuestion(t, db, "single", "A 证题", "A")
	qB := testutil.SeedQuestion(t, db, "single", "B 证题", "A")
	qNone := testutil.SeedQuestion(t, db, "single", "未分区题", "A")
	for q, cred := range map[*model.Question]*int{qA: &credA.ID, qB: &credB.ID, qNone: nil} {
		if err := db.Model(q).Update("credential_id", cred).Error; err != nil {
			t.Fatalf("题目挂证件失败: %v", err)
		}
	}

	student := testutil.SeedStudent(t, db, "qprBfStu", "x")
	now := time.Now()
	recA := model.QuestionPracticeRecord{StudentID: student.ID, QuestionID: qA.ID, IsCorrect: true, PracticeType: "free", CreatedAt: now}
	recB := model.QuestionPracticeRecord{StudentID: student.ID, QuestionID: qB.ID, IsCorrect: true, PracticeType: "free", CreatedAt: now}
	recNone := model.QuestionPracticeRecord{StudentID: student.ID, QuestionID: qNone.ID, IsCorrect: true, PracticeType: "free", CreatedAt: now}
	// 迁移前形状：显式不带分区
	for _, rec := range []*model.QuestionPracticeRecord{&recA, &recB, &recNone} {
		if err := db.Create(rec).Error; err != nil {
			t.Fatalf("插入练习记录失败: %v", err)
		}
	}

	backfill := func() {
		t.Helper()
		if err := db.Exec("SELECT backfill_qpr_credential()").Error; err != nil {
			t.Fatalf("重放回填函数失败: %v", err)
		}
	}
	assertCred := func(name string, id int, want *int) {
		t.Helper()
		var got model.QuestionPracticeRecord
		if err := db.First(&got, id).Error; err != nil {
			t.Fatalf("查记录 %d 失败: %v", id, err)
		}
		switch {
		case want == nil && got.CredentialID != nil:
			t.Fatalf("%s: 应保持未分区, got %d", name, *got.CredentialID)
		case want != nil && got.CredentialID == nil:
			t.Fatalf("%s: 应落证件 %d, got NULL", name, *want)
		case want != nil && got.CredentialID != nil && *got.CredentialID != *want:
			t.Fatalf("%s: 应落证件 %d, got %d", name, *want, *got.CredentialID)
		}
	}

	backfill()
	assertCred("按题目反推（A 证）", recA.ID, &credA.ID)
	assertCred("按题目反推（B 证）", recB.ID, &credB.ID)
	assertCred("题目未分区 → 记录保持 NULL", recNone.ID, nil)

	// 幂等：重放不改变结果（函数保留不删的意义即在此）
	backfill()
	assertCred("重放后（幂等）", recA.ID, &credA.ID)
	assertCred("重放后（幂等）", recNone.ID, nil)
}
