import axios from 'axios'
import type { AxiosRequestConfig, AxiosInstance } from 'axios'
import { ElMessage } from 'element-plus'
import { useAuthStore } from '@/stores/auth'
import router from '@/router'

/** 后端通用 JSON 响应格式 */
export interface ApiResponse<T = any> {
  code: number
  message: string
  data: T
}

/** 请求实例的类型：拦截器已将 AxiosResponse 解包为 ApiResponse */
type TypedRequest = {
  get<T = any>(url: string, config?: AxiosRequestConfig): Promise<ApiResponse<T>>
  post<T = any>(url: string, data?: any, config?: AxiosRequestConfig): Promise<ApiResponse<T>>
  put<T = any>(url: string, data?: any, config?: AxiosRequestConfig): Promise<ApiResponse<T>>
  delete<T = any>(url: string, config?: AxiosRequestConfig): Promise<ApiResponse<T>>
  patch<T = any>(url: string, data?: any, config?: AxiosRequestConfig): Promise<ApiResponse<T>>
  request<T = any>(config: AxiosRequestConfig): Promise<ApiResponse<T>>
  defaults: typeof axios.defaults
  interceptors: AxiosInstance['interceptors']
}

const request = axios.create({
  baseURL: import.meta.env.VITE_API_BASE_URL || '/api',
  timeout: 30000,
  headers: {
    'Content-Type': 'application/json'
  }
})

const aiRequest = axios.create({
  baseURL: import.meta.env.VITE_API_BASE_URL || '/api',
  timeout: 120000,
  headers: {
    'Content-Type': 'application/json'
  }
})

function isSilent(config) {
  return config?.headers?.['X-Silent'] || config?.headers?.['x-silent']
}

request.interceptors.request.use(
  config => {
    const authStore = useAuthStore()
    if (authStore.token) {
      config.headers.Authorization = `Bearer ${authStore.token}`
    }
    return config
  },
  error => {
    return Promise.reject(error)
  }
)

request.interceptors.response.use(
  response => {
    if (response.config.responseType === 'blob') {
      return response.data
    }

    const res = response.data

    if (res.code && res.code !== 200 && res.code !== 201) {
      if (!isSilent(response.config)) {
        ElMessage.error(res.message || '请求失败')
      }
      return Promise.reject(new Error(res.message || '请求失败'))
    }

    return res
  },
  error => {
    if (error.response) {
      const status = error.response.status
      const data = error.response.data

      if (isSilent(error.config)) {
        return Promise.reject(error)
      }

      switch (status) {
        case 401:
          useAuthStore().clearAuthData()
          // 仅在需要登录的页面跳转登录页；公开页面（如残值评估首页）保留当前视图
          if (router.currentRoute.value.matched.some(r => r.meta?.requiresAuth === true)) {
            router.push('/login')
          }
          ElMessage.error('登录已过期，请重新登录')
          break
        case 403:
          ElMessage.error(data?.message || '没有权限访问')
          break
        case 404:
          ElMessage.error('请求的资源不存在')
          break
        case 500:
          ElMessage.error('服务器错误，请稍后重试')
          break
        default:
          const msg = data?.message || error.message || '请求失败'
          ElMessage.error(msg)
      }
    } else if (error.code === 'ECONNABORTED' || /timeout\s+of\s+\d+\s+ms\s+exceeded/i.test(error.message || '')) {
      if (!isSilent(error.config)) {
        ElMessage.error('请求超时，请检查网络或稍后重试')
      }
    } else {
      if (!isSilent(error.config)) {
        ElMessage.error('网络连接失败，请检查后端服务是否启动')
      }
    }
    return Promise.reject(error)
  }
)

aiRequest.interceptors.request.use(
  config => {
    const authStore = useAuthStore()
    if (authStore.token) {
      config.headers.Authorization = `Bearer ${authStore.token}`
    }
    return config
  },
  error => {
    return Promise.reject(error)
  }
)

aiRequest.interceptors.response.use(
  response => {
    const res = response.data

    if (res.code && res.code !== 200 && res.code !== 201) {
      ElMessage.error(res.message || '请求失败')
      return Promise.reject(new Error(res.message || '请求失败'))
    }

    return res
  },
  error => {
    if (error.response) {
      const status = error.response.status
      const data = error.response.data

      switch (status) {
        case 401:
          useAuthStore().clearAuthData()
          // 仅在需要登录的页面跳转登录页；公开页面保留当前视图
          if (router.currentRoute.value.matched.some(r => r.meta?.requiresAuth === true)) {
            router.push('/login')
          }
          ElMessage.error('登录已过期，请重新登录')
          break
        case 403:
          ElMessage.error(data?.message || '没有权限访问')
          break
        case 404:
          ElMessage.error('请求的资源不存在')
          break
        case 500:
          ElMessage.error('服务器错误，请稍后重试')
          break
        default:
          const msg = data?.message || error.message || '请求失败'
          ElMessage.error(msg)
      }
    } else if (error.code === 'ECONNABORTED') {
      ElMessage.error('请求超时，AI生成可能需要较长时间，请稍后查看结果')
    } else {
      ElMessage.error('网络连接失败，请检查后端服务是否启动')
    }
    return Promise.reject(error)
  }
)

export { aiRequest }
export default request as unknown as TypedRequest
