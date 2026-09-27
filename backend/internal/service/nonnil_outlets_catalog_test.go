// 目录树 / 字典列表 / FAQ / 章节文件 / 精选 / 练习与模考 / 学员侧读面的 nonnil 行为例
// （批①-A 第④段「wave 16」）。
//
// 为什么单独成文件：判据 5 的证据跟着出口走，一个域一张表（汇总表机制见 nonnil_declaration_test.go）。
// 本段跨的域比 stats/course 两段更杂，共同点只有一件：这些字段全部由 `make([]T, 0, n)` 或
// `make(map, n)` 起手、只 append，从来没有一条成功出口发得出 `null`。
//
// 下面是**逐字段跑过真实出口后留在 `nullable` 的那半**（清单是按域扫出来的，判据是跑出来的）：
//   - AIChatMessageDTO.images / .sources ⇒ GetSessionMessages 里 `var imgs []string` 只在
//     列非空串时才 Unmarshal，「没图 / 没来源」就是 nil ⇒ `null`。那句 `nullable` 是真的
//     （已带 x-nullable，属批①-B 的消费端加宽，不是改判对象）。
//   - DiagnosisFaultCodePage.items ⇒ ListFaultCodes 把上游助手的 JSON 直接 Decode 进结构体，
//     没有任何 `make` 兜底 ⇒ 上游发 `{"items":null}` 或省略该键时这里就是 `null`。
//     出口形状由对端进程决定，本仓改判不了它。
//   - GenTaskStatus.results ⇒ pending 阶段 async_task.result 列还没写过，GetTaskStatus 只在
//     `len(task.Result) > 0` 时才赋值 ⇒ 实测发 `null`（已带 x-nullable）。
//   - MockExamSubmitDTO.details ⇒ Submit 那一侧是 `make(0,n)` 恒非 null，但同一条声明经
//     GetResult（未交卷记录 result 列为空）发 `null`，且 MockExamResultDTO 内嵌本 DTO 共用它。
//   - ProgressResultDTO.answers_state ⇒ 从未存过进度时是 nil map ⇒ `null`（已带 x-nullable，
//     GetProgress 无 error 出口：「没进度」是 200 不是失败）。
//   - QuestionTagsResultDTO.tag_ids ⇒ 该键是**请求体回显**（api 侧
//     `service.QuestionTagsResultDTO{TagIDs: req.TagIDs}`），客户端发 `"tag_ids": null`
//     或不发该键就拿 `null`。它不是「服务算出来恒非 null」那类字段，改判会把入参形状
//     谎报成出参承诺。
//
// 本段**没跑**因而也没改判的三组，原因各不相同（都不是「嫌麻烦」）：
//   - ContactPlainDTO.photos / .resume_certifications、QuestionCommentPageResult.items、
//     api.AuditLogPageResult.items ⇒ 宿主文件由另一条在飞的分支持有（contact_service.go、
//     question_interaction_service.go、internal/api/），本段不改。前两格另有独立理由：
//     service.JSONArray.MarshalJSON 在 `j == nil` 时**字面发出 `null`**，本就恒可空。
//   - repository.AlgorithmParameters 的 4 格与 repository.SeriesConfigOptions 的 3 格 ⇒ 两道
//     硬阻塞，任一都足以让它留在原地：
//     (1) 判据 5 的证据源只有 `../service` 与 `../api` 两个目录（见 nullability_lock_test.go
//     的 nonNilEvidenceSources），repository 包**不在扫描面内** ⇒ 在
//     internal/valuation/repository/ 下建一张 nonnilOutlets* 表，锁一条也读不到，
//     改判后判据 5 直接判红；
//     (2) 这些出口要真 Postgres：DictionaryRepository 只持 `*pgxpool.Pool`（无 gorm、无
//     sqlite 构造口），listCached 走 pgx.Rows ⇒ testutil.NewMemoryDB 那套内存库进不去，
//     行为例就成了「只在配了 DATABASE_URL 的机器上才跑」——判据 5 要的是一次真跑过。
//     ⇒ 正解是给 nonNilEvidenceSources 加一个 repository 来源 + 给该包一条可跑 PG 的 seam，
//     那是独立一件工具改动，不塞进本段。
package service

import (
	"testing"

	"go.uber.org/zap"

	"forklift-training/internal/model"

	"forklift-training/internal/testutil"
)

var nonnilOutletsCatalog = map[string]func(t *testing.T) any{
	"service.CatalogTreeDTO.specialties":                       outletCatalogTreeEmpty,
	"service.CatalogSpecialtyNode.levels":                      outletCatalogSpecialtyNoLevels,
	"service.LevelListDTO.levels":                              outletLevelListEmpty,
	"service.SpecialtyListDTO.specialties":                     outletSpecialtyListEmpty,
	"service.QuestionTagListDTO.tags":                          outletQuestionTagListEmpty,
	"service.CertificateTemplateListDTO.certificate_templates": outletCertificateTemplateListEmpty,
	"service.CredentialListDTO.credentials":                    outletCredentialListEmpty,

	"service.FaqResult.categories":                outletFaqPublishedEmpty,
	"service.FaqCategoryDTO.entries":              outletFaqPublishedEmptyCategory,
	"service.AdminFaqCategoriesResult.categories": outletAdminFaqCategoriesEmpty,
	"service.AdminFaqEntriesResult.entries":       outletAdminFaqEntriesEmpty,

	"service.ChapterDTO.files":                  outletTutorChapterNoFiles,
	"service.ChapterDetailDTO.files":            outletChapterDetailNoFiles,
	"service.CourseDTO.chapters":                outletAdminCatalogCourseNode,
	"service.CourseDTO.prerequisites":           outletAdminCourseDetailMeta,
	"service.CourseDTO.prerequisite_course_ids": outletAdminCourseDetailMeta,

	"service.FeaturedContentPageResult.items":  outletFeaturedPageEmpty,
	"service.FeaturedContentDetailDTO.related": outletFeaturedDetailNoRelated,

	"service.BatchDeleteFilesResult.failed_ids": outletBatchDeleteFilesEmpty,

	"service.HistoryResultDTO.records":         outletPracticeHistoryEmpty,
	"service.PracticeStartResultDTO.questions": outletPracticeStartSequential,
	"service.MockExamHistoryDTO.exams":         outletMockExamHistoryEmpty,
	"service.MockExamStartDTO.questions":       outletMockExamStart,
	"service.PracticeStatsDTO.by_type":         outletPracticeStatsEmpty,
	"service.QuestionBankStatsDTO.by_type":     outletQuestionBankStatsEmpty,
	"service.QuestionBankStatsDTO.by_status":   outletQuestionBankStatsEmpty,
	"service.WrongQuestionStatsDTO.by_type":    outletWrongQuestionStatsEmpty,

	"service.StudentCoursesDTO.courses":       outletStudentCoursesEmpty,
	"service.StudentCourseDetailDTO.chapters": outletStudentCourseDetailNoChapters,
	"service.StudyRecordPageResult.records":   outletStudyRecordsEmpty,
	"service.StudyDailyStatsDTO.labels":       outletStudyDailyStats,
	"service.StudyDailyStatsDTO.data":         outletStudyDailyStats,
}

func init() {
	nonnilOutletTables = append(nonnilOutletTables, nonnilOutletsCatalog)
}

// ===== 目录树两层 =====

// outletCatalogTreeEmpty 学员端目录树：空库时 specialties 由 make(0,0) 起手 ⇒ `[]`。
func outletCatalogTreeEmpty(t *testing.T) any {
	t.Helper()
	return NewTrainingCatalogService(testutil.NewMemoryDB(t), zap.NewNop()).GetCatalogTree(nil)
}

// outletCatalogSpecialtyNoLevels 方向节点的 levels：**不播等级**，让 make([]CatalogLevelNode,0,n)
// 以 0 容量落地。方向得存在（目录树按方向建节点，没方向就没有节点可举证这一格）。
func outletCatalogSpecialtyNoLevels(t *testing.T) any {
	t.Helper()
	svc, db := newCatalogSvc(t)
	spec := model.Specialty{Code: "nonnil-lv0", Name: "零等级方向", SortOrder: 1, Status: 1}
	if err := db.Create(&spec).Error; err != nil {
		t.Fatalf("播种方向失败: %v", err)
	}
	tree := svc.GetCatalogTree(nil)
	if len(tree.Specialties) == 0 {
		t.Fatal("目录树里没有方向节点：这条证据没有落地")
	}
	return tree.Specialties[0]
}

// ===== 五个字典列表信封 =====
//
// 这些响应此前是 handler 里 `response.Success(c, service.XxxListDTO{...})` 一字段包出来的，
// 包体就在 internal/api/（本段不碰）。这里按 handler 那一行**逐字**复现包装，被包的切片
// 仍取自同一条服务方法（catalogList 的 make([]D,0,n)），所以举的是同一个事实。

func outletLevelListEmpty(t *testing.T) any {
	t.Helper()
	return LevelListDTO{Levels: NewTrainingCatalogService(testutil.NewMemoryDB(t), zap.NewNop()).ListLevels(false)}
}

func outletSpecialtyListEmpty(t *testing.T) any {
	t.Helper()
	return SpecialtyListDTO{Specialties: NewTrainingCatalogService(testutil.NewMemoryDB(t), zap.NewNop()).ListSpecialties(false)}
}

func outletQuestionTagListEmpty(t *testing.T) any {
	t.Helper()
	svc := NewTrainingCatalogService(testutil.NewMemoryDB(t), zap.NewNop())
	tags, err := svc.ListQuestionTags(false, true, nil)
	if err != nil {
		t.Fatalf("空标签列表失败: %v", err)
	}
	return QuestionTagListDTO{Tags: tags}
}

func outletCertificateTemplateListEmpty(t *testing.T) any {
	t.Helper()
	svc := NewTrainingCatalogService(testutil.NewMemoryDB(t), zap.NewNop())
	return CertificateTemplateListDTO{CertificateTemplates: svc.ListCertificateTemplates(false)}
}

func outletCredentialListEmpty(t *testing.T) any {
	t.Helper()
	svc := NewTrainingCatalogService(testutil.NewMemoryDB(t), zap.NewNop())
	return CredentialListDTO{Credentials: svc.ListCredentials(false)}
}

// ===== FAQ 四个信封 =====

func outletFaqPublishedEmpty(t *testing.T) any {
	t.Helper()
	res, err := NewFaqService(testutil.NewMemoryDB(t), zap.NewNop()).ListPublished()
	if err != nil {
		t.Fatalf("学员端帮助中心失败: %v", err)
	}
	return res
}

// outletFaqPublishedEmptyCategory entries 的形状要单独播一条**启用分类且其下零条已发布条目**
// ——ListPublished 对这种分类显式回填 `[]FaqEntryDTO{}`（源码注释：前端免判空）。
// 空库取不到分类节点，也就取不到这一格。
func outletFaqPublishedEmptyCategory(t *testing.T) any {
	t.Helper()
	db := testutil.NewMemoryDB(t)
	cat := model.FaqCategory{Code: "nonnil-faq", Title: "空分类", SortOrder: 1, Enabled: true}
	if err := db.Create(&cat).Error; err != nil {
		t.Fatalf("播种 FAQ 分类失败: %v", err)
	}
	res, err := NewFaqService(db, zap.NewNop()).ListPublished()
	if err != nil {
		t.Fatalf("学员端帮助中心失败: %v", err)
	}
	if len(res.Categories) == 0 {
		t.Fatal("帮助中心里没有分类节点：这条证据没有落地")
	}
	return res.Categories[0]
}

func outletAdminFaqCategoriesEmpty(t *testing.T) any {
	t.Helper()
	items, err := NewFaqService(testutil.NewMemoryDB(t), zap.NewNop()).AdminListCategories()
	if err != nil {
		t.Fatalf("管理端分类清单失败: %v", err)
	}
	return AdminFaqCategoriesResult{Categories: items}
}

func outletAdminFaqEntriesEmpty(t *testing.T) any {
	t.Helper()
	items, err := NewFaqService(testutil.NewMemoryDB(t), zap.NewNop()).AdminListEntries(nil)
	if err != nil {
		t.Fatalf("管理端条目清单失败: %v", err)
	}
	return AdminFaqEntriesResult{Entries: items}
}

// ===== 章节与课程三格 =====

// outletTutorChapterNoFiles 导师端章节列表的 files：**挑那条既无 chapter_file 行、file_url 也是空**
// 的章节（夹具里第 2 条）——两条 legacy/表条目分支都不进，才落在 `fileList == nil ⇒ []` 那格。
func outletTutorChapterNoFiles(t *testing.T) any {
	t.Helper()
	db := testutil.NewMemoryDB(t)
	_, chapters := seedChapterWithMeta(t, db)
	res, err := newTutorServiceForTest(t, db).GetCourseChapters(chapters[0].CourseID)
	if err != nil {
		t.Fatalf("导师端章节列表失败: %v", err)
	}
	if len(res.Chapters) < 2 {
		t.Fatalf("章节列表不足 2 条，取不到无文件的那条: %d", len(res.Chapters))
	}
	return res.Chapters[1]
}

func outletChapterDetailNoFiles(t *testing.T) any {
	t.Helper()
	db := testutil.NewMemoryDB(t)
	_, chapters := seedChapterWithMeta(t, db)
	res, err := newTutorServiceForTest(t, db).GetChapterDetail(chapters[1].ChapterID)
	if err != nil {
		t.Fatalf("章节详情失败: %v", err)
	}
	return res
}

// outletAdminCatalogCourseNode CourseDTO.chapters 是 `*[]ChapterDTO,omitempty`：
// 键缺席（未填充路径）与值为 null 是两件事，前者由 extensions:"x-optional" 表达、不改判，
// 这里要证的是**填充路径**——管理端目录树是仓里唯一走 withChapters=true 的出口，
// 它对「有章节」与「无章节」两种课程都显式赋一个非 nil 指针（后者赋 []ChapterDTO{}）。
func outletAdminCatalogCourseNode(t *testing.T) any {
	t.Helper()
	db := testutil.NewMemoryDB(t)
	seedVisibleCourse(t, db)
	tree := NewTrainingCatalogService(db, zap.NewNop()).GetAdminCatalogTree()
	if len(tree.Specialties) == 0 || len(tree.Specialties[0].Levels) == 0 ||
		len(tree.Specialties[0].Levels[0].Courses) == 0 {
		t.Fatal("管理端目录树里没有课程节点：这条证据没有落地")
	}
	return tree.Specialties[0].Levels[0].Courses[0]
}

// outletAdminCourseDetailMeta 一次调用同时举证 prerequisites 与 prerequisite_course_ids：
// fillCourseMeta 两者都以 `make(0,n)` 起手再取地址，于是 omitempty 只省掉「没调本函数」的读路径，
// 调过的路径恒发数组（`[]` 也算发过）。
func outletAdminCourseDetailMeta(t *testing.T) any {
	t.Helper()
	db := testutil.NewMemoryDB(t)
	res, err := NewAdminCourseService(db, nil, zap.NewNop()).GetCourseDetail(seedVisibleCourse(t, db))
	if err != nil {
		t.Fatalf("管理端课程详情失败: %v", err)
	}
	return res
}

// ===== 精选两格 =====

func outletFeaturedPageEmpty(t *testing.T) any {
	t.Helper()
	svc, _ := newFeaturedTestSvc(t)
	res, err := svc.GetPublicList(1, 20, "")
	if err != nil {
		t.Fatalf("精选公开列表失败: %v", err)
	}
	return res
}

// outletFeaturedDetailNoRelated related 的初值是 []FeaturedContentDTO{}，随后被
// `make(0,len(related))` 整格覆盖 ⇒ 同分类没有第二篇时发 `[]`（不是保留初值，故播一篇就够）。
func outletFeaturedDetailNoRelated(t *testing.T) any {
	t.Helper()
	svc, db := newFeaturedTestSvc(t)
	id := seedPublishedFeatured(t, db, "相关资讯为空", 0)
	res, err := svc.GetPublicDetail(id, false)
	if err != nil {
		t.Fatalf("精选公开详情失败: %v", err)
	}
	return res
}

// ===== 批量删文件 =====

func outletBatchDeleteFilesEmpty(t *testing.T) any {
	t.Helper()
	svc := newTutorServiceForTest(t, testutil.NewMemoryDB(t))
	return svc.BatchDeleteChapterFiles(nil)
}

// ===== 练习 / 模考 / 统计 =====

func outletPracticeHistoryEmpty(t *testing.T) any {
	t.Helper()
	svc, db := newPracticeSvc(t)
	res, err := svc.GetHistory(testutil.SeedStudent(t, db, "练习历史学员", "x").ID, nil, 1, 20, "", "", "")
	if err != nil {
		t.Fatalf("空练习历史失败: %v", err)
	}
	return res
}

// outletPracticeStartSequential 顺序练习开考：池里 0 题直接走 error，所以这条出口最少发 1 题。
// 恒非 null 的判据不靠「凑得出空集」——questions 由 make(0,len(questions)) 起手，
// 而那段长度在 `len(questions)==0 ⇒ error` 守卫之后，两件事各自成立。
func outletPracticeStartSequential(t *testing.T) any {
	t.Helper()
	svc, db := newPracticeSvc(t)
	student := testutil.SeedStudent(t, db, "顺序练习学员", "x")
	testutil.SeedQuestion(t, db, "single", "空池守卫前的第一题", "A")
	res, err := svc.StartSequential(student.ID, nil)
	if err != nil {
		t.Fatalf("顺序练习开考失败: %v", err)
	}
	return res
}

func outletMockExamHistoryEmpty(t *testing.T) any {
	t.Helper()
	db := testutil.NewMemoryDB(t)
	student := testutil.SeedStudent(t, db, "模考历史学员", "x")
	res, err := NewMockExamService(db, nil, zap.NewNop()).GetHistory(student.ID, nil, 1, 20)
	if err != nil {
		t.Fatalf("空模考历史失败: %v", err)
	}
	return res
}

func outletMockExamStart(t *testing.T) any {
	t.Helper()
	db := testutil.NewMemoryDB(t)
	student := testutil.SeedStudent(t, db, "模考开科学员", "x")
	testutil.SeedQuestion(t, db, "single", "模考抽题源", "A")
	res, err := NewMockExamService(db, nil, zap.NewNop()).Start(student.ID, 1, 90, nil)
	if err != nil {
		t.Fatalf("模拟考试开考失败: %v", err)
	}
	return res
}

func outletPracticeStatsEmpty(t *testing.T) any {
	t.Helper()
	svc, db := newPracticeSvc(t)
	student := testutil.SeedStudent(t, db, "练习统计学员", "x")
	res, err := svc.GetStats(student.ID, nil)
	if err != nil {
		t.Fatalf("练习统计失败: %v", err)
	}
	return res
}

// outletQuestionBankStatsEmpty 一次调用举证 by_type 与 by_status 两格：两个 map 都取自
// groupByCount 的 make(map,...) 再按合法维度零填充 ⇒ 恒 `{}` 起、只会更满。
func outletQuestionBankStatsEmpty(t *testing.T) any {
	t.Helper()
	return NewQuestionBankService(testutil.NewMemoryDB(t), nil, zap.NewNop()).GetStats(nil)
}

func outletWrongQuestionStatsEmpty(t *testing.T) any {
	t.Helper()
	db := testutil.NewMemoryDB(t)
	student := testutil.SeedStudent(t, db, "错题统计学员", "x")
	return NewWrongQuestionService(db, nil, zap.NewNop()).GetStats(student.ID)
}

// ===== 学员侧四格 =====

func outletStudentCoursesEmpty(t *testing.T) any {
	t.Helper()
	db := testutil.NewMemoryDB(t)
	student := testutil.SeedStudent(t, db, "我的课程学员", "x")
	res, err := NewStudentService(db, zap.NewNop()).GetStudentCourses(student.ID)
	if err != nil {
		t.Fatalf("我的课程列表失败: %v", err)
	}
	return res
}

func outletStudentCourseDetailNoChapters(t *testing.T) any {
	t.Helper()
	db := testutil.NewMemoryDB(t)
	student := testutil.SeedStudent(t, db, "课程详情学员", "x")
	res, err := NewStudentService(db, zap.NewNop()).GetStudentCourseDetail(student.ID, seedVisibleCourse(t, db))
	if err != nil {
		t.Fatalf("单课程学习详情失败: %v", err)
	}
	return res
}

func outletStudyRecordsEmpty(t *testing.T) any {
	t.Helper()
	db := testutil.NewMemoryDB(t)
	student := testutil.SeedStudent(t, db, "学习记录学员", "x")
	res, err := NewStudentService(db, zap.NewNop()).GetRecords(student.ID, 1, 20, "", "")
	if err != nil {
		t.Fatalf("学习记录分页失败: %v", err)
	}
	return res
}

// outletStudyDailyStats 按天学习统计：BuildDailySeries 恒补齐 7 或 30 格 ⇒ labels 与 data
// 都是**非空**数组（`[]` 这一形状在这条线上根本发不出，判据要的是「不是 null」）。
func outletStudyDailyStats(t *testing.T) any {
	t.Helper()
	db := testutil.NewMemoryDB(t)
	student := testutil.SeedStudent(t, db, "日统计学员", "x")
	return NewStudentService(db, zap.NewNop()).GetStudyStats(student.ID, 7)
}
