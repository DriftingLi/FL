// 已迁移模块：响应类型**不再手写**，唯一事实源是后端注解 → backend/docs/swagger.json →
// `cd backend && go run ./cmd/gen-apitypes`（ADR-0048 决策 1/3，issue #964 片六）。
// 入参（query）类型不生成、仍手写（决策 3）。
import { unwrappedRequest } from './request'
import type { MaterialDTO, MaterialPageResult } from './generated/material'

export type { MaterialDTO, MaterialPageResult }

/** 学习资料条目（chapter_file 聚合视图，与后端 MaterialDTO 对齐）—— 旧名保留为生成别名 */
export type MaterialItem = MaterialDTO

/** 资料列表响应——旧名保留为生成别名 */
export type MaterialListData = MaterialPageResult

export const materialApi = {
  list(params: { course_id?: number; page?: number; page_size?: number }) {
    return unwrappedRequest.get<MaterialPageResult>('/materials', { params })
  }
}
