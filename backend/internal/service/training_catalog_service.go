// Package service 培训目录服务：专业方向 / 课程等级 / 证书模板 / 题库标签。
package service

import (
	"errors"

	"go.uber.org/zap"
	"gorm.io/gorm"

	"forklift-training/internal/model"
)

// TrainingCatalogService 培训目录（课程目录树与管理数据）服务。
type TrainingCatalogService struct {
	db *gorm.DB

	logger *zap.Logger
}

// NewTrainingCatalogService 创建培训目录服务实例。
func NewTrainingCatalogService(db *gorm.DB, logger *zap.Logger) *TrainingCatalogService {
	return &TrainingCatalogService{db: db, logger: logger}
}

// ===== 专业方向 =====

// nextSortOrderValue 返回表内（可选按组过滤）当前最大 sort_order + 1，新项排末尾。
func nextSortOrderValue(db *gorm.DB, table string, where map[string]any) int {
	q := db.Table(table).Select("COALESCE(MAX(sort_order), 0)")
	for k, v := range where {
		q = q.Where(k+" = ?", v)
	}
	var max int
	if err := q.Scan(&max).Error; err != nil {
		return 1
	}
	return max + 1
}

// renumberSortGroup 按 (sort_order, id) 升序把组内全部项重新顺序编号（1..N），返回新序 ID 列表。
// 消除同值 sort_order（默认 0）导致的顺序不可控。
func renumberSortGroup(db *gorm.DB, entity any, idCol string, where map[string]any) ([]int, error) {
	var rows []map[string]any
	q := db.Model(entity).Select(idCol + ", sort_order")
	for k, v := range where {
		q = q.Where(k+" = ?", v)
	}
	q.Order("sort_order ASC, " + idCol + " ASC")
	if err := q.Find(&rows).Error; err != nil {
		return nil, err
	}
	ids := make([]int, 0, len(rows))
	for _, r := range rows {
		ids = append(ids, toInt(r[idCol]))
	}
	return ids, nil
}

// swapGroupPositions 把组内两项交换位置：重编号后交换 a/b 在新序中的下标，再整体落库。
// 即使两项 sort_order 相同（默认 0）也真实生效。
func swapGroupPositions(db *gorm.DB, entity any, idCol string, idA, idB int, where map[string]any) error {
	ids, err := renumberSortGroup(db, entity, idCol, where)
	if err != nil {
		return err
	}
	ia, ib := -1, -1
	for i, id := range ids {
		if id == idA {
			ia = i
		}
		if id == idB {
			ib = i
		}
	}
	if ia < 0 || ib < 0 {
		return errors.New("待交换的项不存在")
	}
	if ia == ib {
		return nil
	}
	ids[ia], ids[ib] = ids[ib], ids[ia]
	return db.Transaction(func(tx *gorm.DB) error {
		for i, id := range ids {
			if err := tx.Model(entity).Where(idCol+" = ?", id).Update("sort_order", i+1).Error; err != nil {
				return err
			}
		}
		return nil
	})
}

// validateStatus 校验状态枚举（0 停用 / 1 启用），nil 视为未提供。
func validateStatus(status *int16) error {
	if status == nil {
		return nil
	}
	if *status != 0 && *status != 1 {
		return errors.New("状态值无效")
	}
	return nil
}

// countByCode 统计表中同编码记录数（excludeID>0 时排除自身，供更新撞码校验）。
func countByCode(db *gorm.DB, table, idCol, code string, excludeID int) (int64, error) {
	q := db.Table(table).Where("code = ?", code)
	if excludeID > 0 {
		q = q.Where(idCol+" <> ?", excludeID)
	}
	var n int64
	if err := q.Count(&n).Error; err != nil {
		return 0, err
	}
	return n, nil
}

// ===== 目录实体 CRUD 轻量共享骨架 =====
//
// 四个目录实体（专业方向/课程等级/证书模板/题库标签）的 CRUD 同构：
// 必填校验、撞码检查、删除、排序交换共享骨架，各实体保留字段映射薄入口。

// validateCatalogCodeName 必填 code/name 校验。
func validateCatalogCodeName(code, name, codeErr, nameErr string) error {
	if code == "" {
		return errors.New(codeErr)
	}
	if name == "" {
		return errors.New(nameErr)
	}
	return nil
}

// ensureCodeUnique 撞码检查（excludeID>0 时排除自身，供更新校验）。
func ensureCodeUnique(db *gorm.DB, table, idCol, code string, excludeID int, dupMsg string) error {
	dup, err := countByCode(db, table, idCol, code, excludeID)
	if err != nil {
		return err
	}
	if dup > 0 {
		return errors.New(dupMsg)
	}
	return nil
}

// deleteCatalogEntity 删除骨架：记录不存在返回指定文案。
func deleteCatalogEntity(db *gorm.DB, model interface{}, id int, notFoundMsg string) error {
	result := db.Delete(model, id)
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return errors.New(notFoundMsg)
	}
	return nil
}

// ListSpecialties 专业方向列表（管理端含停用项，学员端仅启用项）。
func (s *TrainingCatalogService) ListSpecialties(activeOnly bool) []SpecialtyDict {
	q := s.db.Model(&model.Specialty{})
	if activeOnly {
		q = q.Where("status = ?", 1)
	}
	var list []model.Specialty
	q.Order("sort_order ASC, specialty_id ASC").Find(&list)
	items := make([]SpecialtyDict, 0, len(list))
	for i := range list {
		items = append(items, specialtyDict(&list[i]))
	}
	return items
}

// CreateSpecialty 创建专业方向。
func (s *TrainingCatalogService) CreateSpecialty(in SpecialtyInput) (SpecialtyDict, error) {
	if err := validateCatalogCodeName(in.Code, in.Name, "专业方向编码不能为空", "专业方向名称不能为空"); err != nil {
		return SpecialtyDict{}, err
	}
	if err := validateStatus(in.Status); err != nil {
		return SpecialtyDict{}, err
	}
	if err := ensureCodeUnique(s.db, "specialty", "specialty_id", in.Code, 0, "专业方向编码已存在"); err != nil {
		return SpecialtyDict{}, err
	}
	spec := model.Specialty{
		Code:        in.Code,
		Name:        in.Name,
		Description: inputString(in.Description),
		SortOrder:   inputInt(in.SortOrder, nextSortOrderValue(s.db, "specialty", nil)),
		Status:      inputInt16(in.Status, 1),
		CreatedAt:   beijingNow(),
	}
	if err := s.db.Create(&spec).Error; err != nil {
		return SpecialtyDict{}, err
	}
	return specialtyDict(&spec), nil
}

// SwapSpecialtySort 交换两个专业方向的排序位置（真实生效，含同值默认）。
func (s *TrainingCatalogService) SwapSpecialtySort(a, b int) error {
	return swapGroupPositions(s.db, &model.Specialty{}, "specialty_id", a, b, nil)
}

// UpdateSpecialty 更新专业方向。
func (s *TrainingCatalogService) UpdateSpecialty(id int, in SpecialtyInput) (SpecialtyDict, error) {
	var spec model.Specialty
	if err := s.db.First(&spec, id).Error; err != nil {
		return SpecialtyDict{}, errors.New("专业方向不存在")
	}
	if err := validateStatus(in.Status); err != nil {
		return SpecialtyDict{}, err
	}
	if in.Code != "" {
		if in.Code != spec.Code {
			if err := ensureCodeUnique(s.db, "specialty", "specialty_id", in.Code, id, "专业方向编码已存在"); err != nil {
				return SpecialtyDict{}, err
			}
		}
		spec.Code = in.Code
	}
	if in.Name != "" {
		spec.Name = in.Name
	}
	if in.Description != nil {
		spec.Description = *in.Description
	}
	if in.SortOrder != nil {
		spec.SortOrder = *in.SortOrder
	}
	if in.Status != nil {
		spec.Status = *in.Status
	}
	if err := s.db.Save(&spec).Error; err != nil {
		return SpecialtyDict{}, err
	}
	return specialtyDict(&spec), nil
}

// DeleteSpecialty 删除专业方向（已关联课程置空 specialty_id，不级联删除课程）。
func (s *TrainingCatalogService) DeleteSpecialty(id int) error {
	return deleteCatalogEntity(s.db, &model.Specialty{}, id, "专业方向不存在")
}

// ===== 课程等级 =====

// ListLevels 课程等级列表（activeOnly=true 仅启用项）。
func (s *TrainingCatalogService) ListLevels(activeOnly bool) []LevelDict {
	q := s.db.Model(&model.CourseLevel{})
	if activeOnly {
		q = q.Where("status = ?", 1)
	}
	var list []model.CourseLevel
	q.Order("sort_order ASC, level_id ASC").Find(&list)
	items := make([]LevelDict, 0, len(list))
	for i := range list {
		items = append(items, levelDict(&list[i]))
	}
	return items
}

// CreateLevel 创建课程等级。
func (s *TrainingCatalogService) CreateLevel(in LevelInput) (LevelDict, error) {
	if err := validateCatalogCodeName(in.Code, in.Name, "课程等级编码不能为空", "课程等级名称不能为空"); err != nil {
		return LevelDict{}, err
	}
	if err := validateStatus(in.Status); err != nil {
		return LevelDict{}, err
	}
	if err := ensureCodeUnique(s.db, "course_level", "level_id", in.Code, 0, "课程等级编码已存在"); err != nil {
		return LevelDict{}, err
	}
	level := model.CourseLevel{
		Code:        in.Code,
		Name:        in.Name,
		Description: inputString(in.Description),
		SortOrder:   inputInt(in.SortOrder, nextSortOrderValue(s.db, "course_level", nil)),
		Status:      inputInt16(in.Status, 1),
		CreatedAt:   beijingNow(),
	}
	if err := s.db.Create(&level).Error; err != nil {
		return LevelDict{}, err
	}
	return levelDict(&level), nil
}

// SwapLevelSort 交换两个课程等级的排序位置（真实生效，含同值默认）。
func (s *TrainingCatalogService) SwapLevelSort(a, b int) error {
	return swapGroupPositions(s.db, &model.CourseLevel{}, "level_id", a, b, nil)
}

// UpdateLevel 更新课程等级。
func (s *TrainingCatalogService) UpdateLevel(id int, in LevelInput) (LevelDict, error) {
	var level model.CourseLevel
	if err := s.db.First(&level, id).Error; err != nil {
		return LevelDict{}, errors.New("课程等级不存在")
	}
	if err := validateStatus(in.Status); err != nil {
		return LevelDict{}, err
	}
	if in.Code != "" {
		if in.Code != level.Code {
			if err := ensureCodeUnique(s.db, "course_level", "level_id", in.Code, id, "课程等级编码已存在"); err != nil {
				return LevelDict{}, err
			}
		}
		level.Code = in.Code
	}
	if in.Name != "" {
		level.Name = in.Name
	}
	if in.Description != nil {
		level.Description = *in.Description
	}
	if in.SortOrder != nil {
		level.SortOrder = *in.SortOrder
	}
	if in.Status != nil {
		level.Status = *in.Status
	}
	if err := s.db.Save(&level).Error; err != nil {
		return LevelDict{}, err
	}
	return levelDict(&level), nil
}

// DeleteLevel 删除课程等级（已关联课程置空 level_id，不级联删除课程）。
func (s *TrainingCatalogService) DeleteLevel(id int) error {
	return deleteCatalogEntity(s.db, &model.CourseLevel{}, id, "课程等级不存在")
}

// ===== 证书模板 =====

// ListCertificateTemplates 证书模板列表（activeOnly=true 仅启用项）。
func (s *TrainingCatalogService) ListCertificateTemplates(activeOnly bool) []CertificateTemplateDict {
	q := s.db.Model(&model.CertificateTemplate{})
	if activeOnly {
		q = q.Where("status = ?", 1)
	}
	var list []model.CertificateTemplate
	q.Order("id ASC").Find(&list)
	items := make([]CertificateTemplateDict, 0, len(list))
	for i := range list {
		items = append(items, certTemplateDict(&list[i]))
	}
	return items
}

// CreateCertificateTemplate 创建证书模板。
func (s *TrainingCatalogService) CreateCertificateTemplate(in CertificateTemplateInput) (CertificateTemplateDict, error) {
	if err := validateCatalogCodeName(in.Code, in.Name, "证书模板编码不能为空", "证书模板名称不能为空"); err != nil {
		return CertificateTemplateDict{}, err
	}
	if err := validateStatus(in.Status); err != nil {
		return CertificateTemplateDict{}, err
	}
	validityDays := inputInt(in.ValidityDays, 365)
	if validityDays <= 0 {
		return CertificateTemplateDict{}, errors.New("证书有效期必须为正整数（天）")
	}
	if err := ensureCodeUnique(s.db, "certificate_template", "id", in.Code, 0, "证书模板编码已存在"); err != nil {
		return CertificateTemplateDict{}, err
	}
	tpl := model.CertificateTemplate{
		Code:         in.Code,
		Name:         in.Name,
		Description:  inputString(in.Description),
		ValidityDays: validityDays,
		TemplateURL:  inputString(in.TemplateURL),
		Status:       inputInt16(in.Status, 1),
		CreatedAt:    beijingNow(),
		UpdatedAt:    beijingNow(),
	}
	if err := s.db.Create(&tpl).Error; err != nil {
		return CertificateTemplateDict{}, err
	}
	return certTemplateDict(&tpl), nil
}

// UpdateCertificateTemplate 更新证书模板。
func (s *TrainingCatalogService) UpdateCertificateTemplate(id int, in CertificateTemplateInput) (CertificateTemplateDict, error) {
	var tpl model.CertificateTemplate
	if err := s.db.First(&tpl, id).Error; err != nil {
		return CertificateTemplateDict{}, errors.New("证书模板不存在")
	}
	if err := validateStatus(in.Status); err != nil {
		return CertificateTemplateDict{}, err
	}
	if in.Code != "" {
		if in.Code != tpl.Code {
			if err := ensureCodeUnique(s.db, "certificate_template", "id", in.Code, id, "证书模板编码已存在"); err != nil {
				return CertificateTemplateDict{}, err
			}
		}
		tpl.Code = in.Code
	}
	if in.Name != "" {
		tpl.Name = in.Name
	}
	if in.Description != nil {
		tpl.Description = *in.Description
	}
	if in.ValidityDays != nil {
		if *in.ValidityDays <= 0 {
			return CertificateTemplateDict{}, errors.New("证书有效期必须为正整数（天）")
		}
		tpl.ValidityDays = *in.ValidityDays
	}
	if in.TemplateURL != nil {
		tpl.TemplateURL = *in.TemplateURL
	}
	if in.Status != nil {
		tpl.Status = *in.Status
	}
	tpl.UpdatedAt = beijingNow()
	if err := s.db.Save(&tpl).Error; err != nil {
		return CertificateTemplateDict{}, err
	}
	return certTemplateDict(&tpl), nil
}

// DeleteCertificateTemplate 删除证书模板（已关联课程置空 certificate_template_id）。
func (s *TrainingCatalogService) DeleteCertificateTemplate(id int) error {
	return deleteCatalogEntity(s.db, &model.CertificateTemplate{}, id, "证书模板不存在")
}

// ===== 题库标签 =====

// ListQuestionTags 题库标签列表（activeOnly=true 仅启用项）。
// 附带 question_count：学员端统计已发布题目数，管理端统计全部题目数。
func (s *TrainingCatalogService) ListQuestionTags(activeOnly bool) []QuestionTagDict {
	q := s.db.Model(&model.QuestionTag{})
	if activeOnly {
		q = q.Where("status = ?", 1)
	}
	var list []model.QuestionTag
	q.Order("sort_order ASC, id ASC").Find(&list)
	items := make([]QuestionTagDict, 0, len(list))
	if len(list) == 0 {
		return items
	}
	// 一次查询全部标签的题目数（LEFT JOIN 保证无题目标签也返回 0，避免 N+1）
	type countRow struct {
		TagID          int
		TotalCount     int64
		PublishedCount int64
	}
	var rows []countRow
	s.db.Table("question_tag AS t").
		Select("t.id AS tag_id, COUNT(qtr.question_id) AS total_count, "+
			"COUNT(qtr.question_id) FILTER (WHERE q.status = 'published') AS published_count").
		Joins("LEFT JOIN question_tag_relation AS qtr ON qtr.tag_id = t.id").
		Joins("LEFT JOIN question AS q ON q.id = qtr.question_id").
		Where("t.id IN ?", idsOfTags(list)).
		Group("t.id").
		Scan(&rows)
	counts := make(map[int]countRow, len(rows))
	for i := range rows {
		counts[rows[i].TagID] = rows[i]
	}
	for i := range list {
		d := tagDict(&list[i])
		var count int64
		if c, ok := counts[list[i].ID]; ok {
			if activeOnly {
				count = c.PublishedCount
			} else {
				count = c.TotalCount
			}
		}
		d.QuestionCount = &count
		items = append(items, d)
	}
	return items
}

// idsOfTags 提取标签 ID 列表。
func idsOfTags(tags []model.QuestionTag) []int {
	ids := make([]int, len(tags))
	for i := range tags {
		ids[i] = tags[i].ID
	}
	return ids
}

// CreateQuestionTag 创建题库标签。
func (s *TrainingCatalogService) CreateQuestionTag(in QuestionTagInput) (QuestionTagDict, error) {
	if err := validateCatalogCodeName(in.Code, in.Name, "标签编码不能为空", "标签名称不能为空"); err != nil {
		return QuestionTagDict{}, err
	}
	if err := validateStatus(in.Status); err != nil {
		return QuestionTagDict{}, err
	}
	if err := ensureCodeUnique(s.db, "question_tag", "id", in.Code, 0, "标签编码已存在"); err != nil {
		return QuestionTagDict{}, err
	}
	tag := model.QuestionTag{
		Code:        in.Code,
		Name:        in.Name,
		Description: inputString(in.Description),
		SortOrder:   inputInt(in.SortOrder, nextSortOrderValue(s.db, "question_tag", nil)),
		Status:      inputInt16(in.Status, 1),
		CreatedAt:   beijingNow(),
		UpdatedAt:   beijingNow(),
	}
	if err := s.db.Create(&tag).Error; err != nil {
		return QuestionTagDict{}, err
	}
	return tagDict(&tag), nil
}

// UpdateQuestionTag 更新题库标签。
func (s *TrainingCatalogService) UpdateQuestionTag(id int, in QuestionTagInput) (QuestionTagDict, error) {
	var tag model.QuestionTag
	if err := s.db.First(&tag, id).Error; err != nil {
		return QuestionTagDict{}, errors.New("题库标签不存在")
	}
	if err := validateStatus(in.Status); err != nil {
		return QuestionTagDict{}, err
	}
	if in.Code != "" && in.Code != tag.Code {
		if err := ensureCodeUnique(s.db, "question_tag", "id", in.Code, id, "标签编码已存在"); err != nil {
			return QuestionTagDict{}, err
		}
		tag.Code = in.Code
	}
	if in.Name != "" {
		tag.Name = in.Name
	}
	if in.Description != nil {
		tag.Description = *in.Description
	}
	if in.SortOrder != nil {
		tag.SortOrder = *in.SortOrder
	}
	if in.Status != nil {
		tag.Status = *in.Status
	}
	tag.UpdatedAt = beijingNow()
	if err := s.db.Save(&tag).Error; err != nil {
		return QuestionTagDict{}, err
	}
	return tagDict(&tag), nil
}

// DeleteQuestionTag 删除题库标签（自动清理题目关联）。
func (s *TrainingCatalogService) DeleteQuestionTag(id int) error {
	return deleteCatalogEntity(s.db, &model.QuestionTag{}, id, "题库标签不存在")
}

// ===== 题目-标签关联 =====

// GetQuestionTags 查询题目已关联的标签列表。
func (s *TrainingCatalogService) GetQuestionTags(questionID int) ([]QuestionTagRef, error) {
	var q model.Question
	if err := s.db.First(&q, questionID).Error; err != nil {
		return nil, errors.New("题目不存在")
	}
	return s.loadQuestionTags(questionID), nil
}

// SetQuestionTags 全量替换题目标签关联。
func (s *TrainingCatalogService) SetQuestionTags(questionID int, tagIDs []int) error {
	var q model.Question
	if err := s.db.First(&q, questionID).Error; err != nil {
		return errors.New("题目不存在")
	}
	return replaceQuestionTags(s.db, questionID, tagIDs)
}

// replaceQuestionTags 全量替换题目标签关联（校验标签存在）。
func replaceQuestionTags(db *gorm.DB, questionID int, tagIDs []int) error {
	tagIDs = dedupeInts(tagIDs)
	if len(tagIDs) > 0 {
		var count int64
		if err := db.Model(&model.QuestionTag{}).Where("id IN ?", tagIDs).Count(&count).Error; err != nil {
			return err
		}
		if count != int64(len(tagIDs)) {
			return errors.New("包含不存在的标签")
		}
	}
	if len(tagIDs) == 0 {
		return db.Where("question_id = ?", questionID).
			Delete(&model.QuestionTagRelation{}).Error
	}
	rels := make([]model.QuestionTagRelation, 0, len(tagIDs))
	for _, tagID := range tagIDs {
		rels = append(rels, model.QuestionTagRelation{
			QuestionID: questionID,
			TagID:      tagID,
			CreatedAt:  beijingNow(),
		})
	}
	return db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("question_id = ?", questionID).
			Delete(&model.QuestionTagRelation{}).Error; err != nil {
			return err
		}
		return tx.Create(&rels).Error
	})
}

// ===== 目录树（学员端） =====

// GetCatalogTree 目录树（学员端）：专业方向 → 等级 → 课程（仅启用项，课程含章节数）。
func (s *TrainingCatalogService) GetCatalogTree() *CatalogTreeDTO {
	return s.getCatalogTree(true, false)
}

// GetAdminCatalogTree 目录树（管理端）：专业方向 → 等级 → 课程 → 章节。
// 含停用项与全部课程，课程节点附带章节列表（章节拖拽排序用 order_num）。
func (s *TrainingCatalogService) GetAdminCatalogTree() *CatalogTreeDTO {
	return s.getCatalogTree(false, true)
}

// getCatalogTree 构建目录树。
// activeOnly=true 时仅返回启用项（学员端）；withChapters=true 时课程节点附带章节列表（管理端）。
// 节点字段按 key 字母序声明，与旧 map 投影字节序一致（shape-lock 测试锁定）。
func (s *TrainingCatalogService) getCatalogTree(activeOnly, withChapters bool) *CatalogTreeDTO {
	var specialties []model.Specialty
	{
		q := s.db.Model(&model.Specialty{})
		if activeOnly {
			q = q.Where("status = ?", 1)
		}
		q.Order("sort_order ASC, specialty_id ASC").Find(&specialties)
	}

	var levels []model.CourseLevel
	{
		q := s.db.Model(&model.CourseLevel{})
		if activeOnly {
			q = q.Where("status = ?", 1)
		}
		q.Order("sort_order ASC, level_id ASC").Find(&levels)
	}

	// 一次查询全部课程及其章节数（避免逐门查询的 N+1）
	type courseRow struct {
		model.Course
		ChapterCount int64
	}
	var rows []courseRow
	{
		q := s.db.Model(&model.Course{}).
			Select("course.*, (SELECT COUNT(*) FROM chapter WHERE chapter.course_id = course.course_id) AS chapter_count")
		if activeOnly {
			q = q.Where("course.status = ?", 1)
		}
		q.Order("course.sort_order ASC, course.course_id ASC").Find(&rows)
	}

	// 管理端：一次性加载全部章节，按课程分组（避免逐课程查询的 N+1）
	var chaptersByCourse map[int][]ChapterDTO
	if withChapters {
		var chapters []model.Chapter
		s.db.Order("order_num ASC, chapter_id ASC").Find(&chapters)
		chaptersByCourse = make(map[int][]ChapterDTO, len(chapters))
		for i := range chapters {
			chaptersByCourse[chapters[i].CourseID] = append(chaptersByCourse[chapters[i].CourseID], chapterToDTO(&chapters[i]))
		}
	}

	tree := make([]CatalogSpecialtyNode, 0, len(specialties))
	for i := range specialties {
		spec := newCatalogSpecialtyNode(&specialties[i])
		levelItems := make([]CatalogLevelNode, 0, len(levels))
		for j := range levels {
			lv := newCatalogLevelNode(&levels[j])
			courses := make([]CourseDTO, 0)
			for k := range rows {
				if !courseMounted(rows[k].SpecialtyID, rows[k].LevelID) {
					continue
				}
				if *rows[k].SpecialtyID != specialties[i].SpecialtyID || *rows[k].LevelID != levels[j].LevelID {
					continue
				}
				cd := courseToDTO(&rows[k].Course)
				cd.ChapterCount = &rows[k].ChapterCount
				if withChapters {
					fillPrereqIDs(s.db, rows[k].CourseID, &cd)
					if chs, ok := chaptersByCourse[rows[k].CourseID]; ok {
						cd.Chapters = &chs
					} else {
						empty := []ChapterDTO{}
						cd.Chapters = &empty
					}
				}
				courses = append(courses, cd)
			}
			lv.Courses = courses
			levelItems = append(levelItems, lv)
		}
		spec.Levels = levelItems
		tree = append(tree, spec)
	}
	return &CatalogTreeDTO{Specialties: tree}
}

// ===== 辅助 =====

// loadQuestionTags 加载单题标签列表（题目-标签关联摘要）。
func (s *TrainingCatalogService) loadQuestionTags(questionID int) []QuestionTagRef {
	var rows []struct {
		TagID     int    `gorm:"column:tag_id"`
		TagCode   string `gorm:"column:tag_code"`
		TagName   string `gorm:"column:tag_name"`
		SortOrder int    `gorm:"column:tag_sort"`
		Status    int16  `gorm:"column:tag_status"`
	}
	if err := s.db.Table("question_tag_relation AS qtr").
		Select("qtr.tag_id, t.code AS tag_code, t.name AS tag_name, t.sort_order AS tag_sort, t.status AS tag_status").
		Joins("JOIN question_tag AS t ON t.id = qtr.tag_id").
		Where("qtr.question_id = ?", questionID).
		Order("t.sort_order ASC, t.id ASC").
		Scan(&rows).Error; err != nil {
		return []QuestionTagRef{}
	}
	out := make([]QuestionTagRef, 0, len(rows))
	for i := range rows {
		out = append(out, QuestionTagRef{
			ID:        rows[i].TagID,
			Code:      rows[i].TagCode,
			Name:      rows[i].TagName,
			SortOrder: rows[i].SortOrder,
			Status:    rows[i].Status,
		})
	}
	return out
}

// dedupeInts 去重并保持顺序。
func dedupeInts(vals []int) []int {
	seen := make(map[int]struct{}, len(vals))
	out := make([]int, 0, len(vals))
	for _, v := range vals {
		if _, ok := seen[v]; ok {
			continue
		}
		seen[v] = struct{}{}
		out = append(out, v)
	}
	return out
}

// ===== typed 字典辅助 =====

func specialtyDict(s *model.Specialty) SpecialtyDict {
	return SpecialtyDict{
		Code:        s.Code,
		CreatedAt:   formatISO(s.CreatedAt),
		Description: s.Description,
		Name:        s.Name,
		SortOrder:   s.SortOrder,
		SpecialtyID: s.SpecialtyID,
		Status:      s.Status,
	}
}

func levelDict(l *model.CourseLevel) LevelDict {
	return LevelDict{
		Code:        l.Code,
		CreatedAt:   formatISO(l.CreatedAt),
		Description: l.Description,
		LevelID:     l.LevelID,
		Name:        l.Name,
		SortOrder:   l.SortOrder,
		Status:      l.Status,
	}
}

func certTemplateDict(t *model.CertificateTemplate) CertificateTemplateDict {
	return CertificateTemplateDict{
		Code:         t.Code,
		CreatedAt:    formatISO(t.CreatedAt),
		Description:  t.Description,
		ID:           t.ID,
		Name:         t.Name,
		Status:       t.Status,
		TemplateURL:  t.TemplateURL,
		UpdatedAt:    formatISO(t.UpdatedAt),
		ValidityDays: t.ValidityDays,
	}
}

func tagDict(t *model.QuestionTag) QuestionTagDict {
	return QuestionTagDict{
		Code:        t.Code,
		CreatedAt:   formatISO(t.CreatedAt),
		Description: t.Description,
		ID:          t.ID,
		Name:        t.Name,
		SortOrder:   t.SortOrder,
		Status:      t.Status,
		UpdatedAt:   formatISO(t.UpdatedAt),
	}
}

// ===== 目录树节点（typed 投影，字段按 key 字母序与旧 map 字节序一致）=====

// CatalogTreeDTO 目录树响应契约。
type CatalogTreeDTO struct {
	Specialties []CatalogSpecialtyNode `json:"specialties"`
}

// CatalogSpecialtyNode 目录树专业方向节点。
type CatalogSpecialtyNode struct {
	Code        string             `json:"code"`
	CreatedAt   string             `json:"created_at"`
	Description string             `json:"description"`
	Levels      []CatalogLevelNode `json:"levels"`
	Name        string             `json:"name"`
	SortOrder   int                `json:"sort_order"`
	SpecialtyID int                `json:"specialty_id"`
	Status      int16              `json:"status"`
}

// CatalogLevelNode 目录树课程等级节点。
type CatalogLevelNode struct {
	Code        string      `json:"code"`
	Courses     []CourseDTO `json:"courses"`
	CreatedAt   string      `json:"created_at"`
	Description string      `json:"description"`
	LevelID     int         `json:"level_id"`
	Name        string      `json:"name"`
	SortOrder   int         `json:"sort_order"`
	Status      int16       `json:"status"`
}

// newCatalogSpecialtyNode 构造目录树专业方向节点。
func newCatalogSpecialtyNode(s *model.Specialty) CatalogSpecialtyNode {
	return CatalogSpecialtyNode{
		Code:        s.Code,
		CreatedAt:   formatISO(s.CreatedAt),
		Description: s.Description,
		Name:        s.Name,
		SortOrder:   s.SortOrder,
		SpecialtyID: s.SpecialtyID,
		Status:      s.Status,
	}
}

// newCatalogLevelNode 构造目录树课程等级节点。
func newCatalogLevelNode(l *model.CourseLevel) CatalogLevelNode {
	return CatalogLevelNode{
		Code:        l.Code,
		CreatedAt:   formatISO(l.CreatedAt),
		Description: l.Description,
		LevelID:     l.LevelID,
		Name:        l.Name,
		SortOrder:   l.SortOrder,
		Status:      l.Status,
	}
}
