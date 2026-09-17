// AI 助手「失败 → 重试」页面路径（#1104）。
//
// 旧实现把失败/中断拼进正文（'正文\n\n[生成中断：…]' / '[生成失败：…]'），没有可挂重试的状态通道。
// #1104 把一轮发送的编排与三种终态收进 store：原因与该轮输入进 lastTurnError 独立通道，
// 壳在输入框上方渲染可重试入口，点击重试原样重发该轮输入。
// 本 spec 用**真 store + 真壳**（只替换 api 与渲染依赖）走完整页面路径，锁三件事：
//   ① 失败/中断不污染正文（状态文本清零）；② 独立通道被壳渲染成重试入口；
//   ③ 重试原样重发该轮输入（不是把输入框当前内容再发一遍）。
import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import { mount, flushPromises } from '@vue/test-utils'
import { createPinia, setActivePinia } from 'pinia'
import { createRouter, createMemoryHistory } from 'vue-router'
import { epLite } from '@/test/element-lite'

// streamChat 是真链路上的唯一网络面：捕获 handlers，用例内手动驱动 SSE 事件序
const mocks = vi.hoisted(() => ({
  streamChat: vi.fn(),
  warning: vi.fn(),
  error: vi.fn()
}))

// 页面/壳从 barrel 引 ElMessage；epLite 走 element-plus/es 子路径，不受此 mock 影响
vi.mock('element-plus', () => ({
  ElMessage: { warning: mocks.warning, error: mocks.error, success: vi.fn() },
  ElMessageBox: { confirm: vi.fn(), alert: vi.fn(), prompt: vi.fn() }
}))

vi.mock('@/api/aiAssistant', () => ({
  aiAssistantApi: {
    listAssistantModes: vi.fn().mockResolvedValue({
      normal: { id: 1, name: '默认模型', model: 'deepseek-chat', base_url: 'https://api.test' },
      expert: null
    }),
    listSessions: vi.fn().mockResolvedValue([]),
    createSession: vi.fn().mockResolvedValue({
      id: 11,
      title: '新会话',
      model_name: 'deepseek',
      feature_key: 'ai_assistant',
      created_at: '2026-01-01T00:00:00Z',
      updated_at: '2026-01-01T00:00:00Z'
    }),
    getSessionMessages: vi.fn().mockResolvedValue([]),
    streamChat: mocks.streamChat
  }
}))

vi.mock('@/stores/auth', () => ({
  useAuthStore: () => ({ isLoggedIn: true, userInfo: { username: 'tester' } })
}))
vi.mock('@/api/auth', () => ({ authApi: { logout: vi.fn().mockResolvedValue(undefined) } }))
vi.mock('@/utils/subdomain', () => ({ buildSubdomainUrl: vi.fn(() => '/') }))
vi.mock('@/stores/theme', () => ({
  useThemeStore: () => ({ mode: 'system', resolved: 'light', setMode: vi.fn(), cycle: vi.fn() })
}))
vi.mock('markstream-vue', () => ({
  default: {
    name: 'MarkdownRender',
    props: ['mode', 'content', 'final', 'htmlPolicy', 'fade'],
    template: '<div class="markdown-stub">{{ content }}</div>'
  }
}))
vi.mock('markstream-vue/index.css', () => ({}))

import AIAssistantPage from '../AIAssistantPage.vue'

const router = createRouter({
  history: createMemoryHistory(),
  routes: [{ path: '/:pathMatch(.*)*', component: { template: '<div/>' } }]
})

/** 第 n 次 streamChat 调用的 handlers（用例内手动驱动 SSE 事件序） */
function handlersAt(index = 0) {
  return mocks.streamChat.mock.calls[index][1] as {
    onChunk?: (c: string) => void
    onUsage?: (d: Record<string, number>) => void
    onSources?: (s: Array<Record<string, unknown>>) => void
    onDone?: () => void
    onError?: (m: string) => void
  }
}

/** 页面可见正文（用户气泡 + 助手气泡）拼接 */
function bubbleText(wrapper: ReturnType<typeof mount>) {
  return wrapper.findAll('.message-text').map(d => d.text()).join('\n')
}

let wrapper: ReturnType<typeof mount> | null = null

/** 挂载页面并等 init（模式 + 会话列表）落定 */
async function mountPage() {
  wrapper = mount(AIAssistantPage, { global: { plugins: [epLite(), router] } })
  await flushPromises()
  return wrapper
}

/** 打字 + Enter（壳的 Enter 与发送按钮同一判据） */
async function typeAndSend(w: ReturnType<typeof mount>, text: string) {
  await w.find('textarea').setValue(text)
  await w.find('textarea').trigger('keydown.enter')
  await flushPromises()
}

beforeEach(() => {
  vi.clearAllMocks()
  mocks.streamChat.mockImplementation(() => ({ abort: vi.fn() }))
  setActivePinia(createPinia())
})

afterEach(() => {
  wrapper?.unmount()
  wrapper = null
})

describe('AI 助手失败 → 重试页面路径（#1104）', () => {
  it('生成失败：正文不拼失败文本，壳渲染重试入口；点击重试原样重发该轮输入', async () => {
    const w = await mountPage()
    await typeAndSend(w, '液压压力不足怎么查？')
    expect(mocks.streamChat).toHaveBeenCalledTimes(1)

    handlersAt().onChunk?.('已经生成的一半')
    handlersAt().onError?.('上游超时')
    await flushPromises()

    // ① 正文只有生成内容（旧实现在这里拼 '\n\n[生成中断：上游超时]'）
    expect(bubbleText(w)).toContain('已经生成的一半')
    expect(bubbleText(w)).not.toContain('上游超时')
    expect(bubbleText(w)).not.toContain('生成中断')

    // ② 失败原因走独立通道，壳渲染可重试入口
    const banner = w.find('.turn-error')
    expect(banner.exists()).toBe(true)
    expect(banner.text()).toContain('生成失败：上游超时')

    // ③ 重试：原样重发该轮输入
    await banner.findAll('button').find(b => b.text().includes('重试'))!.trigger('click')
    await flushPromises()
    expect(mocks.streamChat).toHaveBeenCalledTimes(2)
    const retryReq = mocks.streamChat.mock.calls[1][0] as { messages: Array<Record<string, unknown>> }
    expect(retryReq.messages[retryReq.messages.length - 1]).toEqual({
      role: 'user',
      content: '液压压力不足怎么查？',
      images: undefined
    })
    // 新一轮开始即清掉失败通道（横幅消失）
    expect(w.find('.turn-error').exists()).toBe(false)
  })

  it('用户中断（停止）：部分正文保留、不出现「[已中断]」后缀，同样给重试入口', async () => {
    const w = await mountPage()
    await typeAndSend(w, '再讲讲电池保养')

    handlersAt().onChunk?.('中断前的内容')
    await flushPromises()
    const stopBtn = w.findAll('button').find(b => b.text().includes('停止'))
    expect(stopBtn).toBeTruthy()
    await stopBtn!.trigger('click')
    await flushPromises()

    expect(bubbleText(w)).toContain('中断前的内容')
    expect(bubbleText(w)).not.toContain('已中断')
    const banner = w.find('.turn-error')
    expect(banner.text()).toContain('已中断生成')
    expect(banner.findAll('button').some(b => b.text().includes('重试'))).toBe(true)
  })
})
