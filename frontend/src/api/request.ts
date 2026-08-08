// 主体系 HTTP 客户端（共享 client 工厂实例化：成功码 200/201、401 清登录态 + 按路由跳转）
// 拦截器解包信封（ADR-0005）：成功直接返回业务负载 data（Promise<T>），业务失败抛错并统一 toast。
import { createHttpClient, createDefaultUnauthorizedPolicy, type ApiResponse } from './client'
import { useAuthStore } from '@/stores/auth'

// 统一 401 策略（单点实现于 client.ts）：清登录态 + 仅需登录的页面跳主登录页
const onUnauthorized = createDefaultUnauthorizedPolicy({
  clearAuth: () => useAuthStore().clearAuthData()
})

// 主体系唯一客户端：所有 api 模块共享（信封解包为唯一行为，无 raw 逃生口）
const unwrappedRequest = createHttpClient({
  baseURL: import.meta.env.VITE_API_BASE_URL || '/api',
  withCredentials: true, // 携带父域名 Cookie，子域名间共享登录态
  successCodes: [200, 201],
  onUnauthorized
})

export { unwrappedRequest }
export type { ApiResponse }
