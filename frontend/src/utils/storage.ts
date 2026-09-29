// 本地存储单点封装：token / userInfo key 只在这里定义。
// 三个前端模块（主体系 / 估值 / AI 助手）的 token 读写统一走这里，避免 key 字面量散落。

/** 统一 HRWAI 登录 access token key */
export const TOKEN_KEY = 'token'
/**
 * refresh token 的 key —— **只读不写**：ADR-0067（票 #1363）之后浏览器侧的 refresh 由服务端
 * 下发的 httpOnly Cookie 承载，JS 拿不到也存不了。本文件因此**不提供** get/setRefreshToken：
 * 留着写口就等于给「下一次有人顺手把令牌塞回 localStorage」留门。
 * 这里保留 key 与 remove 的唯一理由，是清掉 #1363 之前登录留下的存量残留（clearLocalAuth /
 * stores/auth.ts 的 initFromStorage 各清一次）。
 */
export const REFRESH_TOKEN_KEY = 'refresh_token'
/** 用户信息缓存 key */
export const USER_INFO_KEY = 'userInfo'

export function getToken(): string | null {
  return localStorage.getItem(TOKEN_KEY)
}

export function setToken(token: string): void {
  localStorage.setItem(TOKEN_KEY, token)
}

export function removeToken(): void {
  localStorage.removeItem(TOKEN_KEY)
}

export function removeRefreshToken(): void {
  localStorage.removeItem(REFRESH_TOKEN_KEY)
}

export function getUserInfo<T = any>(): T | null {
  const raw = localStorage.getItem(USER_INFO_KEY)
  if (!raw) return null
  try {
    return JSON.parse(raw) as T
  } catch {
    return null
  }
}

export function setUserInfo<T = any>(info: T): void {
  localStorage.setItem(USER_INFO_KEY, JSON.stringify(info))
}

export function removeUserInfo(): void {
  localStorage.removeItem(USER_INFO_KEY)
}

/** 清除本地登录态（access + refresh + userInfo）——auth store、登出与 401 兜底共用 */
export function clearLocalAuth(): void {
  removeToken()
  removeRefreshToken()
  removeUserInfo()
}

export function getStorage<T = unknown>(key: string): T | null {
  try {
    const value = localStorage.getItem(key)
    return value ? (JSON.parse(value) as T) : null
  } catch {
    return localStorage.getItem(key) as unknown as T | null
  }
}

export function setStorage<T = unknown>(key: string, value: T): void {
  try {
    localStorage.setItem(key, JSON.stringify(value))
  } catch {
    localStorage.setItem(key, String(value))
  }
}

export function removeStorage(key: string): void {
  localStorage.removeItem(key)
}
