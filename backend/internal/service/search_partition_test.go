package service

import (
	"reflect"
	"testing"

	"go.uber.org/zap"

	"forklift-training/internal/model"
	"forklift-training/internal/testutil"
)

// TestSearchPartitionRegistryConsistency 分区声明 ↔ 分发 ↔ 聚合一致性（ADR-0050 决策 2）：
// ① 分区表键与 SearchType 常量一一对应（既不会「加了分区漏登记」，也不会留下孤儿分发分支）；
// ② 每个声明都能被 searchItems 查表命中；未声明的类型键必须报错（不静默返回空）；
// ③ 聚合搜索遍历声明表逐区装配——五个响应字段都真的被填上，检索事实逐区落数。
// 本用例是「防加分区漏登记」的机械断言（描述符化的意义所在）。
func TestSearchPartitionRegistryConsistency(t *testing.T) {
	want := []string{SearchTypeCourse, SearchTypeChapter, SearchTypeQuestion, SearchTypeContent, SearchTypeTopic}
	if got := searchPartitionKeys(); !reflect.DeepEqual(got, want) {
		t.Fatalf("分区表键必须与 SearchType 常量一一对应, got %v want %v", got, want)
	}

	db := testutil.NewMemoryDB(t)
	svc := NewSearchService(db, zap.NewNop())
	for _, key := range want {
		if _, _, err := svc.searchItems(key, "液压", 1, 20); err != nil {
			t.Fatalf("分区 %s 已声明但分发不认识: %v", key, err)
		}
	}
	if _, _, err := svc.searchItems("hydraulic", "液压", 1, 20); err == nil {
		t.Fatal("未声明的类型键必须报错（不能静默返回空结果）")
	}

	// 五个分区各埋一条命中同一关键词的记录。
	spID, lvID, credID := 1, 1, 1
	course := model.Course{Name: "液压课程", Status: 1, SpecialtyID: &spID, LevelID: &lvID, CredentialID: &credID, CreatedAt: testutil.Now()}
	if err := db.Create(&course).Error; err != nil {
		t.Fatalf("建课失败: %v", err)
	}
	for _, row := range []any{
		&model.Chapter{CourseID: course.CourseID, Title: "液压章节", Content: "液压正文", CreatedAt: testutil.Now()},
		&model.Question{Type: "single_choice", Content: "液压题干", Answer: "A", Status: "published", CredentialID: &credID},
		&model.FeaturedContent{Title: "液压精选", Summary: "液压", Content: "液压正文", Status: 1, CreatedAt: testutil.Now()},
		&model.ForumTopic{Title: "液压主题", Content: "液压正文", Category: ForumCategoryDiscussion, ContentFormat: ForumContentFormatText, CreatedAt: testutil.Now()},
	} {
		if err := db.Create(row).Error; err != nil {
			t.Fatalf("建行失败: %v", err)
		}
	}

	all, err := svc.Search("液压", "", 1, 20)
	if err != nil {
		t.Fatalf("聚合搜索失败: %v", err)
	}
	got := all.(*SearchAllDTO)
	for _, sec := range []struct {
		name    string
		section SearchSectionDTO
	}{
		{SearchTypeCourse, got.Courses},
		{SearchTypeChapter, got.Chapters},
		{SearchTypeQuestion, got.Questions},
		{SearchTypeContent, got.Contents},
		{SearchTypeTopic, got.Topics},
	} {
		if sec.section.Total != 1 || len(sec.section.Items) != 1 {
			t.Fatalf("聚合搜索的 %s 分区未按声明表装配（应命中 1 条）: total=%d items=%+v",
				sec.name, sec.section.Total, sec.section.Items)
		}
		if sec.section.Items[0].Type != sec.name {
			t.Fatalf("%s 分区的命中条目类型键漂移: %+v", sec.name, sec.section.Items[0])
		}
	}

	// 检索事实逐区落数：漏登记的分区在这里露头（键 → 列的映射只有一处）。
	var fact model.SearchFact
	if err := db.Order("id DESC").First(&fact).Error; err != nil {
		t.Fatalf("读检索事实失败: %v", err)
	}
	if fact.CourseHits != 1 || fact.ChapterHits != 1 || fact.QuestionHits != 1 ||
		fact.ContentHits != 1 || fact.TopicHits != 1 || fact.TotalHits != 5 {
		t.Fatalf("检索事实必须逐区落数（漏登记的分区在这里露头）: %+v", fact)
	}
}
