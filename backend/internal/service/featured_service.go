// Package service 内容精选（公司动态/行业新闻等）服务。
package service

import (
	"errors"

	"gorm.io/gorm"

	"forklift-training/internal/model"
)

// FeaturedService 内容精选服务。
type FeaturedService struct {
	db      *gorm.DB
	fileSvc *FileService
}

// NewFeaturedService 创建内容精选服务实例。
func NewFeaturedService(db *gorm.DB, fileSvc *FileService) *FeaturedService {
	return &FeaturedService{db: db, fileSvc: fileSvc}
}

// featuredCategoryLabels 分类中文标签映射。
var featuredCategoryLabels = map[string]string{
	"company":  "公司动态",
	"industry": "行业新闻",
	"product":  "产品资讯",
	"news":     "资讯",
}

// CategoryLabel 返回分类的中文标签。
func (s *FeaturedService) CategoryLabel(category string) string {
	if label, ok := featuredCategoryLabels[category]; ok {
		return label
	}
	return "资讯"
}

// IsValidCategory 校验分类是否合法。
func (s *FeaturedService) IsValidCategory(category string) bool {
	_, ok := featuredCategoryLabels[category]
	return ok
}

// GetPublicList 公开列表（仅已发布）。
func (s *FeaturedService) GetPublicList(page, pageSize int, category string) map[string]any {
	if page <= 0 {
		page = 1
	}
	if pageSize <= 0 {
		pageSize = 10
	}
	q := s.db.Model(&model.FeaturedContent{}).Where("status = ?", 1)
	if category != "" {
		q = q.Where("category = ?", category)
	}
	var total int64
	q.Count(&total)
	var items []model.FeaturedContent
	q.Order("published_at DESC, content_id DESC").
		Offset((page - 1) * pageSize).Limit(pageSize).Find(&items)
	list := make([]map[string]any, 0, len(items))
	for i := range items {
		list = append(list, featuredToListDict(&items[i]))
	}
	pages := int((total + int64(pageSize) - 1) / int64(pageSize))
	return map[string]any{
		"total":  total,
		"page":   page,
		"pages":  pages,
		"items":  list,
	}
}

// GetPublicDetail 公开详情（含相关资讯 + 上一篇/下一篇，自增阅读量）。
func (s *FeaturedService) GetPublicDetail(id int) (map[string]any, error) {
	var item model.FeaturedContent
	if err := s.db.First(&item, id).Error; err != nil {
		return nil, errors.New("内容不存在")
	}
	if item.Status != 1 {
		return nil, errors.New("内容不存在")
	}

	// 自增阅读量
	s.db.Model(&model.FeaturedContent{}).
		Where("content_id = ?", id).
		UpdateColumn("view_count", item.ViewCount+1)
	item.ViewCount++

	detail := featuredToDetailDict(&item)

	// 相关资讯：同分类最新 5 篇（排除自身）
	var related []model.FeaturedContent
	s.db.Where("status = ? AND category = ? AND content_id <> ?", 1, item.Category, id).
		Order("published_at DESC, content_id DESC").Limit(5).Find(&related)
	relatedList := make([]map[string]any, 0, len(related))
	for i := range related {
		relatedList = append(relatedList, featuredToListDict(&related[i]))
	}
	detail["related"] = relatedList

	// 上一篇：发布时间晚于当前（更近期）
	var prev model.FeaturedContent
	hasPrev := true
	if err := s.db.Where("status = ? AND content_id <> ? AND (published_at > ? OR (published_at = ? AND content_id < ?))",
		1, id, item.PublishedAt, item.PublishedAt, id).
		Order("published_at ASC, content_id ASC").First(&prev).Error; err != nil {
		hasPrev = false
	}
	if hasPrev {
		detail["prev"] = featuredToNavDict(&prev)
	} else {
		detail["prev"] = nil
	}

	// 下一篇：发布时间早于当前（更早期）
	var next model.FeaturedContent
	hasNext := true
	if err := s.db.Where("status = ? AND content_id <> ? AND (published_at < ? OR (published_at = ? AND content_id > ?))",
		1, id, item.PublishedAt, item.PublishedAt, id).
		Order("published_at DESC, content_id DESC").First(&next).Error; err != nil {
		hasNext = false
	}
	if hasNext {
		detail["next"] = featuredToNavDict(&next)
	} else {
		detail["next"] = nil
	}

	return detail, nil
}

// AdminList 管理端列表（含草稿）。
func (s *FeaturedService) AdminList(page, pageSize int, category, status string) map[string]any {
	if page <= 0 {
		page = 1
	}
	if pageSize <= 0 {
		pageSize = 10
	}
	q := s.db.Model(&model.FeaturedContent{})
	if category != "" {
		q = q.Where("category = ?", category)
	}
	if status != "" {
		q = q.Where("status = ?", status)
	}
	var total int64
	q.Count(&total)
	var items []model.FeaturedContent
	q.Order("created_at DESC, content_id DESC").
		Offset((page - 1) * pageSize).Limit(pageSize).Find(&items)
	list := make([]map[string]any, 0, len(items))
	for i := range items {
		list = append(list, featuredToListDict(&items[i]))
	}
	pages := int((total + int64(pageSize) - 1) / int64(pageSize))
	return map[string]any{
		"total": total,
		"page":  page,
		"pages": pages,
		"items": list,
	}
}

// AdminDetail 管理端详情（含正文 Markdown）。
func (s *FeaturedService) AdminDetail(id int) (map[string]any, error) {
	var item model.FeaturedContent
	if err := s.db.First(&item, id).Error; err != nil {
		return nil, errors.New("内容不存在")
	}
	return featuredToDetailDict(&item), nil
}

// Create 创建内容精选。
// data 字段：title(必填)、category(必填)、summary、cover_image、content、source、status(0/1)。
func (s *FeaturedService) Create(data map[string]any) (map[string]any, error) {
	title, _ := data["title"].(string)
	if title == "" {
		return nil, errors.New("标题不能为空")
	}
	category, _ := data["category"].(string)
	if category == "" || !s.IsValidCategory(category) {
		return nil, errors.New("分类无效")
	}
	status := int16(0) // 默认草稿
	if v, ok := data["status"]; ok {
		status = int16(toIntDefault(v, 0))
	}
	if status != 0 && status != 1 {
		status = 0
	}
	now := beijingNow()
	item := model.FeaturedContent{
		Title:      title,
		Summary:    getString(data, "summary"),
		CoverImage: getString(data, "cover_image"),
		Content:    getString(data, "content"),
		Category:   category,
		Source:     getString(data, "source"),
		Status:     status,
		SortOrder:  toIntDefault(data["sort_order"], 0),
		CreatedAt:  now,
		UpdatedAt:  now,
	}
	if status == 1 {
		item.PublishedAt = &now
	}
	if err := s.db.Create(&item).Error; err != nil {
		return nil, err
	}
	return featuredToDetailDict(&item), nil
}

// Update 更新内容精选。
// 若从草稿改为已发布且未提供 published_at，则补写当前时间。
func (s *FeaturedService) Update(id int, data map[string]any) (map[string]any, error) {
	var item model.FeaturedContent
	if err := s.db.First(&item, id).Error; err != nil {
		return nil, errors.New("内容不存在")
	}
	if v, ok := data["title"]; ok {
		if s, _ := v.(string); s != "" {
			item.Title = s
		}
	}
	if v, ok := data["category"]; ok {
		cat, _ := v.(string)
		if cat != "" && !s.IsValidCategory(cat) {
			return nil, errors.New("分类无效")
		}
		if cat != "" {
			item.Category = cat
		}
	}
	// 直接覆盖可写字段
	if v, ok := data["summary"]; ok {
		item.Summary, _ = v.(string)
	}
	if v, ok := data["cover_image"]; ok {
		item.CoverImage, _ = v.(string)
	}
	if v, ok := data["content"]; ok {
		item.Content, _ = v.(string)
	}
	if v, ok := data["source"]; ok {
		item.Source, _ = v.(string)
	}
	if v, ok := data["sort_order"]; ok {
		item.SortOrder = toIntDefault(v, 0)
	}

	// 状态变更处理
	oldStatus := item.Status
	newStatus := oldStatus
	if v, ok := data["status"]; ok {
		newStatus = int16(toIntDefault(v, int(oldStatus)))
		if newStatus != 0 && newStatus != 1 {
			newStatus = oldStatus
		}
	}
	if oldStatus == 0 && newStatus == 1 {
		// 草稿 → 已发布：写入 published_at
		now := beijingNow()
		item.PublishedAt = &now
	}
	// 已发布 → 草稿：保留 published_at 便于重新发布时参考
	item.Status = newStatus
	item.UpdatedAt = beijingNow()

	if err := s.db.Save(&item).Error; err != nil {
		return nil, err
	}
	return featuredToDetailDict(&item), nil
}

// Delete 删除内容精选。
func (s *FeaturedService) Delete(id int) (map[string]any, error) {
	var item model.FeaturedContent
	if err := s.db.First(&item, id).Error; err != nil {
		return nil, errors.New("内容不存在")
	}
	if err := s.db.Delete(&item).Error; err != nil {
		return nil, err
	}
	// 删除关联的封面与正文内图片由 fileSvc 处理（可选，此处略）
	return map[string]any{"content_id": id}, nil
}

// Publish 发布内容精选（草稿 → 已发布）。
func (s *FeaturedService) Publish(id int) (map[string]any, error) {
	var item model.FeaturedContent
	if err := s.db.First(&item, id).Error; err != nil {
		return nil, errors.New("内容不存在")
	}
	if item.Status == 1 {
		return featuredToDetailDict(&item), nil
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
	return featuredToDetailDict(&item), nil
}

// SaveImage 保存图片到 featured 子目录，返回访问 URL。
func (s *FeaturedService) SaveImage(content []byte, filename string) (string, error) {
	if s.fileSvc == nil {
		return "", errors.New("文件服务未初始化")
	}
	url, _ := s.fileSvc.SaveFile(content, filename, "featured")
	return url, nil
}

// ===== dict 辅助 =====

// featuredToListDict 列表项 dict（不含 content 正文）。
func featuredToListDict(c *model.FeaturedContent) map[string]any {
	d := map[string]any{
		"content_id":  c.ContentID,
		"title":       c.Title,
		"summary":     c.Summary,
		"cover_image": c.CoverImage,
		"category":    c.Category,
		"category_label": featuredCategoryLabel(c.Category),
		"source":      c.Source,
		"status":      c.Status,
		"view_count":  c.ViewCount,
		"sort_order":  c.SortOrder,
		"created_at":  formatISO(c.CreatedAt),
		"updated_at":  formatISO(c.UpdatedAt),
	}
	if c.PublishedAt != nil {
		d["published_at"] = formatISO(*c.PublishedAt)
	} else {
		d["published_at"] = nil
	}
	return d
}

// featuredToDetailDict 详情 dict（含 content 正文）。
func featuredToDetailDict(c *model.FeaturedContent) map[string]any {
	d := featuredToListDict(c)
	d["content"] = c.Content
	return d
}

// featuredToNavDict 上一篇/下一篇导航 dict（仅 id + title + category）。
func featuredToNavDict(c *model.FeaturedContent) map[string]any {
	d := map[string]any{
		"content_id":     c.ContentID,
		"title":          c.Title,
		"category":       c.Category,
		"category_label": featuredCategoryLabel(c.Category),
	}
	if c.PublishedAt != nil {
		d["published_at"] = formatISO(*c.PublishedAt)
	} else {
		d["published_at"] = nil
	}
	return d
}

// featuredCategoryLabel 返回分类中文标签。
func featuredCategoryLabel(category string) string {
	if label, ok := featuredCategoryLabels[category]; ok {
		return label
	}
	return "资讯"
}
