// #411 巡检问答积分流水视图契约（组件层）：默认请求锁定问答域 + 按域量词渲染 + 分页透出。
import { describe, it, expect, vi, beforeEach } from 'vitest'
import { mount, flushPromises } from '@vue/test-utils'
import ElementPlus from 'element-plus'

vi.mock('@/api/request', () => ({
  unwrappedRequest: { get: vi.fn() },
}))

import { unwrappedRequest } from '@/api/request'
import InspectionView from '../Inspection.vue'

function mountView() {
  return mount(InspectionView, { global: { plugins: [ElementPlus] } })
}

beforeEach(() => {
  vi.mocked(unwrappedRequest.get).mockReset()
  vi.mocked(unwrappedRequest.get).mockResolvedValueOnce({ count: 3 })
  vi.mocked(unwrappedRequest.get).mockResolvedValueOnce({
    items: [
      { id: 1, user_id: 9, reason: 'accepted_bonus', delta: 40, ref_type: 'forum_topic', ref_id: '42', created_at: '2026-08-31T10:00:00Z' },
      { id: 2, user_id: 9, reason: 'daily_checkin', delta: 5, ref_type: 'task', ref_id: 'daily_checkin', created_at: '2026-08-31T09:00:00Z' },
    ],
    total: 2,
  })
})

describe('Inspection 问答积分流水（#411）', () => {
  it('组件默认请求携带 ref_type=forum_topic（问答域锁定）', async () => {
    mountView()
    await flushPromises()
    const get = vi.mocked(unwrappedRequest.get)
    const ledgerCall = get.mock.calls.find(c => String(c[0]).includes('/admin/points/ledger'))
    expect(ledgerCall).toBeTruthy()
    const params = ledgerCall![1] ? ledgerCall![1].params : undefined
    expect(params && params.ref_type).toBe('forum_topic')
  })

  it('行内引用按业务域渲染量词：任务行不显示「帖 …」', async () => {
    const wrapper = mountView()
    await flushPromises()
    expect(wrapper.text()).toContain('帖 42')
    expect(wrapper.text()).toContain('任务 daily_checkin')
  })

  it('渲染分页控件并透出 total', async () => {
    const wrapper = mountView()
    await flushPromises()
    expect(wrapper.findComponent({ name: 'ElPagination' }).exists()).toBe(true)
  })

  it('切换为跨业务域全量后不再携带 ref_type', async () => {
    const wrapper = mountView()
    await flushPromises()
    const select = wrapper.findComponent({ name: 'ElSelect' })
    await select.setValue('')
    await select.trigger('change')
    await flushPromises()
    const get = vi.mocked(unwrappedRequest.get)
    const ledgerCalls = get.mock.calls.filter(c => String(c[0]).includes('/admin/points/ledger'))
    const ledgerCall = ledgerCalls[ledgerCalls.length - 1]
    const params = ledgerCall[1] ? ledgerCall[1].params : undefined
    expect(params && params.ref_type ? params.ref_type : '').toBe('')
  })
})
