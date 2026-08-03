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

export const authApi = {
  login(data: LoginPayload) {
    return request.post('/auth/login', data)
  },

  register(data: RegisterPayload) {
    return request.post('/auth/register', data)
  },

  adminLogin(data: LoginPayload) {
    return request.post('/auth/admin-login', data)
  },

  tutorLogin(data: LoginPayload) {
    return request.post('/auth/tutor-login', data)
  },

  logout() {
    return request.post('/auth/logout')
  },

  getUserInfo(config?: AxiosRequestConfig) {
    return request.get('/auth/me', config)
  },

  updateProfile(data: { nickname: string }) {
    return request.put('/auth/profile', data)
  },

  uploadAvatar(formData: FormData) {
    return request.post('/auth/avatar', formData, {
      headers: { 'Content-Type': 'multipart/form-data' },
      timeout: 120000
    })
  },

  // ===== 邮箱验证码注册/登录 =====

  sendEmailCode(data: { email: string; purpose: 'register' | 'login' }) {
    return request.post('/auth/email/send-code', data)
  },

  emailRegister(data: { email: string; code: string; name: string; company?: string; password: string }) {
    return request.post('/auth/email/register', data)
  },

  emailLogin(data: { email: string; code: string }) {
    return request.post('/auth/email/login', data)
  },

  // ===== 手机号验证码注册/登录（与邮箱流程对齐）=====

  sendPhoneCode(data: { phone: string; purpose: 'register' | 'login' }) {
    return request.post('/auth/phone/send-code', data)
  },

  phoneRegister(data: { phone: string; code: string; name: string; company?: string; password: string }) {
    return request.post('/auth/phone/register', data)
  },

  phoneLogin(data: { phone: string; code: string }) {
    return request.post('/auth/phone/login', data)
  },

  // ===== 微信扫码（框架占位）=====

  getWechatQRCode() {
    return request.post('/auth/wechat/qrcode')
  },

  // ===== 个人信息：绑定/修改手机号、邮箱、密码 =====

  sendProfileCode(data: { channel: 'email' | 'phone'; target: string }) {
    return request.post('/auth/profile/send-code', data)
  },

  updateProfileEmail(data: { email: string; code: string }) {
    return request.post('/auth/profile/email', data)
  },

  updateProfilePhone(data: { phone: string; code: string }) {
    return request.post('/auth/profile/phone', data)
  },

  updateProfilePassword(password: string) {
    return request.post('/auth/profile/password', { password })
  }
}
