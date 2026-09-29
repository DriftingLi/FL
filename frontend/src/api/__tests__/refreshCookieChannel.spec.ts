// refresh 令牌的浏览器通道（ADR-0067 / 票 #1363）：Cookie 优先 + 请求体兼容的前端侧锁。
//
// 三条判据在前端的形状：
//   判据 1「Cookie 通道存在时忽略请求体」⇒ 前端**不再参与** refresh 的传递：刷新请求不带 refresh_token，
//     且必须带凭证（withCredentials），否则 Cookie 发不出去；
//     并带一条**族线索**头（storage 里那支 access，过期也发）——两族 refresh cookie 可在同一父域
//     并存，服务端按 access 决定续哪一族（#1376 跨端评审 · 移动端 ADR-0030 ②）。
//   判据 3「refresh 不再进 JS 可达存储」⇒ 结构锁：`api/client.ts` 里连 RefreshToken 这个标识符
//     都不得出现（读写都不许），且 `utils/storage.ts` 之外不得有任何 refresh 读写口；
//     行为锁：轮换出来的新 refresh 不落存储、登录响应里的那一份也不经 userInfo 落存储。
//
// 与 tokenSingleSource.spec.ts 的分工：那份锁 access 令牌的单一事实源（#1359 判据，本票不改），
// 本锁管 refresh 的存放面（ADR-0067 撤销了 ADR-0016 的「refresh 由前端持有」那条）。
import { describe, it, expect, vi, beforeEach } from 'vitest'
import { createPinia, setActivePinia } from 'pinia'
import { flushPromises } from '@vue/test-utils'
import axios, { AxiosError } from 'axios'
import { readFileSync, readdirSync, statSync } from 'node:fs'
import { join, resolve } from 'node:path'

const { getUserInfoMock } = vi.hoisted(() => ({ getUserInfoMock: vi.fn() }))
vi.mock('@/api/auth', () => ({
  authApi: { getUserInfo: getUserInfoMock, logout: vi.fn().mockResolvedValue(null) }
}))

import { useAuthStore } from '@/stores/auth'
import { REFRESH_TOKEN_KEY, TOKEN_KEY } from '@/utils/storage'

const SRC = resolve(__dirname, '../..')

/** 剥注释后的源码（同 tokenSingleSource.spec.ts 的口径：结构锁管代码形态，不管说明文字）。 */
function stripComments(src: string): string {
  return src
    .replace(/\/\*[\s\S]*?\*\//g, '')
    .replace(/<!--[\s\S]*?-->/g, '')
    .replace(/\/\/[^\n]*/g, '')
}

const CLIENT_SOURCE = stripComments(readFileSync(join(SRC, 'api/client.ts'), 'utf8'))

function count(source: string, re: RegExp): number {
  return (source.match(re) || []).length
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

describe('结构锁：client.ts 不碰 refresh 的存放面（ADR-0067）', () => {
  it('整个文件里不出现 refresh 令牌的读写口与线上字段', () => {
    // 三条一起锁：读口、写口、key 字面量、以及发给后端的字段名（本票之前的形态）。
    // 不用 /RefreshToken/ 这种宽判据 —— `tryRefreshTokens` 这个函数名本身就含它，会误伤。
    expect(count(CLIENT_SOURCE, /getRefreshToken\s*\(/g)).toBe(0)
    expect(count(CLIENT_SOURCE, /setRefreshToken\s*\(/g)).toBe(0)
    expect(count(CLIENT_SOURCE, /REFRESH_TOKEN_KEY/g)).toBe(0)
    expect(count(CLIENT_SOURCE, /\brefresh_token\b/g)).toBe(0)
  })

  it('连 localStorage 都不直接用（令牌读写只能经 storage.ts 的单点）', () => {
    expect(count(CLIENT_SOURCE, /localStorage/g)).toBe(0)
  })

  it('刷新链路带凭证：refresh cookie 是浏览器侧唯一通道', () => {
    // 必须钉在**刷新那个 client 的构造行**上：只判 `toContain('withCredentials: true')` 会被
    // 工厂里 `createHttpClient(opts)` 的透传行顶掉（去掉刷新行的 withCredentials 也不会红）。
    expect(CLIENT_SOURCE).toMatch(/const refreshHttp = axios\.create\(\{[^\n]*withCredentials: true[^\n]*\}\)/)
  })

  it('刷新请求体不再携带 refresh_token（不带 Cookie 之外的第二支凭证）', () => {
    expect(count(CLIENT_SOURCE, /post[^\n]*\/auth\/refresh[^\n]*\{\s*\}/g)).toBe(1)
  })

  it('刷新请求带族线索头（服务端靠它定族；不发就只能 401，绝不回退 Cookie 名序）', () => {
    // 钉在刷新那一次 post 的调用行上：只判「文件里出现过 familyClueHeaders」会被别的调用顶掉。
    expect(count(CLIENT_SOURCE, /post[^\n]*\/auth\/refresh[^\n]*headers:\s*familyClueHeaders\(\)/g)).toBe(1)
  })

  it('防恒绿：术前的形态必须被同一套扫描判红', () => {
    const before = `
      const rt = getRefreshToken()
      refreshHttp.post('/auth/refresh', { refresh_token: rt })
      setRefreshToken(data.refresh_token)
      localStorage.setItem(REFRESH_TOKEN_KEY, data.refresh_token)
    `
    const hits = [
      count(before, /getRefreshToken\s*\(/g) > 0,
      count(before, /setRefreshToken\s*\(/g) > 0,
      count(before, /REFRESH_TOKEN_KEY/g) > 0,
      count(before, /\brefresh_token\b/g) > 0,
      count(before, /localStorage/g) > 0
    ].filter(Boolean)
    expect(hits).toHaveLength(5)
    // 族线索头是本次新加的形态：术前形状必须在这条上判红，否则等于没锁。
    expect(/post[^\n]*\/auth\/refresh[^\n]*headers:\s*familyClueHeaders\(\)/.test(before)).toBe(false)
  })
})

describe('结构锁：refresh 读写口在整个 src/ 里只剩 storage.ts 的清除口', () => {
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

  const offenders: string[] = []
  const scanned: string[] = []
  for (const file of collectFiles(join(SRC, '.'))) {
    if (file.endsWith(join('utils', 'storage.ts'))) continue
    scanned.push(file)
    const lines = stripComments(readFileSync(file, 'utf8')).split('\n')
    lines.forEach((line, i) => {
      if (/getRefreshToken\(|setRefreshToken\(/.test(line)) {
        offenders.push(`${file.replace(SRC, 'src')}:${i + 1} ${line.trim()}`)
      }
    })
  }

  it('扫描面不是空转（目标文件真在射程内）', () => {
    expect(scanned.length).toBeGreaterThan(100)
    expect(scanned.some(f => f.endsWith(join('api', 'client.ts')))).toBe(true)
    expect(scanned.some(f => f.endsWith(join('stores', 'auth.ts')))).toBe(true)
  })

  it('src/ 里没有 refresh 令牌的读口或写口', () => {
    expect(offenders).toEqual([])
  })
})

describe('行为锁：401 → Cookie 静默刷新 → 重试（新 refresh 不落存储）', () => {
  const ok = (body: unknown) => ({
    data: body,
    status: 200,
    statusText: 'OK',
    headers: { 'content-type': 'application/json' },
    config: {},
    request: {}
  })

  /** 装一个 axios adapter 捕获刷新请求并把链路跑完（client.ts 在模块初始化时 create，故必须先装再 import） */
  async function runRefreshChain(capture: {
    bodies: Array<Record<string, unknown>>
    credentials: boolean[]
    authHeaders: Array<string | undefined>
  }) {
    const previousAdapter = axios.defaults.adapter
    vi.resetModules()
    axios.defaults.adapter = (async (config: {
      url?: string
      data?: string
      withCredentials?: boolean
      headers?: Record<string, unknown>
    }) => {
      if ((config.url || '').includes('/auth/refresh')) {
        capture.bodies.push(config.data ? (JSON.parse(config.data) as Record<string, unknown>) : {})
        capture.credentials.push(config.withCredentials === true)
        capture.authHeaders.push(
          (config.headers?.Authorization ?? config.headers?.authorization) as string | undefined
        )
        return ok({
          code: 200,
          message: 'ok',
          // 响应体照旧含新 refresh（请求体通道客户端需要），前端只是不再留它
          data: { token: 'access-rotated', refresh_token: 'refresh-new' }
        })
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

    try {
      const { createHttpClient } = await import('@/api/client')
      const { useAuthStore: useFreshAuthStore } = await import('@/stores/auth')
      const store = useFreshAuthStore()
      store.initialize()
      await flushPromises()
      const client = createHttpClient({ baseURL: '/api', onUnauthorized: vi.fn() })
      const data = await client.get<{ reached: boolean }>('/courses')
      return { store, data }
    } finally {
      axios.defaults.adapter = previousAdapter
    }
  }

  it('刷新请求不带 refresh_token、带凭证；access 更新而 refresh 不入库', async () => {
    const expired = jwtWithExp(-60)
    localStorage.setItem(TOKEN_KEY, expired)
    // 登录态成立的另一半（initFromStorage 要 userInfo.role，否则清态 ⇒ 刷新链路根本不发）
    localStorage.setItem('userInfo', JSON.stringify({ role: 'hrwai_user', account: 'u1' }))

    const capture = {
      bodies: [] as Array<Record<string, unknown>>,
      credentials: [] as boolean[],
      authHeaders: [] as Array<string | undefined>
    }
    const { store, data } = await runRefreshChain(capture)

    expect(data).toEqual({ reached: true })
    expect(capture.bodies).toHaveLength(1)
    // 判据 1：浏览器侧只提供 Cookie，不提供第二支凭证
    expect(capture.bodies[0]).toEqual({})
    expect(capture.bodies[0]).not.toHaveProperty('refresh_token')
    // 不带凭证就发不出去（refresh cookie 的 Path 收在认证族前缀 /api/auth，是浏览器侧唯一通道）
    expect(capture.credentials).toEqual([true])
    // 族线索（#1376 跨端评审）：storage 里那支**已过期**的 access 仍要原样发出去。
    // 它不参与认证，只回答「本次续期属于哪一族」；不发，服务端在两族并存时只能拒绝
    // （按 Cookie 名序猜族 = 替另一族续期，正是本次修的缺陷）。
    expect(capture.authHeaders).toEqual([`Bearer ${expired}`])
    // 判据 3：轮换出来的新 refresh 不进 JS 可达存储；access 仍按 ADR-0067 决策 3 留原处
    expect(localStorage.getItem(REFRESH_TOKEN_KEY)).toBeNull()
    expect(localStorage.getItem(TOKEN_KEY)).toBe('access-rotated')
    expect(store.token).toBe('access-rotated')
  })

  it('存量残留：#1363 之前登录写进 localStorage 的 refresh 在启动时被清掉', async () => {
    localStorage.setItem(TOKEN_KEY, jwtWithExp(600))
    localStorage.setItem(REFRESH_TOKEN_KEY, 'legacy-refresh')
    localStorage.setItem('userInfo', JSON.stringify({ role: 'hrwai_user' }))

    const store = useAuthStore()
    store.initialize()
    await flushPromises()

    expect(localStorage.getItem(REFRESH_TOKEN_KEY)).toBeNull()
    expect(store.isLoggedIn).toBe(true)
  })

  it('登录响应里的 refresh_token 不落存储（含 userInfo 这条侧漏路径）', async () => {
    const store = useAuthStore()
    store.setAuthData({
      token: 'access-1',
      refresh_token: 'refresh-1',
      role: 'hrwai_user',
      account: 'u1'
    })
    await flushPromises()

    expect(localStorage.getItem(TOKEN_KEY)).toBe('access-1')
    expect(localStorage.getItem(REFRESH_TOKEN_KEY)).toBeNull()
    expect(localStorage.getItem('userInfo')).not.toContain('refresh-1')
    expect(localStorage.getItem('userInfo')).toContain('"account":"u1"')
  })
})
