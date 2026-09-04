// UI 基础组件层冒烟测试。
// 目的不是覆盖样式，而是守住「能挂载、关键 props 生效、事件能发出」这条底线——
// 组件层一旦有人漏了 import 或改坏 props 默认值，这里会先炸。
import { describe, it, expect, afterEach } from 'vitest'
import { mount, flushPromises } from '@vue/test-utils'
import { nextTick } from 'vue'
import type { Component } from 'vue'
import ElementPlus from 'element-plus'

import UiBadge from '../UiBadge.vue'
import UiButton from '../UiButton.vue'
import UiCard from '../UiCard.vue'
import UiCountTo from '../UiCountTo.vue'
import UiDialog from '../UiDialog.vue'
import UiDivider from '../UiDivider.vue'
import UiEmptyState from '../UiEmptyState.vue'
import UiErrorState from '../UiErrorState.vue'
import UiInput from '../UiInput.vue'
import UiLoading from '../UiLoading.vue'
import UiPageHeader from '../UiPageHeader.vue'
import UiProgress from '../UiProgress.vue'
import UiSectionHeader from '../UiSectionHeader.vue'
import UiSelect from '../UiSelect.vue'
import UiSkeleton from '../UiSkeleton.vue'
import UiStatCard from '../UiStatCard.vue'
import UiTag from '../UiTag.vue'
import UiActionChip from '../UiActionChip.vue'
import UiSegmentTabs from '../UiSegmentTabs.vue'

const OPTIONS = [
  { label: '全部', value: 'all' },
  { label: '进行中', value: 'doing' }
]

function mountWith(comp: Component, props: Record<string, unknown> = {}) {
  return mount(comp, { props, global: { plugins: [ElementPlus] } })
}

describe('UiCard', () => {
  it('默认渲染 section，非交互态不带 button 语义', () => {
    const w = mountWith(UiCard, {})
    expect(w.element.tagName).toBe('SECTION')
    expect(w.attributes('role')).toBeUndefined()
  })

  it('interactive 变体带 button 语义与 tabindex', () => {
    const w = mountWith(UiCard, { variant: 'interactive' })
    expect(w.attributes('role')).toBe('button')
    expect(w.attributes('tabindex')).toBe('0')
  })

  it('padding=none 时不加内边距类', () => {
    const w = mountWith(UiCard, { padding: 'none' })
    expect(w.classes().some((c) => /^p-\d/.test(c))).toBe(false)
  })
})

describe('UiButton', () => {
  it('primary 映射到 el-button--primary', () => {
    const w = mountWith(UiButton, { variant: 'primary' })
    expect(w.classes()).toContain('el-button--primary')
  })

  it('ghost 走 EP 的 plain 样式', () => {
    const w = mountWith(UiButton, { variant: 'ghost' })
    expect(w.classes()).toContain('is-plain')
  })

  it('block 撑满宽度', () => {
    const w = mountWith(UiButton, { block: true })
    expect(w.classes()).toContain('w-full')
  })
})

describe('UiTag / UiBadge', () => {
  it('brand 标签叠加品牌浅底色', () => {
    const w = mountWith(UiTag, { tone: 'brand' })
    expect(w.classes()).toContain('bg-ui-50')
  })

  it('角标数量超过 max 显示为 max+', () => {
    expect(mountWith(UiBadge, { count: 150, max: 99 }).text()).toBe('99+')
    expect(mountWith(UiBadge, { count: 5 }).text()).toBe('5')
  })

  it('dot 模式不渲染数字', () => {
    const w = mountWith(UiBadge, { count: 8, dot: true })
    expect(w.text()).toBe('')
  })

  it('数量为 0 时不渲染', () => {
    expect(mountWith(UiBadge, { count: 0 }).text()).toBe('')
  })
})

describe('UiStatCard', () => {
  it('渲染标签、数值与单位', () => {
    const w = mountWith(UiStatCard, { label: '学习时长', value: 128, unit: '分钟' })
    expect(w.text()).toContain('学习时长')
    expect(w.text()).toContain('128')
    expect(w.text()).toContain('分钟')
  })

  it('trend 为正显示上升箭头', () => {
    expect(mountWith(UiStatCard, { label: 'a', trend: 12 }).text()).toContain('↑')
    expect(mountWith(UiStatCard, { label: 'a', trend: -5 }).text()).toContain('↓')
  })

  it('loading 时不显示数值', () => {
    const w = mountWith(UiStatCard, { label: 'a', value: 42, loading: true })
    expect(w.text()).not.toContain('42')
  })
})

describe('UiEmptyState / UiErrorState', () => {
  it('空状态点击行动按钮抛出 action', async () => {
    const w = mountWith(UiEmptyState, { title: '暂无数据', actionText: '去选课' })
    expect(w.text()).toContain('暂无数据')
    await w.find('button').trigger('click')
    expect(w.emitted('action')).toHaveLength(1)
  })

  it('错误态点击重试抛出 retry', async () => {
    const w = mountWith(UiErrorState, { description: '网络异常' })
    expect(w.text()).toContain('网络异常')
    await w.find('button').trigger('click')
    expect(w.emitted('retry')).toHaveLength(1)
  })
})

describe('UiSkeleton / UiLoading', () => {
  it('chart 变体渲染单块占位，table 变体按 count 渲染行', () => {
    expect(mountWith(UiSkeleton, { variant: 'chart' }).findAll('div').length).toBeGreaterThan(0)
    const table = mountWith(UiSkeleton, { variant: 'table', count: 5 })
    expect(table.html()).toContain('h-10')
  })

  it('加载提示文本渲染，block 带最小高度', () => {
    const w = mountWith(UiLoading, { tip: '加载中', block: true })
    expect(w.text()).toContain('加载中')
    expect(w.classes()).toContain('min-h-32')
  })
})

describe('UiSectionHeader / UiPageHeader / UiDivider', () => {
  it('section 头渲染标题与副标题', () => {
    const w = mountWith(UiSectionHeader, { title: '我的课程', subtitle: '共 3 门' })
    expect(w.text()).toContain('我的课程')
    expect(w.text()).toContain('共 3 门')
  })

  it('page 头开启 back 后点击抛出 back', async () => {
    const w = mountWith(UiPageHeader, { title: '课程详情', back: true })
    await w.find('button').trigger('click')
    expect(w.emitted('back')).toHaveLength(1)
  })

  it('分割线带标签时渲染文字', () => {
    expect(mountWith(UiDivider, { label: '或' }).text()).toContain('或')
  })
})

describe('UiProgress / UiCountTo', () => {
  it('进度值超过 100 被夹到 100', () => {
    const w = mountWith(UiProgress, { value: 180 })
    expect(w.text()).not.toContain('180')
  })

  it('数字滚动首帧即显示终值，不依赖动画帧', () => {
    const w = mountWith(UiCountTo, { from: 0, to: 88 })
    expect(w.text()).toBe('88')
  })

  it('decimals 控制小数位', () => {
    expect(mountWith(UiCountTo, { to: 3.14159, decimals: 2 }).text()).toBe('3.14')
  })
})

describe('UiInput / UiSelect', () => {
  it('输入框输入后同步 modelValue', async () => {
    const w = mountWith(UiInput, { modelValue: '' })
    await w.find('input').setValue('叉车')
    expect(w.emitted('update:modelValue')?.[0]).toEqual(['叉车'])
  })

  it('下拉按 options 渲染选项', () => {
    const w = mountWith(UiSelect, { options: OPTIONS })
    expect(w.html()).toContain('el-select')
  })
})

// el-dialog 把内容 teleport 到 body，有两个坑：
// 1. wrapper.html() 抓不到 teleport 内容，必须 attachTo: document.body 后直接查 document；
// 2. 首帧 DOM 未必就位（v-if + transition），mount 后要 flushPromises 才能断言；
//    且 unmount 后 transition 节点会残留，不清干净会让后续用例的 querySelectorAll 串到上一个弹窗。
describe('UiDialog', () => {
  afterEach(() => {
    document.body.innerHTML = ''
  })

  async function mountDialog(props: Record<string, unknown> = {}) {
    const w = mount(UiDialog, {
      props: { modelValue: true, ...props },
      attachTo: document.body,
      global: { plugins: [ElementPlus] }
    })
    await flushPromises()
    await nextTick()
    return w
  }

  function footerButtons() {
    return Array.from(document.querySelectorAll('.el-dialog__footer button'))
  }

  function findButton(text: string) {
    return footerButtons().find((b) => b.textContent?.includes(text))
  }

  it('渲染标题、副标题与默认页脚按钮', async () => {
    const w = await mountDialog({ title: '发布新帖', subtitle: '请遵守社区规范' })
    const body = document.body.textContent ?? ''
    expect(body).toContain('发布新帖')
    expect(body).toContain('请遵守社区规范')
    expect(body).toContain('取消')
    expect(body).toContain('确定')
    w.unmount()
  })

  // 注意 EP 的 .el-dialog__header 容器总会渲染（右上角关闭按钮要挂在里面），
  // 我们的 v-if 控制的是**标题内容**，不是容器本身。
  it('不传 title / subtitle / icon 时头部没有标题区', async () => {
    const w = await mountDialog()
    expect(document.querySelector('.el-dialog__header h3')).toBeNull()
    w.unmount()
  })

  it('hideFooter 时不渲染页脚', async () => {
    const w = await mountDialog({ title: '举报', hideFooter: true })
    expect(document.querySelector('.el-dialog__footer')).toBeNull()
    w.unmount()
  })

  it('点确定发 confirm，且不会顺带发 cancel', async () => {
    const w = await mountDialog({ title: '举报', confirmText: '提交' })
    findButton('提交')?.dispatchEvent(new MouseEvent('click', { bubbles: true }))
    await nextTick()
    expect(w.emitted('confirm')).toBeTruthy()
    expect(w.emitted('cancel')).toBeFalsy()
    w.unmount()
  })

  it('点取消发 cancel 并同步关闭 v-model', async () => {
    const w = await mountDialog({ title: '举报', cancelText: '取消' })
    findButton('取消')?.dispatchEvent(new MouseEvent('click', { bubbles: true }))
    await nextTick()
    expect(w.emitted('cancel')).toBeTruthy()
    expect(w.emitted('update:modelValue')?.[0]).toEqual([false])
    w.unmount()
  })

  it('showCancel=false 时页脚只剩确定按钮', async () => {
    const w = await mountDialog({ title: '提示', showCancel: false, confirmText: '知道了' })
    const labels = footerButtons().map((b) => b.textContent?.trim())
    expect(labels).toHaveLength(1)
    expect(labels[0]).toContain('知道了')
    w.unmount()
  })
})


describe('UiActionChip（操作药丸）', () => {
  it('渲染 label + count，点击发 click 事件', async () => {
    const w = mountWith(UiActionChip, { label: '点赞', count: 12, tone: 'like' })
    expect(w.text()).toContain('点赞')
    expect(w.text()).toContain('12')
    await w.trigger('click')
    expect(w.emitted('click')).toBeTruthy()
  })

  it('tone=like 且 active 时带玫红激活类', () => {
    const w = mountWith(UiActionChip, { label: '点赞', count: 12, tone: 'like', active: true })
    expect(w.classes()).toContain('bg-rose-soft')
    expect(w.classes()).toContain('text-rose')
  })

  it('tone=danger 未激活时无红（hover 才显），激活样式仅互动类有', () => {
    const w = mountWith(UiActionChip, { label: '删除', tone: 'danger' })
    expect(w.classes()).not.toContain('bg-bad-soft')
    expect(w.classes()).toContain('text-ink-3')
  })

  it('compact 不渲染可见 label，但保留 sr-only 可访问名', () => {
    const w = mountWith(UiActionChip, { label: '收藏', count: 3, compact: true })
    // 可见文本区：只有计数（label 进 sr-only，不参与可见层）
    expect(w.find('.sr-only').exists()).toBe(true)
    expect(w.find('.sr-only').text()).toContain('收藏')
    const visible = w.find('button').text().replace(w.find('.sr-only').text(), '')
    expect(visible).toContain('3')
  })

  it('disabled 不发 click', async () => {
    const w = mountWith(UiActionChip, { label: '点赞', disabled: true })
    await w.trigger('click')
    expect(w.emitted('click')).toBeFalsy()
  })
})

describe('UiSegmentTabs（分段选项卡）', () => {
  const opts = [
    { label: '近7天', value: '7d' },
    { label: '近30天', value: '30d' }
  ]

  it('渲染选项并标记激活项 aria-selected', () => {
    const w = mountWith(UiSegmentTabs, { modelValue: '7d', options: opts })
    const btns = w.findAll('button')
    expect(btns).toHaveLength(2)
    expect(btns[0].attributes('aria-selected')).toBe('true')
    expect(btns[1].attributes('aria-selected')).toBe('false')
  })

  it('点击选项发 update:modelValue + change', async () => {
    const w = mountWith(UiSegmentTabs, { modelValue: '7d', options: opts })
    await w.findAll('button')[1].trigger('click')
    expect(w.emitted('update:modelValue')?.[0]).toEqual(['30d'])
    expect(w.emitted('change')?.[0]).toEqual(['30d'])
  })

  it('disabled 时点击不发事件', async () => {
    const w = mountWith(UiSegmentTabs, { modelValue: '7d', options: opts, disabled: true })
    await w.findAll('button')[1].trigger('click')
    expect(w.emitted('update:modelValue')).toBeFalsy()
  })
})
