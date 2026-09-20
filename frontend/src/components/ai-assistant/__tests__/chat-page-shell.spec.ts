// ChatPageShell 壳测试（#398）：槽位渲染、安全渲染单点（markstream escape、无裸 v-html）、
// 侧栏会话操作（重命名/删除/选中）、输入区变体与发送事件、自动滚底钩子不回归。
import { describe, it, expect, vi, beforeEach } from 'vitest'
import { mount, flushPromises } from '@vue/test-utils'
import { reactive, nextTick } from 'vue'
import { epLite } from '@/test/element-lite'
import { testRouter } from '@/test/router'

// 壳在重试被拒时把原因弹给用户（断言用；epLite 走 element-plus/es 子路径，不受影响）
const ElMessage = vi.hoisted(() => ({ warning: vi.fn(), error: vi.fn(), success: vi.fn() }))
vi.mock('element-plus', () => ({ ElMessage }))

import ChatPageShell from '../ChatPageShell.vue'

// ===== 模块替身：store / auth / 路由工具 / markstream =====
const mocks = vi.hoisted(() => ({ store: undefined as any }))

vi.mock('@/stores/aiAssistant', () => ({
  useAIAssistantStore: () => mocks.store
}))
vi.mock('@/stores/auth', () => ({
  useAuthStore: () => ({
    userInfo: { username: 'tester' },
    // 票1：登出单点——壳只调 signOut，不再各写 revoke+清理
    signOut: vi.fn().mockResolvedValue({ revoked: true })
  })
}))
vi.mock('@/utils/subdomain', () => ({
  buildSubdomainUrl: vi.fn(() => '/')
}))
vi.mock('markstream-vue', () => ({
  default: {
    name: 'MarkdownRender',
    props: ['mode', 'content', 'final', 'htmlPolicy', 'fade'],
    template: '<div class="markdown-stub">{{ content }}</div>'
  }
}))
vi.mock('markstream-vue/index.css', () => ({}))
// 主题 store 是 pinia store 且模块顶层创建 matchMedia —— 测试中 mock 掉
vi.mock('@/stores/theme', () => ({
  useThemeStore: () => ({
    mode: 'system',
    resolved: 'light',
    setMode: vi.fn(),
    cycle: vi.fn()
  })
}))

function makeStore(overrides: Record<string, unknown> = {}) {
  return reactive({
    isLoggedIn: true,
    sessionsLoading: false,
    sessions: [{ id: 1, title: '会话一', updated_at: '2026-01-01T00:00:00Z' }],
    currentSessionId: 1,
    messages: [] as Array<Record<string, unknown>>,
    streaming: false,
    streamingContent: '',
    // #620：当轮 usage 独立 state + 状态变更 action（壳渲染脚注、登出走 action）
    lastUsage: null as Record<string, number> | null,
    // #1104：发送判据（壳把页面 prop 与 store 闸门合成）与当轮失败/中断独立通道
    canSend: true,
    lastTurnError: null as Record<string, unknown> | null,
    retryLastTurn: vi.fn().mockResolvedValue(undefined),
    clearMessages: vi.fn(),
    loadSessions: vi.fn().mockResolvedValue(undefined),
    selectSession: vi.fn(),
    deleteSession: vi.fn().mockResolvedValue(undefined),
    renameSession: vi.fn().mockResolvedValue(undefined),
    stopStreaming: vi.fn(),
    ...overrides
  })
}

// 路由表从页面描述符派生：壳里的 href('StudentProfile') / href('Login') 是具名位置，
// 通配 catch-all 只吃路径不吃名字。见 @/test/router。
const router = testRouter()

function mountShell(props: Record<string, unknown> = {}, slots: Record<string, any> = {}) {
  return mount(ChatPageShell, {
    props: {
      logoSub: 'AI 叉车助手 · 测试',
      inputText: '',
      loginRedirect: '/ai-assistant',
      welcomeIcon: { template: '<i/>' },
      welcomeTitle: '欢迎标题',
      welcomeDesc: '欢迎描述',
      ...props
    },
    slots,
    global: {
      plugins: [epLite(), router]
    }
  })
}

beforeEach(() => {
  mocks.store = makeStore()
})

describe('ChatPageShell 槽位', () => {
  it('空状态渲染欢迎区：welcome-top 槽位 → 预设提示词翻页 → welcome-bottom 槽位顺序', async () => {
    const w = mountShell(
      { suggestions: ['问题一', '问题二', '问题三', '问题四'] },
      {
        'welcome-top': '<div class="slot-top">快捷选项</div>',
        'welcome-bottom': '<div class="slot-bottom">功能入口</div>'
      }
    )
    const area = w.find('.welcome-area')
    expect(area.exists()).toBe(true)
    const html = area.html()
    expect(html.indexOf('slot-top')).toBeGreaterThan(-1)
    expect(html.indexOf('slot-top')).toBeLessThan(html.indexOf('suggestion-pager'))
    expect(html.indexOf('suggestion-pager')).toBeLessThan(html.indexOf('slot-bottom'))
    // 游客提示仅未登录时渲染
    expect(w.find('.guest-hint').exists()).toBe(false)
  })

  it('预设提示词横向翻页：默认 3 条一页，箭头切换页面', async () => {
    const w = mountShell({ suggestions: ['一', '二', '三', '四'] })
    expect(w.find('.suggestion-pager').exists()).toBe(true)
    expect(w.findAll('.suggestion-page').length).toBe(2)
    // 第一页可见第一条
    expect(w.find('.suggestion-page').text()).toContain('一')
    const pager = w.find('.suggestion-pager')
    const arrows = pager.findAll('button')
    expect(arrows.length).toBe(2)
    expect(arrows[0].attributes('disabled')).toBeDefined()
    await arrows[1].trigger('click')
    expect(w.findAll('.suggestion-page')[1].text()).toContain('四')
  })

  it('欢迎区横排：一句话标题无副标题（welcomeDesc 不再渲染）', () => {
    const w = mountShell({ welcomeTitle: '上传图纸即读懂部件原理', welcomeDesc: '旧副标题不应出现' })
    const head = w.find('.welcome-head')
    expect(head.exists()).toBe(true)
    expect(head.text()).toContain('上传图纸即读懂部件原理')
    expect(w.find('.welcome-desc').exists()).toBe(false)
    expect(w.text()).not.toContain('旧副标题不应出现')
  })

  it('气泡自动换行不省略（无 truncate，超长撑开）', () => {
    const w = mountShell({ suggestions: ['一', '二', '三', '四'] })
    const bubbles = w.findAll('.suggestion-bubble')
    expect(bubbles.length).toBeGreaterThan(0)
    for (const b of bubbles) {
      expect(b.classes()).not.toContain('truncate')
    }
  })

  it('空态：grid 三行布局——输入框正中行精确居中、欢迎区第一行底对齐依托其上；有消息沉底 1200', async () => {
    const welcome = mountShell({})
    const mainEl = welcome.find('.chat-main')
    expect(mainEl.classes()).toContain('grid')
    expect(mainEl.classes()).toContain('grid-rows-[1fr_auto_1fr]')
    expect(welcome.find('.message-list').classes()).toContain('row-start-1')
    expect(welcome.find('.message-list').classes()).toContain('self-end')
    const inputArea = welcome.find('.chat-input-area')
    expect(inputArea.classes()).toContain('row-start-2')
    expect(inputArea.classes()).toContain('max-w-[760px]')

    mocks.store.messages = [{ id: 2, role: 'user', content: 'hi' }]
    const chatting = mountShell({})
    expect(chatting.find('.welcome-area').exists()).toBe(false)
    expect(chatting.find('.message-list').classes()).toContain('max-w-[1200px]')
    expect(chatting.find('.chat-input-area').classes()).toContain('max-w-[1200px]')
  })

  it('input-toolbar 与 input-above 共存：工具栏在上、图片队列在下（图纸/习题页无挤占回归）', () => {
    const w = mountShell({}, {
      'input-toolbar': '<div class="toolbar-slot">胶囊</div>',
      'input-above': '<div class="pending-slot">图片队列</div>'
    })
    const area = w.find('.chat-input-area')
    expect(area.find('.toolbar-slot').exists()).toBe(true)
    expect(area.find('.pending-slot').exists()).toBe(true)
    const html = area.html()
    expect(html.indexOf('toolbar-slot')).toBeLessThan(html.indexOf('input-wrap'))
    // 图片队列挂输入框上方（input-above），不进消息区、不顶回答正文
    expect(html.indexOf('pending-slot')).toBeLessThan(html.indexOf('input-wrap'))
  })

  it('assistant-extra 槽位透传助手消息（诊断逐轮来源回放挂载点）', () => {
    mocks.store = makeStore({
      messages: [{ id: 2, role: 'assistant', content: '回答', sources: [{ id: 1 }] }]
    })
    const w = mountShell({}, {
      'assistant-extra': '<div class="extra-slot">{{ JSON.stringify({ n: 1 }) }}</div>'
    })
    expect(w.find('.extra-slot').exists()).toBe(true)
  })

  it('raised 布局渲染 input-footer-left 槽位；compact 布局渲染 input-prefix 槽位与 input-above', () => {
    const raised = mountShell(
      { raisedInput: true },
      { 'input-footer-left': '<div class="mode-slot">模式</div>' }
    )
    expect(raised.find('.input-wrap--raised').exists()).toBe(true)
    expect(raised.find('.mode-slot').exists()).toBe(true)

    const compact = mountShell(
      {},
      {
        'input-prefix': '<button class="upload-slot">上传</button>',
        'input-above': '<div class="pending-slot">图片队列</div>'
      }
    )
    expect(compact.find('.input-wrap--raised').exists()).toBe(false)
    expect(compact.find('.input-wrap.has-image').exists()).toBe(true)
    expect(compact.find('.upload-slot').exists()).toBe(true)
    expect(compact.find('.pending-slot').exists()).toBe(true)
  })
})

describe('ChatPageShell 安全渲染', () => {
  it('助手消息统一走 markstream escape（html-policy 透传），无裸 v-html 渲染点', async () => {
    mocks.store.messages = [
      { id: 2, role: 'assistant', content: '<script>alert(1)</script>**hi**' }
    ]
    const w = mountShell({})
    const stub = w.findComponent({ name: 'MarkdownRender' })
    expect(stub.exists()).toBe(true)
    expect(stub.props('htmlPolicy')).toBe('escape')
    expect(stub.props('content')).toContain('**hi**')
    // 页面自身不再有 markdown-body / v-html 渲染点
    expect(w.find('.markdown-body').exists()).toBe(false)
    expect(w.find('.message-text .markdown-stub').exists()).toBe(true)
  })

  it('流式增量内容同样经 markstream 渲染', async () => {
    mocks.store.streaming = true
    mocks.store.streamingContent = '生成中…'
    const w = mountShell({})
    const stub = w.findComponent({ name: 'MarkdownRender' })
    expect(stub.exists()).toBe(true)
    expect(stub.props('htmlPolicy')).toBe('escape')
  })
})

describe('ChatPageShell 侧栏与会话操作', () => {
  it('未登录渲染登录引导侧栏（无会话项）；登录后渲染会话项', async () => {
    mocks.store = makeStore({ isLoggedIn: false })
    const guest = mountShell({})
    expect(guest.find('.session-sidebar').exists()).toBe(true)
    expect(guest.text()).toContain('登录 HRWAI 账号')
    expect(guest.find('.session-item').exists()).toBe(false)

    mocks.store = makeStore()
    const logged = mountShell({})
    expect(logged.find('.session-item').exists()).toBe(true)
  })

  it('点击会话触发 selectSession；开启新对话按钮发出 new-session', async () => {
    const w = mountShell({})
    await w.find('.session-item').trigger('click')
    expect(mocks.store.selectSession).toHaveBeenCalledWith(1)
    await w.find('.new-chat-btn').trigger('click')
    expect(w.emitted('new-session')).toBeTruthy()
  })

  it('enable-rename 时显示重命名按钮，点击进入编辑态', async () => {
    const w = mountShell({ enableRename: true })
    expect(w.find('.session-rename').exists()).toBe(true)
    await w.find('.session-rename').trigger('click')
    await nextTick()
    expect(w.find('.session-item.editing').exists()).toBe(true)
    expect(w.find('.session-item.editing input').exists()).toBe(true)
  })

  it('未启用 enable-rename 时不渲染重命名按钮（功能页零 diff）', () => {
    const w = mountShell({})
    expect(w.find('.session-rename').exists()).toBe(false)
    expect(w.find('.session-delete').exists()).toBe(true)
  })
})

describe('ChatPageShell 输入与发送', () => {
  it('输入触发 update:inputText，点击发送按钮发出 send', async () => {
    const w = mountShell({ canSend: true })
    await w.find('textarea').setValue('你好')
    expect(w.emitted('update:inputText')?.[0]).toEqual(['你好'])
    await w.find('textarea').trigger('keydown.enter')
    expect(w.emitted('send')).toBeTruthy()
    const sendBtn = w.findAll('button').find(b => b.text().includes('发送'))
    await sendBtn!.trigger('click')
    expect(w.emitted('send')!.length).toBe(2)
  })

  it('Enter 与发送按钮读同一判据（#1104）：不能发送时两者都不发出 send', async () => {
    const w = mountShell({ canSend: false })
    await w.find('textarea').trigger('keydown.enter')
    expect(w.emitted('send')).toBeFalsy()
    const sendBtn = w.findAll('button').find(b => b.text().includes('发送'))
    expect(sendBtn!.attributes('disabled')).toBeDefined()
    await sendBtn!.trigger('click')
    expect(w.emitted('send')).toBeFalsy()
  })

  it('Enter 与发送按钮读同一判据：store 闸门关闭（流式中）时 Enter 也不发出 send', async () => {
    mocks.store.canSend = false
    mocks.store.streaming = true
    const w = mountShell({ canSend: true })
    await w.find('textarea').trigger('keydown.enter')
    expect(w.emitted('send')).toBeFalsy()
  })

  it('流式进行中：「新对话」入口显式反馈（禁用 + 标题说明），不再静默吞掉点击', async () => {
    mocks.store.streaming = true
    const w = mountShell({})
    const newChat = w.find('.new-chat-btn')
    expect(newChat.attributes('disabled')).toBeDefined()
    expect(newChat.attributes('title')).toContain('生成中')
    await newChat.trigger('click')
    expect(w.emitted('new-session')).toBeFalsy()
  })

  it('流式中切换为停止按钮并调用 stopStreaming', async () => {
    mocks.store.streaming = true
    const w = mountShell({})
    const stopBtn = w.findAll('button').find(b => b.text().includes('停止'))
    expect(stopBtn).toBeTruthy()
    await stopBtn!.trigger('click')
    expect(mocks.store.stopStreaming).toHaveBeenCalled()
  })

  it('侧栏按 props 渲染副标题与返回链接（顶栏已移除，logo/返回链接均在侧栏）', () => {
    const w = mountShell({ backLinkTo: '/ai-assistant', backLinkText: '返回 AI 助手' })
    expect(w.find('.logo-sub').text()).toBe('AI 叉车助手 · 测试')
    expect(w.find('.back-link').text()).toContain('返回 AI 助手')
  })
})

// #1104：失败/中断走 store.lastTurnError 独立通道——正文不再拼「[生成失败：…]」，
// 壳在输入框上方渲染可重试入口，重试原样重发该轮输入与差异参数（store.retryLastTurn）。
describe('ChatPageShell 当轮失败/中断与重试入口（#1104）', () => {
  it('无失败通道时不渲染重试入口', () => {
    const w = mountShell({})
    expect(w.find('.turn-error').exists()).toBe(false)
  })

  it('生成失败：渲染原因文案 + 点击重试调用 store.retryLastTurn', async () => {
    mocks.store.lastTurnError = { kind: 'error', message: '上游超时', content: '再问一次', opts: {} }
    const w = mountShell({})
    const banner = w.find('.turn-error')
    expect(banner.exists()).toBe(true)
    expect(banner.text()).toContain('生成失败：上游超时')

    const retry = banner.findAll('button').find(b => b.text().includes('重试'))
    expect(retry).toBeTruthy()
    await retry!.trigger('click')
    expect(mocks.store.retryLastTurn).toHaveBeenCalledTimes(1)
  })

  it('用户中断：文案只描述中断事实，同样给重试入口', async () => {
    mocks.store.lastTurnError = { kind: 'aborted', message: '', content: '中断的问题', opts: {} }
    const w = mountShell({})
    const banner = w.find('.turn-error')
    expect(banner.text()).toContain('已中断生成')
    expect(banner.findAll('button').some(b => b.text().includes('重试'))).toBe(true)
  })

  it('重试被前置校验拒绝：把可执行原因弹给用户，不静默', async () => {
    mocks.store.lastTurnError = { kind: 'error', message: '上游超时', content: '再问一次', opts: {} }
    mocks.store.retryLastTurn = vi.fn().mockRejectedValue(new Error('通用对话暂不可用；可改用「智能维修诊断」'))
    const w = mountShell({})
    const retry = w.find('.turn-error').findAll('button').find(b => b.text().includes('重试'))
    await retry!.trigger('click')
    await flushPromises()
    expect(ElMessage.warning).toHaveBeenCalled()
  })
})

describe('ChatPageShell 当轮计费脚注（#620）', () => {
  it('store.lastUsage 存在时渲染当轮脚注；消息正文不含计费文本（usage 与正文分离）', async () => {
    mocks.store = makeStore({
      lastUsage: { points_cost: 5, total_tokens: 1500, balance: 95 },
      messages: [{ id: 2, role: 'assistant', content: '回答内容' }]
    })
    const w = mountShell({})
    const footnote = w.find('.usage-footnote')
    expect(footnote.exists()).toBe(true)
    expect(footnote.text()).toContain('本轮消耗 5 分')
    expect(footnote.text()).toContain('1.5k tokens')
    expect(footnote.text()).toContain('余额 95')
    // 助手正文只有对话内容
    const body = w.findAll('.message-text').map(d => d.text()).join('')
    expect(body).not.toContain('本轮消耗')
  })

  it('无 lastUsage（未发送/已切会话）不渲染脚注', () => {
    const w = mountShell({})
    expect(w.find('.usage-footnote').exists()).toBe(false)
  })
})

describe('ChatPageShell 登出（#620）', () => {
  it('退出登录走 store.clearMessages + loadSessions action，不再直改消息数组', async () => {
    const w = mountShell({})
    // 侧栏底部的用户菜单（首个 ElDropdown 是主题切换的，需定位到 footer 内）
    const userMenu = w.find('.sidebar-footer').findComponent({ name: 'ElDropdown' })
    expect(userMenu.exists()).toBe(true)
    userMenu.vm.$emit('command', 'logout')
    await flushPromises()

    expect(mocks.store.clearMessages).toHaveBeenCalledTimes(1)
    expect(mocks.store.loadSessions).toHaveBeenCalledTimes(1)
  })
})
