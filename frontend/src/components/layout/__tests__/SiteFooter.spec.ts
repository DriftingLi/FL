// SiteFooter 的渲染契约（#1756 第一批）：运营主体、备案号与备案链接三件事必须同时在场。
// 备案号是监管件：只测「元素存在」不够，还要测它**指向工信部备案系统**且是新窗口打开。
import { describe, it, expect } from 'vitest'
import { mount } from '@vue/test-utils'
import SiteFooter from '@/components/layout/SiteFooter.vue'

describe('SiteFooter', () => {
  it('展示运营主体与备案号', () => {
    const wrapper = mount(SiteFooter)
    expect(wrapper.text()).toContain('易道网络科技（广东）有限公司')
    expect(wrapper.text()).toContain('粤ICP备2026005936号-3')
  })

  it('备案号是外链且指向工信部备案系统', () => {
    const wrapper = mount(SiteFooter)
    const link = wrapper.get('a')
    expect(link.attributes('href')).toContain('beian.miit.gov.cn')
    expect(link.attributes('target')).toBe('_blank')
    expect(link.attributes('rel')).toContain('noopener')
  })

  it('带品牌标识', () => {
    const wrapper = mount(SiteFooter)
    expect(wrapper.get('img').attributes('src')).toContain('HRWAIlogo')
  })
})
