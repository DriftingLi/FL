// AIAssistantPage 空态引导（#1061）与发送委派（#1104）。
//
// 背景（#1044）：线上 `ai_assistant_normal` / `ai_assistant_expert` 均未绑定 ⇒ 通用对话
// 在 Web 完全发不出消息，而旧文案只说「当前模式未配置，请联系管理员在"AI 配置"中绑定」——
// 学员端 Web 没有自助配置入口（`/admin/ai-settings` 需 admin 权限，userModels 是移动端专属），
// 提示说了等于没说。本 spec 守：
//   ① 空态文案说明状态 + 指向页内仍可用的专项功能（与功能胶囊同名，学员点得到）；
//   ② Enter/发送路径的提示同样可执行 —— 壳的 Enter 与发送按钮读同一判据（#1104），而
//      「模式未绑定」**不在**该判据内（否则 Enter 会被静默拦掉，学员反而拿不到提示）：
//      发送被 store 的前置校验拒绝时，页面把原因转成 toast，并把问题留在输入框；
//   ③ #1104：发送编排收进 store —— 页面只做「清空输入 → store.send(正文)」，失败/中断
//      走 store.lastTurnError（壳渲染重试入口），页面不再自己拼「[生成失败：…]」。
import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import { mount, flushPromises } from '@vue/test-utils'
import { reactive } from 'vue'
import { epLite } from '@/test/element-lite'

// 页面从 barrel 引 ElMessage；epLite 走 element-plus/es 子路径，不受此 mock 影响
const elMessage = vi.hoisted(() => ({ warning: vi.fn(), success: vi.fn(), error: vi.fn() }))
vi.mock('element-plus', () => ({ ElMessage: elMessage }))

vi.mock('vue-router', () => ({ useRouter: () => ({ push: vi.fn() }) }))

const stores = vi.hoisted(() => ({ store: undefined as unknown }))
vi.mock('@/stores/aiAssistant', () => ({ useAIAssistantStore: () => stores.store }))

// 壳替身：只暴露本页要断言的两个槽（input-toolbar=专项功能胶囊、input-extra=空态引导），
// 并用两个按钮驱动页面逻辑——「填入文本」对应学员打字，「发送」对应按 Enter（真壳同一 emit）。
vi.mock('@/components/ai-assistant/ChatPageShell.vue', () => ({
  default: {
    name: 'ChatPageShell',
    props: ['inputText'],
    emits: ['send', 'update:inputText'],
    template: `
      <div class="shell-stub">
        <slot name="input-toolbar" />
        <slot name="input-extra" />
        <span class="stub-input">{{ inputText }}</span>
        <button class="stub-type" @click="$emit('update:inputText', '叉车液压压力不足怎么查？')">type</button>
        <button class="stub-send" @click="$emit('send')">send</button>
      </div>
    `
  }
}))

import AIAssistantPage from '../AIAssistantPage.vue'

/** 复现线上状态：双模式皆未绑定（#1044 的 `/modes` → {normal:null,expert:null}） */
function makeStore(overrides: Record<string, unknown> = {}) {
  const modeModels = (overrides.modeModels as { normal: unknown; expert: unknown }) ?? { normal: null, expert: null }
  const modeAvailable = !!(modeModels.normal || modeModels.expert)
  return reactive({
    streaming: false,
    selectedMode: 'normal',
    modeModels,
    selectMode: vi.fn(),
    init: vi.fn(),
    // 真 store 的前置校验语义（#1061）：通用模式未绑定时 send 抛可执行文案，页面不得吞掉
    send: modeAvailable
      ? vi.fn().mockResolvedValue(undefined)
      : vi.fn().mockRejectedValue(new Error('通用对话暂不可用（平台模型未配置）；可改用「智能维修诊断」等专项功能，或联系管理员配置')),
    retryLastTurn: vi.fn().mockResolvedValue(undefined),
    lastTurnError: null,
    ...overrides
  })
}

function mountPage() {
  return mount(AIAssistantPage, { global: { plugins: [epLite()] } })
}

let wrapper: ReturnType<typeof mountPage> | null = null

beforeEach(() => {
  elMessage.warning.mockClear()
  stores.store = makeStore()
})

afterEach(() => {
  wrapper?.unmount()
  wrapper = null
})

describe('双模式未绑定时的空态引导（#1061）', () => {
  it('空态渲染可执行指引：说明哪条通道不可用，并点到专项功能', () => {
    wrapper = mountPage()
    const text = wrapper.text()
    expect(text).toContain('通用对话暂不可用')
    expect(text).toContain('智能维修诊断')
  })

  it('指引指向的专项功能确实在页内可点（胶囊同名，不是指向不存在的入口）', () => {
    wrapper = mountPage()
    const capsule = wrapper.findAll('button').find(b => b.text().includes('智能维修诊断'))
    expect(capsule).toBeTruthy()
  })

  it('Enter/发送路径的提示同样可执行，不再只说「联系管理员」', async () => {
    wrapper = mountPage()
    await wrapper.find('.stub-type').trigger('click')
    await wrapper.find('.stub-send').trigger('click')

    expect(elMessage.warning).toHaveBeenCalledTimes(1)
    const msg = String(elMessage.warning.mock.calls[0][0])
    expect(msg).toContain('通用对话暂不可用')
    expect(msg).toContain('智能维修诊断')
  })

  it('发送被拒绝时不吞掉问题：可执行文案进 toast，输入文本留在输入框', async () => {
    wrapper = mountPage()
    await wrapper.find('.stub-type').trigger('click')
    await wrapper.find('.stub-send').trigger('click')
    await flushPromises()

    // 未发出：store.send 被调用但拒绝；问题原样还给学员（页面清空后按失败回填）
    expect((stores.store as any).send).toHaveBeenCalledWith('叉车液压压力不足怎么查？')
    expect(elMessage.warning).toHaveBeenCalledTimes(1)
    expect(wrapper.find('.stub-input').text()).toBe('叉车液压压力不足怎么查？')
  })

  it('模式可用时不出现空态指引（守卫只在真不可用时出现）', () => {
    stores.store = makeStore({
      modeModels: { normal: { id: 1, name: '默认', model: 'deepseek-chat', base_url: 'https://api.test' }, expert: null }
    })
    wrapper = mountPage()
    expect(wrapper.text()).not.toContain('通用对话暂不可用')
  })
})

describe('发送委派（#1104：编排与终态收进 store）', () => {
  it('模式可用：页面只清空输入并调用 store.send(正文)，不再自带 streaming 守卫与吞错', async () => {
    stores.store = makeStore({
      modeModels: { normal: { id: 1, name: '默认', model: 'deepseek-chat', base_url: 'https://api.test' }, expert: null }
    })
    wrapper = mountPage()
    await wrapper.find('.stub-type').trigger('click')
    await wrapper.find('.stub-send').trigger('click')
    await flushPromises()

    const store = stores.store as any
    expect(store.send).toHaveBeenCalledTimes(1)
    expect(store.send).toHaveBeenCalledWith('叉车液压压力不足怎么查？')
    // 输入文本是页面 UI 态：发出即清空（失败才回填）
    expect(wrapper.find('.stub-input').text()).toBe('')
  })

  it('空输入不发起：连 store.send 都不调（判据在壳的 canSend，页面兜底不误发）', async () => {
    stores.store = makeStore({
      modeModels: { normal: { id: 1, name: '默认', model: 'deepseek-chat', base_url: 'https://api.test' }, expert: null }
    })
    wrapper = mountPage()
    await wrapper.find('.stub-send').trigger('click')

    expect((stores.store as any).send).not.toHaveBeenCalled()
  })
})
