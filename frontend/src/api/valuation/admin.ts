// 管理员 CRUD 接口：封装 /api/valuation/admin/* 下所有资源配置接口
// 设计说明：
//   - 通用资源 CRUD（createCrud）：列表走 /dictionaries/RES（学生端只读），
//     新增/编辑/删除走 /admin/RES（需 JWT role=admin）
//   - original-prices 后端为分页响应 { total, page, page_size, list }，list() 自动解包 .list
//   - 系数表（coefficient-configs）：list 走 /dictionaries，update 走 /admin/coefficient-configs/:key（按 key 单个更新）
//
// 响应类型**不再手写**（ADR-0048 决策 1/3，issue #967 片九）：本模块直接用生成物
// frontend/src/api/generated/valuation.ts 的类型；入参（Record<string, unknown>）不生成。
import client from './client'
import type { AlgorithmParameters, CoefficientConfig } from '@/api/generated/valuation'

export type { AlgorithmParameters }

/** 通用资源行：宽松字段，由后端定义具体结构 */
export type AdminRow = Record<string, unknown> & { id?: number }

/** 资源标识符：通常为 id（number） */
export type AdminResourceId = string | number

interface CrudEndpoints<Row = AdminRow> {
  list: (params?: Record<string, unknown>) => Promise<Row[]>
  create: (payload: Record<string, unknown>) => Promise<Row>
  update: (id: AdminResourceId, payload: Record<string, unknown>) => Promise<Row>
  remove: (id: AdminResourceId) => Promise<void>
  /** 从行数据中提取标识符；不存在返回 undefined */
  getIdOf: (row: Row | null | undefined) => AdminResourceId | undefined
}

/** createCrud 选项 */
interface CreateCrudOptions {
  /** list() 是否为分页响应（{total, page, page_size, list}） */
  isPaginated?: boolean
}

/** 创建一个资源的 CRUD 封装（Row 为生成物里的响应类型） */
function createCrud<Row extends AdminRow = AdminRow>(
  resource: string,
  options: CreateCrudOptions = {}
): CrudEndpoints<Row> {
  const { isPaginated = false } = options
  const dictBase = `/dictionaries/${resource}` // GET 列表（学生端字典端点，admin 与学生共用）
  const adminBase = `/admin/${resource}` // POST/PUT/DELETE 写操作（需 admin）
  return {
    async list(params?: Record<string, unknown>) {
      const merged = isPaginated ? { page: 1, page_size: 100, ...params } : params
      // 拦截器已解包信封，data 即业务负载；original-prices 分页时解包 .list
      const data = await client.get<Row[] | { list: Row[] }>(dictBase, { params: merged })
      if (isPaginated && data && typeof data === 'object' && 'list' in data) {
        return data.list ?? []
      }
      return (data as Row[]) ?? []
    },
    async create(payload: Record<string, unknown>) {
      return client.post<Row>(adminBase, payload)
    },
    async update(id: AdminResourceId, payload: Record<string, unknown>) {
      return client.put<Row>(`${adminBase}/${encodeURIComponent(id)}`, payload)
    },
    async remove(id: AdminResourceId): Promise<void> {
      await client.delete(`${adminBase}/${encodeURIComponent(id)}`)
      return undefined
    },
    getIdOf(row: Row | null | undefined): AdminResourceId | undefined {
      const v = row?.id
      if (typeof v === 'string' || typeof v === 'number') return v
      return undefined
    }
  }
}

// ========== 各资源配置 CRUD ==========
// original-prices 后端为分页响应，需特殊解包；其余实体列表为扁平数组。
export const adminResources = {
  originalPrices: createCrud('original-prices', { isPaginated: true }),
  brands: createCrud('brands'),
  vehicleTypes: createCrud('vehicle-types'),
  series: createCrud('series'),
  tonnages: createCrud('tonnages'),
  mastTypes: createCrud('mast-types'),
  mastHeights: createCrud('mast-heights'),
  batteryTypes: createCrud('battery-types'),
  transmissionTypes: createCrud('transmission-types'),
  engineTypes: createCrud('engine-types'),
  conditionRatings: createCrud('condition-ratings'),
  regionCoefficients: createCrud('region-coefficients')
} as const

export type AdminResourceKey = keyof typeof adminResources

// ========== 算法参数聚合接口 ==========

/** 拉取算法参数聚合数据（一次返回 4 类参数） */
export async function listAlgorithmParameters(): Promise<AlgorithmParameters> {
  const resp = await client.get<AlgorithmParameters>('/dictionaries/algorithm-parameters')
  return resp ?? { coefficients: [], brands: [], condition_ratings: [], region_coefficients: [] }
}

/** 更新单个全局系数（PUT /admin/coefficient-configs/:key；后端响应完整行，调用方只用成功与否） */
export async function updateCoefficient(key: string, value: number): Promise<void> {
  await client.put<CoefficientConfig>(`/admin/coefficient-configs/${encodeURIComponent(key)}`, { value })
}

/** 更新单个品牌系数（PUT /admin/brands/:id；响应为 {id,k_brand,is_active}） */
export async function updateBrandCoefficient(id: number, kBrand: number, isActive: boolean): Promise<void> {
  await client.put(`/admin/brands/${encodeURIComponent(id)}`, { k_brand: kBrand, is_active: isActive })
}

/** 更新单个车况系数（PUT /admin/condition-ratings/:id） */
export async function updateConditionCoefficient(id: number, label: string, baseCoefficient: number): Promise<void> {
  await client.put(`/admin/condition-ratings/${encodeURIComponent(id)}`, { label, base_coefficient: baseCoefficient })
}

/** 更新单个区域系数（PUT /admin/region-coefficients/:id） */
export async function updateRegionCoefficient(id: number, coefficient: number): Promise<void> {
  await client.put(`/admin/region-coefficients/${encodeURIComponent(id)}`, { coefficient })
}
