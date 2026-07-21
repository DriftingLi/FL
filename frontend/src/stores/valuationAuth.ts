// 估值模块独立认证 store（与主体系 useAuthStore 完全隔离）
// 使用独立 localStorage key: valuation_token / valuation_userInfo
import { defineStore } from 'pinia'
import { ref } from 'vue'
import type { Ref } from 'vue'
import { valuationAuthApi, type ValuationLoginRes, type ValuationUserInfo } from '@/api/valuation/auth'

const TOKEN_KEY = 'valuation_token'
const INFO_KEY = 'valuation_userInfo'

export const useValuationAuthStore = defineStore('valuationAuth', () => {
  const token: Ref<string> = ref('')
  const userInfo: Ref<ValuationUserInfo> = ref({})
  const isLoggedIn: Ref<boolean> = ref(false)
  const isInitializing: Ref<boolean> = ref(true)

  function initFromStorage() {
    const savedToken = localStorage.getItem(TOKEN_KEY)
    const savedInfo = localStorage.getItem(INFO_KEY)
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
        console.warn('[ValuationAuth] Failed to parse saved user info')
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
      const res = await valuationAuthApi.me()
      if (res.data && res.data.user_id) {
        const updates: Partial<ValuationUserInfo> = {
          user_id: res.data.user_id,
          username: res.data.username,
          role: res.data.role
        }
        if (res.data.name) updates.name = res.data.name
        if (res.data.phone) updates.phone = res.data.phone
        if (res.data.email) updates.email = res.data.email
        if (res.data.company) updates.company = res.data.company
        userInfo.value = { ...userInfo.value, ...updates }
        localStorage.setItem(INFO_KEY, JSON.stringify(userInfo.value))
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

  function setAuthData(data: ValuationLoginRes) {
    if (!data || !data.token) {
      console.warn('[ValuationAuth] setAuthData called with invalid data')
      return
    }
    token.value = data.token
    userInfo.value = {
      token: data.token,
      user_id: data.user_id,
      username: data.username,
      name: data.name,
      role: data.role
    }
    isLoggedIn.value = true
    localStorage.setItem(TOKEN_KEY, data.token)
    localStorage.setItem(INFO_KEY, JSON.stringify(userInfo.value))
  }

  function clearAuthData() {
    token.value = ''
    userInfo.value = {}
    isLoggedIn.value = false
    localStorage.removeItem(TOKEN_KEY)
    localStorage.removeItem(INFO_KEY)
  }

  return {
    token,
    userInfo,
    isLoggedIn,
    isInitializing,
    setAuthData,
    clearAuthData
  }
})
