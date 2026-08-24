import { unwrappedRequest } from './request'

export interface CredentialDict {
  id: number
  code: string
  name: string
  description: string
  category: 'special_operation' | 'skill_level'
  level: number | null
  sort_order: number
  status: number
  created_at: string
  updated_at: string
}

export type GroupedCredentials = {
  special_operation: CredentialDict[]
  skill_level: CredentialDict[]
}

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
    return unwrappedRequest.get<{ credentials: CredentialDict[] }>('/credentials')
  },
  listGrouped() {
    return unwrappedRequest.get<GroupedCredentials>('/credentials/grouped')
  },
  listAdminCredentials() {
    return unwrappedRequest.get<{ credentials: CredentialDict[] }>('/admin/credentials')
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
  getCurrent() {
    return unwrappedRequest.get<{ credential: CredentialDict | null }>('/me/credential')
  },
  setCurrent(credential_id: number) {
    return unwrappedRequest.patch<{ credential: CredentialDict }>('/me/credential', { credential_id })
  }
}
