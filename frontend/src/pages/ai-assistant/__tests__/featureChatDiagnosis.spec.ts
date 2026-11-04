// 诊断页（FeatureChatPage，fault-diagnosis）页面级 spec —— 票 2244。
//
// 组件级 spec 已各自覆盖（Welcome / Guide / Badges / Sources / compose / compress），
// 这里只锁**页面把它们接起来的那几根线**，因为接错时单测全绿而页面是坏的：
//   ① 进页拉一次 intents + health，**不轮询**；两条路径（health 说 ok / 字典端点 502）
//      都必须落到同一个结论：三件套不画、提示条出来；
//   ② 上游健康时三件套出现、提示条不出现；
//   ③ 点品牌卡 / 诉求卡 → 打开向导并传预选（不是直接发一句话）；
//   ④ 向导组好句 → 走 store.send，且**不**经过输入框；
//   ⑤ FAQ 点一下 → 直接发这句话；
//   ⑥ 选图 → 先压缩再上传（上传的是压好的那份，不是原图）；
//   ⑦ 助手消息的 meta.badges / meta.elapsed 真的流到徽章行与来源面板。
import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import { mount, flushPromises } from '@vue/test-utils'
import { reactive } from 'vue'
import { epLite } from '@/test/element-lite'

const elMessage = vi.hoisted(() => ({ warning: vi.fn(), error: vi.fn(), success: vi.fn() }))
vi.mock('element-plus', () => ({ ElMessage: elMessage }))

const route = vi.hoisted(() => ({
  path: '/ai-assistant/fault-diagnosis',
  fullPath: '/ai-assistant/fault-diagnosis'
}))
vi.mock('vue-router', () => ({
  useRoute: () => route,
  useRouter: () => ({ push: vi.fn(), replace: vi.fn() })
}))

const stores = vi.hoisted(() => ({ store: undefined as unknown }))
vi.mock('@/stores/aiAssistant', () => ({ useAIAssistantStore: () => stores.store }))

// 壳替身：把页面用到的五个槽原样暴露 + 把 can-send / input-text 接成 props，
// 并给一个「发消息」入口（页面只通过 @send 与 store.send 两条路发东西）。
// `fakeMessage` 是给 assistant-caps / assistant-extra 两个槽喂消息用的：真壳从 store 读
// 当前轮消息，这里由用例直接注入（默认空消息，避免槽里读 undefined）。
vi.mock('@/components/ai-assistant/ChatPageShell.vue', () => ({
  default: {
    name: 'ChatPageShell',
    props: {
      canSend: { type: Boolean, default: false },
      inputText: { type: String, default: '' },
      fakeMessage: {
        type: Object,
        default: () => ({ id: 0, role: 'assistant', content: '', sources: [], meta: null })
      }
    },
    emits: ['send', 'update:inputText', 'suggest', 'new-session'],
    template: `
      <div class="shell-stub" :data-can-send="canSend ? '1' : '0'">
        <slot name="welcome-top" />
        <slot name="welcome-bottom" />
        <slot name="input-toolbar" />
        <slot name="input-above" />
        <slot name="input-prefix" />
        <div class="caps-host">
          <slot name="assistant-caps" :message="fakeMessage" />
        </div>
        <div class="extra-host">
          <slot name="assistant-extra" :message="fakeMessage" />
        </div>
      </div>
    `
  }
}))

vi.mock('@/components/ui/UiUpload.vue', () => ({
  default: {
    name: 'UiUpload',
    emits: ['change'],
    template: '<div class="upload-stub"><slot /></div>'
  }
}))

// 压缩件替身：只断「先压再传」这一条顺序，压缩算法本身由 utils 的 spec 锁
const compress = vi.hoisted(() => ({ compressImage: vi.fn() }))
vi.mock('@/utils/imageCompress', () => ({ compressImage: compress.compressImage }))

const api = vi.hoisted(() => ({
  getDiagnosisHealth: vi.fn(),
  listDiagnosisIntents: vi.fn(),
  listDiagnosisSystems: vi.fn(),
  listDiagnosisFaults: vi.fn(),
  checkDiagnosisCode: vi.fn()
}))
vi.mock('@/api/aiAssistant', async () => {
  const actual = await vi.importActual<Record<string, unknown>>('@/api/aiAssistant')
  return { ...actual, aiAssistantApi: api }
})

import FeatureChatPage from '../FeatureChatPage.vue'
import UiUploadStub from '@/components/ui/UiUpload.vue'
import DiagnosisGuide from '@/components/ai-assistant/DiagnosisGuide.vue'

const fine = {
  ok: true,
  app_configured: true
}
const brands = [
  { id: 'zhongli', name: '中力 EP', tagline: '电动叉车', icon: '🔋', cls: 'brand-zhongli', picker: 'system' },
  { id: 'linde', name: '林德 Linde', tagline: '按型号', icon: '🛠️', cls: 'brand-linde', picker: 'model' }
]
const faq = [{ q: '故障码怎么看？', a: '看仪表' }]

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

let wrapper: ReturnType<typeof mount> | null = null

function mountPage(attrs: Record<string, unknown> = {}) {
  wrapper = mount(FeatureChatPage, {
    attrs,
    global: { plugins: [epLite()], stubs: { MarkdownRender: true } }
  })
  return wrapper
}

function sets(selector: string) {
  return wrapper!.findAll(selector)
}

beforeEach(() => {
  compress.compressImage.mockReset().mockImplementation(async (f: File) => ({ file: f, compressed: false }))
  api.getDiagnosisHealth.mockReset().mockResolvedValue(fine)
  api.listDiagnosisIntents.mockReset().mockResolvedValue({ brands, faq })
  api.listDiagnosisSystems.mockReset().mockResolvedValue({ items: [] })
  api.listDiagnosisFaults.mockReset().mockResolvedValue({ items: [] })
  api.checkDiagnosisCode.mockReset().mockResolvedValue({ status: 'none', message: '', fix: '', samples: [] })

  stores.store = reactive({
    streaming: false,
    lastTurnError: null,
    initFeature: vi.fn(),
    startDraft: vi.fn(),
    send: vi.fn().mockResolvedValue(undefined),
    uploadImage: vi.fn().mockResolvedValue('/uploads/x.png')
  })
  ;(URL as unknown as { createObjectURL: unknown }).createObjectURL = vi.fn(() => 'blob:x')
  ;(URL as unknown as { revokeObjectURL: unknown }).revokeObjectURL = vi.fn()
})

afterEach(() => {
  wrapper?.unmount()
  wrapper = null
  vi.restoreAllMocks()
})

describe('进页取数：拉一次，不轮询', () => {
  it('mount 时同时拉 health 与 intents，各只调一次', async () => {
    mountPage()
    await flushPromises()
    expect(api.getDiagnosisHealth).toHaveBeenCalledTimes(1)
    expect(api.listDiagnosisIntents).toHaveBeenCalledTimes(1)
    expect(store().initFeature).toHaveBeenCalledWith('fault_diagnosis')
  })

  it('上游健康：三件套出现、提示条不出现', async () => {
    mountPage()
    await flushPromises()
    expect(sets('.brand-card')).toHaveLength(2)
    expect(sets('.scenario-card')).toHaveLength(6)
    expect(sets('.faq-item')).toHaveLength(1)
    expect(wrapper!.text()).not.toContain('智能诊断服务暂未启用')
  })

  it('health 说 ok=false（未配置）：三件套整块不画，出提示条', async () => {
    api.getDiagnosisHealth.mockResolvedValue({ ok: false, app_configured: false })
    mountPage()
    await flushPromises()
    expect(sets('.brand-card')).toHaveLength(0)
    expect(sets('.scenario-card')).toHaveLength(0)
    expect(wrapper!.text()).toContain('智能诊断服务暂未启用')
  })

  it('字典端点 502：同样只出提示条，不冒泡成未处理异常', async () => {
    api.listDiagnosisIntents.mockRejectedValue(new Error('502'))
    mountPage()
    await flushPromises()
    expect(sets('.brand-card')).toHaveLength(0)
    expect(wrapper!.text()).toContain('智能诊断服务暂未启用')
  })

  it('health 本身挂了（网络错）也不阻断页面：当不可用处理', async () => {
    api.getDiagnosisHealth.mockRejectedValue(new Error('network'))
    mountPage()
    await flushPromises()
    // health 拿不到 ⇒ 无法确认服务可用 ⇒ 按不可用处理（整块留白 + 提示条）。
    // 注意这与「intents 成功但 health 失败」不矛盾：品牌卡画出来的前提是 health 说 ok。
    expect(wrapper!.text()).toContain('智能诊断服务暂未启用')
    expect(sets('.brand-card')).toHaveLength(0)
  })
})

describe('点卡 → 打开向导并带预选（不是直接发一句话）', () => {
  it('点品牌卡：向导打开，且该品牌已预选（系统列表随即拉取）', async () => {
    mountPage()
    await flushPromises()
    await sets('.brand-card')[0].trigger('click')
    await flushPromises()

    const guide = wrapper!.findComponent(DiagnosisGuide)
    expect(guide.exists()).toBe(true)
    expect(guide.props('presetBrandId')).toBe('zhongli')
    expect(api.listDiagnosisSystems).toHaveBeenCalledWith('zhongli')
  })

  it('点诉求卡：向导打开且带上现象原文，品牌仍为空（要用户先选品牌）', async () => {
    mountPage()
    await flushPromises()
    await sets('.scenario-card')[2].trigger('click')
    await flushPromises()

    const guide = wrapper!.findComponent(DiagnosisGuide)
    expect(guide.props('presetFault')).toBe('举升无力 / 不能起升')
    expect(guide.props('presetBrandId')).toBe('')
  })

  it('两处点卡都不发消息（只有点发送才发）', async () => {
    mountPage()
    await flushPromises()
    await sets('.brand-card')[0].trigger('click')
    await sets('.scenario-card')[0].trigger('click')
    await flushPromises()
    expect(store().send).not.toHaveBeenCalled()
  })
})

describe('向导组句 → 发送', () => {
  it('向导 emit send 时，句子原样进 store.send（不经输入框）', async () => {
    mountPage()
    await flushPromises()
    // 向导默认收着（UiCollapse lazy）—— 先点胶囊打开，这一步本身就是页面的真实入口
    const capsule = wrapper!.findAll('button').find((b) => b.text().includes('引导选诉求'))
    expect(capsule).toBeTruthy()
    await capsule!.trigger('click')
    await flushPromises()

    wrapper!.findComponent(DiagnosisGuide).vm.$emit('send', '我的中力 EP 叉车（电池_BMS系统）出现故障现象：举升无力。请给我排查步骤、根因分析和维修方案。')
    await flushPromises()

    expect(store().send).toHaveBeenCalledTimes(1)
    const [text, opts] = store().send.mock.calls[0]
    expect(text).toContain('举升无力')
    expect(opts).toEqual({})
  })

  it('发完自动收起向导（下次打开不是上一轮的残局）', async () => {
    mountPage()
    await flushPromises()
    const capsule = wrapper!.findAll('button').find((b) => b.text().includes('引导选诉求'))
    await capsule!.trigger('click')
    await flushPromises()

    wrapper!.findComponent(DiagnosisGuide).vm.$emit('send', '一句话')
    await flushPromises()
    // 收起后胶囊文案回到「引导选诉求」（不是「收起引导」）
    const after = wrapper!.findAll('button').find((b) => b.text().includes('引导选诉求'))
    expect(after).toBeTruthy()
  })

  it('FAQ 点一下直接发这句问题', async () => {
    mountPage()
    await flushPromises()
    await sets('.faq-item')[0].trigger('click')
    await flushPromises()
    expect(store().send).toHaveBeenCalledWith('故障码怎么看？', {})
  })
})

describe('选图：先压缩再上传', () => {
  it('上传的是压缩后的 File，不是原图', async () => {
    const compressedFile = new File([new Uint8Array(200)], 'photo.jpg', { type: 'image/jpeg' })
    compress.compressImage.mockResolvedValue({ file: compressedFile, compressed: true })

    mountPage()
    await flushPromises()
    const raw = { name: 'photo.png', type: 'image/png' }
    wrapper!.findComponent(UiUploadStub).vm.$emit('change', { name: 'photo.png', raw })
    await flushPromises()

    expect(compress.compressImage).toHaveBeenCalledTimes(1)
    expect(compress.compressImage.mock.calls[0][0]).toBe(raw)
    expect(store().uploadImage).toHaveBeenCalledWith(compressedFile)
  })

  it('压缩失败（原样返回）也照样上传原图 —— 压缩不是前置条件', async () => {
    const raw = new File([new Uint8Array(10)], 'a.svg', { type: 'image/svg+xml' })
    compress.compressImage.mockResolvedValue({ file: raw, compressed: false })

    mountPage()
    await flushPromises()
    wrapper!.findComponent(UiUploadStub).vm.$emit('change', { name: 'a.svg', raw })
    await flushPromises()

    expect(store().uploadImage).toHaveBeenCalledWith(raw)
  })

  it('最多 3 张（与后端 diagnosisMaxImages 同值），第 4 张被挡下且提示', async () => {
    mountPage()
    await flushPromises()
    const upload = () => wrapper!.findComponent(UiUploadStub).vm.$emit('change', {
      name: 'x.png',
      raw: new File([new Uint8Array(10)], 'x.png', { type: 'image/png' })
    })
    for (let i = 0; i < 3; i++) {
      upload()
      await flushPromises()
    }
    expect(store().uploadImage).toHaveBeenCalledTimes(3)

    upload()
    await flushPromises()
    expect(store().uploadImage).toHaveBeenCalledTimes(3)
    expect(elMessage.warning).toHaveBeenCalledWith('最多上传 3 张图片')
  })

  // 票 2280 缺陷三：原实现的名额检查在 `await compressImage()` 之前、入队在之后，
  // 多选时五个回调各自读到同一个长度再各自入队 ⇒ 上限形同虚设（实测入队 5 张）。
  it('一次多选 5 张只入队 3 张，第 4/5 张既不压缩也不上传', async () => {
    mountPage()
    await flushPromises()
    const files = Array.from({ length: 5 }, (_, i) => ({
      name: `p${i}.png`,
      raw: new File([new Uint8Array(10)], `p${i}.png`, { type: 'image/png' })
    }))
    // 五个回调在同一个 tick 里连着触发（这正是「多选」的实际时序）
    wrapper!.findComponent(UiUploadStub).vm.$emit('change', files[0])
    wrapper!.findComponent(UiUploadStub).vm.$emit('change', files[1])
    wrapper!.findComponent(UiUploadStub).vm.$emit('change', files[2])
    wrapper!.findComponent(UiUploadStub).vm.$emit('change', files[3])
    wrapper!.findComponent(UiUploadStub).vm.$emit('change', files[4])
    await flushPromises()

    expect(compress.compressImage).toHaveBeenCalledTimes(3)
    expect(store().uploadImage).toHaveBeenCalledTimes(3)
    expect(elMessage.warning).toHaveBeenCalledWith('最多上传 3 张图片')
  })
})

describe('助手消息的元数据流到徽章与来源', () => {
  it('meta.badges 进徽章行，meta.elapsed 进来源面板', async () => {
    mountPage({
      fakeMessage: {
        id: 1,
        role: 'assistant',
        content: '答案',
        sources: [],
        meta: { badges: [{ name: '知识检索', emoji: '🔍', cls: 'cap-rag' }], elapsed: 12.3 }
      }
    })
    await flushPromises()

    const capsHost = wrapper!.find('.caps-host')
    expect(capsHost.findAll('[data-cap]')).toHaveLength(1)
    expect(capsHost.text()).toContain('知识检索')

    // 只有耗时没有来源：来源面板仍渲染（出一行耗时脚注），不是整块消失
    expect(wrapper!.find('.extra-host').text()).toContain('耗时 12.3 秒')
  })

  it('没有 meta 的消息：徽章行空、来源块不渲染，但复制动作仍在（票 2280 缺陷五）', async () => {
    mountPage({ fakeMessage: { id: 2, role: 'assistant', content: '答案', sources: [], meta: null } })
    await flushPromises()
    expect(wrapper!.find('.caps-host').findAll('[data-cap]')).toHaveLength(0)
    // 来源与耗时都没有 ⇒ 来源块整块不渲染（只剩复制按钮这一行）
    expect(wrapper!.find('.extra-host').find('.py-0\\.5').exists()).toBe(false)
    expect(wrapper!.find('.extra-host').text()).not.toContain('耗时')
    expect(wrapper!.find('.extra-host').text()).toContain('复制')
  })

  it('助手回答下方有复制按钮，点击写进剪贴板并提示成功', async () => {
    const writeText = vi.fn().mockResolvedValue(undefined)
    // jsdom 的 navigator.clipboard 只有 getter，必须用 defineProperty 覆盖
    Object.defineProperty(navigator, 'clipboard', {
      value: { writeText },
      configurable: true,
      writable: true
    })

    mountPage({ fakeMessage: { id: 3, role: 'assistant', content: '先查液压油位。', sources: [], meta: null } })
    await flushPromises()

    const copyBtn = wrapper!.find('.extra-host').findAll('button').find((b) => b.text().includes('复制'))
    expect(copyBtn).toBeTruthy()
    await copyBtn!.trigger('click')
    await flushPromises()

    expect(writeText).toHaveBeenCalledWith('先查液压油位。')
    expect(elMessage.success).toHaveBeenCalledWith('已复制')
  })

  it('剪贴板不可用（内网 http 非安全上下文）时给出可行动的提示，不静默失败', async () => {
    Object.defineProperty(navigator, 'clipboard', {
      value: { writeText: vi.fn().mockRejectedValue(new Error('NotAllowedError')) },
      configurable: true,
      writable: true
    })

    mountPage({ fakeMessage: { id: 4, role: 'assistant', content: '答案', sources: [], meta: null } })
    await flushPromises()

    const copyBtn = wrapper!.find('.extra-host').findAll('button').find((b) => b.text().includes('复制'))
    await copyBtn!.trigger('click')
    await flushPromises()

    expect(elMessage.warning).toHaveBeenCalledWith('复制失败，请手动选择文字复制')
  })
})
