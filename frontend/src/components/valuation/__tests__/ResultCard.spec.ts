// 整机残值结果卡的证据面（ADR-0053 §8 票① / spec #1055）。
//
// 断言对外可见的渲染结果（数值与文案），不断言内部类名结构；零 props 的边界（原价为 0）
// 必须不崩、且残值率不出现 Infinity/NaN。
import { describe, it, expect } from 'vitest'
import { mount } from '@vue/test-utils'
import ResultCard from '../ResultCard.vue'

describe('ResultCard', () => {
  it('主数字按万元展示（2 位小数）+ 单位', () => {
    const w = mount(ResultCard, {
      props: { estimatedValue: 123456, confidenceLow: 110000, confidenceHigh: 135000, originalPrice: 300000 }
    })
    expect(w.find('.result-card-value').text().replace(/\s+/g, '')).toBe('12.35万元')
  })

  it('置信下限 / 上限 / 残值率三个维度都渲染', () => {
    const w = mount(ResultCard, {
      props: { estimatedValue: 150000, confidenceLow: 130000, confidenceHigh: 170000, originalPrice: 300000 }
    })
    const metrics = w.findAll('.metric-value').map((m) => m.text())
    expect(metrics).toEqual(['13.00 万元', '17.00 万元', '50.0%'])
  })

  it('原价为 0 或缺省时残值率为 0%（不出现 Infinity / NaN）', () => {
    for (const originalPrice of [0, -1]) {
      const w = mount(ResultCard, {
        props: { estimatedValue: 150000, confidenceLow: 1, confidenceHigh: 2, originalPrice }
      })
      const rate = w.findAll('.metric-value')[2].text()
      expect(rate).toBe('0.0%')
      expect(rate).not.toMatch(/Infinity|NaN/)
    }
  })

  it('原价与残值相等 → 残值率 100%', () => {
    const w = mount(ResultCard, {
      props: { estimatedValue: 300000, confidenceLow: 1, confidenceHigh: 2, originalPrice: 300000 }
    })
    expect(w.findAll('.metric-value')[2].text()).toBe('100.0%')
  })

  it('缺字段（undefined）时不崩：金额位走占位符', () => {
    const w = mount(ResultCard, {
      props: { estimatedValue: 0, confidenceLow: undefined as any, confidenceHigh: undefined as any, originalPrice: 0 }
    })
    expect(w.findAll('.metric-value')[0].text()).toBe('-')
    expect(w.findAll('.metric-value')[1].text()).toBe('-')
    expect(w.findAll('.metric-value')[2].text()).toBe('0.0%')
  })
})
