/**
 * 残值配置管理页（#1355 的第三处销账）：算法参数装载的写回形状。
 *
 * 这一页是登记表里唯一**不能机械搬**的一处：装载器除成功路径（写四份草稿）外，失败路径也要写页面
 * 状态 —— 「错误态与陈旧数据不同屏」，失败即清草稿。#1355 给 `useAsyncPage` 补了 `onError` 槽
 * （与 `apply` 同一条批次代数判据），本页此后的形状是：loader 只取数并回传，
 * 成功在 `apply` 写草稿、失败在 `onError` 清草稿。
 *
 * 本文件按**行为**钉这三件事（结构与形状由 `composables/__tests__/loadFlowLocks.spec.ts` R4/R4c 管）：
 * ① 成功装载把四份草稿写满；② 装载失败把四份草稿清空并进错误态（清草稿语义没在搬迁中丢掉）；
 * ③ 旧一轮的失败后到时，既不打错误态也不清草稿 —— 那正是 `onError` 必须排在代数校验之后的原因。
 *
 * 判定面走 ElTable 的 `data` prop（jsdom 下 `el-table-column` 的 scoped slot 拿不到 row，
 * 口径同 `student/valuation/__tests__/ValuationHistoryView.spec.ts`）。
 */
import { describe, it, expect, vi, beforeEach } from 'vitest'
import { mount, flushPromises } from '@vue/test-utils'
import { ElTable } from 'element-plus/es/components/table/index.mjs'
import { WarningFilled } from '@element-plus/icons-vue'
import { epLite } from '@/test/element-lite'
import type { AlgorithmParameters } from '@/api/valuation/admin'

const listAlgorithmParameters = vi.fn()
const originalPricesList = vi.fn()

vi.mock('@/api/valuation/admin', () => ({
  listAlgorithmParameters: (...args: unknown[]) => listAlgorithmParameters(...args),
  adminResources: {
    originalPrices: {
      list: (...args: unknown[]) => originalPricesList(...args),
      create: vi.fn(),
      update: vi.fn(),
      remove: vi.fn(),
      getIdOf: (row: Record<string, unknown>) => (row as { id?: number }).id ?? null
    },
    regionCoefficients: { create: vi.fn() }
  },
  updateCoefficient: vi.fn(),
  updateBrandCoefficient: vi.fn(),
  updateConditionCoefficient: vi.fn(),
  updateRegionCoefficient: vi.fn()
}))

import ValuationConfigManage from '../ValuationConfigManage.vue'

/** 四份草稿各一到两行的夹具（含一个 `kc_` 前缀行，覆盖共享 draft 的两个派生视图）。 */
function fixture(): AlgorithmParameters {
  return {
    coefficients: [
      { description: '时间衰减', id: 1, key: 'kt_base', updated_at: '', value: 0.9 },
      { description: '油漆', id: 2, key: 'kc_paint', updated_at: '', value: 0.1 }
    ],
    brands: [{ id: 10, is_active: true, k_brand: 1.1, name: '合力' }],
    condition_ratings: [{ base_coefficient: 1, id: 20, label: '优秀', rating: 'A' }],
    region_coefficients: [{ city: '苏州', coefficient: 1.05, id: 30, province: '江苏' }]
  }
}

/** script setup 的绑定在 dev 模式经 proxy 可访问（口径同 ValuationHistoryView.spec.ts）。 */
const api = (w: ReturnType<typeof mount>) => w.vm as unknown as Record<string, any>

/** 六张表的 data（原价表 + 五个算法分区）：按行数汇总，避免依赖 DOM 版式。 */
function tableData(w: ReturnType<typeof mount>): unknown[][] {
  // ElTable 的复杂泛型签名与 findAllComponents 的重载不完全匹配（断言口径同 ValuationHistoryView.spec.ts）
  return w.findAllComponents(ElTable as never).map(t => (t as any).props('data') as unknown[])
}

/** 算法五区的行数（跳过第 0 张 = 原价表）：只在表格确实渲染时可用。 */
function algorithmCounts(w: ReturnType<typeof mount>): number[] {
  return tableData(w)
    .slice(1)
    .map(rows => rows.length)
}

/**
 * 四份 draft 的行数（全局系数 / 品牌 / 车况 / 车况修正项 / 区域）。
 * 走 script-setup 绑定而不是 DOM：错误态下 `UiAsyncSection` 用错误面顶替整个内容区，
 * 表格随之卸载 —— 但 draft 是页面状态，「有没有被清」只能问它本身（判据 2/3 要看的就是这个）。
 */
function draftCounts(w: ReturnType<typeof mount>): number[] {
  const a = api(w)
  return [
    a.globalCoefficientsDraft.length,
    a.brandsDraft.length,
    a.conditionRatingsDraft.length,
    a.kcModifiersDraft.length,
    a.regionCoefficientsDraft.length
  ]
}

async function mountOnAlgorithmTab() {
  const w = mount(ValuationConfigManage, {
    global: {
      plugins: [epLite()],
      // UiErrorState 的图标在生产由 main.ts 全局注册；错误态用例需要它，否则 vue 报 unresolved
      components: { WarningFilled },
      stubs: { 'el-table-column': true }
    }
  })
  await flushPromises()
  // 进入「算法参数」tab：装载入口在此触发（页面按需只装当前 tab）
  api(w).onTabChange('algorithm')
  return w
}

beforeEach(() => {
  vi.clearAllMocks()
  originalPricesList.mockResolvedValue([{ id: 1, brand: '合力', original_price: 100 }])
  listAlgorithmParameters.mockResolvedValue(fixture())
})

describe('ValuationConfigManage 算法参数装载（#1355 写回槽）', () => {
  it('成功装载：loader 回传的数据由 apply 槽写满四份草稿并渲染进表格', async () => {
    const w = await mountOnAlgorithmTab()
    await flushPromises()

    // 全局系数 1 行（kc_ 那行归修正项区）/ 品牌 1 / 车况 1 / 车况修正项 1 / 区域 1
    expect(draftCounts(w)).toEqual([1, 1, 1, 1, 1])
    expect(algorithmCounts(w)).toEqual([1, 1, 1, 1, 1])
    expect(w.text()).not.toContain('算法参数加载失败')
  })

  it('装载失败：清四份草稿并进错误态（「错误态与陈旧数据不同屏」这条语义没在搬迁中丢掉）', async () => {
    listAlgorithmParameters.mockRejectedValue(new Error('boom'))
    const w = await mountOnAlgorithmTab()
    await flushPromises()

    expect(w.text()).toContain('算法参数加载失败')
    expect(draftCounts(w)).toEqual([0, 0, 0, 0, 0])
  })

  it('先成功后失败：第二次的失败把上一次的草稿清空（错误态与陈旧数据不同屏）', async () => {
    const w = await mountOnAlgorithmTab()
    await flushPromises()
    expect(draftCounts(w)).toEqual([1, 1, 1, 1, 1])

    listAlgorithmParameters.mockRejectedValue(new Error('boom'))
    await api(w).loadAlgorithmParams()
    await flushPromises()

    expect(w.text()).toContain('算法参数加载失败')
    expect(draftCounts(w)).toEqual([0, 0, 0, 0, 0])
  })

  it('旧一轮的失败后到（新一轮已成功）：不打错误态、不清新草稿 —— onError 槽排在代数校验之后', async () => {
    let rejectStale!: (e: unknown) => void
    const stale = new Promise<never>((_resolve, reject) => {
      rejectStale = reject
    })
    let calls = 0
    listAlgorithmParameters.mockImplementation(() => {
      calls++
      return calls === 1 ? stale : Promise.resolve(fixture())
    })

    const w = await mountOnAlgorithmTab() // 第 1 轮：挂在 stale 上
    await flushPromises()
    // 第 2 轮：先成功落地，草稿写满
    await api(w).loadAlgorithmParams()
    await flushPromises()
    expect(draftCounts(w)).toEqual([1, 1, 1, 1, 1])

    // 第 1 轮此刻才失败：本轮已作废，错误态与清草稿都不许执行
    // （这条判据的「修复前形状照样抹掉新草稿」反例自检在 useAsyncPage.spec.ts 的 onError 组里，
    //  那里用的是真 composable —— 本文件不另造 shim，免得测的是 shim 而不是实现）
    rejectStale(new Error('boom'))
    await flushPromises()
    expect(w.text()).not.toContain('算法参数加载失败')
    expect(draftCounts(w)).toEqual([1, 1, 1, 1, 1])
  })
})
