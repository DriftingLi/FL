// 共享 HTTP 客户端工厂：Bearer 附加、{code,message,data} 解包、401 策略、错误提示单点实现。
// 三个模块（主体系 / 估值 / AI 助手）各自以 createHttpClient 创建实例，只注入差异：
// baseURL、成功码集合与 401 策略。token 读写统一走 utils/storage.ts（单点 key）。
import axios from 'axios'
import type { AxiosRequestConfig, AxiosInstance } from 'axios'
import { ElMessage } from 'element-plus'
import { getToken } from '@/utils/storage'

/** 后端通用 JSON 响应格式（code = HTTP 状态码，统一信封，见 ADR-0005） */
export interface ApiResponse<T = any> {
  code: number
  message: string
  data: T
}

/** 请求实例的类型：拦截器将响应解包为 ApiResponse（.data 为业务负载）；
 *  responseType 为 blob/arraybuffer 时拦截器直接返回二进制数据 */
type TypedRequest = {
  get<T = any>(url: string, config: AxiosRequestConfig & { responseType: 'blob' }): Promise<Blob>
  get<T = any>(url: string, config: AxiosRequestConfig & { responseType: 'arraybuffer' }): Promise<ArrayBuffer>
  get<T = any>(url: string, config?: AxiosRequestConfig): Promise<ApiResponse<T>>
  post<T = any>(url: string, data: any, config: AxiosRequestConfig & { responseType: 'blob' }): Promise<Blob>
  post<T = any>(url: string, data?: any, config?: AxiosRequestConfig): Promise<ApiResponse<T>>
  put<T = any>(url: string, data?: any, config?: AxiosRequestConfig): Promise<ApiResponse<T>>
  delete<T = any>(url: string, config: AxiosRequestConfig & { responseType: 'blob' }): Promise<Blob>
  delete<T = any>(url: string, config?: AxiosRequestConfig): Promise<ApiResponse<T>>
  patch<T = any>(url: string, data?: any, config?: AxiosRequestConfig): Promise<ApiResponse<T>>
  request<T = any>(config: AxiosRequestConfig): Promise<ApiResponse<T>>
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
  /** 401 处理策略：由各模块提供（清登录态 + 按模块决定跳转） */
  onUnauthorized: () => void
}

/** X-Silent 请求头：静默模式不弹错误提示（如轮询类请求） */
function isSilent(config: AxiosRequestConfig | undefined): unknown {
  return config?.headers?.['X-Silent'] || config?.headers?.['x-silent']
}

/** 创建共享客户端实例（Bearer 附加 / 解包 / 401 分发单点实现） */
export function createHttpClient(opts: HttpClientOptions): TypedRequest {
  const successCodes = opts.successCodes ?? [200, 201]

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
      return config
    },
    err => Promise.reject(err)
  )

  client.interceptors.response.use(
    response => {
      // 二进制响应直接放行（返回 Blob/ArrayBuffer 本身）
      if (response.config.responseType === 'blob' || response.config.responseType === 'arraybuffer') {
        return response.data
      }

      const body = response.data
      if (body && typeof body === 'object' && 'code' in body) {
        if (successCodes.includes(body.code)) {
          // 统一信封对象（.data 为业务负载）
          return body
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
          opts.onUnauthorized()
          if (!isSilent(err.config)) {
            ElMessage.error('登录已过期，请重新登录')
          }
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
      return Promise.reject(err)
    }
  )

  return client as unknown as TypedRequest
}
