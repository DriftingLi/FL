// 导航的能力过滤（#1618 段1）：无权限的叶子不进侧栏，因此变空的分组一并丢弃。
import { describe, it, expect } from 'vitest'
import { buildNavigation, filterNavByCapability, type NavItem } from '@/config/navigation'

function collectLeaves(items: NavItem[]): NavItem[] {
  const out: NavItem[] = []
  for (const item of items) {
    if (item.children?.length) out.push(...collectLeaves(item.children))
    else if (item.routeName) out.push(item)
  }
  return out
}

describe('filterNavByCapability', () => {
  it('管理端导航项携带页面描述符的能力位（否则过滤无从判起）', () => {
    const leaves = collectLeaves(buildNavigation('manage'))
    expect(leaves.length).toBeGreaterThan(0)
    expect(leaves.filter(l => !l.capability)).toEqual([])
  })

  it('能力全不命中 → 整棵导航为空（分组因变空被丢弃）', () => {
    expect(filterNavByCapability(buildNavigation('manage'), () => false)).toEqual([])
  })

  it('只放行一个能力 → 只剩具备该能力的项，且不残留空分组', () => {
    const nav = buildNavigation('manage')
    const target = collectLeaves(nav)[0].capability!
    // 注意：**多个页面可以共享同一能力位**（管理端有 5 个页面都要求 admin.access），
    // 故判据是「剩下的都具备它、且数量等于原树中具备它的叶子数」，不是「只剩一项」。
    const expected = collectLeaves(nav).filter(l => l.capability === target).length
    const filtered = filterNavByCapability(nav, cap => cap === target)
    const leaves = collectLeaves(filtered)
    expect(leaves.length).toBe(expected)
    expect(leaves.filter(l => l.capability !== target)).toEqual([])
    // 分组要么不存在，要么仍有子项 —— 不留空壳
    for (const group of filtered) {
      if (group.children) expect(group.children.length).toBeGreaterThan(0)
    }
  })

  it('过滤不改动原树（纯函数，调用方可重复用同一份 roleNavigation）', () => {
    const nav = buildNavigation('manage')
    const before = collectLeaves(nav).length
    filterNavByCapability(nav, () => false)
    expect(collectLeaves(nav).length).toBe(before)
  })
})
