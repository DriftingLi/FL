package course

import (
	"gorm.io/gorm"

	"forklift-training/internal/model"
)

// ===== 课程附属字段批量回填：随 ListCourses 一起从 internal/service/batch_backfill.go 搬来 =====
//
// P2 波 3b-1（计划外漏项，见 ADR-0070 回写）：这三枚原本住在 internal/service，被课程列表与
// 学员档案两条路径共用；课程域包化后服务侧仍要调它们，于是「助手搬回自己域」——定义归课程域，
// internal/core 反向引用（core → course 是本波 DAG 允许的单向边）。其余课程/章节名回填助手
// （batchCourseNames / batchChapterTitles / courseName / courseNameFound / UnknownCourseName）留在
// internal/core：它们服务的是学习记录与档案两条非课程域路径。

// BatchChapterCounts 一次查询全部课程章节数（缺省 0，消除逐课程 N+1）。
func BatchChapterCounts(db *gorm.DB, courseIDs []int) map[int]int64 {
	result := make(map[int]int64, len(courseIDs))
	for _, id := range courseIDs {
		result[id] = 0
	}
	if len(courseIDs) == 0 {
		return result
	}
	rows := make([]struct {
		CourseID int   `gorm:"column:course_id"`
		N        int64 `gorm:"column:n"`
	}, 0)
	db.Model(&model.Chapter{}).
		Select("course_id, COUNT(*) AS n").
		Where("course_id IN ?", courseIDs).
		Group("course_id").
		Scan(&rows)
	for _, r := range rows {
		result[r.CourseID] = r.N
	}
	return result
}

// BatchPrereqIDs 一次查询全部课程前置课程 ID（缺省空切片，与旧 map 行为一致：[] 而非 null）。
func BatchPrereqIDs(db *gorm.DB, courseIDs []int) map[int][]int {
	result := make(map[int][]int, len(courseIDs))
	if len(courseIDs) == 0 {
		return result
	}
	var rows []model.CoursePrerequisite
	db.Where("course_id IN ?", courseIDs).
		Order("course_id ASC, prerequisite_course_id ASC").
		Find(&rows)
	for _, r := range rows {
		result[r.CourseID] = append(result[r.CourseID], r.PrerequisiteCourseID)
	}
	return result
}

// BatchStudentCounts 一次查询全部课程学习学员数（study_record 去重 student_id）。
func BatchStudentCounts(db *gorm.DB, courseIDs []int) map[int]int64 {
	result := make(map[int]int64, len(courseIDs))
	for _, id := range courseIDs {
		result[id] = 0
	}
	if len(courseIDs) == 0 {
		return result
	}
	rows := make([]struct {
		CourseID int   `gorm:"column:course_id"`
		N        int64 `gorm:"column:n"`
	}, 0)
	db.Table("study_record").
		Select("course_id, COUNT(DISTINCT student_id) AS n").
		Where("course_id IN ?", courseIDs).
		Group("course_id").
		Scan(&rows)
	for _, r := range rows {
		result[r.CourseID] = r.N
	}
	return result
}
