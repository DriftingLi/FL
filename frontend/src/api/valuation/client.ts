// Axios 实例与统一拦截器（已适配维修培训认证体系 + Element Plus）
// 后端统一响应：{code, message, data}（valuation 后端 code===0 为成功）
import axios, { AxiosError, type AxiosResponse, type InternalAxiosRequestConfig } from 'axios'
import { ElMessage } from 'element-plus'
import type { ApiResponse } from '@/types/valuation/evaluation'

// 维修培训 VITE_API_BASE_URL 默认为 /api（vite proxy 代理到 8080）；
// valuation 路由统一挂在 /api/valuation/* 下，故 baseURL 解析为 <base>/api/valuation
const API_BASE_URL = (import.meta.env.VITE_API_BASE_URL || '/api').replace(/\/api$/, '') + '/api/valuation'
const REQUEST_TIMEOUT_MS = 30_000

const TOKEN_STORAGE_KEY = 'valuation_token'
const USER_INFO_KEY = 'valuation_userInfo'
// 主体系 admin token 存储 key（管理员后台接口 /admin/* 需用主体系 JWT 鉴权）
const ADMIN_TOKEN_STORAGE_KEY = 'token'

const client = axios.create({
  baseURL: API_BASE_URL,
  timeout: REQUEST_TIMEOUT_MS,
  headers: { 'Content-Type': 'application/json; charset=utf-8' }
})

// ========== 请求拦截器：附加 JWT ==========
// /admin/* 路径走主体系 admin JWT（localStorage 'token'），其余路径走估值独立 JWT（localStorage 'valuation_token'）
client.interceptors.request.use(
  (config: InternalAxiosRequestConfig) => {
    const url = config.url || ''
    const isAdminPath = url.startsWith('/admin/') || url === '/admin'
    const token = isAdminPath
      ? localStorage.getItem(ADMIN_TOKEN_STORAGE_KEY)
      : localStorage.getItem(TOKEN_STORAGE_KEY)
    if (token) {
      config.headers.Authorization = `Bearer ${token}`
    }
    return config
  },
  (err) => Promise.reject(err)
)

// 延迟引入 valuation auth store 与 router，避免循环依赖
function handleValuationUnauthorized() {
  // 动态加载，防止模块初始化顺序问题
  import('@/stores/valuationAuth')
    .then(({ useValuationAuthStore }) => {
      try {
        useValuationAuthStore().clearAuthData()
      } catch (e) {
        localStorage.removeItem(TOKEN_STORAGE_KEY)
        localStorage.removeItem(USER_INFO_KEY)
      }
    })
    .catch(() => {
      localStorage.removeItem(TOKEN_STORAGE_KEY)
      localStorage.removeItem(USER_INFO_KEY)
    })
  // 仅在需要登录的页面跳转估值专属登录页；公开页面（如残值评估首页）保留当前视图
  import('@/router')
    .then(({ default: router }) => {
      if (router.currentRoute.value.matched.some(r => r.meta?.requiresAuth === true)) {
        router.push('/valuation/login')
      }
    })
    .catch(() => {
      // router 加载失败时不强制跳转，避免在公开页面误跳登录页
    })
}

// 管理员路径 401：主体系 admin JWT 过期，清主体系登录态并跳主登录页
function handleAdminUnauthorized() {
  localStorage.removeItem(ADMIN_TOKEN_STORAGE_KEY)
  localStorage.removeItem('userInfo')
  import('@/router')
    .then(({ default: router }) => {
      router.push('/login')
    })
    .catch(() => {
      // router 加载失败时仅刷新当前页触发主体系鉴权重定向
    })
}

// 判断请求 URL 是否为管理员路径（/admin/* 走主体系 admin JWT）
function isAdminRequest(config: AxiosError['config']): boolean {
  const url = config?.url || ''
  return url.startsWith('/admin/') || url === '/admin'
}

// ========== 响应拦截器 ==========
client.interceptors.response.use(
  (response: AxiosResponse<ApiResponse<unknown>>) => {
    // 二进制响应（如 PDF）直接放行
    if (response.config.responseType === 'blob' || response.config.responseType === 'arraybuffer') {
      return response
    }
    const body = response.data
    if (body && typeof body === 'object' && 'code' in body) {
      if (body.code === 0) {
        // 解包 data 字段，返回 AxiosResponse 形态方便上层 .data 取用
        return { ...response, data: body.data as unknown }
      }
      // 业务错误：弹出提示并 reject
      const errMsg = body.message || `业务错误（code=${body.code}）`
      ElMessage.error(errMsg)
      return Promise.reject(new Error(errMsg))
    }
    return response
  },
  async (err: AxiosError) => {
    if (err.response) {
      const status = err.response.status
      // 401：登录过期，按请求来源分流处理（管理员路径走主体系，其余走估值独立）
      if (status === 401) {
        if (isAdminRequest(err.config)) {
          handleAdminUnauthorized()
        } else {
          handleValuationUnauthorized()
        }
        ElMessage.error('登录已过期，请重新登录')
        return Promise.reject(err)
      }
      // blob/arraybuffer 错误响应需先读取为文本再解析 JSON
      let data: unknown = err.response.data
      const rt = err.config?.responseType
      if ((rt === 'blob' || rt === 'arraybuffer') && data instanceof Blob) {
        try {
          const text = await data.text()
          data = JSON.parse(text)
        } catch {
          data = undefined
        }
      }
      const msg = (data as { message?: string } | undefined)?.message || `请求失败 (${status})`
      ElMessage.error(msg)
    } else if (err.request) {
      ElMessage.error('网络异常：无法连接服务器')
    } else {
      ElMessage.error(`请求错误：${err.message}`)
    }
    return Promise.reject(err)
  }
)

export default client
export { API_BASE_URL }
