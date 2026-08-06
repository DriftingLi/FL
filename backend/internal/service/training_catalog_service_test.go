// Package service 培训目录（专业方向/等级/证书模板/题库标签/目录树）测试。
package service

import (
	"testing"
	"time"

	"gorm.io/gorm"

	"forklift-training/internal/model"
	"forklift-training/internal/testutil"
)

func newCatalogSvc(t *testing.T) (*TrainingCatalogService, *gorm.DB) {
	t.Helper()
	db := testutil.NewMemoryDB(t)
	return NewTrainingCatalogService(db), db
}

// --- 专业方向 ---

func TestSpecialtyCRUD(t *testing.T) {
	svc, _ := newCatalogSvc(t)

	// 创建
	result, err := svc.CreateSpecialty(map[string]any{"code": "operation", "name": "操作", "sort_order": 1})
	if err != nil {
		t.Fatalf("创建专业方向失败: %v", err)
	}
	specID := result["specialty_id"].(int)
	if result["name"] != "操作" || result["status"] != int16(1) {
		t.Fatalf("创建结果不匹配: %+v", result)
	}

	// 编码/名称为空校验
	if _, err := svc.CreateSpecialty(map[string]any{"code": "", "name": "x"}); err == nil {
		t.Fatal("编码为空应报错")
	}
	if _, err := svc.CreateSpecialty(map[string]any{"code": "x", "name": ""}); err == nil {
		t.Fatal("名称为空应报错")
	}

	// 更新
	updated, err := svc.UpdateSpecialty(specID, map[string]any{"name": "操作方向", "status": 0})
	if err != nil {
		t.Fatalf("更新失败: %v", err)
	}
	if updated["name"] != "操作方向" || updated["status"] != int16(0) {
		t.Fatalf("更新结果不匹配: %+v", updated)
	}

	// 列表：管理端含停用项，学员端仅启用项
	all := svc.ListSpecialties(false)
	if len(all["specialties"].([]map[string]any)) != 1 {
		t.Fatal("管理端应看到 1 条（含停用）")
	}
	active := svc.ListSpecialties(true)
	if len(active["specialties"].([]map[string]any)) != 0 {
		t.Fatal("学员端应看不到停用项")
	}

	// 删除 + 不存在
	if err := svc.DeleteSpecialty(specID); err != nil {
		t.Fatalf("删除失败: %v", err)
	}
	if err := svc.DeleteSpecialty(specID); err == nil {
		t.Fatal("重复删除应报错")
	}
	if _, err := svc.UpdateSpecialty(9999, map[string]any{"name": "x"}); err == nil {
		t.Fatal("更新不存在的专业方向应报错")
	}
}

// --- 课程等级 ---

func TestLevelCRUD(t *testing.T) {
	svc, _ := newCatalogSvc(t)
	result, err := svc.CreateLevel(map[string]any{"code": "beginner", "name": "入门", "sort_order": 1})
	if err != nil {
		t.Fatalf("创建等级失败: %v", err)
	}
	levelID := result["level_id"].(int)

	if _, err := svc.CreateLevel(map[string]any{"code": "", "name": "x"}); err == nil {
		t.Fatal("编码为空应报错")
	}
	updated, err := svc.UpdateLevel(levelID, map[string]any{"name": "初级"})
	if err != nil {
		t.Fatalf("更新失败: %v", err)
	}
	if updated["name"] != "初级" {
		t.Fatalf("更新结果不匹配: %+v", updated)
	}
	all := svc.ListLevels(false)
	if len(all["levels"].([]map[string]any)) != 1 {
		t.Fatal("应看到 1 条等级")
	}
	if err := svc.DeleteLevel(levelID); err != nil {
		t.Fatalf("删除失败: %v", err)
	}
	if err := svc.DeleteLevel(levelID); err == nil {
		t.Fatal("重复删除应报错")
	}
}

// --- 证书模板 ---

func TestCertificateTemplateCRUD(t *testing.T) {
	svc, _ := newCatalogSvc(t)
	result, err := svc.CreateCertificateTemplate(map[string]any{
		"code": "CERT_1", "name": "叉车培训证书", "validity_days": 1460,
	})
	if err != nil {
		t.Fatalf("创建模板失败: %v", err)
	}
	tplID := result["id"].(int)
	if result["validity_days"] != 1460 {
		t.Fatalf("有效期不匹配: %+v", result)
	}

	// 无效有效期
	if _, err := svc.CreateCertificateTemplate(map[string]any{"code": "C", "name": "x", "validity_days": 0}); err == nil {
		t.Fatal("有效期为 0 应报错")
	}
	if _, err := svc.UpdateCertificateTemplate(tplID, map[string]any{"validity_days": -5}); err == nil {
		t.Fatal("负有效期应报错")
	}

	// 默认有效期 365
	def, err := svc.CreateCertificateTemplate(map[string]any{"code": "CERT_2", "name": "默认模板"})
	if err != nil {
		t.Fatalf("创建默认模板失败: %v", err)
	}
	if def["validity_days"] != 365 {
		t.Fatalf("默认有效期应为 365, got %v", def["validity_days"])
	}

	updated, err := svc.UpdateCertificateTemplate(tplID, map[string]any{"validity_days": 730})
	if err != nil {
		t.Fatalf("更新失败: %v", err)
	}
	if updated["validity_days"] != 730 {
		t.Fatalf("更新后有效期不匹配: %+v", updated)
	}
	list := svc.ListCertificateTemplates(false)
	if len(list["certificate_templates"].([]map[string]any)) != 2 {
		t.Fatal("应看到 2 条模板")
	}
	if err := svc.DeleteCertificateTemplate(tplID); err != nil {
		t.Fatalf("删除失败: %v", err)
	}
}

// --- 题库标签与题目关联 ---

func TestQuestionTagCRUD(t *testing.T) {
	svc, _ := newCatalogSvc(t)
	result, err := svc.CreateQuestionTag(map[string]any{"code": "hydraulic", "name": "液压", "category": "液压"})
	if err != nil {
		t.Fatalf("创建标签失败: %v", err)
	}
	tagID := result["id"].(int)

	if _, err := svc.CreateQuestionTag(map[string]any{"code": "", "name": "x"}); err == nil {
		t.Fatal("编码为空应报错")
	}
	updated, err := svc.UpdateQuestionTag(tagID, map[string]any{"name": "液压系统"})
	if err != nil {
		t.Fatalf("更新失败: %v", err)
	}
	if updated["name"] != "液压系统" {
		t.Fatalf("更新结果不匹配: %+v", updated)
	}
	active := svc.ListQuestionTags(true)
	if len(active["tags"].([]map[string]any)) != 1 {
		t.Fatal("应看到 1 条标签")
	}
	if err := svc.DeleteQuestionTag(tagID); err != nil {
		t.Fatalf("删除失败: %v", err)
	}
}

// TestListQuestionTags_QuestionCount 标签列表 question_count：
// 学员端仅统计已发布题目，管理端统计全部题目。
func TestListQuestionTags_QuestionCount(t *testing.T) {
	svc, db := newCatalogSvc(t)
	tag, _ := svc.CreateQuestionTag(map[string]any{"code": "regulation", "name": "法规"})
	qsvc := NewQuestionBankService(db)

	// 1 道已发布 + 1 道草稿（未发布）
	published, err := qsvc.CreateQuestion(map[string]any{
		"type": "single_choice", "content": "已发布题", "options": []string{"A", "B"}, "answer": "A",
		"status": "published", "tag_ids": []int{tag["id"].(int)},
	}, nil, "tutor")
	if err != nil {
		t.Fatalf("创建已发布题目失败: %v", err)
	}
	_ = published
	draft, err := qsvc.CreateQuestion(map[string]any{
		"type": "true_false", "content": "草稿题", "answer": "true",
		"status": "draft", "tag_ids": []int{tag["id"].(int)},
	}, nil, "tutor")
	if err != nil {
		t.Fatalf("创建草稿题目失败: %v", err)
	}
	_ = draft
	// 另一个无题目标签
	empty, _ := svc.CreateQuestionTag(map[string]any{"code": "brake", "name": "制动"})

	studentTags := svc.ListQuestionTags(true)["tags"].([]map[string]any)
	byID := map[int]map[string]any{}
	for _, d := range studentTags {
		byID[d["id"].(int)] = d
	}
	if byID[tag["id"].(int)]["question_count"] != int64(1) {
		t.Fatalf("学员端应统计 1 道已发布题, got %v", byID[tag["id"].(int)]["question_count"])
	}
	if byID[empty["id"].(int)]["question_count"] != int64(0) {
		t.Fatalf("无题目标签应为 0, got %v", byID[empty["id"].(int)]["question_count"])
	}

	adminTags := svc.ListQuestionTags(false)["tags"].([]map[string]any)
	byID2 := map[int]map[string]any{}
	for _, d := range adminTags {
		byID2[d["id"].(int)] = d
	}
	if byID2[tag["id"].(int)]["question_count"] != int64(2) {
		t.Fatalf("管理端应统计全部 2 道题, got %v", byID2[tag["id"].(int)]["question_count"])
	}
}

func TestSetQuestionTags(t *testing.T) {
	svc, db := newCatalogSvc(t)
	q := testutil.SeedQuestion(t, db, "single_choice", "液压相关题目", "A")
	tag1, _ := svc.CreateQuestionTag(map[string]any{"code": "hydraulic", "name": "液压", "sort_order": 1})
	tag2, _ := svc.CreateQuestionTag(map[string]any{"code": "brake", "name": "制动", "sort_order": 2})

	// 设置两个标签
	if err := svc.SetQuestionTags(q.ID, []int{tag1["id"].(int), tag2["id"].(int)}); err != nil {
		t.Fatalf("设置标签失败: %v", err)
	}
	tags, err := svc.GetQuestionTags(q.ID)
	if err != nil {
		t.Fatalf("查询标签失败: %v", err)
	}
	got := tags["tags"].([]map[string]any)
	if len(got) != 2 {
		t.Fatalf("应 2 个标签, got %d", len(got))
	}
	if got[0]["name"] != "液压" {
		t.Fatalf("应按 sort_order 排序: %+v", got)
	}

	// 全量替换为 1 个
	if err := svc.SetQuestionTags(q.ID, []int{tag2["id"].(int)}); err != nil {
		t.Fatalf("替换标签失败: %v", err)
	}
	tags, _ = svc.GetQuestionTags(q.ID)
	if len(tags["tags"].([]map[string]any)) != 1 {
		t.Fatal("替换后应只剩 1 个标签")
	}

	// 清空
	if err := svc.SetQuestionTags(q.ID, []int{}); err != nil {
		t.Fatalf("清空标签失败: %v", err)
	}
	tags, _ = svc.GetQuestionTags(q.ID)
	if len(tags["tags"].([]map[string]any)) != 0 {
		t.Fatal("清空后应为 0 个标签")
	}

	// 不存在的标签
	if err := svc.SetQuestionTags(q.ID, []int{9999}); err == nil {
		t.Fatal("不存在的标签应报错")
	}
	// 不存在的题目
	if err := svc.SetQuestionTags(9999, []int{1}); err == nil {
		t.Fatal("不存在的题目应报错")
	}
}

// --- 目录树 ---

func TestGetCatalogTree(t *testing.T) {
	svc, db := newCatalogSvc(t)
	spec := model.Specialty{Code: "operation", Name: "操作", Status: 1, SortOrder: 1, CreatedAt: testutil.Now()}
	if err := db.Create(&spec).Error; err != nil {
		t.Fatalf("创建专业方向失败: %v", err)
	}
	disabledSpec := model.Specialty{Code: "off", Name: "停用方向", Status: 0, SortOrder: 2, CreatedAt: testutil.Now()}
	db.Create(&disabledSpec)
	// GORM 对带 default 标签的零值字段会省略，改用显式更新设置停用状态
	db.Model(&disabledSpec).Update("status", 0)

	lv := model.CourseLevel{Code: "beginner", Name: "入门", Status: 1, SortOrder: 1, CreatedAt: testutil.Now()}
	if err := db.Create(&lv).Error; err != nil {
		t.Fatalf("创建等级失败: %v", err)
	}

	c1 := model.Course{Name: "叉车基础", Category: "CATEGORY_01", Status: 1, TheoryHours: 20,
		SpecialtyID: ptrInt(spec.SpecialtyID), LevelID: ptrInt(lv.LevelID), CreatedAt: testutil.Now()}
	if err := db.Create(&c1).Error; err != nil {
		t.Fatalf("创建课程失败: %v", err)
	}
	c2 := model.Course{Name: "下架课程", Category: "CATEGORY_01", Status: 0,
		SpecialtyID: ptrInt(spec.SpecialtyID), LevelID: ptrInt(lv.LevelID), CreatedAt: testutil.Now()}
	db.Create(&c2)
	db.Model(&c2).Update("status", 0)
	ch := model.Chapter{CourseID: c1.CourseID, Title: "第一章", Duration: 10, CreatedAt: testutil.Now()}
	if err := db.Create(&ch).Error; err != nil {
		t.Fatalf("创建章节失败: %v", err)
	}

	tree := svc.GetCatalogTree()
	specialties := tree["specialties"].([]map[string]any)
	if len(specialties) != 1 {
		t.Fatalf("应只返回启用的专业方向, got %d", len(specialties))
	}
	if specialties[0]["name"] != "操作" {
		t.Fatalf("专业方向名称不匹配: %+v", specialties[0])
	}
	levels := specialties[0]["levels"].([]map[string]any)
	if len(levels) != 1 {
		t.Fatalf("应 1 个等级, got %d", len(levels))
	}
	courses := levels[0]["courses"].([]map[string]any)
	if len(courses) != 1 {
		t.Fatalf("应只返回上架课程, got %d", len(courses))
	}
	if courses[0]["name"] != "叉车基础" || courses[0]["chapter_count"] != int64(1) {
		t.Fatalf("课程数据不匹配: %+v", courses[0])
	}
}

// TestGetAdminCatalogTree 管理端目录树：含停用项、课程章节节点，课程按 sort_order 排序。
func TestGetAdminCatalogTree(t *testing.T) {
	svc, db := newCatalogSvc(t)
	spec := model.Specialty{Code: "operation", Name: "操作", Status: 1, SortOrder: 1, CreatedAt: testutil.Now()}
	if err := db.Create(&spec).Error; err != nil {
		t.Fatalf("创建专业方向失败: %v", err)
	}
	disabledSpec := model.Specialty{Code: "off", Name: "停用方向", Status: 0, SortOrder: 2, CreatedAt: testutil.Now()}
	db.Create(&disabledSpec)
	db.Model(&disabledSpec).Update("status", 0)

	lv := model.CourseLevel{Code: "beginner", Name: "入门", Status: 1, SortOrder: 1, CreatedAt: testutil.Now()}
	if err := db.Create(&lv).Error; err != nil {
		t.Fatalf("创建等级失败: %v", err)
	}
	// 2 门课程：sort_order 2 在前、1 在后，验证按 sort_order 排序；另 1 门下架课程
	c1 := model.Course{Name: "晚建但排序靠前", Category: "CATEGORY_01", Status: 1, SortOrder: 1,
		SpecialtyID: ptrInt(spec.SpecialtyID), LevelID: ptrInt(lv.LevelID), CreatedAt: testutil.Now()}
	if err := db.Create(&c1).Error; err != nil {
		t.Fatalf("创建课程失败: %v", err)
	}
	c2 := model.Course{Name: "早建但排序靠后", Category: "CATEGORY_01", Status: 1, SortOrder: 2,
		SpecialtyID: ptrInt(spec.SpecialtyID), LevelID: ptrInt(lv.LevelID), CreatedAt: testutil.Now().Add(-time.Hour)}
	if err := db.Create(&c2).Error; err != nil {
		t.Fatalf("创建课程失败: %v", err)
	}
	c3 := model.Course{Name: "下架课程", Category: "CATEGORY_01", Status: 0, SortOrder: 3,
		SpecialtyID: ptrInt(spec.SpecialtyID), LevelID: ptrInt(lv.LevelID), CreatedAt: testutil.Now()}
	db.Create(&c3)
	db.Model(&c3).Update("status", 0)

	ch1 := model.Chapter{CourseID: c1.CourseID, Title: "第一章", Duration: 10, OrderNum: 2, CreatedAt: testutil.Now()}
	db.Create(&ch1)
	ch2 := model.Chapter{CourseID: c1.CourseID, Title: "第二章", Duration: 20, OrderNum: 1, CreatedAt: testutil.Now()}
	db.Create(&ch2)

	tree := svc.GetAdminCatalogTree()
	specialties := tree["specialties"].([]map[string]any)
	if len(specialties) != 2 {
		t.Fatalf("管理端应包含停用方向, got %d", len(specialties))
	}
	if specialties[0]["name"] != "操作" || specialties[1]["name"] != "停用方向" {
		t.Fatalf("方向排序不匹配: %+v", specialties)
	}
	courses := specialties[0]["levels"].([]map[string]any)[0]["courses"].([]map[string]any)
	if len(courses) != 3 {
		t.Fatalf("管理端应包含下架课程, got %d", len(courses))
	}
	if courses[0]["name"] != "晚建但排序靠前" || courses[1]["name"] != "早建但排序靠后" {
		t.Fatalf("课程应按 sort_order 排序: %+v", courses)
	}
	chapters := courses[0]["chapters"].([]map[string]any)
	if len(chapters) != 2 {
		t.Fatalf("课程应含章节节点, got %d", len(chapters))
	}
	if chapters[0]["title"] != "第二章" || chapters[1]["title"] != "第一章" {
		t.Fatalf("章节应按 order_num 排序: %+v", chapters)
	}
	if courses[2]["name"] != "下架课程" {
		t.Fatalf("下架课程应保留: %+v", courses[2])
	}
}

// TestCourseSortOrder 课程 sort_order：创建/更新可设置，列表按 sort_order 升序。
func TestCourseSortOrder(t *testing.T) {
	db := testutil.NewMemoryDB(t)
	svc := NewAdminCourseService(db)

	spec := model.Specialty{Code: "operation", Name: "操作", Status: 1, CreatedAt: testutil.Now()}
	db.Create(&spec)
	lv := model.CourseLevel{Code: "beginner", Name: "入门", Status: 1, CreatedAt: testutil.Now()}
	db.Create(&lv)

	// 创建时设置 sort_order
	created, err := svc.CreateCourse(map[string]any{
		"name": "课程A", "category": "CATEGORY_01",
		"specialty_id": spec.SpecialtyID, "level_id": lv.LevelID, "sort_order": 5,
	})
	if err != nil {
		t.Fatalf("创建课程失败: %v", err)
	}
	if created["sort_order"] != 5 {
		t.Fatalf("创建返回的 sort_order 不匹配: %+v", created)
	}
	courseID := created["course_id"].(int)

	// 更新时修改 sort_order
	updated, err := svc.UpdateCourse(courseID, map[string]any{"sort_order": 1})
	if err != nil {
		t.Fatalf("更新课程失败: %v", err)
	}
	if updated["sort_order"] != 1 {
		t.Fatalf("更新后的 sort_order 不匹配: %+v", updated)
	}

	// 负值应报错
	if _, err := svc.CreateCourse(map[string]any{"name": "课程B", "category": "CATEGORY_01", "sort_order": -1}); err == nil {
		t.Fatal("负排序值应报错")
	}

	// 列表按 sort_order 升序（0 在 1 前，再按创建时间倒序）
	c0 := model.Course{Name: "课程C", Category: "CATEGORY_01", Status: 1, SortOrder: 0,
		SpecialtyID: ptrInt(spec.SpecialtyID), LevelID: ptrInt(lv.LevelID), CreatedAt: testutil.Now()}
	db.Create(&c0)
	list := svc.GetCourses(1, 10, "", "", nil, nil)["courses"].([]map[string]any)
	if len(list) != 2 {
		t.Fatalf("应 2 门课程, got %d", len(list))
	}
	if list[0]["name"] != "课程C" || list[1]["name"] != "课程A" {
		t.Fatalf("课程列表应按 sort_order 升序: %+v", list)
	}
}

// --- 课程扩展字段与前置课程 ---

func TestAdminCourse_TrainingFields(t *testing.T) {
	db := testutil.NewMemoryDB(t)
	svc := NewAdminCourseService(db)

	spec := model.Specialty{Code: "maintenance", Name: "维修", Status: 1, CreatedAt: testutil.Now()}
	db.Create(&spec)
	lv := model.CourseLevel{Code: "beginner", Name: "入门", Status: 1, CreatedAt: testutil.Now()}
	db.Create(&lv)
	tpl := model.CertificateTemplate{Code: "CERT", Name: "证书", ValidityDays: 730, Status: 1, CreatedAt: testutil.Now()}
	db.Create(&tpl)

	// 前置课程
	prereq := model.Course{Name: "前置课程", Category: "CATEGORY_01", Status: 1, CreatedAt: testutil.Now()}
	db.Create(&prereq)

	data := map[string]any{
		"name":                    "液压系统维护",
		"category":                "CATEGORY_04",
		"specialty_id":            spec.SpecialtyID,
		"level_id":                lv.LevelID,
		"certificate_template_id": tpl.ID,
		"theory_hours":            30,
		"practice_hours":          20,
		"prerequisite_course_ids": []int{prereq.CourseID},
	}
	result, err := svc.CreateCourse(data)
	if err != nil {
		t.Fatalf("创建课程失败: %v", err)
	}
	courseID := result["course_id"].(int)
	if result["theory_hours"] != 30 || result["practice_hours"] != 20 {
		t.Fatalf("学时字段不匹配: %+v", result)
	}

	detail, err := svc.GetCourseDetail(courseID)
	if err != nil {
		t.Fatalf("获取详情失败: %v", err)
	}
	if detail["specialty"].(map[string]any)["name"] != "维修" {
		t.Fatalf("专业方向元数据缺失: %+v", detail["specialty"])
	}
	if detail["level"].(map[string]any)["name"] != "入门" {
		t.Fatalf("等级元数据缺失: %+v", detail["level"])
	}
	cert := detail["certificate_template"].(map[string]any)
	if cert["validity_days"] != 730 {
		t.Fatalf("证书模板元数据缺失: %+v", cert)
	}
	prereqs := detail["prerequisites"].([]map[string]any)
	if len(prereqs) != 1 || prereqs[0]["name"] != "前置课程" {
		t.Fatalf("前置课程元数据缺失: %+v", prereqs)
	}

	// 不存在的引用应报错
	if _, err := svc.CreateCourse(map[string]any{"name": "x", "category": "CATEGORY_01", "specialty_id": 9999}); err == nil {
		t.Fatal("不存在的专业方向应报错")
	}
	if _, err := svc.CreateCourse(map[string]any{"name": "x", "category": "CATEGORY_01", "theory_hours": -1}); err == nil {
		t.Fatal("负学时应报错")
	}
	if _, err := svc.CreateCourse(map[string]any{"name": "x", "category": "CATEGORY_01", "prerequisite_course_ids": []int{9999}}); err == nil {
		t.Fatal("不存在的前置课程应报错")
	}

	// 更新：替换前置课程 + 清空等级
	if _, err := svc.UpdateCourse(courseID, map[string]any{
		"level_id": 0, "prerequisite_course_ids": []any{},
	}); err != nil {
		t.Fatalf("更新失败: %v", err)
	}
	detail, _ = svc.GetCourseDetail(courseID)
	if len(detail["prerequisites"].([]map[string]any)) != 0 {
		t.Fatal("前置课程应被清空")
	}
	if _, ok := detail["level"]; ok {
		t.Fatal("等级应被清空")
	}

	// 自己作为前置课程应报错
	if _, err := svc.UpdateCourse(courseID, map[string]any{"prerequisite_course_ids": []int{courseID}}); err == nil {
		t.Fatal("自引用前置课程应报错")
	}

	// 多级依赖成环应报错（C→B→A→C 与 A↔B 两课程环）
	a := model.Course{Name: "课程A", Category: "CATEGORY_01", Status: 1, CreatedAt: testutil.Now()}
	b := model.Course{Name: "课程B", Category: "CATEGORY_01", Status: 1, CreatedAt: testutil.Now()}
	c := model.Course{Name: "课程C", Category: "CATEGORY_01", Status: 1, CreatedAt: testutil.Now()}
	db.Create(&a)
	db.Create(&b)
	db.Create(&c)
	if _, err := svc.UpdateCourse(a.CourseID, map[string]any{"prerequisite_course_ids": []int{b.CourseID}}); err != nil {
		t.Fatalf("设置前置课程失败: %v", err)
	}
	if _, err := svc.UpdateCourse(b.CourseID, map[string]any{"prerequisite_course_ids": []int{c.CourseID}}); err != nil {
		t.Fatalf("设置前置课程失败: %v", err)
	}
	if _, err := svc.UpdateCourse(c.CourseID, map[string]any{"prerequisite_course_ids": []int{a.CourseID}}); err == nil {
		t.Fatal("多级依赖成环应报错")
	}
	if _, err := svc.UpdateCourse(b.CourseID, map[string]any{"prerequisite_course_ids": []int{a.CourseID}}); err == nil {
		t.Fatal("两课程互相依赖应报错")
	}
	// 成环请求被拒绝后，原有关联应保持不变
	detail, err = svc.GetCourseDetail(b.CourseID)
	if err != nil {
		t.Fatalf("获取详情失败: %v", err)
	}
	prereqs = detail["prerequisites"].([]map[string]any)
	if len(prereqs) != 1 || prereqs[0]["name"] != "课程C" {
		t.Fatalf("成环拒绝后原关联应保留: %+v", prereqs)
	}
}

// --- 学员端课程详情与列表过滤 ---

func TestCourseService_TrainingFields(t *testing.T) {
	db := testutil.NewMemoryDB(t)
	svc := NewCourseService(db, nil)

	spec := model.Specialty{Code: "safety", Name: "安全", Status: 1, CreatedAt: testutil.Now()}
	db.Create(&spec)
	lv := model.CourseLevel{Code: "beginner", Name: "入门", Status: 1, CreatedAt: testutil.Now()}
	db.Create(&lv)
	course := model.Course{Name: "安全操作规范", Category: "CATEGORY_02", Status: 1,
		SpecialtyID: ptrInt(spec.SpecialtyID), LevelID: ptrInt(lv.LevelID),
		TheoryHours: 12, PracticeHours: 8, CreatedAt: testutil.Now()}
	if err := db.Create(&course).Error; err != nil {
		t.Fatalf("创建课程失败: %v", err)
	}

	// 学员端列表按专业方向/等级过滤
	list := svc.GetCourses(1, 10, "", ptrInt(spec.SpecialtyID), ptrInt(lv.LevelID))
	if list["total"].(int64) != 1 {
		t.Fatalf("过滤后应 1 条, got %v", list["total"])
	}
	empty := svc.GetCourses(1, 10, "", ptrInt(spec.SpecialtyID), ptrInt(9999))
	if empty["total"].(int64) != 0 {
		t.Fatal("不存在的等级应过滤为空")
	}

	// 详情含等级/学时元数据
	detail, err := svc.GetCourseDetail(course.CourseID, 0)
	if err != nil {
		t.Fatalf("获取详情失败: %v", err)
	}
	info := detail["course_info"].(map[string]any)
	if info["theory_hours"] != 12 || info["practice_hours"] != 8 {
		t.Fatalf("学时不匹配: %+v", info)
	}
	if info["level"].(map[string]any)["name"] != "入门" {
		t.Fatalf("等级元数据缺失: %+v", info["level"])
	}
}

// --- 题库标签查询 ---

func TestQuestionBank_Tags(t *testing.T) {
	svc, db := newCatalogSvc(t)
	qsvc := NewQuestionBankService(db)

	tag1, _ := svc.CreateQuestionTag(map[string]any{"code": "regulation", "name": "法规", "sort_order": 1})
	tag2, _ := svc.CreateQuestionTag(map[string]any{"code": "hydraulic", "name": "液压", "sort_order": 2})

	// 创建题目时打标
	q1, err := qsvc.CreateQuestion(map[string]any{
		"type": "single_choice", "content": "法规题", "options": []string{"A", "B"}, "answer": "A",
		"tag_ids": []int{tag1["id"].(int)},
	}, nil, "tutor")
	if err != nil {
		t.Fatalf("创建题目失败: %v", err)
	}
	q2, err := qsvc.CreateQuestion(map[string]any{
		"type": "true_false", "content": "液压题", "answer": "true",
		"tag_ids": []int{tag2["id"].(int)},
	}, nil, "tutor")
	if err != nil {
		t.Fatalf("创建题目失败: %v", err)
	}
	if len(q1["tags"].([]map[string]any)) != 1 || q1["tags"].([]map[string]any)[0]["name"] != "法规" {
		t.Fatalf("创建返回的标签不匹配: %+v", q1["tags"])
	}

	// 按标签过滤
	byTag := qsvc.ListQuestions(1, 20, "", nil, "", "", ptrInt(tag2["id"].(int)))
	if byTag["total"].(int64) != 1 {
		t.Fatalf("按标签过滤应 1 条, got %v", byTag["total"])
	}
	q := byTag["questions"].([]map[string]any)[0]
	if q["content"] != "液压题" {
		t.Fatalf("过滤结果不匹配: %+v", q)
	}
	if len(q["tags"].([]map[string]any)) != 1 {
		t.Fatalf("列表应附带标签: %+v", q["tags"])
	}

	// 更新题目时替换标签
	updated, err := qsvc.UpdateQuestion(q1["id"].(int), map[string]any{"tag_ids": []int{tag2["id"].(int)}})
	if err != nil {
		t.Fatalf("更新题目失败: %v", err)
	}
	if len(updated["tags"].([]map[string]any)) != 1 || updated["tags"].([]map[string]any)[0]["name"] != "液压" {
		t.Fatalf("更新后标签不匹配: %+v", updated["tags"])
	}

	// 详情含标签
	got, err := qsvc.GetQuestion(q2["id"].(int))
	if err != nil {
		t.Fatalf("获取题目失败: %v", err)
	}
	if len(got["tags"].([]map[string]any)) != 1 {
		t.Fatalf("详情应含标签: %+v", got["tags"])
	}
}
