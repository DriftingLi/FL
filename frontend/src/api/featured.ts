import axios from 'axios'
import { unwrappedRequest } from './request'
import { getValidAccessToken } from './client'

/** 内容精选分类标签映射 */
export const featuredCategoryLabels: Record<string, string> = {
  company: '公司动态',
  industry: '行业新闻',
  product: '产品资讯',
  news: '政策法规'
}

/** 内容精选分类选项（管理端表单下拉用） */
export const featuredCategoryOptions = [
  { value: 'company', label: '公司动态' },
  { value: 'industry', label: '行业新闻' },
  { value: 'product', label: '产品资讯' },
  { value: 'news', label: '政策法规' }
]

/** 获取分类中文标签 */
export function categoryLabel(category: string): string {
  return featuredCategoryLabels[category] || '政策法规'
}

/** 精选内容项（wire key 与后端 typed DTO 一致：主键为 content_id） */
export interface FeaturedContent {
  content_id: number
  title: string
  category?: string
  category_label?: string
  summary?: string
  cover_image?: string
  content?: string
  source?: string
  status?: number
  view_count?: number
  sort_order?: number
  published_at?: string | null
  created_at?: string
  updated_at?: string
}

// ===== 公开接口已移除 =====
// 官网门户重构为独立 Nuxt 仓库（ADR-0001）后，公开接口改由门户数据访问层消费
// （portal/api/featured.ts，含 no_view=1 与客户端计数端点）；管理端接口保留于此。

/** 管理端接口 */
export const adminFeaturedApi = {
  /** 管理端列表（含草稿） */
  getList(params: { page?: number; page_size?: number; category?: string; status?: string } = {}) {
    return unwrappedRequest.get<{ items: FeaturedContent[]; total: number }>('/admin/featured-contents', { params })
  },

  /** 管理端详情 */
  getDetail(id: number) {
    return unwrappedRequest.get<FeaturedContent>(`/admin/featured-content/${id}`)
  },

  /** 创建内容精选 */
  create(data: {
    title: string
    category: string
    summary?: string
    cover_image?: string
    content?: string
    source?: string
    status?: number
    sort_order?: number
  }) {
    return unwrappedRequest.post<FeaturedContent>('/admin/featured-content', data)
  },

  /** 更新内容精选 */
  update(id: number, data: {
    title?: string
    category?: string
    summary?: string
    cover_image?: string
    content?: string
    source?: string
    status?: number
    sort_order?: number
  }) {
    return unwrappedRequest.put<FeaturedContent>(`/admin/featured-content/${id}`, data)
  },

  /** 删除内容精选 */
  remove(id: number) {
    return unwrappedRequest.delete<null>(`/admin/featured-content/${id}`)
  },

  /** 发布内容精选（草稿 → 已发布） */
  publish(id: number) {
    return unwrappedRequest.post<null>(`/admin/featured-content/${id}/publish`)
  },

  /** 上传图片（Markdown 编辑器内嵌 + 封面）
   *  后端返回 Vditor 期望格式：{ msg, code: 0|1, data: { errFiles, succMap } }
   *  注意：code=0 表示成功（Vditor 约定），与全局拦截器（仅放行 200/201）冲突，
   *  因此此处改用原生 axios 绕过全局拦截器，调用方需通过 res.data.code 判断成败。
   */
  async uploadImage(file: File) {
    const fd = new FormData()
    fd.append('file', file)
    // 原生 axios 绕过全局拦截器（Vditor code=0 约定），无法依赖 401→自动刷新；
    // 发起前显式换取新鲜 access token（本地过期则静默刷新），避免登录 2h 后上传持续 401
    const headers: Record<string, string> = {}
    const token = await getValidAccessToken()
    if (token) headers.Authorization = `Bearer ${token}`
    const baseURL = import.meta.env.VITE_API_BASE_URL || '/api'
    const res = await axios.post(`${baseURL}/admin/featured-content/upload-image`, fd, { headers })
    // 直接返回后端原始 Vditor 格式数据
    return res.data as {
      msg: string
      code: number
      data: { errFiles: string[]; succMap: Record<string, string> }
    }
  }
}
