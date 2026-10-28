// 主题切换按钮的三档外观（ADR-0079 形变白名单之一）与「顶栏工具位」档（2026-10-28）。
//
// 位置本身由 AppTopBar.spec 守；这里守的是**外观档的边界**：topbar 档必须是控件圆角
// （与同排的铃铛 / 用户菜单同一档），card / ghost 仍是胶囊。圆角从基类下放到变体是
// 这次改动的实现细节，但它同时是 R3「同一元素同一属性不许两个原子类打架」的落点，
// 所以值得钉住：一旦有人把 rounded-pill 加回基类，topbar 档就会被它压掉。
import { describe, it, expect, vi, beforeEach } from 'vitest'
import { mount } from '@vue/test-utils'
import { ElDropdown } from 'element-plus/es/components/dropdown/index.mjs'

const h = vi.hoisted(() => ({
  setMode: vi.fn(),
  mode: 'system' as 'system' | 'light' | 'dark',
  resolved: 'light' as 'light' | 'dark'
}))

vi.mock('@/stores/theme', () => ({
  useThemeStore: () => ({
    get mode() {
      return h.mode
    },
    get resolved() {
      return h.resolved
    },
    setMode: h.setMode
  })
}))

import { epLite } from '@/test/element-lite'
import UiIcon from '@/components/ui/UiIcon.vue'
import ThemeToggle from '../ThemeToggle.vue'

function mountToggle(variant?: 'card' | 'ghost' | 'topbar') {
  return mount(ThemeToggle, {
    props: variant ? { variant } : {},
    global: { plugins: [epLite()] }
  })
}

beforeEach(() => {
  vi.clearAllMocks()
  h.mode = 'system'
  h.resolved = 'light'
})

describe('主题切换按钮', () => {
  it('缺省 card 档是胶囊，topbar 档是控件圆角（圆角归各变体，基类不写）', () => {
    const card = mountToggle()
    expect(card.find('button').classes()).toContain('rounded-pill')
    card.unmount()

    const topbar = mountToggle('topbar')
    expect(topbar.find('button').classes()).toContain('rounded-ctl')
    expect(topbar.find('button').classes()).not.toContain('rounded-pill')
    topbar.unmount()
  })

  it('topbar 档不带描边与阴影（工具位是靠 hover 反色，不是卡片）', () => {
    const w = mountToggle('topbar')
    const cls = w.find('button').classes()
    expect(cls).toContain('border-0')
    expect(cls).not.toContain('shadow-card')
    expect(cls).not.toContain('border')
  })

  it('触发器图标 = 当前三态之一，「跟随系统」优先于 resolved', () => {
    // 每个模式各挂一次：store 是 mock 出来的普通对象，改字段不会触发 computed 重算
    // （真 store 是响应式的，这里测的是「取哪张图」的判据，不是响应性）
    const iconOf = (mode: string, resolved: string) => {
      h.mode = mode as typeof h.mode
      h.resolved = resolved as typeof h.resolved
      const w = mountToggle('topbar')
      const name = w.findComponent(UiIcon).props('name')
      w.unmount()
      return name
    }
    expect(iconOf('system', 'dark'), '跟随系统优先于 resolved，不被伪装成「深色」').toBe('theme-system')
    expect(iconOf('system', 'light')).toBe('theme-system')
    expect(iconOf('dark', 'dark')).toBe('moon')
    expect(iconOf('light', 'light')).toBe('sun')
  })

  it('菜单命令驱动 setMode（三态都能直达，不再循环）', async () => {
    const w = mountToggle('topbar')
    for (const mode of ['light', 'dark', 'system'] as const) {
      await w.findComponent(ElDropdown).vm.$emit('command', mode)
      expect(h.setMode).toHaveBeenLastCalledWith(mode)
    }
    expect(h.setMode).toHaveBeenCalledTimes(3)
  })
})
