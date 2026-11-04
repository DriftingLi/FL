// DiagnosisBadges spec（票 2244）：能力徽章行。
//
// 数据来自 v2 的 done 事件 meta.badges（票 2245 已落库），三字段 name/emoji/cls 全部原样透出。
// 断言的是渲染产物：有徽章就出一排、空数组一个节点都不出、未知 cls 不影响可见性 ——
// 后者是对方加词时的兼容承诺（徽章词表在对方侧，加词不该让前端消失）。
import { describe, it, expect } from 'vitest'
import { mount } from '@vue/test-utils'
import DiagnosisBadges from '../DiagnosisBadges.vue'
import type { DiagnosisBadge } from '@/api/aiAssistant'

function badge(over: Partial<DiagnosisBadge> = {}): DiagnosisBadge {
  return { name: '知识检索', emoji: '🔍', cls: 'cap-rag', ...over }
}

function mountBadges(badges: DiagnosisBadge[]) {
  return mount(DiagnosisBadges, { props: { badges } })
}

describe('DiagnosisBadges：渲染与缺席', () => {
  it('有徽章时逐条渲染 name 与 emoji', () => {
    const w = mountBadges([
      badge({ name: '多模态识图', emoji: '📷', cls: 'cap-vision' }),
      badge()
    ])
    const tags = w.findAll('[data-cap]')
    expect(tags).toHaveLength(2)
    expect(w.text()).toContain('多模态识图')
    expect(w.text()).toContain('📷')
    expect(w.text()).toContain('知识检索')
    // cls 落到 data-cap，不参与语义判定（样式分档的另一条路）
    expect(tags[0].attributes('data-cap')).toBe('cap-vision')
  })

  it('空数组：一个节点都不渲染（不占行高）', () => {
    const w = mountBadges([])
    expect(w.findAll('[data-cap]')).toHaveLength(0)
    expect(w.text()).toBe('')
  })

  it('缺省 prop 同样不渲染', () => {
    const w = mount(DiagnosisBadges)
    expect(w.findAll('[data-cap]')).toHaveLength(0)
  })

  it('未知 cls（对方加词）仍渲染，只是没有 data-cap 之外的差异', () => {
    const w = mountBadges([badge({ name: '未来新能力', cls: 'cap-something-new' })])
    expect(w.findAll('[data-cap]')).toHaveLength(1)
    expect(w.text()).toContain('未来新能力')
  })

  it('缺 emoji 时只渲染文字，不留空节点', () => {
    const w = mountBadges([badge({ name: '诚实兜底', emoji: '' })])
    expect(w.text()).toContain('诚实兜底')
    expect(w.find('[aria-hidden="true"]').exists()).toBe(false)
  })
})
