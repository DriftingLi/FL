// 电池 RUL 结果卡的证据面（ADR-0053 §8 票① / spec #1055）：
// RUL 主数字 / SOH / 置信区间 / 置信度四项的展示，以及 SOH 健康度色阶的三档边界
// （≥90 绿 / ≥80 琥珀 / 其余红——边界值必须落到正确档位）。
import { describe, it, expect } from 'vitest'
import { mount } from '@vue/test-utils'
import { epLite } from '@/test/element-lite'
import BatteryResultCard from '../BatteryResultCard.vue'

const props = {
  rulCycles: 3200,
  sohPercent: 92.5,
  confidenceLow: 2900,
  confidenceHigh: 3500,
  confidence: 0.87
}

describe('BatteryResultCard', () => {
  it('RUL 主数字 + 单位 + 置信区间 + 置信度都渲染', () => {
    const w = mount(BatteryResultCard, { props, global: { plugins: [epLite()] } })
    expect(w.find('.battery-result-value').text().replace(/\s+/g, '')).toBe('3200循环')
    const metrics = w.findAll('.metric-value').map((m) => m.text())
    expect(metrics[0]).toBe('92.5%')
    expect(metrics[1]).toBe('2900 ~ 3500')
    expect(metrics[2]).toBe('87.0%')
  })

  it('SOH 色阶三档边界：90 与 80 走高档位', () => {
    const sohColorAt = (soh: number): string => {
      const w = mount(BatteryResultCard, { props: { ...props, sohPercent: soh }, global: { plugins: [epLite()] } })
      return (w.findAll('.metric-value')[0].element as HTMLElement).style.color
    }
    // 三档：>=90 绿 / >=80 琥珀 / 其余红（边界含等号）
    const green = sohColorAt(90)
    const amber = sohColorAt(80)
    const red = sohColorAt(79.9)
    expect(green).not.toBe(amber)
    expect(amber).not.toBe(red)
    expect(green).toBe(sohColorAt(100))
    expect(amber).toBe(sohColorAt(89.9))
    expect(red).toBe(sohColorAt(0))
  })

  it('置信度按 1 位小数百分比展示（0 与 1 两端不出现 NaN）', () => {
    for (const [confidence, want] of [
      [0, '0.0%'],
      [1, '100.0%']
    ] as const) {
      const w = mount(BatteryResultCard, { props: { ...props, confidence }, global: { plugins: [epLite()] } })
      expect(w.findAll('.metric-value')[2].text()).toBe(want)
    }
  })
})
