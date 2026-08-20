// 用户资料 typed module：账号体系（account/uid/username）唯一 shape 与派生字段的实现。
// 契约与后端 /auth/me（AuthService.GetProfile）对齐：username 为昵称，uid 为字符串雪花 ID；
// 讲师/管理员表仍有 name（显示名）字段，hrwai 用户无。

/** 待审核资料请求（后端 ProfileChangeRequestDTO 对应） */
export interface PendingProfileChange {
  id: number
  user_id: number
  username: string
  avatar_url: string
  field_type: string
  old_value: string
  new_value: string
  status: string
  reject_reason?: string
  reviewed_by?: number | null
  reviewed_at?: string | null
  created_at: string
}

/** 用户资料（登录响应与 /auth/me 共用同一 shape，取代双份手写 AuthUserInfo/UserInfo） */
export interface UserProfile {
  token?: string
  /** refresh token（双令牌会话 ADR-0012，仅登录响应下发，前端单独存储） */
  refresh_token?: string
  user_id?: number
  uid?: string
  account?: string
  username?: string
  /** 讲师/管理员的显示名（hrwai 用户无此字段） */
  name?: string
  avatar_url?: string
  role?: string
  email?: string
  phone?: string
  company?: string
  has_password?: boolean
  pending_profile_change?: PendingProfileChange | null
}

/**
 * 显示名派生（唯一实现）：
 * 讲师/管理员用 name（导师/系统管理员），hrwai 用户用昵称 username，退回登录账号 account。
 * 四个工作台（学员/导师/管理/估值）的回退链全部收敛到此处。
 */
export function displayNameOf(user?: UserProfile | null): string {
  if (!user) return ''
  if (user.role === 'tutor' || user.role === 'admin') {
    return user.name || user.username || user.account || ''
  }
  return user.username || user.account || ''
}
