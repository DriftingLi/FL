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

    if (!isLoggedIn.value) {
      isInitializing.value = false
      return
    }

    try {
      // 静默校验：token 过期时由拦截器直接 reject，不弹错误提示、不跳转登录页
      const res = await authApi.getUserInfo({ headers: { 'X-Silent': '1' } })
      if (res.code === 200 && res.data) {
        // 全量合并 /auth/me 返回的资料（昵称/头像/邮箱等），
        // 避免登录响应只有基础字段导致重新登录后昵称头像回退
        userInfo.value = {
          ...userInfo.value,
          ...res.data
        }
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
      const res = await authApi.getUserInfo({ headers: { 'X-Silent': '1' } })
      if (res.code === 200 && res.data) {
        userInfo.value = {
          ...userInfo.value,
          ...res.data
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
