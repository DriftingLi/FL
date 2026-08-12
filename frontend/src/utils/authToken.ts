// URL auth_token 一次性交接：跨子域名跳转携带、读取后立即从地址栏移除。
// 分层设计：extractAuthTokenFromUrl 为可单测纯函数，consumeAuthTokenFromUrl 负责地址栏清理。

export interface AuthTokenInUrl {
  /** 提取到的 auth_token（缺失/为空时为空串） */
  token: string
  /** 移除 auth_token 后剩余的查询串（无参数时为空串） */
  remainingQuery: string
}

/** 从查询串提取 auth_token 并计算移除后的剩余查询串（纯函数） */
export function extractAuthTokenFromUrl(search: string): AuthTokenInUrl {
  const params = new URLSearchParams(search)
  const token = params.get('auth_token') || ''
  if (token) params.delete('auth_token')
  return { token, remainingQuery: params.toString() }
}

/** 消费地址栏的一次性 auth_token：返回 token 并立即从地址栏移除（无 token 时 no-op） */
export function consumeAuthTokenFromUrl(): string {
  if (typeof window === 'undefined') return ''
  const { token, remainingQuery } = extractAuthTokenFromUrl(window.location.search)
  if (!token) return ''
  const newUrl = window.location.pathname + (remainingQuery ? `?${remainingQuery}` : '') + window.location.hash
  window.history.replaceState(null, '', newUrl)
  return token
}
