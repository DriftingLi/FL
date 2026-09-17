// 生成文件，勿手改（ADR-0019 契约 codegen 专项 / ADR-0048 按域解冻；spec #940 片五③、#952 片一）。
// 域：帮助中心（/api/faq 学员端只读；/api/admin/faq 管理端分类与条目 CRUD）
// 唯一事实源：后端注解 → backend/docs/swagger.json（CI 有新鲜度锁：backend-lint 的 swagger 步骤）。
// 再生成：cd backend && go run ./cmd/gen-apitypes
// 同步契约：backend/internal/apitypes/codegen_test.go 把本文件与注解渲染结果全等比对。
//
// 覆盖端点：
//   GET  /faq
//   GET  /admin/faq/categories
//   POST /admin/faq/categories
//   PUT  /admin/faq/categories/{id}
//   DELETE /admin/faq/categories/{id}
//   GET  /admin/faq/entries
//   POST /admin/faq/entries
//   PUT  /admin/faq/entries/{id}
//   DELETE /admin/faq/entries/{id}
//
// 覆盖的 Go 类型：AdminFaqCategoriesResult / AdminFaqCategoryDTO / AdminFaqEntriesResult / AdminFaqEntryDTO / FaqCategoryDTO / FaqEntryDTO / FaqResult
//
// 可空性 / 缺省态由**注解层**表达，生成器只如实转写（Go 结构体 tag）：
//   - extensions:"x-nullable" → 字段渲染 'T | null'：键一定在，值为 null（Go 指针且无 omitempty）；
//   - extensions:"x-optional" → 字段渲染 'T?'：键**可能整个不存在**（Go omitempty）；
//   - 两者可同时标注（'T?' 且 '| null'）；未标注的一律按「键一定在、非 null」渲染 ——
//     swag 看不到 Go 的 omitempty，漏标即契约撒谎。
// 其余已知限制：
//   - Go 侧 any 字段在 swagger 里是空 schema，渲染 'unknown'（不猜结构）；
//   - 不生成 query / body 的入参类型（只生成响应形状）。
// 需要更精确的形状时先在注解层补齐（先例见 spec #940 片五②的差集清单）。

export interface AdminFaqCategoriesResult {
  categories: AdminFaqCategoryDTO[]
}

export interface AdminFaqCategoryDTO {
  code: string
  enabled: boolean
  entry_count: number
  id: number
  sort_order: number
  title: string
}

export interface AdminFaqEntriesResult {
  entries: AdminFaqEntryDTO[]
}

export interface AdminFaqEntryDTO {
  answer: string
  category_code: string
  category_id: number
  id: number
  published: boolean
  question: string
  sort_order: number
}

export interface FaqCategoryDTO {
  code: string
  entries: FaqEntryDTO[]
  id: number
  sort_order: number
  title: string
}

export interface FaqEntryDTO {
  answer: string
  id: number
  question: string
  sort_order: number
}

export interface FaqResult {
  categories: FaqCategoryDTO[]
}
