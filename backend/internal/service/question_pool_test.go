package service

import (
	"testing"

	"go.uber.org/zap"

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

// TestQuestionPoolScopeCoversTagCountAndSearch 题库池 scope 单点落到「标签计数」与
// 「搜索题目分区」两个读路径（ADR-0050 决策 1）：四条读路径（作答抽题 / 搜索结果 /
// 按 id 取详情 / 标签计数）同走一个 scope——draft 与源标记真题题在任何入口都不可见。
// 本用例是 question_pool_test.go 池三元组断言向两个新落点的扩展。
func TestQuestionPoolScopeCoversTagCountAndSearch(t *testing.T) {
	db := testutil.NewMemoryDB(t)
	catalogSvc := NewTrainingCatalogService(db, zap.NewNop())
	qsvc := NewQuestionBankService(db, nil, zap.NewNop())
	searchSvc := NewSearchService(db, zap.NewNop())

	tag, _ := catalogSvc.CreateQuestionTag(QuestionTagInput{Code: "hydraulic", Name: "液压"})
	srcTag, _ := catalogSvc.CreateQuestionTag(QuestionTagInput{Code: "real_exam", Name: "真题"})
	if err := db.Model(&model.QuestionTag{}).Where("id = ?", srcTag.ID).Update("is_source_tag", true).Error; err != nil {
		t.Fatalf("置 source 标签失败: %v", err)
	}

	cred := &model.Credential{Code: "forklift_n1", Name: "N1证"}
	if err := db.Create(cred).Error; err != nil {
		t.Fatalf("建证件失败: %v", err)
	}
	mk := func(status string, tagIDs []int, content string) int {
		t.Helper()
		q, err := qsvc.CreateQuestion(map[string]any{
			"type": "single_choice", "content": content, "options": []string{"A", "B"}, "answer": "A",
			"status": status, "tag_ids": tagIDs, "credential_id": cred.ID,
		}, nil, "tutor")
		if err != nil {
			t.Fatalf("建题失败: %v", err)
		}
		return q.ID
	}
	inPool := mk("published", []int{tag.ID}, "液压泵池内题")
	mk("draft", []int{tag.ID}, "液压泵草稿题")
	mk("published", []int{srcTag.ID}, "液压泵真题题")
	// 同带主题标签与来源标记标签：源标记排除必须压过主题标签（无证件分区时的漂移点）
	mk("published", []int{tag.ID, srcTag.ID}, "液压泵双标签真题题")

	// 落点一：catalog 标签池计数 = 池口径（1 道池内题；草稿与两道源标记题都不计）。
	// 带证件与不带证件两个分支同源——不带证件（全局池）同样排源标记题，不再有漂移窗口。
	for _, tc := range []struct {
		name string
		cred *int
	}{
		{"带证件分区", &cred.ID},
		{"不带证件全局", nil},
	} {
		counts := map[int]int64{}
		for _, d := range catalogSvc.ListQuestionTags(true, false, tc.cred) {
			if d.QuestionCount != nil {
				counts[d.ID] = *d.QuestionCount
			}
		}
		if counts[tag.ID] != 1 {
			t.Fatalf("%s：标签计数应走池口径 = 1（草稿与源标记题不计）, got %d", tc.name, counts[tag.ID])
		}
	}

	// 落点二：搜索题目分区 = 池口径（只有池内题命中，且命中数与返回条目一致）
	got, err := searchSvc.Search("液压泵", SearchTypeQuestion, 1, 20, &cred.ID)
	if err != nil {
		t.Fatalf("题目分区搜索失败: %v", err)
	}
	page := got.(*SearchPageDTO)
	if page.Total != 1 || len(page.Items) != 1 || page.Items[0].ID != int64(inPool) {
		t.Fatalf("题目分区应只含池内题 %d, got total=%d items=%+v", inPool, page.Total, page.Items)
	}
}
