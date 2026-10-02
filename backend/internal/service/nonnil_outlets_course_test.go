// 课程 / 目录域的 nonnil 行为例（批①-A）；域包拆出去之后（ADR-0070）本文件只留
// **真实出口在 service** 的那几格。
//
// 为什么单独成文件：判据 5 的证据要跟着出口走，一个域一张表比一张巨型表更好读，
// 也让并行推进的改判批次不在同一个文件里互相覆盖（汇总表机制见 nonnil_declaration_test.go）。
//
// 课程域自己的表在 internal/course/nonnil_outlets_test.go（course.CoursePageResult.courses /
// course.AdminCourseDetailDTO.chapters / course.CourseDTO.prerequisites /
// course.CourseDTO.prerequisite_course_ids / course.CourseDetailDTO.chapters /
// course.ChapterDetailDTO.files）——那几格的生产者都在课程域包里。留在这里的三格生产者在
// **service**：导师端章节列表（TutorService.GetCourseChapters）、管理端目录树的课程节点
// （TrainingCatalogService.GetAdminCatalogTree 的 withChapters 分支）、目录树的等级节点。
// 证据跟真实出口走，不跟类型名的前缀走。
package service

import (
	"testing"

	"go.uber.org/zap"

	"forklift-training/internal/model"
	"forklift-training/internal/testutil"
)

var nonnilOutletsCourse = map[string]func(t *testing.T) any{
	"course.ChapterDTO.files":          outletTutorChapterNoFiles,
	"course.CourseDTO.chapters":        outletAdminCatalogCourseNode,
	"service.CatalogLevelNode.courses": outletCatalogLevelNodeNoCourses,
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

// outletCatalogLevelNodeNoCourses 目录树的等级节点：courses 每格都以 make([]CourseDTO, 0) 起手，
// 门数只增不减 ⇒ 恒 `[]`。等级节点只在「有启用专业方向」时才被构造，所以要播方向与等级各一条，
// 并且**不挂课程**——挂了取到的就是非空列表，证的是另一件事。
func outletCatalogLevelNodeNoCourses(t *testing.T) any {
	t.Helper()
	svc, db := newCatalogSvc(t)
	spec := model.Specialty{Code: "nonnil-cat", Name: "空课程目录方向", SortOrder: 1, Status: 1}
	if err := db.Create(&spec).Error; err != nil {
		t.Fatalf("播种方向失败: %v", err)
	}
	lv := model.CourseLevel{Code: "nonnil-cat-lv", Name: "空课程目录等级", SortOrder: 1, Status: 1}
	if err := db.Create(&lv).Error; err != nil {
		t.Fatalf("播种等级失败: %v", err)
	}
	tree := svc.GetCatalogTree(nil)
	if len(tree.Specialties) == 0 || len(tree.Specialties[0].Levels) == 0 {
		t.Fatalf("目录树里没有等级节点：出口取不到 CatalogLevelNode，这条证据没有落地")
	}
	return tree.Specialties[0].Levels[0]
}
