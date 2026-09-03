// useRoleDashboard：学员/导师工作台 time-series 统计 section 的接口级测试。
// seam：composable 接口——statsFetcher 用内存 fixture，不触达 API 层。
// #506：mock useECharts 观测渲染触发——「容器晚于数据挂载」（首屏骨架包裹图表）自动补渲染。
import { describe, it, expect, vi, beforeEach } from 'vitest'
import { nextTick } from 'vue'
import { useRoleDashboard } from '@/composables/useRoleDashboard'
import type { RoleDashboardStats } from '@/composables/useRoleDashboard'

const { initSpy } = vi.hoisted(() => ({ initSpy: vi.fn() }))

vi.mock('@/composables/useECharts', () => ({
  useECharts: vi.fn(() => ({ init: initSpy }))
}))

const flushPromises = () => new Promise((resolve) => setTimeout(resolve, 0))

function makeTabs() {
  return [
    { label: '近7天', value: '7d', days: 7 },
    { label: '近30天', value: '30d', days: 30 }
  ]
}

function makeStats(data: number[]): RoleDashboardStats {
  return {
    days: data.length,
    labels: data.map((_, i) => `d${i}`),
    data,
    total: data.reduce((a, b) => a + b, 0),
    active_days: data.filter((v) => v > 0).length
  }
}

function makeDashboard() {
  return useRoleDashboard({
    statsFetcher: async () => makeStats([1, 0, 2]),
    seriesType: 'line',
    unit: '分钟',
    yAxisName: '分钟',
    summaryText: (s) => `共 ${s.total} 分钟`,
    timeTabs: makeTabs()
  })
}

beforeEach(() => {
  initSpy.mockClear()
})

describe('useRoleDashboard（统计 section 收敛）', () => {
  it('默认选中第一个 tab，fetcher 收到对应天数', async () => {
    const seen: number[] = []
    const d = useRoleDashboard({
      statsFetcher: async (days) => {
        seen.push(days)
        return makeStats([1, 0, 2])
      },
      seriesType: 'line',
      unit: '分钟',
      yAxisName: '分钟',
      summaryText: (s) => `共 ${s.total} 分钟`,
      timeTabs: makeTabs()
    })

    expect(d.currentTab.value).toBe('7d')
    await d.loadStats()
    expect(seen).toEqual([7])
    expect(d.stats.value?.total).toBe(3)
    expect(d.summary.value).toBe('共 3 分钟')
    expect(d.statsEmpty.value).toBe(false)
  })

  it('切换 tab 后按新天数重新拉取并重绘', async () => {
    const seen: number[] = []
    const d = useRoleDashboard({
      statsFetcher: async (days) => {
        seen.push(days)
        return makeStats([5])
      },
      seriesType: 'bar',
      unit: '题',
      yAxisName: '题数',
      summaryText: (s) => `共 ${s.total} 题`,
      timeTabs: makeTabs()
    })
    // 真实页面切 tab 时容器已挂载（首屏装载后）
    d.chartRef.value = document.createElement('div')
    await d.loadStats()
    expect(initSpy).toHaveBeenCalledTimes(1)

    d.currentTab.value = '30d'
    await flushPromises()
    expect(seen).toEqual([7, 30])
    expect(initSpy).toHaveBeenCalledTimes(2)
  })

  it('数据全 0 视为空态，不渲染图表', async () => {
    const d = useRoleDashboard({
      statsFetcher: async () => makeStats([0, 0, 0]),
      seriesType: 'line',
      unit: '分钟',
      yAxisName: '分钟',
      summaryText: (s) => `共 ${s.total} 分钟`,
      timeTabs: makeTabs()
    })

    await d.loadStats()
    expect(d.statsEmpty.value).toBe(true)
    expect(initSpy).not.toHaveBeenCalled()
  })

  it('fetcher 返回 null 时 stats 置空（错误/空响应兜底）', async () => {
    const d = useRoleDashboard({
      statsFetcher: async () => null,
      seriesType: 'line',
      unit: '分钟',
      yAxisName: '分钟',
      summaryText: (s) => `共 ${s.total} 分钟`,
      timeTabs: makeTabs()
    })

    await d.loadStats()
    expect(d.stats.value).toBeNull()
    expect(d.summary.value).toBe('')
    expect(d.statsEmpty.value).toBe(true)
  })

  it('#506 容器晚于数据挂载（首屏骨架）自动补渲染', async () => {
    const d = makeDashboard()
    // 首屏骨架态：chartRef 尚为 null
    // 数据先到位
    await d.loadStats()
    expect(d.stats.value).not.toBeNull()
    expect(initSpy).not.toHaveBeenCalled()
    // 骨架消失、容器挂载
    d.chartRef.value = document.createElement('div')
    await nextTick()
    expect(initSpy).toHaveBeenCalledTimes(1)
  })
})
