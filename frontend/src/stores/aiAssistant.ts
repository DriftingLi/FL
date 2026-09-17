// AI 助手模块 Pinia store（双模式：普通/专家，隐藏底层模型；专项功能：管理端单绑定模型）
import { defineStore } from 'pinia'
import { ref, computed } from 'vue'
import type { Ref } from 'vue'
import {
  aiAssistantApi,
  type ChatSession,
  type ChatMessage,
  type AIMode,
  type AIAssistantModeModels,
  type DiagnosisSource
} from '@/api/aiAssistant'
import { useAuthStore } from '@/stores/auth'

/** 当轮计费消耗（SSE usage 事件负载；ADR-0023 tokens 后计量） */
export interface TurnUsage {
  points_cost: number
  total_tokens: number
  balance: number
}

/**
 * 一轮发送的差异参数（#1104）：图片与结构化过滤由页面按各自差异组装，
 * 「发送编排与终态」收在本 store，页面只提交正文 + 本对象。
 */
export interface SendOptions {
  /** 多模态对话：已上传成功的图片 URL */
  images?: string[]
  /** 智能维修诊断：品牌（结构化过滤，不进正文） */
  brand?: string
  /** 智能维修诊断：车型（结构化过滤，不进正文） */
  model?: string
}

/**
 * 当轮终态失败（#1104）：失败/中断走独立通道——正文里不再拼「[生成中断：…]」/「[生成失败：…]」，
 * 壳读它渲染可重试入口；`content` / `opts` 是该轮原样重发的载荷。
 */
export interface TurnError {
  /** error = 生成失败（SSE error / 网络异常 / 会话创建失败）；aborted = 用户主动中断 */
  kind: 'error' | 'aborted'
  /** 失败原因（aborted 无原因，空串；error 为后端/网络原文） */
  message: string
  /** 该轮用户输入（重试原样重发） */
  content: string
  /** 该轮的差异参数（图片/结构化过滤，随内容一起重发） */
  opts: SendOptions
}

export const useAIAssistantStore = defineStore('aiAssistant', () => {
  // ===== 功能上下文 =====
  // 'ai_assistant'=通用 AI 助手（双模式 normal/expert）；其余为专项功能（管理端单绑定模型）
  const featureKey: Ref<string> = ref('ai_assistant')
  const isFeatureMode = computed(() => featureKey.value !== 'ai_assistant')

  // ===== 双模式模型（普通/专家，仅通用助手使用） =====
  const modeModels: Ref<AIAssistantModeModels> = ref({ normal: null, expert: null })
  const selectedMode: Ref<AIMode> = ref('normal')
  const modelsLoading = ref(false)

  // ===== 会话管理 =====
  const sessions: Ref<ChatSession[]> = ref([])
  const currentSessionId: Ref<number | null> = ref(null)
  const messages: Ref<ChatMessage[]> = ref([])
  const sessionsLoading = ref(false)
  const messagesLoading = ref(false)

  // ===== 流式对话状态（一轮发送的编排与终态，#1104）=====
  const streaming = ref(false)
  const streamingContent = ref('')
  let abortController: AbortController | null = null
  // 轮次守卫：每轮发送领一个递增序号，`activeTurn` 是当前在飞轮次。收尾后置 null，
  // 迟到回调（abort 后 api 层把 AbortError 归到 onDone、切上下文时在飞的 chunk）据此丢弃，
  // 不再把上一轮的内容/终态追加进下一轮或下一个上下文。
  let turnSeq = 0
  let activeTurn: number | null = null
  // 当前在飞轮次的重发载荷（终态时进 lastTurnError；用户输入文本本身留在页面 UI 态）
  let turnDraft: { content: string; opts: SendOptions } | null = null
  // 当轮助手消息 id（三种终态共用同一条消息位；= 用户消息 id + 1，避免同毫秒撞键）
  let turnMsgId = 0

  /** 发送闸门（唯一表述）：无在飞轮次即可接受新一轮；草稿是否就绪由页面按 UI 态并入（#1104） */
  const canSend = computed(() => !streaming.value)

  /**
   * 当轮失败/中断的独立通道（#1104）：壳据此渲染重试入口，重试原样重发该轮输入与差异参数。
   * 新一轮开始、切会话、清空会话时复位。
   */
  const lastTurnError: Ref<TurnError | null> = ref(null)

  // ===== 当轮计费 usage（#620 显性通道）=====
  // SSE usage 事件进独立 state，由壳组件渲染当轮脚注；消息正文不再拼接计费文本
  // （已知取舍：历史回看不显示每轮费用——后端消息未存 usage，如需回看另立项）。
  const lastUsage: Ref<TurnUsage | null> = ref(null)

  // ===== 智能维修诊断当轮来源（SSE sources 事件；包前端 answer_sources 还原）=====
  // 与 usage 同策略：只描述当轮，历史回看不显示（后端未持久化 sources）。
  // 仅 store 内部消费（done 时快照进消息；页面读的是 message.sources），不对外导出。
  const lastSources: Ref<DiagnosisSource[]> = ref([])

  // ===== 登录状态（复用主体系 auth store）=====
  const authStore = useAuthStore()
  const isLoggedIn = computed(() => authStore.isLoggedIn)

  // ===== 模式加载 =====
  async function loadAssistantModes() {
    modelsLoading.value = true
    try {
      const data = await aiAssistantApi.listAssistantModes()
      modeModels.value = data || { normal: null, expert: null }
      // 若当前选中模式未绑定，自动回退到已绑定的另一模式
      if (selectedMode.value === 'normal' && !modeModels.value.normal && modeModels.value.expert) {
        selectedMode.value = 'expert'
      } else if (selectedMode.value === 'expert' && !modeModels.value.expert && modeModels.value.normal) {
        selectedMode.value = 'normal'
      }
      // 若两者均未绑定，保持 normal（前端将提示联系管理员）
    } catch {
      // 错误已由拦截器提示
    } finally {
      modelsLoading.value = false
    }
  }

  function selectMode(mode: AIMode) {
    if (mode !== 'normal' && mode !== 'expert') return
    // 若目标模式未绑定，不切换并保持原值
    if (mode === 'normal' && !modeModels.value.normal) return
    if (mode === 'expert' && !modeModels.value.expert) return
    selectedMode.value = mode
  }

  // ===== 会话管理（T6：请求序号守卫——旧回包不覆盖删除/新建结果）=====
  let sessionsSeq = 0
  async function loadSessions() {
    if (!isLoggedIn.value) {
      sessions.value = []
      return
    }
    const seq = ++sessionsSeq
    sessionsLoading.value = true
    try {
      const list = await aiAssistantApi.listSessions(
        isFeatureMode.value ? featureKey.value : undefined
      )
      if (seq === sessionsSeq) sessions.value = list
    } catch {
      // 错误已由拦截器提示
    } finally {
      if (seq === sessionsSeq) sessionsLoading.value = false
    }
  }

  async function createSession(title?: string, modelName?: string) {
    if (!isLoggedIn.value) return null
    const session = await aiAssistantApi.createSession({
      title,
      model_name: modelName,
      feature_key: isFeatureMode.value ? featureKey.value : undefined
    })
    sessions.value.unshift(session)
    currentSessionId.value = session.id
    messages.value = []
    return session
  }

  async function deleteSession(id: number) {
    await aiAssistantApi.deleteSession(id)
    sessionsSeq++ // 作废在飞的列表请求，避免旧回包复活已删会话
    sessions.value = sessions.value.filter(s => s.id !== id)
    if (currentSessionId.value === id) {
      currentSessionId.value = null
      messages.value = []
    }
    // 后台重拉对账（序号守卫保证只采纳最新回包；await 让调用方可感知完成）
    await loadSessions().catch(() => {})
  }

  async function renameSession(id: number, title: string) {
    const trimmed = title.trim()
    if (!trimmed) throw new Error('标题不能为空')
    await aiAssistantApi.renameSession(id, trimmed)
    const target = sessions.value.find(s => s.id === id)
    if (target) target.title = trimmed
  }

  /**
   * 开启「新对话草稿态」：只清本地消息并断开当前会话，**不落库**。
   * 后端仅在 session_id > 0 时持久化，因此不预建会话即可避免「点一下就产生空历史」；
   * 真正的会话在首次 `send` 时才创建（见 send 的懒创建分支）。
   *
   * 流式进行中拒绝（返回 false，不改任何状态）——壳把「新对话」入口置灰给出**显式反馈**（#1104），
   * 不再 `if (streaming) return` 静默吞掉点击。
   */
  function startDraft(): boolean {
    if (streaming.value) return false
    // #1122：「离开当轮上下文」的入口统一走 dropInflightTurn —— 这里覆盖仍未置 streaming 的在飞轮次
    // （懒创建会话的 await 窗口：activeTurn 已领号、流还没起）。send 恢复后据 activeTurn 失效自行退出。
    dropInflightTurn()
    currentSessionId.value = null
    messages.value = []
    streamingContent.value = ''
    lastUsage.value = null
    lastSources.value = []
    lastTurnError.value = null
    turnDraft = null
    return true
  }

  /**
   * 丢弃在飞轮次（不落消息、不进失败通道）：切上下文 / 登出等复位场景共用（#1104）。
   * `activeTurn = null` 让迟到回调失效，abort 释放底层 fetch。
   */
  function dropInflightTurn() {
    activeTurn = null
    if (abortController) {
      abortController.abort()
      abortController = null
    }
    streaming.value = false
    streamingContent.value = ''
    turnDraft = null
  }

  /**
   * 清空会话消息（#620）：登出等场景的状态变更走 action，组件不再直改 store 内部数组。
   * 流式进行中先丢弃当轮流（迟到回调由轮次守卫失效），再整体复位。
   */
  function clearMessages() {
    dropInflightTurn()
    messages.value = []
    currentSessionId.value = null
    lastUsage.value = null
    lastSources.value = []
    lastTurnError.value = null
  }

  async function selectSession(id: number) {
    if (!isLoggedIn.value) throw new Error('请先登录后查看会话历史')
    // #1122：切换会话即离开当轮上下文 —— 在飞轮次先丢弃（abort 在飞请求 + 复位当轮态），否则旧流
    // 仍会把 chunk / 终态写进**新会话**上下文（迟到的 onChunk/onDone/onError 由轮次守卫丢弃）。
    dropInflightTurn()
    const previousId = currentSessionId.value
    const previousMessages = messages.value
    const previousUsage = lastUsage.value
    const previousSources = lastSources.value
    const previousTurnError = lastTurnError.value
    currentSessionId.value = id
    // 切换会话即离开「当轮」上下文，上一轮脚注/来源/失败随之失效
    lastUsage.value = null
    lastSources.value = []
    lastTurnError.value = null
    messagesLoading.value = true
    try {
      messages.value = await aiAssistantApi.getSessionMessages(id)
    } catch (e: any) {
      // 失败回滚选中态/消息体/当轮态：侧栏高亮、正文与脚注都回到切换前
      currentSessionId.value = previousId
      messages.value = previousMessages
      lastUsage.value = previousUsage
      lastSources.value = previousSources
      lastTurnError.value = previousTurnError
      throw new Error(e?.message || '加载会话消息失败，请重试')
    } finally {
      messagesLoading.value = false
    }
  }

  // ===== 流式对话：一轮发送的编排与终态（#1104）=====
  /** 当轮终态：done = 正常收流；error = 生成失败；aborted = 用户主动中断 */
  type TurnOutcome =
    | { kind: 'done' }
    | { kind: 'error'; message: string }
    | { kind: 'aborted' }

  /** 助手消息落点（三种终态共用）：sources 为当轮快照（ADR-0033 逐轮回放；失败/中断轮不带来源） */
  function pushAssistantMessage(content: string, sources: DiagnosisSource[] | null) {
    const assistantMsg: ChatMessage = {
      id: turnMsgId,
      role: 'assistant',
      content,
      images: null,
      sources,
      created_at: new Date().toISOString()
    }
    messages.value.push(assistantMsg)
  }

  /**
   * 当轮收尾单点（#1104）：三种终态共用一处「落助手消息 + 复位当轮态 + 释放 abortController」。
   * - done：正文与当轮 sources 快照进消息，清空失败通道，并按需重拉一次会话列表；
   * - error / aborted：已生成的部分正文照落（**不再拼**「[生成中断：…]」/「[已中断]」后缀），
   *   原因与该轮输入进 lastTurnError —— 壳据此渲染可重试入口。
   * 只对当前在飞轮次生效：`activeTurn` 一置 null，abort 后迟到的 onDone/onChunk 即被丢弃。
   */
  function finalizeTurn(turn: number, outcome: TurnOutcome) {
    if (turn !== activeTurn) return
    activeTurn = null
    const content = streamingContent.value
    if (outcome.kind === 'done') {
      if (content) {
        pushAssistantMessage(content, lastSources.value.length ? [...lastSources.value] : null)
      }
      lastTurnError.value = null
    } else {
      if (content) pushAssistantMessage(content, null)
      const draft = turnDraft
      if (draft) {
        lastTurnError.value = {
          kind: outcome.kind,
          message: outcome.kind === 'error' ? outcome.message : '',
          content: draft.content,
          opts: draft.opts
        }
      }
    }
    // 一处复位当轮态：正文缓冲 / 在飞标记 / abort 句柄 / 重发载荷
    streamingContent.value = ''
    streaming.value = false
    abortController = null
    turnDraft = null
    // 中断轮不残留计费脚注（usage 只描述当轮）；done/error 保留，由壳渲染
    if (outcome.kind === 'aborted') lastUsage.value = null
    // 流结束 → 精确重拉一次会话列表（后端 done 前已完成异步命名；标题未就绪时显示默认名，下次进入自然命中）。
    // 5 秒定时器已删除（#620）；失败/中断不重拉（一轮失败不改变列表）。
    if (outcome.kind === 'done' && isLoggedIn.value) {
      loadSessions().catch(() => {})
    }
  }

  /**
   * 发送一轮：编排（前置校验 → 懒创建会话 → 乐观落用户消息 → 起流）与终态都收在这里，
   * 页面只提交「正文 + 差异参数」，不再各写一份 trim / streaming 守卫 / 清空输入 / await / 吞错。
   *
   * 前置校验不静默吞：空内容 / 有在飞轮次时本轮不发起（调用方按 `canSend` 判断并在 UI 上表达），
   * 通用模式模型未绑定时抛出文案**可据以行动**的 Error（#1061），由调用方提示；
   * 会话创建失败与流上的失败一样进 lastTurnError（可重试），不丢用户输入。
   */
  async function send(text: string, opts: SendOptions = {}) {
    const images = opts.images && opts.images.length > 0 ? opts.images : undefined
    if ((!text.trim() && !images) || streaming.value) return
    // 专项功能模式：模型由后端按功能绑定解析；通用模式校验双模式可用性
    if (!isFeatureMode.value) {
      const modeAvailable = selectedMode.value === 'normal' ? !!modeModels.value.normal : !!modeModels.value.expert
      if (!modeAvailable) throw new Error('通用对话暂不可用（平台模型未配置）；可改用「智能维修诊断」等专项功能，或联系管理员配置')
    }

    // ===== 开启当轮：轮次序号 + 重发载荷 + 助手消息位 =====
    const turn = ++turnSeq
    activeTurn = turn
    turnDraft = { content: text, opts: { ...opts, images } }
    turnMsgId = Date.now() + 1
    lastTurnError.value = null

    // 懒创建会话：后端仅在 session_id > 0 时持久化消息，因此「开启新对话」只切本地草稿态，
    // 真正落库的时机推迟到用户发出第一条消息（避免侧栏出现没说过话的空会话）
    if (isLoggedIn.value && !currentSessionId.value) {
      let session: ChatSession | null = null
      try {
        session = await createSession()
      } catch (e: any) {
        // 会话没建起来 = 这一轮没发出去：原因进独立通道（内容可重发），不静默丢
        finalizeTurn(turn, { kind: 'error', message: e?.message || '会话创建失败，请重试' })
        return
      }
      if (!session) {
        finalizeTurn(turn, { kind: 'error', message: '会话创建失败，请重试' })
        return
      }
    }
    // #1122：懒创建的 await 期间当轮可能已被丢弃（切会话 / 切上下文 / 新草稿）—— 此时不得再落消息、
    // 起流：否则终态会被轮次守卫丢弃，streaming 永远停在 true（停止按钮也失效）。
    if (turn !== activeTurn) return

    const reqImages = images
    // 拼装消息历史（仅传当前会话已有消息 + 新消息）；历史消息仅文本（后端只取末条图片）
    const historyMessages: Array<{ role: 'user' | 'assistant'; content: string; images?: string[] }> =
      messages.value
        .filter(m => m.role === 'user' || m.role === 'assistant')
        .map(m => ({ role: m.role as 'user' | 'assistant', content: m.content }))
    historyMessages.push({ role: 'user', content: text, images: reqImages })

    // 线格式（生成类型）里 images/sources 是「键在、值可 null」（ADR-0048 片八）：
    // 本地乐观消息与线消息同形，消费处一律 ?. 判空
    const userMsg: ChatMessage = {
      id: Date.now(),
      role: 'user',
      content: text,
      images: reqImages ?? null,
      sources: null,
      created_at: new Date().toISOString()
    }
    messages.value.push(userMsg)

    streaming.value = true
    streamingContent.value = ''
    // 新一轮开始：清上一轮脚注/来源（usage/sources 仅描述当轮）
    lastUsage.value = null
    lastSources.value = []

    const req: any = {
      session_id: isLoggedIn.value ? currentSessionId.value ?? undefined : undefined,
      // 专项功能：feature_key（后端按绑定解析模型）；通用助手：mode + 兼容 config_id
      feature_key: isFeatureMode.value ? featureKey.value : undefined,
      mode: isFeatureMode.value ? undefined : selectedMode.value,
      model_source: 'admin',
      config_id: isFeatureMode.value
        ? undefined
        : (selectedMode.value === 'expert' ? modeModels.value.expert?.id : modeModels.value.normal?.id) ?? undefined,
      // 智能维修诊断（fault_diagnosis）专用：品牌/车型结构化过滤
      brand: opts.brand,
      model: opts.model,
      messages: historyMessages
    }

    const controller = aiAssistantApi.streamChat(req, {
      // 事件回调只在当轮有效：收尾/换轮后的迟到事件丢弃（轮次守卫单点判定）
      onChunk: (chunk) => {
        if (turn === activeTurn) streamingContent.value += chunk
      },
      // usage 进独立 state（当轮脚注由壳渲染），不再拼进消息正文
      onUsage: (data) => {
        if (turn === activeTurn) lastUsage.value = data
      },
      // 来源资料进独立 state（智能维修诊断；当轮来源面板由本页渲染）
      onSources: (sources) => {
        if (turn === activeTurn) lastSources.value = sources
      },
      onDone: () => finalizeTurn(turn, { kind: 'done' }),
      onError: (message) => finalizeTurn(turn, { kind: 'error', message })
    })
    // 当轮 abort 句柄：收尾处（finalizeTurn）统一释放
    if (turn === activeTurn) abortController = controller
  }

  /**
   * 重试上一轮失败/中断的发送（壳的重试入口）：内容与差异参数原样重发。
   * 被前置校验拒绝时（如通用模式未绑定）异常上抛，lastTurnError 保持原样（不吞掉重试入口）。
   */
  async function retryLastTurn() {
    const failed = lastTurnError.value
    if (!failed || streaming.value) return
    await send(failed.content, failed.opts)
  }

  /** 用户中断当轮（壳的「停止」按钮）：收尾走 finalizeTurn(aborted)，部分正文照落 */
  function stopStreaming() {
    if (!streaming.value) return
    const controller = abortController
    // 先收尾再 abort：api 层把 AbortError 归到 onDone，收尾后到达的回调被轮次守卫丢弃
    finalizeTurn(turnSeq, { kind: 'aborted' })
    controller?.abort()
  }

  // ===== 图片上传（多模态对话）=====
  async function uploadImage(file: File): Promise<string> {
    return aiAssistantApi.uploadImage(file)
  }

  // ===== 功能上下文切换：作废在飞列表请求 + 收尾在飞轮次 + 清空会话/当轮态（init/initFeature 共用）=====
  function resetSessionContext() {
    sessionsSeq++
    // #1104：切上下文时在飞轮次一并收尾（此前不复位 streaming，旧流会继续往新上下文追加内容）
    dropInflightTurn()
    sessions.value = []
    messages.value = []
    currentSessionId.value = null
    streamingContent.value = ''
    lastUsage.value = null
    lastSources.value = []
    lastTurnError.value = null
  }

  // ===== 初始化（通用 AI 助手页）=====
  // 从功能页返回时重置功能上下文（T6：切回主界面全清并重拉，回欢迎态）
  async function init() {
    featureKey.value = 'ai_assistant'
    resetSessionContext() // 断开子界面上下文
    await loadAssistantModes()
    if (isLoggedIn.value) {
      await loadSessions()
    }
  }

  // ===== 专项功能初始化 =====
  // 切换功能上下文：重置会话/消息后按功能加载会话（模型由后端绑定解析，无需加载模式列表）
  async function initFeature(key: string) {
    if (featureKey.value === key && sessions.value.length > 0) return
    featureKey.value = key
    resetSessionContext() // 作废旧功能上下文在飞的列表请求
    if (isLoggedIn.value) {
      await loadSessions()
    }
  }

  return {
    // state
    featureKey,
    isFeatureMode,
    modeModels,
    selectedMode,
    modelsLoading,
    sessions,
    currentSessionId,
    messages,
    sessionsLoading,
    messagesLoading,
    streaming,
    streamingContent,
    canSend,
    lastUsage,
    lastTurnError,
    isLoggedIn,
    // actions
    init,
    initFeature,
    loadAssistantModes,
    selectMode,
    loadSessions,
    createSession,
    deleteSession,
    renameSession,
    startDraft,
    clearMessages,
    selectSession,
    send,
    retryLastTurn,
    stopStreaming,
    uploadImage
  }
})
