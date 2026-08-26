// AI 助手模块 Pinia store
// 管理：当前选中的模型、会话列表、当前会话消息、用户自定义模型
// 认证：复用主体系 useAuthStore（HRWAI 账号），未登录时仅支持临时对话
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
import { useAuthStore } from '@/stores/auth'

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
  // ===== 功能上下文 =====
  // 'ai_assistant'=通用 AI 助手（用户选模型）；其余为专项功能（管理端单绑定模型）
  const featureKey: Ref<string> = ref('ai_assistant')
  const isFeatureMode = computed(() => featureKey.value !== 'ai_assistant')

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

  // ===== 登录状态（复用主体系 auth store）=====
  const authStore = useAuthStore()
  const isLoggedIn = computed(() => authStore.isLoggedIn)

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
          label: `${first.model}`
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
      sessions.value = await aiAssistantApi.listSessions(
        isFeatureMode.value ? featureKey.value : undefined
      )
    } catch (e) {
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

  // 重命名会话：调用 API 后立即更新本地列表
  async function renameSession(id: number, title: string) {
    const trimmed = title.trim()
    if (!trimmed) {
      throw new Error('标题不能为空')
    }
    await aiAssistantApi.renameSession(id, trimmed)
    const target = sessions.value.find(s => s.id === id)
    if (target) {
      target.title = trimmed
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
  async function sendMessage(content: string, images?: string[]) {
    if ((!content.trim() && !(images && images.length > 0)) || streaming.value) return
    // 专项功能模式：模型由后端按功能绑定解析；通用模式需用户先选模型
    const model = selectedModel.value
    if (!isFeatureMode.value && !model) {
      throw new Error('请先选择模型')
    }

    const reqImages = images && images.length > 0 ? images : undefined
    // 拼装消息历史（仅传当前会话已有消息 + 新消息）；历史消息仅文本（后端只取末条图片）
    const historyMessages: Array<{ role: 'user' | 'assistant'; content: string; images?: string[] }> =
      messages.value
        .filter(m => m.role === 'user' || m.role === 'assistant')
        .map(m => ({ role: m.role as 'user' | 'assistant', content: m.content }))
    historyMessages.push({ role: 'user', content, images: reqImages })

    // 立即添加用户消息到列表
    const userMsg: ChatMessage = {
      id: Date.now(),
      role: 'user',
      content,
      images: reqImages,
      created_at: new Date().toISOString()
    }
    messages.value.push(userMsg)

    // 开始流式生成
    streaming.value = true
    streamingContent.value = ''
    const assistantMsgId = Date.now() + 1

    const req = {
      session_id: isLoggedIn.value ? currentSessionId.value ?? undefined : undefined,
      feature_key: isFeatureMode.value ? featureKey.value : undefined,
      model_source: isFeatureMode.value ? 'admin' : model!.source, // 专项功能后端忽略此字段
      config_id: isFeatureMode.value ? undefined : model!.configId,
      user_model_id: isFeatureMode.value ? undefined : model!.userModelId,
      custom_api_key: isFeatureMode.value ? undefined : model!.customApiKey,
      custom_base_url: isFeatureMode.value ? undefined : model!.customBaseUrl,
      custom_model: isFeatureMode.value ? undefined : model!.customModel,
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
        // 刷新会话列表（updated_at 变化 + 后端异步命名可能已生成）
        if (isLoggedIn.value) {
          loadSessions().catch(() => {})
          // 后端异步生成标题需要调用模型，延迟 5 秒再刷新一次
          // 仅当当前会话标题仍是占位符时才执行，避免无谓请求
          const curSessionId = currentSessionId.value
          const cur = sessions.value.find(s => s.id === curSessionId)
          if (cur && (cur.title === '新会话' || cur.title === '')) {
            setTimeout(() => {
              loadSessions().catch(() => {})
            }, 5000)
          }
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
      label: `${m.model}`
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

  // ===== 图片上传（多模态对话）=====
  async function uploadImage(file: File): Promise<string> {
    return aiAssistantApi.uploadImage(file)
  }

  // ===== 初始化 =====
  async function init() {
    // 通用 AI 助手页：从功能页返回时重置功能上下文
    featureKey.value = 'ai_assistant'
    await loadAdminModels()
    if (isLoggedIn.value) {
      await Promise.all([loadUserModels(), loadSessions()])
    }
  }

  // ===== 专项功能初始化 =====
  // 切换功能上下文：重置会话/消息后按功能加载会话（模型由后端绑定解析，无需加载模型列表）
  async function initFeature(key: string) {
    if (featureKey.value === key && sessions.value.length > 0) return
    featureKey.value = key
    selectedModel.value = null
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
    initFeature,
    loadAdminModels,
    loadUserModels,
    saveUserModel,
    deleteUserModel,
    loadSessions,
    createSession,
    deleteSession,
    renameSession,
    selectSession,
    sendMessage,
    stopStreaming,
    uploadImage,
    selectAdminModel,
    selectUserModel,
    selectCustomModel
  }
})
