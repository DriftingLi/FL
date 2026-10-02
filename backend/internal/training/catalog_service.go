// Package training 培训域：培训目录（专业方向 / 课程等级 / 证书模板 / 题库标签）与目标证件的
// 读面与管理面实现，以及 HTTP 出口。
//
// 本包是 internal/<域> 形态的样板之一（ADR-0070）：handler.go / handler_admin.go /
// handler_credential.go 是 HTTP 出口（学员公开读面 / 管理面 / 证件面三分），catalog_service.go
// 是域实现入口，catalog_types.go / catalog_specs.go / catalog_engine.go / position_catalog.go
// 是同域协作件。
package training

import (
	"errors"
	"fmt"

	"go.uber.org/zap"
	"gorm.io/gorm"

	"forklift-training/internal/clock"
	"forklift-training/internal/course"
	"forklift-training/internal/model"
	"forklift-training/internal/questionpool"
	"forklift-training/internal/scope"
	"forklift-training/internal/slicesx"
	"forklift-training/internal/timefmt"
)

// Service 培训目录（课程目录树与管理数据）服务。
type Service struct {
	db *gorm.DB

	logger *zap.Logger
}

// NewService 创建培训目录服务实例。
func NewService(db *gorm.DB, logger *zap.Logger) *Service {
	return &Service{db: db, logger: logger}
}

// ===== 专业方向 =====

// ErrCredentialHasContributions 是证件删除的投稿阻塞（#1360 / CONTEXT.md「证件删除的阻塞项」）。
// 本枚哨兵只认领「该证件下挂着投稿行、删除会被挡住」这件事；**条数在包装它的那句文案里**
// （`fmt.Errorf("该证件下仍有 %d 篇投稿，请先迁移或下架：%w", n, ...)`），
// api 侧按哨兵落 400、按 ADR-0064 决策 9 把那句话原样发出去。
// 之所以不把整句写进哨兵常量：哨兵是 errors.Is 的判据，句子随条数变，两者不能混在一枚值里。
var ErrCredentialHasContributions = errors.New("该证件下仍有投稿")

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

// ListSpecialties 专业方向列表（管理端含停用项，学员端仅启用项）。
func (s *Service) ListSpecialties(activeOnly bool) []SpecialtyDict {
	return catalogList(s.db, specialtyCatalogSpec(), activeOnly)
}

// CreateSpecialty 创建专业方向。
func (s *Service) CreateSpecialty(in SpecialtyInput) (SpecialtyDict, error) {
	return catalogCreate(s.db, specialtyCatalogSpec(), &in)
}

// SwapSpecialtySort 交换两个专业方向的排序位置（真实生效，含同值默认）。
func (s *Service) SwapSpecialtySort(a, b int) error {
	return catalogSwap(s.db, specialtyCatalogSpec(), a, b)
}

// UpdateSpecialty 更新专业方向。
func (s *Service) UpdateSpecialty(id int, in SpecialtyInput) (SpecialtyDict, error) {
	return catalogUpdate(s.db, specialtyCatalogSpec(), id, &in)
}

// DeleteSpecialty 删除专业方向（已关联课程置空 specialty_id，不级联删除课程）。
func (s *Service) DeleteSpecialty(id int) error {
	return catalogDelete(s.db, specialtyCatalogSpec(), id)
}

// ListLevels 课程等级列表（activeOnly=true 仅启用项）。
func (s *Service) ListLevels(activeOnly bool) []LevelDict {
	return catalogList(s.db, levelCatalogSpec(), activeOnly)
}

// CreateLevel 创建课程等级。
func (s *Service) CreateLevel(in LevelInput) (LevelDict, error) {
	return catalogCreate(s.db, levelCatalogSpec(), &in)
}

// SwapLevelSort 交换两个课程等级的排序位置（真实生效，含同值默认）。
func (s *Service) SwapLevelSort(a, b int) error {
	return catalogSwap(s.db, levelCatalogSpec(), a, b)
}

// UpdateLevel 更新课程等级。
func (s *Service) UpdateLevel(id int, in LevelInput) (LevelDict, error) {
	return catalogUpdate(s.db, levelCatalogSpec(), id, &in)
}

// DeleteLevel 删除课程等级（已关联课程置空 level_id，不级联删除课程）。
func (s *Service) DeleteLevel(id int) error {
	return catalogDelete(s.db, levelCatalogSpec(), id)
}

// ListCertificateTemplates 证书模板列表（activeOnly=true 仅启用项）。
func (s *Service) ListCertificateTemplates(activeOnly bool) []CertificateTemplateDict {
	return catalogList(s.db, certificateCatalogSpec(), activeOnly)
}

// CreateCertificateTemplate 创建证书模板。
func (s *Service) CreateCertificateTemplate(in CertificateTemplateInput) (CertificateTemplateDict, error) {
	return catalogCreate(s.db, certificateCatalogSpec(), &in)
}

// UpdateCertificateTemplate 更新证书模板。
func (s *Service) UpdateCertificateTemplate(id int, in CertificateTemplateInput) (CertificateTemplateDict, error) {
	return catalogUpdate(s.db, certificateCatalogSpec(), id, &in)
}

// DeleteCertificateTemplate 删除证书模板（已关联课程置空 certificate_template_id）。
func (s *Service) DeleteCertificateTemplate(id int) error {
	return catalogDelete(s.db, certificateCatalogSpec(), id)
}

// ListQuestionTags 题库标签列表（activeOnly=true 仅启用项；includeSourceTags=false 时
// 过滤来源标记标签——专项练习侧不应出现「真题」等 source 标签，管理端传 true 保留可见）。
// 附带 question_count：学员端统计已发布题目数，管理端统计全部题目数。
// credentialID 非 nil 时按目标证件分区（#702：学员端标签计数与抽题池同口径——
// 已发布 + 排除来源标记标签 + 证件分区；nil = 不分区，保持管理端全局口径）。
// 查询失败上抛 error（ADR-0062 票6）：旧写法把来源标签的排除查询失败咽掉 ⇒ 学员端
// 专项练习里冒出「真题」标签，选了就是空池（判据被读成「没有要排除的标签」）。
func (s *Service) ListQuestionTags(activeOnly, includeSourceTags bool, credentialID *int) ([]QuestionTagDict, error) {
	list := catalogList(s.db, questionTagCatalogSpec(), activeOnly)
	if len(list) == 0 {
		return list, nil
	}
	if !includeSourceTags {
		var sourceIDs []int
		if err := s.db.Model(&model.QuestionTag{}).Where("is_source_tag = ?", true).Pluck("id", &sourceIDs).Error; err != nil {
			return nil, err
		}
		if len(sourceIDs) > 0 {
			excluded := make(map[int]bool, len(sourceIDs))
			for _, id := range sourceIDs {
				excluded[id] = true
			}
			filtered := list[:0]
			for _, d := range list {
				if !excluded[d.ID] {
					filtered = append(filtered, d)
				}
			}
			list = filtered
			if len(list) == 0 {
				return list, nil
			}
		}
	}

	ids := make([]int, 0, len(list))
	for _, d := range list {
		ids = append(ids, d.ID)
	}

	// 一次查询全部标签的题目数（LEFT JOIN 保证无题目标签也返回 0，避免 N+1）。
	// 分区语义（#702）：学员端（activeOnly）在 published 计数上叠加题库池 scope——
	// 与抽题/搜索/按 id 取详情逐字同源（叶子包 internal/questionpool，ADR-0050 决策 1）；
	// 管理端（activeOnly=false）保持全量。
	type countRow struct {
		TagID          int
		TotalCount     int64
		PublishedCount int64
	}
	// 池谓词 raw 形态：直接拼接叶子包 internal/questionpool 的 SQL 片段（表别名对齐为 question），
	// 不就地重写——「raw 重写与常量脱钩」是漂移窗口（ADR-0050 决策 1）。
	// 带证件与不带证件两个分支由此同源同口径：#702 的原始声明就是「学员端在 published 计数上
	// 叠加排除来源标记标签题 + 可选证件分区」，原实现只在带证件分支做了排除，
	// 不带证件分支（GET /api/tags 不传 credential_id）漏了——本票按该声明口径补齐。
	query := "SELECT t.id AS tag_id, COUNT(qtr.question_id) AS total_count, " +
		"COUNT(qtr.question_id) FILTER (WHERE " + questionpool.PublishedSQL + " AND " + questionpool.ExcludeSourceTagsSQL
	var args []any
	// 池的证件分区走归属分区具名谓词的 SQL 片段形态（ADR-0056 §2）：nil → 空片段 = 不分区。
	if clause, credArgs := scope.EntityOwnedByClause(questionpool.CredentialColumn, credentialID); clause != "" {
		query += " AND " + clause
		args = append(args, credArgs...)
	}
	query += ") AS published_count " +
		"FROM question_tag AS t LEFT JOIN question_tag_relation AS qtr ON qtr.tag_id = t.id " +
		"LEFT JOIN question AS question ON question.id = qtr.question_id WHERE t.id IN ? GROUP BY t.id"
	args = append(args, ids)
	var rows []countRow
	if err := s.db.Raw(query, args...).Scan(&rows).Error; err != nil {
		return nil, err
	}
	counts := make(map[int]countRow, len(rows))
	for i := range rows {
		counts[rows[i].TagID] = rows[i]
	}
	for i := range list {
		var count int64
		if c, ok := counts[list[i].ID]; ok {
			if activeOnly {
				count = c.PublishedCount
			} else {
				count = c.TotalCount
			}
		}
		list[i].QuestionCount = &count
	}
	return list, nil
}

// CreateQuestionTag 创建题库标签。
func (s *Service) CreateQuestionTag(in QuestionTagInput) (QuestionTagDict, error) {
	return catalogCreate(s.db, questionTagCatalogSpec(), &in)
}

// UpdateQuestionTag 更新题库标签。
func (s *Service) UpdateQuestionTag(id int, in QuestionTagInput) (QuestionTagDict, error) {
	return catalogUpdate(s.db, questionTagCatalogSpec(), id, &in)
}

// DeleteQuestionTag 删除题库标签（自动清理题目关联）。
func (s *Service) DeleteQuestionTag(id int) error {
	return catalogDelete(s.db, questionTagCatalogSpec(), id)
}

// ===== 目标证件 =====

// ListCredentials 目标证件列表（activeOnly=true 仅启用项）。
func (s *Service) ListCredentials(activeOnly bool) []CredentialDict {
	return catalogList(s.db, credentialCatalogSpec(), activeOnly)
}

// CreateCredential 创建目标证件。
func (s *Service) CreateCredential(in CredentialInput) (CredentialDict, error) {
	return catalogCreate(s.db, credentialCatalogSpec(), &in)
}

// UpdateCredential 更新目标证件。
func (s *Service) UpdateCredential(id int, in CredentialInput) (CredentialDict, error) {
	return catalogUpdate(s.db, credentialCatalogSpec(), id, &in)
}

// DeleteCredential 删除目标证件。
//
// 两处分区按 CONTEXT.md「证件删除的阻塞项（credential delete blocker）」分开处置：
//   - 行为历史分区（practice_progress）**随证件删除**：库层 ON DELETE CASCADE，由迁移 000040 声明
//     （不在这里删行——那会把「分区随证件消失」这件结构事实降级成某个调用点记得写）；
//     课程/题目/模考/答题记录挂的是 SET NULL，真实考试卷是 CASCADE，都无需预检。
//   - 投稿（user_contribution）是**内容资产**，外键保持 NO ACTION（000020），这里做预检：
//     算出条数回明确文案（「该证件下仍有 N 篇投稿，请先迁移或下架」），不做静默级联删投稿，
//     也不让数据库拿外键冲突报 500。
//
// 计数口径 = 「挡住删除的那些行」，即该证件下的**全部**投稿行（含 withdrawn/rejected）——
// 外键不看状态，预检少算一档就等于放行后被 FK 判红，那条 500 正是本票要消掉的东西。
// 预检与删除之间存在插入投稿的窗口（配额与状态机在投稿侧另有守卫），真撞上了由 FK 兜底，
// 那是「并发写入撞上删除」的窄窗，不是口径缺口。
func (s *Service) DeleteCredential(id int) error {
	if err := s.checkCredentialDeleteBlockers(id); err != nil {
		return err
	}
	return catalogDelete(s.db, credentialCatalogSpec(), id)
}

// checkCredentialDeleteBlockers 证件删除的投稿预检（#1360）。
// 返回的错误以 ErrCredentialHasContributions 为哨兵（api 侧据此落 400），句子带条数。
//
// 分区谓词走 internal/scope 的具名谓词 EntityOwnedBy（归属分区：读被检索对象自身的证件列），
// 不在调用点手写 credential_id 谓词——那条静态扫描锁（credential_scope_guard_test.go）正是为此立的。
func (s *Service) checkCredentialDeleteBlockers(id int) error {
	var contributions int64
	if err := scope.EntityOwnedBy(s.db.Model(&model.UserContribution{}), "credential_id", &id).
		Count(&contributions).Error; err != nil {
		return err
	}
	if contributions > 0 {
		return fmt.Errorf("该证件下仍有 %d 篇投稿，请先迁移或下架：%w", contributions, ErrCredentialHasContributions)
	}
	return nil
}

// SwapCredentialSort 交换两个目标证件的排序位置。
func (s *Service) SwapCredentialSort(a, b int) error {
	return catalogSwap(s.db, credentialCatalogSpec(), a, b)
}

// GetCurrentCredential 获取学员当前目标证件（未设置时返回 nil）。
func (s *Service) GetCurrentCredential(userID int) (*CredentialDict, error) {
	var u model.HrwaiUser
	if err := s.db.Select("current_credential_id").First(&u, userID).Error; err != nil {
		return nil, errors.New("用户不存在")
	}
	if u.CurrentCredentialID == nil {
		return nil, nil
	}
	var c model.Credential
	if err := s.db.First(&c, *u.CurrentCredentialID).Error; err != nil {
		return nil, nil
	}
	d := credentialDict(&c)
	return &d, nil
}

// SetCurrentCredential 设置学员当前目标证件（校验证件存在且启用）。
func (s *Service) SetCurrentCredential(userID int, credentialID int) (*CredentialDict, error) {
	var c model.Credential
	if err := s.db.First(&c, credentialID).Error; err != nil {
		return nil, ErrCredentialNotFound
	}
	if c.Status != 1 {
		return nil, errors.New("证件已停用")
	}
	if err := s.db.Model(&model.HrwaiUser{}).Where("id = ?", userID).Update("current_credential_id", credentialID).Error; err != nil {
		return nil, err
	}
	d := credentialDict(&c)
	return &d, nil
}

// ListGroupedCredentials 分组返回启用证件（特种作业/技能等级各一组，按 sort_order）。
func (s *Service) ListGroupedCredentials() GroupedCredentialsDTO {
	list := s.ListCredentials(true)
	grouped := GroupedCredentialsDTO{
		SkillLevel:       []CredentialDict{},
		SpecialOperation: []CredentialDict{},
	}
	for _, d := range list {
		if d.Category == "skill_level" {
			grouped.SkillLevel = append(grouped.SkillLevel, d)
		} else {
			grouped.SpecialOperation = append(grouped.SpecialOperation, d)
		}
	}
	return grouped
}

// QuestionTagsResultDTO 题目标签全量替换的响应 {"tag_ids": [...]}（#954 片二）。
type QuestionTagsResultDTO struct {
	TagIDs []int `json:"tag_ids" extensions:"x-nullable" nullability:"nullable"`
}

// ===== 题目-标签关联 =====

// SetQuestionTags 全量替换题目标签关联。
func (s *Service) SetQuestionTags(questionID int, tagIDs []int) error {
	var q model.Question
	if err := s.db.First(&q, questionID).Error; err != nil {
		return errors.New("题目不存在")
	}
	return ReplaceQuestionTags(s.db, questionID, tagIDs)
}

// ReplaceQuestionTags 全量替换题目标签关联（校验标签存在）。
func ReplaceQuestionTags(db *gorm.DB, questionID int, tagIDs []int) error {
	tagIDs = slicesx.Ints(tagIDs)
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
			CreatedAt:  clock.Now(),
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
// credentialID 非 nil 时按目标证件分区（#702：与课程列表同口径；nil = 不分区）。
func (s *Service) GetCatalogTree(credentialID *int) *CatalogTreeDTO {
	return s.getCatalogTree(true, false, credentialID)
}

// GetAdminCatalogTree 目录树（管理端）：专业方向 → 等级 → 课程 → 章节。
// 含停用项与全部课程，课程节点附带章节列表（章节拖拽排序用 order_num）。
func (s *Service) GetAdminCatalogTree() *CatalogTreeDTO {
	return s.getCatalogTree(false, true, nil)
}

// getCatalogTree 构建目录树。
// activeOnly=true 时仅返回启用项（学员端）；withChapters=true 时课程节点附带章节列表（管理端）。
// cred 非 nil 时课程按目标证件分区（学员端 #702）；管理端传 nil 保持全量。
// 节点字段按 key 字母序声明，与旧 map 投影字节序一致（shape-lock 测试锁定）。
func (s *Service) getCatalogTree(activeOnly, withChapters bool, cred *int) *CatalogTreeDTO {
	var specialties []model.Specialty
	{
		q := s.db.Model(&model.Specialty{})
		if activeOnly {
			q = q.Where("status = ?", 1)
		}
		// 排序串取自 catalog_specs.go 的 descriptor（ADR-0060 决策 10）：
		// 本函数此前逐字抄了一份 spec 已声明的串，是同一判据的第二源。
		q.Order(specialtyCatalogSpec().OrderBy).Find(&specialties)
	}

	var levels []model.CourseLevel
	{
		q := s.db.Model(&model.CourseLevel{})
		if activeOnly {
			q = q.Where("status = ?", 1)
		}
		// 同上：课程等级排序串的唯一声明处是 levelCatalogSpec（catalog_specs.go）。
		q.Order(levelCatalogSpec().OrderBy).Find(&levels)
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
		q = scope.EntityOwnedBy(q, "course.credential_id", cred)
		q.Order("course.sort_order ASC, course.course_id ASC").Find(&rows)
	}

	// 管理端：一次性加载全部章节，按课程分组（避免逐课程查询的 N+1）
	var chaptersByCourse map[int][]course.ChapterDTO
	if withChapters {
		var chapters []model.Chapter
		s.db.Order("order_num ASC, chapter_id ASC").Find(&chapters)
		chaptersByCourse = make(map[int][]course.ChapterDTO, len(chapters))
		for i := range chapters {
			chaptersByCourse[chapters[i].CourseID] = append(chaptersByCourse[chapters[i].CourseID], course.ChapterToDTO(&chapters[i]))
		}
	}

	tree := make([]CatalogSpecialtyNode, 0, len(specialties))
	for i := range specialties {
		spec := newCatalogSpecialtyNode(&specialties[i])
		levelItems := make([]CatalogLevelNode, 0, len(levels))
		for j := range levels {
			lv := newCatalogLevelNode(&levels[j])
			courses := make([]course.CourseDTO, 0)
			for k := range rows {
				if !course.CourseMounted(rows[k].SpecialtyID, rows[k].LevelID) {
					continue
				}
				if *rows[k].SpecialtyID != specialties[i].SpecialtyID || *rows[k].LevelID != levels[j].LevelID {
					continue
				}
				cd := course.CourseToDTO(&rows[k].Course)
				cd.ChapterCount = &rows[k].ChapterCount
				if withChapters {
					course.FillPrereqIDs(s.db, rows[k].CourseID, &cd)
					if chs, ok := chaptersByCourse[rows[k].CourseID]; ok {
						cd.Chapters = &chs
					} else {
						empty := []course.ChapterDTO{}
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
func (s *Service) loadQuestionTags(questionID int) []QuestionTagRef {
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

// ===== typed 字典辅助 =====

func specialtyDict(s *model.Specialty) SpecialtyDict {
	return SpecialtyDict{
		Code:        s.Code,
		CreatedAt:   timefmt.FormatISO(s.CreatedAt),
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
		CreatedAt:   timefmt.FormatISO(l.CreatedAt),
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
		CreatedAt:    timefmt.FormatISO(t.CreatedAt),
		Description:  t.Description,
		ID:           t.ID,
		Name:         t.Name,
		Status:       t.Status,
		TemplateURL:  t.TemplateURL,
		UpdatedAt:    timefmt.FormatISO(t.UpdatedAt),
		ValidityDays: t.ValidityDays,
	}
}

func tagDict(t *model.QuestionTag) QuestionTagDict {
	return QuestionTagDict{
		Code:        t.Code,
		CreatedAt:   timefmt.FormatISO(t.CreatedAt),
		Description: t.Description,
		ID:          t.ID,
		Name:        t.Name,
		SortOrder:   t.SortOrder,
		Status:      t.Status,
		UpdatedAt:   timefmt.FormatISO(t.UpdatedAt),
	}
}

func credentialDict(c *model.Credential) CredentialDict {
	return CredentialDict{
		Category:    c.Category,
		Code:        c.Code,
		CreatedAt:   timefmt.FormatISO(c.CreatedAt),
		Description: c.Description,
		ID:          c.ID,
		Level:       c.Level,
		Name:        c.Name,
		SortOrder:   c.SortOrder,
		Status:      c.Status,
		UpdatedAt:   timefmt.FormatISO(c.UpdatedAt),
	}
}

// ===== 目录树节点（typed 投影，字段按 key 字母序与旧 map 字节序一致）=====

// CatalogTreeDTO 目录树响应契约。
type CatalogTreeDTO struct {
	Specialties []CatalogSpecialtyNode `json:"specialties" nullability:"nonnil"`
}

// CatalogSpecialtyNode 目录树专业方向节点。
type CatalogSpecialtyNode struct {
	Code        string             `json:"code"`
	CreatedAt   string             `json:"created_at"`
	Description string             `json:"description"`
	Levels      []CatalogLevelNode `json:"levels" nullability:"nonnil"`
	Name        string             `json:"name"`
	SortOrder   int                `json:"sort_order"`
	SpecialtyID int                `json:"specialty_id"`
	Status      int16              `json:"status"`
}

// CatalogLevelNode 目录树课程等级节点。
type CatalogLevelNode struct {
	Code        string             `json:"code"`
	Courses     []course.CourseDTO `json:"courses" nullability:"nonnil"`
	CreatedAt   string             `json:"created_at"`
	Description string             `json:"description"`
	LevelID     int                `json:"level_id"`
	Name        string             `json:"name"`
	SortOrder   int                `json:"sort_order"`
	Status      int16              `json:"status"`
}

// newCatalogSpecialtyNode 构造目录树专业方向节点。
func newCatalogSpecialtyNode(s *model.Specialty) CatalogSpecialtyNode {
	return CatalogSpecialtyNode{
		Code:        s.Code,
		CreatedAt:   timefmt.FormatISO(s.CreatedAt),
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
		CreatedAt:   timefmt.FormatISO(l.CreatedAt),
		Description: l.Description,
		LevelID:     l.LevelID,
		Name:        l.Name,
		SortOrder:   l.SortOrder,
		Status:      l.Status,
	}
}

// CurrentCredentialID 当前证件的事实源查询（ADR-0047 §4）：供证件作用域守卫解析「本次请求
// 按哪个证件过滤」。一次主键查询；未选证件返回 ok=false（端点按不分区处理）。
func (s *Service) CurrentCredentialID(userID int) (int, bool) {
	var u model.HrwaiUser
	if err := s.db.Select("current_credential_id").First(&u, userID).Error; err != nil {
		return 0, false
	}
	if u.CurrentCredentialID == nil || *u.CurrentCredentialID <= 0 {
		return 0, false
	}
	return *u.CurrentCredentialID, true
}
