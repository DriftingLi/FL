// 打卡日历三态判定纯函数单测（spec #599 / #614）
//
// 用例集与移动端 training-app/叉车维修培训学员端跨端应用/utils/checkinCalendar.test.js
// 的「computeDayStates 三态（streak 定段）」逐条互为镜像：同一名、同一输入、同一期望。
// 未来调整三态口径时，两份测试文件必须同改。
import { describe, it, expect } from 'vitest'
import { computeDayStates } from '../checkinCalendar'

const day = (date: string, checked: boolean, points = 0) => ({ date, checked, points })

describe('computeDayStates 三态（streak 定段）', () => {
  it('今日已打卡：streak 段全部实心 streak', () => {
    const days = [
      day('2026-09-01', true, 5),
      day('2026-09-02', true, 10),
      day('2026-09-03', true, 5),
      day('2026-09-04', true, 5),
      day('2026-09-05', true, 5)
    ]
    expect(computeDayStates(days, '2026-09-05', 5)).toEqual(['streak', 'streak', 'streak', 'streak', 'streak'])
  })

  it('今日未打卡：段尾退到昨日定段（streak=1 单日段）', () => {
    const days = [
      day('2026-09-01', true, 5),
      day('2026-09-02', true, 5),
      day('2026-09-03', false),
      day('2026-09-04', true, 5),
      day('2026-09-05', false)
    ]
    // 09-05 未签，段尾退到 09-04，streak=1 → 仅 09-04 在段内
    expect(computeDayStates(days, '2026-09-05', 1)).toEqual(['past', 'past', 'none', 'streak', 'none'])
  })

  it('今日未打卡且段跨多日：段尾=昨日往前 streak 天', () => {
    const days = [
      day('2026-09-01', true, 5),
      day('2026-09-02', true, 5),
      day('2026-09-03', true, 5),
      day('2026-09-04', true, 5),
      day('2026-09-05', false)
    ]
    // 段区间 [09-01, 09-04]
    expect(computeDayStates(days, '2026-09-05', 4)).toEqual(['streak', 'streak', 'streak', 'streak', 'none'])
  })

  it('今日昨日均未签：无连续段（streak=0），历史打卡归 past', () => {
    const days = [day('2026-09-01', true, 5), day('2026-09-04', false), day('2026-09-05', false)]
    expect(computeDayStates(days, '2026-09-05', 0)).toEqual(['past', 'none', 'none'])
  })

  it('跨月段：段起点落在上月，月内段首之前归 past', () => {
    const days = [
      day('2026-08-29', true, 5),
      day('2026-08-30', true, 5),
      day('2026-08-31', true, 5),
      day('2026-09-01', true, 5),
      day('2026-09-02', true, 5)
    ]
    // streak=4，段区间 [08-30, 09-02]，08-29 在段起点之前
    expect(computeDayStates(days, '2026-09-02', 4)).toEqual(['past', 'streak', 'streak', 'streak', 'streak'])
  })

  it('断签中段：断签历史归 past，段内实心', () => {
    const days = [
      day('2026-09-01', true, 5),
      day('2026-09-02', false),
      day('2026-09-03', true, 5),
      day('2026-09-04', true, 15),
      day('2026-09-05', true, 5)
    ]
    // 今日 09-05 已签，streak=3 → 段区间 [09-03, 09-05]；09-01 断签历史浅底
    expect(computeDayStates(days, '2026-09-05', 3)).toEqual(['past', 'none', 'streak', 'streak', 'streak'])
  })

  it('streak=1 边界：今日已签单日段，段外已打卡归 past', () => {
    const days = [day('2026-09-04', true, 5), day('2026-09-05', true, 5)]
    expect(computeDayStates(days, '2026-09-05', 1)).toEqual(['past', 'streak'])
  })

  it('streak=1 边界：今日未签，昨日单日段', () => {
    const days = [day('2026-09-03', true, 5), day('2026-09-04', true, 5), day('2026-09-05', false)]
    expect(computeDayStates(days, '2026-09-05', 1)).toEqual(['past', 'streak', 'none'])
  })

  it('完全无打卡时全部 none', () => {
    const days = [day('2026-09-04', false), day('2026-09-05', false)]
    expect(computeDayStates(days, '2026-09-05', 0)).toEqual(['none', 'none'])
  })

  it('空数组返回空数组', () => {
    expect(computeDayStates([], '2026-09-05', 0)).toEqual([])
  })

  it('兼容 RFC3339 日期输入', () => {
    const days = [day('2026-09-04T00:00:00Z', true, 5), day('2026-09-05', true, 5)]
    expect(computeDayStates(days, '2026-09-05', 2)).toEqual(['streak', 'streak'])
  })
})
