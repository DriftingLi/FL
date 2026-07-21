import axios from 'axios'
import request from './request'
import { useAuthStore } from '@/stores/auth'

/** 内容精选分类标签映射 */
export const featuredCategoryLabels: Record<string, string> = {
  company: '公司动态',
  industry: '行业新闻',
  product: '产品资讯',
  news: '资讯'
}

/** 内容精选分类选项（管理端表单下拉用） */
export const featuredCategoryOptions = [
  { value: 'company', label: '公司动态' },
  { value: 'industry', label: '行业新闻' },
  { value: 'product', label: '产品资讯' },
  { value: 'news', label: '资讯' }
]

/** 获取分类中文标签 */
export function categoryLabel(category: string): string {
  return featuredCategoryLabels[category] || '资讯'
}

/** 公开接口 */
export const featuredApi = {
  /** 公开列表（仅已发布） */
  getPublicList(params: { page?: number; page_size?: number; category?: string } = {}) {
    return request.get('/featured-contents', { params })
  },

  /** 公开详情（含相关资讯 + 上一篇/下一篇） */
  getPublicDetail(id: number) {
    return request.get(`/featured-content/${id}`)
  }
}

/** 管理端接口 */
export const adminFeaturedApi = {
  /** 管理端列表（含草稿） */
  getList(params: { page?: number; page_size?: number; category?: string; status?: string } = {}) {
    return request.get('/admin/featured-contents', { params })
  },

  /** 管理端详情 */
  getDetail(id: number) {
    return request.get(`/admin/featured-content/${id}`)
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
    return request.post('/admin/featured-content', data)
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
    return request.put(`/admin/featured-content/${id}`, data)
  },

  /** 删除内容精选 */
  remove(id: number) {
    return request.delete(`/admin/featured-content/${id}`)
  },

  /** 发布内容精选（草稿 → 已发布） */
  publish(id: number) {
    return request.post(`/admin/featured-content/${id}/publish`)
  },

  /** 上传图片（Markdown 编辑器内嵌 + 封面）
   *  后端返回 Vditor 期望格式：{ msg, code: 0|1, data: { errFiles, succMap } }
   *  注意：code=0 表示成功（Vditor 约定），与全局拦截器（仅放行 200/201）冲突，
   *  因此此处改用原生 axios 绕过全局拦截器，调用方需通过 res.data.code 判断成败。
   */
  async uploadImage(file: File) {
    const fd = new FormData()
    fd.append('file', file)
    const authStore = useAuthStore()
    const headers: Record<string, string> = {}
    if (authStore.token) headers.Authorization = `Bearer ${authStore.token}`
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
