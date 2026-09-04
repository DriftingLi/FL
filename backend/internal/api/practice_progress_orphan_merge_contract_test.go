// 契约测试 #510：顺序练习孤儿进度归并（pg 双跑）。
// Postgres 适配器由真实 SQL 迁移建表（含 000019 的 merge_orphan_practice_progress 函数），
// 迁移 up 成功 = 函数语法正确；本测试插入「迁移前形状」的孤儿样本后重放函数，验证归并语义：
// 桶行游标取 max、answers 桶底+孤儿覆盖、孤儿行删除、无证件学员 NULL 行保留。
package api

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/gin-gonic/gin"

	"forklift-training/internal/model"
	"forklift-training/internal/service"
	"forklift-training/internal/testutil"
)

func TestOrphanProgressMergeOnPostgres(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db := testutil.NewPostgresDB(t)
	if db == nil {
		t.Skip("DATABASE_URL 未设置")
	}

	// 三个证件 + 两个学员
	credA := model.Credential{Code: "credA", Name: "证A", Category: "special_operation"}
	if err := db.Create(&credA).Error; err != nil {
		t.Fatalf("建证A失败: %v", err)
	}
	credB := model.Credential{Code: "credB", Name: "证B", Category: "special_operation"}
	if err := db.Create(&credB).Error; err != nil {
		t.Fatalf("建证B失败: %v", err)
	}
	credC := model.Credential{Code: "credC", Name: "证C", Category: "special_operation"}
	if err := db.Create(&credC).Error; err != nil {
		t.Fatalf("建证C失败: %v", err)
	}

	pwd, _ := service.HashPassword("pass1234")

	// 学员1（有当前证件 credA）：桶行 + 孤儿行并存（#505 前崩溃形状）
	stu1 := testutil.SeedStudent(t, db, "mergeStu1", pwd)
	if err := db.Model(stu1).Update("current_credential_id", credA.ID).Error; err != nil {
		t.Fatalf("设学员1当前证件失败: %v", err)
	}
	now := time.Now()
	bucket := model.PracticeProgress{
		StudentID: stu1.ID, PracticeMode: "sequential", CredentialID: &credA.ID,
		QuestionIDs: model.JSONB([]byte("[1,2,3]")), CurrentIndex: 3, Total: 3,
		AnswersState: model.JSONB([]byte(`{"1":{"user_answer":"A","is_correct":true},"2":{"user_answer":"B","is_correct":false}}`)),
		UpdatedAt:    now,
	}
	if err := db.Create(&bucket).Error; err != nil {
		t.Fatalf("建桶行失败: %v", err)
	}
	orphan := model.PracticeProgress{
		StudentID: stu1.ID, PracticeMode: "sequential", CredentialID: nil,
		QuestionIDs: model.JSONB([]byte("[1,2,3]")), CurrentIndex: 5, Total: 5,
		AnswersState: model.JSONB([]byte(`{"1":{"user_answer":"C","is_correct":true},"3":{"user_answer":"D","is_correct":true}}`)),
		UpdatedAt:    now.Add(time.Hour),
	}
	if err := db.Create(&orphan).Error; err != nil {
		t.Fatalf("建孤儿行失败: %v", err)
	}

	// 学员2（有当前证件 credB）：只有孤儿行（桶行不存在）
	stu2 := testutil.SeedStudent(t, db, "mergeStu2", pwd)
	if err := db.Model(stu2).Update("current_credential_id", credB.ID).Error; err != nil {
		t.Fatalf("设学员2当前证件失败: %v", err)
	}
	only := model.PracticeProgress{
		StudentID: stu2.ID, PracticeMode: "sequential", CredentialID: nil,
		QuestionIDs: model.JSONB([]byte("[1,2]")), CurrentIndex: 1, Total: 2,
		AnswersState: model.JSONB([]byte(`{"1":{"user_answer":"A","is_correct":true}}`)),
		UpdatedAt:    now,
	}
	if err := db.Create(&only).Error; err != nil {
		t.Fatalf("建学员2孤儿行失败: %v", err)
	}

	// 学员3（无当前证件）：NULL 行是唯一合法行，原样保留
	stu3 := testutil.SeedStudent(t, db, "mergeStu3", pwd)
	legal := model.PracticeProgress{
		StudentID: stu3.ID, PracticeMode: "sequential", CredentialID: nil,
		QuestionIDs: model.JSONB([]byte("[1]")), CurrentIndex: 0, Total: 1,
		AnswersState: model.JSONB([]byte("{}")), UpdatedAt: now,
	}
	if err := db.Create(&legal).Error; err != nil {
		t.Fatalf("建学员3行失败: %v", err)
	}

	// 重放归并函数
	if err := db.Exec("SELECT merge_orphan_practice_progress()").Error; err != nil {
		t.Fatalf("执行归并函数失败: %v", err)
	}

	// 学员1：孤儿删除；桶行游标取 max(3,5)=5、answers 合并（桶底 + 孤儿覆盖 1 号题）
	var b1 model.PracticeProgress
	if err := db.Where("student_id = ? AND practice_mode = 'sequential' AND credential_id = ?", stu1.ID, credA.ID).First(&b1).Error; err != nil {
		t.Fatalf("学员1桶行丢失: %v", err)
	}
	if b1.CurrentIndex != 5 {
		t.Fatalf("学员1桶行游标应取 max=5, got %d", b1.CurrentIndex)
	}
	var cnt int64
	db.Model(&model.PracticeProgress{}).Where("student_id = ? AND credential_id IS NULL", stu1.ID).Count(&cnt)
	if cnt != 0 {
		t.Fatalf("学员1孤儿行应已删除, 剩 %d", cnt)
	}
	// 语义化断言：解析合并后的 answers（PG JSONB 键序/空格不参与比较）
	var merged map[string]map[string]any
	if err := json.Unmarshal(b1.AnswersState, &merged); err != nil {
		t.Fatalf("解析合并 answers 失败: %v", err)
	}
	if merged["1"]["user_answer"] != "C" || merged["2"]["user_answer"] != "B" || merged["3"]["user_answer"] != "D" {
		t.Fatalf("学员1 answers 合并错误: %s", string(b1.AnswersState))
	}
	if len(merged) != 3 {
		t.Fatalf("学员1 answers 应含 3 题, got %d: %s", len(merged), string(b1.AnswersState))
	}
	if b1.Total != 5 {
		t.Fatalf("学员1 total 应取 max=5, got %d", b1.Total)
	}

	// 学员2：孤儿改挂 credB
	var b2 model.PracticeProgress
	if err := db.Where("student_id = ? AND practice_mode = 'sequential' AND credential_id = ?", stu2.ID, credB.ID).First(&b2).Error; err != nil {
		t.Fatalf("学员2行应改挂 credB: %v", err)
	}
	if b2.CurrentIndex != 1 {
		t.Fatalf("学员2游标应保留 1, got %d", b2.CurrentIndex)
	}

	// 学员3：NULL 行原样保留
	var b3 model.PracticeProgress
	if err := db.Where("student_id = ? AND practice_mode = 'sequential' AND credential_id IS NULL", stu3.ID).First(&b3).Error; err != nil {
		t.Fatalf("学员3 NULL 行应保留: %v", err)
	}

	// 幂等：再次执行零影响
	if err := db.Exec("SELECT merge_orphan_practice_progress()").Error; err != nil {
		t.Fatalf("二次执行失败: %v", err)
	}
	db.Where("student_id = ? AND practice_mode = 'sequential' AND credential_id = ?", stu1.ID, credA.ID).First(&b1)
	if b1.CurrentIndex != 5 {
		t.Fatalf("二次执行后学员1游标应仍为 5, got %d", b1.CurrentIndex)
	}
}
