// 课程域的 nonnil 行为例（批①-A）；域包拆出去之后（ADR-0070）本文件只留
// **真实出口在 service** 的那一格。
//
// 为什么单独成文件：判据 5 的证据要跟着出口走，一个域一张表比一张巨型表更好读，
// 也让并行推进的改判批次不在同一个文件里互相覆盖（汇总表机制见 nonnil_declaration_test.go）。
//
// 课程域自己的表在 internal/course/nonnil_outlets_test.go（course.CoursePageResult.courses /
// course.AdminCourseDetailDTO.chapters / course.CourseDTO.prerequisites /
// course.CourseDTO.prerequisite_course_ids / course.CourseDetailDTO.chapters /
// course.ChapterDetailDTO.files）；目录域两格（course.CourseDTO.chapters 的填充分支、
// training.CatalogLevelNode.courses）随波 3b-2 搬进 internal/training/nonnil_outlets_test.go
// ——那两格的生产者（管理端目录树 / 学员端目录树）现在都在培训域包里。
// 留在这里的一格生产者是 **service** 的导师端章节列表（TutorService.GetCourseChapters）。
// 证据跟真实出口走，不跟类型名的前缀走。
package service

import (
	"testing"

	"forklift-training/internal/testutil"
)

var nonnilOutletsCourse = map[string]func(t *testing.T) any{
	"course.ChapterDTO.files": outletTutorChapterNoFiles,
}

func init() {
	nonnilOutletTables = append(nonnilOutletTables, nonnilOutletsCourse)
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
