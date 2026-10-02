// 培训域的 nonnil 行为例（ADR-0065 决策 5 判据 5）：跑真实出口、看发出的是 [] 还是 null。
//
// 为什么证据跟着域包走（ADR-0070）：判据 5 的表是「哪些域声明 nonnil、谁举证」的对应关系，
// 域包拆出去之后域的实现与它的举证住在同一个包里，改名/搬目录不会让两侧各自漂。
// 断言本体只有一份：testutil.AssertNonNilOutlets。
//
// 这批键是波 3b-2 从 internal/service 的三张表搬过来的（catalog 七格、people 的 positions 一格、
// stats 的证件分组两格），判据是「生产者是谁」——这些出口的服务方法本体就在本包。
// 另有两格键名前缀不归本域、但出口在本包：course.CourseDTO.chapters（管理端目录树的课程节点，
// 波 3b-2 从 internal/service 搬来）与 training.CatalogLevelNode.courses（学员端目录树的等级节点）。
// 证据跟真实出口走、不跟类型名的前缀走；course.ChapterDTO.files 的生产者是导师域的服务
// （TutorService.GetCourseChapters），波 4d 起该出口与它的证据都在 internal/tutor/nonnil_outlets_test.go。
package training

import (
	"testing"

	"go.uber.org/zap"
	"gorm.io/gorm"

	"forklift-training/internal/model"

	"forklift-training/internal/testutil"
)

// nonnilOutletsTraining 键 = 包名.类型名.json键，值 = 走真实出口取到的结果。
// 前缀 nonnilOutlets 是约定的证据表名：apitypes 的表态锁按它扫目录收键（见
// internal/apitypes/nullability_lock_test.go 的 nonNilEvidenceSources）。
var nonnilOutletsTraining = map[string]func(t *testing.T) any{
	"training.CatalogTreeDTO.specialties":                       outletCatalogTreeEmpty,
	"training.CatalogSpecialtyNode.levels":                      outletCatalogSpecialtyNoLevels,
	"training.LevelListDTO.levels":                              outletLevelListEmpty,
	"training.SpecialtyListDTO.specialties":                     outletSpecialtyListEmpty,
	"training.QuestionTagListDTO.tags":                          outletQuestionTagListEmpty,
	"training.CertificateTemplateListDTO.certificate_templates": outletCertificateTemplateListEmpty,
	"training.CredentialListDTO.credentials":                    outletCredentialListEmpty,
	"training.PositionListDTO.positions":                        outletPositionListEmpty,
	"training.GroupedCredentialsDTO.skill_level":                outletGroupedCredentialsNone,
	"training.GroupedCredentialsDTO.special_operation":          outletGroupedCredentialsNone,
	"training.CatalogLevelNode.courses":                         outletCatalogLevelNodeNoCourses,
	"course.CourseDTO.chapters":                                 outletAdminCatalogCourseNode,
}

// TestNonNilDeclaredOutletsNeverEmitNull 本域的举证入口（判据本体在 testutil）。
func TestNonNilDeclaredOutletsNeverEmitNull(t *testing.T) {
	testutil.AssertNonNilOutlets(t, []map[string]func(t *testing.T) any{nonnilOutletsTraining})
}

// ===== 目录树两层 =====

// outletCatalogTreeEmpty 学员端目录树：空库时 specialties 由 make(0,0) 起手 ⇒ [ ]。
func outletCatalogTreeEmpty(t *testing.T) any {
	t.Helper()
	return NewService(testutil.NewMemoryDB(t), zap.NewNop()).GetCatalogTree(nil)
}

// outletCatalogSpecialtyNoLevels 方向节点的 levels：**不播等级**，让 make([]CatalogLevelNode,0,n)
// 以 0 容量落地。方向得存在（目录树按方向建节点，没方向就没有节点可举证这一格）。
func outletCatalogSpecialtyNoLevels(t *testing.T) any {
	t.Helper()
	svc, db := newCatalogSvc(t)
	spec := model.Specialty{Code: "nonnil-lv0", Name: "零等级方向", SortOrder: 1, Status: 1}
	if err := db.Create(&spec).Error; err != nil {
		t.Fatalf("播种方向失败: %v", err)
	}
	tree := svc.GetCatalogTree(nil)
	if len(tree.Specialties) == 0 {
		t.Fatal("目录树里没有方向节点：这条证据没有落地")
	}
	return tree.Specialties[0]
}

// ===== 五个字典列表信封 =====
//
// 这些响应此前是 handler 里 response.Success(c, training.XxxListDTO{...}) 一字段包出来的，
// 包体就在本包 handler 里（原 internal/api/training_catalog.go，波 3b-2 三分进 handler*.go）。
// 这里按 handler 那一行**逐字**复现包装，被包的切片仍取自同一条服务方法
// （catalogList 的 make([]D,0,n)），所以举的是同一个事实。

func outletLevelListEmpty(t *testing.T) any {
	t.Helper()
	return LevelListDTO{Levels: NewService(testutil.NewMemoryDB(t), zap.NewNop()).ListLevels(false)}
}

func outletSpecialtyListEmpty(t *testing.T) any {
	t.Helper()
	return SpecialtyListDTO{Specialties: NewService(testutil.NewMemoryDB(t), zap.NewNop()).ListSpecialties(false)}
}

func outletQuestionTagListEmpty(t *testing.T) any {
	t.Helper()
	svc := NewService(testutil.NewMemoryDB(t), zap.NewNop())
	tags, err := svc.ListQuestionTags(false, true, nil)
	if err != nil {
		t.Fatalf("空标签列表失败: %v", err)
	}
	return QuestionTagListDTO{Tags: tags}
}

func outletCertificateTemplateListEmpty(t *testing.T) any {
	t.Helper()
	svc := NewService(testutil.NewMemoryDB(t), zap.NewNop())
	return CertificateTemplateListDTO{CertificateTemplates: svc.ListCertificateTemplates(false)}
}

func outletCredentialListEmpty(t *testing.T) any {
	t.Helper()
	svc := NewService(testutil.NewMemoryDB(t), zap.NewNop())
	return CredentialListDTO{Credentials: svc.ListCredentials(false)}
}

// ===== 岗位字典 =====

// outletPositionListEmpty 岗位字典：空库时 positions 是空集（catalogList 的 make(0,n)）。
func outletPositionListEmpty(t *testing.T) any {
	t.Helper()
	return PositionListDTO{Positions: NewService(testutil.NewMemoryDB(t), zap.NewNop()).ListPositions(false)}
}

// ===== 证件分组 =====

// outletGroupedCredentialsNone 证件分组：两组都以 [ ]CredentialDict{} 起手，空集也发 [ ]。
func outletGroupedCredentialsNone(t *testing.T) any {
	t.Helper()
	return NewService(testutil.NewMemoryDB(t), zap.NewNop()).ListGroupedCredentials()
}

// ===== 目录树的课程节点与等级节点（波 3b-2 从 internal/service 搬来）=====

// outletAdminCatalogCourseNode CourseDTO.chapters 是 `*[]ChapterDTO,omitempty`：
// 键缺席（未填充路径）与值为 null 是两件事，前者由 extensions:"x-optional" 表达、不改判，
// 这里要证的是**填充路径**——管理端目录树是仓里唯一走 withChapters=true 的出口，
// 它对「有章节」与「无章节」两种课程都显式赋一个非 nil 指针（后者赋 []ChapterDTO{}）。
func outletAdminCatalogCourseNode(t *testing.T) any {
	t.Helper()
	db := testutil.NewMemoryDB(t)
	seedVisibleCourse(t, db)
	tree := NewService(db, zap.NewNop()).GetAdminCatalogTree()
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

// seedVisibleCourse 播一门「已发布 + 已挂载」但**没有章节**的课程，返回其 id。
// （与 internal/course 的同名夹具各持一份：两包互不 import 对方的测试文件。）
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
