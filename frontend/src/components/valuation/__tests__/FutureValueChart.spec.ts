// 未来价值图表的证据面（ADR-0053 §8 票① / spec #1055）。
//
// 本域此前**没有任何自动化测试**（全站唯一 0 spec 的域），而它恰好是「前后端口径耦合最紧」
// 的一段。这里固定一条不变量：**衰减公式的唯一实现在后端**，前端只按后端下发的锚点做
// `基准 × 锚点^n` 的乘法与展示派生（年衰减率 = 1 − 锚点）。后来者若把公式搬回前端，本用例报红。
import { describe, it, expect, vi, beforeEach } from 'vitest'
import { mount } from '@vue/test-utils'

// ECharts 在 jsdom 下无 canvas：替身掉组合式封装，只捕获它收到的 option（断言取数，不断言版式）
const initSpy = vi.fn()
vi.mock('@/composables/useECharts', () => ({
  useECharts: () => ({ init: initSpy })
}))

import FutureValueChart from '../FutureValueChart.vue'

function lastOption(): any {
  expect(initSpy).toHaveBeenCalled()
  return initSpy.mock.calls[initSpy.mock.calls.length - 1][0]
}

beforeEach(() => {
  initSpy.mockClear()
})

describe('FutureValueChart', () => {
  it('点数序列 = 基准 × 锚点^n（含当前年，共 years+1 个点）', () => {
    mount(FutureValueChart, {
      props: { estimatedValue: 100000, decayAnchor: 0.8, years: 3 }
    })
    const series = lastOption().series[0].data
    expect(series).toHaveLength(4)
    const values = series.map((d: any) => d.value)
    // 逐点等于 base × anchor^n（用容差比：Math.pow 有浮点尾差，如 0.8^2 = 0.6400000000000001
    // → 64000.000000000015。那是乘法的原样结果，不是前端做过的加工——所以既断言「等于乘法」
    // 也断言「没有四舍五入/取整」）
    values.forEach((v: number, n: number) => {
      expect(v).toBeCloseTo(100000 * Math.pow(0.8, n), 6)
    })
    expect(values[2]).not.toBe(64000) // 原样保留浮点结果（若被加工成整数，这条会红）
  })

  it('X 轴标签：首点「当前」，其余按 saleYear 或相对年', () => {
    const withSaleYear = mount(FutureValueChart, {
      props: { estimatedValue: 100000, decayAnchor: 0.9, years: 2, saleYear: 2026 }
    })
    expect(lastOption().xAxis.data).toEqual(['当前', '2027年', '2028年'])
    withSaleYear.unmount()

    mount(FutureValueChart, { props: { estimatedValue: 100000, decayAnchor: 0.9, years: 2 } })
    expect(lastOption().xAxis.data).toEqual(['当前', '+1年', '+2年'])
  })

  it('首点用「当前」色，其余用主色（视觉口径：只有当前年是浅色）', () => {
    mount(FutureValueChart, { props: { estimatedValue: 100000, decayAnchor: 0.5, years: 2 } })
    const colors = lastOption().series[0].data.map((d: any) => d.itemStyle.color)
    expect(colors[0]).not.toBe(colors[1])
    expect(colors[1]).toBe(colors[2])
  })

  it('副标题的年衰减率是展示派生（1 − 锚点），不是重算的领域公式', () => {
    const w = mount(FutureValueChart, { props: { estimatedValue: 100000, decayAnchor: 0.75 } })
    expect(w.find('.decay-rate').text()).toBe('25.0%')
    // 锚点为 1（不衰减）时派生值为 0，不会出现负数
    const w2 = mount(FutureValueChart, { props: { estimatedValue: 100000, decayAnchor: 1 } })
    expect(w2.find('.decay-rate').text()).toBe('0.0%')
  })

  it('锚点 > 1 时派生值下钳到 0（不做负数展示）', () => {
    const w = mount(FutureValueChart, { props: { estimatedValue: 100000, decayAnchor: 1.2 } })
    expect(w.find('.decay-rate').text()).toBe('0.0%')
  })

  it('估值不为负：乘法结果下钳到 0', () => {
    mount(FutureValueChart, { props: { estimatedValue: -100, decayAnchor: 0.5, years: 1 } })
    expect(lastOption().series[0].data.map((d: any) => d.value)).toEqual([0, 0])
  })

  it('height 透传为画布样式（默认 320px）', () => {
    const w = mount(FutureValueChart, {
      props: { estimatedValue: 1, decayAnchor: 0.9, height: '420px' }
    })
    expect((w.find('.chart-canvas').element as HTMLElement).style.height).toBe('420px')
    const w2 = mount(FutureValueChart, { props: { estimatedValue: 1, decayAnchor: 0.9 } })
    expect((w2.find('.chart-canvas').element as HTMLElement).style.height).toBe('320px')
  })
})
