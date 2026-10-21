// Package featured 内容精选（公司动态/行业新闻等）服务。
package featured

import (
	"errors"

	"go.uber.org/zap"
	"gorm.io/gorm"

	"forklift-training/internal/clock"
	"forklift-training/internal/filestore"
	"forklift-training/internal/model"
	"forklift-training/pkg/paging"
	"forklift-training/pkg/response"
)

// Service 内容精选服务。
type Service struct {
	db      *gorm.DB
	fileSvc *filestore.FileStore

	logger *zap.Logger
}

// NewService 创建内容精选服务实例。
func NewService(db *gorm.DB, fileSvc *filestore.FileStore, logger *zap.Logger) *Service {
	return &Service{db: db, fileSvc: fileSvc, logger: logger}
}

// featuredCategoryLabels 分类中文标签映射。
var featuredCategoryLabels = map[string]string{
	"company":  "公司动态",
	"industry": "行业新闻",
	"product":  "产品资讯",
	"news":     "政策法规",
}

// ErrFeaturedContentNotFound 「精选内容行不存在」的唯一载体（ADR-0064 决策 1/2）：
// 此前 8 处各写一遍同文案裸 errors.New，且把「查不动」一起塌进来。
var ErrFeaturedContentNotFound = errors.New("内容不存在")

// 精选域的「输入不合法」三件事实（ADR-0064 决策 3 / #1287 第 3 族：「分类无效 → 400」一系）。
// 之前是裸 errors.New ⇒ 落端点默认面 500（客户端不知道该重试还是该改请求）；
// 现在各自具名，由 api 层显式落 400。
var (
	ErrFeaturedTitleRequired   = errors.New("标题不能为空")
	ErrFeaturedCategoryInvalid = errors.New("分类无效")
	ErrFeaturedImageInvalid    = errors.New("图片地址无效（仅支持本站上传的精选图片）")
)

// CategoryLabel 返回分类的中文标签。
func (s *Service) CategoryLabel(category string) string {
	return featuredCategoryLabel(category)
}

// IsValidCategory 校验分类是否合法。
func (s *Service) IsValidCategory(category string) bool {
	_, ok := featuredCategoryLabels[category]
	return ok
}

// GetPublicList 公开列表（仅已发布），支持排序：latest（按时间，默认）/ hot（按浏览量）。
func (s *Service) GetPublicList(page, pageSize int, category string, sort ...string) (FeaturedContentPageResult, error) {
	sorted := ""
	if len(sort) > 0 && sort[0] == "hot" {
		sorted = "view_count DESC, published_at DESC, content_id DESC"
	} else {
		sorted = "published_at DESC, content_id DESC"
	}
	items, total, page, pageSize, err := paging.Query[model.FeaturedContent](s.db, page, pageSize, 10, sorted, func(q *gorm.DB) *gorm.DB {
		q = q.Where("status = ?", 1)
		if category != "" {
			q = q.Where("category = ?", category)
		}
		return q
	})
	if err != nil {
		return FeaturedContentPageResult{}, err
	}
	list := make([]FeaturedContentDTO, 0, len(items))
	for i := range items {
		list = append(list, featuredContentDTO(&items[i]))
	}
	return FeaturedContentPageResult{
		Items: list,
		Page:  page,
		Pages: response.PageCount(total, pageSize),
		Total: total,
	}, nil
}

// GetPublicDetail 公开详情（含相关资讯 + 上一篇/下一篇）。
// countView=false 时不改变 view_count（SSR/爬虫路径），true 时自增阅读量（现网既有行为）。
func (s *Service) GetPublicDetail(id int, countView bool) (*FeaturedContentDetailDTO, error) {
	var item model.FeaturedContent
	if err := s.db.First(&item, id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrFeaturedContentNotFound
		}
		return nil, err // 查不动不得被读成「不存在」（ADR-0064 决策 1）
	}
	if item.Status != 1 {
		return nil, ErrFeaturedContentNotFound
	}

	if countView {
		// 原子自增阅读量（并发安全，避免读改写竞态）
		s.db.Model(&model.FeaturedContent{}).
			Where("content_id = ?", id).
			UpdateColumn("view_count", gorm.Expr("view_count + 1"))
		item.ViewCount++
	}

	detail := featuredContentDetailDTO(&item)

	// 相关资讯：同分类最新 5 篇（排除自身）
	var related []model.FeaturedContent
	s.db.Where("status = ? AND category = ? AND content_id <> ?", 1, item.Category, id).
		Order("published_at DESC, content_id DESC").Limit(5).Find(&related)
	relatedList := make([]FeaturedContentDTO, 0, len(related))
	for i := range related {
		relatedList = append(relatedList, featuredContentDTO(&related[i]))
	}
	detail.Related = relatedList

	// 上一篇：发布时间晚于当前（更近期）
	var prev model.FeaturedContent
	if err := s.db.Where("status = ? AND content_id <> ? AND (published_at > ? OR (published_at = ? AND content_id < ?))",
		1, id, item.PublishedAt, item.PublishedAt, id).
		Order("published_at ASC, content_id ASC").First(&prev).Error; err == nil {
		dto := featuredNavDTO(&prev)
		detail.Prev = &dto
	}

	// 下一篇：发布时间早于当前（更早期）
	var next model.FeaturedContent
	if err := s.db.Where("status = ? AND content_id <> ? AND (published_at < ? OR (published_at = ? AND content_id > ?))",
		1, id, item.PublishedAt, item.PublishedAt, id).
		Order("published_at DESC, content_id DESC").First(&next).Error; err == nil {
		dto := featuredNavDTO(&next)
		detail.Next = &dto
	}

	return &detail, nil
}

// AdminList 管理端列表（含草稿与定时中）。
// state 过滤（#1748）：空 = 全部 / draft 草稿 / scheduled 定时中 / published 已发布。
// 「定时中」不是一个 status 取值，而是「status = 0 且 scheduled_at 非空」的组合，故按组合筛。
func (s *Service) AdminList(page, pageSize int, category, state string) (FeaturedContentAdminPageResult, error) {
	if err := validateFeaturedState(state); err != nil {
		return FeaturedContentAdminPageResult{}, err
	}
	items, total, page, pageSize, err := paging.Query[model.FeaturedContent](s.db, page, pageSize, 10, "created_at DESC, content_id DESC", func(q *gorm.DB) *gorm.DB {
		if category != "" {
			q = q.Where("category = ?", category)
		}
		switch state {
		case featuredStateDraft:
			q = q.Where("status = ? AND scheduled_at IS NULL", 0)
		case featuredStateScheduled:
			q = q.Where("status = ? AND scheduled_at IS NOT NULL", 0)
		case featuredStatePublished:
			q = q.Where("status = ?", 1)
		}
		return q
	})
	if err != nil {
		return FeaturedContentAdminPageResult{}, err
	}
	list := make([]FeaturedContentAdminListDTO, 0, len(items))
	for i := range items {
		list = append(list, featuredContentAdminListDTO(&items[i]))
	}
	return FeaturedContentAdminPageResult{
		Items: list,
		Page:  page,
		Pages: response.PageCount(total, pageSize),
		Total: total,
	}, nil
}

// AdminDetail 管理端详情（含正文 Markdown）。
func (s *Service) AdminDetail(id int) (*FeaturedContentAdminDetailDTO, error) {
	var item model.FeaturedContent
	if err := s.db.First(&item, id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrFeaturedContentNotFound
		}
		return nil, err // 查不动不得被读成「不存在」（ADR-0064 决策 1）
	}
	dto := featuredContentAdminDetailDTO(&item)
	return &dto, nil
}

// Create 创建内容精选（默认草稿）。
// status=1 时按「发布时间」落定：未携带 = 立即发布；≤ now = 补录；> now = 定时中（#1748）。
func (s *Service) Create(in FeaturedContentInput) (*FeaturedContentAdminDetailDTO, error) {
	if in.Title == "" {
		return nil, ErrFeaturedTitleRequired
	}
	if in.Category == "" || !s.IsValidCategory(in.Category) {
		return nil, ErrFeaturedCategoryInvalid
	}
	if err := featuredImageGate("", "", in.CoverImage, in.Content); err != nil {
		return nil, err
	}
	status := int16(0) // 默认草稿
	if in.Status != nil {
		status = *in.Status
	}
	if status != 0 && status != 1 {
		status = 0
	}
	sortOrder := 0
	if in.SortOrder != nil {
		sortOrder = *in.SortOrder
	}
	now := clock.Now()
	// 发布时间入参决定最终三字段（ADR-0076 语义矩阵）；未携带时保持 #1748 前的行为：
	// 发布补当前时间、草稿不带时间。
	plan := featuredPublishPlan{Status: status}
	if in.PublishedAt != nil {
		applied, err := applyFeaturedPublishTime(status, *in.PublishedAt, now)
		if err != nil {
			return nil, err
		}
		plan = applied
	} else if status == 1 {
		plan.PublishedAt = &now
	}
	item := model.FeaturedContent{
		Title:       in.Title,
		Summary:     in.Summary,
		CoverImage:  in.CoverImage,
		Content:     in.Content,
		Category:    in.Category,
		Source:      in.Source,
		Status:      plan.Status,
		SortOrder:   sortOrder,
		PublishedAt: plan.PublishedAt,
		ScheduledAt: plan.ScheduledAt,
		CreatedAt:   now,
		UpdatedAt:   now,
	}
	if err := s.db.Create(&item).Error; err != nil {
		return nil, err
	}
	dto := featuredContentAdminDetailDTO(&item)
	return &dto, nil
}

// Update 更新内容精选。
// 携带 published_at 时按语义矩阵落定（≤ now 补录 / > now 转为定时中）。
// 未携带时分两格：**只改内容**（连 status 也没带）⇒ 排期原样保留；**点名了状态** ⇒ 状态落定
// （草稿 → 已发布补当前时间、已发布 → 草稿保留 published_at），排期一律清空（= 取消定时）。
func (s *Service) Update(id int, in FeaturedContentUpdateInput) (*FeaturedContentAdminDetailDTO, error) {
	var item model.FeaturedContent
	if err := s.db.First(&item, id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrFeaturedContentNotFound
		}
		return nil, err // 查不动不得被读成「不存在」（ADR-0064 决策 1）
	}
	newCover, newContent := item.CoverImage, item.Content
	if in.CoverImage != nil {
		newCover = *in.CoverImage
	}
	if in.Content != nil {
		newContent = *in.Content
	}
	if err := featuredImageGate(item.CoverImage, item.Content, newCover, newContent); err != nil {
		return nil, err
	}
	if in.Title != nil && *in.Title != "" {
		item.Title = *in.Title
	}
	if in.Category != nil {
		cat := *in.Category
		if cat != "" {
			if !s.IsValidCategory(cat) {
				return nil, ErrFeaturedCategoryInvalid
			}
			item.Category = cat
		}
	}
	if in.Summary != nil {
		item.Summary = *in.Summary
	}
	if in.CoverImage != nil {
		item.CoverImage = *in.CoverImage
	}
	if in.Content != nil {
		item.Content = *in.Content
	}
	if in.Source != nil {
		item.Source = *in.Source
	}
	if in.SortOrder != nil {
		item.SortOrder = *in.SortOrder
	}

	// 状态与时间一起落定（ADR-0076 语义矩阵）：目标状态先定下来，再由「发布时间入参」决定
	// 是补录、立即发布还是转为定时中 —— 三者是同一次赋值，不能分两处各写一半。
	oldStatus := item.Status
	targetStatus := oldStatus
	if in.Status != nil {
		if next := *in.Status; next == 0 || next == 1 {
			targetStatus = next
		}
	}
	now := clock.Now()
	var plan featuredPublishPlan
	if in.PublishedAt != nil {
		applied, err := applyFeaturedPublishTime(targetStatus, *in.PublishedAt, now)
		if err != nil {
			return nil, err
		}
		plan = applied
	} else {
		// 未携带发布时间：排期是否保留取决于「有没有点名状态」。
		//   * 只改标题/正文（status 也没带）⇒ 排期原样保留（#1748 用户故事 6：临上线前的小修不必重排）；
		//   * 点名了状态 ⇒ 这是一次状态落定，排期一律清空（草稿 = 取消定时、发布 = 提前上线）。
		plan = featuredPublishPlan{Status: targetStatus, PublishedAt: item.PublishedAt, ScheduledAt: item.ScheduledAt}
		if in.Status != nil {
			plan.ScheduledAt = nil
			if targetStatus == 1 && oldStatus == 0 {
				plan.PublishedAt = &now
			}
		}
	}
	if plan.Status == 1 && plan.PublishedAt == nil {
		plan.PublishedAt = &now // 不变量 1 的写侧半边：已发布必须有时间
	}
	item.Status = plan.Status
	item.PublishedAt = plan.PublishedAt
	item.ScheduledAt = plan.ScheduledAt
	item.UpdatedAt = now

	if err := s.db.Save(&item).Error; err != nil {
		return nil, err
	}
	dto := featuredContentAdminDetailDTO(&item)
	return &dto, nil
}

// Delete 删除内容精选，并清理封面与正文内本站图片（featured/ 前缀）的存储文件。
func (s *Service) Delete(id int) (*FeaturedDeleteResult, error) {
	var item model.FeaturedContent
	if err := s.db.First(&item, id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrFeaturedContentNotFound
		}
		return nil, err // 查不动不得被读成「不存在」（ADR-0064 决策 1）
	}
	if err := s.db.Delete(&item).Error; err != nil {
		return nil, err
	}
	// 清理封面与正文图片（仅清理本站 featured 子目录文件，外部链接不动）
	s.deleteFeaturedImages(item.CoverImage, item.Content)
	return &FeaturedDeleteResult{ContentID: id}, nil
}

// deleteFeaturedImages 清理精选内容关联的图片存储文件。
// cover 为封面 URL；content 正文中 ![...](url) 语法引用的图片 URL 会被提取。
// 仅删除 featured/ 子目录（local /static/uploads/featured/、R2 <domain>/featured/）下的文件。
func (s *Service) deleteFeaturedImages(cover, content string) {
	if s.fileSvc == nil {
		return
	}
	var urls []string
	if cover != "" && filestore.IsSiteAttachmentURL(cover, filestore.FeaturedImageDirPrefix) {
		urls = append(urls, cover)
	}
	for _, u := range filestore.MarkdownImageURLs(content) {
		if filestore.IsSiteAttachmentURL(u, filestore.FeaturedImageDirPrefix) {
			urls = append(urls, u)
		}
	}
	s.fileSvc.DeleteFiles(urls)
}

// Publish 发布内容精选：无 body = 立即发布；带 body 时按语义矩阵落定（#1748）。
// 已发布且未指定时间时保持幂等（不改 published_at）—— 与 #1748 前的行为逐字一致。
func (s *Service) Publish(id int, publishedAt *string) (*FeaturedContentAdminDetailDTO, error) {
	var item model.FeaturedContent
	if err := s.db.First(&item, id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrFeaturedContentNotFound
		}
		return nil, err // 查不动不得被读成「不存在」（ADR-0064 决策 1）
	}
	now := clock.Now()
	plan := featuredPublishPlan{Status: 1, PublishedAt: &now}
	if publishedAt != nil {
		applied, err := applyFeaturedPublishTime(1, *publishedAt, now)
		if err != nil {
			return nil, err
		}
		plan = applied
	} else if item.Status == 1 {
		dto := featuredContentAdminDetailDTO(&item)
		return &dto, nil
	}
	if err := s.db.Model(&item).Updates(map[string]any{
		"status":       plan.Status,
		"published_at": plan.PublishedAt,
		"scheduled_at": plan.ScheduledAt,
		"updated_at":   now,
	}).Error; err != nil {
		return nil, err
	}
	item.Status = plan.Status
	item.PublishedAt = plan.PublishedAt
	item.ScheduledAt = plan.ScheduledAt
	item.UpdatedAt = now
	dto := featuredContentAdminDetailDTO(&item)
	return &dto, nil
}

// IncrementViewCount 客户端计数：仅已发布内容可计数，返回最新阅读量。
// 「不存在」与「查不动」分开（ADR-0062 票6）：本域 12 处塌缩里最后漏掉的一处，
// 由第③批档位台账的 500 档注入照出。
func (s *Service) IncrementViewCount(id int) (int, error) {
	var item model.FeaturedContent
	if err := s.db.First(&item, id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return 0, ErrFeaturedContentNotFound
		}
		return 0, err
	}
	if item.Status != 1 {
		return 0, ErrFeaturedContentNotFound
	}
	newCount := item.ViewCount + 1
	// 原子自增（并发安全），与 GetPublicDetail 计数路径一致
	if err := s.db.Model(&model.FeaturedContent{}).
		Where("content_id = ?", id).
		UpdateColumn("view_count", gorm.Expr("view_count + 1")).Error; err != nil {
		return 0, err
	}
	return newCount, nil
}

// SaveImage 保存图片到 featured 子目录，返回访问 URL。
func (s *Service) SaveImage(content []byte, filename string) (string, error) {
	if s.fileSvc == nil {
		return "", errors.New("文件服务未初始化")
	}
	return s.fileSvc.Save(content, filename, filestore.FeaturedImageDirPrefix)
}

// featuredImageGate 精选写面归属门禁（第十二波票 4，与论坛走同一条判据）：
// 目标封面/正文内嵌图中**新出现**的 URL 必须是本站精选图片（featured 前缀）。
// 库中已有的 URL 属「编辑未改不动」——存量外链不被门禁开启惩罚。
func featuredImageGate(oldCover, oldContent, newCover, newContent string) error {
	known := map[string]bool{}
	if oldCover != "" {
		known[oldCover] = true
	}
	for _, u := range filestore.MarkdownImageURLs(oldContent) {
		known[u] = true
	}
	check := func(u string) error {
		if u == "" || known[u] {
			return nil
		}
		if !filestore.IsSiteAttachmentURL(u, filestore.FeaturedImageDirPrefix) {
			return ErrFeaturedImageInvalid
		}
		return nil
	}
	if err := check(newCover); err != nil {
		return err
	}
	for _, u := range filestore.MarkdownImageURLs(newContent) {
		if err := check(u); err != nil {
			return err
		}
	}
	return nil
}

// featuredCategoryLabel 返回分类中文标签。
func featuredCategoryLabel(category string) string {
	if label, ok := featuredCategoryLabels[category]; ok {
		return label
	}
	if category == "news" {
		return "政策法规"
	}
	return "政策法规"
}
