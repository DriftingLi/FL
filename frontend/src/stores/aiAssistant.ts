// AI 助手模块 Pinia store（双模式：普通/专家，隐藏底层模型）
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
  // ===== 双模式模型（普通/专家） =====
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
      sessions.value = await aiAssistantApi.listSessions()
    } catch {
      // 错误已由拦截器提示
    } finally {
      sessionsLoading.value = false
    }
  }

  async function createSession(title?: string, modelName?: string) {
    if (!isLoggedIn.value) return null
    const session = await aiAssistantApi.createSession({ title, model_name: modelName })
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
  async function sendMessage(content: string) {
    if (!content.trim() || streaming.value) return
    // 校验模式可用性
    const modeAvailable = selectedMode.value === 'normal' ? !!modeModels.value.normal : !!modeModels.value.expert
    if (!modeAvailable) throw new Error('当前模式未绑定模型，请联系管理员配置')

    const historyMessages = messages.value
      .filter(m => m.role === 'user' || m.role === 'assistant')
      .map(m => ({ role: m.role as 'user' | 'assistant', content: m.content }))
    historyMessages.push({ role: 'user', content })

    const userMsg: ChatMessage = {
      id: Date.now(),
      role: 'user',
      content,
      created_at: new Date().toISOString()
    }
    messages.value.push(userMsg)

    streaming.value = true
    streamingContent.value = ''
    const assistantMsgId = Date.now() + 1

    const req: any = {
      session_id: isLoggedIn.value ? currentSessionId.value ?? undefined : undefined,
      mode: selectedMode.value,
      // 兼容旧后端：同时带 model_source/config_id
      model_source: 'admin',
      config_id: (selectedMode.value === 'expert' ? modeModels.value.expert?.id : modeModels.value.normal?.id) ?? undefined,
      messages: historyMessages
    }

    abortController = aiAssistantApi.streamChat(req, {
      onChunk: (chunk) => {
        streamingContent.value += chunk
      },
      onDone: () => {
        const assistantMsg: ChatMessage = {
          id: assistantMsgId,
          role: 'assistant',
          content: streamingContent.value,
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

  // ===== 初始化 =====
  async function init() {
    await loadAssistantModes()
    if (isLoggedIn.value) {
      await loadSessions()
    }
  }

  return {
    // state
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
    loadAssistantModes,
    selectMode,
    loadSessions,
    createSession,
    deleteSession,
    renameSession,
    selectSession,
    sendMessage,
    stopStreaming
  }
})
