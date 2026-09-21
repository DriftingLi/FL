// 「我的笔记」（Notebook.vue）契约测试（#1078 / ADR-0055）：
// 1) 列表按 scope 拉取并渲染（正文首行当标题 + 摘要 + 题目徽标/独立标签 + 更新时间）；
// 2) 分段控件切换 scope → 以对应 scope 重新请求；
// 3) 空态文案按 scope 区分；
// 4) 删除经确认框后调 remove 并刷新。
// seam：组件层，mock '@/api/note'（不依赖真实后端）。
import { describe, it, expect, vi, beforeEach } from 'vitest'
import { mount, flushPromises } from '@vue/test-utils'
import { ElMessage } from 'element-plus'
import { epLite } from '@/test/element-lite'
import { testRouter } from '@/test/router'

vi.mock('@/api/note', () => ({
  noteApi: {
    list: vi.fn(),
    create: vi.fn(),
    update: vi.fn(),
    remove: vi.fn()
  }
}))

vi.mock('@/composables/useConfirm', () => ({
  useConfirm: () => ({
    confirm: vi.fn().mockResolvedValue(undefined),
    confirmDanger: vi.fn().mockResolvedValue(undefined),
    prompt: vi.fn().mockResolvedValue({ value: '' })
  })
}))

import { noteApi } from '@/api/note'
import Notebook from '../Notebook.vue'

const questionNote = {
  id: 1,
  content: '液压泵异响的判断\n先看油位，再看滤芯',
  question_id: 101,
  question_content: '液压泵异响应先检查什么？',
  updated_at: '2026-09-17T02:00:00Z'
}

const standaloneNote = {
  id: 2,
  content: '周末复习计划',
  question_id: null,
  question_content: '',
  updated_at: '2026-09-16T02:00:00Z'
}

function mockList(items: unknown[], total = items.length) {
  vi.mocked(noteApi.list).mockResolvedValue({ items, page: 1, page_size: 20, total } as never)
}

function mountPage() {
  // 装真路由（表从描述符派生）：跳题落点是 href('StudentQuestionDetail', { id })，
  // 断言的是**解析出来的 href**，比桩元素的 to 属性更接近用户实际点到的东西。
  return mount(Notebook, { global: { plugins: [epLite(), testRouter()] } })
}

beforeEach(() => {
  vi.mocked(noteApi.list).mockReset()
  vi.mocked(noteApi.create).mockReset()
  vi.mocked(noteApi.update).mockReset()
  vi.mocked(noteApi.remove).mockReset()
  mockList([questionNote, standaloneNote])
  ;(vi.spyOn(ElMessage, 'success') as unknown as { mockImplementation: (f: () => void) => unknown }).mockImplementation(() => {})
  ;(vi.spyOn(ElMessage, 'warning') as unknown as { mockImplementation: (f: () => void) => unknown }).mockImplementation(() => {})
})

describe('我的笔记 列表（#1078）', () => {
  it('首屏拉全部笔记，正文首行当标题、其余行当摘要', async () => {
    const w = mountPage()
    await flushPromises()
    expect(noteApi.list).toHaveBeenCalledWith({ scope: 'all', page: 1, page_size: 20 })
    const text = w.text()
    expect(text).toContain('液压泵异响的判断')
    expect(text).toContain('先看油位，再看滤芯')
    expect(text).toContain('周末复习计划')
  })

  it('题目笔记带可跳题的题目徽标，独立笔记带「独立笔记」标签', async () => {
    const w = mountPage()
    await flushPromises()
    // 未装 router 时 RouterLink 被 VTU 桩掉，故断言桩元素上的 to 属性（跳题落点）
    expect(w.html()).toContain('/training/questions/101')
    expect(w.text()).toContain('独立笔记')
  })

  it('渲染更新时间', async () => {
    const w = mountPage()
    await flushPromises()
    expect(w.text()).toContain('2026-09-17')
  })
})

describe('我的笔记 scope 筛选（#1078）', () => {
  it('切到「独立笔记」以 scope=standalone 重新请求', async () => {
    const w = mountPage()
    await flushPromises()
    const tab = w.findAll('button').find(b => b.text().includes('独立笔记'))
    expect(tab).toBeTruthy()
    await tab!.trigger('click')
    await flushPromises()
    expect(noteApi.list).toHaveBeenLastCalledWith({ scope: 'standalone', page: 1, page_size: 20 })
  })

  it('空态文案按 scope 区分', async () => {
    mockList([])
    const w = mountPage()
    await flushPromises()
    expect(w.text()).toContain('还没有笔记')
  })
})

describe('我的笔记 删除（#1078）', () => {
  it('删除经确认框后调 remove 并刷新列表', async () => {
    vi.mocked(noteApi.remove).mockResolvedValue(undefined as never)
    const w = mountPage()
    await flushPromises()
    const del = w.findAll('button').find(b => b.text().includes('删除'))
    expect(del).toBeTruthy()
    await del!.trigger('click')
    await flushPromises()
    expect(noteApi.remove).toHaveBeenCalledWith(1)
    expect(noteApi.list).toHaveBeenCalledTimes(2)
  })
})
