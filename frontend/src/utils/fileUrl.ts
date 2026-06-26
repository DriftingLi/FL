export function resolveFileUrl(url) {
  if (!url) return ''
  if (url.startsWith('http://') || url.startsWith('https://') || url.startsWith('blob:')) {
    return url
  }
  const apiBase = import.meta.env.VITE_API_BASE_URL || '/api'
  const baseUrl = apiBase.replace(/\/api\/?$/, '').replace(/\/$/, '')
  const cleanUrl = url.startsWith('/') ? url : '/' + url
  return baseUrl + cleanUrl
}
