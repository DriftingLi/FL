import { defineStore } from 'pinia'
import { ref } from 'vue'
import { authApi } from '@/api/auth'

export const useAuthStore = defineStore('auth', () => {
  const token = ref('')
  const userInfo = ref({})
  const isLoggedIn = ref(false)
  const isInitializing = ref(true)

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
      const res = await authApi.getUserInfo()
      if (res.code === 200 && res.data) {
        const updates = {
          user_id: res.data.user_id,
          username: res.data.username,
          role: res.data.role
        }
        if (res.data.name) {
          updates.name = res.data.name
        }
        if (res.data.level) {
          updates.level = res.data.level
        }
        userInfo.value = {
          ...userInfo.value,
          ...updates
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

  function setAuthData(data) {
    if (!data || !data.token) {
      console.warn('[Auth] setAuthData called with invalid data')
      return
    }

    token.value = data.token
    userInfo.value = data
    isLoggedIn.value = true

    localStorage.setItem('token', data.token)
    localStorage.setItem('userInfo', JSON.stringify(data))
  }

  function clearAuthData() {
    token.value = ''
    userInfo.value = {}
    isLoggedIn.value = false

    localStorage.removeItem('token')
    localStorage.removeItem('userInfo')
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
