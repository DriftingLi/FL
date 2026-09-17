// #493 简历库：方形网格卡片 + 加载更多（追加、END）。
import { describe, it, expect, vi, beforeEach } from 'vitest'
import { mount, flushPromises } from '@vue/test-utils'
import { epLite } from '@/test/element-lite'

vi.mock('element-china-area-data', () => ({
  pcTextArr: [
    { label: '江苏省', children: [{ label: '苏州市' }] },
  ],
}))
vi.mock('@/api/recruit', () => ({
  recruitApi: { listResumes: vi.fn() },
}))
vi.mock('@/api/request', () => ({
  unwrappedRequest: { get: vi.fn() },
}))
vi.mock('vue-router', () => ({
  RouterLink: { template: `<a><slot /></a>` },
}))

import { recruitApi } from '@/api/recruit'
import { unwrappedRequest } from '@/api/request'
import Resumes from '../Resumes.vue'

function mkCard(id: number) {
  return {
    user_id: id, real_name_masked: '张*', real_name: '张*', expected_regions: [],
    salary_negotiable: false, experience_years: 3, updated_at: '2026-09-01T00:00:00Z',
  }
}

function mountPage() {
  return mount(Resumes, { global: { plugins: [epLite()] } })
}

beforeEach(() => {
  vi.mocked(recruitApi.listResumes).mockReset()
  vi.mocked(unwrappedRequest.get).mockReset()
  vi.mocked(unwrappedRequest.get).mockResolvedValue({ positions: [], credentials: [] })
})

describe('简历库网格与加载更多（#493）', () => {
  it('首批 20 条显示加载更多，点击追加', async () => {
    const b1 = Array.from({ length: 20 }, (_, i) => mkCard(i + 1))
    const b2 = [mkCard(21)]
    vi.mocked(recruitApi.listResumes).mockResolvedValueOnce({ items: b1, total: 21 } as any)
      .mockResolvedValueOnce({ items: b2, total: 21 } as any)
    const wrapper = mountPage()
    await flushPromises()
    const btn = wrapper.findAll('button').find((b) => b.text().includes('加载更多'))!
    expect(btn.exists()).toBe(true)
    await btn.trigger('click')
    await flushPromises()
    expect(wrapper.text()).toContain('没有更多了')
    expect(recruitApi.listResumes).toHaveBeenCalledTimes(2)
  })

  it('空结果显示空态', async () => {
    vi.mocked(recruitApi.listResumes).mockResolvedValue({ items: [], total: 0 } as any)
    const wrapper = mountPage()
    await flushPromises()
    expect(wrapper.text()).toContain('暂无公开简历')
    // 空态不是错误态：不给「重试」入口
    expect(wrapper.findAll('button').some(b => b.text().includes('重试'))).toBe(false)
  })

  it('错误态：加载失败渲染错误态 + 重试（与空态互斥）', async () => {
    vi.mocked(recruitApi.listResumes).mockRejectedValue(new Error('boom'))
    const wrapper = mountPage()
    await flushPromises()
    expect(wrapper.text()).toContain('简历加载失败')
    expect(wrapper.text()).not.toContain('暂无公开简历')
  })

  // #1101：append 式分页只剩 useAsyncPage 一个实现（filterDeps 承担筛选重置，
  // 页面不再有 resetAndLoad / loadMore / hasMore 三份手抄）。
  it('filterDeps：改筛选轴即清空累积 + 回第 1 批重装', async () => {
    const b1 = Array.from({ length: 20 }, (_, i) => mkCard(i + 1))
    const b2 = Array.from({ length: 20 }, (_, i) => mkCard(200 + i))
    vi.mocked(recruitApi.listResumes)
      .mockResolvedValueOnce({ items: b1, total: 40 } as any)
      .mockResolvedValueOnce({ items: b2, total: 40 } as any)

    const wrapper = mountPage()
    await flushPromises()
    expect(wrapper.text()).toContain('张*')

    // 模拟筛选轴变化：控件 @change 后生效快照变化 → filterDeps 触发重置重装
    const salary = wrapper.find('input[placeholder="最低薪资"]')
    await salary.setValue('8000')
    await salary.trigger('change')
    await flushPromises()

    expect(recruitApi.listResumes).toHaveBeenCalledTimes(2)
    expect(vi.mocked(recruitApi.listResumes).mock.calls[1][0]).toMatchObject({ page: 1, salary_min: 8000 })
  })
})
