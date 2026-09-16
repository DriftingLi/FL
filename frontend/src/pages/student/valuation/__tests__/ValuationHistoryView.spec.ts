// 评估历史列表的证据面（ADR-0053 §8 票① / spec #1055）：
// 分页参数拼装 / 筛选变化回第一页并重装一次 / 失败时清空列表（拦截器已提示）。
//
// 判定面刻意避开表格渲染：`el-table-column` 的 scoped slot 在 jsdom 下拿不到 row，
// 而本票要钉的是**编排**（页码与请求参数），不是表格版式——故把列 stub 掉，
// 行数据经 ElTable 的 data prop 断言。
import { describe, it, expect, vi, beforeEach } from 'vitest'
import { mount, flushPromises } from '@vue/test-utils'
import { ElTable } from 'element-plus/es/components/table/index.mjs'
import { epLite } from '@/test/element-lite'

const listEvaluations = vi.fn()
vi.mock('@/api/valuation/evaluation', () => ({
  listEvaluations: (...args: unknown[]) => listEvaluations(...args)
}))

const push = vi.fn()
vi.mock('vue-router', () => ({
  useRouter: () => ({ push })
}))

import ValuationHistoryView from '../ValuationHistoryView.vue'

const row = (id: number) => ({
  id,
  brand: '合力',
  vehicle_type: '内燃叉车',
  series: 'H系列',
  tonnage: 3,
  condition_rating: 'A',
  estimated_value: 120000,
  original_price: 300000,
  created_at: '2026-09-16T10:00:00+08:00'
})

const MOUNT_OPTS = {
  global: {
    plugins: [epLite()],
    stubs: { 'el-table-column': true }
  }
}

async function mountView(items = [row(1)], total = 1) {
  listEvaluations.mockResolvedValue({ list: items, total })
  const w = mount(ValuationHistoryView, MOUNT_OPTS)
  await flushPromises()
  return w
}

/** 页面内部编排入口（script setup 的绑定在 dev 模式经 proxy 可访问） */
const api = (w: ReturnType<typeof mount>) => w.vm as unknown as Record<string, any>

beforeEach(() => {
  listEvaluations.mockReset()
  push.mockReset()
})

describe('ValuationHistoryView', () => {
  it('首屏按第 1 页 + 每页 10 条拉取，行数据进表格', async () => {
    const w = await mountView()
    expect(listEvaluations).toHaveBeenCalledTimes(1)
    expect(listEvaluations.mock.calls[0][0]).toEqual({ page: 1, page_size: 10 })
    expect((w.findComponent(ElTable as never) as any).props('data')).toHaveLength(1)
  })

  it('切换筛选条件：回到第一页并只重装一次（不在越界页上查询）', async () => {
    const w = await mountView()
    ;(api(w).onPageChange as (p: number) => void)(3)
    await flushPromises()
    expect(listEvaluations.mock.calls[1][0]).toEqual({ page: 3, page_size: 10 })

    listEvaluations.mockClear()
    // 筛选轴直接设值（el-select 的 v-model 目标）后再走 onFilterChange
    api(w).filterVehicleType = '内燃叉车'
    ;(api(w).onFilterChange as () => void)()
    await flushPromises()
    expect(listEvaluations).toHaveBeenCalledTimes(1)
    expect(listEvaluations.mock.calls[0][0]).toEqual({
      page: 1,
      page_size: 10,
      vehicle_type: '内燃叉车'
    })
  })

  it('筛选值为空时不拼装该查询参数', async () => {
    const w = await mountView()
    listEvaluations.mockClear()
    api(w).filterBrand = ''
    api(w).filterVehicleType = ''
    ;(api(w).onFilterChange as () => void)()
    await flushPromises()
    expect(listEvaluations.mock.calls[0][0]).toEqual({ page: 1, page_size: 10 })
  })

  it('改变每页条数：回第一页并带上新 page_size', async () => {
    const w = await mountView()
    listEvaluations.mockClear()
    ;(api(w).onSizeChange as (s: number) => void)(50)
    await flushPromises()
    expect(listEvaluations.mock.calls[0][0]).toEqual({ page: 1, page_size: 50 })
  })

  it('拉取失败：列表清空、不抛错（错误提示由拦截器统一处理）', async () => {
    listEvaluations.mockRejectedValue(new Error('boom'))
    const w = mount(ValuationHistoryView, MOUNT_OPTS)
    await flushPromises()
    expect(api(w).list).toEqual([])
    expect(api(w).total).toBe(0)
  })
})
