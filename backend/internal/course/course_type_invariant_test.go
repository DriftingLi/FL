// 内容级互斥不变式的契约测试（#2760，用户 2026-11-22 拍板「彻底区分」）。
//
// 这条不变式说的是：**一门课不能同时有视频内容和图文章节**。主类型只说这门课挂在哪个
// 目录，不说里面真是什么；允许混合就等于让「视频课程」里出现文章。
//
// 两面都要测，因为它们拦的是**不同路**：
//   - 章节写面（建章 / 改章 / 上传文件）：往图文档里塞视频
//   - 课程写面（改类型）：把含视频章节的课改成图文课
//
// 只测一面，「另一面仍能造出混合课」这个缺陷就测不出来——改类型比改内容省事，
// 正是最容易被走的那条绕行路。
package course

import (
	"errors"
	"testing"

	"go.uber.org/zap"

	"forklift-training/internal/model"
	"forklift-training/internal/testutil"
)

// courseTypeWriteFixture 写面夹具：一门视频课 + 一门图文课，各带一个同质章节。
type courseTypeWriteFixture struct {
	svc         *AdminService
	video       model.Course
	illustrated model.Course
	categoryID  int
}

func setupCourseTypeWriteFixture(t *testing.T) *courseTypeWriteFixture {
	t.Helper()
	db := testutil.NewMemoryDB(t)
	f := &courseTypeWriteFixture{svc: NewAdminService(db, nil, nil, zap.NewNop())}

	cred := model.Credential{Code: "CTW", Name: "类型写面证件", Category: "special_operation", Status: 1, SortOrder: 1}
	if err := db.Create(&cred).Error; err != nil {
		t.Fatal(err)
	}
	cat := model.TrainingCategory{Name: "类型写面分类", SortOrder: 1, Status: 1, LegacyCredentialID: &cred.ID}
	if err := db.Create(&cat).Error; err != nil {
		t.Fatal(err)
	}
	f.categoryID = cat.ID

	mk := func(name, courseType string) model.Course {
		t.Helper()
		c := model.Course{Name: name, CourseType: courseType, CategoryID: &cat.ID, Status: 1, CreatedAt: testutil.Now()}
		if err := db.Create(&c).Error; err != nil {
			t.Fatal(err)
		}
		return c
	}
	f.video = mk("视频课", model.CourseTypeVideo)
	f.illustrated = mk("图文课", model.CourseTypeIllustrated)

	// 各挂一个同质章节（直接落库：本章测写面护栏，建章本身另有护栏，绕开它才能
	// 单独验证「改章」这一条）
	if err := db.Create(&model.Chapter{CourseID: f.video.CourseID, Title: "视频章", ContentType: "video",
		OrderNum: 1, CreatedAt: testutil.Now()}).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&model.Chapter{CourseID: f.illustrated.CourseID, Title: "图文章", ContentType: "text",
		OrderNum: 1, CreatedAt: testutil.Now()}).Error; err != nil {
		t.Fatal(err)
	}
	return f
}

// TestCreateChapterRejectsCrossNatureContent 建章时内容形态必须与本课类型同质。
func TestCreateChapterRejectsCrossNatureContent(t *testing.T) {
	f := setupCourseTypeWriteFixture(t)
	title := "新章"
	textType := "text"
	videoType := "video"

	// 视频课 + 图文章节 ⇒ 拒
	_, err := f.svc.CreateChapter(f.video.CourseID, &ChapterInput{Title: &title, ContentType: &textType})
	if !errors.Is(err, ErrCourseContentTypeConflict) {
		t.Fatalf("视频课建图文章节应被拒，实得 %v", err)
	}

	// 图文课 + 视频章节 ⇒ 拒
	_, err = f.svc.CreateChapter(f.illustrated.CourseID, &ChapterInput{Title: &title, ContentType: &videoType})
	if !errors.Is(err, ErrCourseContentTypeConflict) {
		t.Fatalf("图文课建视频章节应被拒，实得 %v", err)
	}

	// 视频课 + 视频章节 ⇒ 通过，且**传入的形态真的落库**
	// （判过的形态与落库的形态必须是同一个值，否则不变式在写入那一刻就破了）
	created, err := f.svc.CreateChapter(f.video.CourseID, &ChapterInput{Title: &title, ContentType: &videoType})
	if err != nil {
		t.Fatalf("视频课建视频章节应通过，实得 %v", err)
	}
	if created.ContentType != "video" {
		t.Fatalf("建章返回的 content_type = %q，传入 video 却没落库", created.ContentType)
	}
	var stored model.Chapter
	if err := f.svc.db.First(&stored, created.ChapterID).Error; err != nil {
		t.Fatal(err)
	}
	if stored.ContentType != "video" {
		t.Fatalf("库里 content_type = %q，want video（校验按 video 过、落库成默认值是漏洞）", stored.ContentType)
	}

	// 不传 content_type ⇒ 默认 text ⇒ 对图文课通过、对视频课拒
	if _, err := f.svc.CreateChapter(f.illustrated.CourseID, &ChapterInput{Title: &title}); err != nil {
		t.Fatalf("图文课建默认形态章节应通过，实得 %v", err)
	}
	if _, err := f.svc.CreateChapter(f.video.CourseID, &ChapterInput{Title: &title}); !errors.Is(err, ErrCourseContentTypeConflict) {
		t.Fatalf("视频课建默认（text）章节应被拒，实得 %v", err)
	}
}

// TestUpdateChapterRejectsCrossNatureContent 改章的内容形态同样受护栏约束。
func TestUpdateChapterRejectsCrossNatureContent(t *testing.T) {
	f := setupCourseTypeWriteFixture(t)
	db := f.svc.db
	var videoChapter, illustratedChapter model.Chapter
	if err := db.Where("course_id = ?", f.video.CourseID).First(&videoChapter).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Where("course_id = ?", f.illustrated.CourseID).First(&illustratedChapter).Error; err != nil {
		t.Fatal(err)
	}

	textType := "text"
	// 把视频课的章节改成 text ⇒ 拒（且不得落库）
	if _, err := f.svc.UpdateChapter(t.Context(), videoChapter.ChapterID, &ChapterInput{ContentType: &textType}); !errors.Is(err, ErrCourseContentTypeConflict) {
		t.Fatalf("把视频章的形态改成 text 应被拒，实得 %v", err)
	}
	var after model.Chapter
	if err := db.First(&after, videoChapter.ChapterID).Error; err != nil {
		t.Fatal(err)
	}
	if after.ContentType != "video" {
		t.Fatalf("被拒的改章仍改了库：content_type = %q", after.ContentType)
	}

	// 只改标题（不带 content_type）⇒ 通过（形态不变，无需再判）
	newTitle := "改过的标题"
	if _, err := f.svc.UpdateChapter(t.Context(), videoChapter.ChapterID, &ChapterInput{Title: &newTitle}); err != nil {
		t.Fatalf("只改标题应通过，实得 %v", err)
	}

	// 图文章改成 video ⇒ 拒
	videoType := "video"
	if _, err := f.svc.UpdateChapter(t.Context(), illustratedChapter.ChapterID, &ChapterInput{ContentType: &videoType}); !errors.Is(err, ErrCourseContentTypeConflict) {
		t.Fatalf("把图文章的形态改成 video 应被拒，实得 %v", err)
	}
}

// TestUpdateCourseRejectsTypeChangeWithMismatchedChapters 改类型方向的护栏：
// 课里有视频章节时不能改成图文课，反之亦然。这条是**绕行路**的封堵。
func TestUpdateCourseRejectsTypeChangeWithMismatchedChapters(t *testing.T) {
	f := setupCourseTypeWriteFixture(t)
	videoType := model.CourseTypeVideo
	illustratedType := model.CourseTypeIllustrated

	// 视频课（有视频章节）→ 图文 ⇒ 拒
	_, err := f.svc.UpdateCourse(f.video.CourseID, &CourseInput{CourseType: &illustratedType})
	if !errors.Is(err, ErrCourseContentTypeConflict) {
		t.Fatalf("含视频章节的课改成图文应被拒，实得 %v", err)
	}

	// 图文课（有图文章节）→ 视频 ⇒ 拒
	_, err = f.svc.UpdateCourse(f.illustrated.CourseID, &CourseInput{CourseType: &videoType})
	if !errors.Is(err, ErrCourseContentTypeConflict) {
		t.Fatalf("含图文章节的课改成视频应被拒，实得 %v", err)
	}

	// 同类型重报 ⇒ 通过（幂等：管理端表单每次都会带上当前类型）
	if _, err := f.svc.UpdateCourse(f.video.CourseID, &CourseInput{CourseType: &videoType}); err != nil {
		t.Fatalf("同类型重报应通过，实得 %v", err)
	}

	// 不带 course_type 的局部更新 ⇒ 通过（老调用方兼容）
	hot := true
	if _, err := f.svc.UpdateCourse(f.video.CourseID, &CourseInput{IsHot: &hot}); err != nil {
		t.Fatalf("不带 course_type 的更新应通过，实得 %v", err)
	}
	var after model.Course
	if err := f.svc.db.First(&after, f.video.CourseID).Error; err != nil {
		t.Fatal(err)
	}
	if after.CourseType != model.CourseTypeVideo {
		t.Fatalf("省略 course_type 时应保留原值，实得 %q", after.CourseType)
	}
}

// TestUpdateCourseAllowsPartialEditOfMigratedMixedCourse 存量混合课的局部更新不被拦。
//
// 规格「课程内容更新后仍由我决定主类型」：迁移预填可能留下混合课（任一已发布视频章节
// ⇒ video，但仍有图文章节），等管理员复核。复核之前他改个名字、切一下上下架必须能成——
// 若反向护栏在每次 UpdateCourse 都跑，这门课除了「改类型」以外什么都改不了，
// 与「复核前维持归类稳定」直接冲突。
func TestUpdateCourseAllowsPartialEditOfMigratedMixedCourse(t *testing.T) {
	f := setupCourseTypeWriteFixture(t)
	db := f.svc.db

	// 造一门遗留混合课：类型是 video，但章节既有 video 又有 text（迁移预填的形态）
	mixed := model.Course{Name: "待复核混合课", CourseType: model.CourseTypeVideo,
		CategoryID: &f.categoryID, Status: 1, CreatedAt: testutil.Now()}
	if err := db.Create(&mixed).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&model.Chapter{CourseID: mixed.CourseID, Title: "视频章", ContentType: "video",
		OrderNum: 1, CreatedAt: testutil.Now()}).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&model.Chapter{CourseID: mixed.CourseID, Title: "图文章", ContentType: "text",
		OrderNum: 2, CreatedAt: testutil.Now()}).Error; err != nil {
		t.Fatal(err)
	}

	// 局部更新（改名、切上下架）应通过：类型没变，护栏不该介入
	newName := "待复核混合课（已改名）"
	got, err := f.svc.UpdateCourse(mixed.CourseID, &CourseInput{Name: &newName})
	if err != nil {
		t.Fatalf("存量混合课的局部改名应通过（复核前维持归类稳定），实得 %v", err)
	}
	if got.Name != newName || got.CourseType != model.CourseTypeVideo {
		t.Fatalf("局部更新结果错：%+v", got)
	}

	// 但**真的改类型**仍要被拦（护栏没有因此被整体拆掉）
	illustrated := model.CourseTypeIllustrated
	if _, err := f.svc.UpdateCourse(mixed.CourseID, &CourseInput{CourseType: &illustrated}); !errors.Is(err, ErrCourseContentTypeConflict) {
		t.Fatalf("存量混合课改成图文课仍应被内容级互斥拦住，实得 %v", err)
	}
}

// TestCreateCourseRequiresCourseType 创建必填 + 值域校验。
func TestCreateCourseRequiresCourseType(t *testing.T) {
	f := setupCourseTypeWriteFixture(t)
	name := "新课"
	catID := f.categoryID

	// 未携带 ⇒ 必填错
	if _, err := f.svc.CreateCourse(&CourseInput{Name: &name, CategoryID: &catID}); !errors.Is(err, ErrCourseTypeRequired) {
		t.Fatalf("不带 course_type 建课应被拒（必填），实得 %v", err)
	}
	// 非法值 ⇒ 值域错
	bad := "mixed"
	if _, err := f.svc.CreateCourse(&CourseInput{Name: &name, CategoryID: &catID, CourseType: &bad}); !errors.Is(err, ErrCourseTypeInvalid) {
		t.Fatalf("course_type=mixed 建课应被拒，实得 %v", err)
	}
	// 空串 ⇒ 值域错（不是「走默认」）
	empty := ""
	if _, err := f.svc.CreateCourse(&CourseInput{Name: &name, CategoryID: &catID, CourseType: &empty}); !errors.Is(err, ErrCourseTypeInvalid) {
		t.Fatalf("course_type='' 建课应被拒，实得 %v", err)
	}
	// 合法值 ⇒ 通过，且 DTO 回带类型
	for _, want := range []string{model.CourseTypeVideo, model.CourseTypeIllustrated} {
		got, err := f.svc.CreateCourse(&CourseInput{Name: &name, CategoryID: &catID, CourseType: &want})
		if err != nil {
			t.Fatalf("course_type=%s 建课应通过，实得 %v", want, err)
		}
		if got.CourseType != want {
			t.Fatalf("建课回带 course_type = %q, want %q", got.CourseType, want)
		}
	}
}

// TestChapterContentNatureMapping 形态归类表本身：PPT 归图文（票 2760 写死），
// 认不出的形态不猜（ok=false ⇒ 拒）。
func TestChapterContentNatureMapping(t *testing.T) {
	for _, tc := range []struct {
		contentType string
		want        string
		ok          bool
	}{
		{"video", model.CourseTypeVideo, true},
		// 票 2760：「图文课程含 PPT」
		{"ppt", model.CourseTypeIllustrated, true},
		{"text", model.CourseTypeIllustrated, true},
		{"document", model.CourseTypeIllustrated, true},
		// 空串按库侧 default（text）归类
		{"", model.CourseTypeIllustrated, true},
		// 认不出的形态不猜
		{"audio", "", false},
		{"Video", "", false},
	} {
		got, ok := chapterContentNature(tc.contentType)
		if ok != tc.ok || got != tc.want {
			t.Errorf("chapterContentNature(%q) = (%q, %v), want (%q, %v)", tc.contentType, got, ok, tc.want, tc.ok)
		}
	}
}
