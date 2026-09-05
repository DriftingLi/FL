// CheckInPage 每日打卡页契约（ADR-0028）：
// - 概览条渲染连续/累计/今日状态；
// - 已打卡格与连续段格渲染对应色块 class（连续中实心 / 已断开浅底）；
// - 今日已打卡后按钮不再触发打卡请求（幂等）。
// seam：页面组件层，mock '@/api/checkin'（不依赖真实后端）。
import { describe, it, expect, vi, beforeEach } from 'vitest'
import { mount, flushPromises } from '@vue/test-utils'
import ElementPlus from 'element-plus'

vi.mock('@/api/checkin', () => ({
  checkInApi: {
    checkIn: vi.fn(),
    getCalendar: vi.fn(),
    getRank: vi.fn()
  }
}))

import { checkInApi } from '@/api/checkin'
import { shanghaiDateStr } from '@/utils/format'
import CheckInPage from '../CheckInPage.vue'

function shanghaiNow(): { y: number; m: number; today: string } {
  const today = shanghaiDateStr(new Date())
  const [y, m] = today.split('-').map(Number)
  return { y, m, today }
}

function dateStr(y: number, m: number, d: number): string {
  return `${y}-${String(m).padStart(2, '0')}-${String(d).padStart(2, '0')}`
}

async function mountPage() {
  const wrapper = mount(CheckInPage, {
    global: {
      plugins: [ElementPlus],
      stubs: { RouterLink: { template: '<a><slot /></a>' } }
    }
  })
  await flushPromises()
  return wrapper
}

beforeEach(() => {
  vi.mocked(checkInApi.getRank).mockResolvedValue({ items: [], total: 0, page: 1, pages: 0, me: null })
  vi.mocked(checkInApi.checkIn).mockReset()
})

describe('CheckInPage 打卡页（ADR-0028）', () => {
  it('概览条渲染连续/累计，今日未打卡时按钮可打卡', async () => {
    const { y, m } = shanghaiNow()
    const d1 = dateStr(y, m, 1)
    vi.mocked(checkInApi.getCalendar).mockResolvedValue({
      days: [{ date: d1, checked: true, points: 5 }],
      streak: 1,
      total: 1,
      today_checked: false
    })
    const wrapper = await mountPage()
    expect(wrapper.text()).toContain('连续打卡')
    expect(wrapper.text()).toContain('立即打卡')
    expect(wrapper.text()).not.toContain('今日已打卡')
  })

  it('今日已打卡：显示 +5 且点击不再发起打卡请求（幂等）', async () => {
    const { today } = shanghaiNow()
    vi.mocked(checkInApi.getCalendar).mockResolvedValue({
      days: [{ date: today, checked: true, points: 5 }],
      streak: 1,
      total: 1,
      today_checked: true
    })
    const wrapper = await mountPage()
    expect(wrapper.text()).toContain('今日已打卡')
    // 点击主打卡按钮不触发请求（disabled 由组件状态驱动）
    const doneBtn = wrapper.findAll('button').find((b) => b.text().includes('今日已打卡'))
    expect(doneBtn).toBeTruthy()
    await doneBtn!.trigger('click')
    await flushPromises()
    expect(checkInApi.checkIn).not.toHaveBeenCalled()
  })

  it('连续段内已打卡格渲染主色实心 class；段外历史打卡渲染浅底 class', async () => {
    const { y, m, today } = shanghaiNow()
    const day = Number(today.split('-')[2])
    if (day < 3) return // 今天太靠月初，无法构造「月内孤立历史 + 连续段」场景，跳过
    const d1 = dateStr(y, m, 1) // 月内第 1 天：孤立历史打卡（与今天不连续）
    const dPrev = dateStr(y, m, day - 1)
    const mockDays = [
      { date: d1, checked: true, points: 5 },
      { date: dPrev, checked: true, points: 5 },
      { date: today, checked: true, points: 5 }
    ]
    vi.mocked(checkInApi.getCalendar).mockResolvedValue({
      days: mockDays,
      streak: 2,
      total: 3,
      today_checked: true
    })
    const wrapper = await mountPage()
    const cellByDay = (d: number) =>
      wrapper.findAll('div.relative').find((cell) => {
        const num = cell.find('span.font-medium')
        return num.exists() && num.text() === String(d)
      })
    // 今天（连续段末端）为实心高亮 bg-ui-600
    const todayCell = cellByDay(day)
    expect(todayCell).toBeTruthy()
    expect(todayCell!.classes()).toContain('bg-ui-600')
    // 月内第 1 天（已断开的历史打卡）为浅底 bg-ui-100，非实心
    const histCell = cellByDay(1)
    expect(histCell).toBeTruthy()
    expect(histCell!.classes()).toContain('bg-ui-100')
    expect(histCell!.classes()).not.toContain('bg-ui-500')
    expect(histCell!.classes()).not.toContain('bg-ui-600')
  })

  it('打卡成功：请求被调用并提示积分入账', async () => {
    const { y, m } = shanghaiNow()
    const d1 = dateStr(y, m, 1)
    vi.mocked(checkInApi.getCalendar).mockResolvedValue({
      days: [{ date: d1, checked: true, points: 5 }],
      streak: 1,
      total: 1,
      today_checked: false
    })
    vi.mocked(checkInApi.checkIn).mockResolvedValue({
      checked: true, streak: 2, total: 2, today_checked: true, points: 5
    })
    const wrapper = await mountPage()
    const btn = wrapper.findAll('button').find((b) => b.text().includes('立即打卡'))
    expect(btn).toBeTruthy()
    await btn!.trigger('click')
    await flushPromises()
    expect(checkInApi.checkIn).toHaveBeenCalledTimes(1)
  })
})
