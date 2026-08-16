// Package service 内容精选 typed surface。
// JSON 字段声明按 key 字母序排列，与旧 map 字典序列化的字节序保持一致（ADR-0009）。
package service

import (
	"time"

	"forklift-training/internal/model"
)

// FeaturedContentDTO 精选内容列表项（不含正文）。
type FeaturedContentDTO struct {
	Category      string  `json:"category"`
	CategoryLabel string  `json:"category_label"`
	ContentID     int     `json:"content_id"`
	CoverImage    string  `json:"cover_image"`
	CreatedAt     string  `json:"created_at"`
	PublishedAt   *string `json:"published_at"`
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
	PublishedAt   *string              `json:"published_at"`
	Related       []FeaturedContentDTO `json:"related"`
	SortOrder     int                  `json:"sort_order"`
	Source        string               `json:"source"`
	Status        int16                `json:"status"`
	Summary       string               `json:"summary"`
	Title         string               `json:"title"`
	UpdatedAt     string               `json:"updated_at"`
	ViewCount     int                  `json:"view_count"`
}

// FeaturedContentAdminDetailDTO 管理端详情（列表项 + 正文，无相关资讯与导航）。
type FeaturedContentAdminDetailDTO struct {
	Category      string  `json:"category"`
	CategoryLabel string  `json:"category_label"`
	Content       string  `json:"content"`
	ContentID     int     `json:"content_id"`
	CoverImage    string  `json:"cover_image"`
	CreatedAt     string  `json:"created_at"`
	PublishedAt   *string `json:"published_at"`
	SortOrder     int     `json:"sort_order"`
	Source        string  `json:"source"`
	Status        int16   `json:"status"`
	Summary       string  `json:"summary"`
	Title         string  `json:"title"`
	UpdatedAt     string  `json:"updated_at"`
	ViewCount     int     `json:"view_count"`
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
	Items []FeaturedContentDTO `json:"items"`
	Page  int                  `json:"page"`
	Pages int                  `json:"pages"`
	Total int64                `json:"total"`
}

// FeaturedContentInput 创建内容精选入参。
type FeaturedContentInput struct {
	Title      string `json:"title"`
	Category   string `json:"category"`
	Summary    string `json:"summary"`
	CoverImage string `json:"cover_image"`
	Content    string `json:"content"`
	Source     string `json:"source"`
	Status     *int16 `json:"status"`
	SortOrder  *int   `json:"sort_order"`
}

// FeaturedContentUpdateInput 更新内容精选入参（指针区分「未携带」与「零值」）。
type FeaturedContentUpdateInput struct {
	Title      *string `json:"title"`
	Category   *string `json:"category"`
	Summary    *string `json:"summary"`
	CoverImage *string `json:"cover_image"`
	Content    *string `json:"content"`
	Source     *string `json:"source"`
	Status     *int16  `json:"status"`
	SortOrder  *int    `json:"sort_order"`
}

// FeaturedDeleteResult 删除结果。
type FeaturedDeleteResult struct {
	ContentID int `json:"content_id"`
}

func featuredPublishedAt(publishedAt *time.Time) *string {
	if publishedAt == nil {
		return nil
	}
	v := formatISO(*publishedAt)
	return &v
}

func featuredContentDTO(c *model.FeaturedContent) FeaturedContentDTO {
	return FeaturedContentDTO{
		Category:      c.Category,
		CategoryLabel: featuredCategoryLabel(c.Category),
		ContentID:     c.ContentID,
		CoverImage:    c.CoverImage,
		CreatedAt:     formatISO(c.CreatedAt),
		PublishedAt:   featuredPublishedAt(c.PublishedAt),
		SortOrder:     c.SortOrder,
		Source:        c.Source,
		Status:        c.Status,
		Summary:       c.Summary,
		Title:         c.Title,
		UpdatedAt:     formatISO(c.UpdatedAt),
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
		SortOrder:     base.SortOrder,
		Source:        base.Source,
		Status:        base.Status,
		Summary:       base.Summary,
		Title:         base.Title,
		UpdatedAt:     base.UpdatedAt,
		ViewCount:     base.ViewCount,
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
