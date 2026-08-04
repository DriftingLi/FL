import request from './request'
import type { AxiosRequestConfig } from 'axios'

export interface LoginPayload {
  username: string
  password: string
}

export interface RegisterPayload {
  phone: string
  password: string
  name: string
  email?: string
  company?: string
}

/** 登录 / /auth/me 返回的用户信息 */
export interface AuthUserInfo {
  token?: string
  user_id?: number
  username?: string
  name?: string
  nickname?: string
  avatar_url?: string
  role?: string
  email?: string
  phone?: string
  [key: string]: any
}

export const authApi = {
  login(data: LoginPayload) {
    return request.post<AuthUserInfo>('/auth/login', data)
  },

  register(data: RegisterPayload) {
    return request.post<AuthUserInfo>('/auth/register', data)
  },

  adminLogin(data: LoginPayload) {
    return request.post<AuthUserInfo>('/auth/admin-login', data)
  },

  tutorLogin(data: LoginPayload) {
    return request.post<AuthUserInfo>('/auth/tutor-login', data)
  },

  logout() {
    return request.post<null>('/auth/logout')
  },

  getUserInfo(config?: AxiosRequestConfig) {
    return request.get<AuthUserInfo>('/auth/me', config)
  },

  updateProfile(data: { nickname: string }) {
    return request.put<AuthUserInfo>('/auth/profile', data)
  },

  uploadAvatar(formData: FormData) {
    return request.post<{ url: string }>('/auth/avatar', formData, {
      headers: { 'Content-Type': 'multipart/form-data' },
      timeout: 120000
    })
  },

  // ===== 邮箱验证码注册/登录 =====

  sendEmailCode(data: { email: string; purpose: 'register' | 'login' }) {
    return request.post<null>('/auth/email/send-code', data)
  },

  emailRegister(data: { email: string; code: string; nickname: string; company?: string; password: string }) {
    return request.post<AuthUserInfo>('/auth/email/register', data)
  },

  emailLogin(data: { email: string; code: string }) {
    return request.post<AuthUserInfo>('/auth/email/login', data)
  },

  // ===== 手机号验证码注册/登录（与邮箱流程对齐）=====

  sendPhoneCode(data: { phone: string; purpose: 'register' | 'login' }) {
    return request.post<null>('/auth/phone/send-code', data)
  },

  phoneRegister(data: { phone: string; code: string; nickname: string; company?: string; password: string }) {
    return request.post<AuthUserInfo>('/auth/phone/register', data)
  },

  phoneLogin(data: { phone: string; code: string }) {
    return request.post<AuthUserInfo>('/auth/phone/login', data)
  },

  // ===== 微信扫码（框架占位）=====

  getWechatQRCode() {
    return request.post<{ qr_code_url?: string }>('/auth/wechat/qrcode')
  },

  // ===== 个人信息：绑定/修改手机号、邮箱、密码 =====

  sendProfileCode(data: { channel: 'email' | 'phone'; target: string }) {
    return request.post<null>('/auth/profile/send-code', data)
  },

  updateProfileEmail(data: { email: string; code: string }) {
    return request.post<AuthUserInfo>('/auth/profile/email', data)
  },

  updateProfilePhone(data: { phone: string; code: string }) {
    return request.post<AuthUserInfo>('/auth/profile/phone', data)
  },

  updateProfilePassword(password: string) {
    return request.post<null>('/auth/profile/password', { password })
  }
}
