package service

import (
	"fmt"
	"reflect"
	"strings"
	"testing"

	"forklift-training/internal/model"
	"forklift-training/internal/testutil"
)

// ===== #982 搜索能力升级：章节分区 / 回复命中 / 匹配面 / 命中片段 / 排序 / 检索事实 =====

// 章节分区：可见性跟随所属课程（已发布 + 挂载不变式），并按当前证件分区。
func TestSearchChapterPartitionFollowsCourseVisibility(t *testing.T) {
	db := testutil.NewMemoryDB(t)
	svc := NewSearchService(db, nil)
	credA, credB := 1, 2
	spID, lvID := 1, 1

	mkCourse := func(name string, cred *int, mounted bool, status ...int16) model.Course {
		st := int16(1)
		if len(status) > 0 {
			st = status[0]
		}
		c := model.Course{Name: name, Status: st, CredentialID: cred, CreatedAt: testutil.Now()}
		if mounted {
			c.SpecialtyID = &spID
			c.LevelID = &lvID
		}
		if err := db.Create(&c).Error; err != nil {
			t.Fatalf("建课失败: %v", err)
		}
		// Course.Status 的 gorm default:1 会把零值写成 1（GORM 对带默认值的零值字段走 DB 默认），
		// 所以「未发布」必须显式回写一次，否则用例自己在骗自己。
		if st != 1 {
			if err := db.Model(&model.Course{}).Where("course_id = ?", c.CourseID).Update("status", st).Error; err != nil {
				t.Fatalf("回写课程状态失败: %v", err)
			}
		}
		return c
	}
	mkChapter := func(courseID int, title string) model.Chapter {
		ch := model.Chapter{CourseID: courseID, Title: title, Content: "正文", CreatedAt: testutil.Now()}
		if err := db.Create(&ch).Error; err != nil {
			t.Fatalf("建章节失败: %v", err)
		}
		return ch
	}

	mkChapter(mkCourse("证件A课程", &credA, true).CourseID, "液压系统拆装")
	mkChapter(mkCourse("证件B课程", &credB, true).CourseID, "液压系统拆装B")
	mkChapter(mkCourse("未挂载课程", &credA, false).CourseID, "液压系统拆装未挂载")
	// 未发布课程下的章节同样不可检索（章节可见性跟随课程的 published 口径）
	mkChapter(mkCourse("未发布课程", &credA, true, 0).CourseID, "液压系统拆装未发布")

	got, err := svc.Search("液压", SearchTypeChapter, 1, 20, &credA)
	if err != nil {
		t.Fatalf("章节分区搜索失败: %v", err)
	}
	page := got.(*SearchPageDTO)
	if page.Total != 1 {
		t.Fatalf("章节分区应只含当前证件+已挂载课程的章节, got %d: %+v", page.Total, page.Items)
	}
	if page.Items[0].Title != "液压系统拆装" {
		t.Fatalf("命中的章节不对: %+v", page.Items[0])
	}
	if page.Items[0].ParentID == 0 {
		t.Fatalf("章节结果必须带所属课程 ID（落点需要）: %+v", page.Items[0])
	}
}

// 命中位置与命中片段：标题命中优先于正文命中；片段是命中窗口而非开头截断。
func TestSearchHitPositionOrderingAndSnippet(t *testing.T) {
	db := testutil.NewMemoryDB(t)
	svc := NewSearchService(db, nil)
	spID, lvID := 1, 1
	mk := func(name, desc string) model.Course {
		c := model.Course{Name: name, Description: desc, Status: 1, SpecialtyID: &spID, LevelID: &lvID, CreatedAt: testutil.Now()}
		if err := db.Create(&c).Error; err != nil {
			t.Fatalf("建课失败: %v", err)
		}
		return c
	}
	// 正文命中，但 ID 更小、sort_order 更靠前 —— 仍必须排在标题命中之后
	bodyHit := mk("拆装工艺", strings.Repeat("前", 60)+"液压"+strings.Repeat("后", 60))
	_ = bodyHit
	mk("液压系统原理", "与本关键词无关的正文")

	got, err := svc.Search("液压", SearchTypeCourse, 1, 20, nil)
	if err != nil {
		t.Fatalf("搜索失败: %v", err)
	}
	page := got.(*SearchPageDTO)
	if len(page.Items) != 2 {
		t.Fatalf("应命中两门课程, got %+v", page.Items)
	}
	if page.Items[0].Title != "液压系统原理" {
		t.Fatalf("标题命中必须优先: %+v", page.Items)
	}
	if page.Items[0].HitField != SearchHitTitle {
		t.Fatalf("标题命中的 hit_field 应为 title, got %q", page.Items[0].HitField)
	}
	bodyItem := page.Items[1]
	if bodyItem.HitField != SearchHitBody {
		t.Fatalf("正文化的命中 hit_field 应为 body, got %q", bodyItem.HitField)
	}
	if !strings.Contains(bodyItem.Snippet, "液压") {
		t.Fatalf("命中片段必须包含关键词: %q", bodyItem.Snippet)
	}
	if strings.HasPrefix(bodyItem.Snippet, "前前前") {
		t.Fatalf("命中片段不得是开头截断: %q", bodyItem.Snippet)
	}
	if !strings.Contains(bodyItem.Summary, "…") {
		t.Fatalf("summary 保留原口径（开头截断）以兼容老客户端: %q", bodyItem.Summary)
	}
}

// 匹配面补全：课程简介、内容精选正文。
func TestSearchMatchesCourseDescriptionAndFeaturedBody(t *testing.T) {
	db := testutil.NewMemoryDB(t)
	svc := NewSearchService(db, nil)
	spID, lvID := 1, 1
	c := model.Course{Name: "无关键词的课名", Description: "本课讲液压泵的检修", Status: 1, SpecialtyID: &spID, LevelID: &lvID, CreatedAt: testutil.Now()}
	if err := db.Create(&c).Error; err != nil {
		t.Fatalf("建课失败: %v", err)
	}
	fc := model.FeaturedContent{Title: "无关键词的标题", Summary: "无关键词的摘要", Content: "正文里提到液压系统的保养", Status: 1, CreatedAt: testutil.Now()}
	if err := db.Create(&fc).Error; err != nil {
		t.Fatalf("建精选失败: %v", err)
	}

	coursePage, err := svc.Search("液压", SearchTypeCourse, 1, 20, nil)
	if err != nil || coursePage.(*SearchPageDTO).Total != 1 {
		t.Fatalf("课程简介应参与匹配: %v %+v", err, coursePage)
	}
	contentPage, err := svc.Search("液压", SearchTypeContent, 1, 20, nil)
	if err != nil || contentPage.(*SearchPageDTO).Total != 1 {
		t.Fatalf("内容精选正文应参与匹配: %v %+v", err, contentPage)
	}
}

// 论坛回复命中：结果仍指向主题，但必须标注命中在回复并给出回复上下文。
func TestSearchTopicMatchesReplyContent(t *testing.T) {
	db := testutil.NewMemoryDB(t)
	svc := NewSearchService(db, nil)
	topic := model.ForumTopic{UserID: 1, Title: "变速箱异响", Content: "换了油还是响", Category: "discussion", ContentFormat: "text", CreatedAt: testutil.Now(), UpdatedAt: testutil.Now()}
	if err := db.Create(&topic).Error; err != nil {
		t.Fatalf("建帖失败: %v", err)
	}
	reply := model.ForumReply{TopicID: topic.ID, UserID: 2, Content: strings.Repeat("垫", 30) + "液压泵压力不足" + strings.Repeat("垫", 30), ContentFormat: "text", CreatedAt: testutil.Now()}
	if err := db.Create(&reply).Error; err != nil {
		t.Fatalf("建回复失败: %v", err)
	}

	got, err := svc.Search("液压泵", SearchTypeTopic, 1, 20, nil)
	if err != nil {
		t.Fatalf("搜索失败: %v", err)
	}
	page := got.(*SearchPageDTO)
	if page.Total != 1 {
		t.Fatalf("回复正文应参与主题匹配, got %+v", page.Items)
	}
	if page.Items[0].HitField != SearchHitReply {
		t.Fatalf("回复命中应标注 reply, got %q", page.Items[0].HitField)
	}
	if !strings.Contains(page.Items[0].Snippet, "液压泵") {
		t.Fatalf("回复命中的片段应来自回复正文: %q", page.Items[0].Snippet)
	}
}

// 聚合响应新增章节分区。
func TestSearchAllIncludesChapterSection(t *testing.T) {
	db := testutil.NewMemoryDB(t)
	svc := NewSearchService(db, nil)
	spID, lvID := 1, 1
	course := model.Course{Name: "课", Status: 1, SpecialtyID: &spID, LevelID: &lvID, CreatedAt: testutil.Now()}
	if err := db.Create(&course).Error; err != nil {
		t.Fatalf("建课失败: %v", err)
	}
	ch := model.Chapter{CourseID: course.CourseID, Title: "液压章节", Content: "x", CreatedAt: testutil.Now()}
	if err := db.Create(&ch).Error; err != nil {
		t.Fatalf("建章节失败: %v", err)
	}
	all, err := svc.Search("液压", "", 1, 20, nil)
	if err != nil {
		t.Fatalf("聚合搜索失败: %v", err)
	}
	if all.(*SearchAllDTO).Chapters.Total != 1 {
		t.Fatalf("聚合搜索应含章节分区: %+v", all)
	}
}

// 检索事实：匿名（结构上不得有 user / credential / ip 列），零结果词可查。
func TestSearchFactsAreAnonymousAndZeroResultQueryable(t *testing.T) {
	db := testutil.NewMemoryDB(t)
	svc := NewSearchService(db, nil)

	// 结构性锁：事实表不得指向人（字段名里出现 user / credential / ip 即红）
	ft := reflect.TypeOf(model.SearchFact{})
	for i := 0; i < ft.NumField(); i++ {
		name := strings.ToLower(ft.Field(i).Name)
		for _, banned := range []string{"user", "credential", "ip", "device", "session"} {
			if strings.Contains(name, banned) {
				t.Fatalf("检索事实表不得含指向人的列: %s", ft.Field(i).Name)
			}
		}
	}

	// 聚合搜索（type 缺省）+ 零命中 = 零结果搜索
	if _, err := svc.Search("查无此词的液压", "", 1, 20, nil); err != nil {
		t.Fatalf("搜索失败: %v", err)
	}
	var facts int64
	if err := db.Model(&model.SearchFact{}).Count(&facts).Error; err != nil || facts != 1 {
		t.Fatalf("每次搜索应落一条检索事实, got %d err=%v", facts, err)
	}

	zero, err := svc.ZeroResultKeywords(30, 20)
	if err != nil {
		t.Fatalf("零结果词查询失败: %v", err)
	}
	if len(zero) != 1 || zero[0].Keyword != "查无此词的液压" || zero[0].Times != 1 {
		t.Fatalf("零结果词应含该关键词: %+v", zero)
	}

	// **指定类型**搜索的 0 命中不算零结果词：那只说明该分区没有，不等于平台没有
	if _, err := svc.Search("查无此词的章节", SearchTypeChapter, 1, 20, nil); err != nil {
		t.Fatalf("搜索失败: %v", err)
	}
	zero, err = svc.ZeroResultKeywords(30, 20)
	if err != nil {
		t.Fatalf("零结果词查询失败: %v", err)
	}
	for _, z := range zero {
		if z.Keyword == "查无此词的章节" {
			t.Fatalf("指定类型搜索的 0 命中不得进零结果列表: %+v", zero)
		}
	}

	// 聚合搜索的分区命中数逐区落库（哪一类内容搜不到可回答）
	spID, lvID := 1, 1
	c := model.Course{Name: "液压系统", Status: 1, SpecialtyID: &spID, LevelID: &lvID, CreatedAt: testutil.Now()}
	if err := db.Create(&c).Error; err != nil {
		t.Fatalf("建课失败: %v", err)
	}
	if err := db.Create(&model.Chapter{CourseID: c.CourseID, Title: "液压章节", Content: "x", CreatedAt: testutil.Now()}).Error; err != nil {
		t.Fatalf("建章节失败: %v", err)
	}
	if _, err := svc.Search("液压", "", 1, 20, nil); err != nil {
		t.Fatalf("搜索失败: %v", err)
	}
	var last model.SearchFact
	if err := db.Order("id DESC").First(&last).Error; err != nil {
		t.Fatalf("读检索事实失败: %v", err)
	}
	if last.SearchType != "" || last.CourseHits != 1 || last.ChapterHits != 1 || last.TotalHits != 2 {
		t.Fatalf("聚合检索事实的分区命中数不对: %+v", last)
	}
	if last.QuestionHits != 0 || last.ContentHits != 0 || last.TopicHits != 0 {
		t.Fatalf("未命中分区的计数应为 0: %+v", last)
	}
	// 有命中的词不进零结果列表
	zero, err = svc.ZeroResultKeywords(30, 20)
	if err != nil {
		t.Fatalf("零结果词查询失败: %v", err)
	}
	for _, z := range zero {
		if z.Keyword == "液压" {
			t.Fatalf("有命中的词不得进零结果列表: %+v", zero)
		}
	}
}

// 二级排序键真正生效（一级键是命中位置，这里固定为同一级再比二级键）。
func TestSearchSecondaryOrderKeys(t *testing.T) {
	db := testutil.NewMemoryDB(t)
	svc := NewSearchService(db, nil)
	spID, lvID := 1, 1
	mk := func(name string, sortOrder int) model.Course {
		c := model.Course{Name: name, Status: 1, SpecialtyID: &spID, LevelID: &lvID, SortOrder: sortOrder, CreatedAt: testutil.Now()}
		if err := db.Create(&c).Error; err != nil {
			t.Fatalf("建课失败: %v", err)
		}
		return c
	}
	// 两门都是标题命中：id 大的 sort_order 更靠前 → 必须按 sort_order 排，而不是 course_id
	mk("液压入门", 9)
	mk("液压进阶", 1)

	got, err := svc.Search("液压", SearchTypeCourse, 1, 20, nil)
	if err != nil {
		t.Fatalf("搜索失败: %v", err)
	}
	items := got.(*SearchPageDTO).Items
	if len(items) != 2 || items[0].Title != "液压进阶" {
		t.Fatalf("课程二级键 sort_order 未生效: %+v", items)
	}

	// 时效内容：两条都是正文命中，最近有回复的排前
	older := testutil.Now().AddDate(0, 0, -3)
	newer := testutil.Now()
	t1 := model.ForumTopic{UserID: 1, Title: "老帖", Content: "液压泵异响", Category: "discussion", ContentFormat: "text", LastReplyAt: &older, CreatedAt: older, UpdatedAt: older}
	t2 := model.ForumTopic{UserID: 1, Title: "新帖", Content: "液压泵异响", Category: "discussion", ContentFormat: "text", LastReplyAt: &newer, CreatedAt: newer, UpdatedAt: newer}
	if err := db.Create(&t1).Error; err != nil {
		t.Fatalf("建帖失败: %v", err)
	}
	if err := db.Create(&t2).Error; err != nil {
		t.Fatalf("建帖失败: %v", err)
	}
	topics, err := svc.Search("液压泵", SearchTypeTopic, 1, 20, nil)
	if err != nil {
		t.Fatalf("搜索失败: %v", err)
	}
	tItems := topics.(*SearchPageDTO).Items
	if len(tItems) != 2 || tItems[0].Title != "新帖" {
		t.Fatalf("论坛帖二级键 last_reply_at 未生效: %+v", tItems)
	}
}

// 越界页行为（#1095 收编 paging.QueryWithScan 的等价判据）：页大小 >100 回退默认 20（不是截断到 100），
// 页码 <=0 回退第 1 页；total 口径与钳制无关（永远是匹配总数）。响应里的 page/pages 仍按原始入参算。
func TestSearchPageClampFallsBackToDefaults(t *testing.T) {
	db := testutil.NewMemoryDB(t)
	svc := NewSearchService(db, nil)
	for i := 0; i < 25; i++ {
		testutil.SeedQuestion(t, db, "single_choice", fmt.Sprintf("液压滤芯 %02d", i), "A")
	}

	got, err := svc.Search("液压", SearchTypeQuestion, 1, 1000, nil)
	if err != nil {
		t.Fatalf("搜索失败: %v", err)
	}
	page := got.(*SearchPageDTO)
	if page.Total != 25 {
		t.Fatalf("total=%d, want 25（钳制不影响总数口径）", page.Total)
	}
	if len(page.Items) != 20 {
		t.Fatalf("page_size=1000 必须回退默认 20（不是 1000、也不是截断到 100），实得 %d 条", len(page.Items))
	}

	first, err := svc.Search("液压", SearchTypeQuestion, 0, 20, nil)
	if err != nil {
		t.Fatalf("搜索失败: %v", err)
	}
	if n := len(first.(*SearchPageDTO).Items); n != 20 {
		t.Fatalf("page=0 必须回退第 1 页（取满 20 条），实得 %d 条", n)
	}
}

var _ = fmt.Sprintf
