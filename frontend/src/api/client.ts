// 共享 HTTP 客户端工厂：Bearer 附加、{code,message,data} 解包、401 策略、错误提示单点实现。
// 三个模块（主体系 / 估值 / AI 助手）各自以 createHttpClient 创建实例，只注入差异：
// baseURL、成功码集合与 401 策略。token 读写统一走 utils/storage.ts（单点 key）。
//
// 信封解包（ADR-0005）：拦截器成功（code ∈ 成功码集合）直接返回业务负载 data（Promise<T>），
// 业务失败抛错并统一 toast。responseType 为 blob/arraybuffer 时直接放行二进制数据。
import axios from 'axios'
import type { AxiosError, AxiosRequestConfig, AxiosInstance } from 'axios'
import { ElMessage } from 'element-plus'
import { getToken, getRefreshToken, setToken, setRefreshToken } from '@/utils/storage'
import { useCredentialStore } from '@/stores/credential'

/**
 * 错误的语义分类，供上层决定「能否重试」「渲染哪种错误态」。
 *
 * 只作为字段挂在 reject 出去的 Error 上，**不改变现有 toast 逻辑与 reject 的值本身**，
 * 因此对所有既有调用方完全向后兼容。
 */
export type ApiErrorKind =
  | 'network'
  | 'timeout'
  | 'server'
  | 'auth'
  | 'forbidden'
  | 'notfound'
  | 'business'

function classifyError(err: AxiosError): ApiErrorKind {
  if (
    err.code === 'ECONNABORTED' ||
    /timeout\s+of\s+\d+\s+ms\s+exceeded/i.test(err.message || '')
  ) {
    return 'timeout'
  }
  const status = err.response?.status
  if (status === 401) return 'auth'
  if (status === 403) return 'forbidden'
  if (status === 404) return 'notfound'
  if (status && status >= 500) return 'server'
  if (status && status >= 400) return 'business'
  return 'network'
}

function attachKind(err: unknown): void {
  const e = err as AxiosError & { kind?: ApiErrorKind }
  e.kind = classifyError(e)
}

// ===== 双令牌静默刷新（ADR-0012）：模块级共享的单飞行，三端实例并发去重 =====
// 401 时统一用 refresh token 换新 access + 新 refresh，再重试原请求；
// 刷新失败才走各实例的 onUnauthorized（清登录态 + 跳登录）。
let refreshPromise: Promise<boolean> | null = null

// 刷新专用裸 client（不走本工厂拦截器，避免 401 递归）；路径固定为全局 /api/auth/refresh
const refreshHttp = axios.create({ baseURL: '/api', timeout: 30000 })

function isRefreshEndpoint(url: string): boolean {
  return url.includes('/auth/refresh')
}

/** 静默刷新（单飞行）：并发 401 只发一次刷新请求，成功后更新双令牌 */
function tryRefreshTokens(): Promise<boolean> {
  const rt = getRefreshToken()
  if (!rt) return Promise.resolve(false)
  if (!refreshPromise) {
    refreshPromise = refreshHttp
      .post('/auth/refresh', { refresh_token: rt })
      .then(res => {
        const data = res.data?.data
        if (data?.token && data?.refresh_token) {
          setToken(data.token)
          setRefreshToken(data.refresh_token)
          return true
        }
        return false
      })
      .catch(() => false)
      .finally(() => {
        refreshPromise = null
      })
  }
  return refreshPromise
}

/** 解析 JWT 载荷并判断 access 是否已过期（解析失败视为过期，走刷新） */
function isAccessTokenExpired(token: string): boolean {
  try {
    const payload = token.split('.')[1]
    const json = JSON.parse(atob(payload.replace(/-/g, '+').replace(/_/g, '/')))
    return typeof json.exp !== 'number' || json.exp * 1000 <= Date.now()
  } catch {
    return true
  }
}

/**
 * 获取当前有效的 access token：
 *  本地未过期 → 直接返回；已过期/缺失且存在 refresh → 静默刷新后返回新 token；
 *  刷新失败或未登录 → null。
 * 供绕过 client 拦截器的裸请求（featured 上传 / AI 助手 fetch 等）在发起前
 * 换取新鲜 token，避免 401 后因不经过拦截器而无法自动续期。
 */
export async function getValidAccessToken(): Promise<string | null> {
  const token = getToken()
  if (token && !isAccessTokenExpired(token)) {
    return token
  }
  const refreshed = await tryRefreshTokens()
  return refreshed ? getToken() : null
}

/** 后端通用 JSON 响应格式（code = HTTP 状态码，统一信封，见 ADR-0005） */
export interface ApiResponse<T = any> {
  code: number
  message: string
  data: T
}

/** Raw blob 响应：同时携带响应头，供需要读取 Content-Disposition 等响应头做文件名解析的下载场景（导出文件名单点，#230）。 */
export interface RawBlobResponse<T = Blob> {
  data: T
  headers: Record<string, string | string[] | undefined>
}

/** 解包客户端类型：拦截器成功直接返回业务负载 T；responseType 为 blob/arraybuffer 时返回二进制数据 */
type UnwrappedRequest = {
  get<T = any>(url: string, config: AxiosRequestConfig & { responseType: 'blob'; raw: true }): Promise<RawBlobResponse<T>>
  get<T = any>(url: string, config: AxiosRequestConfig & { responseType: 'blob' }): Promise<Blob>
  get<T = any>(url: string, config: AxiosRequestConfig & { responseType: 'arraybuffer' }): Promise<ArrayBuffer>
  get<T = any>(url: string, config?: AxiosRequestConfig): Promise<T>
  post<T = any>(url: string, data: any, config: AxiosRequestConfig & { responseType: 'blob' }): Promise<Blob>
  post<T = any>(url: string, data?: any, config?: AxiosRequestConfig): Promise<T>
  put<T = any>(url: string, data?: any, config?: AxiosRequestConfig): Promise<T>
  delete<T = any>(url: string, config: AxiosRequestConfig & { responseType: 'blob' }): Promise<Blob>
  delete<T = any>(url: string, config?: AxiosRequestConfig): Promise<T>
  patch<T = any>(url: string, data?: any, config?: AxiosRequestConfig): Promise<T>
  request<T = any>(config: AxiosRequestConfig): Promise<T>
  defaults: typeof axios.defaults
  interceptors: AxiosInstance['interceptors']
}

export interface HttpClientOptions {
  /** 请求前缀：'/api' / '/api/valuation' / '/api/ai-assistant' */
  baseURL: string
  /** 携带父域名 Cookie（子域名共享登录）；默认 false */
  withCredentials?: boolean
  /** 成功判定集合（默认 [200, 201]） */
  successCodes?: number[]
  /** 401 处理策略：由各模块提供（清登录态 + 按模块决定跳转）；可用 createDefaultUnauthorizedPolicy 收敛 */
  onUnauthorized: () => void
  /**
   * 证件过滤默认注入（#387，学员端主体系专用）：params 为普通对象且未显式传
   * credential_id 时，注入当前证件 id。估值/AI 助手两套 client 不开启。
   */
  injectCredentialId?: boolean
  /** 注入豁免前缀（仅 injectCredentialId 生效时读取；缺省 = 不做证件过滤的模块） */
  credentialExemptPrefixes?: string[]
}

/**
 * 证件过滤注入豁免前缀（#387）。口径：全局过滤器只覆盖课程/题库/练习/模考/错题/搜索/收藏
 * （CONTEXT.md「当前证件」），论坛与认证不过滤；凭证上下文自身（/credentials、/me）不注入；
 * admin/tutor/recruit 三端控制台共享主 client 但 credential_id 语义各不相同，一律不带。
 */
const DEFAULT_CREDENTIAL_EXEMPT_PREFIXES = [
  '/auth',
  '/captcha',
  '/credentials',
  '/me',
  '/forum',
  '/admin',
  '/tutor',
  '/recruit'
]

/** 豁免判定：url 与前缀按路径段对齐（/me 不误伤 /materials 这类前缀重叠） */
function isCredentialExempt(url: string, prefixes: string[]): boolean {
  return prefixes.some(p => url === p || url.startsWith(`${p}/`))
}

/**
 * 证件过滤默认注入（#387 单点实现，替代 api 模块内 9 处复制）：
 * params 为普通对象且未显式传 credential_id 时注入当前证件 id。
 * Pinia store 在拦截器回调内惰性获取（避免模块级循环依赖），
 * Pinia 未激活的环境（如直连工厂的单测）静默跳过。
 */
function injectCredentialId(config: AxiosRequestConfig, exemptPrefixes: string[]): void {
  if (isCredentialExempt(config.url || '', exemptPrefixes)) return
  const params = config.params as unknown
  if (
    !params ||
    typeof params !== 'object' ||
    Array.isArray(params) ||
    Object.prototype.toString.call(params) !== '[object Object]'
  ) {
    return
  }
  if ('credential_id' in (params as Record<string, unknown>)) return
  try {
    const cred = useCredentialStore().current?.id
    if (cred) (params as Record<string, unknown>).credential_id = cred
  } catch {
    // Pinia 未激活：跳过注入（与既有 try/catch 注入的容错口径一致）
  }
}

/** X-Silent 请求头：静默模式不弹错误提示（如轮询类请求） */
function isSilent(config: AxiosRequestConfig | undefined): unknown {
  return config?.headers?.['X-Silent'] || config?.headers?.['x-silent']
}

export interface UnauthorizedPolicyOptions {
  /** 清理登录态（auth store 不可用时需自带 storage 兜底） */
  clearAuth: () => void
  /** 是否跳转登录页；false = 仅清态不跳转（如 AI 助手未登录临时会话） */
  redirect?: boolean
  /** 登录页路径解析；默认恒为主登录页 /login */
  resolveLoginPath?: (currentPath: string) => string
}

/** 统一 401 策略：清登录态 + 仅需登录的页面跳登录页（单点实现，各实例只注入差异） */
export function createDefaultUnauthorizedPolicy(opts: UnauthorizedPolicyOptions): () => void {
  return () => {
    opts.clearAuth()
    if (opts.redirect === false) return
    // 延迟引入 router，避免与路由模块循环依赖
    import('@/router')
      .then(({ default: router }) => {
        if (router.currentRoute.value.matched.some(r => r.meta?.requiresAuth === true)) {
          router.push(opts.resolveLoginPath?.(router.currentRoute.value.path) ?? '/login')
        }
      })
      .catch(() => {
        // router 加载失败时不强制跳转，避免在公开页面误跳登录页
      })
  }
}

/** 创建共享客户端实例（Bearer 附加 / 解包 / 401 分发单点实现） */
export function createHttpClient<O extends HttpClientOptions>(opts: O): UnwrappedRequest {
  const successCodes = opts.successCodes ?? [200, 201]
  const exemptPrefixes = opts.credentialExemptPrefixes ?? DEFAULT_CREDENTIAL_EXEMPT_PREFIXES

  const client = axios.create({
    baseURL: opts.baseURL,
    timeout: 30000,
    ...(opts.withCredentials ? { withCredentials: true } : {}),
    headers: {
      'Content-Type': 'application/json; charset=utf-8'
    }
  })

  client.interceptors.request.use(
    config => {
      const token = getToken()
      if (token) {
        config.headers.Authorization = `Bearer ${token}`
      }
      if (opts.injectCredentialId) {
        injectCredentialId(config, exemptPrefixes)
      }
      return config
    },
    err => Promise.reject(err)
  )

  client.interceptors.response.use(
    response => {
      // 二进制响应：默认直接放行 Blob/ArrayBuffer 本身；请求标记 raw 时返回含响应头的完整对象
      //（供读取 Content-Disposition 等响应头的下载场景解析文件名，#230）。
      if (response.config.responseType === 'blob' || response.config.responseType === 'arraybuffer') {
        if ((response.config as AxiosRequestConfig & { raw?: boolean }).raw) {
          return {
            data: response.data,
            headers: response.headers
          }
        }
        return response.data
      }

      const body = response.data
      if (body && typeof body === 'object' && 'code' in body) {
        if (successCodes.includes(body.code)) {
          // 解包模式：成功（code ∈ 成功码集合）直接返回业务负载 data，调用方拿 Promise<T>
          return body.data
        }
        const msg = body.message || '请求失败'
        if (!isSilent(response.config)) {
          ElMessage.error(msg)
        }
        return Promise.reject(new Error(msg))
      }
      return body
    },
    async err => {
      if (err.response) {
        const status = err.response.status
        const data = err.response.data
        if (status === 401) {
          const url = err.config?.url || ''
          const cfg = err.config as AxiosRequestConfig & { _retry?: boolean }
          // 双令牌（ADR-0012）：非刷新端点、未重试过、本地有 refresh → 静默刷新后重试原请求
          if (!isRefreshEndpoint(url) && !cfg._retry && getRefreshToken()) {
            const refreshed = await tryRefreshTokens()
            if (refreshed) {
              cfg._retry = true
              // 重走本 client：请求拦截器重新读取已更新的 access token，再走解包
              return client.request(cfg)
            }
          }
          opts.onUnauthorized()
          if (!isSilent(err.config)) {
            ElMessage.error('登录已过期，请重新登录')
          }
          attachKind(err)
          return Promise.reject(err)
        }
        if (!isSilent(err.config)) {
          switch (status) {
            case 403:
              ElMessage.error(data?.message || '没有权限访问')
              break
            case 404:
              ElMessage.error('请求的资源不存在')
              break
            case 500:
              ElMessage.error('服务器错误，请稍后重试')
              break
            default: {
              const msg = data?.message || `请求失败 (${status})`
              ElMessage.error(msg)
            }
          }
        }
      } else if (err.code === 'ECONNABORTED' || /timeout\s+of\s+\d+\s+ms\s+exceeded/i.test(err.message || '')) {
        if (!isSilent(err.config)) {
          ElMessage.error('请求超时，请检查网络或稍后重试')
        }
      } else if (!isSilent(err.config)) {
        ElMessage.error('网络连接失败，请检查后端服务是否启动')
      }
      attachKind(err)
      return Promise.reject(err)
    }
  )

  return client as unknown as UnwrappedRequest
}
