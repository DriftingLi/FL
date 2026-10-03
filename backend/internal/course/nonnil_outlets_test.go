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
	"course.AdminCourseDetailDTO.chapters":     outletAdminCourseDetailNoChapters,
	"course.CourseDTO.prerequisites":           outletAdminCourseDetailMeta,
	"course.CourseDTO.prerequisite_course_ids": outletAdminCourseDetailMeta,
	"course.CourseDetailDTO.chapters":          outletCourseDetailNoChapters,
	"course.ChapterDetailDTO.files":            outletChapterDetailNoFiles,
}

// TestNonNilDeclaredOutletsNeverEmitNull 本域的举证入口（判据本体在 testutil）。
func TestNonNilDeclaredOutletsNeverEmitNull(t *testing.T) {
	testutil.AssertNonNilOutlets(t, []map[string]func(t *testing.T) any{nonnilOutletsCourse})
}

// outletCoursePageNoCourses 空库拉学员端课程列表：无一行课程时 courses 发 `[]`。
func outletCoursePageNoCourses(t *testing.T) any {
	t.Helper()
	res, err := NewService(testutil.NewMemoryDB(t), nil, zap.NewNop()).GetCourses(1, 20, nil, nil, nil, "")
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
	res, err := NewAdminService(db, nil, zap.NewNop()).GetCourseDetail(seedVisibleCourse(t, db))
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
	res, err := NewAdminService(db, nil, zap.NewNop()).GetCourseDetail(seedVisibleCourse(t, db))
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
	res, err := ChapterDetailShared(db, &chapters[1], false, 0)
	if err != nil {
		t.Fatalf("章节详情失败: %v", err)
	}
	return res
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
	course := model.Course{Name: "非空课程", Status: 1, SpecialtyID: &pos, LevelID: &lvl, CreatedAt: testutil.Now()}
	if err := db.Create(&course).Error; err != nil {
		t.Fatalf("播种课程失败: %v", err)
	}
	return course.CourseID
}
