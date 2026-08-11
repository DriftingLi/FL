// 已迁移模块：走 unwrappedRequest（拦截器解包信封，成功直接返回业务数据 Promise<T>，
// 业务失败抛错并统一 toast，调用方不再自检 res.code）
import { unwrappedRequest } from './request'
import type { AxiosRequestConfig } from 'axios'

export interface LoginPayload {
  username: string
  password: string
}

/** 登录 / /auth/me 返回的用户信息（username 为昵称，uid 为字符串形式的雪花 ID） */
export interface AuthUserInfo {
  token?: string
  user_id?: number
  uid?: string
  account?: string
  username?: string
  name?: string
  avatar_url?: string
  role?: string
  email?: string
  phone?: string
  [key: string]: any
}

export const authApi = {
  login(data: LoginPayload) {
    return unwrappedRequest.post<AuthUserInfo>('/auth/login', data)
  },

  adminLogin(data: LoginPayload) {
    return unwrappedRequest.post<AuthUserInfo>('/auth/admin-login', data)
  },

  tutorLogin(data: LoginPayload) {
    return unwrappedRequest.post<AuthUserInfo>('/auth/tutor-login', data)
  },

  logout() {
    return unwrappedRequest.post<null>('/auth/logout')
  },

  getUserInfo(config?: AxiosRequestConfig) {
    return unwrappedRequest.get<AuthUserInfo>('/auth/me', config)
  },

  updateProfile(data: { nickname: string }) {
    return unwrappedRequest.put<AuthUserInfo>('/auth/profile', data)
  },

  uploadAvatar(formData: FormData) {
    return unwrappedRequest.post<{ url: string }>('/auth/avatar', formData, {
      headers: { 'Content-Type': 'multipart/form-data' },
      timeout: 120000
    })
  },

  // ===== 邮箱验证码注册/登录 =====

  sendEmailCode(data: { email: string; purpose: 'register' | 'login' }) {
    return unwrappedRequest.post<null>('/auth/email/send-code', data)
  },

  emailRegister(data: { email: string; code: string; nickname: string; company?: string; password: string }) {
    return unwrappedRequest.post<AuthUserInfo>('/auth/email/register', data)
  },

  emailLogin(data: { email: string; code: string }) {
    return unwrappedRequest.post<AuthUserInfo>('/auth/email/login', data)
  },

  // ===== 手机号验证码注册/登录（与邮箱流程对齐）=====

  sendPhoneCode(data: { phone: string; purpose: 'register' | 'login' }) {
    return unwrappedRequest.post<null>('/auth/phone/send-code', data)
  },

  phoneRegister(data: { phone: string; code: string; nickname: string; company?: string; password: string }) {
    return unwrappedRequest.post<AuthUserInfo>('/auth/phone/register', data)
  },

  phoneLogin(data: { phone: string; code: string }) {
    return unwrappedRequest.post<AuthUserInfo>('/auth/phone/login', data)
  },

  // ===== 微信扫码（框架占位）=====

  getWechatQRCode() {
    return unwrappedRequest.post<{ qr_code_url?: string }>('/auth/wechat/qrcode')
  },

  // ===== 个人信息：绑定/修改手机号、邮箱、密码 =====

  sendProfileCode(data: { channel: 'email' | 'phone'; target: string }) {
    return unwrappedRequest.post<null>('/auth/profile/send-code', data)
  },

  updateProfileEmail(data: { email: string; code: string }) {
    return unwrappedRequest.post<AuthUserInfo>('/auth/profile/email', data)
  },

  updateProfilePhone(data: { phone: string; code: string }) {
    return unwrappedRequest.post<AuthUserInfo>('/auth/profile/phone', data)
  },

  updateProfilePassword(password: string) {
    return unwrappedRequest.post<null>('/auth/profile/password', { password })
  },

  // ===== 修改登录账号（短信验证码确认）=====

  sendAccountChangeCode() {
    return unwrappedRequest.post<null>('/auth/account/send-code')
  },

  updateAccount(data: { account: string; code: string }) {
    return unwrappedRequest.put<null>('/auth/account', data)
  }
}
