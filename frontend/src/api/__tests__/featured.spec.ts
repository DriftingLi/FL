// featured.ts 分类字典契约（#2063）：六态标签齐、未知类别回落原值（不再静默冒充「政策法规」）。
import { describe, it, expect, vi } from 'vitest'

vi.mock('@/api/request', () => ({
  unwrappedRequest: { get: vi.fn(), post: vi.fn(), put: vi.fn(), delete: vi.fn() }
}))
vi.mock('@/api/client', () => ({ getValidAccessToken: vi.fn() }))

import { categoryLabel, featuredCategoryLabels, featuredCategoryOptions } from '../featured'

describe('内容精选分类字典（#2063）', () => {
  it('六态标签齐：旧四态一字不动 + 考情 / 公告', () => {
    expect(featuredCategoryLabels).toEqual({
      company: '公司动态',
      industry: '行业新闻',
      product: '产品资讯',
      news: '政策法规',
      exam: '考情',
      notice: '公告'
    })
  })

  it('管理端下拉选项与字典同源（六项）', () => {
    expect(featuredCategoryOptions).toHaveLength(6)
    for (const opt of featuredCategoryOptions) {
      expect(opt.label).toBe(featuredCategoryLabels[opt.value])
    }
  })

  it('categoryLabel 对新类别给出正确中文', () => {
    expect(categoryLabel('exam')).toBe('考情')
    expect(categoryLabel('notice')).toBe('公告')
  })

  it('未知类别回落原值，不再静默显示成「政策法规」', () => {
    expect(categoryLabel('legacy_cat')).toBe('legacy_cat')
    expect(categoryLabel('')).toBe('')
  })
})
