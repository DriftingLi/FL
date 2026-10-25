import { describe, expect, it, vi, beforeEach, afterEach } from 'vitest'
import { mount } from '@vue/test-utils'
import UiIcon from '../UiIcon.vue'

/**
 * `UiIcon` 的**挂载契约**（外部行为）：渲染成什么样、多大、怎么被读屏念、
 * 未知名怎么办、静态路径会不会顺手把动画模块拉下来。
 */
const { morphModuleFactory } = vi.hoisted(() => ({ morphModuleFactory: vi.fn() }))

vi.mock('../UiIconMorph.vue', () => {
  morphModuleFactory()
  return { default: { name: 'UiIconMorphStub', props: ['icon'], template: '<svg data-morph-stub="1" />' } }
})

describe('UiIcon（静态）', () => {
  let warn: ReturnType<typeof vi.spyOn>

  beforeEach(() => {
    morphModuleFactory.mockClear()
    warn = vi.spyOn(console, 'warn').mockImplementation(() => {})
  })

  afterEach(() => {
    warn.mockRestore()
  })

  it('默认按字号渲染（1em），并落在 24 网格视图框内', () => {
    const w = mount(UiIcon, { props: { name: 'sun' } })
    const svg = w.find('svg')
    expect(svg.exists()).toBe(true)
    expect(svg.attributes('width')).toBe('1em')
    expect(svg.attributes('height')).toBe('1em')
    expect(svg.attributes('viewBox')).toBe('0 0 24 24')
    expect(svg.attributes('fill')).toBe('none')
    expect(svg.attributes('stroke')).toBe('currentColor')
    expect(svg.attributes('stroke-width')).toBe('2')
  })

  it('数字尺寸按 px 落地', () => {
    const w = mount(UiIcon, { props: { name: 'moon', size: 18 } })
    expect(w.find('svg').attributes('width')).toBe('18px')
    expect(w.find('svg').attributes('height')).toBe('18px')
  })

  it('图形真的进了 SVG 命名空间（不是被当成 HTML 元素丢掉）', () => {
    const w = mount(UiIcon, { props: { name: 'sun' } })
    expect(w.element.querySelectorAll('svg > :not(title)').length).toBeGreaterThan(0)
    expect(w.element.querySelector('svg path, svg circle, svg line, svg rect')).not.toBeNull()
  })

  it('没有 label 即装饰性图形：aria-hidden，且没有 title', () => {
    const w = mount(UiIcon, { props: { name: 'sun' } })
    expect(w.find('svg').attributes('aria-hidden')).toBe('true')
    expect(w.find('svg').attributes('role')).toBeUndefined()
    expect(w.find('title').exists()).toBe(false)
  })

  it('有 label 时可访问名走 role=img + title（读屏与既有可访问名锁都认这条）', () => {
    const w = mount(UiIcon, { props: { name: 'moon', label: '深色主题' } })
    expect(w.find('svg').attributes('role')).toBe('img')
    expect(w.find('svg').attributes('aria-hidden')).toBeUndefined()
    expect(w.find('title').text()).toBe('深色主题')
  })

  it('class 透传到根 svg（迁移时字号/间距类原样保留）', () => {
    const w = mount(UiIcon, { props: { name: 'sun' }, attrs: { class: 'text-sm text-ink-3' } })
    expect(w.find('svg').classes()).toEqual(expect.arrayContaining(['text-sm', 'text-ink-3']))
  })

  it('未登记的名字降级为警示图标并告警，不白屏', () => {
    const w = mount(UiIcon, { props: { name: 'no-such-icon' as never } })
    expect(w.find('svg').exists()).toBe(true)
    expect(w.element.querySelectorAll('svg > :not(title)').length).toBeGreaterThan(0)
    expect(warn).toHaveBeenCalledWith(expect.stringContaining('no-such-icon'))
  })

  it('静态用法不加载形变模块', async () => {
    mount(UiIcon, { props: { name: 'theme-system' } })
    await Promise.resolve()
    expect(morphModuleFactory).not.toHaveBeenCalled()
  })
})
