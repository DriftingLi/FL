// 管理端「学员反馈」列表（#2070 / ADR-0089）。
// 票面四条：列表渲染 / 状态筛选 / 标记已处理调用与回显 / 空态 + 分页。
// seam：页面组件层，mock `@/api/feedback`（不依赖真实后端）。
import { describe, it, expect, vi, beforeEach } from 'vitest'
import { mount, flushPromises } from '@vue/test-utils'
import { epLite } from '@/test/element-lite'

const ElMessage = vi.hoisted(() => ({ success: vi.fn(), error: vi.fn(), warning: vi.fn() }))
// 页面从 barrel 引 ElMessage；epLite 走 element-plus/es 子路径，不受此 mock 影响
vi.mock('element-plus', () => ({ ElMessage }))

vi.mock('@/api/feedback', () => ({
  feedbackApi: { list: vi.fn(), markHandled: vi.fn() }
}))

import { feedbackApi } from '@/api/feedback'
import type { FeedbackDTO } from '@/api/feedback'
import UiEmptyState from '@/components/ui/UiEmptyState.vue'
import UiPagination from '@/components/ui/UiPagination.vue'
import UiSelect from '@/components/ui/UiSelect.vue'
import FeedbackManage from '../FeedbackManage.vue'

function feedbackOf(over: Partial<FeedbackDTO> = {}): FeedbackDTO {
  return {
    id: 1,
    content: '第三节的视频打不开，换浏览器也一样',
    images: [],
    contact: '微信 abc-123',
    status: 'pending',
    handled_at: null,
    created_at: '2026-10-28T10:00:00+08:00',
    user_id: 2,
    username: '学员小张',
    ...over
  }
}

function mountPage() {
  return mount(FeedbackManage, { global: { plugins: [epLite()] } })
}

beforeEach(() => {
  vi.clearAllMocks()
  vi.mocked(feedbackApi.list).mockResolvedValue({ items: [feedbackOf()], total: 1 })
  vi.mocked(feedbackApi.markHandled).mockResolvedValue(
    feedbackOf({ status: 'handled', handled_at: '2026-10-28T11:00:00+08:00' })
  )
})

describe('FeedbackManage 学员反馈列表（#2070）', () => {
  it('列表渲染提交人、正文、联系方式与状态', async () => {
    const w = mountPage()
    await flushPromises()

    expect(feedbackApi.list).toHaveBeenCalledWith({ page: 1, page_size: 20, status: undefined })
    expect(w.text()).toContain('学员小张')
    expect(w.text()).toContain('第三节的视频打不开')
    expect(w.text()).toContain('微信 abc-123')
    expect(w.text()).toContain('待处理')
    expect(w.text()).toContain('标记已处理')
  })

  it('状态筛选：回第一页并按状态请求；已处理的行不再显示操作按钮', async () => {
    const w = mountPage()
    await flushPromises()

    const select = w.findComponent(UiSelect)
    select.vm.$emit('update:modelValue', 'handled')
    select.vm.$emit('change')
    await flushPromises()

    expect(feedbackApi.list).toHaveBeenLastCalledWith({ page: 1, page_size: 20, status: 'handled' })

    // 换成一页已处理的行：操作列不再是「标记已处理」
    vi.mocked(feedbackApi.list).mockResolvedValue({
      items: [feedbackOf({ status: 'handled', handled_at: '2026-10-28T11:00:00+08:00' })],
      total: 1
    })
    select.vm.$emit('change')
    await flushPromises()
    expect(w.text()).toContain('已处理')
    expect(w.text()).not.toContain('标记已处理')
  })

  it('标记已处理：调接口、提示、并把行刷成已处理', async () => {
    const w = mountPage()
    await flushPromises()

    const btn = w.findAll('button').find((b) => b.text().includes('标记已处理'))
    expect(btn, '应有「标记已处理」按钮').toBeTruthy()
    // 处理后重载回已处理行（服务端事实）
    vi.mocked(feedbackApi.list).mockResolvedValue({
      items: [feedbackOf({ status: 'handled', handled_at: '2026-10-28T11:00:00+08:00' })],
      total: 1
    })
    await btn!.trigger('click')
    await flushPromises()

    expect(feedbackApi.markHandled).toHaveBeenCalledWith(1)
    expect(ElMessage.success).toHaveBeenCalledWith('已标记为已处理')
    expect(w.text()).toContain('已处理')
    expect(feedbackApi.list).toHaveBeenCalledTimes(2)
  })

  it('空态：没有反馈时给空态文案，不给空表格', async () => {
    vi.mocked(feedbackApi.list).mockResolvedValue({ items: [], total: 0 })
    const w = mountPage()
    await flushPromises()

    const empty = w.findComponent(UiEmptyState)
    expect(empty.exists()).toBe(true)
    expect(empty.text()).toContain('暂无反馈')
  })

  it('分页：翻到第 2 页按 page=2 再请求（total 与 items 同源）', async () => {
    vi.mocked(feedbackApi.list).mockResolvedValue({
      items: Array.from({ length: 20 }, (_, i) => feedbackOf({ id: i + 1, content: `反馈 ${i + 1}` })),
      total: 45
    })
    const w = mountPage()
    await flushPromises()

    const pag = w.findComponent(UiPagination)
    expect(pag.exists()).toBe(true)
    pag.vm.$emit('update:currentPage', 2)
    pag.vm.$emit('current-change', 2)
    await flushPromises()

    expect(feedbackApi.list).toHaveBeenLastCalledWith({ page: 2, page_size: 20, status: undefined })
  })
})
