// Ticket 1b（issue #54）：目录维护修正——新建项排同级末尾（sort_order=max+1）、
// 相邻交换真实生效（相等默认值也成立）、方向/等级/证书/标签编码必填。
// seam：service 层（sqlite 内存库）。
package service

import (
	"strings"
	"testing"

	"forklift-training/internal/model"
	"forklift-training/internal/testutil"
)

// TestCreateCatalogItemsAppendToEnd 新建方向/等级/标签自动排同级末尾（max+1），不再默认 0。
func TestCreateCatalogItemsAppendToEnd(t *testing.T) {
	db := testutil.NewMemoryDB(t)
	svc := NewTrainingCatalogService(db)

	// 专业方向
	s1, err := svc.CreateSpecialty(map[string]any{"code": "op", "name": "操作"})
	if err != nil {
		t.Fatalf("创建方向失败: %v", err)
	}
	s2, err := svc.CreateSpecialty(map[string]any{"code": "mt", "name": "维修"})
	if err != nil {
		t.Fatalf("创建方向失败: %v", err)
	}
	if s1["sort_order"] != 1 || s2["sort_order"] != 2 {
		t.Fatalf("方向 sort_order 应为 1,2, got %v,%v", s1["sort_order"], s2["sort_order"])
	}

	// 课程等级
	l1, err := svc.CreateLevel(map[string]any{"code": "bg", "name": "入门"})
	if err != nil {
		t.Fatalf("创建等级失败: %v", err)
	}
	l2, err := svc.CreateLevel(map[string]any{"code": "in", "name": "进阶"})
	if err != nil {
		t.Fatalf("创建等级失败: %v", err)
	}
	if l1["sort_order"] != 1 || l2["sort_order"] != 2 {
		t.Fatalf("等级 sort_order 应为 1,2, got %v,%v", l1["sort_order"], l2["sort_order"])
	}

	// 题库标签
	tag1, err := svc.CreateQuestionTag(map[string]any{"code": "hydraulic", "name": "液压", "category": "液压"})
	if err != nil {
		t.Fatalf("创建标签失败: %v", err)
	}
	tag2, err := svc.CreateQuestionTag(map[string]any{"code": "brake", "name": "制动", "category": "制动"})
	if err != nil {
		t.Fatalf("创建标签失败: %v", err)
	}
	if tag1["sort_order"] != 1 || tag2["sort_order"] != 2 {
		t.Fatalf("标签 sort_order 应为 1,2, got %v,%v", tag1["sort_order"], tag2["sort_order"])
	}
}

// TestCreateCourseAppendsToEndOfGroup 新建课程自动排所属方向+等级组的末尾。
func TestCreateCourseAppendsToEndOfGroup(t *testing.T) {
	db := testutil.NewMemoryDB(t)
	svc := NewAdminCourseService(db, nil)

	spec := model.Specialty{Code: "mt", Name: "维修", Status: 1, CreatedAt: testutil.Now()}
	if err := db.Create(&spec).Error; err != nil {
		t.Fatalf("创建方向失败: %v", err)
	}
	lv := model.CourseLevel{Code: "bg", Name: "入门", Status: 1, CreatedAt: testutil.Now()}
	if err := db.Create(&lv).Error; err != nil {
		t.Fatalf("创建等级失败: %v", err)
	}
	spec2 := model.Specialty{Code: "op", Name: "操作", Status: 1, CreatedAt: testutil.Now()}
	if err := db.Create(&spec2).Error; err != nil {
		t.Fatalf("创建方向失败: %v", err)
	}

	c1, err := svc.CreateCourse(map[string]any{"name": "课程1", "specialty_id": spec.SpecialtyID, "level_id": lv.LevelID})
	if err != nil {
		t.Fatalf("创建课程失败: %v", err)
	}
	c2, err := svc.CreateCourse(map[string]any{"name": "课程2", "specialty_id": spec.SpecialtyID, "level_id": lv.LevelID})
	if err != nil {
		t.Fatalf("创建课程失败: %v", err)
	}
	// 另一方向+等级组：从 1 重新开始
	c3, err := svc.CreateCourse(map[string]any{"name": "课程3", "specialty_id": spec2.SpecialtyID, "level_id": lv.LevelID})
	if err != nil {
		t.Fatalf("创建课程失败: %v", err)
	}
	if c1["sort_order"] != 1 || c2["sort_order"] != 2 {
		t.Fatalf("同组课程 sort_order 应为 1,2, got %v,%v", c1["sort_order"], c2["sort_order"])
	}
	if c3["sort_order"] != 1 {
		t.Fatalf("不同组课程 sort_order 应从 1 开始, got %v", c3["sort_order"])
	}
	// 显式传入 sort_order 时尊重传值
	c4, err := svc.CreateCourse(map[string]any{"name": "课程4", "specialty_id": spec.SpecialtyID, "level_id": lv.LevelID, "sort_order": 9})
	if err != nil {
		t.Fatalf("创建课程失败: %v", err)
	}
	if c4["sort_order"] != 9 {
		t.Fatalf("显式 sort_order 应为 9, got %v", c4["sort_order"])
	}
}

// TestSwapCatalogSortWithEqualValues 相邻交换在 sort_order 相同（默认 0）时也真实生效。
func TestSwapCatalogSortWithEqualValues(t *testing.T) {
	db := testutil.NewMemoryDB(t)
	svc := NewTrainingCatalogService(db)

	a := model.Specialty{Code: "a", Name: "A", SortOrder: 0, Status: 1, CreatedAt: testutil.Now()}
	b := model.Specialty{Code: "b", Name: "B", SortOrder: 0, Status: 1, CreatedAt: testutil.Now()}
	if err := db.Create(&a).Error; err != nil {
		t.Fatalf("创建方向失败: %v", err)
	}
	if err := db.Create(&b).Error; err != nil {
		t.Fatalf("创建方向失败: %v", err)
	}

	if err := svc.SwapSpecialtySort(a.SpecialtyID, b.SpecialtyID); err != nil {
		t.Fatalf("交换失败: %v", err)
	}
	var a2, b2 model.Specialty
	if err := db.First(&a2, a.SpecialtyID).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.First(&b2, b.SpecialtyID).Error; err != nil {
		t.Fatal(err)
	}
	if a2.SortOrder == b2.SortOrder {
		t.Fatalf("交换后两项 sort_order 必须不同, got %d,%d", a2.SortOrder, b2.SortOrder)
	}
	// 交换后 A 应排在 B 后面（B 的 sort_order 更小）
	list := svc.ListSpecialties(false)["specialties"].([]map[string]any)
	if list[0]["name"] != "B" || list[1]["name"] != "A" {
		t.Fatalf("交换后顺序应为 B,A, got %+v", list)
	}

	// 同值三连（0,0,0）：交换任意相邻两项后依然保持 distinct 且可继续交换
	c := model.Specialty{Code: "c", Name: "C", SortOrder: 0, Status: 1, CreatedAt: testutil.Now()}
	if err := db.Create(&c).Error; err != nil {
		t.Fatalf("创建方向失败: %v", err)
	}
	if err := svc.SwapSpecialtySort(b.SpecialtyID, c.SpecialtyID); err != nil {
		t.Fatalf("二次交换失败: %v", err)
	}
	var orders []int
	db.Model(&model.Specialty{}).Pluck("sort_order", &orders)
	seen := map[int]bool{}
	for _, o := range orders {
		if seen[o] {
			t.Fatalf("交换后出现重复 sort_order: %v", orders)
		}
		seen[o] = true
	}
}

// TestSwapCourseSortGroupBoundary 课程交换限制在同一个方向+等级组内。
func TestSwapCourseSortGroupBoundary(t *testing.T) {
	db := testutil.NewMemoryDB(t)
	svc := NewAdminCourseService(db, nil)

	spec := model.Specialty{Code: "mt", Name: "维修", Status: 1, CreatedAt: testutil.Now()}
	if err := db.Create(&spec).Error; err != nil {
		t.Fatal(err)
	}
	lv := model.CourseLevel{Code: "bg", Name: "入门", Status: 1, CreatedAt: testutil.Now()}
	if err := db.Create(&lv).Error; err != nil {
		t.Fatal(err)
	}
	c1 := model.Course{Name: "A", Status: 1, SpecialtyID: ptrInt(spec.SpecialtyID), LevelID: ptrInt(lv.LevelID), CreatedAt: testutil.Now()}
	c2 := model.Course{Name: "B", Status: 1, SpecialtyID: ptrInt(spec.SpecialtyID), LevelID: ptrInt(lv.LevelID), CreatedAt: testutil.Now()}
	if err := db.Create(&c1).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&c2).Error; err != nil {
		t.Fatal(err)
	}
	if err := svc.SwapCourseSort(c1.CourseID, c2.CourseID); err != nil {
		t.Fatalf("同组交换失败: %v", err)
	}

	// 不同等级组（同一方向）应报错
	lv2 := model.CourseLevel{Code: "in", Name: "进阶", Status: 1, CreatedAt: testutil.Now()}
	if err := db.Create(&lv2).Error; err != nil {
		t.Fatal(err)
	}
	c3 := model.Course{Name: "C", Status: 1, SpecialtyID: ptrInt(spec.SpecialtyID), LevelID: ptrInt(lv2.LevelID), CreatedAt: testutil.Now()}
	if err := db.Create(&c3).Error; err != nil {
		t.Fatal(err)
	}
	if err := svc.SwapCourseSort(c1.CourseID, c3.CourseID); err == nil {
		t.Fatal("跨等级组交换应报错")
	}
}

// TestCatalogCodeRequired 方向/等级/证书/标签创建时编码必填（与 UI 提示一致）。
func TestCatalogCodeRequired(t *testing.T) {
	db := testutil.NewMemoryDB(t)
	svc := NewTrainingCatalogService(db)

	cases := []struct {
		name string
		fn   func() error
		want string
	}{
		{"方向", func() error { _, err := svc.CreateSpecialty(map[string]any{"name": "X"}); return err }, "专业方向编码不能为空"},
		{"等级", func() error { _, err := svc.CreateLevel(map[string]any{"name": "X"}); return err }, "课程等级编码不能为空"},
		{"证书", func() error { _, err := svc.CreateCertificateTemplate(map[string]any{"name": "X"}); return err }, "证书模板编码不能为空"},
		{"标签", func() error { _, err := svc.CreateQuestionTag(map[string]any{"name": "X", "category": "Y"}); return err }, "标签编码不能为空"},
	}
	for _, c := range cases {
		err := c.fn()
		if err == nil || !strings.Contains(err.Error(), c.want) {
			t.Fatalf("%s 编码必填校验失败, got %v", c.name, err)
		}
	}
}
