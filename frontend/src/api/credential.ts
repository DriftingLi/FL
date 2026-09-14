// 已迁移模块：响应类型**不再手写**，唯一事实源是后端注解 → backend/docs/swagger.json →
// `cd backend && go run ./cmd/gen-apitypes`（ADR-0048 决策 1/3，issue #964 片六）。
// 入参（body）类型不生成、仍手写（决策 3）。
import { unwrappedRequest } from './request'
import type { CredentialDict, CredentialListDTO, CurrentCredentialDTO, GroupedCredentialsDTO } from './generated/credential'

export type { CredentialDict, CredentialListDTO, CurrentCredentialDTO, GroupedCredentialsDTO }

/** 证件分组（special_operation / skill_level 两组恒在）—— 旧名保留为生成别名 */
export type GroupedCredentials = GroupedCredentialsDTO

/** 创建/更新证件入参（不生成，ADR-0048 决策 3） */
export interface CredentialPayload {
  code: string
  name: string
  category: 'special_operation' | 'skill_level'
  level?: number | null
  description?: string
  sort_order?: number
  status?: number
}

export const credentialApi = {
  listCredentials() {
    return unwrappedRequest.get<CredentialListDTO>('/credentials')
  },
  listGrouped() {
    return unwrappedRequest.get<GroupedCredentialsDTO>('/credentials/grouped')
  },
  listAdminCredentials() {
    return unwrappedRequest.get<CredentialListDTO>('/admin/credentials')
  },
  createCredential(data: CredentialPayload) {
    return unwrappedRequest.post<CredentialDict>('/admin/credential', data)
  },
  updateCredential(id: number, data: Partial<CredentialPayload>) {
    return unwrappedRequest.put<CredentialDict>(`/admin/credential/${id}`, data)
  },
  deleteCredential(id: number) {
    return unwrappedRequest.delete<null>(`/admin/credential/${id}`)
  },
  swapCredential(id: number, swapWith: number) {
    return unwrappedRequest.put<null>(`/admin/credential/${id}/sort`, { swap_with: swapWith })
  },
  /** 当前证件：后端恒回 {credential: <dict|null>}（未选择证件时为 null） */
  getCurrent() {
    return unwrappedRequest.get<CurrentCredentialDTO>('/me/credential')
  },
  setCurrent(credential_id: number) {
    return unwrappedRequest.patch<CurrentCredentialDTO>('/me/credential', { credential_id })
  }
}
