// 管理员 CRUD 接口：封装 /api/valuation/admin/* 下所有资源配置接口
// 设计说明：每个资源端点（original-prices / brands / ...）CRUD 形态一致，
//         抽象一个 createCrud 工厂避免重复样板；具体字段差异通过 Record<string, unknown> 透传
import client from './client'
import type { CoefficientConfig } from '@/types/valuation/evaluation'

/** 通用资源行：宽松字段，由后端定义具体结构 */
export type AdminRow = Record<string, unknown> & { id?: number }

interface CrudEndpoints {
  list: <T = AdminRow>(params?: Record<string, unknown>) => Promise<T[]>
  create: <T = AdminRow>(payload: Record<string, unknown>) => Promise<T>
  update: <T = AdminRow>(id: number, payload: Record<string, unknown>) => Promise<T>
  remove: (id: number) => Promise<void>
}

/** 创建一个资源的 CRUD 封装（路径：/admin/${resource}） */
function createCrud(resource: string): CrudEndpoints {
  const base = `/admin/${resource}`
  return {
    list<T = AdminRow>(params?: Record<string, unknown>) {
      return client
        .get<unknown, { data: T[] }>(base, { params })
        .then((r) => r.data ?? [])
    },
    create<T = AdminRow>(payload: Record<string, unknown>) {
      return client
        .post<unknown, { data: T }>(base, payload)
        .then((r) => r.data)
    },
    update<T = AdminRow>(id: number, payload: Record<string, unknown>) {
      return client
        .put<unknown, { data: T }>(`${base}/${id}`, payload)
        .then((r) => r.data)
    },
    remove(id: number) {
      return client.delete(`${base}/${id}`).then(() => undefined)
    }
  }
}

// ========== 各资源配置 CRUD（路径：/api/valuation/admin/${resource}） ==========

export const adminResources = {
  originalPrices: createCrud('original-prices'),
  brandTypes: createCrud('brand-types'),
  brands: createCrud('brands'),
  vehicleTypes: createCrud('vehicle-types'),
  series: createCrud('series'),
  tonnages: createCrud('tonnages'),
  configTypes: createCrud('config-types'),
  mastTypes: createCrud('mast-types'),
  mastHeights: createCrud('mast-heights'),
  batteryTypes: createCrud('battery-types'),
  conditionRatings: createCrud('condition-ratings'),
  regionCoefficients: createCrud('region-coefficients')
} as const

export type AdminResourceKey = keyof typeof adminResources

// ========== 系数表（GET + PUT） ==========

/** 拉取算法参数表 */
export async function listAdminCoefficients(): Promise<CoefficientConfig[]> {
  const resp = await client.get<unknown, { data: CoefficientConfig[] }>('/admin/coefficients')
  return resp.data ?? []
}

/** 整体更新算法参数表（PUT 全量替换） */
export async function updateAdminCoefficients(payload: CoefficientConfig[]): Promise<CoefficientConfig[]> {
  const resp = await client.put<unknown, { data: CoefficientConfig[] }>('/admin/coefficients', payload)
  return resp.data ?? []
}
