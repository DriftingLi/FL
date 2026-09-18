// Package paging 通用分页查询：参数钳制 + count/find + 信封字段。
// 主体系与估值模块共用（列表接口样板单一实现）。
//
// 信封登记（ADR-0056 §1 / issue #1095）：**声明表**在 internal/api/envelope_registry.go
// （那里能同时引用 service / api / valuation model 三类结果类型；本包若引用它们会成环）。
// 本文件只放装配原语：Dialect 方言词汇 + ItemsPage typed page。
package paging

// Dialect 分页元数据方言（只登记不改名：改名是破坏性契约变更，违反 ADR-0048「只增不破」）。
type Dialect string

const (
	// DialectPages 用 page + pages 表达页数（13 个域）。
	DialectPages Dialect = "pages"
	// DialectPageSize 用 page + page_size 表达页大小（10 个域）。
	DialectPageSize Dialect = "page_size"
	// DialectNone 只有 total（无 pages / page_size 键）。
	DialectNone Dialect = "none"
)

// ItemsPage 通用分页信封（typed page）：items + page + page_size + total。
//
// 键序与 api 层既有 gin.H{"items","page","page_size","total"} 的输出逐字节一致
// （encoding/json 对 map 按 key 排序：items < page < page_size < total），
// 供没有域内 page DTO 的列表端点（巡检两条读路径）替代手拼 map；新端点应优先用它，
// 而不是再抄一个只有字段名不同的 page struct。
type ItemsPage[T any] struct {
	Items    []T   `json:"items"`
	Page     int   `json:"page"`
	PageSize int   `json:"page_size"`
	Total    int64 `json:"total"`
}
