// 令牌单一事实源（票 #1359 / spec #1345 真实缺陷 #11）—— 行为锁 + 结构锁。
//
// 术前形态：`stores/auth.ts` 里 `const token = ref('')`，登录时 `token.value = data.token` 与
// `setToken(data.token)` 双写；重新加载时 `initFromStorage` 取的还是 `userInfo` 里登录那一刻的
// 令牌快照。而 `api/client.ts` 的 401 静默刷新**只写 storage**、不知道那份副本 ⇒
//   ① 长驻页面（MarkdownEditor 的插图上传）读缓存副本发请求 ⇒ 401；
//   ② 刷新过令牌后 reload，storage 的新值被 userInfo 里的旧快照盖掉（漂移被固化）。
// 术后形态：token 是 customRef 派生值 —— 读每次现取 storage，写只写同一个 key（见 stores/auth.ts 注释）。
//
// 三条判据各自的锁：
//   判据 1「刷新之后 authStore.token 与 localStorage 一致」→ 外改 storage 立刻同值 + 真 401 静默刷新链路；
//   判据 2「长驻页面插图不 401」→ 消费面结构锁（页面/组件不得把 store 的令牌内联进请求头）+
//          MarkdownEditor 的按请求现取用例（components/tutor/__tests__/markdownEditorUploadToken.spec.ts）；
//   判据 3「结构上不存在第二份 token 真相」→ 源码扫描：碰令牌 storage 的三个 API 只许出现在派生 ref 里。
import { describe, it, expect, vi, beforeEach } from 'vitest'
import { createPinia, setActivePinia } from 'pinia'
import { flushPromises } from '@vue/test-utils'
import axios, { AxiosError } from 'axios'
import { watch } from 'vue'
import { readFileSync, readdirSync, statSync } from 'node:fs'
import { join, resolve } from 'node:path'

const { getUserInfoMock } = vi.hoisted(() => ({ getUserInfoMock: vi.fn() }))
vi.mock('@/api/auth', () => ({
  authApi: { getUserInfo: getUserInfoMock, logout: vi.fn().mockResolvedValue(null) }
}))

import { useAuthStore } from '../auth'
import { REFRESH_TOKEN_KEY, TOKEN_KEY, USER_INFO_KEY, getToken } from '@/utils/storage'

/** frontend/src（本文件在 src/stores/__tests__ 下）。 */
const SRC = resolve(__dirname, '../..')

/**
 * 剥掉注释后的源码：结构锁管的是**代码形态**，而本票的注释必须能写下「术前是什么样」
 * （`const token = ref('')` 这类反面写法正是被锁的东西）—— 不剥注释就是自己判红。
 */
function stripComments(src: string): string {
  return src
    .replace(/\/\*[\s\S]*?\*\//g, '')
    .replace(/<!--[\s\S]*?-->/g, '')
    .replace(/\/\/[^\n]*/g, '')
}

beforeEach(() => {
  setActivePinia(createPinia())
  localStorage.clear()
  getUserInfoMock.mockReset()
  getUserInfoMock.mockResolvedValue({ account: 'u1', role: 'hrwai_user', user_id: 1 })
})

/** 造一枚 exp 在若干秒后的 JWT（client.ts 只解载荷的 exp，不验签） */
function jwtWithExp(deltaSeconds: number): string {
  const payload = btoa(JSON.stringify({ exp: Math.floor(Date.now() / 1000) + deltaSeconds }))
  return `h.${payload}.s`
}

describe('判据 1：store.token 是 storage 的派生值（无缓存窗口）', () => {
  it('storage 被外部改写（静默刷新就是这么写的）后，读 store.token 立刻同值', () => {
    const store = useAuthStore()
    store.setAuthData({ token: 'access-old', role: 'hrwai_user', user_id: 1 })
    expect(store.token).toBe('access-old')

    // api/client.ts 的 tryRefreshTokens 就是这个写法：只写 storage，不通知 store
    localStorage.setItem(TOKEN_KEY, 'access-rotated')

    expect(store.token).toBe('access-rotated')
    expect(store.token).toBe(getToken())
  })

  it('两次读取之间 storage 变了 ⇒ 第二次读到新值（不是 computed 那种缓存）', () => {
    const store = useAuthStore()
    localStorage.setItem(TOKEN_KEY, 'first')
    expect(store.token).toBe('first')
    localStorage.setItem(TOKEN_KEY, 'second')
    expect(store.token).toBe('second')
  })

  it('刷新后重新加载：storage 的 access 优先于 userInfo 里的登录快照', async () => {
    // 静默刷新留下的现场：TOKEN_KEY 是新的，userInfo 里还是登录那一刻的快照
    localStorage.setItem(TOKEN_KEY, 'access-fresh')
    localStorage.setItem(USER_INFO_KEY, JSON.stringify({ token: 'login-time-snapshot', role: 'hrwai_user' }))

    const store = useAuthStore()
    store.initialize()
    await flushPromises()

    expect(store.token).toBe('access-fresh')
    expect(store.token).toBe(localStorage.getItem(TOKEN_KEY))
    expect(store.isLoggedIn).toBe(true)
  })

  it('真 401 链路：静默刷新只写 storage，store.token 与新 access 同值', async () => {
    const expired = jwtWithExp(-60)
    localStorage.setItem(TOKEN_KEY, expired)
    localStorage.setItem(REFRESH_TOKEN_KEY, 'refresh-old')
    localStorage.setItem(USER_INFO_KEY, JSON.stringify({ token: expired, role: 'hrwai_user' }))

    const ok = (body: unknown) => ({
      data: body,
      status: 200,
      statusText: 'OK',
      headers: { 'content-type': 'application/json' },
      config: {},
      request: {}
    })
    const refreshBodies: Array<Record<string, unknown>> = []
    const adapter = (async (config: { url?: string; data?: string; headers?: Record<string, unknown> }) => {
      if ((config.url || '').includes('/auth/refresh')) {
        refreshBodies.push(config.data ? (JSON.parse(config.data) as Record<string, unknown>) : {})
        // 响应形状照生成面 RefreshResultDTO（ADR-0016 的请求体通道照旧，本票不改契约）
        return ok({ code: 200, message: 'ok', data: { token: 'access-rotated', refresh_token: 'refresh-new' } })
      }
      if (config.headers?.Authorization === 'Bearer access-rotated') {
        return ok({ code: 200, message: 'ok', data: { reached: true } })
      }
      throw new AxiosError(
        'Request failed with status code 401',
        'ERR_BAD_REQUEST',
        config as never,
        null,
        { status: 401, data: { code: 401, message: 'unauthorized' }, headers: {}, config, request: {} } as never
      )
    }) as never

    // client.ts 的 refreshHttp 与 api 层实例都在**模块初始化时** axios.create（adapter 是快照），
    // 故必须先装 adapter、再首次加载这条链路，否则刷新请求走真 XHR。
    const previousAdapter = axios.defaults.adapter
    vi.resetModules()
    axios.defaults.adapter = adapter
    const { createHttpClient } = await import('@/api/client')
    const { useAuthStore: useFreshAuthStore } = await import('../auth')

    try {
      const store = useFreshAuthStore()
      store.initialize()
      await flushPromises()
      expect(store.token).toBe(expired)

      const client = createHttpClient({ baseURL: '/api', onUnauthorized: vi.fn() })
      const data = await client.get<{ reached: boolean }>('/courses')

      // 重试的那一发带的是刷新后的新令牌 ⇒ 链路真的走通了
      expect(data).toEqual({ reached: true })
      expect(refreshBodies).toEqual([{ refresh_token: 'refresh-old' }])
      // 判据 1：刷新之后 store 侧与 storage 侧同值（术前那份缓存副本到这里还是 expired）
      expect(store.token).toBe('access-rotated')
      expect(localStorage.getItem(TOKEN_KEY)).toBe('access-rotated')
      expect(localStorage.getItem(REFRESH_TOKEN_KEY)).toBe('refresh-new')
    } finally {
      axios.defaults.adapter = previousAdapter
    }
  })
})

describe('判据 3：结构上不存在第二份 token 真相', () => {
  const AUTH_STORE_SOURCE = stripComments(readFileSync(join(SRC, 'stores/auth.ts'), 'utf8'))

  /** 派生 ref 的那一段（从 customRef 声明到它的收尾 `}))`） */
  const tokenBlock = AUTH_STORE_SOURCE.match(/const token = customRef[\s\S]*?\n\s*\}\)/)?.[0] ?? ''
  const count = (re: RegExp) => (AUTH_STORE_SOURCE.match(re) || []).length

  it('token 由 customRef 派生，读 storage、写也只落 storage', () => {
    expect(tokenBlock).not.toBe('')
    expect(tokenBlock).toContain('getToken()')
    expect(tokenBlock).toContain('setToken(value)')
    expect(tokenBlock).toContain('removeToken()')
  })

  it('不存在缓存副本的声明形态（`const token = ref(...)`）', () => {
    expect(count(/const token[^=\n]*=\s*ref\(/g)).toBe(0)
  })

  it('碰令牌 storage 的三个 API 各只出现一次，且都在派生 ref 内（无「缓存 + storage 双写」）', () => {
    expect(count(/getToken\(/g)).toBe(1)
    expect(count(/setToken\(/g)).toBe(1)
    expect(count(/removeToken\(/g)).toBe(1)
  })

  it('不从 userInfo 的登录快照里取令牌值', () => {
    expect(count(/savedInfo\.token/g)).toBe(0)
    expect(count(/userInfo(\.value)?\.token/g)).toBe(0)
  })

  it('经 store 写令牌一定落进 storage（不存在只改内存的通道）', () => {
    const store = useAuthStore()
    store.token = 'written-through-store'
    expect(localStorage.getItem(TOKEN_KEY)).toBe('written-through-store')
    expect(store.token).toBe('written-through-store')

    store.token = ''
    expect(localStorage.getItem(TOKEN_KEY)).toBeNull()
    expect(store.token).toBe('')
  })

  it('store 侧写入会触发依赖（清登录态时 UI 判据仍会重算）', () => {
    const store = useAuthStore()
    localStorage.setItem(TOKEN_KEY, 'a')
    const seen: string[] = []
    const stop = watch(() => store.token, value => seen.push(value), { flush: 'sync' })

    store.token = 'b'
    store.clearAuthData()
    stop()

    // 外改 storage（静默刷新）不触发依赖：值本身永远现取，触发只用于「登录态跃迁」
    expect(seen).toEqual(['b', ''])
  })

  it('防恒绿：术前的形态必须被同一套扫描判红', () => {
    const broken = `
export const useAuthStore = defineStore('auth', () => {
  const token: Ref<string> = ref('')
  function setAuthData(data: UserProfile) {
    token.value = data.token
    setToken(data.token)
  }
  function initFromStorage() {
    if (savedToken && savedInfo.token) token.value = savedInfo.token
  }
})
`
    const hits = [
      /const token[^=\n]*=\s*ref\(/.test(broken),
      (broken.match(/setToken\(/g) || []).length > 0,
      /savedInfo\.token/.test(broken)
    ].filter(Boolean)
    // 三条判据在术前形态里全部命中（扫描面不是恒绿的装饰）
    expect(hits).toHaveLength(3)
  })
})

describe('判据 2 的结构锁：页面/组件不得把 store 的令牌内联进请求头', () => {
  /**
   * 长驻页面 401 的形态（MarkdownEditor 术前写法）：在组件里把 `xxxStore.token` 直接拼进
   * Authorization。合法写法是从请求层的按请求现取（getValidAccessToken 一类单点）拿到的局部值。
   */
  const HEADER_TOKEN_RE = /(Authorization|Bearer)[^\n]*\b\w*[sS]tore\.token\b/

  function collectFiles(dir: string): string[] {
    const out: string[] = []
    for (const entry of readdirSync(dir)) {
      if (entry === '__tests__' || entry === 'node_modules') continue
      const full = join(dir, entry)
      if (statSync(full).isDirectory()) out.push(...collectFiles(full))
      else if (entry.endsWith('.vue') || entry.endsWith('.ts')) out.push(full)
    }
    return out
  }

  const violations: string[] = []
  const scanned: string[] = []
  for (const dir of ['pages', 'components']) {
    for (const file of collectFiles(join(SRC, dir))) {
      scanned.push(file)
      const lines = stripComments(readFileSync(file, 'utf8')).split('\n')
      lines.forEach((line, i) => {
        if (HEADER_TOKEN_RE.test(line)) {
          violations.push(`${file.replace(SRC, 'src')}:${i + 1} ${line.trim()}`)
        }
      })
    }
  }

  it('扫描面不是空转：本票的目标文件真在射程内', () => {
    // 走查静默失灵（目录改名 / 后缀判据写错）会让「无违规」恒真 —— 与 checks.md 里
    // 「扫描面为空即 Fatal」的防空转半边同口径
    expect(scanned.length).toBeGreaterThan(100)
    expect(scanned.some(f => f.endsWith(join('tutor', 'MarkdownEditor.vue')))).toBe(true)
  })

  it('pages/ 与 components/ 里没有「请求头内联 store 令牌」的写法', () => {
    expect(violations).toEqual([])
  })

  it('防恒绿：术前的 MarkdownEditor 写法必须被同一判据判红', () => {
    const broken = '    headers.Authorization = `Bearer ${authStore.token}`'
    expect(HEADER_TOKEN_RE.test(broken)).toBe(true)
    const legit = '      headers: { Authorization: `Bearer ${token}` }'
    expect(HEADER_TOKEN_RE.test(legit)).toBe(false)
  })
})

describe('消费面：按请求现取（freshAccessToken 单点）', () => {
  it('本地 access 未过期 ⇒ 直接返回 storage 的值，不发刷新请求', async () => {
    const fresh = jwtWithExp(600)
    localStorage.setItem(TOKEN_KEY, fresh)
    const store = useAuthStore()

    await expect(store.freshAccessToken()).resolves.toBe(fresh)
  })

  it('storage 无令牌 ⇒ 返回 null（未登录不硬拼 Authorization）', async () => {
    const store = useAuthStore()
    await expect(store.freshAccessToken()).resolves.toBeNull()
  })
})
