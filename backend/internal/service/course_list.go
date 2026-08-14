package service

import (
	"gorm.io/gorm"

	"forklift-training/internal/model"
	"forklift-training/pkg/paging"
	"forklift-training/pkg/response"
)

// ===== 课程列表 module：学员/管理/导师三路径共享实现 =====
//
// interface：CourseListOptions（列表差异点）+ ListCourses（唯一实现）。
// implementation 内部吸收挂载不变式、分页归一化与三类回填的批量查询（去 N+1）。

// CourseListOptions 列表差异点：学员端仅已挂载+上架并附证书名；
// 管理端全量可搜索；导师端与学员端同口径并附学习学员数（ADR-0012 §2 导师可见性口径）。
type CourseListOptions struct {
	OnlyMounted      bool
	Keyword          string
	SpecialtyID      *int
	LevelID          *int
	WithStudentCount bool
	DefaultPageSize  int
}

// ListCourses 课程列表共享实现（分页归一化、章节数/前置课程/学员数批量回填、信封组装只此一份）。
func ListCourses(db *gorm.DB, page, pageSize int, opts CourseListOptions) CoursePageResult {
	courses, total, page, pageSize := paging.Query[model.Course](db, page, pageSize, opts.DefaultPageSize, "sort_order ASC, created_at DESC, course_id DESC", func(q *gorm.DB) *gorm.DB {
		if opts.OnlyMounted {
			// 挂载不变式 + 上架：学员端/导师端可见性口径
			q = q.Where("status = ?", 1)
			q = mountedCourseScope(q)
		}
		if opts.Keyword != "" {
			q = q.Where("name LIKE ?", "%"+opts.Keyword+"%")
		}
		if opts.SpecialtyID != nil {
			q = q.Where("specialty_id = ?", *opts.SpecialtyID)
		}
		if opts.LevelID != nil {
			q = q.Where("level_id = ?", *opts.LevelID)
		}
		return q
	})

	ids := make([]int, 0, len(courses))
	for i := range courses {
		ids = append(ids, courses[i].CourseID)
	}

	// 证书模板数量少，一次加载映射（学员端/导师端路径附带证书名，避免逐课程 N+1）
	var certNameByID map[int]string
	if opts.OnlyMounted && len(courses) > 0 {
		var certs []model.CertificateTemplate
		db.Find(&certs)
		certNameByID = make(map[int]string, len(certs))
		for i := range certs {
			certNameByID[certs[i].ID] = certs[i].Name
		}
	}

	// 批量回填：章节数 / 前置课程 / 学习学员数各一次查询
	chapterCountByCourse := batchChapterCounts(db, ids)
	prereqByCourse := batchPrereqIDs(db, ids)
	var studentCountByCourse map[int]int64
	if opts.WithStudentCount && len(courses) > 0 {
		studentCountByCourse = batchStudentCounts(db, ids)
	}

	items := make([]CourseDTO, 0, len(courses))
	for i := range courses {
		item := courseToDTO(&courses[i])
		count := chapterCountByCourse[courses[i].CourseID]
		item.ChapterCount = &count
		ids := prereqByCourse[courses[i].CourseID]
		if ids == nil {
			ids = []int{}
		}
		item.PrerequisiteCourseIDs = &ids
		if opts.WithStudentCount {
			n := studentCountByCourse[courses[i].CourseID]
			item.StudentCount = &n
		}
		if opts.OnlyMounted {
			if id := courses[i].CertificateTemplateID; id != nil {
				item.CertificateName = certNameByID[*id]
			}
		}
		items = append(items, item)
	}
	return CoursePageResult{
		Courses: items,
		Page:    page,
		Pages:   response.PageCount(total, pageSize),
		Total:   total,
	}
}

// batchChapterCounts 一次查询全部课程章节数（缺省 0，消除逐课程 N+1）。
func batchChapterCounts(db *gorm.DB, courseIDs []int) map[int]int64 {
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

// batchPrereqIDs 一次查询全部课程前置课程 ID（缺省空切片，与旧 map 行为一致：[] 而非 null）。
func batchPrereqIDs(db *gorm.DB, courseIDs []int) map[int][]int {
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

// batchStudentCounts 一次查询全部课程学习学员数（study_record 去重 student_id）。
func batchStudentCounts(db *gorm.DB, courseIDs []int) map[int]int64 {
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
