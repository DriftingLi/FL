// epLite 自测（#765 / #769）：验证按需注册插件在 happy-dom 下的三条底线 ——
// 白名单组件可渲染、v-loading 指令可用、extra 补充注册生效。
// 不 stub、不 shallowMount：断言依赖 EP 真实渲染出的 .el-* class。
import { describe, it, expect } from 'vitest'
import { mount } from '@vue/test-utils'
import { ElCarousel, ElCarouselItem } from 'element-plus'
import { epLite, EP_LITE_WHITELIST } from '../element-lite'

describe('epLite（测试专用 EP 按需注册）', () => {
  it('白名单组件经插件注册后可真实渲染（button / tag / input / table）', () => {
    const w = mount(
      {
        template:
          '<div><el-button>点我</el-button><el-tag size="small">标签</el-tag><el-input /><el-table :data="rows"><el-table-column prop="a" /></el-table></div>',
        setup: () => ({ rows: [{ a: 1 }] })
      },
      { global: { plugins: [epLite()] } }
    )
    expect(w.find('.el-button').exists()).toBe(true)
    expect(w.find('.el-tag').exists()).toBe(true)
    expect(w.find('.el-input__wrapper').exists()).toBe(true)
    expect(w.find('.el-table').exists()).toBe(true)
  })

  it('v-loading 指令已随插件注册（全站 34 处使用）', () => {
    const w = mount({ template: '<div v-loading="true">x</div>' }, {
      global: { plugins: [epLite()] }
    })
    expect(w.find('.el-loading-mask').exists()).toBe(true)
  })

  it('extra 参数可补充白名单之外的组件（ElCarousel 不在白名单）', () => {
    const template = '<div><el-carousel><el-carousel-item /></el-carousel></div>'
    const without = mount({ template }, { global: { plugins: [epLite()] } })
    expect(without.find('.el-carousel').exists()).toBe(false)

    const withExtra = mount({ template }, {
      global: { plugins: [epLite([ElCarousel, ElCarouselItem])] }
    })
    expect(withExtra.find('.el-carousel').exists()).toBe(true)
  })

  it('重复注册同一组件不抛错（去重）', () => {
    expect(() => epLite(EP_LITE_WHITELIST)).not.toThrow()
  })
})
