// ChatPageShell 壳测试（#398）：槽位渲染、安全渲染单点（markstream escape、无裸 v-html）、
// 侧栏会话操作（重命名/删除/选中）、输入区变体与发送事件、自动滚底钩子不回归。
import { describe, it, expect, vi, beforeEach } from 'vitest'
import { mount } from '@vue/test-utils'
import { reactive, nextTick } from 'vue'
import { createRouter, createMemoryHistory } from 'vue-router'
import ElementPlus from 'element-plus'

import ChatPageShell from '../ChatPageShell.vue'

// ===== 模块替身：store / auth / 路由工具 / markstream =====
const mocks = vi.hoisted(() => ({ store: undefined as any }))

vi.mock('@/stores/aiAssistant', () => ({
  useAIAssistantStore: () => mocks.store
}))
vi.mock('@/stores/auth', () => ({
  useAuthStore: () => ({
    userInfo: { username: 'tester' },
    clearAuthData: vi.fn()
  })
}))
vi.mock('@/api/auth', () => ({
  authApi: { logout: vi.fn().mockResolvedValue(undefined) }
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

function makeStore(overrides: Record<string, unknown> = {}) {
  return reactive({
    isLoggedIn: true,
    sessionsLoading: false,
    sessions: [{ id: 1, title: '会话一', updated_at: '2026-01-01T00:00:00Z' }],
    currentSessionId: 1,
    messages: [] as Array<Record<string, unknown>>,
    streaming: false,
    streamingContent: '',
    selectSession: vi.fn(),
    deleteSession: vi.fn().mockResolvedValue(undefined),
    renameSession: vi.fn().mockResolvedValue(undefined),
    stopStreaming: vi.fn(),
    ...overrides
  })
}

const router = createRouter({
  history: createMemoryHistory(),
  routes: [{ path: '/:pathMatch(.*)*', component: { template: '<div/>' } }]
})

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
      plugins: [ElementPlus, router]
    }
  })
}

beforeEach(() => {
  mocks.store = makeStore()
})

describe('ChatPageShell 槽位', () => {
  it('空状态渲染欢迎区：welcome-top 槽位 → 预设提示词 → welcome-bottom 槽位顺序', async () => {
    const w = mountShell(
      { suggestions: ['问题一', '问题二'] },
      {
        'welcome-top': '<div class="slot-top">快捷选项</div>',
        'welcome-bottom': '<div class="slot-bottom">功能入口</div>'
      }
    )
    const area = w.find('.welcome-area')
    expect(area.exists()).toBe(true)
    const html = area.html()
    expect(html.indexOf('slot-top')).toBeGreaterThan(-1)
    expect(html.indexOf('slot-top')).toBeLessThan(html.indexOf('suggestion-grid'))
    expect(html.indexOf('suggestion-grid')).toBeLessThan(html.indexOf('slot-bottom'))
    // 游客提示仅未登录时渲染
    expect(w.find('.guest-hint').exists()).toBe(false)
  })

  it('有消息或流式中不渲染欢迎区', async () => {
    mocks.store.messages = [{ id: 2, role: 'user', content: 'hi' }]
    const w = mountShell({})
    expect(w.find('.welcome-area').exists()).toBe(false)
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
  it('未登录不渲染侧栏，登录后渲染', async () => {
    mocks.store = makeStore({ isLoggedIn: false })
    const guest = mountShell({})
    expect(guest.find('.session-sidebar').exists()).toBe(false)

    mocks.store = makeStore()
    const logged = mountShell({})
    expect(logged.find('.session-sidebar').exists()).toBe(true)
  })

  it('点击会话触发 selectSession；新建按钮发出 new-session', async () => {
    const w = mountShell({})
    await w.find('.session-item').trigger('click')
    expect(mocks.store.selectSession).toHaveBeenCalledWith(1)
    await w.find('.sidebar-header .el-button').trigger('click')
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

  it('流式中切换为停止按钮并调用 stopStreaming', async () => {
    mocks.store.streaming = true
    const w = mountShell({})
    const stopBtn = w.findAll('button').find(b => b.text().includes('停止'))
    expect(stopBtn).toBeTruthy()
    await stopBtn!.trigger('click')
    expect(mocks.store.stopStreaming).toHaveBeenCalled()
  })

  it('顶部栏按 props 渲染返回链接与副标题', () => {
    const w = mountShell({ backLinkTo: '/ai-assistant', backLinkText: '返回 AI 助手' })
    expect(w.find('.logo-sub').text()).toBe('AI 叉车助手 · 测试')
    const links = w.findAll('.back-link').map(l => l.text())
    expect(links).toContain('返回 AI 助手')
    expect(links).toContain('返回官网')
  })
})
