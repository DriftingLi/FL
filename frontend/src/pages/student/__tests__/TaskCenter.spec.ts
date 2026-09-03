// 任务中心三态契约（#409）：领取成功/业务幂等失败后按钮三态正确、提示只出现一次。
// seam：组件层，mock '@/api/points'（不依赖真实后端）；错误分类使用客户端 kind。
import { describe, it, expect, vi, beforeEach } from 'vitest'
import { mount, flushPromises } from '@vue/test-utils'
import { ElMessage } from 'element-plus'
import ElementPlus from 'element-plus'

vi.mock('@/api/points', () => ({
  pointsApi: {
    getBalance: vi.fn(),
    getTasks: vi.fn(),
    claim: vi.fn(),
  },
}))

import { pointsApi } from '@/api/points'
import type { PointsTaskItem } from '@/api/points'
import TaskCenter from '../TaskCenter.vue'

function mountPage() {
  return mount(TaskCenter, {
    global: { plugins: [ElementPlus] },
  })
}

function claimableTask(over: Partial<PointsTaskItem> = {}): PointsTaskItem {
  return {
    code: 'growth_mock',
    group: 'growth',
    title: '完成 1 次模考',
    desc: '每日完成 1 次模考',
    points: 20,
    status: 'claimable',
    progress: 1,
    total: 1,
    ...over,
  }
}

beforeEach(() => {
  vi.mocked(pointsApi.getBalance).mockResolvedValue({ balance: 12, total_earned: 340, total_spent: 120 })
  vi.mocked(pointsApi.getTasks).mockResolvedValue({
    tasks: [claimableTask()],
  })
  vi.mocked(pointsApi.claim).mockReset()
  // ElMessage 的方法类型是 MessageHandler；mock 实现返回 void 时需显式 cast（vue-tsc 校验）。
  ;(vi.spyOn(ElMessage, 'success') as unknown as { mockImplementation: (f: () => void) => unknown }).mockImplementation(() => {})
  ;(vi.spyOn(ElMessage, 'warning') as unknown as { mockImplementation: (f: () => void) => unknown }).mockImplementation(() => {})
  ;(vi.spyOn(ElMessage, 'error') as unknown as { mockImplementation: (f: () => void) => unknown }).mockImplementation(() => {})
})

describe('TaskCenter 领取三态与单次提示（#409）', () => {
  it('领取成功：按钮立即变已领取，且只弹一次成功 toast', async () => {
    vi.mocked(pointsApi.claim).mockResolvedValue({
      balance: 32,
      total_earned: 360,
      task_status: 'claimed',
    })
    const wrapper = mountPage()
    await flushPromises()
    expect(wrapper.text()).toContain('领取')
    await wrapper.find('button').trigger('click')
    await flushPromises()
    expect(wrapper.text()).toContain('已领取')
    expect(wrapper.find('button').exists()).toBe(false)
    expect(ElMessage.success).toHaveBeenCalledTimes(1)
    expect(ElMessage.error).not.toHaveBeenCalled()
  })

  it('业务幂等失败（kind=business）：按钮自愈为已领取，提示按分组语义只弹一次', async () => {
    const err = Object.assign(new Error('今日已领取'), { kind: 'business' })
    vi.mocked(pointsApi.claim).mockRejectedValue(err)
    const wrapper = mountPage()
    await flushPromises()
    await wrapper.find('button').trigger('click')
    await flushPromises()
    expect(wrapper.text()).toContain('已领取')
    expect(wrapper.find('button').exists()).toBe(false)
    expect(ElMessage.warning).toHaveBeenCalledTimes(1)
    expect(ElMessage.warning).toHaveBeenCalledWith('今日已领取')
    expect(ElMessage.error).not.toHaveBeenCalled()
  })

  it('非幂等失败（kind=network）：保持可领取，弹一次错误提示', async () => {
    const err = Object.assign(new Error('网络异常'), { kind: 'network' })
    vi.mocked(pointsApi.claim).mockRejectedValue(err)
    const wrapper = mountPage()
    await flushPromises()
    const btn = wrapper.find('button')
    expect(btn.exists()).toBe(true)
    await btn.trigger('click')
    await flushPromises()
    expect(wrapper.find('button').exists()).toBe(true)
    expect(ElMessage.error).toHaveBeenCalledTimes(1)
    expect(ElMessage.error).toHaveBeenCalledWith('网络异常')
  })
})