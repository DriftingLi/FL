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

// ListSpecialties 专业方向列表（管理端含停用项，学员端仅启用项）。
func (s *TrainingCatalogService) ListSpecialties(activeOnly bool) map[string]any {
	q := s.db.Model(&model.Specialty{})
	if activeOnly {
		q = q.Where("status = ?", 1)
	}
	var list []model.Specialty
	q.Order("sort_order ASC, specialty_id ASC").Find(&list)
	items := make([]map[string]any, 0, len(list))
	for i := range list {
		items = append(items, specialtyToDict(&list[i]))
	}
	return map[string]any{"specialties": items}
}

// CreateSpecialty 创建专业方向。
func (s *TrainingCatalogService) CreateSpecialty(data map[string]any) (map[string]any, error) {
	code, _ := data["code"].(string)
	name, _ := data["name"].(string)
	if code == "" {
		return nil, errors.New("专业方向编码不能为空")
	}
	if name == "" {
		return nil, errors.New("专业方向名称不能为空")
	}
	spec := model.Specialty{
		Code:        code,
		Name:        name,
		Description: getString(data, "description"),
		SortOrder:   toIntDefault(data["sort_order"], nextSortOrderValue(s.db, "specialty", nil)),
		Status:      int16(toIntDefault(data["status"], 1)),
		CreatedAt:   beijingNow(),
	}
	if err := s.db.Create(&spec).Error; err != nil {
		return nil, err
	}
	return specialtyToDict(&spec), nil
}

// SwapSpecialtySort 交换两个专业方向的排序位置（真实生效，含同值默认）。
func (s *TrainingCatalogService) SwapSpecialtySort(a, b int) error {
	return swapGroupPositions(s.db, &model.Specialty{}, "specialty_id", a, b, nil)
}

// UpdateSpecialty 更新专业方向。
func (s *TrainingCatalogService) UpdateSpecialty(id int, data map[string]any) (map[string]any, error) {
	var spec model.Specialty
	if err := s.db.First(&spec, id).Error; err != nil {
		return nil, errors.New("专业方向不存在")
	}
	if v, ok := data["code"].(string); ok && v != "" {
		spec.Code = v
	}
	if v, ok := data["name"].(string); ok && v != "" {
		spec.Name = v
	}
	if v, ok := data["description"]; ok {
		spec.Description, _ = v.(string)
	}
	if v, ok := data["sort_order"]; ok {
		spec.SortOrder = toIntDefault(v, spec.SortOrder)
	}
	if v, ok := data["status"]; ok {
		spec.Status = int16(toIntDefault(v, int(spec.Status)))
	}
	if err := s.db.Save(&spec).Error; err != nil {
		return nil, err
	}
	return specialtyToDict(&spec), nil
}

// DeleteSpecialty 删除专业方向（已关联课程置空 specialty_id，不级联删除课程）。
func (s *TrainingCatalogService) DeleteSpecialty(id int) error {
	result := s.db.Delete(&model.Specialty{}, id)
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return errors.New("专业方向不存在")
	}
	return nil
}

// ===== 课程等级 =====

// ListLevels 课程等级列表（activeOnly=true 仅启用项）。
func (s *TrainingCatalogService) ListLevels(activeOnly bool) map[string]any {
	q := s.db.Model(&model.CourseLevel{})
	if activeOnly {
		q = q.Where("status = ?", 1)
	}
	var list []model.CourseLevel
	q.Order("sort_order ASC, level_id ASC").Find(&list)
	items := make([]map[string]any, 0, len(list))
	for i := range list {
		items = append(items, levelToDict(&list[i]))
	}
	return map[string]any{"levels": items}
}

// CreateLevel 创建课程等级。
func (s *TrainingCatalogService) CreateLevel(data map[string]any) (map[string]any, error) {
	code, _ := data["code"].(string)
	name, _ := data["name"].(string)
	if code == "" {
		return nil, errors.New("课程等级编码不能为空")
	}
	if name == "" {
		return nil, errors.New("课程等级名称不能为空")
	}
	level := model.CourseLevel{
		Code:        code,
		Name:        name,
		Description: getString(data, "description"),
		SortOrder:   toIntDefault(data["sort_order"], nextSortOrderValue(s.db, "course_level", nil)),
		Status:      int16(toIntDefault(data["status"], 1)),
		CreatedAt:   beijingNow(),
	}
	if err := s.db.Create(&level).Error; err != nil {
		return nil, err
	}
	return levelToDict(&level), nil
}

// SwapLevelSort 交换两个课程等级的排序位置（真实生效，含同值默认）。
func (s *TrainingCatalogService) SwapLevelSort(a, b int) error {
	return swapGroupPositions(s.db, &model.CourseLevel{}, "level_id", a, b, nil)
}

// UpdateLevel 更新课程等级。
func (s *TrainingCatalogService) UpdateLevel(id int, data map[string]any) (map[string]any, error) {
	var level model.CourseLevel
	if err := s.db.First(&level, id).Error; err != nil {
		return nil, errors.New("课程等级不存在")
	}
	if v, ok := data["code"].(string); ok && v != "" {
		level.Code = v
	}
	if v, ok := data["name"].(string); ok && v != "" {
		level.Name = v
	}
	if v, ok := data["description"]; ok {
		level.Description, _ = v.(string)
	}
	if v, ok := data["sort_order"]; ok {
		level.SortOrder = toIntDefault(v, level.SortOrder)
	}
	if v, ok := data["status"]; ok {
		level.Status = int16(toIntDefault(v, int(level.Status)))
	}
	if err := s.db.Save(&level).Error; err != nil {
		return nil, err
	}
	return levelToDict(&level), nil
}

// DeleteLevel 删除课程等级（已关联课程置空 level_id，不级联删除课程）。
func (s *TrainingCatalogService) DeleteLevel(id int) error {
	result := s.db.Delete(&model.CourseLevel{}, id)
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return errors.New("课程等级不存在")
	}
	return nil
}

// ===== 证书模板 =====

// ListCertificateTemplates 证书模板列表（activeOnly=true 仅启用项）。
func (s *TrainingCatalogService) ListCertificateTemplates(activeOnly bool) map[string]any {
	q := s.db.Model(&model.CertificateTemplate{})
	if activeOnly {
		q = q.Where("status = ?", 1)
	}
	var list []model.CertificateTemplate
	q.Order("id ASC").Find(&list)
	items := make([]map[string]any, 0, len(list))
	for i := range list {
		items = append(items, certTemplateToDict(&list[i]))
	}
	return map[string]any{"certificate_templates": items}
}

// CreateCertificateTemplate 创建证书模板。
func (s *TrainingCatalogService) CreateCertificateTemplate(data map[string]any) (map[string]any, error) {
	code, _ := data["code"].(string)
	name, _ := data["name"].(string)
	if code == "" {
		return nil, errors.New("证书模板编码不能为空")
	}
	if name == "" {
		return nil, errors.New("证书模板名称不能为空")
	}
	validityDays := toIntDefault(data["validity_days"], 365)
	if validityDays <= 0 {
		return nil, errors.New("证书有效期必须为正整数（天）")
	}
	tpl := model.CertificateTemplate{
		Code:         code,
		Name:         name,
		Description:  getString(data, "description"),
		ValidityDays: validityDays,
		TemplateURL:  getString(data, "template_url"),
		Status:       int16(toIntDefault(data["status"], 1)),
		CreatedAt:    beijingNow(),
		UpdatedAt:    beijingNow(),
	}
	if err := s.db.Create(&tpl).Error; err != nil {
		return nil, err
	}
	return certTemplateToDict(&tpl), nil
}

// UpdateCertificateTemplate 更新证书模板。
func (s *TrainingCatalogService) UpdateCertificateTemplate(id int, data map[string]any) (map[string]any, error) {
	var tpl model.CertificateTemplate
	if err := s.db.First(&tpl, id).Error; err != nil {
		return nil, errors.New("证书模板不存在")
	}
	if v, ok := data["code"].(string); ok && v != "" {
		tpl.Code = v
	}
	if v, ok := data["name"].(string); ok && v != "" {
		tpl.Name = v
	}
	if v, ok := data["description"]; ok {
		tpl.Description, _ = v.(string)
	}
	if v, ok := data["validity_days"]; ok {
		days := toIntDefault(v, tpl.ValidityDays)
		if days <= 0 {
			return nil, errors.New("证书有效期必须为正整数（天）")
		}
		tpl.ValidityDays = days
	}
	if v, ok := data["template_url"]; ok {
		tpl.TemplateURL, _ = v.(string)
	}
	if v, ok := data["status"]; ok {
		tpl.Status = int16(toIntDefault(v, int(tpl.Status)))
	}
	tpl.UpdatedAt = beijingNow()
	if err := s.db.Save(&tpl).Error; err != nil {
		return nil, err
	}
	return certTemplateToDict(&tpl), nil
}

// DeleteCertificateTemplate 删除证书模板（已关联课程置空 certificate_template_id）。
func (s *TrainingCatalogService) DeleteCertificateTemplate(id int) error {
	result := s.db.Delete(&model.CertificateTemplate{}, id)
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return errors.New("证书模板不存在")
	}
	return nil
}

// ===== 题库标签 =====

// ListQuestionTags 题库标签列表（activeOnly=true 仅启用项）。
// 附带 question_count：学员端统计已发布题目数，管理端统计全部题目数。
func (s *TrainingCatalogService) ListQuestionTags(activeOnly bool) map[string]any {
	q := s.db.Model(&model.QuestionTag{})
	if activeOnly {
		q = q.Where("status = ?", 1)
	}
	var list []model.QuestionTag
	q.Order("sort_order ASC, id ASC").Find(&list)
	items := make([]map[string]any, 0, len(list))
	if len(list) == 0 {
		return map[string]any{"tags": items}
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
		d := tagToDict(&list[i])
		if c, ok := counts[list[i].ID]; ok {
			if activeOnly {
				d["question_count"] = c.PublishedCount
			} else {
				d["question_count"] = c.TotalCount
			}
		} else {
			d["question_count"] = int64(0)
		}
		items = append(items, d)
	}
	return map[string]any{"tags": items}
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
func (s *TrainingCatalogService) CreateQuestionTag(data map[string]any) (map[string]any, error) {
	code, _ := data["code"].(string)
	name, _ := data["name"].(string)
	if code == "" {
		return nil, errors.New("标签编码不能为空")
	}
	if name == "" {
		return nil, errors.New("标签名称不能为空")
	}
	var dup int64
	if err := s.db.Model(&model.QuestionTag{}).Where("code = ?", code).Count(&dup).Error; err != nil {
		return nil, err
	}
	if dup > 0 {
		return nil, errors.New("标签编码已存在")
	}
	tag := model.QuestionTag{
		Code:        code,
		Name:        name,
		Description: getString(data, "description"),
		SortOrder:   toIntDefault(data["sort_order"], nextSortOrderValue(s.db, "question_tag", nil)),
		Status:      int16(toIntDefault(data["status"], 1)),
		CreatedAt:   beijingNow(),
		UpdatedAt:   beijingNow(),
	}
	if err := s.db.Create(&tag).Error; err != nil {
		return nil, err
	}
	return tagToDict(&tag), nil
}

// UpdateQuestionTag 更新题库标签。
func (s *TrainingCatalogService) UpdateQuestionTag(id int, data map[string]any) (map[string]any, error) {
	var tag model.QuestionTag
	if err := s.db.First(&tag, id).Error; err != nil {
		return nil, errors.New("题库标签不存在")
	}
	if v, ok := data["code"].(string); ok && v != "" {
		if v != tag.Code {
			var dup int64
			if err := s.db.Model(&model.QuestionTag{}).Where("code = ? AND id <> ?", v, id).Count(&dup).Error; err != nil {
				return nil, err
			}
			if dup > 0 {
				return nil, errors.New("标签编码已存在")
			}
		}
		tag.Code = v
	}
	if v, ok := data["name"].(string); ok && v != "" {
		tag.Name = v
	}
	if v, ok := data["description"]; ok {
		tag.Description, _ = v.(string)
	}
	if v, ok := data["sort_order"]; ok {
		tag.SortOrder = toIntDefault(v, tag.SortOrder)
	}
	if v, ok := data["status"]; ok {
		tag.Status = int16(toIntDefault(v, int(tag.Status)))
	}
	tag.UpdatedAt = beijingNow()
	if err := s.db.Save(&tag).Error; err != nil {
		return nil, err
	}
	return tagToDict(&tag), nil
}

// DeleteQuestionTag 删除题库标签（自动清理题目关联）。
func (s *TrainingCatalogService) DeleteQuestionTag(id int) error {
	result := s.db.Delete(&model.QuestionTag{}, id)
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return errors.New("题库标签不存在")
	}
	return nil
}

// ===== 题目-标签关联 =====

// GetQuestionTags 查询题目已关联的标签列表。
func (s *TrainingCatalogService) GetQuestionTags(questionID int) (map[string]any, error) {
	var q model.Question
	if err := s.db.First(&q, questionID).Error; err != nil {
		return nil, errors.New("题目不存在")
	}
	return map[string]any{"tags": s.loadQuestionTags(questionID)}, nil
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
func (s *TrainingCatalogService) GetCatalogTree() map[string]any {
	return s.getCatalogTree(true, false)
}

// GetAdminCatalogTree 目录树（管理端）：专业方向 → 等级 → 课程 → 章节。
// 含停用项与全部课程，课程节点附带章节列表（章节拖拽排序用 order_num）。
func (s *TrainingCatalogService) GetAdminCatalogTree() map[string]any {
	return s.getCatalogTree(false, true)
}

// getCatalogTree 构建目录树。
// activeOnly=true 时仅返回启用项（学员端）；withChapters=true 时课程节点附带章节列表（管理端）。
func (s *TrainingCatalogService) getCatalogTree(activeOnly, withChapters bool) map[string]any {
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
	var chaptersByCourse map[int][]map[string]any
	if withChapters {
		var chapters []model.Chapter
		s.db.Order("order_num ASC, chapter_id ASC").Find(&chapters)
		chaptersByCourse = make(map[int][]map[string]any, len(chapters))
		for i := range chapters {
			chaptersByCourse[chapters[i].CourseID] = append(chaptersByCourse[chapters[i].CourseID], chapterToDict(&chapters[i]))
		}
	}

	tree := make([]map[string]any, 0, len(specialties))
	for i := range specialties {
		spec := specialtyToDict(&specialties[i])
		levelItems := make([]map[string]any, 0, len(levels))
		for j := range levels {
			lv := levelToDict(&levels[j])
			courses := make([]map[string]any, 0)
			for k := range rows {
				if rows[k].SpecialtyID == nil || rows[k].LevelID == nil {
					continue
				}
				if *rows[k].SpecialtyID != specialties[i].SpecialtyID || *rows[k].LevelID != levels[j].LevelID {
					continue
				}
				cd := courseToDict(&rows[k].Course)
				cd["chapter_count"] = rows[k].ChapterCount
				if withChapters {
					fillPrereqIDs(s.db, rows[k].CourseID, cd)
					if chs, ok := chaptersByCourse[rows[k].CourseID]; ok {
						cd["chapters"] = chs
					} else {
						cd["chapters"] = []map[string]any{}
					}
				}
				courses = append(courses, cd)
			}
			lv["courses"] = courses
			levelItems = append(levelItems, lv)
		}
		spec["levels"] = levelItems
		tree = append(tree, spec)
	}
	return map[string]any{"specialties": tree}
}

// ===== 辅助 =====

// loadQuestionTags 加载单题标签列表。
func (s *TrainingCatalogService) loadQuestionTags(questionID int) []map[string]any {
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
		return []map[string]any{}
	}
	out := make([]map[string]any, 0, len(rows))
	for i := range rows {
		out = append(out, map[string]any{
			"id":         rows[i].TagID,
			"code":       rows[i].TagCode,
			"name":       rows[i].TagName,
			"sort_order": rows[i].SortOrder,
			"status":     rows[i].Status,
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

// ===== dict 辅助 =====

func specialtyToDict(s *model.Specialty) map[string]any {
	return map[string]any{
		"specialty_id": s.SpecialtyID,
		"code":         s.Code,
		"name":         s.Name,
		"description":  s.Description,
		"sort_order":   s.SortOrder,
		"status":       s.Status,
		"created_at":   formatISO(s.CreatedAt),
	}
}

func levelToDict(l *model.CourseLevel) map[string]any {
	return map[string]any{
		"level_id":    l.LevelID,
		"code":        l.Code,
		"name":        l.Name,
		"description": l.Description,
		"sort_order":  l.SortOrder,
		"status":      l.Status,
		"created_at":  formatISO(l.CreatedAt),
	}
}

func certTemplateToDict(t *model.CertificateTemplate) map[string]any {
	return map[string]any{
		"id":            t.ID,
		"code":          t.Code,
		"name":          t.Name,
		"description":   t.Description,
		"validity_days": t.ValidityDays,
		"template_url":  t.TemplateURL,
		"status":        t.Status,
		"created_at":    formatISO(t.CreatedAt),
		"updated_at":    formatISO(t.UpdatedAt),
	}
}

func tagToDict(t *model.QuestionTag) map[string]any {
	return map[string]any{
		"id":          t.ID,
		"code":        t.Code,
		"name":        t.Name,
		"description": t.Description,
		"sort_order":  t.SortOrder,
		"status":      t.Status,
		"created_at":  formatISO(t.CreatedAt),
		"updated_at":  formatISO(t.UpdatedAt),
	}
}
