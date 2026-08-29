// UI 基础组件层冒烟测试。
// 目的不是覆盖样式，而是守住「能挂载、关键 props 生效、事件能发出」这条底线——
// 组件层一旦有人漏了 import 或改坏 props 默认值，这里会先炸。
import { describe, it, expect } from 'vitest'
import { mount } from '@vue/test-utils'
import type { Component } from 'vue'
import ElementPlus from 'element-plus'

import UiBadge from '../UiBadge.vue'
import UiButton from '../UiButton.vue'
import UiCard from '../UiCard.vue'
import UiCountTo from '../UiCountTo.vue'
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
