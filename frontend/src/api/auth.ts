// 已迁移模块：走 unwrappedRequest（拦截器解包信封，成功直接返回业务数据 Promise<T>，
// 业务失败抛错并统一 toast，调用方不再自检 res.code）
//
// 响应类型**不再手写**：唯一事实源是后端注解 → backend/docs/swagger.json →
// `cd backend && go run ./cmd/gen-apitypes`（ADR-0048 决策 1/3，issue #959 片三）。
// 本文件只留请求壳、端点装配与名称适配，入参（query / body）类型不生成、仍手写。
//
// **边界**：`@/types/user` 的 `UserProfile` 是**会话态 UI 模型**（token + 「登录响应基础字段 ∪
// /auth/me 全量资料」的合并结果，见 stores/auth.ts 的注释），不是某一个端点的线格式 ——
// 它由 store 持有并落 localStorage，故保留手写。本模块的响应一律用生成类型
// （`LoginResult` / `ProfileDTO` / `ProfileChangeRequestDTO` …）；调用方把生成类型交给
// `setAuthData` 时按结构兼容（UserProfile 全字段可选）即可。
import { unwrappedRequest } from './request'
import type { AxiosRequestConfig } from 'axios'
import type {
  GenerateCaptchaDTO,
  LoginResult,
  ProfileChangeRequestDTO,
  ProfileDTO,
  WechatQRCodeInfoDTO,
  WxLoginResult
} from './generated/auth'
import { getRefreshToken } from '@/utils/storage'

export type { GenerateCaptchaDTO, LoginResult, ProfileChangeRequestDTO, ProfileDTO, WechatQRCodeInfoDTO, WxLoginResult }

export interface LoginPayload {
  username: string
  password: string
}

export const authApi = {
  login(data: LoginPayload) {
    return unwrappedRequest.post<LoginResult>('/auth/login', data)
  },

  adminLogin(data: LoginPayload) {
    return unwrappedRequest.post<LoginResult>('/auth/admin-login', data)
  },

  tutorLogin(data: LoginPayload) {
    return unwrappedRequest.post<LoginResult>('/auth/tutor-login', data)
  },

  recruiterLogin(data: LoginPayload) {
    return unwrappedRequest.post<LoginResult>('/auth/recruiter-login', data)
  },

  logout() {
    // refresh_token 缺失时发空串（与迁移前逐字一致；Go 侧空串/缺键同为未提供）
    const refresh_token = getRefreshToken() || ''
    return unwrappedRequest.post<null>('/auth/logout', { refresh_token })
  },

  getUserInfo(config?: AxiosRequestConfig) {
    return unwrappedRequest.get<ProfileDTO>('/auth/me', config)
  },

  updateProfile(data: { nickname?: string; company?: string }) {
    return unwrappedRequest.put<ProfileChangeRequestDTO>('/auth/profile', data)
  },

  updateCompany(data: { company: string }) {
    return unwrappedRequest.put<ProfileChangeRequestDTO>('/auth/profile', data)
  },

  deleteAccount() {
    return unwrappedRequest.delete<null>('/auth/account')
  },

  /** 头像上传：后端落一条资料变更审核单（**不是** { url }），审核通过后生效 */
  uploadAvatar(formData: FormData) {
    return unwrappedRequest.post<ProfileChangeRequestDTO>('/auth/avatar', formData, {
      headers: { 'Content-Type': 'multipart/form-data' },
      timeout: 120000
    })
  },

  // ===== 邮箱验证码注册/登录 =====

  sendEmailCode(data: { email: string; purpose: 'register' | 'login' | 'reset_password'; captcha_id: string; captcha_value: string }) {
    return unwrappedRequest.post<null>('/auth/email/send-code', data)
  },

  emailRegister(data: { email: string; code: string; nickname: string; company?: string; password: string }) {
    return unwrappedRequest.post<LoginResult>('/auth/email/register', data)
  },

  emailLogin(data: { email: string; code: string }) {
    return unwrappedRequest.post<LoginResult>('/auth/email/login', data)
  },

  // ===== 手机号验证码注册/登录（与邮箱流程对齐）=====

  sendPhoneCode(data: { phone: string; purpose: 'register' | 'login' | 'reset_password'; captcha_id: string; captcha_value: string }) {
    return unwrappedRequest.post<null>('/auth/phone/send-code', data)
  },

  phoneRegister(data: { phone: string; code: string; nickname: string; company?: string; password: string }) {
    return unwrappedRequest.post<LoginResult>('/auth/phone/register', data)
  },

  phoneLogin(data: { phone: string; code: string }) {
    return unwrappedRequest.post<LoginResult>('/auth/phone/login', data)
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
    return unwrappedRequest.get<GenerateCaptchaDTO>('/captcha')
  },

  // ===== 微信扫码（框架占位）=====

  getWechatQRCode() {
    return unwrappedRequest.post<WechatQRCodeInfoDTO>('/auth/wechat/qrcode')
  },

  // ===== 个人信息：绑定/修改手机号、邮箱、密码 =====

  sendProfileCode(data: { channel: 'email' | 'phone'; target: string }) {
    return unwrappedRequest.post<null>('/auth/profile/send-code', data)
  },

  /** 绑定/改邮箱：后端只回 message（data 为 null），资料以 /auth/me 为准 */
  updateProfileEmail(data: { email: string; code: string }) {
    return unwrappedRequest.post<null>('/auth/profile/email', data)
  },

  /** 绑定/改手机号：后端只回 message（data 为 null），资料以 /auth/me 为准 */
  updateProfilePhone(data: { phone: string; code: string }) {
    return unwrappedRequest.post<null>('/auth/profile/phone', data)
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

  /** 改登录账号：后端重新签发双令牌（LoginResult），前端据此换会话 */
  updateAccount(data: { account: string; code: string }) {
    return unwrappedRequest.put<LoginResult>('/auth/account', data)
  }
}
