// Package paging 通用分页查询：参数钳制 + count/find + 信封字段。
// 主体系与估值模块共用（列表接口样板单一实现）。
package paging

import "gorm.io/gorm"

// Clamp 分页参数钳制：<=0 时回退默认值。
func Clamp(page, pageSize, defaultPageSize int) (int, int) {
	if page <= 0 {
		page = 1
	}
	if pageSize <= 0 {
		pageSize = defaultPageSize
	}
	return page, pageSize
}

// ClampMax 分页参数钳制（带上限）：<=0 回退默认值，>max 也回退默认值。
// 用于「页大小有上限」的列表（超上限回退默认，而非截断到上限）。
func ClampMax(page, pageSize, defaultPageSize, maxPageSize int) (int, int) {
	page, pageSize = Clamp(page, pageSize, defaultPageSize)
	if pageSize > maxPageSize {
		pageSize = defaultPageSize
	}
	return page, pageSize
}

// Page 分页信封字段（total/page/page_size，各列表接口共用）。
type Page struct {
	Total    int64 `json:"total"`
	Page     int   `json:"page"`
	PageSize int   `json:"page_size"`
}

// Query 分页查询：钳制 → count → find（同一过滤条件作用域）。
// build 附加过滤条件（可选）；order 为空跳过排序。返回 (items, total, page, pageSize)。
func Query[T any](db *gorm.DB, page, pageSize, defaultPageSize int, order string, build func(q *gorm.DB) *gorm.DB) ([]T, int64, int, int) {
	page, pageSize = Clamp(page, pageSize, defaultPageSize)
	q := db.Model(new(T))
	if build != nil {
		q = build(q)
	}
	var total int64
	q.Count(&total)
	if order != "" {
		q = q.Order(order)
	}
	var items []T
	q.Offset((page - 1) * pageSize).Limit(pageSize).Find(&items)
	return items, total, page, pageSize
}
