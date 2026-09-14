import axios from 'axios'
import { unwrappedRequest } from './request'
import { getValidAccessToken } from './client'
import type {
  FeaturedContentAdminDetailDTO,
  FeaturedContentDetailDTO,
  FeaturedContentDTO,
  FeaturedContentPageResult,
  FeaturedDeleteResult
} from './generated/featured'

// 响应类型**不再手写**：唯一事实源是后端注解 → backend/docs/swagger.json →
// `cd backend && go run ./cmd/gen-apitypes`（ADR-0048 决策 1/3，issue #965 片七）。
// 入参（query / body）类型不生成、仍手写（ADR-0048 决策 3）。
//
// 差异处置（详见片七字段级差异清单）：旧的 `FeaturedContent` 是**列表项与详情混用**的一份
// 全可选 interface（凭空多出 content 字段、可选性全面放宽），与注解产物不一一对应，
// 归类②「手写类型过时」→ 删旧名，列表用 FeaturedContentDTO、详情用 FeaturedContentAdminDetailDTO。
export type { FeaturedContentDTO, FeaturedContentAdminDetailDTO, FeaturedContentPageResult, FeaturedDeleteResult }

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

// ===== 公开详情：本仓 training 域的第二个消费者（ADR-0049 决策 4）=====
// 公开列表/计数端点的消费者仍是独立 Nuxt 门户仓（ADR-0001，portal/api/featured.ts）；
// 但**详情**自 ADR-0049 决策 4 起也被训练域消费——搜索结果的「内容精选」落点需要一个
// 自包含的详情页（不再把学员带出工作区），故这里保留一个公开详情读取。
export const featuredApi = {
  /** 内容精选详情（公开端点；默认计数阅读量，传 no_view=1 时不计数） */
  getDetail(id: number, params?: { no_view?: string }) {
    return unwrappedRequest.get<FeaturedContentDetailDTO>(`/featured-content/${id}`, { params })
  }
}

/** 管理端接口 */
export const adminFeaturedApi = {
  /** 管理端列表（含草稿） */
  getList(params: { page?: number; page_size?: number; category?: string; status?: string } = {}) {
    return unwrappedRequest.get<FeaturedContentPageResult>('/admin/featured-contents', { params })
  },

  /** 管理端详情 */
  getDetail(id: number) {
    return unwrappedRequest.get<FeaturedContentAdminDetailDTO>(`/admin/featured-content/${id}`)
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
    return unwrappedRequest.post<FeaturedContentAdminDetailDTO>('/admin/featured-content', data)
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
    return unwrappedRequest.put<FeaturedContentAdminDetailDTO>(`/admin/featured-content/${id}`, data)
  },

  /** 删除内容精选 */
  remove(id: number) {
    return unwrappedRequest.delete<FeaturedDeleteResult>(`/admin/featured-content/${id}`)
  },

  /** 发布内容精选（草稿 → 已发布） */
  publish(id: number) {
    return unwrappedRequest.post<FeaturedContentAdminDetailDTO>(`/admin/featured-content/${id}/publish`)
  },

  /** 上传图片（Markdown 编辑器内嵌 + 封面）
   *  后端返回 Vditor 期望格式：{ msg, code: 0|1, data: { errFiles, succMap } }
   *  注意：code=0 表示成功（Vditor 约定），与全局拦截器（仅放行 200/201）冲突，
   *  因此此处改用原生 axios 绕过全局拦截器，调用方需通过 res.data.code 判断成败。
   *
   *  该响应**不是统一信封**（Vditor 协议），注解层只登记端点、不指认 data
   *  （域声明 NoData:true），故这里保留内联的非信封形状（ADR-0048 决策 6 口径）。
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
