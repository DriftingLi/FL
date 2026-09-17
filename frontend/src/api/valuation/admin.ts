// 管理员 CRUD 接口：封装 /api/valuation/admin/* 下所有资源配置接口
// 设计说明：
//   - 资源 CRUD 走**具名方法表** adminResources（显式资源 → 显式路径）：列表走
//     /dictionaries/<resource>（学生端只读，admin 与学生共用），新增/编辑/删除走
//     /admin/<resource>（需 JWT role=admin）。路径一律写成字面量 —— 消费面覆盖锁
//     （scripts/check-api-consumers.mjs）只认「第一个实参是字面量」的调用，用 resource 形参
//     拼路径会让锁读不出端点（#1120 销掉它最后 4 条欠条）。
//   - 规格族 6 个资源（tonnages / mast-types / mast-heights / battery-types /
//     transmission-types / engine-types）只有 create / remove：描述符只声明 Create + Delete，
//     PUT 路由从未注册（#1119）；能力面由 SpecCrudEndpoints 钉死，给它们补 update 会编译不过。
//   - original-prices 后端为分页响应 { total, page, page_size, list }，list() 自动解包 .list
//   - 系数表（coefficient-configs）：list 走 /dictionaries，update 走 /admin/coefficient-configs/:key（按 key 单个更新）
//   - 只读字典查询（表单下拉用）在 api/valuation/dictionaries.ts；本模块只管管理端写面 + 列表
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

/** 一个资源的能力面（列表 + 增删改）；路径在 adminResources 里逐资源写死。 */
interface CrudEndpoints<Row = AdminRow> {
  list: (params?: Record<string, unknown>) => Promise<Row[]>
  create: (payload: Record<string, unknown>) => Promise<Row>
  update: (id: AdminResourceId, payload: Record<string, unknown>) => Promise<Row>
  remove: (id: AdminResourceId) => Promise<void>
  /** 从行数据中提取标识符；不存在返回 undefined */
  getIdOf: (row: Row | null | undefined) => AdminResourceId | undefined
}

/** 规格族资源的能力面：描述符只声明 Create + Delete（无 PUT 路由，#1119），故没有 update。 */
type SpecCrudEndpoints<Row = AdminRow> = Pick<CrudEndpoints<Row>, 'create' | 'remove' | 'getIdOf'>

/** 具名方法表的类型面：显式列出 12 个资源，规格族 6 个写不进 update。 */
interface AdminResourceTable {
  originalPrices: CrudEndpoints
  brands: CrudEndpoints
  vehicleTypes: CrudEndpoints
  series: CrudEndpoints
  tonnages: SpecCrudEndpoints
  mastTypes: SpecCrudEndpoints
  mastHeights: SpecCrudEndpoints
  batteryTypes: SpecCrudEndpoints
  transmissionTypes: SpecCrudEndpoints
  engineTypes: SpecCrudEndpoints
  conditionRatings: CrudEndpoints
  regionCoefficients: CrudEndpoints
}

/** 从行数据中提取标识符；不存在返回 undefined（全部资源同构，单点实现）。 */
function getIdOf(row: AdminRow | null | undefined): AdminResourceId | undefined {
  const v = row?.id
  if (typeof v === 'string' || typeof v === 'number') return v
  return undefined
}

/** 列表响应解包：拦截器已解包信封，data 即业务负载；分页资源（original-prices）再解包 .list。 */
function toRows(
  data: AdminRow[] | { list: AdminRow[] } | null | undefined,
  isPaginated: boolean
): AdminRow[] {
  if (isPaginated && data && typeof data === 'object' && 'list' in data) {
    return (data as { list?: AdminRow[] }).list ?? []
  }
  return (data as AdminRow[]) ?? []
}

// ========== 各资源配置 CRUD（路径字面量写死，消费面锁可静态判定）==========
export const adminResources: AdminResourceTable = {
  // original-prices 后端为分页响应，需特殊解包；其余实体列表为扁平数组。
  originalPrices: {
    async list(params?: Record<string, unknown>) {
      const data = await client.get<AdminRow[] | { list: AdminRow[] }>('/dictionaries/original-prices', {
        params: { page: 1, page_size: 100, ...params }
      })
      return toRows(data, true)
    },
    create: (payload: Record<string, unknown>) => client.post<AdminRow>('/admin/original-prices', payload),
    update: (id: AdminResourceId, payload: Record<string, unknown>) =>
      client.put<AdminRow>(`/admin/original-prices/${encodeURIComponent(id)}`, payload),
    async remove(id: AdminResourceId): Promise<void> {
      await client.delete(`/admin/original-prices/${encodeURIComponent(id)}`)
    },
    getIdOf
  },
  brands: {
    async list(params?: Record<string, unknown>) {
      return toRows(await client.get<AdminRow[]>('/dictionaries/brands', { params }), false)
    },
    create: (payload: Record<string, unknown>) => client.post<AdminRow>('/admin/brands', payload),
    update: (id: AdminResourceId, payload: Record<string, unknown>) =>
      client.put<AdminRow>(`/admin/brands/${encodeURIComponent(id)}`, payload),
    async remove(id: AdminResourceId): Promise<void> {
      await client.delete(`/admin/brands/${encodeURIComponent(id)}`)
    },
    getIdOf
  },
  vehicleTypes: {
    async list(params?: Record<string, unknown>) {
      return toRows(await client.get<AdminRow[]>('/dictionaries/vehicle-types', { params }), false)
    },
    create: (payload: Record<string, unknown>) => client.post<AdminRow>('/admin/vehicle-types', payload),
    update: (id: AdminResourceId, payload: Record<string, unknown>) =>
      client.put<AdminRow>(`/admin/vehicle-types/${encodeURIComponent(id)}`, payload),
    async remove(id: AdminResourceId): Promise<void> {
      await client.delete(`/admin/vehicle-types/${encodeURIComponent(id)}`)
    },
    getIdOf
  },
  series: {
    async list(params?: Record<string, unknown>) {
      return toRows(await client.get<AdminRow[]>('/dictionaries/series', { params }), false)
    },
    create: (payload: Record<string, unknown>) => client.post<AdminRow>('/admin/series', payload),
    update: (id: AdminResourceId, payload: Record<string, unknown>) =>
      client.put<AdminRow>(`/admin/series/${encodeURIComponent(id)}`, payload),
    async remove(id: AdminResourceId): Promise<void> {
      await client.delete(`/admin/series/${encodeURIComponent(id)}`)
    },
    getIdOf
  },
  // 规格族：单字段唯一列 + Create/Delete（描述符无 Update ⇒ 无 PUT 路由，#1119）
  tonnages: {
    create: (payload: Record<string, unknown>) => client.post<AdminRow>('/admin/tonnages', payload),
    async remove(id: AdminResourceId): Promise<void> {
      await client.delete(`/admin/tonnages/${encodeURIComponent(id)}`)
    },
    getIdOf
  },
  mastTypes: {
    create: (payload: Record<string, unknown>) => client.post<AdminRow>('/admin/mast-types', payload),
    async remove(id: AdminResourceId): Promise<void> {
      await client.delete(`/admin/mast-types/${encodeURIComponent(id)}`)
    },
    getIdOf
  },
  mastHeights: {
    create: (payload: Record<string, unknown>) => client.post<AdminRow>('/admin/mast-heights', payload),
    async remove(id: AdminResourceId): Promise<void> {
      await client.delete(`/admin/mast-heights/${encodeURIComponent(id)}`)
    },
    getIdOf
  },
  batteryTypes: {
    create: (payload: Record<string, unknown>) => client.post<AdminRow>('/admin/battery-types', payload),
    async remove(id: AdminResourceId): Promise<void> {
      await client.delete(`/admin/battery-types/${encodeURIComponent(id)}`)
    },
    getIdOf
  },
  transmissionTypes: {
    create: (payload: Record<string, unknown>) => client.post<AdminRow>('/admin/transmission-types', payload),
    async remove(id: AdminResourceId): Promise<void> {
      await client.delete(`/admin/transmission-types/${encodeURIComponent(id)}`)
    },
    getIdOf
  },
  engineTypes: {
    create: (payload: Record<string, unknown>) => client.post<AdminRow>('/admin/engine-types', payload),
    async remove(id: AdminResourceId): Promise<void> {
      await client.delete(`/admin/engine-types/${encodeURIComponent(id)}`)
    },
    getIdOf
  },
  conditionRatings: {
    async list(params?: Record<string, unknown>) {
      return toRows(await client.get<AdminRow[]>('/dictionaries/condition-ratings', { params }), false)
    },
    create: (payload: Record<string, unknown>) => client.post<AdminRow>('/admin/condition-ratings', payload),
    update: (id: AdminResourceId, payload: Record<string, unknown>) =>
      client.put<AdminRow>(`/admin/condition-ratings/${encodeURIComponent(id)}`, payload),
    async remove(id: AdminResourceId): Promise<void> {
      await client.delete(`/admin/condition-ratings/${encodeURIComponent(id)}`)
    },
    getIdOf
  },
  regionCoefficients: {
    async list(params?: Record<string, unknown>) {
      return toRows(await client.get<AdminRow[]>('/dictionaries/region-coefficients', { params }), false)
    },
    create: (payload: Record<string, unknown>) => client.post<AdminRow>('/admin/region-coefficients', payload),
    update: (id: AdminResourceId, payload: Record<string, unknown>) =>
      client.put<AdminRow>(`/admin/region-coefficients/${encodeURIComponent(id)}`, payload),
    async remove(id: AdminResourceId): Promise<void> {
      await client.delete(`/admin/region-coefficients/${encodeURIComponent(id)}`)
    },
    getIdOf
  }
}

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
