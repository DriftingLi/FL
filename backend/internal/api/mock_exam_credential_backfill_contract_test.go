// 契约测试 #1003：模考记录证件分区回填（pg 双跑）。
//
// Postgres 适配器由真实 SQL 迁移建表（含 000032 的 backfill_mock_exam_credential 函数），
// 迁移 up 成功 = 函数语法正确；本测试插入「迁移前形状」（credential_id IS NULL）的样本后
// 重放函数，验证两阶段回填语义：
//
//	阶段一：题目集内证件唯一 → 落该证件（精确证据优先，与学员当前证件无关）；
//	阶段二：题目反推不出（题目已删 / 跨证件）→ 挂学员当前证件（照 000013 口径）；
//	学员无当前证件且无题目证据 → 留 NULL（不分区）；重放幂等。
package api

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/gin-gonic/gin"

	"forklift-training/internal/model"
	"forklift-training/internal/testutil"
)

func TestMockExamCredentialBackfillOnPostgres(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db := testutil.NewPostgresDB(t)
	if db == nil {
		t.Skip("DATABASE_URL 未设置")
	}

	credA := model.Credential{Code: "bfA", Name: "证A", Category: "special_operation", Status: 1}
	credB := model.Credential{Code: "bfB", Name: "证B", Category: "special_operation", Status: 1}
	for _, c := range []*model.Credential{&credA, &credB} {
		if err := db.Create(c).Error; err != nil {
			t.Fatalf("建证件失败: %v", err)
		}
	}

	// 题目证据：两道 A 证件题、一道 B 证件题
	qA1 := testutil.SeedQuestion(t, db, "single", "A 题一", "A")
	qA2 := testutil.SeedQuestion(t, db, "single", "A 题二", "A")
	qB1 := testutil.SeedQuestion(t, db, "single", "B 题", "A")
	for q, cred := range map[*model.Question]int{qA1: credA.ID, qA2: credA.ID, qB1: credB.ID} {
		if err := db.Model(q).Update("credential_id", cred).Error; err != nil {
			t.Fatalf("题目挂证件失败: %v", err)
		}
	}

	// 学员1：当前证件 A；学员2：未选证件
	stu1 := testutil.SeedStudent(t, db, "bfStu1", "x")
	if err := db.Model(stu1).Update("current_credential_id", credA.ID).Error; err != nil {
		t.Fatalf("设学员1当前证件失败: %v", err)
	}
	stu2 := testutil.SeedStudent(t, db, "bfStu2", "x")

	now := time.Now()
	ids := func(v ...int) model.JSONB {
		b, _ := json.Marshal(v) // 试卷题目 ID 顺序序列化
		return model.JSONB(b)
	}
	pureA := model.MockExam{StudentID: stu1.ID, Status: "submitted", QuestionIDs: ids(qA1.ID, qA2.ID), StartTime: &now, SubmitTime: &now, CreatedAt: now}
	mixed := model.MockExam{StudentID: stu1.ID, Status: "submitted", QuestionIDs: ids(qA1.ID, qB1.ID), StartTime: &now, SubmitTime: &now, CreatedAt: now}
	gone := model.MockExam{StudentID: stu1.ID, Status: "submitted", QuestionIDs: ids(999999), StartTime: &now, SubmitTime: &now, CreatedAt: now}
	noEvidence := model.MockExam{StudentID: stu2.ID, Status: "submitted", QuestionIDs: ids(999999), StartTime: &now, SubmitTime: &now, CreatedAt: now}
	for _, m := range []*model.MockExam{&pureA, &mixed, &gone, &noEvidence} {
		if err := db.Create(m).Error; err != nil {
			t.Fatalf("插入模拟考试失败: %v", err)
		}
	}

	backfill := func() {
		t.Helper()
		if err := db.Exec("SELECT backfill_mock_exam_credential()").Error; err != nil {
			t.Fatalf("重放回填函数失败: %v", err)
		}
	}
	credOf := func(id int) *int {
		t.Helper()
		var got model.MockExam
		if err := db.First(&got, id).Error; err != nil {
			t.Fatalf("查记录 %d 失败: %v", id, err)
		}
		return got.CredentialID
	}
	assertCred := func(name string, id int, want *int) {
		t.Helper()
		got := credOf(id)
		switch {
		case want == nil && got != nil:
			t.Fatalf("%s: 应保持未分区, got %d", name, *got)
		case want != nil && got == nil:
			t.Fatalf("%s: 应落证件 %d, got NULL", name, *want)
		case want != nil && got != nil && *got != *want:
			t.Fatalf("%s: 应落证件 %d, got %d", name, *want, *got)
		}
	}

	backfill()
	assertCred("题目集内证件唯一 → 按题目反推", pureA.ID, &credA.ID)
	assertCred("跨证件题目 → 阶段二挂学员当前证件", mixed.ID, &credA.ID)
	assertCred("题目已删 → 阶段二挂学员当前证件", gone.ID, &credA.ID)
	assertCred("无当前证件且无题目证据 → 留 NULL", noEvidence.ID, nil)

	// 幂等：重放不改变结果（函数保留不删的意义即在此）
	backfill()
	assertCred("重放后（幂等）", pureA.ID, &credA.ID)
	assertCred("重放后（幂等）", gone.ID, &credA.ID)
	assertCred("重放后（幂等）", noEvidence.ID, nil)
}
