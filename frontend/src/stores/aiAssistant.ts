// AI 助手模块 Pinia store（双模式：普通/专家，隐藏底层模型；专项功能：管理端单绑定模型）
import { defineStore } from 'pinia'
import { ref, computed } from 'vue'
import type { Ref } from 'vue'
import {
  aiAssistantApi,
  type ChatSession,
  type ChatMessage,
  type AIMode,
  type AIAssistantModeModels
} from '@/api/aiAssistant'
import { useAuthStore } from '@/stores/auth'

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

  // ===== 流式对话状态 =====
  const streaming = ref(false)
  const streamingContent = ref('')
  let abortController: AbortController | null = null

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

  // ===== 会话管理 =====
  async function loadSessions() {
    if (!isLoggedIn.value) {
      sessions.value = []
      return
    }
    sessionsLoading.value = true
    try {
      sessions.value = await aiAssistantApi.listSessions(
        isFeatureMode.value ? featureKey.value : undefined
      )
    } catch {
      // 错误已由拦截器提示
    } finally {
      sessionsLoading.value = false
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
    sessions.value = sessions.value.filter(s => s.id !== id)
    if (currentSessionId.value === id) {
      currentSessionId.value = null
      messages.value = []
    }
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
   * 真正的会话在首次 sendMessage 时才创建（见 sendMessage 懒创建分支）。
   */
  function startDraft() {
    if (streaming.value) return
    currentSessionId.value = null
    messages.value = []
    streamingContent.value = ''
  }

  async function selectSession(id: number) {
    if (!isLoggedIn.value) return
    currentSessionId.value = id
    messagesLoading.value = true
    try {
      messages.value = await aiAssistantApi.getSessionMessages(id)
    } catch {
      messages.value = []
    } finally {
      messagesLoading.value = false
    }
  }

  // ===== 流式对话 =====
  async function sendMessage(content: string, images?: string[]) {
    if ((!content.trim() && !(images && images.length > 0)) || streaming.value) return
    // 专项功能模式：模型由后端按功能绑定解析；通用模式校验双模式可用性
    if (!isFeatureMode.value) {
      const modeAvailable = selectedMode.value === 'normal' ? !!modeModels.value.normal : !!modeModels.value.expert
      if (!modeAvailable) throw new Error('当前模式未绑定模型，请联系管理员配置')
    }

    // 懒创建会话：后端仅在 session_id > 0 时持久化消息，因此「开启新对话」只切本地草稿态，
    // 真正落库的时机推迟到用户发出第一条消息（避免侧栏出现没说过话的空会话）
    if (isLoggedIn.value && !currentSessionId.value) {
      const session = await createSession()
      if (!session) return
    }

    const reqImages = images && images.length > 0 ? images : undefined
    // 拼装消息历史（仅传当前会话已有消息 + 新消息）；历史消息仅文本（后端只取末条图片）
    const historyMessages: Array<{ role: 'user' | 'assistant'; content: string; images?: string[] }> =
      messages.value
        .filter(m => m.role === 'user' || m.role === 'assistant')
        .map(m => ({ role: m.role as 'user' | 'assistant', content: m.content }))
    historyMessages.push({ role: 'user', content, images: reqImages })

    const userMsg: ChatMessage = {
      id: Date.now(),
      role: 'user',
      content,
      images: reqImages,
      created_at: new Date().toISOString()
    }
    messages.value.push(userMsg)

    streaming.value = true
    streamingContent.value = ''
    const assistantMsgId = Date.now() + 1

    const req: any = {
      session_id: isLoggedIn.value ? currentSessionId.value ?? undefined : undefined,
      // 专项功能：feature_key（后端按绑定解析模型）；通用助手：mode + 兼容 config_id
      feature_key: isFeatureMode.value ? featureKey.value : undefined,
      mode: isFeatureMode.value ? undefined : selectedMode.value,
      model_source: 'admin',
      config_id: isFeatureMode.value
        ? undefined
        : (selectedMode.value === 'expert' ? modeModels.value.expert?.id : modeModels.value.normal?.id) ?? undefined,
      messages: historyMessages
    }

    let lastUsage: { points_cost: number; total_tokens: number; balance: number } | null = null
    abortController = aiAssistantApi.streamChat(req, {
      onChunk: (chunk) => {
        streamingContent.value += chunk
      },
      onUsage: (data) => {
        lastUsage = data
      },
      onDone: () => {
        let finalContent = streamingContent.value
        if (lastUsage) {
          finalContent += `\n\n— 本轮消耗 ${lastUsage.points_cost} 分 · ${(lastUsage.total_tokens / 1000).toFixed(1)}k tokens · 余额 ${lastUsage.balance}`
        }
        const assistantMsg: ChatMessage = {
          id: assistantMsgId,
          role: 'assistant',
          content: finalContent,
          created_at: new Date().toISOString()
        }
        messages.value.push(assistantMsg)
        streamingContent.value = ''
        streaming.value = false
        abortController = null
        if (isLoggedIn.value) {
          loadSessions().catch(() => {})
          const curSessionId = currentSessionId.value
          const cur = sessions.value.find(s => s.id === curSessionId)
          if (cur && (cur.title === '新会话' || cur.title === '')) {
            setTimeout(() => loadSessions().catch(() => {}), 5000)
          }
        }
      },
      onError: (message) => {
        if (streamingContent.value) {
          const assistantMsg: ChatMessage = {
            id: assistantMsgId,
            role: 'assistant',
            content: streamingContent.value + '\n\n[生成中断：' + message + ']',
            created_at: new Date().toISOString()
          }
          messages.value.push(assistantMsg)
        } else {
          const errorMsg: ChatMessage = {
            id: assistantMsgId,
            role: 'assistant',
            content: '[生成失败：' + message + ']',
            created_at: new Date().toISOString()
          }
          messages.value.push(errorMsg)
        }
        streamingContent.value = ''
        streaming.value = false
        abortController = null
      }
    })
  }

  function stopStreaming() {
    if (abortController) {
      abortController.abort()
      abortController = null
    }
    streaming.value = false
    if (streamingContent.value) {
      const assistantMsg: ChatMessage = {
        id: Date.now(),
        role: 'assistant',
        content: streamingContent.value + '\n\n[已中断]',
        created_at: new Date().toISOString()
      }
      messages.value.push(assistantMsg)
      streamingContent.value = ''
    }
  }

  // ===== 图片上传（多模态对话）=====
  async function uploadImage(file: File): Promise<string> {
    return aiAssistantApi.uploadImage(file)
  }

  // ===== 初始化（通用 AI 助手页） =====
  async function init() {
    // 从功能页返回时重置功能上下文
    featureKey.value = 'ai_assistant'
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
    sessions.value = []
    messages.value = []
    currentSessionId.value = null
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
    selectSession,
    sendMessage,
    stopStreaming,
    uploadImage
  }
})
