// 课程 / 目录域的 nonnil 行为例（批①-A）。
//
// 为什么单独成文件：判据 5 的证据要跟着出口走，一个域一张表比一张巨型表更好读，
// 也让并行推进的改判批次不在同一个文件里互相覆盖（汇总表机制见 nonnil_declaration_test.go）。
package service

import (
	"testing"

	"go.uber.org/zap"

	"forklift-training/internal/model"
	"forklift-training/internal/testutil"
)

var nonnilOutletsCourse = map[string]func(t *testing.T) any{
	"service.CoursePageResult.courses":      outletCoursePageNoCourses,
	"service.AdminCourseDetailDTO.chapters": outletAdminCourseDetailNoChapters,
	"service.CatalogLevelNode.courses":      outletCatalogLevelNodeNoCourses,
}

func init() {
	nonnilOutletTables = append(nonnilOutletTables, nonnilOutletsCourse)
}

// outletCoursePageNoCourses 空库拉学员端课程列表：无一行课程时 courses 发 `[]`。
func outletCoursePageNoCourses(t *testing.T) any {
	t.Helper()
	svc := NewCourseService(testutil.NewMemoryDB(t), nil, zap.NewNop())
	res, err := svc.GetCourses(1, 20, nil, nil, nil, "")
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
	svc := NewAdminCourseService(db, nil, zap.NewNop())
	res, err := svc.GetCourseDetail(seedVisibleCourse(t, db))
	if err != nil {
		t.Fatalf("管理端课程详情失败: %v", err)
	}
	return res
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
