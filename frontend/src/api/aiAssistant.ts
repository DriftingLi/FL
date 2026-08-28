// AI 助手模块 API 客户端（共享 client 工厂实例化：成功码 200、401 清登录态不跳转）
// 路径前缀：/api/ai-assistant/*
// 认证：统一 HRWAI 账号体系，token 走 utils/storage.ts 单点
// SSE 流式对话使用 fetch + ReadableStream 消费，不通过 axios
import { createHttpClient, createDefaultUnauthorizedPolicy, getValidAccessToken } from './client'
import { getToken, removeToken, removeUserInfo } from '@/utils/storage'

const API_BASE_URL = (import.meta.env.VITE_API_BASE_URL || '/api').replace(/\/api$/, '') + '/api/ai-assistant'

const client = createHttpClient({
  baseURL: API_BASE_URL,
  successCodes: [200],
  // 统一 401 策略（client.ts 单点）：仅清本地 HRWAI 登录态，不跳转（AI 助手支持未登录临时对话）
  onUnauthorized: createDefaultUnauthorizedPolicy({
    clearAuth: () => {
      removeToken()
      removeUserInfo()
    },
    redirect: false
  })
})

// ===== 类型定义 =====

export interface AdminModelOption {
  id: number
  name: string
  model: string
  base_url: string
}

export interface UserModelDTO {
  id: number
  name: string
  api_key: string // 脱敏后的 key
  base_url: string
  model: string
  created_at: string
  updated_at: string
}

export interface SaveUserModelReq {
  id?: number
  name: string
  api_key: string
  base_url: string
  model: string
}

export interface ChatSession {
  id: number
  title: string
  model_name: string
  feature_key?: string
  created_at: string
  updated_at: string
}

export interface ChatMessage {
  id: number
  role: 'user' | 'assistant' | 'system'
  content: string
  images?: string[]
  created_at: string
}

export type ModelSource = 'admin' | 'user' | 'custom'
export type AIMode = 'normal' | 'expert'

export interface AIAssistantModeModels {
  normal: AdminModelOption | null
  expert: AdminModelOption | null
}

export interface StreamChatReq {
  session_id?: number
  // 专项功能键（fault_consult 等，管理端单绑定模型）
  feature_key?: string
  // 新双模式（推荐）：normal | expert，隐藏底层模型
  mode?: AIMode
  model_source: ModelSource
  config_id?: number
  user_model_id?: number
  custom_api_key?: string
  custom_base_url?: string
  custom_model?: string
  messages: Array<{ role: 'user' | 'assistant'; content: string; images?: string[] }>
}

// ===== API 方法 =====

export const aiAssistantApi = {
  /** GET /api/ai-assistant/modes — 公开，返回普通/专家分别绑定的模型（新） */
  listAssistantModes() {
    return client.get<AIAssistantModeModels>('/modes')
  },

  /** GET /api/ai-assistant/models — 公开，列出管理员配置的可用模型（兼容旧） */
  listAdminModels() {
    return client.get<AdminModelOption[]>('/models')
  },

  /** GET /api/ai-assistant/user-models — 需登录，列出当前用户的自定义模型 */
  listUserModels() {
    return client.get<UserModelDTO[]>('/user-models')
  },

  /** POST /api/ai-assistant/user-models — 创建/更新用户自定义模型 */
  saveUserModel(data: SaveUserModelReq) {
    return client.post('/user-models', data)
  },

  /** DELETE /api/ai-assistant/user-models/:id — 删除用户自定义模型 */
  deleteUserModel(id: number) {
    return client.delete(`/user-models/${id}`)
  },

  /** GET /api/ai-assistant/sessions — 需登录，列出当前用户的会话（feature_key 过滤） */
  listSessions(featureKey?: string) {
    return client.get<ChatSession[]>('/sessions', {
      params: featureKey ? { feature_key: featureKey } : undefined
    })
  },

  /** POST /api/ai-assistant/sessions — 需登录，创建会话 */
  createSession(data: { title?: string; model_name?: string; feature_key?: string }) {
    return client.post<ChatSession>('/sessions', data)
  },

  /** POST /api/ai-assistant/upload-image — 可选登录，上传对话图片 */
  async uploadImage(file: File): Promise<string> {
    const formData = new FormData()
    formData.append('file', file)
    // fetch 不经 client 拦截器，无 401→自动刷新；发起前换取新鲜 token（过期则静默刷新）
    const headers: Record<string, string> = {}
    const token = await getValidAccessToken()
    if (token) {
      headers.Authorization = `Bearer ${token}`
    }
    const resp = await fetch(API_BASE_URL + '/upload-image', {
      method: 'POST',
      headers,
      body: formData
    })
    if (!resp.ok) {
      let message = `上传失败（HTTP ${resp.status}）`
      try {
        const body = await resp.json()
        if (body?.message) message = body.message
      } catch {
        // 非 JSON 响应，保留默认消息
      }
      throw new Error(message)
    }
    const body = await resp.json()
    const url = body?.data?.url
    if (!url) {
      throw new Error('上传返回数据异常')
    }
    return url as string
  },

  /** DELETE /api/ai-assistant/sessions/:id — 需登录，删除会话 */
  deleteSession(id: number) {
    return client.delete(`/sessions/${id}`)
  },

  /** PATCH /api/ai-assistant/sessions/:id/title — 需登录，重命名会话 */
  renameSession(id: number, title: string) {
    return client.patch(`/sessions/${id}/title`, { title })
  },

  /** GET /api/ai-assistant/sessions/:id/messages — 需登录，获取会话消息 */
  getSessionMessages(id: number) {
    return client.get<ChatMessage[]>(`/sessions/${id}/messages`)
  },

  /**
   * POST /api/ai-assistant/chat — 流式对话（SSE）
   * 使用 fetch + ReadableStream 消费 text/event-stream
   * onChunk 回调在每个 message 事件时被调用
   * onDone 在收到 done 事件时调用
   * onError 在收到 error 事件或网络异常时调用
   * 返回一个 AbortController，调用 .abort() 可中断生成
   */
  streamChat(
    req: StreamChatReq,
    handlers: {
      onChunk?: (content: string) => void
      onDone?: () => void
      onError?: (message: string) => void
    }
  ): AbortController {
    const controller = new AbortController()
    const token = getToken()

    const headers: Record<string, string> = {
      'Content-Type': 'application/json',
      Accept: 'text/event-stream'
    }
    if (token) {
      headers.Authorization = `Bearer ${token}`
    }

    const url = API_BASE_URL + '/chat'
    fetch(url, {
      method: 'POST',
      headers,
      body: JSON.stringify(req),
      signal: controller.signal
    })
      .then(async (resp) => {
        if (!resp.ok) {
          const text = await resp.text().catch(() => '')
          throw new Error(text || `HTTP ${resp.status}`)
        }
        const reader = resp.body?.getReader()
        if (!reader) throw new Error('无法读取响应流')
        const decoder = new TextDecoder('utf-8')
        let buffer = ''
        try {
          while (true) {
            const { value, done } = await reader.read()
            if (done) break
            buffer += decoder.decode(value, { stream: true })
            // SSE 事件以 \n\n 分隔
            let idx: number
            while ((idx = buffer.indexOf('\n\n')) >= 0) {
              const rawEvent = buffer.slice(0, idx)
              buffer = buffer.slice(idx + 2)
              const evt = parseSSEEvent(rawEvent)
              if (!evt) continue
              if (evt.event === 'message') {
                const content = (evt.data as { content?: string })?.content || ''
                if (content) handlers.onChunk?.(content)
              } else if (evt.event === 'error') {
                const msg = (evt.data as { message?: string })?.message || '生成失败'
                handlers.onError?.(msg)
                return
              } else if (evt.event === 'done') {
                handlers.onDone?.()
                return
              }
            }
          }
          // 流结束但未收到 done 事件，视为完成
          handlers.onDone?.()
        } finally {
          // 释放 reader，避免浏览器报 ERR_ABORTED
          reader.cancel().catch(() => {})
        }
      })
      .catch((err: Error) => {
        if (err.name === 'AbortError') {
          // 用户主动中断，不视为错误
          handlers.onDone?.()
          return
        }
        handlers.onError?.(err.message || '网络异常')
      })

    return controller
  }
}

// 解析单条 SSE 事件块
function parseSSEEvent(raw: string): { event: string; data: any } | null {
  const lines = raw.split('\n')
  let event = 'message'
  const dataLines: string[] = []
  for (const line of lines) {
    if (line.startsWith('event:')) {
      event = line.slice(6).trim()
    } else if (line.startsWith('data:')) {
      dataLines.push(line.slice(5).trim())
    }
  }
  if (dataLines.length === 0) return { event, data: null }
  const dataStr = dataLines.join('\n')
  try {
    return { event, data: JSON.parse(dataStr) }
  } catch {
    return { event, data: dataStr }
  }
}

export default client
