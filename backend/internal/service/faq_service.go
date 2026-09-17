package service

import (
	"errors"
	"regexp"
	"strings"

	"go.uber.org/zap"
	"gorm.io/gorm"

	"forklift-training/internal/model"
)

// 帮助中心（FAQ）域（#1079）：分类 + 条目两张表，管理端 CRUD、学员端只读。
// 学员面**一次返回分类 + 全部已发布条目**（量级小），搜索走端上过滤——不新增搜索接口、
// 不进全局搜索域（ADR-0049）。

// faqCodeRe 分类稳定标识的格式：小写字母/数字/下划线，2–64 位。
// 它会被管理端与日志引用，故不许出现空格与中文（改名走 title，不改 code）。
var faqCodeRe = regexp.MustCompile(`^[a-z0-9_]{2,64}$`)

var (
	// ErrFaqCategoryNotFound 分类不存在。
	ErrFaqCategoryNotFound = errors.New("分类不存在")
	// ErrFaqEntryNotFound 条目不存在。
	ErrFaqEntryNotFound = errors.New("条目不存在")
	// ErrFaqCategoryCodeUsed 分类标识已被占用（code 唯一）。
	ErrFaqCategoryCodeUsed = errors.New("分类标识已存在")
)

// FaqEntryDTO 帮助中心条目。字段按 JSON key 字母序声明（answer / id / question / sort_order）。
type FaqEntryDTO struct {
	Answer    string `json:"answer"`
	ID        int    `json:"id"`
	Question  string `json:"question"`
	SortOrder int    `json:"sort_order"`
}

// FaqCategoryDTO 帮助中心分类（含其下已发布条目）。
type FaqCategoryDTO struct {
	Code      string        `json:"code"`
	Entries   []FaqEntryDTO `json:"entries"`
	ID        int           `json:"id"`
	SortOrder int           `json:"sort_order"`
	Title     string        `json:"title"`
}

// FaqResult 学员端帮助中心整页载荷。
type FaqResult struct {
	Categories []FaqCategoryDTO `json:"categories"`
}

// AdminFaqCategoryDTO 管理端分类条目（含停用态与条目计数，不带条目正文）。
type AdminFaqCategoryDTO struct {
	Code       string `json:"code"`
	Enabled    bool   `json:"enabled"`
	EntryCount int64  `json:"entry_count"`
	ID         int    `json:"id"`
	SortOrder  int    `json:"sort_order"`
	Title      string `json:"title"`
}

// AdminFaqEntryDTO 管理端条目（含未发布，并带所属分类 code 供列表展示与筛选）。
type AdminFaqEntryDTO struct {
	Answer       string `json:"answer"`
	CategoryCode string `json:"category_code"`
	CategoryID   int    `json:"category_id"`
	ID           int    `json:"id"`
	Published    bool   `json:"published"`
	Question     string `json:"question"`
	SortOrder    int    `json:"sort_order"`
}

// AdminFaqCategoriesResult 管理端分类清单载荷。
type AdminFaqCategoriesResult struct {
	Categories []AdminFaqCategoryDTO `json:"categories"`
}

// AdminFaqEntriesResult 管理端条目清单载荷。
type AdminFaqEntriesResult struct {
	Entries []AdminFaqEntryDTO `json:"entries"`
}

// FaqCategoryInput 分类写入口径（新建与更新共用）。
type FaqCategoryInput struct {
	Code      string
	Title     string
	SortOrder int
	Enabled   bool
}

// FaqEntryInput 条目写入口径（新建与更新共用）。
type FaqEntryInput struct {
	CategoryID int
	Question   string
	Answer     string
	SortOrder  int
	Published  bool
}

// FaqService 帮助中心服务：学员端只读 + 管理端 CRUD。
type FaqService struct {
	db     *gorm.DB
	logger *zap.Logger
}

// NewFaqService 创建帮助中心服务。
func NewFaqService(db *gorm.DB, logger *zap.Logger) *FaqService {
	return &FaqService{db: db, logger: logger}
}

// ListPublished 学员端帮助中心：enabled 分类 + 其下 published 条目，各按 (sort_order, id) 升序。
// 两条查询一次取齐（禁 N+1），在内存里按 category_id 分组——没有分类的条目无处可挂，
// 故不会被返回（那是管理端「未归类的已发布条目」问题，由管理面暴露）。
func (s *FaqService) ListPublished() (*FaqResult, error) {
	var cats []model.FaqCategory
	if err := s.db.Where("enabled = ?", true).Order("sort_order ASC, id ASC").Find(&cats).Error; err != nil {
		return nil, err
	}
	var entries []model.Faq
	if err := s.db.Where("published = ?", true).Order("sort_order ASC, id ASC").Find(&entries).Error; err != nil {
		return nil, err
	}
	byCat := make(map[int][]FaqEntryDTO, len(cats))
	for _, e := range entries {
		byCat[e.CategoryID] = append(byCat[e.CategoryID], FaqEntryDTO{
			Answer: e.Answer, ID: e.ID, Question: e.Question, SortOrder: e.SortOrder,
		})
	}
	out := make([]FaqCategoryDTO, 0, len(cats))
	for _, c := range cats {
		es := byCat[c.ID]
		if es == nil {
			es = []FaqEntryDTO{} // 空分类返回空数组而非 null（前端免判空）
		}
		out = append(out, FaqCategoryDTO{Code: c.Code, Entries: es, ID: c.ID, SortOrder: c.SortOrder, Title: c.Title})
	}
	return &FaqResult{Categories: out}, nil
}

// AdminListCategories 管理端分类清单（含停用），带条目计数（一次 GROUP BY，禁 N+1）。
func (s *FaqService) AdminListCategories() ([]AdminFaqCategoryDTO, error) {
	var cats []model.FaqCategory
	if err := s.db.Order("sort_order ASC, id ASC").Find(&cats).Error; err != nil {
		return nil, err
	}
	type row struct {
		CategoryID int
		N          int64
	}
	var rows []row
	if err := s.db.Model(&model.Faq{}).Select("category_id, COUNT(*) AS n").Group("category_id").Scan(&rows).Error; err != nil {
		return nil, err
	}
	counts := make(map[int]int64, len(rows))
	for _, r := range rows {
		counts[r.CategoryID] = r.N
	}
	out := make([]AdminFaqCategoryDTO, 0, len(cats))
	for _, c := range cats {
		out = append(out, AdminFaqCategoryDTO{
			Code: c.Code, Enabled: c.Enabled, EntryCount: counts[c.ID], ID: c.ID,
			SortOrder: c.SortOrder, Title: c.Title,
		})
	}
	return out, nil
}

// validateCategoryInput 分类写入口径校验。
func validateCategoryInput(in *FaqCategoryInput) error {
	in.Code = strings.TrimSpace(in.Code)
	in.Title = strings.TrimSpace(in.Title)
	if !faqCodeRe.MatchString(in.Code) {
		return errors.New("分类标识只能用小写字母、数字与下划线（2-64 位）")
	}
	if in.Title == "" {
		return errors.New("分类名称不能为空")
	}
	if len([]rune(in.Title)) > 32 {
		return errors.New("分类名称不能超过 32 字")
	}
	return nil
}

// validateEntryInput 条目写入口径校验。
func validateEntryInput(in *FaqEntryInput) error {
	in.Question = strings.TrimSpace(in.Question)
	in.Answer = strings.TrimSpace(in.Answer)
	if in.Question == "" {
		return errors.New("问题不能为空")
	}
	if len([]rune(in.Question)) > 200 {
		return errors.New("问题不能超过 200 字")
	}
	if in.Answer == "" {
		return errors.New("答案不能为空")
	}
	if len([]rune(in.Answer)) > 4000 {
		return errors.New("答案不能超过 4000 字")
	}
	return nil
}

// AdminCreateCategory 新建分类。
func (s *FaqService) AdminCreateCategory(in FaqCategoryInput) (*AdminFaqCategoryDTO, error) {
	if err := validateCategoryInput(&in); err != nil {
		return nil, err
	}
	var dup int64
	if err := s.db.Model(&model.FaqCategory{}).Where("code = ?", in.Code).Count(&dup).Error; err != nil {
		return nil, err
	}
	if dup > 0 {
		return nil, ErrFaqCategoryCodeUsed
	}
	c := model.FaqCategory{Code: in.Code, Title: in.Title, SortOrder: in.SortOrder, Enabled: in.Enabled, CreatedAt: beijingNow(), UpdatedAt: beijingNow()}
	if err := s.db.Create(&c).Error; err != nil {
		if isDuplicateError(err) {
			return nil, ErrFaqCategoryCodeUsed
		}
		return nil, err
	}
	return &AdminFaqCategoryDTO{Code: c.Code, Enabled: c.Enabled, EntryCount: 0, ID: c.ID, SortOrder: c.SortOrder, Title: c.Title}, nil
}

// AdminUpdateCategory 改分类。
func (s *FaqService) AdminUpdateCategory(id int, in FaqCategoryInput) (*AdminFaqCategoryDTO, error) {
	if err := validateCategoryInput(&in); err != nil {
		return nil, err
	}
	var c model.FaqCategory
	if err := s.db.First(&c, id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrFaqCategoryNotFound
		}
		return nil, err
	}
	var dup int64
	if err := s.db.Model(&model.FaqCategory{}).Where("code = ? AND id <> ?", in.Code, id).Count(&dup).Error; err != nil {
		return nil, err
	}
	if dup > 0 {
		return nil, ErrFaqCategoryCodeUsed
	}
	c.Code, c.Title, c.SortOrder, c.Enabled, c.UpdatedAt = in.Code, in.Title, in.SortOrder, in.Enabled, beijingNow()
	if err := s.db.Save(&c).Error; err != nil {
		if isDuplicateError(err) {
			return nil, ErrFaqCategoryCodeUsed
		}
		return nil, err
	}
	var n int64
	_ = s.db.Model(&model.Faq{}).Where("category_id = ?", id).Count(&n).Error
	return &AdminFaqCategoryDTO{Code: c.Code, Enabled: c.Enabled, EntryCount: n, ID: c.ID, SortOrder: c.SortOrder, Title: c.Title}, nil
}

// AdminDeleteCategory 删分类。**其下条目一并删除**——管理端确认框必须写明这一点
// （半删的分类会让条目变成无处可挂的孤儿）。
//
// 级联**在应用层显式做**，而不是只靠 DDL 的 ON DELETE CASCADE：测试内存库不强制外键，
// 只靠 DDL 会让「测试绿、生产也绿但测试其实没验到」；照 auth_service 注销清理的既有先例
// （「有 CASCADE 的表显式删除以兼容测试内存库」）。DDL 上的 CASCADE 保留为直接 SQL 删除的兜底。
func (s *FaqService) AdminDeleteCategory(id int) error {
	return s.db.Transaction(func(tx *gorm.DB) error {
		var c model.FaqCategory
		if err := tx.First(&c, id).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return ErrFaqCategoryNotFound
			}
			return err
		}
		if err := tx.Where("category_id = ?", id).Delete(&model.Faq{}).Error; err != nil {
			return err
		}
		return tx.Delete(&model.FaqCategory{}, id).Error
	})
}

// AdminListEntries 管理端条目清单（含未发布），可按分类过滤。
func (s *FaqService) AdminListEntries(categoryID *int) ([]AdminFaqEntryDTO, error) {
	q := s.db.Model(&model.Faq{}).
		Select("faq.id, faq.category_id, faq.question, faq.answer, faq.sort_order, faq.published, faq_category.code AS category_code").
		Joins("JOIN faq_category ON faq_category.id = faq.category_id")
	if categoryID != nil {
		q = q.Where("faq.category_id = ?", *categoryID)
	}
	type row struct {
		ID           int
		CategoryID   int
		Question     string
		Answer       string
		SortOrder    int
		Published    bool
		CategoryCode string
	}
	var rows []row
	if err := q.Order("faq_category.sort_order ASC, faq.sort_order ASC, faq.id ASC").Scan(&rows).Error; err != nil {
		return nil, err
	}
	out := make([]AdminFaqEntryDTO, 0, len(rows))
	for _, r := range rows {
		out = append(out, AdminFaqEntryDTO{
			Answer: r.Answer, CategoryCode: r.CategoryCode, CategoryID: r.CategoryID, ID: r.ID,
			Published: r.Published, Question: r.Question, SortOrder: r.SortOrder,
		})
	}
	return out, nil
}

// faqEntryDTOOf 单条读回（新写 / 更新后回填分类 code）。
func (s *FaqService) faqEntryDTOOf(e *model.Faq) (*AdminFaqEntryDTO, error) {
	var c model.FaqCategory
	if err := s.db.First(&c, e.CategoryID).Error; err != nil {
		return nil, err
	}
	return &AdminFaqEntryDTO{
		Answer: e.Answer, CategoryCode: c.Code, CategoryID: e.CategoryID, ID: e.ID,
		Published: e.Published, Question: e.Question, SortOrder: e.SortOrder,
	}, nil
}

// AdminCreateEntry 新建条目。
func (s *FaqService) AdminCreateEntry(in FaqEntryInput) (*AdminFaqEntryDTO, error) {
	if err := validateEntryInput(&in); err != nil {
		return nil, err
	}
	var c model.FaqCategory
	if err := s.db.First(&c, in.CategoryID).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrFaqCategoryNotFound
		}
		return nil, err
	}
	e := model.Faq{
		CategoryID: in.CategoryID, Question: in.Question, Answer: in.Answer,
		SortOrder: in.SortOrder, Published: in.Published, CreatedAt: beijingNow(), UpdatedAt: beijingNow(),
	}
	if err := s.db.Create(&e).Error; err != nil {
		return nil, err
	}
	return s.faqEntryDTOOf(&e)
}

// AdminUpdateEntry 改条目。
func (s *FaqService) AdminUpdateEntry(id int, in FaqEntryInput) (*AdminFaqEntryDTO, error) {
	if err := validateEntryInput(&in); err != nil {
		return nil, err
	}
	var e model.Faq
	if err := s.db.First(&e, id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrFaqEntryNotFound
		}
		return nil, err
	}
	var c model.FaqCategory
	if err := s.db.First(&c, in.CategoryID).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrFaqCategoryNotFound
		}
		return nil, err
	}
	e.CategoryID, e.Question, e.Answer = in.CategoryID, in.Question, in.Answer
	e.SortOrder, e.Published, e.UpdatedAt = in.SortOrder, in.Published, beijingNow()
	if err := s.db.Save(&e).Error; err != nil {
		return nil, err
	}
	return s.faqEntryDTOOf(&e)
}

// AdminDeleteEntry 删条目。
func (s *FaqService) AdminDeleteEntry(id int) error {
	res := s.db.Delete(&model.Faq{}, id)
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return ErrFaqEntryNotFound
	}
	return nil
}
