// 已迁移模块：走 unwrappedRequest（拦截器解包信封，成功直接返回业务数据 Promise<T>，
// 业务失败抛错并统一 toast，调用方不再自检 res.code）
import { unwrappedRequest } from './request'
import type { AxiosRequestConfig } from 'axios'
import type { UserProfile } from '@/types/user'
import { getRefreshToken } from '@/utils/storage'

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
    // 双令牌（ADR-0012）：撤销请求体携带的 refresh token（登出后 refresh 失效）
    const refresh_token = getRefreshToken() || ''
    return unwrappedRequest.post<null>('/auth/logout', { refresh_token })
  },

  getUserInfo(config?: AxiosRequestConfig) {
    return unwrappedRequest.get<UserProfile>('/auth/me', config)
  },

  updateProfile(data: { nickname?: string; company?: string }) {
    return unwrappedRequest.put<UserProfile>('/auth/profile', data)
  },

  updateCompany(data: { company: string }) {
    return unwrappedRequest.put<UserProfile>('/auth/profile', data)
  },

  deleteAccount() {
    return unwrappedRequest.delete<null>('/auth/account')
  },

  uploadAvatar(formData: FormData) {
    return unwrappedRequest.post<{ url: string }>('/auth/avatar', formData, {
      headers: { 'Content-Type': 'multipart/form-data' },
      timeout: 120000
    })
  },

  // ===== 邮箱验证码注册/登录 =====

  sendEmailCode(data: { email: string; purpose: 'register' | 'login' | 'reset_password'; captcha_id: string; captcha_value: string }) {
    return unwrappedRequest.post<null>('/auth/email/send-code', data)
  },

  emailRegister(data: { email: string; code: string; nickname: string; company?: string; password: string }) {
    return unwrappedRequest.post<UserProfile>('/auth/email/register', data)
  },

  emailLogin(data: { email: string; code: string }) {
    return unwrappedRequest.post<UserProfile>('/auth/email/login', data)
  },

  // ===== 手机号验证码注册/登录（与邮箱流程对齐）=====

  sendPhoneCode(data: { phone: string; purpose: 'register' | 'login' | 'reset_password'; captcha_id: string; captcha_value: string }) {
    return unwrappedRequest.post<null>('/auth/phone/send-code', data)
  },

  phoneRegister(data: { phone: string; code: string; nickname: string; company?: string; password: string }) {
    return unwrappedRequest.post<UserProfile>('/auth/phone/register', data)
  },

  phoneLogin(data: { phone: string; code: string }) {
    return unwrappedRequest.post<UserProfile>('/auth/phone/login', data)
  },

  // ===== 忘记密码（验证码重置）=====

  emailResetPassword(data: { email: string; code: string; password: string }) {
    return unwrappedRequest.post<null>('/auth/email/reset-password', data)
  },

  phoneResetPassword(data: { phone: string; code: string; password: string }) {
    return unwrappedRequest.post<null>('/auth/phone/reset-password', data)
  },

  // ===== 图形验证码（人机验证）=====

  getCaptcha() {
    return unwrappedRequest.get<{ id: string; image: string }>('/captcha')
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

  updateProfilePassword(data: { code: string; password: string }) {
    return unwrappedRequest.post<null>('/auth/profile/password', data)
  },

  sendChangePasswordCode() {
    return unwrappedRequest.post<null>('/auth/profile/password/send-code')
  },

  // ===== 修改登录账号（短信验证码确认）=====

  sendAccountChangeCode() {
    return unwrappedRequest.post<null>('/auth/account/send-code')
  },

  updateAccount(data: { account: string; code: string }) {
    return unwrappedRequest.put<UserProfile>('/auth/account', data)
  }
}
