// AI 助手模块 API 客户端（共享 client 工厂实例化：成功码 200、401 清登录态不跳转）
// 路径前缀：/api/ai-assistant/*
// 认证：统一 HRWAI 账号体系，token 走 utils/storage.ts 单点
// SSE 流式对话使用 fetch + ReadableStream 消费，不通过 axios
//
// 响应类型**不再手写**：唯一事实源是后端注解 → backend/docs/swagger.json →
// `cd backend && go run ./cmd/gen-apitypes`（ADR-0048 决策 1/3，issue #966 片八）。
// 本文件只留请求壳、端点装配与名称适配；入参（query / body）类型不生成、仍手写。
//
// **边界（ADR-0048 决策 6）**：SSE 流式对话 `POST /ai-assistant/chat` 与其事件 payload
// （message / sources / usage / error / done）**不走统一信封、不在生成面** —— 它是裸
// fetch + ReadableStream 自解析（见 streamChat），事件类型手写并逐条注明「非生成面」；
// 唯一例外是 sources 事件复用生成类型 DiagnosisSource（与历史回放同一形状，不另立事实源）。
// 走信封的裸 fetch（upload-image，读 body.data.url）已接上生成类型。
import { createHttpClient, createDefaultUnauthorizedPolicy, getValidAccessToken } from './client'
import { getToken, removeToken, removeUserInfo } from '@/utils/storage'
import type {
  AIAssistantModeModels,
  AIChatMessageDTO,
  AIChatSessionDTO,
  AIImageUploadResultDTO,
  AISessionRenameResultDTO,
  DiagnosisBrandOption,
  DiagnosisFaultCodeItem,
  DiagnosisFaultCodePage,
  DiagnosisSource,
  ModelOption,
  UserModelDTO
} from './generated/aiAssistant'

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

// ===== 类型 re-export（响应形状唯一事实源 = 生成物）=====
export type {
  AIAssistantModeModels,
  DiagnosisBrandOption,
  DiagnosisFaultCodeItem,
  DiagnosisFaultCodePage,
  DiagnosisSource,
  ModelOption,
  UserModelDTO
}

// 旧名 → 生成名（形状一致，保留既有 import 路径与名字不破）：
/** 管理员配置的可用模型（= 生成物 ModelOption，字段逐一一致） */
export type AdminModelOption = ModelOption
/** 会话（= 生成物 AIChatSessionDTO；feature_key 在注解层是**必有键**，旧手写的 `?` 已过时） */
export type ChatSession = AIChatSessionDTO
/** 会话消息（= 生成物 AIChatMessageDTO；images/sources 在注解层是「键在、值可 null」） */
export type ChatMessage = AIChatMessageDTO

// ===== 入参类型（不生成，ADR-0048 决策 3）=====

export interface SaveUserModelReq {
  id?: number
  name: string
  api_key: string
  base_url: string
  model: string
}

/** 模型来源词表（入参，不生成） */
export type ModelSource = 'admin' | 'user' | 'custom'
/** 双模式词表（入参 + store 选中态，不生成；注解层尚无枚举词汇，见 ADR-0048 片一） */
export type AIMode = 'normal' | 'expert'

/** SSE 流式对话请求（入参，不生成） */
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
  // 智能维修诊断（fault_diagnosis）专用：品牌/车型过滤（结构化传参，不走文本注入）
  brand?: string
  model?: string
  messages: Array<{ role: 'user' | 'assistant'; content: string; images?: string[] }>
}

/** SSE usage 事件 payload（**非生成面（不走信封）**，ADR-0048 决策 6） */
export interface StreamUsageEvent {
  points_cost: number
  total_tokens: number
  balance: number
  prompt_tokens?: number
  completion_tokens?: number
}

// ===== API 方法 =====

export const aiAssistantApi = {
  /** GET /api/ai-assistant/modes — 公开，返回普通/专家分别绑定的模型（新） */
  listAssistantModes() {
    return client.get<AIAssistantModeModels>('/modes')
  },

  /** GET /api/ai-assistant/models — 公开，列出管理员配置的可用模型（兼容旧） */
  listAdminModels() {
    return client.get<ModelOption[]>('/models')
  },

  /** GET /api/ai-assistant/user-models — 需登录，列出当前用户的自定义模型 */
  listUserModels() {
    return client.get<UserModelDTO[]>('/user-models')
  },

  /** POST /api/ai-assistant/user-models — 创建/更新用户自定义模型（data 为 null） */
  saveUserModel(data: SaveUserModelReq) {
    return client.post<null>('/user-models', data)
  },

  /** DELETE /api/ai-assistant/user-models/:id — 删除用户自定义模型（data 为 null） */
  deleteUserModel(id: number) {
    return client.delete<null>(`/user-models/${id}`)
  },

  /** GET /api/ai-assistant/sessions — 需登录，列出当前用户的会话（feature_key 过滤） */
  listSessions(featureKey?: string) {
    return client.get<AIChatSessionDTO[]>('/sessions', {
      params: featureKey ? { feature_key: featureKey } : undefined
    })
  },

  /** POST /api/ai-assistant/sessions — 需登录，创建会话 */
  createSession(data: { title?: string; model_name?: string; feature_key?: string }) {
    return client.post<AIChatSessionDTO>('/sessions', data)
  },

  /** POST /api/ai-assistant/upload-image — 可选登录，上传对话图片
   * 裸 fetch（不经 client 拦截器，无 401→自动刷新；发起前换取新鲜 token）；
   * 但**走统一信封**：响应形状来自生成物 AIImageUploadResultDTO（data.url）。 */
  async uploadImage(file: File): Promise<string> {
    const formData = new FormData()
    formData.append('file', file)
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
        // 错误分支是统一信封的 message（raw fetch 拿不到拦截器解包，此处显式声明信封）
        const body = (await resp.json()) as { message?: string }
        if (body?.message) message = body.message
      } catch {
        // 非 JSON 响应，保留默认消息
      }
      throw new Error(message)
    }
    const body = (await resp.json()) as { data?: AIImageUploadResultDTO }
    const url = body?.data?.url
    if (!url) {
      throw new Error('上传返回数据异常')
    }
    return url
  },

  /** DELETE /api/ai-assistant/sessions/:id — 需登录，删除会话（data 为 null） */
  deleteSession(id: number) {
    return client.delete<null>(`/sessions/${id}`)
  },

  /** PATCH /api/ai-assistant/sessions/:id/title — 需登录，重命名会话 */
  renameSession(id: number, title: string) {
    return client.patch<AISessionRenameResultDTO>(`/sessions/${id}/title`, { title })
  },

  /** GET /api/ai-assistant/sessions/:id/messages — 需登录，获取会话消息 */
  getSessionMessages(id: number) {
    return client.get<AIChatMessageDTO[]>(`/sessions/${id}/messages`)
  },

  /** GET /api/ai-assistant/diagnosis/brands — 智能维修诊断品牌列表 */
  listDiagnosisBrands() {
    return client.get<DiagnosisBrandOption[]>('/diagnosis/brands')
  },

  /** GET /api/ai-assistant/diagnosis/models — 某品牌车型列表 */
  listDiagnosisModels(brand?: string) {
    return client.get<string[]>('/diagnosis/models', { params: brand ? { brand } : undefined })
  },

  /** GET /api/ai-assistant/diagnosis/fault-codes — 故障码分页查询 */
  listDiagnosisFaultCodes(params: { brand?: string; keyword?: string; page?: number; page_size?: number }) {
    return client.get<DiagnosisFaultCodePage>('/diagnosis/fault-codes', { params })
  },

  /** GET /api/ai-assistant/diagnosis/manual/* — 手册静态资源代理 URL（溯源图片；可选认证直接 <img>）。
   * 该端点是**原样字节流（非统一信封）**，故只拼 URL、不经 client 解包。 */
  manualUrl(subpath: string): string {
    const segs = subpath.split('/').map(encodeURIComponent).join('/')
    return `${API_BASE_URL}/diagnosis/manual/${segs}`
  },

  /**
   * POST /api/ai-assistant/chat — 流式对话（SSE）
   * **非生成面（不走信封）**：ADR-0048 决策 6 明确 SSE 事件 payload 不在契约 codegen 范围；
   * 本方法与其 handlers 类型全部手写，不得从 generated/aiAssistant 取（sources 事件是唯一例外：
   * 它复用历史回放的 DiagnosisSource 形状，故直接引用生成类型）。
   * 使用 fetch + ReadableStream 消费 text/event-stream
   * onChunk 回调在每个 message 事件时被调用
   * onDone 在收到 done 事件时调用
   * onError 在收到 error 事件或网络异常时调用
   * 返回一个 AbortController，调用 .abort() 可中断生成
   */
  streamChat(
    req: StreamChatReq,
    handlers: {
      /** SSE message 事件 payload：{content}（非生成面） */
      onChunk?: (content: string) => void
      /** SSE done 事件（无 payload，非生成面） */
      onDone?: () => void
      /** SSE error 事件 payload：{message} / 网络异常（非生成面） */
      onError?: (message: string) => void
      /** SSE usage 事件 payload（非生成面） */
      onUsage?: (data: StreamUsageEvent) => void
      /** SSE sources 事件 payload：复用生成类型 DiagnosisSource（与历史回放同形状） */
      onSources?: (sources: DiagnosisSource[]) => void
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
              } else if (evt.event === 'usage') {
                handlers.onUsage?.(evt.data as StreamUsageEvent)
              } else if (evt.event === 'sources') {
                const payload = (evt.data as { sources?: DiagnosisSource[] })?.sources || []
                handlers.onSources?.(payload)
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

// 解析单条 SSE 事件块（非生成面：SSE 事件解析单点）
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
