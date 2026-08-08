// 估值模块 HTTP 客户端（共享 client 工厂实例化：成功码 200——统一信封 ADR-0005）
// 后端统一响应：{code, message, data}（code = HTTP 状态码，成功即 200）
// 拦截器解包信封：成功直接返回业务负载 data（Promise<T>），业务失败抛错并统一 toast。
import { createHttpClient, createDefaultUnauthorizedPolicy } from '@/api/client'

// 维修培训 VITE_API_BASE_URL 默认为 /api（vite proxy 代理到 8080）；
// valuation 路由统一挂在 /api/valuation/* 下，故 baseURL 解析为 <base>/api/valuation
const API_BASE_URL = (import.meta.env.VITE_API_BASE_URL || '/api').replace(/\/api$/, '') + '/api/valuation'

const client = createHttpClient({
  baseURL: API_BASE_URL,
  successCodes: [200],
  // 统一 401 策略（client.ts 单点）：清登录态 + 估值路径跳估值登录页、其余跳主登录页
  onUnauthorized: createDefaultUnauthorizedPolicy({
    clearAuth: () => {
      // 延迟引入 auth store，避免循环依赖；store 不可用时兜底清 storage
      import('@/stores/auth')
        .then(({ useAuthStore }) => {
          try {
            useAuthStore().clearAuthData()
          } catch {
            removeLocalAuth()
          }
        })
        .catch(() => {
          removeLocalAuth()
        })
    },
    resolveLoginPath: currentPath => (currentPath.startsWith('/valuation') ? '/valuation/login' : '/login')
  })
})

// 尽力清除本地登录态（auth store 不可用时的兜底）
function removeLocalAuth(): void {
  localStorage.removeItem('token')
  localStorage.removeItem('userInfo')
}

export default client
export { API_BASE_URL }
