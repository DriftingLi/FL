// UiAsyncSection：列表四段式合成组件（#1054）的组件级测试。
// 断言的是**外部行为**：给定四态组合时渲染出哪个分支、优先级是否恒定、
// retry 事件与 retrying 防连点的透传、表格档（skeleton=false）不渲染骨架——不测内部实现。
import { describe, it, expect } from 'vitest'
import { mount } from '@vue/test-utils'
import { epLite } from '@/test/element-lite'

import UiAsyncSection from '../UiAsyncSection.vue'
import UiErrorState from '../UiErrorState.vue'
import UiSkeleton from '../UiSkeleton.vue'

function mountSection(props: Record<string, unknown> = {}, slots: Record<string, string> = {}) {
  return mount(UiAsyncSection, {
    props,
    slots,
    global: { plugins: [epLite()] }
  })
}

describe('UiAsyncSection 渲染优先级（错误 → 骨架 → 空态 → 内容）', () => {
  it('全静默（无 error/loading/empty）→ 渲染默认插槽（内容）', () => {
    const w = mountSection({}, { default: '<div class="content">内容</div>' })
    expect(w.find('.content').exists()).toBe(true)
  })

  it('error 优先于 loading 与 empty：失败且装载中只显示错误态', () => {
    const w = mountSection(
      { error: true, loading: true, empty: true, retrying: false },
      { default: '<div class="content">内容</div>', empty: '<div class="empty-slot">空</div>' }
    )
    expect(w.findComponent(UiErrorState).exists()).toBe(true)
    expect(w.find('.content').exists()).toBe(false)
    expect(w.find('.empty-slot').exists()).toBe(false)
  })

  it('loading 优先于 empty：装载中显示骨架，不显示空态', () => {
    const w = mountSection(
      { loading: true, empty: true },
      { skeleton: '<div class="skeleton-slot">骨架</div>', empty: '<div class="empty-slot">空</div>' }
    )
    expect(w.find('.skeleton-slot').exists()).toBe(true)
    expect(w.find('.empty-slot').exists()).toBe(false)
  })

  it('empty 且未在装载 → 显示 #empty 插槽，不渲染内容', () => {
    const w = mountSection(
      { empty: true },
      { default: '<div class="content">内容</div>', empty: '<div class="empty-slot">空</div>' }
    )
    expect(w.find('.empty-slot').exists()).toBe(true)
    expect(w.find('.content').exists()).toBe(false)
  })

  it('防御：empty 与 loading 同真时按加载处理（空态判定只在装载完成后成立）', () => {
    const w = mountSection(
      { loading: true, empty: true },
      { default: '<div class="content">内容</div>', empty: '<div class="empty-slot">空</div>' }
    )
    expect(w.find('.empty-slot').exists()).toBe(false)
    // 未提供骨架槽 → 默认骨架（不是内容、不是空态）
    expect(w.find('.content').exists()).toBe(false)
    expect(w.findComponent(UiSkeleton).exists()).toBe(true)
  })
})

describe('UiAsyncSection 骨架', () => {
  it('未提供 #skeleton 插槽时给默认列表骨架（UiSkeleton）', () => {
    const w = mountSection({ loading: true })
    expect(w.findComponent(UiSkeleton).exists()).toBe(true)
  })

  it('提供 #skeleton 插槽时原样渲染（零视觉变更的迁移姿势）', () => {
    const w = mountSection({ loading: true }, { skeleton: '<div class="my-skeleton">自定义骨架</div>' })
    expect(w.find('.my-skeleton').exists()).toBe(true)
    expect(w.findComponent(UiSkeleton).exists()).toBe(false)
  })

  it('表格档 skeleton=false：装载中也渲染内容（加载观感交给表格自带遮罩）', () => {
    const w = mountSection(
      { loading: true, skeleton: false },
      { default: '<div class="table-wrap">表格</div>' }
    )
    expect(w.find('.table-wrap').exists()).toBe(true)
    expect(w.findComponent(UiSkeleton).exists()).toBe(false)
  })
})

describe('UiAsyncSection 错误态交互', () => {
  it('点击重试派发 retry 事件；文案透传错误态', async () => {
    const w = mountSection(
      { error: true, errorTitle: '列表加载失败', errorDescription: '网络异常' },
      {}
    )
    const err = w.findComponent(UiErrorState)
    expect(err.exists()).toBe(true)
    expect(err.props('title')).toBe('列表加载失败')
    expect(err.props('description')).toBe('网络异常')
    expect(err.props('retrying')).toBe(false)

    await err.find('button').trigger('click')
    expect(w.emitted('retry')).toHaveLength(1)
  })

  it('retrying 透传：重试中按钮进入加载/禁用态（防连点，事件不派发）', async () => {
    const w = mountSection({ error: true, retrying: true }, {})
    const err = w.findComponent(UiErrorState)
    expect(err.props('retrying')).toBe(true)

    const btn = err.find('button')
    expect(btn.attributes('disabled')).toBeDefined()
    await btn.trigger('click')
    expect(w.emitted('retry')).toBeUndefined()
  })

  it('errorTitle/errorDescription 缺省时回落 UiErrorState 的默认标题', () => {
    const w = mountSection({ error: true })
    // 传 undefined 的 prop 不覆盖子组件 withDefaults 的默认值——自定义文案只在页面显式给时生效
    expect(w.findComponent(UiErrorState).props('title')).toBe('加载失败')
  })
})
