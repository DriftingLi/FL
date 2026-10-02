// 学员域的 nonnil 行为例（ADR-0065 决策 5 判据 5）：跑真实出口、看发出的是 [] 还是 null。
//
// 为什么证据跟着域包走（ADR-0070）：判据 5 的表是「哪些域声明 nonnil、谁举证」的对应关系，
// 域包拆出去之后域的实现与它的举证住在同一个包里，改名/搬目录不会让两侧各自漂。
// 断言本体只有一份：testutil.AssertNonNilOutlets。
//
// 这六格是波 4b 从 internal/service 的 nonnilOutletsCatalog / nonnilOutletsStats 两张表搬来的
// （生产者 GetProfile / GetRecords / GetStudyStats / GetStudentCourses / GetStudentCourseDetail
// 的方法体就在本包），键前缀随包名改成 student。
package student

import (
	"testing"

	"go.uber.org/zap"
	"gorm.io/gorm"

	"forklift-training/internal/model"
	"forklift-training/internal/testutil"
)

// nonnilOutletsStudent 键 = 包名.类型名.json键，值 = 走真实出口取到的结果。
// 前缀 nonnilOutlets 是约定的证据表名：apitypes 的表态锁按它扫目录收键（见
// internal/apitypes/nullability_lock_test.go 的 nonNilEvidenceSources）。
var nonnilOutletsStudent = map[string]func(t *testing.T) any{
	"student.StudentCoursesDTO.courses":         outletStudentCoursesEmpty,
	"student.StudentCourseDetailDTO.chapters":   outletStudentCourseDetailNoChapters,
	"student.StudyRecordPageResult.records":     outletStudyRecordsEmpty,
	"student.StudyDailyStatsDTO.labels":         outletStudyDailyStats,
	"student.StudyDailyStatsDTO.data":           outletStudyDailyStats,
	"student.StudentProfileDTO.course_progress": outletStudentProfileNoStudy,
}

// TestNonNilDeclaredOutletsNeverEmitNull 本域的举证入口（判据本体在 testutil）。
func TestNonNilDeclaredOutletsNeverEmitNull(t *testing.T) {
	testutil.AssertNonNilOutlets(t, []map[string]func(t *testing.T) any{nonnilOutletsStudent})
}

func outletStudentCoursesEmpty(t *testing.T) any {
	t.Helper()
	db := testutil.NewMemoryDB(t)
	student := testutil.SeedStudent(t, db, "我的课程学员", "x")
	res, err := NewService(db, zap.NewNop()).GetStudentCourses(student.ID)
	if err != nil {
		t.Fatalf("我的课程列表失败: %v", err)
	}
	return res
}

func outletStudentCourseDetailNoChapters(t *testing.T) any {
	t.Helper()
	db := testutil.NewMemoryDB(t)
	student := testutil.SeedStudent(t, db, "课程详情学员", "x")
	res, err := NewService(db, zap.NewNop()).GetStudentCourseDetail(student.ID, seedVisibleCourse(t, db))
	if err != nil {
		t.Fatalf("单课程学习详情失败: %v", err)
	}
	return res
}

func outletStudyRecordsEmpty(t *testing.T) any {
	t.Helper()
	db := testutil.NewMemoryDB(t)
	student := testutil.SeedStudent(t, db, "学习记录学员", "x")
	res, err := NewService(db, zap.NewNop()).GetRecords(student.ID, 1, 20, "", "")
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
	return NewService(db, zap.NewNop()).GetStudyStats(student.ID, 7)
}

// outletStudentProfileNoStudy 学员档案：有账号、零学习记录时 course_progress 是空集。
func outletStudentProfileNoStudy(t *testing.T) any {
	t.Helper()
	db := testutil.NewMemoryDB(t)
	student := testutil.SeedStudent(t, db, "档案学员", "x")
	res, err := NewService(db, zap.NewNop()).GetProfile(student.ID)
	if err != nil {
		t.Fatalf("取档案失败: %v", err)
	}
	return res
}

// seedVisibleCourse 播一门「已发布 + 已挂载」但**没有章节**的课程，返回其 id
// （原 internal/service/nonnil_declaration_test.go 的同名助手域内副本，波 4b 随学员五格搬来）。
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
