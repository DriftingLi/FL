// 管理员 CRUD 接口：封装 /api/valuation/admin/* 下所有资源配置接口
// 设计说明：
//   - 列表（list）：GET /dictionaries/${resource}（后端字典端点，admin 与学生共用只读列表）
//   - 新增/编辑/删除：POST/PUT/DELETE /admin/${resource}（需 JWT role=admin）
//   - original-prices 后端为分页响应 { total, page, page_size, list }，list() 自动解包 .list
//   - 系数表（coefficient-configs）：list 走 /dictionaries，update 走 /admin/coefficient-configs/:key（按 key 单个更新）
//   - brand-types 后端用 :name 作为 PUT/DELETE 路径参数（无 id 字段），通过 idField='name' 适配
import client from './client'
import type { CoefficientConfig } from '@/types/valuation/evaluation'

/** 通用资源行：宽松字段，由后端定义具体结构 */
export type AdminRow = Record<string, unknown> & { id?: number }

/** 资源标识符：通常为 id（number），brand-types 等用 name（string） */
export type AdminResourceId = string | number

interface CrudEndpoints {
  list: <T = AdminRow>(params?: Record<string, unknown>) => Promise<T[]>
  create: <T = AdminRow>(payload: Record<string, unknown>) => Promise<T>
  update: <T = AdminRow>(id: AdminResourceId, payload: Record<string, unknown>) => Promise<T>
  remove: (id: AdminResourceId) => Promise<void>
  /** 从行数据中提取标识符（idField 对应字段的值）；不存在返回 undefined */
  getIdOf: (row: AdminRow | null | undefined) => AdminResourceId | undefined
}

/** original-prices 分页响应结构 */
interface OriginalPricesPage {
  total: number
  page: number
  page_size: number
  list: AdminRow[]
}

/** createCrud 选项 */
interface CreateCrudOptions {
  /** list() 是否为分页响应（{total, page, page_size, list}） */
  isPaginated?: boolean
  /** 标识符字段名，默认 'id'；brand-types 用 'name' */
  idField?: string
}

/** 创建一个资源的 CRUD 封装 */
// - resource：资源路径片段（如 'original-prices' / 'brands' / 'brand-types'）
// - options.isPaginated：为 true 时 list() 从 {total, page, page_size, list} 中解包 .list，
//                        并默认请求 page=1&page_size=100（后端最大值），避免管理端表格被分页截断
// - options.idField：标识符字段名，默认 'id'；brand-types 后端无 id 字段，用 :name 路径参数
function createCrud(resource: string, options: CreateCrudOptions = {}): CrudEndpoints {
  const { isPaginated = false, idField = 'id' } = options
  const dictBase = `/dictionaries/${resource}` // GET 列表（学生端字典端点，admin 与学生共用）
  const adminBase = `/admin/${resource}` // POST/PUT/DELETE 写操作（需 admin）
  return {
    list<T = AdminRow>(params?: Record<string, unknown>) {
      const merged = isPaginated
        ? { page: 1, page_size: 100, ...params }
        : params
      return client
        .get<unknown, { data: T[] | OriginalPricesPage }>(dictBase, { params: merged })
        .then((r) => {
          const data = r.data
          if (isPaginated && data && typeof data === 'object' && 'list' in data) {
            return (data.list as T[]) ?? []
          }
          return (data as T[]) ?? []
        })
    },
    create<T = AdminRow>(payload: Record<string, unknown>) {
      return client
        .post<unknown, { data: T }>(adminBase, payload)
        .then((r) => r.data)
    },
    update<T = AdminRow>(id: AdminResourceId, payload: Record<string, unknown>) {
      return client
        .put<unknown, { data: T }>(`${adminBase}/${encodeURIComponent(id)}`, payload)
        .then((r) => r.data)
    },
    remove(id: AdminResourceId) {
      return client.delete(`${adminBase}/${encodeURIComponent(id)}`).then(() => undefined)
    },
    getIdOf(row: AdminRow | null | undefined): AdminResourceId | undefined {
      if (!row) return undefined
      const v = row[idField]
      if (typeof v === 'string' || typeof v === 'number') return v
      return undefined
    }
  }
}

// ========== 各资源配置 CRUD ==========
// original-prices 后端为分页响应，需特殊解包
// brand-types 后端无 id 字段，PUT/DELETE 用 :name 作为路径参数
export const adminResources = {
  originalPrices: createCrud('original-prices', { isPaginated: true }),
  brandTypes: createCrud('brand-types', { idField: 'name' }),
  brands: createCrud('brands'),
  vehicleTypes: createCrud('vehicle-types'),
  series: createCrud('series'),
  tonnages: createCrud('tonnages'),
  configTypes: createCrud('config-types'),
  mastTypes: createCrud('mast-types'),
  mastHeights: createCrud('mast-heights'),
  batteryTypes: createCrud('battery-types'),
  transmissionTypes: createCrud('transmission-types'),
  engineTypes: createCrud('engine-types'),
  conditionRatings: createCrud('condition-ratings'),
  regionCoefficients: createCrud('region-coefficients')
} as const

export type AdminResourceKey = keyof typeof adminResources

// ========== 系数表（GET 列表 + PUT 单个 key） ==========

/** 拉取算法参数表（GET /dictionaries/coefficient-configs） */
export async function listAdminCoefficients(): Promise<CoefficientConfig[]> {
  const resp = await client.get<unknown, { data: CoefficientConfig[] }>('/dictionaries/coefficient-configs')
  return resp.data ?? []
}

/**
 * 整体更新算法参数表
 * 后端契约：PUT /admin/coefficient-configs/:key  Body: { value: number }（按 key 单个更新）
 * 这里并发提交所有变更项；任意一项失败则整体 reject
 */
export async function updateAdminCoefficients(payload: CoefficientConfig[]): Promise<CoefficientConfig[]> {
  await Promise.all(
    payload.map((c) =>
      client.put<unknown, { data: unknown }>(`/admin/coefficient-configs/${encodeURIComponent(c.key)}`, {
        value: c.value
      })
    )
  )
  // 返回提交的 payload，便于调用方直接更新本地副本
  return payload
}
