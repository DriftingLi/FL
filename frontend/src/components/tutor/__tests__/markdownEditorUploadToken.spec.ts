// MarkdownEditor 插图上传的令牌获取（票 #1359 / spec #1345 真实缺陷 #11）。
//
// 术前形态：`buildUploadConfig()` 在**创建编辑器时**读一次 `authStore.token`（store 的缓存副本）
// 塞进 `upload.headers`。本组件常驻章节编辑 / 精选正文编辑页，而 Vditor 走自己的 XHR、
// 不经 `api/client.ts` 的拦截器 ⇒ 令牌在别处被静默刷新后，这份快照就是过期令牌 ⇒ 插图 401。
// 术后形态：不在创建时抓快照 —— `upload.file`（Vditor 发 xhr 前 await）先换新鲜 access token
// （副作用是把它写进 storage），`upload.setHeaders`（每次上传前同步调用）现取派生值拼头。
//
// 这里用**真 auth store + 真 storage 单点**（只替身 Vditor、authApi 与请求层的现取单点），
// 锁的就是「每次上传现取」。
import { describe, it, expect, vi, beforeEach } from 'vitest'
import { mount, flushPromises } from '@vue/test-utils'
import { createPinia, setActivePinia } from 'pinia'
import { epLite } from '@/test/element-lite'
import MarkdownEditor from '../MarkdownEditor.vue'
import { TOKEN_KEY } from '@/utils/storage'

vi.mock('vditor/dist/index.css', () => ({}))
vi.mock('vditor/dist/js/i18n/zh_CN.js', () => ({}))

const { getUserInfoMock } = vi.hoisted(() => ({ getUserInfoMock: vi.fn() }))
// auth store 走真实实现（派生 token），请求层的「按请求现取」单点用 spy 替身：
// 本 spec 要证的是**组件每次上传都走那个单点**，静默刷新链路本身由 stores/__tests__/tokenSingleSource.spec.ts 测。
const freshTokenCalls = vi.hoisted(() => vi.fn(async () => localStorage.getItem('token')))
vi.mock('@/api/client', async (importOriginal) => {
  const actual = (await importOriginal()) as Record<string, unknown>
  return { ...actual, getValidAccessToken: freshTokenCalls }
})
vi.mock('@/api/auth', () => ({ authApi: { getUserInfo: getUserInfoMock, logout: vi.fn() } }))

const vditorCtor = vi.hoisted(() => vi.fn())
vi.mock('vditor', () => ({ default: vditorCtor }))

const UPLOAD_URL = '/api/admin/featured-content/upload-image'

interface UploadConfig {
  url?: string
  /** 术前形态：创建时抓好的请求头快照；术后必须不存在（改由 setHeaders 每次现取） */
  headers?: Record<string, string>
  file?: (files: File[]) => Promise<File[]>
  setHeaders?: () => Record<string, string>
}

/** 安装 Vditor 替身，收集每次 new Vditor 收到的 options */
function installVditorMock(): Array<{ upload?: UploadConfig }> {
  const instances: Array<{ upload?: UploadConfig }> = []
  vditorCtor.mockImplementation(function (this: Record<string, unknown>, _el: HTMLElement, opts: { upload?: UploadConfig }) {
    instances.push(opts)
    this.destroy = vi.fn()
    this.getValue = () => ''
    this.setValue = vi.fn()
  })
  return instances
}

/** 造一枚 exp 在若干秒后的 JWT（client.ts 的 getValidAccessToken 只解载荷的 exp） */
function jwtWithExp(deltaSeconds: number): string {
  const payload = btoa(JSON.stringify({ exp: Math.floor(Date.now() / 1000) + deltaSeconds }))
  return `h.${payload}.s`
}

function mountEditor() {
  return mount(MarkdownEditor, {
    props: { modelValue: '正文', height: 400, uploadUrl: UPLOAD_URL },
    global: { plugins: [epLite()] }
  })
}

const oneFile = () => [new File(['x'], 'a.png', { type: 'image/png' })]

beforeEach(() => {
  localStorage.clear()
  setActivePinia(createPinia())
  getUserInfoMock.mockReset()
  getUserInfoMock.mockResolvedValue({ account: 'u1', role: 'hrwai_user', user_id: 1 })
  // 单点替身按用例计数（默认实现：返回 storage 里现有的 access，等价于「本地未过期」那一支）
  freshTokenCalls.mockReset()
  freshTokenCalls.mockImplementation(async () => localStorage.getItem('token'))
})

describe('插图上传的令牌：每次上传现取', () => {
  it('不在创建编辑器时抓令牌快照（术前形态回流即红）', async () => {
    // hoisted 的替身里只能写字面量 'token'，这里把它钉回 storage 单点的常量（否则两处会静默漂移）
    expect(TOKEN_KEY).toBe('token')
    localStorage.setItem(TOKEN_KEY, jwtWithExp(600))
    const instances = installVditorMock()

    mountEditor()
    await flushPromises()

    const upload = instances[0].upload!
    expect(upload.url).toBe(UPLOAD_URL)
    // 创建时就拼好的 headers 快照 = 长驻页面 401 的根因，必须没有
    expect(upload.headers).toBeUndefined()
    expect(typeof upload.file).toBe('function')
    expect(typeof upload.setHeaders).toBe('function')
  })

  it('长驻页面期间令牌被刷新（只写 storage）⇒ 下一次上传的头随之更新，不 401', async () => {
    const before = jwtWithExp(600)
    localStorage.setItem(TOKEN_KEY, before)
    const instances = installVditorMock()

    mountEditor()
    await flushPromises()
    const upload = instances[0].upload!

    // Vditor 的真实顺序：先 await upload.file(files)，再 setHeaders(vditor, xhr)
    //（vditor/src/ts/upload/index.ts:187 与 :230 → upload/setHeaders.ts:3）
    const files = oneFile()
    await expect(upload.file!(files)).resolves.toBe(files)
    // 每次上传发起前都经「按请求现取」的单点（本地过期则静默刷新，新令牌落 storage）
    expect(freshTokenCalls).toHaveBeenCalledTimes(1)
    expect(upload.setHeaders!()).toEqual({ Authorization: `Bearer ${before}` })

    // 静默刷新落地：api/client.ts 只写 storage（它不认识 store）
    const after = jwtWithExp(7200)
    localStorage.setItem(TOKEN_KEY, after)

    await upload.file!(oneFile())
    expect(freshTokenCalls).toHaveBeenCalledTimes(2)
    expect(upload.setHeaders!()).toEqual({ Authorization: `Bearer ${after}` })
  })

  it('未登录（storage 无令牌）⇒ 不硬拼空的 Authorization', async () => {
    const instances = installVditorMock()

    mountEditor()
    await flushPromises()
    const upload = instances[0].upload!

    await upload.file!(oneFile())

    expect(freshTokenCalls).toHaveBeenCalledTimes(1)
    expect(upload.setHeaders!()).toEqual({})
  })
})
