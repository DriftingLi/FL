// AI 助手模块 Pinia store
// 管理：当前选中的模型、会话列表、当前会话消息、用户自定义模型
// 认证：复用 valuation_auth（HRWAI 账号），未登录时仅支持临时对话
import { defineStore } from 'pinia'
import { ref, computed } from 'vue'
import type { Ref } from 'vue'
import {
  aiAssistantApi,
  type AdminModelOption,
  type UserModelDTO,
  type ChatSession,
  type ChatMessage,
  type ModelSource,
  type SaveUserModelReq
} from '@/api/aiAssistant'
import { useValuationAuthStore } from '@/stores/valuationAuth'

export interface SelectedModel {
  source: ModelSource
  // source=admin: configId
  // source=user: userModelId
  // source=custom: 临时输入的 api_key/base_url/model
  configId?: number
  userModelId?: number
  customApiKey?: string
  customBaseUrl?: string
  customModel?: string
  // 显示用：模型名称
  label: string
}

export const useAIAssistantStore = defineStore('aiAssistant', () => {
  // ===== 模型列表 =====
  const adminModels: Ref<AdminModelOption[]> = ref([])
  const userModels: Ref<UserModelDTO[]> = ref([])
  const modelsLoading = ref(false)

  // ===== 当前选中的模型 =====
  const selectedModel: Ref<SelectedModel | null> = ref(null)

  // ===== 会话管理 =====
  const sessions: Ref<ChatSession[]> = ref([])
  const currentSessionId: Ref<number | null> = ref(null)
  const messages: Ref<ChatMessage[]> = ref([])
  const sessionsLoading = ref(false)
  const messagesLoading = ref(false)

  // ===== 流式对话状态 =====
  const streaming = ref(false)
  const streamingContent = ref('') // 正在生成的内容
  let abortController: AbortController | null = null

  // ===== 登录状态（复用 valuation auth）=====
  const valuationAuth = useValuationAuthStore()
  const isLoggedIn = computed(() => valuationAuth.isLoggedIn)

  // ===== 模型加载 =====
  async function loadAdminModels() {
    modelsLoading.value = true
    try {
      adminModels.value = await aiAssistantApi.listAdminModels()
      // 自动选中第一个管理员模型（仅当未选中时）
      if (!selectedModel.value && adminModels.value.length > 0) {
        const first = adminModels.value[0]
        selectedModel.value = {
          source: 'admin',
          configId: first.id,
          label: `${first.name} · ${first.model}`
        }
      }
    } catch (e) {
      // 错误已由拦截器提示
    } finally {
      modelsLoading.value = false
    }
  }

  async function loadUserModels() {
    if (!isLoggedIn.value) return
    try {
      userModels.value = await aiAssistantApi.listUserModels()
    } catch (e) {
      // 错误已由拦截器提示
    }
  }

  async function saveUserModel(data: SaveUserModelReq) {
    await aiAssistantApi.saveUserModel(data)
    await loadUserModels()
  }

  async function deleteUserModel(id: number) {
    await aiAssistantApi.deleteUserModel(id)
    // 若删除的是当前选中的用户模型，回退到第一个管理员模型
    if (selectedModel.value?.source === 'user' && selectedModel.value.userModelId === id) {
      selectedModel.value = null
      await loadAdminModels()
    }
    await loadUserModels()
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
    } catch (e) {
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

  async function selectSession(id: number) {
    if (!isLoggedIn.value) return
    currentSessionId.value = id
    messagesLoading.value = true
    try {
      messages.value = await aiAssistantApi.getSessionMessages(id)
    } catch (e) {
      messages.value = []
    } finally {
      messagesLoading.value = false
    }
  }

  // ===== 流式对话 =====
  async function sendMessage(content: string) {
    if (!content.trim() || streaming.value) return
    if (!selectedModel.value) {
      throw new Error('请先选择模型')
    }

    const model = selectedModel.value
    // 拼装消息历史（仅传当前会话已有消息 + 新消息）
    const historyMessages = messages.value
      .filter(m => m.role === 'user' || m.role === 'assistant')
      .map(m => ({ role: m.role as 'user' | 'assistant', content: m.content }))
    historyMessages.push({ role: 'user', content })

    // 立即添加用户消息到列表
    const userMsg: ChatMessage = {
      id: Date.now(),
      role: 'user',
      content,
      created_at: new Date().toISOString()
    }
    messages.value.push(userMsg)

    // 开始流式生成
    streaming.value = true
    streamingContent.value = ''
    const assistantMsgId = Date.now() + 1

    const req = {
      session_id: isLoggedIn.value ? currentSessionId.value ?? undefined : undefined,
      model_source: model.source,
      config_id: model.configId,
      user_model_id: model.userModelId,
      custom_api_key: model.customApiKey,
      custom_base_url: model.customBaseUrl,
      custom_model: model.customModel,
      messages: historyMessages
    }

    abortController = aiAssistantApi.streamChat(req, {
      onChunk: (chunk) => {
        streamingContent.value += chunk
      },
      onDone: () => {
        // 将流式内容固化为助手消息
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
        // 刷新会话列表（updated_at 变化）
        if (isLoggedIn.value) {
          loadSessions().catch(() => {})
        }
      },
      onError: (message) => {
        // 如果已有部分内容，保存为助手消息
        if (streamingContent.value) {
          const assistantMsg: ChatMessage = {
            id: assistantMsgId,
            role: 'assistant',
            content: streamingContent.value + '\n\n[生成中断：' + message + ']',
            created_at: new Date().toISOString()
          }
          messages.value.push(assistantMsg)
        } else {
          // 没有内容，仅显示错误
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
    // 保留已生成的内容
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

  // ===== 模型选择 =====
  function selectAdminModel(configId: number) {
    const m = adminModels.value.find(x => x.id === configId)
    if (!m) return
    selectedModel.value = {
      source: 'admin',
      configId: m.id,
      label: `${m.name} · ${m.model}`
    }
  }

  function selectUserModel(userModelId: number) {
    const m = userModels.value.find(x => x.id === userModelId)
    if (!m) return
    selectedModel.value = {
      source: 'user',
      userModelId: m.id,
      label: `${m.name} · ${m.model}`
    }
  }

  function selectCustomModel(apiKey: string, baseUrl: string, modelName: string) {
    selectedModel.value = {
      source: 'custom',
      customApiKey: apiKey,
      customBaseUrl: baseUrl,
      customModel: modelName,
      label: `自定义 · ${modelName}`
    }
  }

  // ===== 初始化 =====
  async function init() {
    await loadAdminModels()
    if (isLoggedIn.value) {
      await Promise.all([loadUserModels(), loadSessions()])
    }
  }

  return {
    // state
    adminModels,
    userModels,
    modelsLoading,
    selectedModel,
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
    loadAdminModels,
    loadUserModels,
    saveUserModel,
    deleteUserModel,
    loadSessions,
    createSession,
    deleteSession,
    selectSession,
    sendMessage,
    stopStreaming,
    selectAdminModel,
    selectUserModel,
    selectCustomModel
  }
})
