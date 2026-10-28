// 练习统计 by_tag 契约（#2068）：按题库标签聚合正确率，与 by_type 并列、不替换。
//
// 口径（与实现注释同一句）：分母 = 该学员在该标签题目上的全部作答次数（含重做），分子 = 其中正确次数；
// 一题多标签 ⇒ 该次作答在每个标签下各计一次；只返回有作答的标签（0 分母不进结果，所以不会有 NaN）。
package practicemode

import (
	"testing"

	"forklift-training/internal/model"
	"forklift-training/internal/questionbank"
	"forklift-training/internal/testutil"
)

func TestPracticeStatsByTagContract(t *testing.T) {
	t.Parallel()
	db := testutil.NewMemoryDB(t)
	student := testutil.SeedStudent(t, db, "tag_stats_stu", "x")
	now := testutil.Now()

	tagA := model.QuestionTag{Code: "stats-tag-a", Name: "法规", SortOrder: 1, Status: 1, CreatedAt: now, UpdatedAt: now}
	tagB := model.QuestionTag{Code: "stats-tag-b", Name: "液压", SortOrder: 2, Status: 1, CreatedAt: now, UpdatedAt: now}
	tagC := model.QuestionTag{Code: "stats-tag-c", Name: "无作答标签", SortOrder: 3, Status: 1, CreatedAt: now, UpdatedAt: now}
	for _, tg := range []*model.QuestionTag{&tagA, &tagB, &tagC} {
		if err := db.Create(tg).Error; err != nil {
			t.Fatalf("创建标签失败: %v", err)
		}
	}
	q1 := testutil.SeedQuestion(t, db, "single_choice", "标签题一", "A")
	q2 := testutil.SeedQuestion(t, db, "single_choice", "标签题二", "B")
	for _, rel := range []model.QuestionTagRelation{
		{QuestionID: q1.ID, TagID: tagA.ID, CreatedAt: now},
		{QuestionID: q2.ID, TagID: tagA.ID, CreatedAt: now},
		{QuestionID: q2.ID, TagID: tagB.ID, CreatedAt: now},
	} {
		if err := db.Create(&rel).Error; err != nil {
			t.Fatalf("建标签关系失败: %v", err)
		}
	}
	// q1：1 对 1 错；q2：1 错 ⇒ tagA 合计 3 次 / 1 对；tagB 合计 1 次 / 0 对。
	for _, rec := range []model.QuestionPracticeRecord{
		{StudentID: student.ID, QuestionID: q1.ID, IsCorrect: true, UserAnswer: "A", CreatedAt: now},
		{StudentID: student.ID, QuestionID: q1.ID, IsCorrect: false, UserAnswer: "B", CreatedAt: now},
		{StudentID: student.ID, QuestionID: q2.ID, IsCorrect: false, UserAnswer: "C", CreatedAt: now},
	} {
		if err := db.Create(&rec).Error; err != nil {
			t.Fatalf("建作答记录失败: %v", err)
		}
	}

	svc := NewService(db, nil, nil)
	res, err := svc.GetStats(student.ID, nil)
	if err != nil {
		t.Fatalf("GetStats 失败: %v", err)
	}
	byTag := map[int]PracticeTagStat{}
	for _, st := range res.ByTag {
		byTag[st.TagID] = st
	}
	a, ok := byTag[tagA.ID]
	if !ok || a.Total != 3 || a.Correct != 1 || a.Accuracy != 33.3 || a.TagName != "法规" {
		t.Fatalf("tagA 应为 3 次 / 1 对 / 33.3%%, got %+v", a)
	}
	b, ok := byTag[tagB.ID]
	if !ok || b.Total != 1 || b.Correct != 0 || b.Accuracy != 0 {
		t.Fatalf("tagB 应为 1 次 / 0 对, got %+v", b)
	}
	if _, ok := byTag[tagC.ID]; ok {
		t.Fatalf("无作答的标签不该出现（0 分母不入结果）: %+v", byTag[tagC.ID])
	}
	// 排序：sort_order 升序（tagA 先于 tagB）
	if len(res.ByTag) != 2 || res.ByTag[0].TagID != tagA.ID || res.ByTag[1].TagID != tagB.ID {
		t.Fatalf("by_tag 应按 sort_order 排序, got %+v", res.ByTag)
	}
	// 向后兼容：by_type 形状不变（对全部合法题型零填充）
	if len(res.ByType) != len(questionbank.ValidQuestionTypes) {
		t.Fatalf("by_type 应仍按合法题型零填充, got %d", len(res.ByType))
	}
}
