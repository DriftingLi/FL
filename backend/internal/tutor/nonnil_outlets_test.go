// 讲师域的 nonnil 行为例（ADR-0065 决策 5 判据 5）：跑真实出口、看发出的是 [] 还是 null。
//
// 为什么证据跟着域包走（ADR-0070）：判据 5 的表是「哪些域声明 nonnil、谁举证」的对应关系，
// 域包拆出去之后域的实现与它的举证住在同一个包里，改名 / 搬目录不会让两侧各自漂。
// 断言本体只有一份：testutil.AssertNonNilOutlets。
//
// 三格是波 4d 从 internal/service 的两张表搬来的（生产者 GetCourseChapters / BatchDeleteChapterFiles
// 的方法体就在本包）。前两格的**类型**属课程域（course.*），生产者却是本包 —— 证据跟真实出口走，
// 不跟类型名的前缀走（判词原文随格搬来，见下）。第三格的类型随 tutor/dto.go 搬进本包，
// 键前缀随包名从 service 改成 tutor。
package tutor

import (
	"testing"

	"gorm.io/gorm"

	"forklift-training/internal/model"

	"forklift-training/internal/testutil"
)

// nonnilOutletsTutor 键 = 包名.类型名.json键，值 = 走真实出口取到的结果。
// 前缀 nonnilOutlets 是约定的证据表名：apitypes 的表态锁按它扫目录收键（见
// internal/apitypes/nullability_lock_test.go 的 nonNilEvidenceSources）。
var nonnilOutletsTutor = map[string]func(t *testing.T) any{
	"course.TutorCourseChaptersDTO.chapters":  outletTutorCourseChaptersEmpty,
	"course.ChapterDTO.files":                 outletTutorChapterNoFiles,
	"tutor.BatchDeleteFilesResult.failed_ids": outletBatchDeleteFilesEmpty,
}

// TestNonNilDeclaredOutletsNeverEmitNull 本域的举证入口（判据本体在 testutil）。
func TestNonNilDeclaredOutletsNeverEmitNull(t *testing.T) {
	testutil.AssertNonNilOutlets(t, []map[string]func(t *testing.T) any{nonnilOutletsTutor})
}

// outletTutorCourseChaptersEmpty 导师端章节列表：有课程、零章节时 chapters 是空集。
func outletTutorCourseChaptersEmpty(t *testing.T) any {
	t.Helper()
	db := testutil.NewMemoryDB(t)
	res, err := newTutorServiceForTest(t, db).GetCourseChapters(seedVisibleCourse(t, db))
	if err != nil {
		t.Fatalf("导师端章节列表失败: %v", err)
	}
	return res
}

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

// outletBatchDeleteFilesEmpty 批量删文件：传空 id 列表时 failed_ids 由 make 起手，仍是空集。
func outletBatchDeleteFilesEmpty(t *testing.T) any {
	t.Helper()
	svc := newTutorServiceForTest(t, testutil.NewMemoryDB(t))
	return svc.BatchDeleteChapterFiles(nil)
}

// seedVisibleCourse 播一门「已发布 + 已挂载」但**没有章节**的课程，返回其 id。
// （逐字副本：源 internal/service/nonnil_declaration_test.go，波 4d 随唯一消费它的出口搬来，
// 留驻侧那份已删 —— 域包测试不 import internal/core 的测试文件。）
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
