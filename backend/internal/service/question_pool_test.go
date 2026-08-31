package service

import (
	"testing"

	"forklift-training/internal/model"
	"forklift-training/internal/testutil"
)

// TestPoolCountConsistency 池计数单点（#413）：同参下 countPoolByOpts 与抽题数量一致，
// 并验证池三元组（已发布 + 排除来源标记标签 + 证件分区）。
func TestPoolCountConsistency(t *testing.T) {
	db := testutil.NewMemoryDB(t)
	cred1 := 1
	cred2 := 2
	for i := 0; i < 3; i++ {
		q := model.Question{Type: "single_choice", Content: "c1", Answer: "A", Status: "published", CredentialID: &cred1}
		if err := db.Create(&q).Error; err != nil {
			t.Fatal(err)
		}
	}
	for i := 0; i < 2; i++ {
		q := model.Question{Type: "single_choice", Content: "c2", Answer: "A", Status: "published", CredentialID: &cred2}
		if err := db.Create(&q).Error; err != nil {
			t.Fatal(err)
		}
	}
	// 来源标记标签（真题）题：published 但必须被池排除
	srcTag := model.QuestionTag{Code: "real", Name: "真题", IsSourceTag: true}
	if err := db.Create(&srcTag).Error; err != nil {
		t.Fatal(err)
	}
	realQ := model.Question{Type: "single_choice", Content: "real", Answer: "A", Status: "published", CredentialID: &cred1}
	if err := db.Create(&realQ).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&model.QuestionTagRelation{QuestionID: realQ.ID, TagID: srcTag.ID}).Error; err != nil {
		t.Fatal(err)
	}
	// 草稿不进池
	if err := db.Create(&model.Question{Type: "single_choice", Content: "draft", Answer: "A", Status: "draft", CredentialID: &cred1}).Error; err != nil {
		t.Fatal(err)
	}

	// 同参一致性：计数 == 抽题数量
	o := sampleQuestionsOpts{cred: &cred1}
	cnt, err := countPoolByOpts(db, o)
	if err != nil {
		t.Fatal(err)
	}
	all, err := sampleQuestionsByOpts(db, o)
	if err != nil {
		t.Fatal(err)
	}
	if int(cnt) != len(all) {
		t.Fatalf("计数与抽题数量不一致: count=%d len=%d", cnt, len(all))
	}
	// 证件1池 = 3 已发布普通题（真题与 draft 被排除）
	if cnt != 3 {
		t.Fatalf("证件1池应为 3, got %d", cnt)
	}
	// 证件分区：不带证件参数与带不同证件参数返回不同集合
	noCred, err := sampleQuestionsByOpts(db, sampleQuestionsOpts{})
	if err != nil {
		t.Fatal(err)
	}
	cred2All, err := sampleQuestionsByOpts(db, sampleQuestionsOpts{cred: &cred2})
	if err != nil {
		t.Fatal(err)
	}
	if len(noCred) != 5 || len(cred2All) != 2 {
		t.Fatalf("无证件池应为 5（全部 published），证件2池应为 2；got %d / %d", len(noCred), len(cred2All))
	}
	// 真题题必须不在池内（全池隔离语义不变）
	for _, q := range noCred {
		if q.ID == realQ.ID {
			t.Fatal("真题题不应出现在池内")
		}
	}
}
