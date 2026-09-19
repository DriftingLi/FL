// Package service 内容精选（公司动态/行业新闻等）服务。
package service

import (
	"errors"

	"go.uber.org/zap"
	"gorm.io/gorm"

	"forklift-training/internal/model"
	"forklift-training/pkg/paging"
	"forklift-training/pkg/response"
)

// FeaturedService 内容精选服务。
type FeaturedService struct {
	db      *gorm.DB
	fileSvc *FileStore

	logger *zap.Logger
}

// NewFeaturedService 创建内容精选服务实例。
func NewFeaturedService(db *gorm.DB, fileSvc *FileStore, logger *zap.Logger) *FeaturedService {
	return &FeaturedService{db: db, fileSvc: fileSvc, logger: logger}
}

// featuredCategoryLabels 分类中文标签映射。
var featuredCategoryLabels = map[string]string{
	"company":  "公司动态",
	"industry": "行业新闻",
	"product":  "产品资讯",
	"news":     "政策法规",
}

// CategoryLabel 返回分类的中文标签。
func (s *FeaturedService) CategoryLabel(category string) string {
	return featuredCategoryLabel(category)
}

// IsValidCategory 校验分类是否合法。
func (s *FeaturedService) IsValidCategory(category string) bool {
	_, ok := featuredCategoryLabels[category]
	return ok
}

// GetPublicList 公开列表（仅已发布），支持排序：latest（按时间，默认）/ hot（按浏览量）。
func (s *FeaturedService) GetPublicList(page, pageSize int, category string, sort ...string) (FeaturedContentPageResult, error) {
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
func (s *FeaturedService) GetPublicDetail(id int, countView bool) (*FeaturedContentDetailDTO, error) {
	var item model.FeaturedContent
	if err := s.db.First(&item, id).Error; err != nil {
		return nil, errors.New("内容不存在")
	}
	if item.Status != 1 {
		return nil, errors.New("内容不存在")
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

// AdminList 管理端列表（含草稿）。
func (s *FeaturedService) AdminList(page, pageSize int, category, status string) (FeaturedContentPageResult, error) {
	items, total, page, pageSize, err := paging.Query[model.FeaturedContent](s.db, page, pageSize, 10, "created_at DESC, content_id DESC", func(q *gorm.DB) *gorm.DB {
		if category != "" {
			q = q.Where("category = ?", category)
		}
		if status != "" {
			q = q.Where("status = ?", status)
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

// AdminDetail 管理端详情（含正文 Markdown）。
func (s *FeaturedService) AdminDetail(id int) (*FeaturedContentAdminDetailDTO, error) {
	var item model.FeaturedContent
	if err := s.db.First(&item, id).Error; err != nil {
		return nil, errors.New("内容不存在")
	}
	dto := featuredContentAdminDetailDTO(&item)
	return &dto, nil
}

// Create 创建内容精选（默认草稿；status=1 时写入 published_at）。
func (s *FeaturedService) Create(in FeaturedContentInput) (*FeaturedContentAdminDetailDTO, error) {
	if in.Title == "" {
		return nil, errors.New("标题不能为空")
	}
	if in.Category == "" || !s.IsValidCategory(in.Category) {
		return nil, errors.New("分类无效")
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
	now := beijingNow()
	item := model.FeaturedContent{
		Title:      in.Title,
		Summary:    in.Summary,
		CoverImage: in.CoverImage,
		Content:    in.Content,
		Category:   in.Category,
		Source:     in.Source,
		Status:     status,
		SortOrder:  sortOrder,
		CreatedAt:  now,
		UpdatedAt:  now,
	}
	if status == 1 {
		item.PublishedAt = &now
	}
	if err := s.db.Create(&item).Error; err != nil {
		return nil, err
	}
	dto := featuredContentAdminDetailDTO(&item)
	return &dto, nil
}

// Update 更新内容精选。
// 若从草稿改为已发布，则补写当前时间；已发布 → 草稿保留 published_at。
func (s *FeaturedService) Update(id int, in FeaturedContentUpdateInput) (*FeaturedContentAdminDetailDTO, error) {
	var item model.FeaturedContent
	if err := s.db.First(&item, id).Error; err != nil {
		return nil, errors.New("内容不存在")
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
				return nil, errors.New("分类无效")
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

	// 状态变更处理
	oldStatus := item.Status
	newStatus := oldStatus
	if in.Status != nil {
		newStatus = *in.Status
		if newStatus != 0 && newStatus != 1 {
			newStatus = oldStatus
		}
	}
	if oldStatus == 0 && newStatus == 1 {
		now := beijingNow()
		item.PublishedAt = &now
	}
	item.Status = newStatus
	item.UpdatedAt = beijingNow()

	if err := s.db.Save(&item).Error; err != nil {
		return nil, err
	}
	dto := featuredContentAdminDetailDTO(&item)
	return &dto, nil
}

// Delete 删除内容精选，并清理封面与正文内本站图片（featured/ 前缀）的存储文件。
func (s *FeaturedService) Delete(id int) (*FeaturedDeleteResult, error) {
	var item model.FeaturedContent
	if err := s.db.First(&item, id).Error; err != nil {
		return nil, errors.New("内容不存在")
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
func (s *FeaturedService) deleteFeaturedImages(cover, content string) {
	if s.fileSvc == nil {
		return
	}
	var urls []string
	if cover != "" && IsSiteAttachmentURL(cover, FeaturedImageDirPrefix) {
		urls = append(urls, cover)
	}
	for _, u := range markdownImageURLs(content) {
		if IsSiteAttachmentURL(u, FeaturedImageDirPrefix) {
			urls = append(urls, u)
		}
	}
	s.fileSvc.DeleteFiles(urls)
}

// Publish 发布内容精选（草稿 → 已发布）。
func (s *FeaturedService) Publish(id int) (*FeaturedContentAdminDetailDTO, error) {
	var item model.FeaturedContent
	if err := s.db.First(&item, id).Error; err != nil {
		return nil, errors.New("内容不存在")
	}
	if item.Status == 1 {
		dto := featuredContentAdminDetailDTO(&item)
		return &dto, nil
	}
	now := beijingNow()
	if err := s.db.Model(&item).Updates(map[string]any{
		"status":       int16(1),
		"published_at": now,
		"updated_at":   now,
	}).Error; err != nil {
		return nil, err
	}
	item.Status = 1
	item.PublishedAt = &now
	item.UpdatedAt = now
	dto := featuredContentAdminDetailDTO(&item)
	return &dto, nil
}

// IncrementViewCount 客户端计数：仅已发布内容可计数，返回最新阅读量。
func (s *FeaturedService) IncrementViewCount(id int) (int, error) {
	var item model.FeaturedContent
	if err := s.db.First(&item, id).Error; err != nil {
		return 0, errors.New("内容不存在")
	}
	if item.Status != 1 {
		return 0, errors.New("内容不存在")
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
func (s *FeaturedService) SaveImage(content []byte, filename string) (string, error) {
	if s.fileSvc == nil {
		return "", errors.New("文件服务未初始化")
	}
	return s.fileSvc.Save(content, filename, FeaturedImageDirPrefix)
}

// featuredImageGate 精选写面归属门禁（第十二波票 4，与论坛走同一条判据）：
// 目标封面/正文内嵌图中**新出现**的 URL 必须是本站精选图片（featured 前缀）。
// 库中已有的 URL 属「编辑未改不动」——存量外链不被门禁开启惩罚。
func featuredImageGate(oldCover, oldContent, newCover, newContent string) error {
	known := map[string]bool{}
	if oldCover != "" {
		known[oldCover] = true
	}
	for _, u := range markdownImageURLs(oldContent) {
		known[u] = true
	}
	check := func(u string) error {
		if u == "" || known[u] {
			return nil
		}
		if !IsSiteAttachmentURL(u, FeaturedImageDirPrefix) {
			return errors.New("图片地址无效（仅支持本站上传的精选图片）")
		}
		return nil
	}
	if err := check(newCover); err != nil {
		return err
	}
	for _, u := range markdownImageURLs(newContent) {
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
