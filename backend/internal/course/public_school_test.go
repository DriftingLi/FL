// 公开网校读面的契约测试（#2753 实施地图节点 B）。
//
// 判据面（节点 A 已拍板的口径）在这里逐条钉住，避免日后有人把「分类存在即可见」
// 当回公开判据：
//  1. 分类必须**已启用**（停用分类 = 空项）
//  2. 分类必须**映射到证件**（无映射的独立分类不属于公开面）
//  3. 映射到的证件必须**已启用**（证件停用 = 该分类整体退出公开面）
//  4. 课程必须已发布
//  5. 分类选项隐藏空项（course_count > 0 才出）
//  6. 列表与直达详情**同判**：列表里看不见的课，直达详情必须 404
//  7. 关键词只匹配公开名称与简介，且先筛后分页（不是筛完一页再丢）
//  8. 详情不泄漏章节正文/文件地址/权益/进度
//
// 为什么逐条写而不是写「一个 happy path」：这些判据每一条都对应一个**具体的泄漏方向**
// （停用分类仍在公开面、无映射分类混进公开目录、关键词匹配到未公开正文……），
// 合成一个用例就只能测到「最松的那条判据」。
package course

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
	"gorm.io/gorm"

	"forklift-training/internal/model"
	"forklift-training/internal/security"
	"forklift-training/internal/testutil"
)

// publicSchoolFixture 公开网校判据夹具：把「分类 × 证件 × 课程」的笛卡尔组合一次建好。
//
// 每个字段对应一条判据，用例名里带着它，红了就一眼看出是哪条判据破了。
type publicSchoolFixture struct {
	router *gin.Engine
	db     *gorm.DB

	activeCategory   model.TrainingCategory // 已启用 + 映射到启用证件（公开面唯一合法来源）
	disabledCategory model.TrainingCategory // 已启用，但映射到的证件停用
	unmappedCategory model.TrainingCategory // 已启用，但无证件映射
	offCategory      model.TrainingCategory // 停用分类（映射有效）

	visibleCourse  model.Course // 完全合格：公开面可见
	unpubCourse    model.Course // 已发布但课程 status=0
	disabledCatCrs model.Course // 分类映射到停用证件
	unmappedCatCrs model.Course // 分类无证件映射
	offCatCourse   model.Course // 停用分类下的已发布课程
}

// setupPublicSchoolFixture 建夹具（内存库，无 Postgres 依赖 —— 这几条判据全是读面谓词，
// 不需要真实 PG；PG 契约面（迁移/回填）由 api 包的 Postgres 用例覆盖）。
func setupPublicSchoolFixture(t *testing.T) *publicSchoolFixture {
	t.Helper()
	testutil.SetTestGinMode()
	db := testutil.NewMemoryDB(t)

	activeCred := model.Credential{Code: "N1", Name: "叉车司机N1证", Category: "special_operation", Status: 1, SortOrder: 1}
	disabledCred := model.Credential{Code: "OFF", Name: "已停用证件", Category: "special_operation", Status: 1, SortOrder: 2}
	stoppedCatCred := model.Credential{Code: "STOP", Name: "映射到停用分类的证件", Category: "special_operation", Status: 1, SortOrder: 3}
	if err := db.Create(&activeCred).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&disabledCred).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&stoppedCatCred).Error; err != nil {
		t.Fatal(err)
	}
	// 停用证件：建好后再置 0（模型的 default:1 会覆盖创建时的 0）
	if err := db.Model(&disabledCred).Update("status", int16(0)).Error; err != nil {
		t.Fatal(err)
	}

	f := &publicSchoolFixture{db: db}
	categories := []model.TrainingCategory{
		// 公开面唯一合法来源：启用 + 映射到启用证件
		{Name: "叉车司机N1证", SortOrder: 1, Status: 1, LegacyCredentialID: &activeCred.ID},
		// 启用但映射到停用证件 ⇒ 整类退出公开面
		{Name: "已停用证件分类", SortOrder: 2, Status: 1, LegacyCredentialID: &disabledCred.ID},
		// 启用但无证件映射 ⇒ 不属于公开面（本机预览库的「叉车维修」「安全操作」即此形态）
		{Name: "无映射独立分类", SortOrder: 3, Status: 1},
		// 停用分类（映射到自己的证件，验证「停用」这一维独立于证件是否启用）
		{Name: "已停用分类", SortOrder: 4, Status: 1, LegacyCredentialID: &stoppedCatCred.ID},
	}
	if err := db.Create(&categories).Error; err != nil {
		t.Fatal(err)
	}
	f.activeCategory = categories[0]
	f.disabledCategory = categories[1]
	f.unmappedCategory = categories[2]
	f.offCategory = categories[3]
	// 停用分类：建好后再置 0（同上，模型的 default:1 会覆盖创建时的 0）
	if err := db.Model(&f.offCategory).Update("status", int16(0)).Error; err != nil {
		t.Fatal(err)
	}

	price := 100
	courses := []model.Course{
		{Name: "液压系统原理与维护", Description: "讲液压泵与阀的检修", CategoryID: &f.activeCategory.ID, Status: 1, PointsPrice: &price},
		{Name: "已下架的旧课", Description: "这门课已下架", CategoryID: &f.activeCategory.ID, Status: 1},
		{Name: "停用证件下的课", Description: "证件停用后不该出现在公开面", CategoryID: &f.disabledCategory.ID, Status: 1},
		{Name: "无映射分类下的课", Description: "独立分类不属于公开面", CategoryID: &f.unmappedCategory.ID, Status: 1},
		{Name: "停用分类下的课", Description: "停用分类不该出现在公开面", CategoryID: &f.offCategory.ID, Status: 1},
	}
	if err := db.Create(&courses).Error; err != nil {
		t.Fatal(err)
	}
	f.visibleCourse = courses[0]
	if f.visibleCourse.SpecialtyID != nil || f.visibleCourse.LevelID != nil {
		t.Fatal("公开目录与详情的正向夹具应同时缺少专业方向和课程等级挂载")
	}
	f.unpubCourse = courses[1]
	f.disabledCatCrs = courses[2]
	f.unmappedCatCrs = courses[3]
	f.offCatCourse = courses[4]
	// 下架：status=0（模型的 default:1 会覆盖创建时的 0，故建好后更新）
	if err := db.Model(&f.unpubCourse).Update("status", int16(0)).Error; err != nil {
		t.Fatal(err)
	}

	// 章节：一门已发布槽位（公开可见元数据）、一门仅待审槽位（不得出现在公开详情）
	chapters := []model.Chapter{
		{CourseID: f.visibleCourse.CourseID, Title: "先读", OrderNum: 1, Content: "私人正文：不该外发", ContentType: "text", FileURL: "/private/working", Duration: 12},
		{CourseID: f.visibleCourse.CourseID, Title: "在审章节", OrderNum: 2, Content: "待审正文", ContentType: "text"},
	}
	if err := db.Create(&chapters).Error; err != nil {
		t.Fatal(err)
	}
	slots := []model.ChapterContentSlot{
		{ChapterID: chapters[0].ChapterID, Slot: model.ChapterReviewPublished, ContentType: "video", FileURL: "/private/published"},
		{ChapterID: chapters[1].ChapterID, Slot: model.ChapterReviewPending, ContentType: "text"},
	}
	if err := db.Create(&slots).Error; err != nil {
		t.Fatal(err)
	}

	f.router = gin.New()
	session := security.NewSession("public-school-test", time.Hour, security.CookieConfig{})
	RegisterRoutes(f.router.Group("/api"), session, nil, NewService(db, nil, zap.NewNop()))
	return f
}

// get 打一条 GET，返回 recorder。
func (f *publicSchoolFixture) get(t *testing.T, path string) *httptest.ResponseRecorder {
	t.Helper()
	w := httptest.NewRecorder()
	f.router.ServeHTTP(w, httptest.NewRequest(http.MethodGet, path, nil))
	return w
}

// decodeData 解信封的 data 段。
func decodeData[T any](t *testing.T, w *httptest.ResponseRecorder) T {
	t.Helper()
	var envelope struct {
		Data json.RawMessage `json:"data"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &envelope); err != nil {
		t.Fatalf("解信封失败: %v (%s)", err, w.Body)
	}
	var out T
	if err := json.Unmarshal(envelope.Data, &out); err != nil {
		t.Fatalf("解 data 失败: %v (%s)", err, envelope.Data)
	}
	return out
}

// TestPublicSchoolCategoriesHideEmptyAndIneligible 分类选项的三条判据 + 空项隐藏。
func TestPublicSchoolCategoriesHideEmptyAndIneligible(t *testing.T) {
	f := setupPublicSchoolFixture(t)
	w := f.get(t, "/api/public-school/categories")
	if w.Code != http.StatusOK {
		t.Fatalf("分类选项 = %d: %s", w.Code, w.Body)
	}
	list := decodeData[PublicSchoolCategoryListDTO](t, w)
	if len(list.Categories) != 1 {
		t.Fatalf("只应有 1 个合格分类（启用+映射有效证件+有公开课），实得 %d: %s", len(list.Categories), w.Body)
	}
	got := list.Categories[0]
	if got.ID != f.activeCategory.ID {
		t.Fatalf("分类 id = %d, want %d", got.ID, f.activeCategory.ID)
	}
	// 显示名与排序直接沿用分类行（节点 A 口径：不从证件表重新取，避免两处名字漂移）
	if got.Name != f.activeCategory.Name {
		t.Fatalf("分类名 = %q, want %q（公开显示名沿用分类行）", got.Name, f.activeCategory.Name)
	}
	// 计数口径：只剩一门合格课（另一门已下架），停用/无映射/停用证件下的三门不计
	if got.CourseCount != 1 {
		t.Fatalf("course_count = %d, want 1（只数公开面可见的课）", got.CourseCount)
	}
}

// TestPublicSchoolCoursesApplyEligibilityAndHideInvisible 目录只出合格课；不合格分类 id 返回空集而非 404。
func TestPublicSchoolCoursesApplyEligibilityAndHideInvisible(t *testing.T) {
	f := setupPublicSchoolFixture(t)
	w := f.get(t, "/api/public-school/courses")
	if w.Code != http.StatusOK {
		t.Fatalf("目录 = %d: %s", w.Code, w.Body)
	}
	page := decodeData[PublicSchoolCoursePageDTO](t, w)
	if page.Total != 1 || len(page.Courses) != 1 {
		t.Fatalf("目录只应有 1 门合格课，实得 total=%d len=%d: %s", page.Total, len(page.Courses), w.Body)
	}
	if page.Courses[0].CourseID != f.visibleCourse.CourseID {
		t.Fatalf("目录课程 = %d, want %d", page.Courses[0].CourseID, f.visibleCourse.CourseID)
	}
	if page.Courses[0].PointsPrice == nil || *page.Courses[0].PointsPrice != 100 {
		t.Fatalf("目录行丢了积分价: %s", w.Body)
	}
	if page.Courses[0].Category.ID != f.activeCategory.ID || page.Courses[0].Category.Name != f.activeCategory.Name {
		t.Fatalf("目录行丢了分类投影: %s", w.Body)
	}

	// 每个不合格分类都返回空集（200），不是 404：目录是发现面，空集可表达。
	for _, tc := range []struct {
		name string
		id   int
	}{
		{"停用证件分类", f.disabledCategory.ID},
		{"无映射分类", f.unmappedCategory.ID},
		{"停用分类", f.offCategory.ID},
		{"不存在的分类", 99999},
	} {
		w := f.get(t, fmt.Sprintf("/api/public-school/courses?category_id=%d", tc.id))
		if w.Code != http.StatusOK {
			t.Fatalf("%s 的分类筛选应为 200 空集，实得 %d: %s", tc.name, w.Code, w.Body)
		}
		got := decodeData[PublicSchoolCoursePageDTO](t, w)
		if got.Total != 0 || len(got.Courses) != 0 {
			t.Fatalf("%s 不该有公开课，实得 total=%d: %s", tc.name, got.Total, w.Body)
		}
	}
}

// TestPublicSchoolListAndDetailAgreeOnVisibility 列表与直达详情同判（节点 B 的显式要求）：
// 目录里看不见的课，直达详情必须 404 —— 否则「藏起来」只是藏了入口，链接仍可传播。
func TestPublicSchoolListAndDetailAgreeOnVisibility(t *testing.T) {
	f := setupPublicSchoolFixture(t)

	// 正向：合格课详情 200 且形状正确
	w := f.get(t, fmt.Sprintf("/api/public-school/courses/%d", f.visibleCourse.CourseID))
	if w.Code != http.StatusOK {
		t.Fatalf("合格课详情 = %d: %s", w.Code, w.Body)
	}
	detail := decodeData[PublicSchoolCourseDetailDTO](t, w)
	if detail.CourseID != f.visibleCourse.CourseID || detail.Category.ID != f.activeCategory.ID {
		t.Fatalf("详情投影错: %s", w.Body)
	}
	if detail.ChapterCount != 1 || len(detail.Chapters) != 1 || detail.Chapters[0].Title != "先读" {
		t.Fatalf("只应出 1 个已发布章节（在审章节不得出现），实得 %d: %s", len(detail.Chapters), w.Body)
	}
	if detail.Chapters[0].ContentType != "video" || detail.Chapters[0].Duration != 12 {
		t.Fatalf("章节元数据不完整: %s", w.Body)
	}

	// 反向：四类不可见课 + 不存在 id，全部 404 且同一句文案（不泄漏是哪一态）
	var bodies []string
	for _, tc := range []struct {
		name string
		id   int
	}{
		{"已下架课程", f.unpubCourse.CourseID},
		{"停用证件分类下的课", f.disabledCatCrs.CourseID},
		{"无映射分类下的课", f.unmappedCatCrs.CourseID},
		{"停用分类下的课", f.offCatCourse.CourseID},
		{"不存在的课程", 99999},
	} {
		w := f.get(t, fmt.Sprintf("/api/public-school/courses/%d", tc.id))
		if w.Code != http.StatusNotFound {
			t.Fatalf("%s 详情应为 404，实得 %d: %s ⇒ 列表与详情不同判", tc.name, w.Code, w.Body)
		}
		bodies = append(bodies, w.Body.String())
	}
	for i := 1; i < len(bodies); i++ {
		if bodies[i] != bodies[0] {
			t.Fatalf("三种不可见态与「不存在」的响应体不同，会泄漏是哪一态：\n%s\n%s", bodies[0], bodies[i])
		}
	}
}

// TestPublicSchoolDetailDoesNotExposePrivateData 公开详情只承诺公开字段。
func TestPublicSchoolDetailDoesNotExposePrivateData(t *testing.T) {
	f := setupPublicSchoolFixture(t)
	w := f.get(t, fmt.Sprintf("/api/public-school/courses/%d", f.visibleCourse.CourseID))
	if w.Code != http.StatusOK {
		t.Fatalf("详情 = %d: %s", w.Code, w.Body)
	}
	// 逐个私有字段名断言：这些是「不该外发」的具名事实，不是泛泛而谈的「别泄漏」
	for _, key := range []string{`"content":`, `"file_url":`, `"entitled":`, `"progress":`, `"last_chapter_id":`, "在审章节", "/private/"} {
		if strings.Contains(w.Body.String(), key) {
			t.Errorf("公开详情含私有字段 %s: %s", key, w.Body)
		}
	}
}

// TestPublicSchoolKeywordMatchesPublicFieldsOnly 关键词只匹配公开名称与简介，且先筛后分页。
func TestPublicSchoolKeywordMatchesPublicFieldsOnly(t *testing.T) {
	f := setupPublicSchoolFixture(t)
	// 放一门「简介命中、名称不命中」与一门「都不命中」的课，验证匹配范围是名称∪简介。
	descOnly := model.Course{Name: "另一个题目", Description: "本课讲液压阀的密封更换", CategoryID: &f.activeCategory.ID, Status: 1}
	noMatch := model.Course{Name: "无关课程", Description: "讲电气基础", CategoryID: &f.activeCategory.ID, Status: 1}
	if err := f.db.Create(&descOnly).Error; err != nil {
		t.Fatal(err)
	}
	if err := f.db.Create(&noMatch).Error; err != nil {
		t.Fatal(err)
	}

	// 关键词「液压」：命中 f.visibleCourse（名称含「液压」）与 descOnly（简介含「液压」）
	w := f.get(t, "/api/public-school/courses?q="+url.QueryEscape("液压"))
	if w.Code != http.StatusOK {
		t.Fatalf("关键词查询 = %d: %s", w.Code, w.Body)
	}
	page := decodeData[PublicSchoolCoursePageDTO](t, w)
	if page.Total != 2 {
		t.Fatalf("关键词「液压」应命中 2 门（名称命中 1 + 简介命中 1），实得 total=%d: %s", page.Total, w.Body)
	}

	// 关键的是「在审章节的正文不外泄」：用只在待审正文里出现的词搜，必须 0 命中
	w = f.get(t, "/api/public-school/courses?q="+url.QueryEscape("待审正文"))
	page = decodeData[PublicSchoolCoursePageDTO](t, w)
	if page.Total != 0 {
		t.Fatalf("关键词匹配到了未公开的章节正文（应只匹配名称与简介），实得 total=%d: %s", page.Total, w.Body)
	}

	// LIKE 通配符必须按字面处理：输入 % 不能变成「匹配全部」
	w = f.get(t, "/api/public-school/courses?q="+url.QueryEscape("%"))
	page = decodeData[PublicSchoolCoursePageDTO](t, w)
	if page.Total != 0 {
		t.Fatalf("关键词 %% 被当成通配符刷出了 %d 门课（应按字面匹配，0 命中）", page.Total)
	}

	// 先筛后分页：page_size=1 时，「液压」这一筛应该有 2 页而不是 1 页
	w = f.get(t, "/api/public-school/courses?q="+url.QueryEscape("液压")+"&page_size=1")
	page = decodeData[PublicSchoolCoursePageDTO](t, w)
	if page.Total != 2 || page.Pages != 2 || len(page.Courses) != 1 {
		t.Fatalf("先筛后分页失效：total=%d pages=%d len=%d（应为 2/2/1）", page.Total, page.Pages, len(page.Courses))
	}
	// 第二页必须是另一门（分页真的在翻，而不是每页都同一门）
	first := page.Courses[0].CourseID
	w = f.get(t, "/api/public-school/courses?q="+url.QueryEscape("液压")+"&page_size=1&page=2")
	page = decodeData[PublicSchoolCoursePageDTO](t, w)
	if len(page.Courses) != 1 || page.Courses[0].CourseID == first {
		t.Fatalf("第二页与第一页内容相同或为空：first=%d second=%v", first, page.Courses)
	}
}

// TestPublicSchoolRejectsInvalidCategoryID 分类 ID 非正整数按 400（与既有公开目录同判）。
func TestPublicSchoolRejectsInvalidCategoryID(t *testing.T) {
	f := setupPublicSchoolFixture(t)
	for _, bad := range []string{"0", "-1", "abc"} {
		w := f.get(t, "/api/public-school/courses?category_id="+bad)
		if w.Code != http.StatusBadRequest {
			t.Fatalf("category_id=%s 应为 400，实得 %d: %s", bad, w.Code, w.Body)
		}
	}
	// 缺省不带筛选是合法的
	if w := f.get(t, "/api/public-school/courses"); w.Code != http.StatusOK {
		t.Fatalf("不带 category_id 应为 200，实得 %d: %s", w.Code, w.Body)
	}
}
