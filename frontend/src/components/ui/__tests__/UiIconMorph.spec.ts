import { describe, expect, it, vi, beforeEach } from 'vitest'
import { flushPromises, mount } from '@vue/test-utils'
import { nextTick } from 'vue'
import UiIcon from '../UiIcon.vue'

/**
 * 形变**接线**契约：UiIcon 只负责「把哪个图形交给形变层、用哪个弹簧/降级口径」，
 * 形变数学是第三方库的事，本仓不重复测它。
 */
const { received, renderCount } = vi.hoisted(() => ({ received: [] as unknown[], renderCount: { value: 0 } }))

vi.mock('../UiIconMorph.vue', async () => {
  const { defineComponent, h } = await import('vue')
  return {
    default: defineComponent({
      name: 'UiIconMorphStub',
      props: { icon: { type: [Array, String], default: undefined }, spring: { default: undefined }, reducedMotion: { default: undefined } },
      setup(props) {
        return () => {
          received.push({ icon: props.icon, spring: props.spring, reducedMotion: props.reducedMotion })
          renderCount.value += 1
          return h('svg', { 'data-morph-stub': '1' })
        }
      }
    })
  }
})

const last = () => received[received.length - 1] as { icon: unknown; spring: unknown; reducedMotion: unknown }

describe('UiIcon（形变接线）', () => {
  beforeEach(() => {
    received.length = 0
  })

  it('morph 打开后改走形变层，并把当前图形交出去', async () => {
    const w = mount(UiIcon, { props: { name: 'sun', morph: true } })
    await flushPromises()
    expect(w.find('[data-morph-stub]').exists()).toBe(true)
    expect(last().icon).toBeTruthy()
  })

  it('name 变化时交出去的是新图形（形变层的输入来自这里）', async () => {
    const w = mount(UiIcon, { props: { name: 'sun', morph: true } })
    await flushPromises()
    const first = last().icon
    await w.setProps({ name: 'moon' })
    await nextTick()
    await flushPromises()
    expect(last().icon).not.toBe(first)
  })

  it('默认降级口径跟随系统，默认弹簧为 snappy', async () => {
    mount(UiIcon, { props: { name: 'sun', morph: true } })
    await flushPromises()
    expect(last().reducedMotion).toBe('user')
    expect(last().spring).toBe('snappy')
  })

  it('morph 从 false 翻到 true 时补加载，并改走形变层', async () => {
    const w = mount(UiIcon, { props: { name: 'sun', morph: false } })
    await flushPromises()
    expect(w.find('[data-morph-stub]').exists()).toBe(false)
    await w.setProps({ morph: true })
    await flushPromises()
    expect(w.find('[data-morph-stub]').exists()).toBe(true)
  })

  it('调用方可以覆盖弹簧与降级口径', async () => {
    mount(UiIcon, { props: { name: 'sun', morph: true, spring: 'bouncy', reducedMotion: 'always' } })
    await flushPromises()
    expect(last().spring).toBe('bouncy')
    expect(last().reducedMotion).toBe('always')
  })
})
