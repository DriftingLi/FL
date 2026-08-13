// 已迁移模块：走 unwrappedRequest（拦截器解包信封，成功直接返回业务数据 Promise<T>，
// 业务失败抛错并统一 toast，调用方不再自检 res.code）
import { unwrappedRequest } from './request'
import type { AxiosRequestConfig } from 'axios'
import type { UserProfile } from '@/types/user'

export interface LoginPayload {
  username: string
  password: string
}

export const authApi = {
  login(data: LoginPayload) {
    return unwrappedRequest.post<UserProfile>('/auth/login', data)
  },

  adminLogin(data: LoginPayload) {
    return unwrappedRequest.post<UserProfile>('/auth/admin-login', data)
  },

  tutorLogin(data: LoginPayload) {
    return unwrappedRequest.post<UserProfile>('/auth/tutor-login', data)
  },

  logout() {
    return unwrappedRequest.post<null>('/auth/logout')
  },

  getUserInfo(config?: AxiosRequestConfig) {
    return unwrappedRequest.get<UserProfile>('/auth/me', config)
  },

  updateProfile(data: { nickname: string }) {
    return unwrappedRequest.put<UserProfile>('/auth/profile', data)
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
    return unwrappedRequest.post<UserProfile>('/auth/email/register', data)
  },

  emailLogin(data: { email: string; code: string }) {
    return unwrappedRequest.post<UserProfile>('/auth/email/login', data)
  },

  // ===== 手机号验证码注册/登录（与邮箱流程对齐）=====

  sendPhoneCode(data: { phone: string; purpose: 'register' | 'login' }) {
    return unwrappedRequest.post<null>('/auth/phone/send-code', data)
  },

  phoneRegister(data: { phone: string; code: string; nickname: string; company?: string; password: string }) {
    return unwrappedRequest.post<UserProfile>('/auth/phone/register', data)
  },

  phoneLogin(data: { phone: string; code: string }) {
    return unwrappedRequest.post<UserProfile>('/auth/phone/login', data)
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
    return unwrappedRequest.post<UserProfile>('/auth/profile/email', data)
  },

  updateProfilePhone(data: { phone: string; code: string }) {
    return unwrappedRequest.post<UserProfile>('/auth/profile/phone', data)
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
