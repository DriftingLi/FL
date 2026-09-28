// 图片待发队列（pendingImages）的身份与响应式 —— 票 #1358 / spec #1345 真实缺陷 #10。
//
// 术前形态：`pendingImages.value.push(裸对象)`。Vue 把数组做成深层响应式后，**读回**元素的
// 那一刻才为它建代理，于是 `arr[i] !== pending`（代理 vs 裸对象）恒成立 ⇒
//   ① catch 分支「按身份从队列里摘掉失败项」的过滤一条都不命中，失败项永远留在队列里；
//   ② `pending.url = ...` / `pending.uploading = false` 写在裸对象上、绕过代理 ⇒ 不触发重渲染
//      （「上传中」蒙层不会自己消失，canSend 也不会立刻翻真）。
// 术后形态：`const p = reactive({...}); push(p)` —— reactive 幂等，p 就是 arr[i] 读回的那个代理。
//
// 本 spec 用真页面组件（壳与上传件按仓库约定替身：只暴露本页要断言的两个槽）锁票面三条判据：
//   判据 1 上传完成后 canSend 立即为真、蒙层消失；
//   判据 2 上传失败后该项被移除且 blob URL 被释放；
//   判据 3 「失败项按身份移除」这一条有覆盖（术前无覆盖，且判据 1/2 在术前直接红）。
import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import { mount, flushPromises } from '@vue/test-utils'
import { reactive } from 'vue'
import { epLite } from '@/test/element-lite'

// 页面从 barrel 引 ElMessage；epLite 走 element-plus/es 子路径，不受此 mock 影响
const elMessage = vi.hoisted(() => ({ warning: vi.fn(), error: vi.fn(), success: vi.fn() }))
vi.mock('element-plus', () => ({ ElMessage: elMessage }))

// 图纸识别（drawing_recognition）：supportsImage=true、maxImages=4、非诊断适配器
// ⇒ 页面渲染图片上传按钮与待发队列，且不拉品牌/故障码目录。
const route = vi.hoisted(() => ({ path: '/ai-assistant/drawing', fullPath: '/ai-assistant/drawing' }))
vi.mock('vue-router', () => ({
  useRoute: () => route,
  useRouter: () => ({ push: vi.fn(), replace: vi.fn() })
}))

// store 替身：uploadImage 是本 spec 的唯一网络面（真 store 只是转调 api 层，见 stores/aiAssistant.ts）
const stores = vi.hoisted(() => ({ store: undefined as unknown }))
vi.mock('@/stores/aiAssistant', () => ({ useAIAssistantStore: () => stores.store }))

// 壳替身：暴露本页要断言的两个槽（input-above=待发队列、input-prefix=上传按钮），
// 并把 can-send 判据原样接成 prop —— 真壳的合成逻辑不在本票射程，判据看的就是这个入参。
vi.mock('@/components/ai-assistant/ChatPageShell.vue', () => ({
  default: {
    name: 'ChatPageShell',
    props: ['canSend', 'inputText'],
    emits: ['send', 'update:inputText', 'suggest', 'new-session'],
    template: `
      <div class="shell-stub" :data-can-send="canSend ? '1' : '0'">
        <slot name="input-above" />
        <slot name="input-prefix" />
      </div>
    `
  }
}))

// 上传件替身：真件是 el-upload 薄封装，@change 的载荷就是 Element Plus 的 UploadFile；
// 用例直接用 $emit('change', file) 驱动页面的 handleImageSelect。
vi.mock('@/components/ui/UiUpload.vue', () => ({
  default: {
    name: 'UiUpload',
    emits: ['change'],
    template: '<div class="upload-stub"><slot /></div>'
  }
}))

// 来源面板（诊断专属，本 feature 不渲染）带 markstream 重依赖，替身掉以免拖慢本 spec
vi.mock('@/components/ai-assistant/DiagnosisSources.vue', () => ({
  default: { name: 'DiagnosisSources', template: '<div class="sources-stub" />' }
}))

import FeatureChatPage from '../FeatureChatPage.vue'
import ChatPageShellStub from '@/components/ai-assistant/ChatPageShell.vue'
import UiUploadStub from '@/components/ui/UiUpload.vue'

type MockFn = ReturnType<typeof vi.fn>
interface StoreStub {
  streaming: boolean
  lastTurnError: unknown
  initFeature: MockFn
  startDraft: MockFn
  send: MockFn
  uploadImage: MockFn
}
const store = () => stores.store as StoreStub

function mountPage() {
  return mount(FeatureChatPage, { global: { plugins: [epLite()] } })
}

let wrapper: ReturnType<typeof mountPage> | null = null

/** blob URL 计数（每个文件一个，可据此断言「摘掉的是哪一张」与「释放的是哪一张」） */
let blobSeq = 0
const createObjectURL = vi.fn(() => `blob:preview/${++blobSeq}`)
const revokeObjectURL = vi.fn()
/** Vue 的运行期告警（console.warn）：排队两张图时若 v-for key 撞车，keyed diff 会直接打这条 */
let vueWarnings: string[] = []

function mountPageAndAssign() {
  wrapper = mountPage()
  return wrapper
}

/** 壳收到的 can-send 判据（页面侧就是 draftReady 这个 computed） */
function canSend() {
  return wrapper!.findComponent(ChatPageShellStub).props('canSend') as boolean
}

const masks = () => wrapper!.findAll('.pending-image-mask')
const items = () => wrapper!.findAll('.pending-image-item')
const thumbSrcs = () => wrapper!.findAll('.pending-image-thumb').map(t => t.attributes('src'))
const revokedUrls = () => revokeObjectURL.mock.calls.map(c => c[0])
/**
 * v-for key 撞车的运行期告警（key 用 url 时必然出现：上传中 url 恒为空串，
 * 排队两张就是两个同名 key，keyed diff 移除时可能摘错节点）⇒ 每次多图排队都断言它为空。
 */
const duplicateKeyWarnings = () => vueWarnings.filter(w => /Duplicate keys/i.test(w))

/** 学员选了一张图：走 UiUpload 的 @change（真链路上就是这一个事件） */
function pickImage(name: string) {
  wrapper!.findComponent(UiUploadStub).vm.$emit('change', { name, raw: { name, type: 'image/png' } })
}

/** 可控结束的上传（用例自己决定何时 resolve / reject） */
function deferred() {
  let resolve!: (url: string) => void
  let reject!: (err: Error) => void
  const promise = new Promise<string>((res, rej) => {
    resolve = res
    reject = rej
  })
  promise.catch(() => undefined)
  return { promise, resolve, reject }
}

beforeEach(() => {
  blobSeq = 0
  createObjectURL.mockImplementation(() => `blob:preview/${++blobSeq}`)
  revokeObjectURL.mockReset()
  elMessage.error.mockReset()
  elMessage.warning.mockReset()
  // URL.createObjectURL / revokeObjectURL 在 happy-dom 下不一定存在，直接接管这两个面
  ;(URL as unknown as { createObjectURL: unknown }).createObjectURL = createObjectURL
  ;(URL as unknown as { revokeObjectURL: unknown }).revokeObjectURL = revokeObjectURL
  vueWarnings = []
  vi.spyOn(console, 'warn').mockImplementation((...args: unknown[]) => {
    vueWarnings.push(args.map(String).join(' '))
  })

  stores.store = reactive({
    streaming: false,
    lastTurnError: null,
    initFeature: vi.fn(),
    startDraft: vi.fn(),
    send: vi.fn().mockResolvedValue(undefined),
    uploadImage: vi.fn()
  })
})

afterEach(() => {
  wrapper?.unmount()
  wrapper = null
  vi.restoreAllMocks()
})

describe('上传完成：蒙层消失与 canSend 翻真（判据 1）', () => {
  it('上传中蒙层在且 canSend 为假；resolve 后同一轮就消失并翻真', async () => {
    const up = deferred()
    // 返回值就是这张图的服务器 URL：裸对象入队时这句写在代理之外，视图不会重渲染
    store().uploadImage.mockReturnValue(up.promise)

    mountPageAndAssign()
    pickImage('a.png')
    await flushPromises()

    expect(items()).toHaveLength(1)
    expect(masks()).toHaveLength(1)
    expect(canSend()).toBe(false)

    up.resolve('https://cdn/a.png')
    await flushPromises()

    expect(masks()).toHaveLength(0)
    expect(canSend()).toBe(true)
    // 队列里那张缩略图仍在，用的就是本地 blob 预览
    expect(items()).toHaveLength(1)
    expect(thumbSrcs()).toEqual(['blob:preview/1'])
  })
})

describe('上传失败：按身份摘掉失败项（判据 2 + 判据 3）', () => {
  it('两张图里失败的那张被摘掉、成功的留在队列，且只释放失败项的 blob URL', async () => {
    const slow = deferred()
    store().uploadImage
      .mockReturnValueOnce(slow.promise)
      .mockRejectedValueOnce(new Error('图片过大'))

    mountPageAndAssign()
    pickImage('ok.png')
    pickImage('bad.png')
    await flushPromises()

    // 身份过滤真的命中了：队列只剩第一张（术前 filter 一条都不删，这里会是两张）
    expect(items()).toHaveLength(1)
    expect(thumbSrcs()).toEqual(['blob:preview/1'])
    // 只释放失败项的 blob，成功项的预览还在用
    expect(revokedUrls()).toEqual(['blob:preview/2'])
    expect(elMessage.error).toHaveBeenCalledWith('图片过大')
    // 排队两张 + 摘掉一张的整个过程中，keyed diff 没撞过 key
    expect(duplicateKeyWarnings()).toEqual([])

    // 成功那张随后完成：队列一张、无蒙层、可发送
    slow.resolve('https://cdn/ok.png')
    await flushPromises()
    expect(masks()).toHaveLength(0)
    expect(canSend()).toBe(true)
    expect(thumbSrcs()).toEqual(['blob:preview/1'])
  })

  it('两张都失败：队列清空、两个 blob URL 都释放（失败项不会互相顶替）', async () => {
    store().uploadImage
      .mockRejectedValueOnce(new Error('e1'))
      .mockRejectedValueOnce(new Error('e2'))

    mountPageAndAssign()
    pickImage('a.png')
    pickImage('b.png')
    await flushPromises()

    expect(items()).toHaveLength(0)
    expect(revokedUrls()).toEqual(['blob:preview/1', 'blob:preview/2'])
    // 队列空 + 无文本 ⇒ 不可发送
    expect(canSend()).toBe(false)
  })

  it('手动移除剩余项：blob URL 同样释放，队列与 DOM 一致', async () => {
    store().uploadImage.mockResolvedValue('https://cdn/a.png')

    mountPageAndAssign()
    pickImage('a.png')
    pickImage('b.png')
    await flushPromises()
    expect(items()).toHaveLength(2)

    await items()[0].find('.pending-image-remove').trigger('click')
    await flushPromises()

    expect(items()).toHaveLength(1)
    expect(thumbSrcs()).toEqual(['blob:preview/2'])
    expect(revokedUrls()).toEqual(['blob:preview/1'])
    // 两张都上传成功（这里连服务器 URL 都相同）时也不该撞 key —— key 用的是每个 blob 唯一的 previewUrl
    expect(duplicateKeyWarnings()).toEqual([])
  })
})
