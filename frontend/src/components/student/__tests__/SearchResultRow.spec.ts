// 结果行（ADR-0049 决策 6）：命中片段走 ADR-0044 的纯文本投影，高亮在投影后的文本上重定位。
import { describe, expect, it } from 'vitest'
import { mount } from '@vue/test-utils'
import SearchResultRow from '../SearchResultRow.vue'

function item(over: Record<string, unknown> = {}) {
  return {
    type: 'topic',
    id: 1,
    title: '变速箱异响',
    cover: '',
    summary: '开头截断的摘要',
    snippet: '',
    hit_field: 'body',
    parent_id: 0,
    ...over
  } as never
}

describe('SearchResultRow 命中片段与高亮', () => {
  it('markdown 标记被投影掉，不把 ** 直接摆给学员', () => {
    const w = mount(SearchResultRow, {
      props: { item: item({ snippet: '**液压**泵压力不足' }), keyword: '液压' }
    })
    expect(w.text()).toContain('液压泵压力不足')
    expect(w.text()).not.toContain('**')
  })

  it('命中词在投影后的文本上被高亮（mark）', () => {
    const w = mount(SearchResultRow, {
      props: { item: item({ snippet: '前文…液压泵压力不足…后文' }), keyword: '液压' }
    })
    const marks = w.findAll('mark')
    expect(marks.length).toBeGreaterThan(0)
    expect(marks.some((m) => m.text() === '液压')).toBe(true)
  })

  it('命中在回复时明确标注（否则学员点进去找不到关键词）', () => {
    const w = mount(SearchResultRow, {
      props: { item: item({ hit_field: 'reply', snippet: '回复里提到液压泵' }), keyword: '液压' }
    })
    expect(w.text()).toContain('命中在回复')
  })

  it('无 snippet 时退回 summary（老客户端口径仍可用）', () => {
    const w = mount(SearchResultRow, {
      props: { item: item({ snippet: '', summary: '摘要兜底文本' }), keyword: '不存在' }
    })
    expect(w.text()).toContain('摘要兜底文本')
  })
})
