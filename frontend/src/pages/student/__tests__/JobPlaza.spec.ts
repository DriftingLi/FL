// #493 职位广场：方形网格 + 加载更多（追加、到底 END 态、筛选重置）。
import { describe, it, expect, vi, beforeEach } from 'vitest'
import { mount, flushPromises } from '@vue/test-utils'
import ElementPlus from 'element-plus'

vi.mock('@/api/job', () => ({
  jobApi: { listPublicJobs: vi.fn() },
}))
vi.mock('@/api/request', () => ({
  unwrappedRequest: { get: vi.fn() },
}))

import { jobApi } from '@/api/job'
import { unwrappedRequest } from '@/api/request'
import JobPlaza from '../JobPlaza.vue'

function mkJob(id: number) {
  return {
    id, recruiter_id: 1, title: `职位${id}`, region: '江苏苏州', salary_text: '6-9K',
    status: 'open', forced_offline: false, published_at: '2026-09-01T00:00:00Z',
    created_at: '2026-09-01T00:00:00Z', updated_at: '2026-09-01T00:00:00Z', company_name: '企业',
  }
}

function mountPage() {
  return mount(JobPlaza, { global: { plugins: [ElementPlus] } })
}

beforeEach(() => {
  vi.mocked(jobApi.listPublicJobs).mockReset()
  vi.mocked(unwrappedRequest.get).mockReset()
  vi.mocked(unwrappedRequest.get).mockResolvedValue({ positions: [] })
})

describe('JobPlaza 加载更多（#493）', () => {
  it('首批 20 条：显示加载更多；点加载更多追加第 2 批', async () => {
    const batch1 = Array.from({ length: 20 }, (_, i) => mkJob(i + 1))
    const batch2 = Array.from({ length: 5 }, (_, i) => mkJob(21 + i))
    vi.mocked(jobApi.listPublicJobs).mockResolvedValueOnce({ items: batch1, total: 25 } as any)
      .mockResolvedValueOnce({ items: batch2, total: 25 } as any)
    const wrapper = mountPage()
    await flushPromises()
    const btn = wrapper.findAll('button').find((b) => b.text().includes('加载更多'))!
    expect(btn.exists()).toBe(true)
    await btn.trigger('click')
    await flushPromises()
    // 卡片以「职位N」标题渲染（router-link 不依赖真实路由）
    expect(wrapper.text()).toContain('职位25')
    expect(wrapper.text()).toContain('职位1')
    expect(wrapper.text()).toContain('没有更多了')
  })

  it('不足 20 条首批直接 END', async () => {
    vi.mocked(jobApi.listPublicJobs).mockResolvedValue({ items: [mkJob(1)], total: 1 } as any)
    const wrapper = mountPage()
    await flushPromises()
    expect(wrapper.text()).toContain('没有更多了')
    const btn = wrapper.findAll('button').find((b) => b.text().includes('加载更多'))
    expect(btn).toBeUndefined()
  })

  it('空列表显示空态', async () => {
    vi.mocked(jobApi.listPublicJobs).mockResolvedValue({ items: [], total: 0 } as any)
    const wrapper = mountPage()
    await flushPromises()
    expect(wrapper.text()).toContain('暂无招聘中的职位')
  })
})
