import { defineStore } from 'pinia'
import { ref } from 'vue'
import type { Ref } from 'vue'
import { authApi } from '@/api/auth'
import type { UserProfile } from '@/types/user'
import { getToken, getUserInfo, setToken, setRefreshToken, setUserInfo, clearLocalAuth } from '@/utils/storage'
import { consumeAuthTokenFromUrl } from '@/utils/authToken'

export const useAuthStore = defineStore('auth', () => {
  const token: Ref<string> = ref('')
  const userInfo: Ref<UserProfile> = ref({})
  const isLoggedIn: Ref<boolean> = ref(false)

  // 初始化 Promise 缓存：main.ts 显式启动一次，路由守卫 await 同一 Promise 等待完成
  let readyPromise: Promise<void> | null = null

  function initFromStorage() {
    const savedToken = getToken()
    const savedInfo = getUserInfo<UserProfile>()

    if (savedToken && savedInfo && savedInfo.token && savedInfo.role) {
      token.value = savedInfo.token
      userInfo.value = savedInfo
      isLoggedIn.value = true
      return
    }
    if (savedToken && !savedInfo) {
      console.warn('[Auth] Failed to parse saved user info')
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
        setToken(carriedToken)
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
        setUserInfo(userInfo.value)
      } else {
        clearAuthData()
      }
    } catch (e) {
      clearAuthData()
    }
  }

  /** 幂等初始化：由 main.ts 显式调用（localStorage 恢复 + /auth/me 校验 + URL auth_token 交接） */
  function initialize(): Promise<void> {
    if (!readyPromise) {
      readyPromise = validateToken()
    }
    return readyPromise
  }

  function setAuthData(data: UserProfile) {
    if (!data || !data.token) {
      console.warn('[Auth] setAuthData called with invalid data')
      return
    }

    token.value = data.token
    userInfo.value = data
    isLoggedIn.value = true

    setToken(data.token)
    if (data.refresh_token) {
      setRefreshToken(data.refresh_token)
    }
    setUserInfo(data)

    // 登录响应只含基础字段（无昵称/头像等），异步拉取 /auth/me 补齐完整资料
    refreshUserInfo()
  }

  function clearAuthData() {
    token.value = ''
    userInfo.value = {}
    isLoggedIn.value = false

    clearLocalAuth()
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
        setUserInfo(userInfo.value)
      }
    } catch (e) {
      console.warn('[Auth] refreshUserInfo failed:', e)
    }
  }

  return {
    token,
    userInfo,
    isLoggedIn,
    initialize,
    setAuthData,
    clearAuthData,
    refreshUserInfo
  }
})
