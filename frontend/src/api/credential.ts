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
  getCurrent() {
    return unwrappedRequest.get<{ credential: CredentialDict | null }>('/me/credential')
  },
  setCurrent(credential_id: number) {
    return unwrappedRequest.patch<{ credential: CredentialDict }>('/me/credential', { credential_id })
  }
}
