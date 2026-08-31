// Package service #385 回归：答题会话 ResumeSet/SaveSet 断点续练协商语义
// 与抽题池单点（sampleQuestionsByOpts）。
package service

import (
	"testing"

	"go.uber.org/zap"

	"forklift-training/internal/model"
	"forklift-training/internal/testutil"
)

// TestResumeSetTagSemantics 标签练习协商：首次进入抽样固定顺序；同集续练沿用
// 已存顺序与游标；集合变化刷新顺序并复位游标（中途变化不重抽样）；练完后重进重新抽样。
// 注：count 抽样（3 抽 2）后二次进入按既有语义刷新为全量（saved 与现集合不同集），
// 续练路径以同集断言。
func TestResumeSetTagSemantics(t *testing.T) {
	db := testutil.NewMemoryDB(t)
	mode := "tag:7"

	// 首次进入（count 抽样：3 抽 2）：固定顺序、游标 0
	ids1, idx, err := ResumeSet(db, 1, ResumeSetSpec{
		Mode: mode, FreshIDs: []int{11, 22, 33}, ReuseSaved: true,
		Sample: func(ids []int) []int { return shuffleTruncate(ids, 2) },
	})
	if err != nil {
		t.Fatalf("首次进入失败: %v", err)
	}
	if idx != 0 || len(ids1) != 2 {
		t.Fatalf("新开始应抽样 2 题、游标 0: ids=%v idx=%d", ids1, idx)
	}

	// 同集续练（count=0 全量场景）：沿用已存顺序（顺序协商保持，游标才有效）
	ids2, idx2, err := ResumeSet(db, 1, ResumeSetSpec{
		Mode: mode, FreshIDs: ids1, ReuseSaved: true,
		Sample: func(ids []int) []int { return shuffleTruncate(ids, 2) },
	})
	if err != nil {
		t.Fatalf("续练失败: %v", err)
	}
	if idx2 != 0 {
		t.Fatalf("游标未推进时续练应从 0 开始, got %d", idx2)
	}
	for i := range ids1 {
		if ids2[i] != ids1[i] {
			t.Fatalf("续练顺序必须与已存一致: %v vs %v", ids1, ids2)
		}
	}

	// 推进游标到 1 后续练：游标恢复（FreshIDs=已抽样的同集）
	if err := SaveSet(db, 1, mode, nil, 1, 0, nil); err != nil {
		t.Fatalf("保存进度失败: %v", err)
	}
	_, idx3, err := ResumeSet(db, 1, ResumeSetSpec{Mode: mode, FreshIDs: ids1, ReuseSaved: true})
	if err != nil {
		t.Fatalf("续练失败: %v", err)
	}
	if idx3 != 1 {
		t.Fatalf("续练游标应为 1, got %d", idx3)
	}

	// 集合变化（题目下架）：刷新为新集合、游标复位 0、不重抽样（中途变化）
	ids4, idx4, err := ResumeSet(db, 1, ResumeSetSpec{
		Mode: mode, FreshIDs: []int{11, 44}, ReuseSaved: true,
		Sample: func(ids []int) []int { return shuffleTruncate(ids, 2) },
	})
	if err != nil {
		t.Fatalf("集合变化续练失败: %v", err)
	}
	if idx4 != 0 {
		t.Fatalf("集合变化应复位游标, got %d", idx4)
	}
	if len(ids4) != 2 || ids4[0] != 11 || ids4[1] != 44 {
		t.Fatalf("集合变化应采用新集合全量: %v", ids4)
	}

	// 练完（游标 == 总数）后重进：重新抽样（新开始）
	if err := SaveSet(db, 2, mode, nil, 2, 2, nil); err != nil {
		t.Fatalf("保存完成进度失败: %v", err)
	}
	ids5, idx5, err := ResumeSet(db, 2, ResumeSetSpec{
		Mode: mode, FreshIDs: []int{11, 44}, ReuseSaved: true,
		Sample: func(ids []int) []int { return shuffleTruncate(ids, 2) },
	})
	if err != nil {
		t.Fatalf("完成后重进失败: %v", err)
	}
	if idx5 != 0 {
		t.Fatalf("完成后重进游标应归零, got %d", idx5)
	}
	if len(ids5) != 2 {
		t.Fatalf("完成后重进应重新抽样: %v", ids5)
	}
}

// TestResumeSetSequentialCursorAcrossRefresh 顺序练习协商：恒用现集合（id 升序），
// 游标对「新总数」未越界则保留（题库扩充跨变化保留游标），越界复位 0。
func TestResumeSetSequentialCursorAcrossRefresh(t *testing.T) {
	db := testutil.NewMemoryDB(t)

	// 首次进入：题库 3 题，游标 0
	ids, idx, err := ResumeSet(db, 1, ResumeSetSpec{Mode: "sequential", FreshIDs: []int{1, 2, 3}, KeepCursorOnRefresh: true})
	if err != nil {
		t.Fatalf("首次进入失败: %v", err)
	}
	if idx != 0 || len(ids) != 3 {
		t.Fatalf("首次进入应为全量 3 题、游标 0: ids=%v idx=%d", ids, idx)
	}
	// 推进游标
	if err := SaveSet(db, 1, "sequential", nil, 2, 3, nil); err != nil {
		t.Fatalf("保存进度失败: %v", err)
	}
	// 题库扩充到 5 题：游标 2 保留
	_, idx2, err := ResumeSet(db, 1, ResumeSetSpec{Mode: "sequential", FreshIDs: []int{1, 2, 3, 4, 5}, KeepCursorOnRefresh: true})
	if err != nil {
		t.Fatalf("扩充续练失败: %v", err)
	}
	if idx2 != 2 {
		t.Fatalf("题库扩充应保留未越界游标, got %d", idx2)
	}
	// 题库收缩到 2 题：游标 2 越界复位 0
	ids3, idx3, err := ResumeSet(db, 1, ResumeSetSpec{Mode: "sequential", FreshIDs: []int{1, 2}, KeepCursorOnRefresh: true})
	if err != nil {
		t.Fatalf("收缩续练失败: %v", err)
	}
	if idx3 != 0 || len(ids3) != 2 {
		t.Fatalf("越界游标应复位 0、全量 2 题: ids=%v idx=%d", ids3, idx3)
	}
}

// TestQuestionPoolOptsUnified 抽题池单点：published + 排真题 + 证件分区三元组
// 在标签专项与顺序练习两入口同样生效（内联重复已收编）。
func TestQuestionPoolOptsUnified(t *testing.T) {
	db := testutil.NewMemoryDB(t)
	catalogSvc := NewTrainingCatalogService(db, zap.NewNop())
	qsvc := NewQuestionBankService(db, nil, zap.NewNop())
	psvc := NewPracticeModeService(db, nil, zap.NewNop())

	tag, _ := catalogSvc.CreateQuestionTag(QuestionTagInput{Code: "hydraulic", Name: "液压", SortOrder: ptrInt(1)})
	srcTag, _ := catalogSvc.CreateQuestionTag(QuestionTagInput{Code: "real_exam", Name: "真题", SortOrder: ptrInt(2)})
	if err := db.Model(&model.QuestionTag{}).Where("id = ?", srcTag.ID).Update("is_source_tag", true).Error; err != nil {
		t.Fatalf("置 source 标签失败: %v", err)
	}

	cred := &model.Credential{Code: "forklift_n1", Name: "N1证", Category: "special_operation"}
	if err := db.Create(cred).Error; err != nil {
		t.Fatalf("建证件失败: %v", err)
	}
	mk := func(credID *int, tagIDs []int, content string) int {
		input := map[string]any{
			"type": "single_choice", "content": content, "options": []string{"A", "B"}, "answer": "A",
			"status": "published", "tag_ids": tagIDs,
		}
		if credID != nil {
			input["credential_id"] = *credID
		}
		q, err := qsvc.CreateQuestion(input, nil, "tutor")
		if err != nil {
			t.Fatalf("建题失败: %v", err)
		}
		return q.ID
	}
	inCred := mk(&cred.ID, []int{tag.ID}, "证件内普通题")
	_ = mk(nil, []int{tag.ID}, "其他证件普通题")
	sourceTagged := mk(&cred.ID, []int{srcTag.ID}, "证件内真题题")

	// 标签专项：仅当前证件、排除真题题
	got, err := psvc.StartTagPractice(1, tag.ID, 0, &cred.ID)
	if err != nil {
		t.Fatalf("标签专项失败: %v", err)
	}
	if len(got.Questions) != 1 || got.Questions[0].ID != inCred {
		t.Fatalf("标签专项应仅含当前证件普通题: %+v", got.Questions)
	}
	// 顺序练习：同口径
	seq, err := psvc.StartSequential(1, &cred.ID)
	if err != nil {
		t.Fatalf("顺序练习失败: %v", err)
	}
	for _, q := range seq.Questions {
		if q.ID == sourceTagged {
			t.Fatal("顺序练习不应含真题题")
		}
		if q.Content == "其他证件普通题" {
			t.Fatal("顺序练习应按证件分区")
		}
	}
	// 随机练习（走同一单点的抽样分支）：排除真题题
	free, err := psvc.GetFreeQuestions("", 0, &cred.ID)
	if err != nil {
		t.Fatalf("随机抽题失败: %v", err)
	}
	for _, q := range free {
		if q.ID == sourceTagged {
			t.Fatal("随机抽题不应含真题题")
		}
	}
}
