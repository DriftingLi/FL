import { defineStore } from 'pinia'
import { ref } from 'vue'
import type { Ref } from 'vue'
import { authApi } from '@/api/auth'

export interface UserInfo {
  token?: string
  user_id?: number
  username?: string
  name?: string
  nickname?: string
  avatar_url?: string
  role?: string
  avatar?: string
  [key: string]: any
}

export const useAuthStore = defineStore('auth', () => {
  const token: Ref<string> = ref('')
  const userInfo: Ref<UserInfo> = ref({})
  const isLoggedIn: Ref<boolean> = ref(false)
  const isInitializing: Ref<boolean> = ref(true)

  // 读取跨子域名跳转附带的一次性 token，并立即从地址栏移除
  function consumeAuthTokenFromUrl(): string {
    if (typeof window === 'undefined') return ''
    const params = new URLSearchParams(window.location.search)
    const token = params.get('auth_token')
    if (!token) return ''
    params.delete('auth_token')
    const query = params.toString()
    const newUrl = window.location.pathname + (query ? `?${query}` : '') + window.location.hash
    window.history.replaceState(null, '', newUrl)
    return token
  }

  function initFromStorage() {
    const savedToken = localStorage.getItem('token')
    const savedInfo = localStorage.getItem('userInfo')

    if (savedToken && savedInfo) {
      try {
        const parsed = JSON.parse(savedInfo)
        if (parsed && parsed.token && parsed.role) {
          token.value = parsed.token
          userInfo.value = parsed
          isLoggedIn.value = true
          return
        }
      } catch (e) {
        console.warn('[Auth] Failed to parse saved user info')
      }
    }

    clearAuthData()
  }

  async function validateToken() {
    initFromStorage()

    try {
      // 跨子域名跳转携带的 token：优先于本地缓存，供 Cookie 不可用环境恢复登录态
      const carriedToken = consumeAuthTokenFromUrl()
      if (carriedToken) {
        token.value = carriedToken
        isLoggedIn.value = true
        localStorage.setItem('token', carriedToken)
      }
      // 登录态以 /auth/me 为准：父域名 Cookie 共享后，
      // 即使本地无 token（跨子域名首次访问），也能恢复登录；
      // token 过期时由拦截器直接 reject，不弹错误提示、不跳转登录页
      const info = await authApi.getUserInfo({ headers: { 'X-Silent': '1' } })
      if (info) {
        // 全量合并 /auth/me 返回的资料（昵称/头像/邮箱等），
        // 避免登录响应只有基础字段导致重新登录后昵称头像回退
        userInfo.value = {
          ...userInfo.value,
          ...info
        }
        isLoggedIn.value = true
        localStorage.setItem('userInfo', JSON.stringify(userInfo.value))
      } else {
        clearAuthData()
      }
    } catch (e) {
      clearAuthData()
    } finally {
      isInitializing.value = false
    }
  }

  validateToken()

  function setAuthData(data: UserInfo) {
    if (!data || !data.token) {
      console.warn('[Auth] setAuthData called with invalid data')
      return
    }

    token.value = data.token
    userInfo.value = data
    isLoggedIn.value = true

    localStorage.setItem('token', data.token)
    localStorage.setItem('userInfo', JSON.stringify(data))

    // 登录响应只含基础字段（无昵称/头像等），异步拉取 /auth/me 补齐完整资料
    refreshUserInfo()
  }

  function clearAuthData() {
    token.value = ''
    userInfo.value = {}
    isLoggedIn.value = false

    localStorage.removeItem('token')
    localStorage.removeItem('userInfo')
  }

  // 重新拉取 /auth/me 并合并到 userInfo（昵称/头像等资料更新后调用）
  async function refreshUserInfo() {
    try {
      const info = await authApi.getUserInfo({ headers: { 'X-Silent': '1' } })
      if (info) {
        userInfo.value = {
          ...userInfo.value,
          ...info
        }
        localStorage.setItem('userInfo', JSON.stringify(userInfo.value))
      }
    } catch (e) {
      console.warn('[Auth] refreshUserInfo failed:', e)
    }
  }

  return {
    token,
    userInfo,
    isLoggedIn,
    isInitializing,
    setAuthData,
    clearAuthData,
    refreshUserInfo
  }
})
