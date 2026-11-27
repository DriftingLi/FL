// 课程域的 nonnil 行为例（ADR-0065 决策 5 判据 5）：跑真实出口、看发出的是 [] 还是 null。
//
// 为什么证据跟着域包走（ADR-0070）：判据 5 的表是「哪些域声明 nonnil、谁举证」的对应关系，
// 域包拆出去之后域的实现与它的举证住在同一个包里，改名/搬目录不会让两侧各自漂。
// 断言本体只有一份：testutil.AssertNonNilOutlets。
//
// 两个课程 DTO 字段**不在这里**：它们的生产者不是本包 —— course.ChapterDTO.files 只由
// TutorService.GetCourseChapters 填（波 4d 起该出口与它的证据都在 internal/tutor/nonnil_outlets_test.go），
// course.CourseDTO.chapters 只由 TrainingCatalogService.GetAdminCatalogTree 的 withChapters 分支填
// （证据在 internal/training/nonnil_outlets_test.go）。证据跟真实出口走，不跟类型名的前缀走。
package course

import (
	"context"
	"forklift-training/internal/notification"
	"testing"

	"go.uber.org/zap"
	"gorm.io/gorm"

	"forklift-training/internal/model"

	"forklift-training/internal/testutil"
)

// nonnilOutletsCourse 键 = 包名.类型名.json键，值 = 走真实出口取到的结果。
// 前缀 nonnilOutlets 是约定的证据表名：apitypes 的表态锁按它扫目录收键（见
// internal/apitypes/nullability_lock_test.go 的 nonNilEvidenceSources）。
var nonnilOutletsCourse = map[string]func(t *testing.T) any{
	"course.CoursePageResult.courses":          outletCoursePageNoCourses,
	"course.CourseFacets.categories":           outletAdminCourseFacetsEmpty,
	"course.AdminCourseDetailDTO.chapters":     outletAdminCourseDetailNoChapters,
	"course.CourseDTO.prerequisites":           outletAdminCourseDetailMeta,
	"course.CourseDTO.prerequisite_course_ids": outletAdminCourseDetailMeta,
	"course.CourseDetailDTO.chapters":          outletCourseDetailNoChapters,
	"course.PublicCourseDetailDTO.chapters":    outletPublicCourseDetailNoChapters,
	"course.PublicCoursePageDTO.courses":       outletPublicCoursePageNoCourses,
	"course.ChapterDetailDTO.files":            outletChapterDetailNoFiles,
	// #2499 C9 审核队列三型：空队列/无文件快照/无失败明细都必须发 `[]` 而不是 null
	// （前端 v-for 与 failures 遍历直接消费，null 会炸）。
	"course.ChapterReviewQueueResult.items": outletEmptyReviewQueue,
	"course.ChapterReviewSnapshotDTO.files": outletSnapshotNoFiles,
	"course.BatchApproveResult.failures":    outletBatchApproveNoFailures,
	"course.BatchRejectResult.failures":     outletBatchRejectNoFailures,
	// #2596 D1：无人认领的课程发空名单 `[]`（章节页的共认名单直接 v-for，null 会炸）。
	"course.CourseClaimantsResult.claimants": outletNoClaimants,
	// #2753 独立公开网校读面三格：空库/无合格分类时也必须发 `[]` 而不是 null
	// （htmx 模板的 range 与分类下拉直接消费，null 会让「无结果」渲染成「加载失败」）。
	"course.PublicSchoolCategoryListDTO.categories": outletPublicSchoolNoCategories,
	"course.PublicSchoolCoursePageDTO.courses":      outletPublicSchoolNoCourses,
	"course.PublicSchoolCourseDetailDTO.chapters":   outletPublicSchoolDetailNoChapters,
	// #2760 证件题库目录两格：空库时 question_banks 发 `[]`、每个分类的 by_type 也是 `[]`-等价的
	// 空 map（JSON `{}`，不是 null）——前端模板与 open-school 的 range 都直接消费。
	"course.PublicQuestionBankListDTO.question_banks": outletPublicQuestionBanksEmpty,
	"course.PublicQuestionBankCategoryDTO.by_type":    outletPublicQuestionBanksAggregated,
}

// outletAdminCourseFacetsEmpty 空库管理端列表：facet 分类数组发 `[]` 而不是 null。
func outletAdminCourseFacetsEmpty(t *testing.T) any {
	t.Helper()
	facets, err := loadAdminCourseFacets(testutil.NewMemoryDB(t))
	if err != nil {
		t.Fatalf("空库加载课程 facets 失败: %v", err)
	}
	return facets
}

// outletPublicQuestionBanksEmpty 空库拉题库目录：question_banks 发 `[]` 而不是 null。
func outletPublicQuestionBanksEmpty(t *testing.T) any {
	t.Helper()
	res, err := NewService(testutil.NewMemoryDB(t), nil, zap.NewNop()).ListPublicQuestionBanks(context.Background())
	if err != nil {
		t.Fatalf("空库拉公开题库目录失败: %v", err)
	}
	return res
}

// outletPublicQuestionBanksAggregated 有题的分类：by_type 必为实际命中的题型字典（非 null）。
func outletPublicQuestionBanksAggregated(t *testing.T) any {
	t.Helper()
	db := testutil.NewMemoryDB(t)
	cred := model.Credential{Code: "QB", Name: "题库证件", Category: "special_operation", Status: 1, SortOrder: 1}
	if err := db.Create(&cred).Error; err != nil {
		t.Fatal(err)
	}
	cat := model.TrainingCategory{Name: "题库分类", Status: 1, LegacyCredentialID: &cred.ID}
	if err := db.Create(&cat).Error; err != nil {
		t.Fatal(err)
	}
	q := model.Question{Type: "single_choice", Content: "池内题", Answer: "A", Status: "published", CategoryID: &cat.ID}
	if err := db.Create(&q).Error; err != nil {
		t.Fatal(err)
	}
	res, err := NewService(db, nil, zap.NewNop()).ListPublicQuestionBanks(context.Background())
	if err != nil {
		t.Fatalf("公开题库目录失败: %v", err)
	}
	if len(res.QuestionBanks) != 1 {
		t.Fatalf("应有 1 个合格分类，实得 %d", len(res.QuestionBanks))
	}
	// 举证对象是字段所属的那一行 DTO，不是整张列表信封——
	// AssertNonNilOutlets 在返回值顶层找 jsonKey（by_type）。
	return res.QuestionBanks[0]
}

// outletPublicSchoolNoCategories 空库拉公开网校分类选项：无合格分类时 categories 发 `[]`。
func outletPublicSchoolNoCategories(t *testing.T) any {
	t.Helper()
	res, err := NewService(testutil.NewMemoryDB(t), nil, zap.NewNop()).ListPublicSchoolCategories(context.Background(), "")
	if err != nil {
		t.Fatalf("空库拉公开网校分类失败: %v", err)
	}
	return res
}

// outletPublicSchoolNoCourses 空库拉公开网校目录：无课程时 courses 发 `[]`。
func outletPublicSchoolNoCourses(t *testing.T) any {
	t.Helper()
	res, err := NewService(testutil.NewMemoryDB(t), nil, zap.NewNop()).ListPublicSchoolCourses(context.Background(), publicSchoolListRequest{})
	if err != nil {
		t.Fatalf("空库拉公开网校目录失败: %v", err)
	}
	return res
}

// outletPublicSchoolDetailNoChapters 合格的公开课但无已发布章节：chapters 发 `[]`。
//
// 夹具必须把分类同时做成「已启用 + 映射到有效启用证件」——这是公开网校相对既有公开详情
// 多出来的两条判据（#2753 节点 A）；沿用 seedVisibleCourse 的分类（无证件映射）会直接 404。
func outletPublicSchoolDetailNoChapters(t *testing.T) any {
	t.Helper()
	db := testutil.NewMemoryDB(t)
	credential := model.Credential{Code: "PUB", Name: "公开面证件", Category: "special_operation", Status: 1, SortOrder: 1}
	if err := db.Create(&credential).Error; err != nil {
		t.Fatal(err)
	}
	category := model.TrainingCategory{Name: "公开面分类", Status: 1, LegacyCredentialID: &credential.ID}
	if err := db.Create(&category).Error; err != nil {
		t.Fatal(err)
	}
	item := model.Course{Name: "无章节公开课", CategoryID: &category.ID, Status: 1}
	if err := db.Create(&item).Error; err != nil {
		t.Fatal(err)
	}
	res, err := NewService(db, nil, zap.NewNop()).GetPublicSchoolCourseDetail(context.Background(), item.CourseID)
	if err != nil {
		t.Fatalf("公开网校详情失败: %v", err)
	}
	return res
}

func outletPublicCourseDetailNoChapters(t *testing.T) any {
	db := testutil.NewMemoryDB(t)
	category := model.TrainingCategory{Name: "维修", Status: 1}
	if err := db.Create(&category).Error; err != nil {
		t.Fatal(err)
	}
	item := model.Course{Name: "目录样例", CategoryID: &category.ID}
	if err := db.Create(&item).Error; err != nil {
		t.Fatal(err)
	}
	result, err := NewService(db, nil, zap.NewNop()).GetPublicCourseDetail(context.Background(), item.CourseID)
	if err != nil {
		t.Fatal(err)
	}
	return result
}

func outletPublicCoursePageNoCourses(t *testing.T) any {
	result, err := NewService(testutil.NewMemoryDB(t), nil, zap.NewNop()).ListPublicCourses(context.Background(), publicCourseListRequest{})
	if err != nil {
		t.Fatal(err)
	}
	return result
}

// TestNonNilDeclaredOutletsNeverEmitNull 本域的举证入口（判据本体在 testutil）。
func TestNonNilDeclaredOutletsNeverEmitNull(t *testing.T) {
	testutil.AssertNonNilOutlets(t, []map[string]func(t *testing.T) any{nonnilOutletsCourse})
}

// outletCoursePageNoCourses 空库拉学员端课程列表：无一行课程时 courses 发 `[]`。
func outletCoursePageNoCourses(t *testing.T) any {
	t.Helper()
	res, err := NewService(testutil.NewMemoryDB(t), nil, zap.NewNop()).GetCourses(1, 20, nil, nil, nil, "", nil)
	if err != nil {
		t.Fatalf("空库拉课程列表失败: %v", err)
	}
	return res
}

// outletAdminCourseDetailNoChapters 管理端课程详情：章节由 loadCourseWithChapters 一次性 make 出来，
// 无章节时也发 `[]`（与学员端 CourseDetailDTO.chapters 同一个装载实现）。
func outletAdminCourseDetailNoChapters(t *testing.T) any {
	t.Helper()
	db := testutil.NewMemoryDB(t)
	res, err := NewAdminService(db, nil, notification.NewService(db, zap.NewNop()), zap.NewNop()).GetCourseDetail(seedVisibleCourse(t, db))
	if err != nil {
		t.Fatalf("管理端课程详情失败: %v", err)
	}
	return res
}

// outletAdminCourseDetailMeta 一次调用同时举证 prerequisites 与 prerequisite_course_ids：
// fillCourseMeta 两者都以 `make(0,n)` 起手再取地址，于是 omitempty 只省掉「没调本函数」的读路径，
// 调过的路径恒发数组（`[]` 也算发过）。
func outletAdminCourseDetailMeta(t *testing.T) any {
	t.Helper()
	db := testutil.NewMemoryDB(t)
	res, err := NewAdminService(db, nil, notification.NewService(db, zap.NewNop()), zap.NewNop()).GetCourseDetail(seedVisibleCourse(t, db))
	if err != nil {
		t.Fatalf("管理端课程详情失败: %v", err)
	}
	return res
}

// outletCourseDetailNoChapters 学员端课程详情：chapters 由 loadCourseWithChapters 以 make(0,n) 起手，
// 无章节时发 `[]`。
func outletCourseDetailNoChapters(t *testing.T) any {
	t.Helper()
	db := testutil.NewMemoryDB(t)
	res, err := NewService(db, nil, zap.NewNop()).GetCourseDetail(seedVisibleCourse(t, db), 0)
	if err != nil {
		t.Fatalf("课程详情失败: %v", err)
	}
	return res
}

// outletChapterDetailNoFiles 章节详情（导师档 fillStudyStatus=false）：挑那条既无 chapter_file 行、
// file_url 也是空的章节（夹具里第 2 条）——两条 legacy/表条目分支都不进，才落在 `fileList == nil ⇒ []` 那格。
//
// 直接调 ChapterDetailShared 而不是某个服务的包装：它就是真实出口本体
// （tutor.Service.GetChapterDetail 回的是它、学员端 Service.GetChapterDetail
// 只是多一道可见性谓词后回它），域包内也不需要导师域的服务。
func outletChapterDetailNoFiles(t *testing.T) any {
	t.Helper()
	db := testutil.NewMemoryDB(t)
	_, chapters := seedChapterWithMeta(t, db)
	res, err := ChapterDetailShared(db, &chapters[1], false, 0, false)
	if err != nil {
		t.Fatalf("章节详情失败: %v", err)
	}
	return res
}

// outletEmptyReviewQueue 空库拉审核队列：无待审章节时 items 发 `[]`（#2499 C9）。
func outletEmptyReviewQueue(t *testing.T) any {
	t.Helper()
	res, err := NewAdminService(testutil.NewMemoryDB(t), nil, notification.NewService(testutil.NewMemoryDB(t), zap.NewNop()), zap.NewNop()).ListPendingChapterReviews(context.Background(), 1, 20)
	if err != nil {
		t.Fatalf("空库拉审核队列失败: %v", err)
	}
	return res
}

// outletSnapshotNoFiles 在审快照但章节无文件：files 发 `[]`（#2499 C9）。
func outletSnapshotNoFiles(t *testing.T) any {
	t.Helper()
	db := testutil.NewMemoryDB(t)
	course := model.Course{Name: "快照课程", Status: 1, CreatedAt: testutil.Now()}
	if err := db.Create(&course).Error; err != nil {
		t.Fatalf("播种课程失败: %v", err)
	}
	ch := model.Chapter{CourseID: course.CourseID, Title: "无文件章", Content: "正文",
		ContentType: "text", CreatedAt: testutil.Now()}
	if err := db.Create(&ch).Error; err != nil {
		t.Fatalf("播种章节失败: %v", err)
	}
	if err := SubmitChapterForReview(context.Background(), db, ch.ChapterID, 1); err != nil {
		t.Fatalf("提交审核失败: %v", err)
	}
	res, err := NewAdminService(db, nil, notification.NewService(db, zap.NewNop()), zap.NewNop()).GetChapterReviewSnapshot(context.Background(), ch.ChapterID)
	if err != nil {
		t.Fatalf("查快照失败: %v", err)
	}
	return res
}

// outletBatchApproveNoFailures 空批量过审：failures 发 `[]`（#2499 C9）。
func outletBatchApproveNoFailures(t *testing.T) any {
	t.Helper()
	res, err := NewAdminService(testutil.NewMemoryDB(t), nil, notification.NewService(testutil.NewMemoryDB(t), zap.NewNop()), zap.NewNop()).
		BatchApproveChapterReviews(context.Background(), nil)
	if err != nil {
		t.Fatalf("空批量过审失败: %v", err)
	}
	return res
}

// outletBatchRejectNoFailures 空批量驳回：failures 发 `[]`（#2499 C9）。
func outletBatchRejectNoFailures(t *testing.T) any {
	t.Helper()
	res, err := NewAdminService(testutil.NewMemoryDB(t), nil, notification.NewService(testutil.NewMemoryDB(t), zap.NewNop()), zap.NewNop()).
		BatchRejectChapterReviews(context.Background(), nil, "统一理由")
	if err != nil {
		t.Fatalf("空批量驳回失败: %v", err)
	}
	return res
}

// outletNoClaimants 无人认领的课程：名单发 `[]` 而不是 null（#2596 D1）。
// 走真实出口 ListCourseClaimants（tutor.Service 那层只是加了「课程存在」前置再回它）。
func outletNoClaimants(t *testing.T) any {
	t.Helper()
	db := testutil.NewMemoryDB(t)
	claimants, err := ListCourseClaimants(db, seedVisibleCourse(t, db))
	if err != nil {
		t.Fatalf("空名单查询失败: %v", err)
	}
	return CourseClaimantsResult{Claimants: claimants}
}

// seedVisibleCourse 播一门「已发布 + 已挂载」但**没有章节**的课程，返回其 id。
// （与 internal/core 的同名夹具各持一份：两包互不 import 对方的测试文件。）
func seedVisibleCourse(t *testing.T, db *gorm.DB) int {
	t.Helper()
	spec := model.Specialty{Code: "nonnil", Name: "非空方向", SortOrder: 1, Status: 1}
	if err := db.Create(&spec).Error; err != nil {
		t.Fatalf("播种方向失败: %v", err)
	}
	lv := model.CourseLevel{Code: "nonnil-lv", Name: "非空等级", SortOrder: 1, Status: 1}
	if err := db.Create(&lv).Error; err != nil {
		t.Fatalf("播种等级失败: %v", err)
	}
	pos := spec.SpecialtyID
	lvl := lv.LevelID
	course := model.Course{Name: "非空课程", CategoryID: &testutil.SeedTrainingCategory(t, db, "课程分类").ID, Status: 1, SpecialtyID: &pos, LevelID: &lvl, CreatedAt: testutil.Now()}
	if err := db.Create(&course).Error; err != nil {
		t.Fatalf("播种课程失败: %v", err)
	}
	return course.CourseID
}
