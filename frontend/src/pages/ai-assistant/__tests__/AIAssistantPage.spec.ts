// AIAssistantPage 空态引导（#1061）：双模式均未绑定时，页面必须给出**可据以行动**的指引。
//
// 背景（#1044）：线上 `ai_assistant_normal` / `ai_assistant_expert` 均未绑定 ⇒ 通用对话
// 在 Web 完全发不出消息，而旧文案只说「当前模式未配置，请联系管理员在"AI 配置"中绑定」——
// 学员端 Web 没有自助配置入口（`/admin/ai-settings` 需 admin 权限，userModels 是移动端专属），
// 提示说了等于没说。本 spec 守两件事：
//   ① 空态文案说明状态 + 指向页内仍可用的专项功能（与功能胶囊同名，学员点得到）；
//   ② Enter/发送路径的 toast 同样可执行 —— 真壳的 handleEnter 不看 canSend，
//      所以「输入后按 Enter」是学员实际撞到提示的主路径（占位符也写着「Enter 发送」）。
import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import { mount } from '@vue/test-utils'
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
    emits: ['send', 'update:inputText'],
    template: `
      <div class="shell-stub">
        <slot name="input-toolbar" />
        <slot name="input-extra" />
        <button class="stub-type" @click="$emit('update:inputText', '叉车液压压力不足怎么查？')">type</button>
        <button class="stub-send" @click="$emit('send')">send</button>
      </div>
    `
  }
}))

import AIAssistantPage from '../AIAssistantPage.vue'

/** 复现线上状态：双模式皆未绑定（#1044 的 `/modes` → {normal:null,expert:null}） */
function makeStore(overrides: Record<string, unknown> = {}) {
  return reactive({
    streaming: false,
    selectedMode: 'normal',
    modeModels: { normal: null, expert: null },
    selectMode: vi.fn(),
    init: vi.fn(),
    sendMessage: vi.fn().mockResolvedValue(undefined),
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

  it('Enter/发送路径的 toast 同样可执行，不再只说「联系管理员」', async () => {
    wrapper = mountPage()
    await wrapper.find('.stub-type').trigger('click')
    await wrapper.find('.stub-send').trigger('click')

    expect(elMessage.warning).toHaveBeenCalledTimes(1)
    const msg = String(elMessage.warning.mock.calls[0][0])
    expect(msg).toContain('通用对话暂不可用')
    expect(msg).toContain('智能维修诊断')
  })

  it('模式可用时不出现空态指引（守卫只在真不可用时出现）', () => {
    stores.store = makeStore({
      modeModels: { normal: { id: 1, name: '默认', model: 'deepseek-chat', base_url: 'https://api.test' }, expert: null }
    })
    wrapper = mountPage()
    expect(wrapper.text()).not.toContain('通用对话暂不可用')
  })
})
