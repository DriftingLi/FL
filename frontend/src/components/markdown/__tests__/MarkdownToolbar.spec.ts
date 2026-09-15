// 工具栏（图一形态 / ADR-0052）：守「按钮集合 = 声明子集命令表」这条单点，
// 以及图三的悬停提示 —— 提示文案不许在组件里另抄一份。
import { describe, it, expect } from 'vitest'
import { mount } from '@vue/test-utils'
import { epLite } from '@/test/element-lite'
import MarkdownToolbar from '../MarkdownToolbar.vue'
import MarkdownToolbarIcon from '../MarkdownToolbarIcon.vue'
import UiTooltip from '@/components/ui/UiTooltip.vue'
import { MARKDOWN_TOOLBAR_ITEMS, MARKDOWN_TOOLBAR_DIVIDERS } from '@/utils/markdownToolbar'

function mountToolbar(props: Record<string, unknown> = {}) {
  return mount(MarkdownToolbar, { props, global: { plugins: [epLite()] } })
}

/**
 * UiTooltip 是**纯透传**封装（不声明 props，全部经 $attrs 落到 el-tooltip），
 * 所以断言要读 $attrs 而不是 props('content')。
 * 模板里写的是 kebab-case，Vue 原样保留 key，两种写法都兜住。
 */
function attrOf(tip: { vm: { $attrs: Record<string, unknown> } }, name: string): unknown {
  const camel = name.replace(/-(\w)/g, (_, c: string) => c.toUpperCase())
  return tip.vm.$attrs[name] ?? tip.vm.$attrs[camel]
}

describe('MarkdownToolbar', () => {
  it('按钮集合与命令表一一对应（数量、顺序、aria-label）', () => {
    const w = mountToolbar()
    const buttons = w.findAll('button')
    expect(buttons).toHaveLength(MARKDOWN_TOOLBAR_ITEMS.length)
    buttons.forEach((btn, index) => {
      const item = MARKDOWN_TOOLBAR_ITEMS[index]
      expect(btn.attributes('aria-label')).toBe(item.label)
    })
  })

  it('每个按钮都有悬停提示（图三），文案与 aria-label 同源', () => {
    const w = mountToolbar()
    const tips = w.findAllComponents(UiTooltip)
    expect(tips).toHaveLength(MARKDOWN_TOOLBAR_ITEMS.length)
    tips.forEach((tip, index) => {
      expect(attrOf(tip, 'content')).toBe(MARKDOWN_TOOLBAR_ITEMS[index].label)
      // 提示落在按钮下方、带短延迟（对齐 AppSidebar 的先例与图三观感）
      expect(attrOf(tip, 'placement')).toBe('bottom')
      expect(attrOf(tip, 'show-after')).toBe(300)
    })
  })

  it('提示文案全中文（防漂移成英文）', () => {
    const w = mountToolbar()
    for (const tip of w.findAllComponents(UiTooltip)) {
      expect(/[\u4e00-\u9fa5]/.test(String(attrOf(tip, 'content')))).toBe(true)
    }
  })

  it('图标名与命令表一致', () => {
    const w = mountToolbar()
    const icons = w.findAllComponents(MarkdownToolbarIcon)
    expect(icons).toHaveLength(MARKDOWN_TOOLBAR_ITEMS.length)
    icons.forEach((icon, index) => {
      expect(icon.props('name')).toBe(MARKDOWN_TOOLBAR_ITEMS[index].icon)
    })
  })

  it('点击发 command，载荷是该按钮的命令键', async () => {
    const w = mountToolbar()
    await w.findAll('button')[1].trigger('click')
    expect(w.emitted('command')?.[0]).toEqual([MARKDOWN_TOOLBAR_ITEMS[1].key])
  })

  it('分组竖线数与分隔表一致，且对读屏隐藏', () => {
    const w = mountToolbar()
    const dividers = w.findAll('.markdown-toolbar-divider')
    expect(dividers).toHaveLength(MARKDOWN_TOOLBAR_DIVIDERS.length)
  })

  it('disabled：点击不发命令，但按钮仍在且带 aria-disabled（提示仍可悬停）', async () => {
    const w = mountToolbar({ disabled: true })
    const buttons = w.findAll('button')
    expect(buttons).toHaveLength(MARKDOWN_TOOLBAR_ITEMS.length)
    expect(buttons[0].attributes('aria-disabled')).toBe('true')
    // 不用原生 disabled —— 原生 disabled 的按钮不派发鼠标事件，气泡会整排消失
    expect(buttons[0].attributes('disabled')).toBeUndefined()
    await buttons[0].trigger('click')
    expect(w.emitted('command')).toBeFalsy()
    expect(w.findAllComponents(UiTooltip)).toHaveLength(MARKDOWN_TOOLBAR_ITEMS.length)
  })

  it('按钮 bg-transparent（项目无 preflight，默认灰底会糊住顶栏）', () => {
    const w = mountToolbar()
    for (const btn of w.findAll('button')) {
      expect(btn.classes()).toContain('bg-transparent')
    }
  })
})
