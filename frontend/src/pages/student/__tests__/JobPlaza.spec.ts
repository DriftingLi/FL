// #493 职位广场：方形网格 + 加载更多（追加、到底 END 态、筛选重置）。
import { describe, it, expect, vi, beforeEach } from 'vitest'
import { mount, flushPromises } from '@vue/test-utils'
import { epLite } from '@/test/element-lite'

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
  return mount(JobPlaza, { global: { plugins: [epLite()] } })
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
    // 空态不是错误态：不给「重试」入口
    expect(wrapper.findAll('button').some(b => b.text().includes('重试'))).toBe(false)
  })

  it('错误态：加载失败渲染错误态 + 重试（与空态互斥）', async () => {
    vi.mocked(jobApi.listPublicJobs).mockRejectedValue(new Error('boom'))
    const wrapper = mountPage()
    await flushPromises()
    expect(wrapper.text()).toContain('职位加载失败')
    expect(wrapper.text()).not.toContain('暂无招聘中的职位')
  })
})

// #1101 两条生产路径（useAsyncPage 的 isEmpty / filterDeps 第一次被真实页面穿过）：
// 判据不再是页面里的 `items.length === 0` 与手写 resetAndLoad，而是 composable 内的单点。
describe('JobPlaza 编排侧判据（#1101：isEmpty / filterDeps 的生产消费者）', () => {
  it('filterDeps：改筛选轴（无需 @change 回调）即清空累积 + 回第 1 批重装', async () => {
    const b1 = Array.from({ length: 20 }, (_, i) => mkJob(i + 1))
    const b2 = Array.from({ length: 20 }, (_, i) => mkJob(100 + i))
    vi.mocked(jobApi.listPublicJobs)
      .mockResolvedValueOnce({ items: b1, total: 40 } as any)
      .mockResolvedValueOnce({ items: b2, total: 40 } as any)

    const wrapper = mountPage()
    await flushPromises()
    expect(wrapper.text()).toContain('职位1')

    // 模拟筛选轴变化：控件 @change（失焦/回车）后生效快照变化 → filterDeps 触发重置重装
    const region = wrapper.find('input[placeholder="地区"]')
    await region.setValue('江苏苏州')
    await region.trigger('change')
    await flushPromises()

    expect(jobApi.listPublicJobs).toHaveBeenCalledTimes(2)
    // 回第 1 批：第 2 次请求不带 page 递进（page=1）
    expect(vi.mocked(jobApi.listPublicJobs).mock.calls[1][0]).toMatchObject({ page: 1, region: '江苏苏州' })
    // 旧批次不残留（累积被清空）：新结果的第 1 批 20 条都在，旧批次一条都不在
    const titles = wrapper.text().match(/职位\d+/g) ?? []
    expect(titles.filter(t => Number(t.slice(2)) >= 100)).toHaveLength(20)
    expect(titles.filter(t => Number(t.slice(2)) < 100)).toHaveLength(0)
  })
})
