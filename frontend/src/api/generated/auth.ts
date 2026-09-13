// 生成文件，勿手改（ADR-0019 契约 codegen 专项 / ADR-0048 按域解冻；spec #940 片五③、#952 片一）。
// 域：认证与账号（/api/auth/*、/api/captcha：登录 / 双令牌 / 资料 / 验证码 / 微信）
// 唯一事实源：后端注解 → backend/docs/swagger.json（CI 有新鲜度锁：backend-lint 的 swagger 步骤）。
// 再生成：cd backend && go run ./cmd/gen-apitypes
// 同步契约：backend/internal/apitypes/codegen_test.go 把本文件与注解渲染结果全等比对。
//
// 覆盖端点：
//   POST /auth/login
//   POST /auth/admin-login
//   POST /auth/tutor-login
//   POST /auth/recruiter-login
//   POST /auth/logout
//   POST /auth/refresh
//   GET  /auth/me
//   PUT  /auth/profile
//   DELETE /auth/account
//   POST /auth/avatar
//   POST /auth/email/send-code
//   POST /auth/phone/send-code
//   POST /auth/email/register
//   POST /auth/phone/register
//   POST /auth/email/login
//   POST /auth/phone/login
//   POST /auth/email/reset-password
//   POST /auth/phone/reset-password
//   GET  /captcha
//   POST /auth/wechat/qrcode
//   POST /auth/wx-login
//   POST /auth/wechat/login
//   POST /auth/profile/send-code
//   POST /auth/profile/email
//   POST /auth/profile/phone
//   POST /auth/profile/password
//   POST /auth/profile/password/send-code
//   POST /auth/account/send-code
//   PUT  /auth/account
//
// 覆盖的 Go 类型：GenerateCaptchaDTO / LoginResult / ProfileChangeRequestDTO / ProfileDTO / RefreshResultDTO / WechatQRCodeInfoDTO / WxLoginResult
//
// 可空性 / 缺省态由**注解层**表达，生成器只如实转写（Go 结构体 tag）：
//   - extensions:"x-nullable" → 字段渲染 'T | null'：键一定在，值为 null（Go 指针且无 omitempty）；
//   - extensions:"x-optional" → 字段渲染 'T?'：键**可能整个不存在**（Go omitempty）；
//   - 两者可同时标注（'T?' 且 '| null'）；未标注的一律按「键一定在、非 null」渲染 ——
//     swag 看不到 Go 的 omitempty，漏标即契约撒谎。
// 其余已知限制：
//   - Go 侧 any 字段在 swagger 里是空 schema，渲染 'unknown'（不猜结构）；
//   - 不生成 query / body 的入参类型（只生成响应形状）。
// 需要更精确的形状时先在注解层补齐（先例见 spec #940 片五②的差集清单）。

export interface GenerateCaptchaDTO {
  id: string
  image: string
}

export interface LoginResult {
  account: string
  refresh_token: string
  role: string
  token: string
  user_id: number
  username: string
}

export interface ProfileChangeRequestDTO {
  avatar_url: string
  created_at: string
  field_type: string
  id: number
  new_value: string
  old_value: string
  reject_reason: string
  reviewed_at?: string
  reviewed_by?: number
  status: string
  user_id: number
  username: string
}

export interface ProfileDTO {
  account: string
  avatar_url?: string
  company?: string
  email?: string
  has_password?: boolean
  name?: string
  pending_profile_change?: ProfileChangeRequestDTO | null
  phone?: string
  role: string
  uid?: string
  user_id: number
  username?: string
}

export interface RefreshResultDTO {
  refresh_token: string
  token: string
}

export interface WechatQRCodeInfoDTO {
  enabled: boolean
  message: string
  qr_url: string
}

export interface WxLoginResult {
  account: string
  avatar: string
  isNew: boolean
  name: string
  refresh_token: string
  role: string
  token: string
  user_id: number
  username: string
}
