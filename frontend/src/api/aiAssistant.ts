// AI 助手模块 API 客户端
// 路径前缀：/api/ai-assistant/*
// 认证：统一 HRWAI 账号体系，token 存储于 localStorage 'token'（与主体系 useAuthStore 一致）
// SSE 流式对话使用 fetch + ReadableStream 消费，不通过 axios
import axios, { AxiosError, type AxiosResponse, type InternalAxiosRequestConfig } from 'axios'
import { ElMessage } from 'element-plus'

const API_BASE_URL = (import.meta.env.VITE_API_BASE_URL || '/api').replace(/\/api$/, '') + '/api/ai-assistant'
const REQUEST_TIMEOUT_MS = 30_000
const TOKEN_STORAGE_KEY = 'token'

const client = axios.create({
  baseURL: API_BASE_URL,
  timeout: REQUEST_TIMEOUT_MS,
  headers: { 'Content-Type': 'application/json; charset=utf-8' }
})

// 请求拦截器：附加 HRWAI JWT
client.interceptors.request.use(
  (config: InternalAxiosRequestConfig) => {
    const token = localStorage.getItem(TOKEN_STORAGE_KEY)
    if (token) {
      config.headers.Authorization = `Bearer ${token}`
    }
    return config
  },
  (err) => Promise.reject(err)
)

// 响应拦截器：解包 {code,message,data}，401 清除登录态
client.interceptors.response.use(
  (response: AxiosResponse) => {
    const body = response.data
    if (body && typeof body === 'object' && 'code' in body) {
      if (body.code === 0) {
        return { ...response, data: body.data }
      }
      const errMsg = body.message || `业务错误（code=${body.code}）`
      // 40100 为未登录/Token 失效业务码，不弹 ElMessage（由调用方处理跳转）
      if (body.code !== 40100) {
        ElMessage.error(errMsg)
      }
      return Promise.reject(new Error(errMsg))
    }
    return response
  },
  async (err: AxiosError) => {
    if (err.response) {
      const status = err.response.status
      if (status === 401) {
        // 清除本地 HRWAI 登录态，让 UI 显示未登录状态
        localStorage.removeItem(TOKEN_STORAGE_KEY)
        localStorage.removeItem('userInfo')
        // 不强制跳转登录页，AI 助手支持未登录临时对话
        ElMessage.error('登录已过期，请重新登录')
        return Promise.reject(err)
      }
      const data = err.response.data as { message?: string } | undefined
      const msg = data?.message || `请求失败 (${status})`
      ElMessage.error(msg)
    } else if (err.request) {
      ElMessage.error('网络异常：无法连接服务器')
    } else {
      ElMessage.error(`请求错误：${err.message}`)
    }
    return Promise.reject(err)
  }
)

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
  created_at: string
  updated_at: string
}

export interface ChatMessage {
  id: number
  role: 'user' | 'assistant' | 'system'
  content: string
  created_at: string
}

export type ModelSource = 'admin' | 'user' | 'custom'

export interface StreamChatReq {
  session_id?: number
  model_source: ModelSource
  config_id?: number
  user_model_id?: number
  custom_api_key?: string
  custom_base_url?: string
  custom_model?: string
  messages: Array<{ role: 'user' | 'assistant'; content: string }>
}

// ===== API 方法 =====

export const aiAssistantApi = {
  /** GET /api/ai-assistant/models — 公开，列出管理员配置的可用模型 */
  listAdminModels() {
    return client.get<AdminModelOption[]>('/models').then(r => r.data)
  },

  /** GET /api/ai-assistant/user-models — 需登录，列出当前用户的自定义模型 */
  listUserModels() {
    return client.get<UserModelDTO[]>('/user-models').then(r => r.data)
  },

  /** POST /api/ai-assistant/user-models — 创建/更新用户自定义模型 */
  saveUserModel(data: SaveUserModelReq) {
    return client.post('/user-models', data).then(r => r.data)
  },

  /** DELETE /api/ai-assistant/user-models/:id — 删除用户自定义模型 */
  deleteUserModel(id: number) {
    return client.delete(`/user-models/${id}`).then(r => r.data)
  },

  /** GET /api/ai-assistant/sessions — 需登录，列出当前用户的会话 */
  listSessions() {
    return client.get<ChatSession[]>('/sessions').then(r => r.data)
  },

  /** POST /api/ai-assistant/sessions — 需登录，创建会话 */
  createSession(data: { title?: string; model_name?: string }) {
    return client.post<ChatSession>('/sessions', data).then(r => r.data)
  },

  /** DELETE /api/ai-assistant/sessions/:id — 需登录，删除会话 */
  deleteSession(id: number) {
    return client.delete(`/sessions/${id}`).then(r => r.data)
  },

  /** PATCH /api/ai-assistant/sessions/:id/title — 需登录，重命名会话 */
  renameSession(id: number, title: string) {
    return client.patch(`/sessions/${id}/title`, { title }).then(r => r.data)
  },

  /** GET /api/ai-assistant/sessions/:id/messages — 需登录，获取会话消息 */
  getSessionMessages(id: number) {
    return client.get<ChatMessage[]>(`/sessions/${id}/messages`).then(r => r.data)
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
    const token = localStorage.getItem(TOKEN_STORAGE_KEY)

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
