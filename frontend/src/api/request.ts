// 主体系 HTTP 客户端（共享 client 工厂实例化：成功码 200/201、401 清登录态 + 按路由跳转）
import { createHttpClient, type ApiResponse } from './client'
import { useAuthStore } from '@/stores/auth'
import router from '@/router'

const request = createHttpClient({
  baseURL: import.meta.env.VITE_API_BASE_URL || '/api',
  withCredentials: true, // 携带父域名 Cookie，子域名间共享登录态
  successCodes: [200, 201],
  onUnauthorized: () => {
    useAuthStore().clearAuthData()
    // 仅在需要登录的页面跳转登录页；公开页面（如残值评估首页）保留当前视图
    if (router.currentRoute.value.matched.some(r => r.meta?.requiresAuth === true)) {
      router.push('/login')
    }
  }
})

export default request
export type { ApiResponse }
