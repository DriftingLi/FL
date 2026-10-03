package student

import (
	"gorm.io/gorm"

	"forklift-training/internal/model"
)

// ===== 批量回填 module：课程/章节附属字段的批量加载（消除逐行 N+1） =====
//
// 课程列表（ListCourses）、学员档案曲线进度（queryProfile）、学习记录（GetRecords）
// 三路径共享同一组批量查询。所有回填缺省语义集中于此：
//   - 未知课程名 → UnknownCourseName（学习记录路径）；档案进度路径对未知课程跳过整行。
//   - 章节数缺省 0（与旧有 count=0 语义一致）；空结果回填空切片而非 null。
//
// 未来新增「按若干 course_id / chapter_id 回填附属字段」一律走本 module，禁止逐行 N+1。
//
// P2 波 3b-1：三个**课程附属**加载器（BatchChapterCounts / BatchPrereqIDs / BatchStudentCounts）
// 随课程列表一起搬进 internal/course/batch_backfill.go（课程域包定义，本包反向调用）。

// UnknownCourseName 未知课程的缺省回填文案（学习记录课程名缺省值，集中单点）。
const UnknownCourseName = "未知课程"

// batchCourseNames 一次查询全部课程名（仅返回存在的课程；缺省解析由
// courseName / courseNameFound 统一处理，避免调用方各自写差集）。
func batchCourseNames(db *gorm.DB, courseIDs []int) map[int]string {
	result := make(map[int]string, len(courseIDs))
	if len(courseIDs) == 0 {
		return result
	}
	var courses []model.Course
	db.Where("course_id IN ?", courseIDs).
		Select("course_id, name").
		Find(&courses)
	for _, c := range courses {
		result[c.CourseID] = c.Name
	}
	return result
}

// batchChapterTitles 一次查询全部章节标题（仅返回存在的章节；未匹配时调用方保持 nil）。
func batchChapterTitles(db *gorm.DB, chapterIDs []int) map[int]string {
	result := make(map[int]string, len(chapterIDs))
	if len(chapterIDs) == 0 {
		return result
	}
	var chapters []model.Chapter
	db.Where("chapter_id IN ?", chapterIDs).
		Select("chapter_id, title").
		Find(&chapters)
	for _, c := range chapters {
		result[c.ChapterID] = c.Title
	}
	return result
}

// courseName 返回课程名；未知课程回退 UnknownCourseName（学习记录等缺省文案路径）。
// 缺省语义集中单点，调用方不得自行书写「未知课程」字面量。
func courseName(names map[int]string, id int) string {
	if n, ok := names[id]; ok {
		return n
	}
	return UnknownCourseName
}

// courseNameFound 返回课程名与是否存在；档案进度等「未知课程跳过整行」路径用。
func courseNameFound(names map[int]string, id int) (string, bool) {
	n, ok := names[id]
	return n, ok
}
