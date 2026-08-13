// Package service 培训目录（专业方向/等级/证书模板/题库标签/目录树）测试。
package service

import (
	"encoding/json"
	"testing"
	"time"

	"go.uber.org/zap"
	"gorm.io/gorm"

	"forklift-training/internal/model"
	"forklift-training/internal/testutil"
)

func newCatalogSvc(t *testing.T) (*TrainingCatalogService, *gorm.DB) {
	t.Helper()
	db := testutil.NewMemoryDB(t)
	return NewTrainingCatalogService(db, zap.NewNop()), db
}

// p16 构造 *int16 指针（测试用）。
func p16(v int16) *int16 { return &v }

// --- 专业方向 ---

func TestSpecialtyCRUD(t *testing.T) {
	svc, _ := newCatalogSvc(t)

	// 创建
	result, err := svc.CreateSpecialty(SpecialtyInput{Code: "operation", Name: "操作", SortOrder: ptrInt(1)})
	if err != nil {
		t.Fatalf("创建专业方向失败: %v", err)
	}
	specID := result.SpecialtyID
	if result.Name != "操作" || result.Status != 1 {
		t.Fatalf("创建结果不匹配: %+v", result)
	}

	// 编码/名称为空校验
	if _, err := svc.CreateSpecialty(SpecialtyInput{Name: "x"}); err == nil {
		t.Fatal("编码为空应报错")
	}
	if _, err := svc.CreateSpecialty(SpecialtyInput{Code: "x"}); err == nil {
		t.Fatal("名称为空应报错")
	}

	// 更新
	updated, err := svc.UpdateSpecialty(specID, SpecialtyInput{Name: "操作方向", Status: p16(0)})
	if err != nil {
		t.Fatalf("更新失败: %v", err)
	}
	if updated.Name != "操作方向" || updated.Status != 0 {
		t.Fatalf("更新结果不匹配: %+v", updated)
	}

	// 列表：管理端含停用项，学员端仅启用项
	all := svc.ListSpecialties(false)
	if len(all) != 1 {
		t.Fatal("管理端应看到 1 条（含停用）")
	}
	active := svc.ListSpecialties(true)
	if len(active) != 0 {
		t.Fatal("学员端应看不到停用项")
	}

	// 删除 + 不存在
	if err := svc.DeleteSpecialty(specID); err != nil {
		t.Fatalf("删除失败: %v", err)
	}
	if err := svc.DeleteSpecialty(specID); err == nil {
		t.Fatal("重复删除应报错")
	}
	if _, err := svc.UpdateSpecialty(9999, SpecialtyInput{Name: "x"}); err == nil {
		t.Fatal("更新不存在的专业方向应报错")
	}
}

// TestSpecialtyValidation 专业方向校验（收口在 service）：编码/名称必填、编码唯一、状态枚举。
func TestSpecialtyValidation(t *testing.T) {
	svc, _ := newCatalogSvc(t)

	if _, err := svc.CreateSpecialty(SpecialtyInput{Code: "", Name: "x"}); err == nil || err.Error() != "专业方向编码不能为空" {
		t.Fatalf("编码为空应报「专业方向编码不能为空」, got %v", err)
	}
	if _, err := svc.CreateSpecialty(SpecialtyInput{Code: "x", Name: ""}); err == nil || err.Error() != "专业方向名称不能为空" {
		t.Fatalf("名称为空应报「专业方向名称不能为空」, got %v", err)
	}

	if _, err := svc.CreateSpecialty(SpecialtyInput{Code: "op", Name: "操作"}); err != nil {
		t.Fatalf("创建失败: %v", err)
	}
	if _, err := svc.CreateSpecialty(SpecialtyInput{Code: "op", Name: "重复"}); err == nil || err.Error() != "专业方向编码已存在" {
		t.Fatalf("重复编码应报「专业方向编码已存在」, got %v", err)
	}
	if _, err := svc.CreateSpecialty(SpecialtyInput{Code: "x2", Name: "x", Status: p16(5)}); err == nil || err.Error() != "状态值无效" {
		t.Fatalf("非法状态应报「状态值无效」, got %v", err)
	}

	created, _ := svc.CreateSpecialty(SpecialtyInput{Code: "x3", Name: "x"})
	if _, err := svc.UpdateSpecialty(created.SpecialtyID, SpecialtyInput{Status: p16(2)}); err == nil || err.Error() != "状态值无效" {
		t.Fatalf("更新非法状态应报「状态值无效」, got %v", err)
	}
	if _, err := svc.UpdateSpecialty(created.SpecialtyID, SpecialtyInput{Code: "op"}); err == nil || err.Error() != "专业方向编码已存在" {
		t.Fatalf("更新撞码应报「专业方向编码已存在」, got %v", err)
	}
	if _, err := svc.UpdateSpecialty(created.SpecialtyID, SpecialtyInput{Code: "x3"}); err != nil {
		t.Fatalf("更新为自身编码应成功: %v", err)
	}
}

// --- 课程等级 ---

func TestLevelCRUD(t *testing.T) {
	svc, _ := newCatalogSvc(t)
	result, err := svc.CreateLevel(LevelInput{Code: "beginner", Name: "入门", SortOrder: ptrInt(1)})
	if err != nil {
		t.Fatalf("创建等级失败: %v", err)
	}
	levelID := result.LevelID

	if _, err := svc.CreateLevel(LevelInput{Name: "x"}); err == nil {
		t.Fatal("编码为空应报错")
	}
	updated, err := svc.UpdateLevel(levelID, LevelInput{Name: "初级"})
	if err != nil {
		t.Fatalf("更新失败: %v", err)
	}
	if updated.Name != "初级" {
		t.Fatalf("更新结果不匹配: %+v", updated)
	}
	all := svc.ListLevels(false)
	if len(all) != 1 {
		t.Fatal("应看到 1 条等级")
	}
	if err := svc.DeleteLevel(levelID); err != nil {
		t.Fatalf("删除失败: %v", err)
	}
	if err := svc.DeleteLevel(levelID); err == nil {
		t.Fatal("重复删除应报错")
	}
}

// TestLevelValidation 课程等级校验（收口在 service）：编码唯一 + 状态枚举。
func TestLevelValidation(t *testing.T) {
	svc, _ := newCatalogSvc(t)

	if _, err := svc.CreateLevel(LevelInput{Code: "bg", Name: "入门"}); err != nil {
		t.Fatalf("创建失败: %v", err)
	}
	if _, err := svc.CreateLevel(LevelInput{Code: "bg", Name: "重复"}); err == nil || err.Error() != "课程等级编码已存在" {
		t.Fatalf("重复编码应报「课程等级编码已存在」, got %v", err)
	}
	if _, err := svc.CreateLevel(LevelInput{Code: "x", Name: "x", Status: p16(9)}); err == nil || err.Error() != "状态值无效" {
		t.Fatalf("非法状态应报「状态值无效」, got %v", err)
	}
}

// --- 证书模板 ---

func TestCertificateTemplateCRUD(t *testing.T) {
	svc, _ := newCatalogSvc(t)
	result, err := svc.CreateCertificateTemplate(CertificateTemplateInput{
		Code: "CERT_1", Name: "叉车培训证书", ValidityDays: ptrInt(1460),
	})
	if err != nil {
		t.Fatalf("创建模板失败: %v", err)
	}
	tplID := result.ID
	if result.ValidityDays != 1460 {
		t.Fatalf("有效期不匹配: %+v", result)
	}

	// 无效有效期
	if _, err := svc.CreateCertificateTemplate(CertificateTemplateInput{Code: "C", Name: "x", ValidityDays: ptrInt(0)}); err == nil {
		t.Fatal("有效期为 0 应报错")
	}
	if _, err := svc.UpdateCertificateTemplate(tplID, CertificateTemplateInput{ValidityDays: ptrInt(-5)}); err == nil {
		t.Fatal("负有效期应报错")
	}

	// 默认有效期 365
	def, err := svc.CreateCertificateTemplate(CertificateTemplateInput{Code: "CERT_2", Name: "默认模板"})
	if err != nil {
		t.Fatalf("创建默认模板失败: %v", err)
	}
	if def.ValidityDays != 365 {
		t.Fatalf("默认有效期应为 365, got %d", def.ValidityDays)
	}

	updated, err := svc.UpdateCertificateTemplate(tplID, CertificateTemplateInput{ValidityDays: ptrInt(730)})
	if err != nil {
		t.Fatalf("更新失败: %v", err)
	}
	if updated.ValidityDays != 730 {
		t.Fatalf("更新后有效期不匹配: %+v", updated)
	}
	list := svc.ListCertificateTemplates(false)
	if len(list) != 2 {
		t.Fatal("应看到 2 条模板")
	}
	if err := svc.DeleteCertificateTemplate(tplID); err != nil {
		t.Fatalf("删除失败: %v", err)
	}
}

// TestCertificateTemplateValidation 证书模板校验（收口在 service）：编码唯一 + 状态枚举。
func TestCertificateTemplateValidation(t *testing.T) {
	svc, _ := newCatalogSvc(t)

	if _, err := svc.CreateCertificateTemplate(CertificateTemplateInput{Code: "C1", Name: "模板"}); err != nil {
		t.Fatalf("创建失败: %v", err)
	}
	if _, err := svc.CreateCertificateTemplate(CertificateTemplateInput{Code: "C1", Name: "重复"}); err == nil || err.Error() != "证书模板编码已存在" {
		t.Fatalf("重复编码应报「证书模板编码已存在」, got %v", err)
	}
	if _, err := svc.CreateCertificateTemplate(CertificateTemplateInput{Code: "C2", Name: "x", Status: p16(2)}); err == nil || err.Error() != "状态值无效" {
		t.Fatalf("非法状态应报「状态值无效」, got %v", err)
	}
}

// --- 题库标签与题目关联 ---

func TestQuestionTagCRUD(t *testing.T) {
	svc, _ := newCatalogSvc(t)
	result, err := svc.CreateQuestionTag(QuestionTagInput{Code: "hydraulic", Name: "液压"})
	if err != nil {
		t.Fatalf("创建标签失败: %v", err)
	}
	tagID := result.ID

	if _, err := svc.CreateQuestionTag(QuestionTagInput{Name: "x"}); err == nil {
		t.Fatal("编码为空应报错")
	}
	updated, err := svc.UpdateQuestionTag(tagID, QuestionTagInput{Name: "液压系统"})
	if err != nil {
		t.Fatalf("更新失败: %v", err)
	}
	if updated.Name != "液压系统" {
		t.Fatalf("更新结果不匹配: %+v", updated)
	}
	active := svc.ListQuestionTags(true)
	if len(active) != 1 {
		t.Fatal("应看到 1 条标签")
	}
	if err := svc.DeleteQuestionTag(tagID); err != nil {
		t.Fatalf("删除失败: %v", err)
	}
}

// TestQuestionTagValidation 题库标签校验（收口在 service）：状态枚举。
func TestQuestionTagValidation(t *testing.T) {
	svc, _ := newCatalogSvc(t)

	if _, err := svc.CreateQuestionTag(QuestionTagInput{Code: "hydraulic", Name: "液压", Status: p16(5)}); err == nil || err.Error() != "状态值无效" {
		t.Fatalf("非法状态应报「状态值无效」, got %v", err)
	}
	tag, _ := svc.CreateQuestionTag(QuestionTagInput{Code: "hydraulic", Name: "液压"})
	if _, err := svc.UpdateQuestionTag(tag.ID, QuestionTagInput{Status: p16(2)}); err == nil || err.Error() != "状态值无效" {
		t.Fatalf("更新非法状态应报「状态值无效」, got %v", err)
	}
}

// TestQuestionTagCodeUnique 标签编码唯一性：创建/更新重复编码均返回友好错误。
func TestQuestionTagCodeUnique(t *testing.T) {
	svc, _ := newCatalogSvc(t)
	tag1, err := svc.CreateQuestionTag(QuestionTagInput{Code: "hydraulic", Name: "液压"})
	if err != nil {
		t.Fatalf("创建失败: %v", err)
	}
	if _, err := svc.CreateQuestionTag(QuestionTagInput{Code: "hydraulic", Name: "重复"}); err == nil || err.Error() != "标签编码已存在" {
		t.Fatalf("重复编码创建应报「标签编码已存在」, got %v", err)
	}
	if _, err := svc.UpdateQuestionTag(tag1.ID, QuestionTagInput{Code: "hydraulic"}); err != nil {
		t.Fatalf("更新为自身编码应成功: %v", err)
	}
	tag2, err := svc.CreateQuestionTag(QuestionTagInput{Code: "brake", Name: "制动"})
	if err != nil {
		t.Fatalf("创建失败: %v", err)
	}
	if _, err := svc.UpdateQuestionTag(tag2.ID, QuestionTagInput{Code: "hydraulic"}); err == nil || err.Error() != "标签编码已存在" {
		t.Fatalf("改编码撞车应报「标签编码已存在」, got %v", err)
	}
}

// TestListQuestionTags_QuestionCount 标签列表 question_count：
// 学员端仅统计已发布题目，管理端统计全部题目。
func TestListQuestionTags_QuestionCount(t *testing.T) {
	svc, db := newCatalogSvc(t)
	tag, _ := svc.CreateQuestionTag(QuestionTagInput{Code: "regulation", Name: "法规"})
	qsvc := NewQuestionBankService(db, nil, zap.NewNop())

	// 1 道已发布 + 1 道草稿（未发布）
	published, err := qsvc.CreateQuestion(map[string]any{
		"type": "single_choice", "content": "已发布题", "options": []string{"A", "B"}, "answer": "A",
		"status": "published", "tag_ids": []int{tag.ID},
	}, nil, "tutor")
	if err != nil {
		t.Fatalf("创建已发布题目失败: %v", err)
	}
	_ = published
	draft, err := qsvc.CreateQuestion(map[string]any{
		"type": "true_false", "content": "草稿题", "answer": "true",
		"status": "draft", "tag_ids": []int{tag.ID},
	}, nil, "tutor")
	if err != nil {
		t.Fatalf("创建草稿题目失败: %v", err)
	}
	_ = draft
	// 另一个无题目标签
	empty, _ := svc.CreateQuestionTag(QuestionTagInput{Code: "brake", Name: "制动"})

	studentTags := svc.ListQuestionTags(true)
	byID := map[int]QuestionTagDict{}
	for _, d := range studentTags {
		byID[d.ID] = d
	}
	if byID[tag.ID].QuestionCount == nil || *byID[tag.ID].QuestionCount != 1 {
		t.Fatalf("学员端应统计 1 道已发布题, got %v", byID[tag.ID].QuestionCount)
	}
	if byID[empty.ID].QuestionCount == nil || *byID[empty.ID].QuestionCount != 0 {
		t.Fatalf("无题目标签应为 0, got %v", byID[empty.ID].QuestionCount)
	}

	adminTags := svc.ListQuestionTags(false)
	byID2 := map[int]QuestionTagDict{}
	for _, d := range adminTags {
		byID2[d.ID] = d
	}
	if byID2[tag.ID].QuestionCount == nil || *byID2[tag.ID].QuestionCount != 2 {
		t.Fatalf("管理端应统计全部 2 道题, got %v", byID2[tag.ID].QuestionCount)
	}
}

func TestSetQuestionTags(t *testing.T) {
	svc, db := newCatalogSvc(t)
	q := testutil.SeedQuestion(t, db, "single_choice", "液压相关题目", "A")
	tag1, _ := svc.CreateQuestionTag(QuestionTagInput{Code: "hydraulic", Name: "液压", SortOrder: ptrInt(1)})
	tag2, _ := svc.CreateQuestionTag(QuestionTagInput{Code: "brake", Name: "制动", SortOrder: ptrInt(2)})

	// 设置两个标签
	if err := svc.SetQuestionTags(q.ID, []int{tag1.ID, tag2.ID}); err != nil {
		t.Fatalf("设置标签失败: %v", err)
	}
	tags := svc.loadQuestionTags(q.ID)
	if len(tags) != 2 {
		t.Fatalf("应 2 个标签, got %d", len(tags))
	}
	if tags[0].Name != "液压" {
		t.Fatalf("应按 sort_order 排序: %+v", tags)
	}

	// 全量替换为 1 个
	if err := svc.SetQuestionTags(q.ID, []int{tag2.ID}); err != nil {
		t.Fatalf("替换标签失败: %v", err)
	}
	tags = svc.loadQuestionTags(q.ID)
	if len(tags) != 1 {
		t.Fatal("替换后应只剩 1 个标签")
	}

	// 清空
	if err := svc.SetQuestionTags(q.ID, []int{}); err != nil {
		t.Fatalf("清空标签失败: %v", err)
	}
	tags = svc.loadQuestionTags(q.ID)
	if len(tags) != 0 {
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

	c1 := model.Course{Name: "叉车基础", Status: 1, TheoryHours: 20,
		SpecialtyID: ptrInt(spec.SpecialtyID), LevelID: ptrInt(lv.LevelID), CreatedAt: testutil.Now()}
	if err := db.Create(&c1).Error; err != nil {
		t.Fatalf("创建课程失败: %v", err)
	}
	c2 := model.Course{Name: "下架课程", Status: 0,
		SpecialtyID: ptrInt(spec.SpecialtyID), LevelID: ptrInt(lv.LevelID), CreatedAt: testutil.Now()}
	db.Create(&c2)
	db.Model(&c2).Update("status", 0)
	ch := model.Chapter{CourseID: c1.CourseID, Title: "第一章", Duration: 10, CreatedAt: testutil.Now()}
	if err := db.Create(&ch).Error; err != nil {
		t.Fatalf("创建章节失败: %v", err)
	}

	tree := svc.GetCatalogTree()
	if len(tree.Specialties) != 1 {
		t.Fatalf("应只返回启用的专业方向, got %d", len(tree.Specialties))
	}
	if tree.Specialties[0].Name != "操作" {
		t.Fatalf("专业方向名称不匹配: %+v", tree.Specialties[0])
	}
	levels := tree.Specialties[0].Levels
	if len(levels) != 1 {
		t.Fatalf("应 1 个等级, got %d", len(levels))
	}
	courses := levels[0].Courses
	if len(courses) != 1 {
		t.Fatalf("应只返回上架课程, got %d", len(courses))
	}
	if courses[0].Name != "叉车基础" || *courses[0].ChapterCount != 1 {
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
	c1 := model.Course{Name: "晚建但排序靠前", Status: 1, SortOrder: 1,
		SpecialtyID: ptrInt(spec.SpecialtyID), LevelID: ptrInt(lv.LevelID), CreatedAt: testutil.Now()}
	if err := db.Create(&c1).Error; err != nil {
		t.Fatalf("创建课程失败: %v", err)
	}
	c2 := model.Course{Name: "早建但排序靠后", Status: 1, SortOrder: 2,
		SpecialtyID: ptrInt(spec.SpecialtyID), LevelID: ptrInt(lv.LevelID), CreatedAt: testutil.Now().Add(-time.Hour)}
	if err := db.Create(&c2).Error; err != nil {
		t.Fatalf("创建课程失败: %v", err)
	}
	c3 := model.Course{Name: "下架课程", Status: 0, SortOrder: 3,
		SpecialtyID: ptrInt(spec.SpecialtyID), LevelID: ptrInt(lv.LevelID), CreatedAt: testutil.Now()}
	db.Create(&c3)
	db.Model(&c3).Update("status", 0)

	ch1 := model.Chapter{CourseID: c1.CourseID, Title: "第一章", Duration: 10, OrderNum: 2, CreatedAt: testutil.Now()}
	db.Create(&ch1)
	ch2 := model.Chapter{CourseID: c1.CourseID, Title: "第二章", Duration: 20, OrderNum: 1, CreatedAt: testutil.Now()}
	db.Create(&ch2)

	tree := svc.GetAdminCatalogTree()
	specialties := tree.Specialties
	if len(specialties) != 2 {
		t.Fatalf("管理端应包含停用方向, got %d", len(specialties))
	}
	if specialties[0].Name != "操作" || specialties[1].Name != "停用方向" {
		t.Fatalf("方向排序不匹配: %+v", specialties)
	}
	courses := specialties[0].Levels[0].Courses
	if len(courses) != 3 {
		t.Fatalf("管理端应包含下架课程, got %d", len(courses))
	}
	if courses[0].Name != "晚建但排序靠前" || courses[1].Name != "早建但排序靠后" {
		t.Fatalf("课程应按 sort_order 排序: %+v", courses)
	}
	chapters := *courses[0].Chapters
	if len(chapters) != 2 {
		t.Fatalf("课程应含章节节点, got %d", len(chapters))
	}
	if chapters[0].Title != "第二章" || chapters[1].Title != "第一章" {
		t.Fatalf("章节应按 order_num 排序: %+v", chapters)
	}
	if courses[2].Name != "下架课程" {
		t.Fatalf("下架课程应保留: %+v", courses[2])
	}
}

// --- 字典 JSON 契约（字节级） ---
// 旧实现以 map[string]any 返回字典，encoding/json 按键排序序列化；
// typed DTO 的字段声明顺序必须与排序后键序一致，保证响应 JSON 字节级不变。

func TestSpecialtyDictJSON(t *testing.T) {
	got, _ := json.Marshal(SpecialtyDict{
		Code: "op", CreatedAt: "2026-08-08T08:00:00.000000", Description: "方向说明",
		Name: "操作", SortOrder: 1, SpecialtyID: 3, Status: 1,
	})
	want := `{"code":"op","created_at":"2026-08-08T08:00:00.000000","description":"方向说明","name":"操作","sort_order":1,"specialty_id":3,"status":1}`
	if string(got) != want {
		t.Fatalf("专业方向字典 JSON 与旧 map 契约不符\n got: %s\nwant: %s", got, want)
	}
}

func TestLevelDictJSON(t *testing.T) {
	got, _ := json.Marshal(LevelDict{
		Code: "bg", CreatedAt: "2026-08-08T08:00:00.000000", Description: "",
		LevelID: 2, Name: "入门", SortOrder: 1, Status: 1,
	})
	want := `{"code":"bg","created_at":"2026-08-08T08:00:00.000000","description":"","level_id":2,"name":"入门","sort_order":1,"status":1}`
	if string(got) != want {
		t.Fatalf("课程等级字典 JSON 与旧 map 契约不符\n got: %s\nwant: %s", got, want)
	}
}

func TestCertificateTemplateDictJSON(t *testing.T) {
	got, _ := json.Marshal(CertificateTemplateDict{
		Code: "C1", CreatedAt: "2026-08-08T08:00:00.000000", Description: "模板",
		ID: 1, Name: "证书", Status: 1, TemplateURL: "https://x/t.pdf",
		UpdatedAt: "2026-08-09T08:00:00.000000", ValidityDays: 365,
	})
	want := `{"code":"C1","created_at":"2026-08-08T08:00:00.000000","description":"模板","id":1,"name":"证书","status":1,"template_url":"https://x/t.pdf","updated_at":"2026-08-09T08:00:00.000000","validity_days":365}`
	if string(got) != want {
		t.Fatalf("证书模板字典 JSON 与旧 map 契约不符\n got: %s\nwant: %s", got, want)
	}
}

func TestQuestionTagDictJSON(t *testing.T) {
	// 创建/更新返回：无 question_count
	got, _ := json.Marshal(QuestionTagDict{
		Code: "hydraulic", CreatedAt: "2026-08-08T08:00:00.000000", Description: "",
		ID: 1, Name: "液压", SortOrder: 1, Status: 1,
		UpdatedAt: "2026-08-08T08:00:00.000000",
	})
	want := `{"code":"hydraulic","created_at":"2026-08-08T08:00:00.000000","description":"","id":1,"name":"液压","sort_order":1,"status":1,"updated_at":"2026-08-08T08:00:00.000000"}`
	if string(got) != want {
		t.Fatalf("题库标签字典 JSON 与旧 map 契约不符\n got: %s\nwant: %s", got, want)
	}

	// 列表返回：question_count 恒存在（含 0）
	zero := int64(0)
	withCount, _ := json.Marshal(QuestionTagDict{
		Code: "hydraulic", CreatedAt: "2026-08-08T08:00:00.000000", Description: "",
		ID: 1, Name: "液压", QuestionCount: &zero, SortOrder: 1, Status: 1,
		UpdatedAt: "2026-08-08T08:00:00.000000",
	})
	wantCount := `{"code":"hydraulic","created_at":"2026-08-08T08:00:00.000000","description":"","id":1,"name":"液压","question_count":0,"sort_order":1,"status":1,"updated_at":"2026-08-08T08:00:00.000000"}`
	if string(withCount) != wantCount {
		t.Fatalf("题库标签列表字典 JSON 与旧 map 契约不符\n got: %s\nwant: %s", withCount, wantCount)
	}
}

func TestQuestionTagRefJSON(t *testing.T) {
	got, _ := json.Marshal(QuestionTagRef{Code: "hydraulic", ID: 1, Name: "液压", SortOrder: 1, Status: 1})
	want := `{"code":"hydraulic","id":1,"name":"液压","sort_order":1,"status":1}`
	if string(got) != want {
		t.Fatalf("题目-标签关联 JSON 与旧 map 契约不符\n got: %s\nwant: %s", got, want)
	}
}

// TestCourseSortOrder 课程 sort_order：创建/更新可设置，列表按 sort_order 升序。
func TestCourseSortOrder(t *testing.T) {
	db := testutil.NewMemoryDB(t)
	svc := NewAdminCourseService(db, nil, zap.NewNop())

	spec := model.Specialty{Code: "operation", Name: "操作", Status: 1, CreatedAt: testutil.Now()}
	db.Create(&spec)
	lv := model.CourseLevel{Code: "beginner", Name: "入门", Status: 1, CreatedAt: testutil.Now()}
	db.Create(&lv)

	// 创建时设置 sort_order
	created, err := svc.CreateCourse(&CourseInput{
		Name:        ptrStr("课程A"),
		SpecialtyID: ptrInt(spec.SpecialtyID), LevelID: ptrInt(lv.LevelID), SortOrder: ptrInt(5),
	})
	if err != nil {
		t.Fatalf("创建课程失败: %v", err)
	}
	if created.SortOrder != 5 {
		t.Fatalf("创建返回的 sort_order 不匹配: %+v", created)
	}
	courseID := created.CourseID

	// 更新时修改 sort_order
	updated, err := svc.UpdateCourse(courseID, &CourseInput{SortOrder: ptrInt(1)})
	if err != nil {
		t.Fatalf("更新课程失败: %v", err)
	}
	if updated.SortOrder != 1 {
		t.Fatalf("更新后的 sort_order 不匹配: %+v", updated)
	}

	// 负值应报错
	if _, err := svc.CreateCourse(&CourseInput{Name: ptrStr("课程B"), SortOrder: ptrInt(-1)}); err == nil {
		t.Fatal("负排序值应报错")
	}

	// 列表按 sort_order 升序（0 在 1 前，再按创建时间倒序）
	c0 := model.Course{Name: "课程C", Status: 1, SortOrder: 0,
		SpecialtyID: ptrInt(spec.SpecialtyID), LevelID: ptrInt(lv.LevelID), CreatedAt: testutil.Now()}
	db.Create(&c0)
	list := svc.GetCourses(1, 10, "", nil, nil).Courses
	if len(list) != 2 {
		t.Fatalf("应 2 门课程, got %d", len(list))
	}
	if list[0].Name != "课程C" || list[1].Name != "课程A" {
		t.Fatalf("课程列表应按 sort_order 升序: %+v", list)
	}
}

// --- 课程扩展字段与前置课程 ---

func TestAdminCourse_TrainingFields(t *testing.T) {
	db := testutil.NewMemoryDB(t)
	svc := NewAdminCourseService(db, nil, zap.NewNop())

	spec := model.Specialty{Code: "maintenance", Name: "维修", Status: 1, CreatedAt: testutil.Now()}
	db.Create(&spec)
	lv := model.CourseLevel{Code: "beginner", Name: "入门", Status: 1, CreatedAt: testutil.Now()}
	db.Create(&lv)
	tpl := model.CertificateTemplate{Code: "CERT", Name: "证书", ValidityDays: 730, Status: 1, CreatedAt: testutil.Now()}
	db.Create(&tpl)

	// 前置课程
	prereq := model.Course{Name: "前置课程", Status: 1, CreatedAt: testutil.Now()}
	db.Create(&prereq)

	data := &CourseInput{
		Name:                  ptrStr("液压系统维护"),
		SpecialtyID:           ptrInt(spec.SpecialtyID),
		LevelID:               ptrInt(lv.LevelID),
		CertificateTemplateID: ptrInt(tpl.ID),
		TheoryHours:           ptrInt(30),
		PracticeHours:         ptrInt(20),
		PrerequisiteCourseIDs: []int{prereq.CourseID},
	}
	result, err := svc.CreateCourse(data)
	if err != nil {
		t.Fatalf("创建课程失败: %v", err)
	}
	courseID := result.CourseID
	if result.TheoryHours != 30 || result.PracticeHours != 20 {
		t.Fatalf("学时字段不匹配: %+v", result)
	}

	detail, err := svc.GetCourseDetail(courseID)
	if err != nil {
		t.Fatalf("获取详情失败: %v", err)
	}
	if detail.Specialty.Name != "维修" {
		t.Fatalf("专业方向元数据缺失: %+v", detail.Specialty)
	}
	if detail.Level.Name != "入门" {
		t.Fatalf("等级元数据缺失: %+v", detail.Level)
	}
	if detail.CertificateTemplate.ValidityDays != 730 {
		t.Fatalf("证书模板元数据缺失: %+v", detail.CertificateTemplate)
	}
	prereqs := *detail.Prerequisites
	if len(prereqs) != 1 || prereqs[0].Name != "前置课程" {
		t.Fatalf("前置课程元数据缺失: %+v", prereqs)
	}

	// 不存在的引用应报错
	if _, err := svc.CreateCourse(&CourseInput{Name: ptrStr("x"), SpecialtyID: ptrInt(9999)}); err == nil {
		t.Fatal("不存在的专业方向应报错")
	}
	if _, err := svc.CreateCourse(&CourseInput{Name: ptrStr("x"), TheoryHours: ptrInt(-1)}); err == nil {
		t.Fatal("负学时应报错")
	}
	if _, err := svc.CreateCourse(&CourseInput{Name: ptrStr("x"), PrerequisiteCourseIDs: []int{9999}}); err == nil {
		t.Fatal("不存在的前置课程应报错")
	}

	// 更新：等级不可清空（应用层必填，旧 category 退役后方向/等级为必备维度）
	if _, err := svc.UpdateCourse(courseID, &CourseInput{
		LevelID: ptrInt(0), PrerequisiteCourseIDs: []int{},
	}); err == nil {
		t.Fatal("清空课程等级应报错")
	}
	// 只替换前置课程（不含方向/等级字段）应成功，且前置课程被清空
	if _, err := svc.UpdateCourse(courseID, &CourseInput{PrerequisiteCourseIDs: []int{}}); err != nil {
		t.Fatalf("更新失败: %v", err)
	}
	detail, _ = svc.GetCourseDetail(courseID)
	if len(*detail.Prerequisites) != 0 {
		t.Fatal("前置课程应被清空")
	}

	// 自己作为前置课程应报错
	if _, err := svc.UpdateCourse(courseID, &CourseInput{PrerequisiteCourseIDs: []int{courseID}}); err == nil {
		t.Fatal("自引用前置课程应报错")
	}

	// 多级依赖成环应报错（C→B→A→C 与 A↔B 两课程环）
	a := model.Course{Name: "课程A", Status: 1, CreatedAt: testutil.Now()}
	b := model.Course{Name: "课程B", Status: 1, CreatedAt: testutil.Now()}
	c := model.Course{Name: "课程C", Status: 1, CreatedAt: testutil.Now()}
	db.Create(&a)
	db.Create(&b)
	db.Create(&c)
	if _, err := svc.UpdateCourse(a.CourseID, &CourseInput{PrerequisiteCourseIDs: []int{b.CourseID}}); err != nil {
		t.Fatalf("设置前置课程失败: %v", err)
	}
	if _, err := svc.UpdateCourse(b.CourseID, &CourseInput{PrerequisiteCourseIDs: []int{c.CourseID}}); err != nil {
		t.Fatalf("设置前置课程失败: %v", err)
	}
	if _, err := svc.UpdateCourse(c.CourseID, &CourseInput{PrerequisiteCourseIDs: []int{a.CourseID}}); err == nil {
		t.Fatal("多级依赖成环应报错")
	}
	if _, err := svc.UpdateCourse(b.CourseID, &CourseInput{PrerequisiteCourseIDs: []int{a.CourseID}}); err == nil {
		t.Fatal("两课程互相依赖应报错")
	}
	// 成环请求被拒绝后，原有关联应保持不变
	detail, err = svc.GetCourseDetail(b.CourseID)
	if err != nil {
		t.Fatalf("获取详情失败: %v", err)
	}
	prereqs = *detail.Prerequisites
	if len(prereqs) != 1 || prereqs[0].Name != "课程C" {
		t.Fatalf("成环拒绝后原关联应保留: %+v", prereqs)
	}
}

// --- 学员端课程详情与列表过滤 ---

func TestCourseService_TrainingFields(t *testing.T) {
	db := testutil.NewMemoryDB(t)
	svc := NewCourseService(db, nil, zap.NewNop())

	spec := model.Specialty{Code: "safety", Name: "安全", Status: 1, CreatedAt: testutil.Now()}
	db.Create(&spec)
	lv := model.CourseLevel{Code: "beginner", Name: "入门", Status: 1, CreatedAt: testutil.Now()}
	db.Create(&lv)
	course := model.Course{Name: "安全操作规范", Status: 1,
		SpecialtyID: ptrInt(spec.SpecialtyID), LevelID: ptrInt(lv.LevelID),
		TheoryHours: 12, PracticeHours: 8, CreatedAt: testutil.Now()}
	if err := db.Create(&course).Error; err != nil {
		t.Fatalf("创建课程失败: %v", err)
	}

	// 学员端列表按专业方向/等级过滤
	list := svc.GetCourses(1, 10, ptrInt(spec.SpecialtyID), ptrInt(lv.LevelID))
	if list.Total != 1 {
		t.Fatalf("过滤后应 1 条, got %v", list.Total)
	}
	empty := svc.GetCourses(1, 10, ptrInt(spec.SpecialtyID), ptrInt(9999))
	if empty.Total != 0 {
		t.Fatal("不存在的等级应过滤为空")
	}

	// 详情含等级/学时元数据
	detail, err := svc.GetCourseDetail(course.CourseID, 0)
	if err != nil {
		t.Fatalf("获取详情失败: %v", err)
	}
	info := detail.CourseInfo
	if info.TheoryHours != 12 || info.PracticeHours != 8 {
		t.Fatalf("学时不匹配: %+v", info)
	}
	if info.Level.Name != "入门" {
		t.Fatalf("等级元数据缺失: %+v", info.Level)
	}
}

// --- 题库标签查询 ---

func TestQuestionBank_Tags(t *testing.T) {
	svc, db := newCatalogSvc(t)
	qsvc := NewQuestionBankService(db, nil, zap.NewNop())

	tag1, _ := svc.CreateQuestionTag(QuestionTagInput{Code: "regulation", Name: "法规", SortOrder: ptrInt(1)})
	tag2, _ := svc.CreateQuestionTag(QuestionTagInput{Code: "hydraulic", Name: "液压", SortOrder: ptrInt(2)})

	// 创建题目时打标
	q1, err := qsvc.CreateQuestion(map[string]any{
		"type": "single_choice", "content": "法规题", "options": []string{"A", "B"}, "answer": "A",
		"tag_ids": []int{tag1.ID},
	}, nil, "tutor")
	if err != nil {
		t.Fatalf("创建题目失败: %v", err)
	}
	q2, err := qsvc.CreateQuestion(map[string]any{
		"type": "true_false", "content": "液压题", "answer": "true",
		"tag_ids": []int{tag2.ID},
	}, nil, "tutor")
	if err != nil {
		t.Fatalf("创建题目失败: %v", err)
	}
	if len(q1.Tags.([]map[string]any)) != 1 || q1.Tags.([]map[string]any)[0]["name"] != "法规" {
		t.Fatalf("创建返回的标签不匹配: %+v", q1.Tags)
	}

	// 按标签过滤
	byTag := qsvc.ListQuestions(1, 20, "", "", "", ptrInt(tag2.ID))
	if byTag["total"].(int64) != 1 {
		t.Fatalf("按标签过滤应 1 条, got %v", byTag["total"])
	}
	q := byTag["questions"].([]QuestionDTO)[0]
	if q.Content != "液压题" {
		t.Fatalf("过滤结果不匹配: %+v", q)
	}
	if len(q.Tags.([]map[string]any)) != 1 {
		t.Fatalf("列表应附带标签: %+v", q.Tags)
	}

	// 更新题目时替换标签
	updated, err := qsvc.UpdateQuestion(q1.ID, map[string]any{"tag_ids": []int{tag2.ID}})
	if err != nil {
		t.Fatalf("更新题目失败: %v", err)
	}
	if len(updated.Tags.([]map[string]any)) != 1 || updated.Tags.([]map[string]any)[0]["name"] != "液压" {
		t.Fatalf("更新后标签不匹配: %+v", updated.Tags)
	}

	// 详情含标签
	got, err := qsvc.GetQuestion(q2.ID)
	if err != nil {
		t.Fatalf("获取题目失败: %v", err)
	}
	if len(got.Tags.([]map[string]any)) != 1 {
		t.Fatalf("详情应含标签: %+v", got.Tags)
	}
}
