// Package featured 内容精选 typed surface。
// **响应** DTO 的 JSON 字段声明按 key 字母序排列，与旧 map 字典序列化的字节序保持一致（ADR-0009）；
// 入参结构体不在此列（它们按表单顺序写，重排只会制造无关 diff）。
package featured

import (
	"time"

	"forklift-training/internal/model"
	"forklift-training/internal/timefmt"
)

// FeaturedContentDTO 精选内容列表项（不含正文）。
type FeaturedContentDTO struct {
	Category      string  `json:"category"`
	CategoryLabel string  `json:"category_label"`
	ContentID     int     `json:"content_id"`
	CoverImage    string  `json:"cover_image"`
	CreatedAt     string  `json:"created_at"`
	PublishedAt   *string `json:"published_at" extensions:"x-nullable"`
	SortOrder     int     `json:"sort_order"`
	Source        string  `json:"source"`
	Status        int16   `json:"status"`
	Summary       string  `json:"summary"`
	Title         string  `json:"title"`
	UpdatedAt     string  `json:"updated_at"`
	ViewCount     int     `json:"view_count"`
}

// FeaturedContentDetailDTO 公开详情（列表项 + 正文 + 相关资讯 + 上/下一篇）。
type FeaturedContentDetailDTO struct {
	Category      string               `json:"category"`
	CategoryLabel string               `json:"category_label"`
	Content       string               `json:"content"`
	ContentID     int                  `json:"content_id"`
	CoverImage    string               `json:"cover_image"`
	CreatedAt     string               `json:"created_at"`
	Next          *FeaturedNavDTO      `json:"next"`
	Prev          *FeaturedNavDTO      `json:"prev"`
	PublishedAt   *string              `json:"published_at" extensions:"x-nullable"`
	Related       []FeaturedContentDTO `json:"related" nullability:"nonnil"`
	SortOrder     int                  `json:"sort_order"`
	Source        string               `json:"source"`
	Status        int16                `json:"status"`
	Summary       string               `json:"summary"`
	Title         string               `json:"title"`
	UpdatedAt     string               `json:"updated_at"`
	ViewCount     int                  `json:"view_count"`
}

// FeaturedContentAdminDetailDTO 管理端详情（列表项 + 正文，无相关资讯与导航）。
// scheduled_at（#1748）：排期时间，仅「定时中」非空；编辑器靠它区分「定时」与「草稿」。
type FeaturedContentAdminDetailDTO struct {
	Category      string  `json:"category"`
	CategoryLabel string  `json:"category_label"`
	Content       string  `json:"content"`
	ContentID     int     `json:"content_id"`
	CoverImage    string  `json:"cover_image"`
	CreatedAt     string  `json:"created_at"`
	PublishedAt   *string `json:"published_at" extensions:"x-nullable"`
	ScheduledAt   *string `json:"scheduled_at" extensions:"x-nullable"`
	SortOrder     int     `json:"sort_order"`
	Source        string  `json:"source"`
	Status        int16   `json:"status"`
	Summary       string  `json:"summary"`
	Title         string  `json:"title"`
	UpdatedAt     string  `json:"updated_at"`
	ViewCount     int     `json:"view_count"`
}

// FeaturedContentAdminListDTO 管理端列表项（#1748）。
//
// 为什么另起一份而不是给公开的 FeaturedContentDTO 加一列：公开列表只发已发布内容，
// scheduled_at 在那里恒为 null —— 那是「公开接口零变更」这条承诺上的一个多余键。
// 管理端要它（三态徽标、排期时间列、state 过滤的结果自证），公开面不要它。
type FeaturedContentAdminListDTO struct {
	Category      string  `json:"category"`
	CategoryLabel string  `json:"category_label"`
	ContentID     int     `json:"content_id"`
	CoverImage    string  `json:"cover_image"`
	CreatedAt     string  `json:"created_at"`
	PublishedAt   *string `json:"published_at" extensions:"x-nullable"`
	ScheduledAt   *string `json:"scheduled_at" extensions:"x-nullable"`
	SortOrder     int     `json:"sort_order"`
	Source        string  `json:"source"`
	Status        int16   `json:"status"`
	Summary       string  `json:"summary"`
	Title         string  `json:"title"`
	UpdatedAt     string  `json:"updated_at"`
	ViewCount     int     `json:"view_count"`
}

// FeaturedContentAdminPageResult 管理端列表分页结果（行形状见 FeaturedContentAdminListDTO）。
type FeaturedContentAdminPageResult struct {
	Items []FeaturedContentAdminListDTO `json:"items" nullability:"nonnil"`
	Page  int                           `json:"page"`
	Pages int                           `json:"pages"`
	Total int64                         `json:"total"`
}

// FeaturedNavDTO 上/下一篇导航。
type FeaturedNavDTO struct {
	Category      string  `json:"category"`
	CategoryLabel string  `json:"category_label"`
	ContentID     int     `json:"content_id"`
	PublishedAt   *string `json:"published_at"`
	Title         string  `json:"title"`
}

// FeaturedContentPageResult 内容精选分页结果。
type FeaturedContentPageResult struct {
	Items []FeaturedContentDTO `json:"items" nullability:"nonnil"`
	Page  int                  `json:"page"`
	Pages int                  `json:"pages"`
	Total int64                `json:"total"`
}

// FeaturedContentInput 创建内容精选入参。
// published_at（#1748）：可选发布时间，RFC3339；缺省 = 立即发布，空串 = 用当前时间。
type FeaturedContentInput struct {
	Title       string  `json:"title"`
	Category    string  `json:"category"`
	Summary     string  `json:"summary"`
	CoverImage  string  `json:"cover_image"`
	Content     string  `json:"content"`
	Source      string  `json:"source"`
	Status      *int16  `json:"status"`
	SortOrder   *int    `json:"sort_order"`
	PublishedAt *string `json:"published_at"`
}

// FeaturedContentUpdateInput 更新内容精选入参（指针区分「未携带」与「零值」）。
type FeaturedContentUpdateInput struct {
	Title       *string `json:"title"`
	Category    *string `json:"category"`
	Summary     *string `json:"summary"`
	CoverImage  *string `json:"cover_image"`
	Content     *string `json:"content"`
	Source      *string `json:"source"`
	Status      *int16  `json:"status"`
	SortOrder   *int    `json:"sort_order"`
	PublishedAt *string `json:"published_at"`
}

// FeaturedDeleteResult 删除结果。
type FeaturedDeleteResult struct {
	ContentID int `json:"content_id"`
}

func featuredPublishedAt(publishedAt *time.Time) *string {
	if publishedAt == nil {
		return nil
	}
	v := timefmt.FormatISO(*publishedAt)
	return &v
}

func featuredContentDTO(c *model.FeaturedContent) FeaturedContentDTO {
	return FeaturedContentDTO{
		Category:      c.Category,
		CategoryLabel: featuredCategoryLabel(c.Category),
		ContentID:     c.ContentID,
		CoverImage:    c.CoverImage,
		CreatedAt:     timefmt.FormatISO(c.CreatedAt),
		PublishedAt:   featuredPublishedAt(c.PublishedAt),
		SortOrder:     c.SortOrder,
		Source:        c.Source,
		Status:        c.Status,
		Summary:       c.Summary,
		Title:         c.Title,
		UpdatedAt:     timefmt.FormatISO(c.UpdatedAt),
		ViewCount:     c.ViewCount,
	}
}

func featuredContentDetailDTO(c *model.FeaturedContent) FeaturedContentDetailDTO {
	base := featuredContentDTO(c)
	return FeaturedContentDetailDTO{
		Category:      base.Category,
		CategoryLabel: base.CategoryLabel,
		Content:       c.Content,
		ContentID:     base.ContentID,
		CoverImage:    base.CoverImage,
		CreatedAt:     base.CreatedAt,
		PublishedAt:   base.PublishedAt,
		Related:       []FeaturedContentDTO{},
		SortOrder:     base.SortOrder,
		Source:        base.Source,
		Status:        base.Status,
		Summary:       base.Summary,
		Title:         base.Title,
		UpdatedAt:     base.UpdatedAt,
		ViewCount:     base.ViewCount,
	}
}

func featuredContentAdminDetailDTO(c *model.FeaturedContent) FeaturedContentAdminDetailDTO {
	base := featuredContentDTO(c)
	return FeaturedContentAdminDetailDTO{
		Category:      base.Category,
		CategoryLabel: base.CategoryLabel,
		Content:       c.Content,
		ContentID:     base.ContentID,
		CoverImage:    base.CoverImage,
		CreatedAt:     base.CreatedAt,
		PublishedAt:   base.PublishedAt,
		ScheduledAt:   timefmt.FormatTimePtr(c.ScheduledAt),
		SortOrder:     base.SortOrder,
		Source:        base.Source,
		Status:        base.Status,
		Summary:       base.Summary,
		Title:         base.Title,
		UpdatedAt:     base.UpdatedAt,
		ViewCount:     base.ViewCount,
	}
}

func featuredContentAdminListDTO(c *model.FeaturedContent) FeaturedContentAdminListDTO {
	return FeaturedContentAdminListDTO{
		Category:      c.Category,
		CategoryLabel: featuredCategoryLabel(c.Category),
		ContentID:     c.ContentID,
		CoverImage:    c.CoverImage,
		CreatedAt:     timefmt.FormatISO(c.CreatedAt),
		PublishedAt:   featuredPublishedAt(c.PublishedAt),
		ScheduledAt:   timefmt.FormatTimePtr(c.ScheduledAt),
		SortOrder:     c.SortOrder,
		Source:        c.Source,
		Status:        c.Status,
		Summary:       c.Summary,
		Title:         c.Title,
		UpdatedAt:     timefmt.FormatISO(c.UpdatedAt),
		ViewCount:     c.ViewCount,
	}
}

func featuredNavDTO(c *model.FeaturedContent) FeaturedNavDTO {
	return FeaturedNavDTO{
		Category:      c.Category,
		CategoryLabel: featuredCategoryLabel(c.Category),
		ContentID:     c.ContentID,
		PublishedAt:   featuredPublishedAt(c.PublishedAt),
		Title:         c.Title,
	}
}
