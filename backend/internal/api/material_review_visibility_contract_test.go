// 契约测试 #2498 C11：学习资料聚合面必须与学员读面同谓词（课程可见 ∧ 章节有已发布槽）。
//
// 为什么这个口要单独锁：资料聚合是第三个读面（课程详情 / 章节详情 / 章节附件投递），
// 此前 materialScope 只 join 了 chapter 与 course.status=1，**没限已发布槽**——
// 未过审新章的附件会从这里泄漏出去，正是 spec C6「列表一律按不存在」要堵的口。
package api

import (
	"net/http"
	"testing"

	"forklift-training/internal/model"
	"forklift-training/internal/testutil"
)

func TestMaterialAggregateHidesUnpublishedChapters(t *testing.T) {
	testutil.SetTestGinMode()
	r, cfg, db := newSlice6Env(t)
	student := seedStudent(t, db, "material_review_stu", "hash")
	token := slice6Token(t, cfg, student.ID, student.Username, "hrwai_user")

	ptr := func(v int) *int { return &v }
	spec := model.Specialty{Code: "mat-rev", Name: "资料方向", SortOrder: 1, Status: 1}
	lv := model.CourseLevel{Code: "mat-rev-lv", Name: "入门", SortOrder: 1, Status: 1}
	c := model.Course{Name: "资料可见性课程", Status: 1,
		SpecialtyID: ptr(spec.SpecialtyID), LevelID: ptr(lv.LevelID), CreatedAt: testutil.Now()}
	for _, row := range []any{&spec, &lv, &c} {
		if err := db.Create(row).Error; err != nil {
			t.Fatalf("播种失败: %v", err)
		}
	}

	// 两个章节各挂一份课件：一个已过审、一个从未提交审核。
	published := model.Chapter{CourseID: c.CourseID, Title: "已过审章节", Content: "v1",
		ContentType: "text", OrderNum: 1, CreatedAt: testutil.Now()}
	draft := model.Chapter{CourseID: c.CourseID, Title: "未过审章节", Content: "草稿",
		ContentType: "text", OrderNum: 2, CreatedAt: testutil.Now()}
	for _, ch := range []*model.Chapter{&published, &draft} {
		if err := db.Create(ch).Error; err != nil {
			t.Fatalf("建章节失败: %v", err)
		}
	}
	for _, f := range []model.ChapterFile{
		{ChapterID: &published.ChapterID, FileURL: "/static/uploads/pub.pdf", FileName: "已过审课件.pdf",
			ContentType: "document", FileSize: 1, CreatedAt: testutil.Now()},
		{ChapterID: &draft.ChapterID, FileURL: "/static/uploads/draft.pdf", FileName: "未过审课件.pdf",
			ContentType: "document", FileSize: 1, CreatedAt: testutil.Now()},
	} {
		file := f
		if err := db.Create(&file).Error; err != nil {
			t.Fatalf("建章节文件失败: %v", err)
		}
	}
	publishChapterForRead(t, db, published.ChapterID) // 只发布一个

	rec := catalogRequest(t, r, token, http.MethodGet, "/api/materials?page=1&page_size=20", "")
	m := slice6AssertKeys(t, rec, http.StatusOK, "materials", "page", "pages", "total")
	items, _ := m["materials"].([]any)
	if len(items) != 1 {
		t.Fatalf("资料面应只含已过审章节的附件 1 条, got %d: %v", len(items), items)
	}
	item, _ := items[0].(map[string]any)
	if name, _ := item["file_name"].(string); name != "已过审课件.pdf" {
		t.Fatalf("资料面泄漏了未过审章节的附件: %v", item)
	}

	// 详情面同谓词：未过审章节的附件按不存在（不泄漏存在性）
	draftFileID := 0
	if err := db.Model(&model.ChapterFile{}).Where("chapter_id = ?", draft.ChapterID).
		Pluck("file_id", &draftFileID).Error; err != nil {
		t.Fatalf("取未过审附件 id 失败: %v", err)
	}
	rec = catalogRequest(t, r, token, http.MethodGet, "/api/materials/"+itoa(draftFileID), "")
	if rec.Code != http.StatusNotFound {
		t.Fatalf("未过审章节的附件详情应按不存在 404, got %d: %s", rec.Code, rec.Body.String())
	}
}
