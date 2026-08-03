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

  emailRegister(data: { email: string; code: string; name: string; company?: string }) {
    return request.post('/auth/email/register', data)
  },

  emailLogin(data: { email: string; code: string }) {
    return request.post('/auth/email/login', data)
  }
}
