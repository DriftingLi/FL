// AI 助手 store 单测（#620 / spec #603）：隐藏通道显性化——
// 1) usage 进独立 lastUsage state，消息正文不再拼接计费脚注；
// 2) 流结束（done）后精确重拉一次会话列表（5 秒定时器已删除，次数断言）；
// 3) clearMessages action：登出等场景走 action，组件不再直改内部数组。
//
// #1104：一轮发送的编排（send）与三种终态（done / error / aborted）收进 store，
// 失败/中断进 lastTurnError 独立通道（含原样重发载荷）——正文里不再出现「[生成失败：…]」。
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
  onSources?: (s: Array<Record<string, unknown>>) => void
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

/** 建 store 并绑定 normal 模式模型（send 的前置校验要求） */
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
  // streamChat 返回带 spy 的 controller（中断/复位路径断言用）
  streamChatMock.mockImplementation(() => ({ abort: vi.fn() }))
  setActivePinia(createPinia())
})

afterEach(() => {
  vi.useRealTimers()
})

/** 起一轮发送并拿到该轮 handlers（send 同步起流后返回，故 await 完即可驱动事件） */
async function startSend(store: ReturnType<typeof useAIAssistantStore>, content = '你好') {
  const p = store.send(content)
  await p
  await flush()
  return { p, h: lastHandlers() }
}

/** streamChat 第 n 次调用返回的 controller（中断路径断言 abort 用） */
function controllerAt(index = 0) {
  return streamChatMock.mock.results[index].value as { abort: ReturnType<typeof vi.fn> }
}

/** 完整走完一轮：chunk → usage → done */
async function sendAndStream(store: ReturnType<typeof useAIAssistantStore>, content = '你好') {
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
    await sendAndStream(store)

    expect(store.lastUsage).toEqual(USAGE)
    const assistant = store.messages.find(m => m.role === 'assistant')
    expect(assistant?.content).toBe('回答内容')
    expect(assistant?.content).not.toContain('本轮消耗')
  })

  it('新一轮发送即清上一轮 usage（脚注只描述当轮）', async () => {
    const store = createStore()
    await sendAndStream(store, '第一轮')
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
    await sendAndStream(store)
    expect(store.lastUsage).toEqual(USAGE)

    await store.selectSession(11)
    await flush()
    expect(store.lastUsage).toBeNull()
  })

  it('中断在飞轮次即复位 lastUsage（中断轮不残留脚注）', async () => {
    const store = createStore()
    const { h } = await startSend(store, '中断轮')
    h.onUsage?.(USAGE)
    expect(store.streaming).toBe(true)
    expect(store.lastUsage).toEqual(USAGE)

    store.stopStreaming()
    expect(store.lastUsage).toBeNull()
  })
})

// #1104：一轮发送的编排与终态收进 store —— 三种终态各自只经 finalizeTurn 一处收尾
// （落助手消息 / 复位当轮态与 abortController / 进失败通道），页面不再各写一份。
describe('当轮终态（#1104：done / error / aborted 一处收尾）', () => {
  it('done：正文与当轮 sources 快照落消息，失败通道为空，当轮态与闸门复位', async () => {
    const store = createStore()
    const { p, h } = await startSend(store, '正常收流')
    expect(store.canSend).toBe(false)
    h.onChunk?.('回答')
    h.onSources?.([{ id: 9, text: '来源', metadata: {} }])
    h.onDone?.()
    await p
    await flush()

    expect(store.messages.filter(m => m.role === 'assistant')).toEqual([
      expect.objectContaining({
        content: '回答',
        sources: [{ id: 9, text: '来源', metadata: {} }]
      })
    ])
    expect(store.lastTurnError).toBeNull()
    expect(store.streaming).toBe(false)
    expect(store.streamingContent).toBe('')
    expect(store.canSend).toBe(true)
  })

  it('error：部分正文照落且不拼失败后缀；原因与该轮输入进 lastTurnError（可重发）', async () => {
    const store = createStore()
    const { p, h } = await startSend(store, '失败轮的问题')
    h.onChunk?.('已经生成的一半')
    h.onError?.('上游超时')
    await p
    await flush()

    const assistant = store.messages.filter(m => m.role === 'assistant')
    expect(assistant).toHaveLength(1)
    // 状态不再拼进正文（旧实现是 '已经生成的一半\n\n[生成中断：上游超时]'）
    expect(assistant[0].content).toBe('已经生成的一半')
    expect(assistant[0].content).not.toContain('生成中断')
    expect(store.lastTurnError).toMatchObject({
      kind: 'error',
      message: '上游超时',
      content: '失败轮的问题'
    })
    expect(store.streaming).toBe(false)
    expect(store.streamingContent).toBe('')
  })

  it('error 且正文为空：不再落「[生成失败：…]」占位消息，失败原因只走独立通道', async () => {
    const store = createStore()
    const { p, h } = await startSend(store, '空正文失败')
    h.onError?.('网络异常')
    await p
    await flush()

    expect(store.messages.filter(m => m.role === 'assistant')).toEqual([])
    expect(store.lastTurnError).toMatchObject({ kind: 'error', message: '网络异常' })
  })

  it('aborted：部分正文照落（不带「[已中断]」后缀），中断进独立通道，abort 被释放', async () => {
    const store = createStore()
    const { p, h } = await startSend(store, '中断我')
    h.onChunk?.('中断前生成的内容')
    store.stopStreaming()

    expect(controllerAt().abort).toHaveBeenCalledTimes(1)
    const assistant = store.messages.filter(m => m.role === 'assistant')
    expect(assistant).toHaveLength(1)
    expect(assistant[0].content).toBe('中断前生成的内容')
    expect(assistant[0].content).not.toContain('已中断')
    expect(store.lastTurnError).toMatchObject({ kind: 'aborted', content: '中断我' })
    expect(store.streaming).toBe(false)
    expect(store.streamingContent).toBe('')

    // abort 后 api 层把 AbortError 归到 onDone：迟到回调被轮次守卫丢弃，不二次收尾
    h.onDone?.()
    await p
    await flush()
    expect(store.messages.filter(m => m.role === 'assistant')).toHaveLength(1)
    expect(store.lastTurnError).toMatchObject({ kind: 'aborted' })
  })

  it('收尾后闸门复位：第二轮可立即发起（abortController 与在飞标记都已释放）', async () => {
    const store = createStore()
    const { p, h } = await startSend(store, '第一轮')
    h.onError?.('失败')
    await p
    await flush()
    expect(store.canSend).toBe(true)

    await startSend(store, '第二轮')
    expect(streamChatMock).toHaveBeenCalledTimes(2)
  })

  it('真闸门：在飞轮次未收尾时不接受第二轮（canSend=false，流不重复起）', async () => {
    const store = createStore()
    const { p } = await startSend(store, '第一轮')
    expect(store.canSend).toBe(false)
    await store.send('第二轮')
    expect(streamChatMock).toHaveBeenCalledTimes(1)
    await p
  })

  it('重试：原样重发该轮内容与差异参数，失败通道在新轮开始时清空', async () => {
    const store = createStore()
    const { p, h } = await startSend(store, '重试我')
    h.onError?.('上游超时')
    await p
    await flush()

    await store.retryLastTurn()
    await flush()
    expect(streamChatMock).toHaveBeenCalledTimes(2)
    const retryReq = streamChatMock.mock.calls[1][0] as { messages: Array<Record<string, unknown>> }
    expect(retryReq.messages[retryReq.messages.length - 1]).toEqual({
      role: 'user',
      content: '重试我',
      images: undefined
    })
    expect(store.lastTurnError).toBeNull()
    expect(store.streaming).toBe(true)
  })

  it('重试带图片/结构化过滤：差异参数随内容一起重发', async () => {
    const store = createStore()
    const p = store.send('带图重试', { images: ['/uploads/a.png'], brand: '合力', model: 'H2000' })
    await p
    await flush()
    lastHandlers().onError?.('挂了')
    await p
    await flush()

    await store.retryLastTurn()
    await flush()
    const retryReq = streamChatMock.mock.calls[1][0] as Record<string, unknown>
    expect(retryReq.brand).toBe('合力')
    expect(retryReq.model).toBe('H2000')
    const msgs = retryReq.messages as Array<Record<string, unknown>>
    expect(msgs[msgs.length - 1]).toEqual({
      role: 'user',
      content: '带图重试',
      images: ['/uploads/a.png']
    })
  })

  it('会话创建失败：这一轮没发出去，原因进独立通道（可重试），用户内容不丢', async () => {
    const store = createStore()
    vi.mocked(aiAssistantApi.createSession).mockRejectedValueOnce(new Error('网络开小差'))
    await store.send('创建会话失败的一轮')
    await flush()

    expect(store.streaming).toBe(false)
    expect(store.lastTurnError).toMatchObject({
      kind: 'error',
      message: '网络开小差',
      content: '创建会话失败的一轮'
    })
    // 没有起流，也没有落用户消息（这一轮从未发出）
    expect(streamChatMock).not.toHaveBeenCalled()
  })
})

describe('新对话草稿态（#1104：流式中显式拒绝，不静默吞）', () => {
  it('流式进行中 startDraft 返回 false 且不改任何状态', async () => {
    const store = createStore()
    const { p } = await startSend(store, '流式中')
    const before = [...store.messages]

    expect(store.startDraft()).toBe(false)
    expect(store.messages).toEqual(before)
    expect(store.streaming).toBe(true)
    await p
  })

  it('空闲时 startDraft 返回 true：消息/当轮脚注/失败通道一并复位', async () => {
    const store = createStore()
    const { p, h } = await startSend(store, '失败轮')
    h.onUsage?.(USAGE)
    h.onError?.('挂了')
    await p
    await flush()
    expect(store.lastTurnError).not.toBeNull()

    expect(store.startDraft()).toBe(true)
    expect(store.messages).toEqual([])
    expect(store.lastUsage).toBeNull()
    expect(store.lastTurnError).toBeNull()
  })
})

describe('done 后精确重拉一次会话列表（#620）', () => {
  it('done → loadSessions 恰好一次；推进 10s 亦不触发（5 秒定时器已删除）', async () => {
    const store = createStore()
    await sendAndStream(store)

    expect(aiAssistantApi.listSessions).toHaveBeenCalledTimes(1)
    await vi.advanceTimersByTimeAsync(10_000)
    expect(aiAssistantApi.listSessions).toHaveBeenCalledTimes(1)
  })

  it('重拉次数只由 done 决定：标题为「新会话」也只拉一次', async () => {
    const store = createStore()
    // createSession 桩返回「新会话」标题（旧实现会因此再排 5s 定时器）
    await sendAndStream(store, '标题验证')
    await vi.advanceTimersByTimeAsync(10_000)
    expect(aiAssistantApi.listSessions).toHaveBeenCalledTimes(1)
  })

  it('未登录：done 后不重拉（临时对话无会话列表）', async () => {
    authState.isLoggedIn = false
    setActivePinia(createPinia()) // 重建 store，按当前登录态快照
    const store = createStore()
    await sendAndStream(store, '游客问')

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
      { id: 2, title: '二', model_name: '', feature_key: '', created_at: '', updated_at: '' }
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
    store.lastTurnError = { kind: 'error', message: '上一轮挂了', content: '旧问题', opts: {} }
    vi.mocked(aiAssistantApi.getSessionMessages).mockRejectedValueOnce(new Error('网络异常'))
    await expect(store.selectSession(9)).rejects.toThrow('网络异常')
    expect(store.messages).toEqual(before)
    expect(store.currentSessionId).toBe(3)
    expect(store.lastUsage).toEqual(USAGE)
    expect(store.lastTurnError).toMatchObject({ kind: 'error', message: '上一轮挂了' })
  })

  it('init 切回主界面全清：消息/选中/当轮态复位并重拉主界面列表', async () => {
    const store = createStore()
    await sendAndStream(store)
    expect(store.messages.length).toBeGreaterThan(0)
    vi.clearAllMocks() // send 内的懒创建/重拉计数清零，只看 init 行为
    await store.init()
    await flush()
    expect(store.messages).toEqual([])
    expect(store.currentSessionId).toBeNull()
    expect(store.lastUsage).toBeNull()
    expect(aiAssistantApi.listAssistantModes).toHaveBeenCalled()
    expect(aiAssistantApi.listSessions).toHaveBeenCalled()
  })

  it('切上下文时在飞轮次一并收尾：不再往新上下文追加内容（孤立流）', async () => {
    const store = createStore()
    const { h } = await startSend(store, '在飞轮次')
    h.onChunk?.('旧上下文的一半')

    await store.init()
    await flush()

    expect(store.streaming).toBe(false)
    expect(store.streamingContent).toBe('')
    expect(store.messages).toEqual([])
    // 迟到事件（旧流仍在飞）不得写进新上下文
    h.onChunk?.('迟到的增量')
    h.onDone?.()
    expect(store.streamingContent).toBe('')
    expect(store.messages).toEqual([])
  })
})

describe('clearMessages action（#620）', () => {
  it('清空消息/会话上下文/当轮 usage 与流式状态', async () => {
    const store = createStore()
    await sendAndStream(store)
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
    // send 同步置 streaming 后才调用 streamChat，此处流式进行中
    expect(store.streaming).toBe(true)

    store.clearMessages()
    await p
    await flush()

    expect(controllerAt().abort).toHaveBeenCalledTimes(1)
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

// #1061：双模式均未绑定时，通用对话守卫给出的必须是一条**可据以行动**的文案。
// 背景（#1044）：旧文案「当前模式未绑定模型，请联系管理员配置」把学员逼到死路——
// 基础版学员在 Web 端没有任何自助配置入口，提示说了等于没说。
describe('双模式未绑定时的空态引导（#1061）', () => {
  it('双模式均未绑定：抛出的文案说明状态，并指向页内仍可用的专项功能', async () => {
    const store = useAIAssistantStore()
    store.modeModels = { normal: null, expert: null }
    store.selectedMode = 'normal'

    const err = await store.send('叉车液压压力不足怎么查？').then(
      () => null,
      (e: Error) => e
    )
    expect(err).toBeInstanceOf(Error)
    const msg = (err as Error).message
    // 可执行性判据：① 说清「哪条通道不可用」② 点到学员当下真能用的入口
    // （「智能维修诊断」与页内功能胶囊同名，是线上确认可用的那条）
    expect(msg).toContain('通用对话暂不可用')
    expect(msg).toContain('智能维修诊断')
  })

  it('只绑一条模式即放行：守卫只在双模式都空时拦截（不误伤可用模式）', async () => {
    const store = createStore() // normal 已绑、expert 为空
    const { p } = await startSend(store, '普通模式可用')
    await expect(p).resolves.toBeUndefined()
  })
})
