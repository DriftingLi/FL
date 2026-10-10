import { defineStore } from 'pinia'
import { ref, customRef } from 'vue'
import type { Ref } from 'vue'
import { authApi } from '@/api/auth'
import { adminApi } from '@/api/admin'
import { getValidAccessToken } from '@/api/client'
import { isDynamicRole } from '@/utils/authzRuntime'
import type { UserProfile } from '@/types/user'
import { getToken, getUserInfo, setToken, removeToken, removeRefreshToken, setUserInfo, clearLocalAuth } from '@/utils/storage'
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
  /**
   * 运行时能力集（#1618 段1）：**只有动态角色（管理端）有内容** —— 登录/恢复登录后经
   * GET /admin/me/capabilities 拉取；静态角色（学员/讲师/招聘者）恒为空数组，它们的可达面
   * 仍由生成的能力表回答。两条件并存是刻意的，「谁能做什么」的判定入口只有
   * `utils/authzRuntime.ts` 的 holdsCapability（守卫与侧栏都走它，不各写一遍）。
   */
  const capabilities: Ref<string[]> = ref([])

  // 初始化 Promise 缓存：main.ts 显式启动一次，路由守卫 await 同一 Promise 等待完成
  let readyPromise: Promise<void> | null = null

  function initFromStorage() {
    // ADR-0067（票 #1363）：#1363 之前的登录把 refresh 写进了 localStorage，那是**存量残留**。
    // 浏览器侧的续期通道已经换成 httpOnly Cookie（JS 读不到），留着那一份只是把 7 天凭证
    // 继续摊在「任何同源脚本都能扫一遍存储」的窗口里——本票要关的就是这一层。
    // 清它不伤可用性：续期不再需要前端手上那一份（服务端按 Cookie 轮换）。
    removeRefreshToken()
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
        // 恢复登录同样要拉能力集：强刷后守卫与侧栏都靠它（否则管理端首屏会被判无权限）
        await loadCapabilities()
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
    // ADR-0067（票 #1363）：refresh 不进任何 JS 可达存储。浏览器侧续期只认服务端下发的
    // httpOnly Cookie；响应体里那一份是给移动端/非浏览器客户端的，Web 端不是它的消费者。
    // 必须在**入库前**剥掉：`setUserInfo` 会把整个对象序列化进 localStorage，
    // 只删掉 setRefreshToken 那一行等于没删——refresh 会顺着 userInfo 漏回存储。
    const profile = { ...data }
    delete profile.refresh_token
    userInfo.value = profile
    isLoggedIn.value = true

    setUserInfo(profile)

    // 登录响应只含基础字段（无昵称/头像等），异步拉取 /auth/me 补齐完整资料
    refreshUserInfo()
    // 管理端能力集随登录态刷新（非管理角色会直接清空，不发请求）
    void loadCapabilities()
  }

  /**
   * 拉取当前管理员的运行时能力集（#1618 段1）。
   *
   * fail closed：拉取失败一律落空集，**不保留上一份** —— 上一份可能已被超管改小，
   * 留着它等于让「刚被收回权限的人」继续看到菜单（点进去才 403）。
   * 非动态角色不发请求（学员/讲师/招聘者的可达面由静态表回答）。
   */
  async function loadCapabilities(): Promise<void> {
    if (!isDynamicRole(userInfo.value?.role)) {
      capabilities.value = []
      return
    }
    try {
      const res = await adminApi.fetchMyCapabilities()
      capabilities.value = res?.capabilities ?? []
    } catch (e) {
      capabilities.value = []
      console.warn('[Auth] loadCapabilities failed:', e)
    }
  }

  function clearAuthData() {
    // 令牌侧经派生 ref 的 setter 清；clearLocalAuth 是「整份登录态」的清除单点（key 归 storage 层），
    // 不是令牌的第二个写口
    token.value = ''
    userInfo.value = {}
    isLoggedIn.value = false
    capabilities.value = []

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
    capabilities,
    loadCapabilities,
    initialize,
    setAuthData,
    clearAuthData,
    signOut,
    refreshUserInfo,
    freshAccessToken
  }
})
