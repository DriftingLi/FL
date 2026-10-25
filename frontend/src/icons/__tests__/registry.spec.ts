import { describe, expect, it } from 'vitest'
import { FALLBACK_ICON, ICONS, hasIcon, resolveIcon } from '../registry'

/**
 * 注册表是「名字 → 图形数据」的单一事实源：这里锁的是**数据完整性**，
 * 不是某个图标长什么样（图形本身来自上游库，改了上游由评审与 B 票的截图接住）。
 */
describe('图标注册表', () => {
  const entries = Object.entries(ICONS)

  it('至少登记了主题三态与降级图标', () => {
    expect(Object.keys(ICONS)).toEqual(expect.arrayContaining(['sun', 'moon', 'theme-system', FALLBACK_ICON]))
  })

  it('每个条目的图形非空，且元素标签都有名字', () => {
    for (const [name, node] of entries) {
      expect(node.length, `${name} 没有图形`).toBeGreaterThan(0)
      for (const [tag, attrs] of node) {
        expect(typeof tag, `${name} 的元素标签必须是字符串`).toBe('string')
        expect(tag.length).toBeGreaterThan(0)
        expect(attrs).toBeTypeOf('object')
      }
    }
  })

  it('名字一律 kebab-case（小写字母、数字、连字符）', () => {
    for (const name of Object.keys(ICONS)) {
      expect(name, name).toMatch(/^[a-z][a-z0-9]*(-[a-z0-9]+)*$/)
    }
  })

  it('降级目标本身一定存在（否则降级会二次崩）', () => {
    expect(hasIcon(FALLBACK_ICON)).toBe(true)
    expect(ICONS[FALLBACK_ICON].length).toBeGreaterThan(0)
  })

  it('未登记的名字回落到降级图标，而不是抛错', () => {
    const unknown = resolveIcon('not-a-real-icon' as never)
    expect(unknown.fallback).toBe(true)
    expect(unknown.node).toBe(ICONS[FALLBACK_ICON])
  })

  it('已登记的名字不会误报 fallback', () => {
    const sun = resolveIcon('sun')
    expect(sun.fallback).toBe(false)
    expect(sun.node.length).toBeGreaterThan(0)
  })
})
