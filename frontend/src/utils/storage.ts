// 本地存储单点封装：token / userInfo key 只在这里定义。
// 三个前端模块（主体系 / 估值 / AI 助手）的 token 读写统一走这里，避免 key 字面量散落。

/** 统一 HRWAI 登录 token key */
export const TOKEN_KEY = 'token'
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

/** 清除本地登录态（token + userInfo）——auth store 与 401 兜底共用 */
export function clearLocalAuth(): void {
  removeToken()
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
