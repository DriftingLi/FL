// AI 助手 store 单测（#620 / spec #603）：隐藏通道显性化——
// 1) usage 进独立 lastUsage state，消息正文不再拼接计费脚注；
// 2) 流结束（done）后精确重拉一次会话列表（5 秒定时器已删除，次数断言）；
// 3) clearMessages action：登出等场景走 action，组件不再直改内部数组。
// streamChat 以 vi.mock 捕获 handlers，用例内手动驱动 SSE 事件序。
// 全文件 fake timers：flushPromises 依赖 setTimeout，这里统一用
// advanceTimersByTimeAsync(0) 冲刷微任务（推进 10s 亦可验证定时器确已删除）。
import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import { createPinia, setActivePinia } from 'pinia'

// 登录态可切换的 auth store 桩（plain 值快照：切换后需重建 pinia 使 store 重新读取）
const authState = vi.hoisted(() => ({ isLoggedIn: true }))

vi.mock('@/stores/auth', () => ({
  useAuthStore: () => ({ isLoggedIn: authState.isLoggedIn, userInfo: { role: 'hrwai_user' } })
}))

const streamChatMock = vi.hoisted(() => vi.fn())

vi.mock('@/api/aiAssistant', () => ({
  aiAssistantApi: {
    listAssistantModes: vi.fn().mockResolvedValue({ normal: null, expert: null }),
    listSessions: vi.fn().mockResolvedValue([]),
    createSession: vi.fn().mockResolvedValue({
      id: 11,
      title: '新会话',
      model_name: 'deepseek',
      created_at: '2026-01-01T00:00:00Z',
      updated_at: '2026-01-01T00:00:00Z'
    }),
    getSessionMessages: vi.fn().mockResolvedValue([]),
    deleteSession: vi.fn().mockResolvedValue(null),
    renameSession: vi.fn().mockResolvedValue(null),
    streamChat: streamChatMock
  }
}))

import { aiAssistantApi } from '@/api/aiAssistant'
import { useAIAssistantStore } from '@/stores/aiAssistant'

type Handlers = {
  onChunk?: (c: string) => void
  onUsage?: (d: { points_cost: number; total_tokens: number; balance: number }) => void
  onDone?: () => void
  onError?: (m: string) => void
}

/** streamChat 最近一次调用的 handlers（用例内手动驱动 SSE 事件序） */
function lastHandlers(): Handlers {
  return streamChatMock.mock.calls[streamChatMock.mock.calls.length - 1][1] as Handlers
}

/** 冲刷微任务（fake timers 下的 flushPromises 替代） */
async function flush() {
  await vi.advanceTimersByTimeAsync(0)
}

const USAGE = { points_cost: 5, total_tokens: 1500, balance: 95 }

/** 建 store 并绑定 normal 模式模型（sendMessage 前置校验要求） */
function createStore() {
  const store = useAIAssistantStore()
  store.modeModels = {
    normal: { id: 1, name: '默认模型', model: 'deepseek-chat', base_url: 'https://api.test' },
    expert: null
  }
  store.selectedMode = 'normal'
  return store
}

beforeEach(() => {
  vi.clearAllMocks()
  vi.useFakeTimers()
  // streamChat 返回带 spy 的 controller（clearMessages 中止路径断言用）
  streamChatMock.mockImplementation(() => ({ abort: vi.fn() }))
  setActivePinia(createPinia())
})

afterEach(() => {
  vi.useRealTimers()
})

async function startSend(store: ReturnType<typeof useAIAssistantStore>, content = '你好') {
  const p = store.sendMessage(content)
  // 等 sendMessage 完成（streamChat 已同步调用、返回 controller），再取 handlers 驱动事件
  await p
  await flush()
  return { p, h: lastHandlers() }
}

/** 完整走完一轮：chunk → usage → done */
async function sendMessageAndStream(store: ReturnType<typeof useAIAssistantStore>, content = '你好') {
  const { p, h } = await startSend(store, content)
  h.onChunk?.('回答内容')
  h.onUsage?.(USAGE)
  h.onDone?.()
  await p
  await flush()
}

describe('usage 独立通道（#620）', () => {
  it('usage 事件写入 store.lastUsage；当轮由壳渲染，正文不再拼接计费脚注', async () => {
    const store = createStore()
    await sendMessageAndStream(store)

    expect(store.lastUsage).toEqual(USAGE)
    const assistant = store.messages.find(m => m.role === 'assistant')
    expect(assistant?.content).toBe('回答内容')
    expect(assistant?.content).not.toContain('本轮消耗')
  })

  it('新一轮发送即清上一轮 usage（脚注只描述当轮）', async () => {
    const store = createStore()
    await sendMessageAndStream(store, '第一轮')
    expect(store.lastUsage).toEqual(USAGE)

    // 第二轮流式开始（未收到 usage 前 lastUsage 已复位）
    const { p, h } = await startSend(store, '第二轮')
    expect(store.lastUsage).toBeNull()
    h.onChunk?.('二')
    h.onDone?.()
    await p
    await flush()
    // 第二轮无 usage 事件 → lastUsage 保持 null，正文完整
    expect(store.lastUsage).toBeNull()
    const contents = store.messages.filter(m => m.role === 'assistant').map(m => m.content)
    expect(contents).toEqual(['回答内容', '二'])
  })

  it('切换会话即离开当轮上下文：lastUsage 复位', async () => {
    const store = createStore()
    await sendMessageAndStream(store)
    expect(store.lastUsage).toEqual(USAGE)

    await store.selectSession(11)
    await flush()
    expect(store.lastUsage).toBeNull()
  })

  it('停止流式即复位 lastUsage（中断轮不残留脚注）', async () => {
    const store = createStore()
    await sendMessageAndStream(store)
    expect(store.lastUsage).toEqual(USAGE)

    store.stopStreaming()
    expect(store.lastUsage).toBeNull()
  })
})

describe('done 后精确重拉一次会话列表（#620）', () => {
  it('done → loadSessions 恰好一次；推进 10s 亦不触发（5 秒定时器已删除）', async () => {
    const store = createStore()
    await sendMessageAndStream(store)

    expect(aiAssistantApi.listSessions).toHaveBeenCalledTimes(1)
    await vi.advanceTimersByTimeAsync(10_000)
    expect(aiAssistantApi.listSessions).toHaveBeenCalledTimes(1)
  })

  it('重拉次数只由 done 决定：标题为「新会话」也只拉一次', async () => {
    const store = createStore()
    // createSession 桩返回「新会话」标题（旧实现会因此再排 5s 定时器）
    await sendMessageAndStream(store, '标题验证')
    await vi.advanceTimersByTimeAsync(10_000)
    expect(aiAssistantApi.listSessions).toHaveBeenCalledTimes(1)
  })

  it('未登录：done 后不重拉（临时对话无会话列表）', async () => {
    authState.isLoggedIn = false
    setActivePinia(createPinia()) // 重建 store，按当前登录态快照
    const store = createStore()
    await sendMessageAndStream(store, '游客问')

    expect(aiAssistantApi.listSessions).not.toHaveBeenCalled()
    // 正文照常落消息（游客对话不落库但本地可见）
    expect(store.messages.some(m => m.role === 'assistant' && m.content === '回答内容')).toBe(true)
  })
})

describe('会话链路（T6：序号守卫/删除对账/选中抛错/init 全清）', () => {
  it('删除成功：本地过滤 + 后台重拉对账（listSessions 被触发）', async () => {
    authState.isLoggedIn = true
    setActivePinia(createPinia()) // 按登录态重建 store（前一个用例可能切到未登录）
    const store = createStore()
    vi.mocked(aiAssistantApi.listSessions).mockResolvedValueOnce([
      { id: 2, title: '二', model_name: '', created_at: '', updated_at: '' }
    ])
    await store.deleteSession(1)
    await flush()
    expect(aiAssistantApi.deleteSession).toHaveBeenCalledWith(1)
    expect(aiAssistantApi.listSessions).toHaveBeenCalled()
    expect(store.sessions.map(s => s.id)).toEqual([2])
  })

  it('选中失败抛错并回滚选中态/消息体/当轮态（壳弹提示用）', async () => {
    authState.isLoggedIn = true
    setActivePinia(createPinia()) // 按登录态重建 store
    const store = createStore()
    store.currentSessionId = 3
    const before = [{ id: 1, role: 'user', content: '旧正文', created_at: '' }] as any
    store.messages = before
    store.lastUsage = { ...USAGE }
    store.lastSources = [{ id: 7, text: '旧来源', metadata: {} }] as any
    vi.mocked(aiAssistantApi.getSessionMessages).mockRejectedValueOnce(new Error('网络异常'))
    await expect(store.selectSession(9)).rejects.toThrow('网络异常')
    expect(store.messages).toEqual(before)
    expect(store.currentSessionId).toBe(3)
    expect(store.lastUsage).toEqual(USAGE)
    expect(store.lastSources).toEqual([{ id: 7, text: '旧来源', metadata: {} }])
  })

  it('init 切回主界面全清：消息/选中/当轮态复位并重拉主界面列表', async () => {
    const store = createStore()
    await sendMessageAndStream(store)
    expect(store.messages.length).toBeGreaterThan(0)
    vi.clearAllMocks() // sendMessage 内的懒创建/重拉计数清零，只看 init 行为
    await store.init()
    await flush()
    expect(store.messages).toEqual([])
    expect(store.currentSessionId).toBeNull()
    expect(store.lastUsage).toBeNull()
    expect(aiAssistantApi.listAssistantModes).toHaveBeenCalled()
    expect(aiAssistantApi.listSessions).toHaveBeenCalled()
  })
})

describe('clearMessages action（#620）', () => {
  it('清空消息/会话上下文/当轮 usage 与流式状态', async () => {
    const store = createStore()
    await sendMessageAndStream(store)
    expect(store.messages.length).toBeGreaterThan(0)

    store.clearMessages()

    expect(store.messages).toEqual([])
    expect(store.currentSessionId).toBeNull()
    expect(store.lastUsage).toBeNull()
    expect(store.streaming).toBe(false)
    expect(store.streamingContent).toBe('')
  })

  it('流式进行中清空：中止当轮流（abort 被调用）并整体复位', async () => {
    const store = createStore()
    const { p } = await startSend(store, '流式中清空')
    // sendMessage 同步置 streaming 后才调用 streamChat，此处流式进行中
    const controller = streamChatMock.mock.results[0].value as { abort: ReturnType<typeof vi.fn> }
    expect(store.streaming).toBe(true)

    store.clearMessages()
    await p
    await flush()

    expect(controller.abort).toHaveBeenCalledTimes(1)
    expect(store.messages).toEqual([])
    expect(store.streaming).toBe(false)
    expect(store.lastUsage).toBeNull()
  })

  it('done 时正文为空不落空消息（中止/清空路径不产生空泡）', async () => {
    const store = createStore()
    const { p, h } = await startSend(store, '空正文')
    h.onDone?.()
    await p
    await flush()

    expect(store.messages.filter(m => m.role === 'assistant')).toEqual([])
  })
})
