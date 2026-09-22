// PointsLedger 积分明细页三态 + 筛选 + 规则抽屉契约（#512）。
// seam：页面组件层，mock '@/api/points'（不依赖真实后端）。
import { describe, it, expect, vi, beforeEach } from 'vitest'
import { mount, flushPromises } from '@vue/test-utils'
import { epLite } from '@/test/element-lite'

vi.mock('@/api/points', () => ({
  pointsApi: {
    getBalance: vi.fn(),
    getLedger: vi.fn()
  }
}))

import { pointsApi } from '@/api/points'
import UiPagination from '@/components/ui/UiPagination.vue'
import PointsLedger from '../PointsLedger.vue'

function mountPage() {
  return mount(PointsLedger, {
    global: {
      plugins: [epLite()],
      stubs: { RouterLink: { template: '<a><slot /></a>' } }
    }
  })
}

function ledgerItem(over: Record<string, unknown> = {}) {
  return {
    id: 1,
    user_id: 1,
    delta: 10,
    reason: 'task_daily_checkin',
    ref_type: 'task',
    ref_id: '1',
    created_at: '2026-09-01T10:00:00+08:00',
    expires_at: null,
    ...over
  }
}

beforeEach(() => {
  vi.mocked(pointsApi.getBalance).mockResolvedValue({ balance: 100, total_earned: 500, total_spent: 400 })
  vi.mocked(pointsApi.getLedger).mockResolvedValue({
    items: [ledgerItem(), ledgerItem({ id: 2, delta: -30, reason: 'redeem_course' })],
    total: 2,
    page: 1,
    pages: 1
  })
})

describe('PointsLedger 积分明细页（#512）', () => {
  it('渲染账户四格：余额 / 收入 / 支出 / 永久有效徽标', async () => {
    const w = mountPage()
    await flushPromises()
    expect(w.text()).toContain('100')
    expect(w.text()).toContain('+500')
    expect(w.text()).toContain('−400')
    expect(w.text()).toContain('当前永久有效')
  })

  it('渲染流水行：事由 / ±金额', async () => {
    const w = mountPage()
    await flushPromises()
    const text = w.text()
    expect(text).toContain('+10')
    expect(text).toContain('-30')
  })

  it('未收录 reason 显示默认方向文案不崩', async () => {
    vi.mocked(pointsApi.getLedger).mockResolvedValue({
      items: [ledgerItem({ id: 9, delta: -5, reason: 'custom_weird_code' })],
      total: 1, page: 1, pages: 1
    })
    const w = mountPage()
    await flushPromises()
    expect(w.text()).toContain('积分消耗')
  })

  it('筛选切到支出：请求携带 direction=out（后端分页过滤）', async () => {
    const w = mountPage()
    await flushPromises()
    vi.mocked(pointsApi.getLedger).mockClear()
    const btns = w.findAll('[role="tab"]')
    const outTab = btns.find((b) => b.text() === '支出')
    expect(outTab).toBeTruthy()
    await outTab!.trigger('click')
    await flushPromises()
    const lastCall = vi.mocked(pointsApi.getLedger).mock.calls.at(-1)?.[0]
    expect(lastCall).toMatchObject({ direction: 'out' })
  })

  // ADR-0062 决策 10（第十四波 B 票 10）的实测缺陷：本页是全站唯一拿 handlePageChange()
  // 处理筛选切换的页面 —— 它只重装「当前那一页」，于是翻到第 4 页再点「支出」就是
  // 请求第 4 页那个（通常不存在的）子集，屏上「暂无积分流水」而记录确实存在。
  // 正解是声明进 filterDeps（#1054 那条 seam），回第一页重装由 composable 单点负责。
  it('翻到第 4 页再切「支出」：请求回第一页（筛选项变化不得直调 handlePageChange）', async () => {
    vi.mocked(pointsApi.getLedger).mockResolvedValue({
      items: [ledgerItem()],
      total: 100,
      page: 1,
      pages: 5
    })
    const w = mountPage()
    await flushPromises()

    // 真实翻页一步：分页器写回页码 + 发 current-change（与用户点页码等价）
    const pag = w.findComponent(UiPagination)
    pag.vm.$emit('update:currentPage', 4)
    pag.vm.$emit('current-change', 4)
    await flushPromises()
    expect(vi.mocked(pointsApi.getLedger).mock.calls.at(-1)?.[0]).toMatchObject({ page: 4 })

    vi.mocked(pointsApi.getLedger).mockClear()
    const outTab = w.findAll('[role="tab"]').find((b) => b.text() === '支出')
    expect(outTab).toBeTruthy()
    await outTab!.trigger('click')
    await flushPromises()

    const last = vi.mocked(pointsApi.getLedger).mock.calls.at(-1)?.[0]
    expect(last).toMatchObject({ page: 1, direction: 'out' })
    // 回到第一页后确实有记录 —— 不是「暂无积分流水」那个假空态
    expect(w.text()).not.toContain('暂无积分流水')
  })

  it('点「积分规则」打开抽屉', async () => {
    const w = mountPage()
    await flushPromises()
    const btn = w.findAll('button').find((b) => b.text().includes('积分规则'))
    expect(btn).toBeTruthy()
    await btn!.trigger('click')
    await flushPromises()
    // el-drawer append-to-body：内容在 document.body
    expect(document.body.textContent).toContain('如何获得')
  })
})
