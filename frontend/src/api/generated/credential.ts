// 生成文件，勿手改（ADR-0019 契约 codegen 专项 / ADR-0048 按域解冻；spec #940 片五③、#952 片一）。
// 域：目标证件（/api/credentials*、/api/me/credential：公开列表 / 分组 / 管理端 CRUD / 当前证件）
// 唯一事实源：后端注解 → backend/docs/swagger.json（CI 有新鲜度锁：backend-lint 的 swagger 步骤）。
// 再生成：cd backend && go run ./cmd/gen-apitypes
// 同步契约：backend/internal/apitypes/codegen_test.go 把本文件与注解渲染结果全等比对。
//
// 覆盖端点：
//   GET  /credentials
//   GET  /credentials/grouped
//   GET  /admin/credentials
//   POST /admin/credential
//   PUT  /admin/credential/{id}
//   DELETE /admin/credential/{id}
//   PUT  /admin/credential/{id}/sort
//   GET  /me/credential
//   PATCH /me/credential
//
// 覆盖的 Go 类型：CredentialDict / CredentialListDTO / CurrentCredentialDTO / GroupedCredentialsDTO
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

export interface CredentialDict {
  category: string
  code: string
  created_at: string
  description: string
  id: number
  level: number | null
  name: string
  sort_order: number
  status: number
  updated_at: string
}

export interface CredentialListDTO {
  credentials: CredentialDict[]
}

export interface CurrentCredentialDTO {
  credential: CredentialDict | null
}

export interface GroupedCredentialsDTO {
  skill_level: CredentialDict[]
  special_operation: CredentialDict[]
}
