// 估值模块独立认证 API（与主体系 authApi 完全隔离）
// 调用 /api/valuation/auth/* 路径，使用独立的 valuation_token
import client from './client'

export interface ValuationLoginReq {
  account: string // 用户名或手机号
  password: string
}

export interface ValuationRegisterReq {
  phone: string
  password: string
  name: string
  email?: string
  company?: string
}

export interface ValuationLoginRes {
  token: string
  user_id: number
  username: string
  name: string
  role: string // 固定 'valuation_user'
}

export interface ValuationUserInfo {
  token?: string
  user_id?: number
  username?: string
  name?: string
  phone?: string
  email?: string
  company?: string
  role?: string // 固定 'valuation_user'
  [key: string]: any
}

export const valuationAuthApi = {
  /** POST /api/valuation/auth/login */
  login(data: ValuationLoginReq) {
    return client.post<ValuationLoginRes>('/auth/login', data)
  },
  /** POST /api/valuation/auth/register */
  register(data: ValuationRegisterReq) {
    return client.post<{ id: number; username: string; name: string; phone: string }>('/auth/register', data)
  },
  /** GET /api/valuation/auth/me （需 ValuationJWTAuth） */
  me() {
    return client.get<ValuationUserInfo>('/auth/me')
  },
  /** POST /api/valuation/auth/logout （需 ValuationJWTAuth） */
  logout() {
    return client.post('/auth/logout')
  }
}
