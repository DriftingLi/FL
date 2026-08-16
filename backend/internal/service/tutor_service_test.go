package service

import (
	"testing"

	"go.uber.org/zap"
	"gorm.io/gorm"

	"forklift-training/internal/model"
	"forklift-training/internal/testutil"
)

func newTutorServiceForTest(t *testing.T, db *gorm.DB) *TutorService {
	t.Helper()
	return NewTutorService(db, "", nil, nil, zap.NewNop())
}

// seedUnmountedCourse 建一门未挂方向/等级的已上架课程（导师端旧口径可见，新口径不可见）。
func seedUnmountedCourse(t *testing.T, db *gorm.DB) model.Course {
	t.Helper()
	c := model.Course{Name: "未挂载课程", Status: 1, CreatedAt: testutil.Now()}
	if err := db.Create(&c).Error; err != nil {
		t.Fatalf("创建未挂载课程失败: %v", err)
	}
	return c
}

// seedStudyRecords 为课程造学习记录（去重 student_id 计数用）。
func seedStudyRecords(t *testing.T, db *gorm.DB, courseID int, students int) {
	t.Helper()
	for i := 1; i <= students; i++ {
		r := model.StudyRecord{
			StudentID: i, CourseID: courseID, Progress: 0.5, StudyDate: testutil.Now(),
		}
		if err := db.Create(&r).Error; err != nil {
			t.Fatalf("创建学习记录失败: %v", err)
		}
	}
}

// TestTutorCourseListHidesUnmounted 锁定 ADR-0012 §2 行为变更：
// 导师端列表与学员端口径统一，未挂方向/等级的课程不可见。
func TestTutorCourseListHidesUnmounted(t *testing.T) {
	db := testutil.NewMemoryDB(t)
	svc := newTutorServiceForTest(t, db)
	_, _ = seedCatalogCourse(t, db)
	seedUnmountedCourse(t, db)

	list := svc.GetCourses(1, 10, nil, nil)
	if list.Total != 2 { // 主课程 + 前置课程（均挂载）；未挂载课程不可见
		t.Fatalf("total = %d, want 2（未挂载课程应隐藏）", list.Total)
	}
	for _, c := range list.Courses {
		if c.Name == "未挂载课程" {
			t.Fatalf("未挂载课程不应出现在导师列表: %+v", c)
		}
	}
}

// TestTutorCourseListBatchFills 章节数/前置课程/学习学员数批量回填正确。
func TestTutorCourseListBatchFills(t *testing.T) {
	db := testutil.NewMemoryDB(t)
	svc := newTutorServiceForTest(t, db)
	course, _ := seedCatalogCourse(t, db)
	seedStudyRecords(t, db, course.CourseID, 3)

	list := svc.GetCourses(1, 10, nil, nil)
	for _, c := range list.Courses {
		if c.CourseID != course.CourseID {
			continue
		}
		if c.ChapterCount == nil || *c.ChapterCount != 2 {
			t.Fatalf("chapter_count = %v, want 2", c.ChapterCount)
		}
		if c.PrerequisiteCourseIDs == nil || len(*c.PrerequisiteCourseIDs) != 1 {
			t.Fatalf("prerequisite_course_ids = %v, want 1 条", c.PrerequisiteCourseIDs)
		}
		if c.StudentCount == nil || *c.StudentCount != 3 {
			t.Fatalf("student_count = %v, want 3", c.StudentCount)
		}
		return
	}
	t.Fatalf("未在列表中找到主课程")
}

// TestTutorCourseListNoNPlusOne 列表回填批量查询：多课程时查询数为常数级（无 N+1）。
func TestTutorCourseListNoNPlusOne(t *testing.T) {
	db := testutil.NewMemoryDB(t)
	svc := newTutorServiceForTest(t, db)

	// 3 门挂载课程 + 学习记录
	spec := model.Specialty{Code: "maintenance", Name: "维修", SortOrder: 1, Status: 1}
	if err := db.Create(&spec).Error; err != nil {
		t.Fatalf("创建方向失败: %v", err)
	}
	lv := model.CourseLevel{Code: "beginner", Name: "入门", SortOrder: 1, Status: 1}
	if err := db.Create(&lv).Error; err != nil {
		t.Fatalf("创建等级失败: %v", err)
	}
	for i := 1; i <= 3; i++ {
		c := model.Course{Name: "课程", Status: 1,
			SpecialtyID: &spec.SpecialtyID, LevelID: &lv.LevelID, CreatedAt: testutil.Now()}
		if err := db.Create(&c).Error; err != nil {
			t.Fatalf("创建课程失败: %v", err)
		}
		seedStudyRecords(t, db, c.CourseID, 2)
	}

	var queryCount int
	cb := db.Callback().Query().Before("gorm:query")
	if err := cb.Register("t4:nplus1", func(*gorm.DB) { queryCount++ }); err != nil {
		t.Fatalf("注册查询计数回调失败: %v", err)
	}
	defer func() {
		_ = db.Callback().Query().Before("gorm:query").Remove("t4:nplus1")
	}()

	svc.GetCourses(1, 10, nil, nil)
	// 2（分页 count+find）+ 1（证书模板）+ 3（章节数/前置课程/学员数批量）= 6
	if queryCount > 6 {
		t.Fatalf("列表查询数为 %d，超过常数级上限 6（存在 N+1）", queryCount)
	}
}
