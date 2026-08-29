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
// Filter：hot|featured|all（缺省 all），映射 is_hot / is_featured 双 bool（可叠加，见 Spec #326 Q9-Q12）。
type CourseListOptions struct {
	OnlyMounted      bool
	Keyword          string
	SpecialtyID      *int
	LevelID          *int
	CredentialID     *int
	Filter           string
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
		if opts.CredentialID != nil {
			q = q.Where("credential_id = ?", *opts.CredentialID)
		}
		if opts.Filter == "hot" {
			q = q.Where("is_hot = ?", true)
		} else if opts.Filter == "featured" {
			q = q.Where("is_featured = ?", true)
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

// 批量回填已收敛到 batch_backfill.go（批量回填 module，课程列表/学员档案/学习记录共享）。
