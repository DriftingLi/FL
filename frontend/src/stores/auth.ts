import { defineStore } from 'pinia'
import { ref, customRef } from 'vue'
import type { Ref } from 'vue'
import { authApi } from '@/api/auth'
import { getValidAccessToken } from '@/api/client'
import type { UserProfile } from '@/types/user'
import { getToken, getUserInfo, setToken, removeToken, setRefreshToken, setUserInfo, clearLocalAuth } from '@/utils/storage'
import { consumeAuthTokenFromUrl } from '@/utils/authToken'

export const useAuthStore = defineStore('auth', () => {
  /**
   * 令牌的唯一事实源 = storage（`utils/storage.ts` 的 TOKEN_KEY）——票 #1359 / spec #1345 真实缺陷 #11。
   *
   * 术前形态 `const token = ref('')` + 登录时 `token.value = data.token` 有两处漂移：
   *  ① 内存副本与 storage 是**双写**，而 `api/client.ts` 的 401 静默刷新只写 storage、不知道这份副本
   *     ⇒ 长驻页面（章节编辑 / 精选正文编辑的插图上传）拿着过期副本发请求，直接 401；
   *  ② 重新加载时 `initFromStorage` 取的是 `userInfo` 里登录那一刻的令牌快照，
   *     刷新过的新令牌被旧快照盖掉 ⇒ 漂移被固化。
   *
   * 术后：读 `getToken()`（customRef 每次访问都现取，不是 computed ⇒ 没有缓存窗口），
   * 写只写同一个 key。本文件里唯一碰令牌 storage 的地方就是这个 customRef，
   * 其余函数一律经 `token.value` 走它（结构锁 + 行为锁见 `__tests__/tokenSingleSource.spec.ts`）。
   */
  const token = customRef<string>((track, trigger) => ({
    get() {
      track()
      return getToken() ?? ''
    },
    set(value) {
      if (value) setToken(value)
      else removeToken()
      trigger()
    }
  }))
  const userInfo: Ref<UserProfile> = ref({})
  const isLoggedIn: Ref<boolean> = ref(false)

  // 初始化 Promise 缓存：main.ts 显式启动一次，路由守卫 await 同一 Promise 等待完成
  let readyPromise: Promise<void> | null = null

  function initFromStorage() {
    // 令牌不从这里赋值出去：经派生 ref 现取（就是 storage 的 TOKEN_KEY），
    // userInfo 只用来恢复资料与角色；其登录快照里的 token 字段不再是事实源。
    const savedToken = token.value
    const savedInfo = getUserInfo<UserProfile>()

    if (savedToken && savedInfo && savedInfo.role) {
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
      // 跨子域名跳转携带的 token：优先于本地登录态，供 Cookie 不可用环境恢复登录
      const carriedToken = consumeAuthTokenFromUrl()
      if (carriedToken) {
        // 写进唯一事实源（派生 ref 的 setter 落 storage），读取侧随即跟上
        token.value = carriedToken
        isLoggedIn.value = true
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

    // 令牌写入的唯一入口：派生 ref 的 setter（只落 storage，不再另存内存副本）
    token.value = data.token
    userInfo.value = data
    isLoggedIn.value = true

    if (data.refresh_token) {
      setRefreshToken(data.refresh_token)
    }
    setUserInfo(data)

    // 登录响应只含基础字段（无昵称/头像等），异步拉取 /auth/me 补齐完整资料
    refreshUserInfo()
  }

  function clearAuthData() {
    // 令牌侧经派生 ref 的 setter 清；clearLocalAuth 是「整份登录态」的清除单点（key 归 storage 层），
    // 不是令牌的第二个写口
    token.value = ''
    userInfo.value = {}
    isLoggedIn.value = false

    clearLocalAuth()
  }

  /**
   * 登出单点（第十二波票 1，#1168）：revoke → 清本地 → 返回结果。
   * 本地清除是无条件承诺：后端失败也清（网络不畅也退得出）。
   * confirm、跳转落点与本页专属清理留在调用方 adapter，本接口不携带交互参数。
   */
  async function signOut(): Promise<{ revoked: boolean }> {
    let revoked = true
    try {
      await authApi.logout()
    } catch {
      revoked = false
    }
    clearAuthData()
    return { revoked }
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

  /**
   * 按请求现取新鲜 access token（票 #1359 判据 2）：给**绕过 client 拦截器**的裸请求
   * （Vditor 插图上传这类原生 XHR）在发起前换一次 —— 本地过期则静默刷新，新令牌照
   * ADR-0016 / ADR-0067 的现有形态落进 storage（请求契约不改）。
   * 实现单点仍在 `api/client.ts` 的 `getValidAccessToken()`，这里只是认证域的转发面：
   * 页面与业务组件不得直接引用请求层（`scripts/check-api-seam.mjs`）。
   */
  function freshAccessToken(): Promise<string | null> {
    return getValidAccessToken()
  }

  return {
    token,
    userInfo,
    isLoggedIn,
    initialize,
    setAuthData,
    clearAuthData,
    signOut,
    refreshUserInfo,
    freshAccessToken
  }
})
