// UiCollapse：折叠的语义是「高度过渡 + 内容常驻」，不是 display:none。
// 这里锁两件：关闭态给 inert（里面不能再被 Tab 到），以及 open 切换只换类、不卸载 slot。
import { describe, it, expect } from 'vitest'
import { mount } from '@vue/test-utils'
import UiCollapse from '../UiCollapse.vue'

describe('UiCollapse', () => {
  it('关闭时压轨道并 inert，内容仍在 DOM 里', () => {
    const w = mount(UiCollapse, {
      props: { open: false },
      slots: { default: '<p class="collapse-body">内容</p>' }
    })
    const [root, inner] = w.findAll('div')
    expect(root.classes()).toContain('grid-rows-[0fr]')
    expect(inner.classes()).toContain('opacity-0')
    expect(inner.attributes('inert')).toBeDefined()
    expect(w.find('.collapse-body').exists()).toBe(true)
  })

  it('展开时轨道拉满、移除 inert；反向切换不卸载 slot', async () => {
    const w = mount(UiCollapse, {
      props: { open: false },
      slots: { default: '<p class="collapse-body">内容</p>' }
    })
    await w.setProps({ open: true })
    const [root, inner] = w.findAll('div')
    expect(root.classes()).toContain('grid-rows-[1fr]')
    expect(inner.classes()).toContain('opacity-100')
    expect(inner.attributes('inert')).toBeUndefined()
    expect(w.find('.collapse-body').exists()).toBe(true)
  })

  it('lazy：首次展开前不挂载内容，展开后保留（收起不卸载）', async () => {
    const w = mount(UiCollapse, {
      props: { open: false, lazy: true },
      slots: { default: '<p class="collapse-body">内容</p>' }
    })
    expect(w.find('.collapse-body').exists()).toBe(false)
    await w.setProps({ open: true })
    expect(w.find('.collapse-body').exists()).toBe(true)
    await w.setProps({ open: false })
    // 保留 DOM，离场高度才有内容可压
    expect(w.find('.collapse-body').exists()).toBe(true)
  })
})
