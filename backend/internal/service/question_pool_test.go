package service

import (
	"encoding/json"
	"testing"

	"go.uber.org/zap"
	"gorm.io/gorm"

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
		q := createQuestionAs(t, qsvc, db, QuestionCreateInput{
			Type: "single_choice", Content: content, Options: json.RawMessage(`["A","B"]`), Answer: json.RawMessage(`"A"`),
			TagIDs: tagIDs, CredentialID: cred.ID,
		}, status)
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
		tags, err := catalogSvc.ListQuestionTags(true, false, tc.cred)
		if err != nil {
			t.Fatalf("%s：标签列表查询失败: %v", tc.name, err)
		}
		for _, d := range tags {
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

// TestQuestionReadScopeThreeFormsAgree scope 值对象的三个出口必须同源（ADR-0062 决策 4）：
// 链式（Apply）/ SQL 片段（WhereSQL，raw JOIN 装配点用）/ by-id 判定（VisibleByID）
// 三者对同一份数据必须给出同一行集——多形态正是历史上「常量与 raw 重写脱钩」的漂移窗口。
func TestQuestionReadScopeThreeFormsAgree(t *testing.T) {
	db := testutil.NewMemoryDB(t)
	credA, credB := 31, 32
	mk := func(content, status string, cid *int) model.Question {
		q := model.Question{Type: "single_choice", Content: content, Answer: "A", Status: status, CredentialID: cid}
		if err := db.Create(&q).Error; err != nil {
			t.Fatal(err)
		}
		return q
	}
	inA := mk("池内-A", "published", &credA)
	mk("池内-B", "published", &credB)
	mk("草稿", "draft", &credA)
	mk("待审", "pending", &credA)
	srcTag := model.QuestionTag{Code: "src", Name: "真题", IsSourceTag: true}
	if err := db.Create(&srcTag).Error; err != nil {
		t.Fatal(err)
	}
	realQ := mk("真题题", "published", &credA)
	if err := db.Create(&model.QuestionTagRelation{QuestionID: realQ.ID, TagID: srcTag.ID}).Error; err != nil {
		t.Fatal(err)
	}

	// idsViaFragment 用 WhereSQL 的片段形态跑一遍同一张表（别名固定 question）。
	idsViaFragment := func(scope QuestionReadScope) map[int]bool {
		sql, args := scope.WhereSQL()
		var rows []model.Question
		if err := db.Table("question AS question").Where(sql, args...).Find(&rows).Error; err != nil {
			t.Fatalf("片段形态查询失败: %v", err)
		}
		set := map[int]bool{}
		for _, r := range rows {
			set[r.ID] = true
		}
		return set
	}
	idsViaChain := func(scope QuestionReadScope) map[int]bool {
		var rows []model.Question
		if err := scope.Apply(db.Model(&model.Question{})).Find(&rows).Error; err != nil {
			t.Fatalf("链式形态查询失败: %v", err)
		}
		set := map[int]bool{}
		for _, r := range rows {
			set[r.ID] = true
		}
		return set
	}

	for _, tc := range []struct {
		name  string
		scope QuestionReadScope
	}{
		{"当前证件 A", NewQuestionReadScope(&credA)},
		{"未选证件 = 不分区看全部", NewQuestionReadScope(nil)},
	} {
		chain, frag := idsViaChain(tc.scope), idsViaFragment(tc.scope)
		if len(chain) != len(frag) {
			t.Fatalf("%s：链式 %v 与片段 %v 行集不同", tc.name, chain, frag)
		}
		for id := range chain {
			if !frag[id] {
				t.Fatalf("%s：片段形态漏了 %d", tc.name, id)
			}
			if !tc.scope.VisibleByID(db, id) {
				t.Fatalf("%s：by-id 判定与行集不一致（%d 在池内却判不可见）", tc.name, id)
			}
		}
	}

	// by-id 判定必须把池三元组一条不落用上（漏任何一条都会在这里红）。
	poolScoped := NewQuestionReadScope(&credA)
	if !poolScoped.VisibleByID(db, inA.ID) {
		t.Fatal("池内题应可见")
	}
	var draft, pending, other, real model.Question
	db.Where("content = ?", "草稿").First(&draft)
	db.Where("content = ?", "待审").First(&pending)
	db.Where("content = ?", "池内-B").First(&other)
	db.Where("content = ?", "真题题").First(&real)
	for name, q := range map[string]model.Question{"draft": draft, "pending": pending, "非当前证件": other, "源标记真题": real} {
		if poolScoped.VisibleByID(db, q.ID) {
			t.Fatalf("%s 题不得经 by-id 判定对学员可见", name)
		}
	}
	// 未选证件时证件那一格不分区，但 published / 排真题两条照旧。
	global := NewQuestionReadScope(nil)
	if !global.VisibleByID(db, other.ID) {
		t.Fatal("未选证件时不分区：别的证件的池内题也应可见")
	}
	if global.VisibleByID(db, real.ID) || global.VisibleByID(db, draft.ID) {
		t.Fatal("未选证件不得放宽 published / 排真题两条")
	}

	// 编辑面 scope：只有筛选轴，没有池（这是它与学员面的差别，不得被并成一条）。
	var edited []model.Question
	if err := NewQuestionEditScope(&credA).ApplyListFilter(db.Model(&model.Question{})).Find(&edited).Error; err != nil {
		t.Fatal(err)
	}
	if len(edited) != 4 { // 草稿 + 待审 + 池内-A + 真题题：编辑面全量，只按证件筛
		t.Fatalf("编辑面应读到该证件全量 4 题（含 draft/pending/真题）, got %d", len(edited))
	}
	if n := countAll(db, NewQuestionEditScope(nil)); n != 5 {
		t.Fatalf("编辑面无筛选轴时应看全部 5 题, got %d", n)
	}
}

func countAll(db *gorm.DB, scope QuestionEditScope) int {
	var n int64
	if err := scope.ApplyListFilter(db.Model(&model.Question{})).Count(&n).Error; err != nil {
		return -1
	}
	return int(n)
}
