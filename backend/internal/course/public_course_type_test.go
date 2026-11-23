package course

import (
	"fmt"
	"net/http"
	"net/url"
	"testing"

	"forklift-training/internal/model"
)

// courseTypeFixture 在 publicSchoolFixture 之上补两组类型的课，供分流判据使用。
//
// 复用 setupPublicSchoolFixture 而不是另起一套：类型筛选是**叠在**已有公开资格判据之上的
// 一层，若另建夹具就会把「资格判据仍成立」这件事留成未测的空档。
type courseTypeFixture struct {
	*publicSchoolFixture
	videoCourse       model.Course
	illustratedCourse model.Course
	// videoOnlyCategory 只在视频课下有公开课的分类（用于验证类型页的分类选项收口）
	videoOnlyCategory model.TrainingCategory
}

func setupCourseTypeFixture(t *testing.T) *courseTypeFixture {
	t.Helper()
	base := setupPublicSchoolFixture(t)

	// 第二个合格分类：只有视频课，没有图文课 ⇒ 图文页的证件下拉里不该出现它
	secondCred := model.Credential{Code: "N2", Name: "叉车司机N2证", Category: "special_operation", Status: 1, SortOrder: 2}
	if err := base.db.Create(&secondCred).Error; err != nil {
		t.Fatal(err)
	}
	videoOnly := model.TrainingCategory{Name: "叉车司机N2证", SortOrder: 2, Status: 1, LegacyCredentialID: &secondCred.ID}
	if err := base.db.Create(&videoOnly).Error; err != nil {
		t.Fatal(err)
	}

	// 已有的合格课（f.visibleCourse）改成明确的图文课，另建一门视频课放在第二分类
	if err := base.db.Model(&model.Course{}).Where("course_id = ?", base.visibleCourse.CourseID).
		Update("course_type", model.CourseTypeIllustrated).Error; err != nil {
		t.Fatal(err)
	}
	base.visibleCourse.CourseType = model.CourseTypeIllustrated

	video := model.Course{
		Name: "视频课·货叉操作", Description: "全视频讲解", CourseType: model.CourseTypeVideo,
		CategoryID: &videoOnly.ID, Status: 1,
	}
	if err := base.db.Create(&video).Error; err != nil {
		t.Fatal(err)
	}
	// 第二分类里再放一门图文课？不需要：这个分类的用途正是「只有视频课」。
	return &courseTypeFixture{
		publicSchoolFixture: base,
		videoCourse:         video,
		illustratedCourse:   base.visibleCourse,
		videoOnlyCategory:   videoOnly,
	}
}

// TestPublicSchoolCourseTypeFilterSplitsCatalog 类型筛选把目录一分为二，且互不相交、并集为全量。
func TestPublicSchoolCourseTypeFilterSplitsCatalog(t *testing.T) {
	f := setupCourseTypeFixture(t)

	// 无筛选 = 全部
	all := decodeData[PublicSchoolCoursePageDTO](t, f.get(t, "/api/public-school/courses?page_size=60"))
	if all.Total != 2 {
		t.Fatalf("不筛类型应返回全部 2 门合格课，实得 %d", all.Total)
	}

	video := decodeData[PublicSchoolCoursePageDTO](t, f.get(t, "/api/public-school/courses?course_type=video&page_size=60"))
	if video.Total != 1 || len(video.Courses) != 1 || video.Courses[0].CourseID != f.videoCourse.CourseID {
		t.Fatalf("视频页应只有课程 %d，实得 total=%d: %+v", f.videoCourse.CourseID, video.Total, video.Courses)
	}
	// 目录行必须带类型（「全部课程」页也要看得出这门课是什么形式）
	if video.Courses[0].CourseType != model.CourseTypeVideo {
		t.Fatalf("视频页的目录行 course_type = %q", video.Courses[0].CourseType)
	}

	illustrated := decodeData[PublicSchoolCoursePageDTO](t, f.get(t, "/api/public-school/courses?course_type=illustrated&page_size=60"))
	if illustrated.Total != 1 || illustrated.Courses[0].CourseID != f.illustratedCourse.CourseID {
		t.Fatalf("图文页应只有课程 %d，实得 total=%d: %+v", f.illustratedCourse.CourseID, illustrated.Total, illustrated.Courses)
	}

	// 互斥：两页没有交集
	if video.Courses[0].CourseID == illustrated.Courses[0].CourseID {
		t.Fatal("同一门课同时出现在视频页与图文页")
	}
	// 并集 = 全量
	if video.Total+illustrated.Total != all.Total {
		t.Fatalf("两页题量之和 %d != 全量 %d（分流有漏或重复）", video.Total+illustrated.Total, all.Total)
	}
}

// TestPublicSchoolCourseTypeCombinesWithCategoryAndKeyword 类型与分类、关键词三个筛选叠加
// （票 Testing Decisions：「覆盖类型、证件、关键词与分页组合」）。
func TestPublicSchoolCourseTypeCombinesWithCategoryAndKeyword(t *testing.T) {
	f := setupCourseTypeFixture(t)

	// 类型 + 分类：视频课在第二分类，图文课在第一分类
	got := decodeData[PublicSchoolCoursePageDTO](t, f.get(t,
		fmt.Sprintf("/api/public-school/courses?course_type=video&category_id=%d", f.videoOnlyCategory.ID)))
	if got.Total != 1 || got.Courses[0].CourseID != f.videoCourse.CourseID {
		t.Fatalf("类型+分类组合错: total=%d %+v", got.Total, got.Courses)
	}
	// 类型 + 分类：图文课不在第二分类 ⇒ 空集（200 而非 404）
	got = decodeData[PublicSchoolCoursePageDTO](t, f.get(t,
		fmt.Sprintf("/api/public-school/courses?course_type=illustrated&category_id=%d", f.videoOnlyCategory.ID)))
	if got.Total != 0 {
		t.Fatalf("图文页 + 只有视频课的分类应为空集，实得 %d", got.Total)
	}

	// 类型 + 关键词：命中
	q := url.QueryEscape("货叉")
	got = decodeData[PublicSchoolCoursePageDTO](t, f.get(t, "/api/public-school/courses?course_type=video&q="+q))
	if got.Total != 1 {
		t.Fatalf("类型+关键词应命中 1 门，实得 %d", got.Total)
	}
	// 类型 + 关键词：词在另一类型里 ⇒ 空集（类型筛选先于/并行于关键词，不会串味）
	got = decodeData[PublicSchoolCoursePageDTO](t, f.get(t, "/api/public-school/courses?course_type=illustrated&q="+q))
	if got.Total != 0 {
		t.Fatalf("图文页不该命中视频课的标题，实得 %d", got.Total)
	}

	// 类型 + 分页：类型筛选在分页前应用
	got = decodeData[PublicSchoolCoursePageDTO](t, f.get(t, "/api/public-school/courses?course_type=video&page_size=1&page=2"))
	if got.Total != 1 || got.Pages != 1 || len(got.Courses) != 0 {
		t.Fatalf("类型筛选应先于分页：total=%d pages=%d len=%d", got.Total, got.Pages, len(got.Courses))
	}
}

// TestPublicSchoolCategoryOptionsNarrowToType 类型页的证件下拉只列该类型有公开课的分类（US-4）。
func TestPublicSchoolCategoryOptionsNarrowToType(t *testing.T) {
	f := setupCourseTypeFixture(t)

	// 全部课程：两个分类都有课
	all := decodeData[PublicSchoolCategoryListDTO](t, f.get(t, "/api/public-school/categories"))
	if len(all.Categories) != 2 {
		t.Fatalf("全部分类选项应有 2 个（两个分类各有公开课），实得 %d: %+v", len(all.Categories), all.Categories)
	}

	// 视频页：只有第二分类（第一分类只有图文课）
	video := decodeData[PublicSchoolCategoryListDTO](t, f.get(t, "/api/public-school/categories?course_type=video"))
	if len(video.Categories) != 1 || video.Categories[0].ID != f.videoOnlyCategory.ID {
		t.Fatalf("视频页分类选项应只有 %d，实得 %+v", f.videoOnlyCategory.ID, video.Categories)
	}
	if video.Categories[0].CourseCount != 1 {
		t.Fatalf("视频页的 course_count 应按类型计数（应为 1），实得 %d", video.Categories[0].CourseCount)
	}

	// 图文页：只有第一分类
	illustrated := decodeData[PublicSchoolCategoryListDTO](t, f.get(t, "/api/public-school/categories?course_type=illustrated"))
	if len(illustrated.Categories) != 1 || illustrated.Categories[0].ID != f.activeCategory.ID {
		t.Fatalf("图文页分类选项应只有 %d，实得 %+v", f.activeCategory.ID, illustrated.Categories)
	}
}

// TestPublicSchoolRejectsInvalidCourseType 非法类型值答 400 —— 不静默回退成「全部」。
//
// 为什么这条必须单独立：静默回退是**最舒服的**实现，也是最坏的语义——
// 访客点「视频课程」，URL 里的参数被手改坏，页面却照常给全部课程，
// 标签与内容对不上，且没有任何信号说明发生了什么。
func TestPublicSchoolRejectsInvalidCourseType(t *testing.T) {
	f := setupCourseTypeFixture(t)
	for _, bad := range []string{"mixed", "VIDEO", "Video", "illustrated ", "0"} {
		w := f.get(t, "/api/public-school/courses?course_type="+url.QueryEscape(bad))
		if w.Code != http.StatusBadRequest {
			t.Fatalf("course_type=%q 应为 400，实得 %d: %s", bad, w.Code, w.Body)
		}
	}
	// 空串与缺省都合法（= 全部课程）：空串是表单「全部分类」选项提交的形状
	for _, path := range []string{
		"/api/public-school/courses",
		"/api/public-school/courses?course_type=",
		"/api/public-school/categories?course_type=",
	} {
		if w := f.get(t, path); w.Code != http.StatusOK {
			t.Fatalf("%s 应为 200，实得 %d: %s", path, w.Code, w.Body)
		}
	}
}

// TestPublicSchoolDetailCarriesCourseTypeAndSurvivesTypeChange 详情带类型；改类型后
// **分享链接仍然有效**（US-7：收藏或分享的地址不会因为归类变化而 404）。
func TestPublicSchoolDetailCarriesCourseTypeAndSurvivesTypeChange(t *testing.T) {
	f := setupCourseTypeFixture(t)
	path := fmt.Sprintf("/api/public-school/courses/%d", f.videoCourse.CourseID)

	w := f.get(t, path)
	if w.Code != http.StatusOK {
		t.Fatalf("详情 = %d: %s", w.Code, w.Body)
	}
	if got := decodeData[PublicSchoolCourseDetailDTO](t, w).CourseType; got != model.CourseTypeVideo {
		t.Fatalf("详情 course_type = %q, want video", got)
	}

	// 管理员把该课从视频改成图文（该课没有章节，故内容级互斥不拦）
	if err := f.db.Model(&model.Course{}).Where("course_id = ?", f.videoCourse.CourseID).
		Update("course_type", model.CourseTypeIllustrated).Error; err != nil {
		t.Fatal(err)
	}

	// 分享链接仍 200（类型不是可见性条件的一部分——这是本条的**核心**判据：
	// 若把类型塞进详情资格谓词，改一次类型就会让所有已分享链接 404）
	w = f.get(t, path)
	if w.Code != http.StatusOK {
		t.Fatalf("改类型后原详情链接 = %d（分享链接不该因归类变化失效）: %s", w.Code, w.Body)
	}
	if got := decodeData[PublicSchoolCourseDetailDTO](t, w).CourseType; got != model.CourseTypeIllustrated {
		t.Fatalf("改类型后详情 course_type = %q, want illustrated", got)
	}

	// 而目录层面已换边：视频页不再有它、图文页有它
	video := decodeData[PublicSchoolCoursePageDTO](t, f.get(t, "/api/public-school/courses?course_type=video"))
	for _, item := range video.Courses {
		if item.CourseID == f.videoCourse.CourseID {
			t.Fatal("改类型后该课仍出现在视频页")
		}
	}
	illustrated := decodeData[PublicSchoolCoursePageDTO](t, f.get(t, "/api/public-school/courses?course_type=illustrated"))
	found := false
	for _, item := range illustrated.Courses {
		if item.CourseID == f.videoCourse.CourseID {
			found = true
		}
	}
	if !found {
		t.Fatal("改类型后该课没出现在图文页")
	}
}
