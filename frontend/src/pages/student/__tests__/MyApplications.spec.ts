// spec #449 T3 #452：撤回弹窗的「一并撤回联系方式授权」必须默认不勾选——
// 决定 10（撤回不连带）的 UI 落点，HTTP 面不可见，因此用页面 spec 守住。
import { describe, it, expect, vi, beforeEach } from 'vitest'
import { mount, flushPromises } from '@vue/test-utils'
import ElementPlus from 'element-plus'

vi.mock('@/api/job', () => ({
  jobApi: { listMyApplications: vi.fn(), withdrawApplication: vi.fn() },
}))

import { jobApi } from '@/api/job'
import MyApplications from '../MyApplications.vue'

const app = {
  id: 1, job_posting_id: 10, job_title: '叉车维修技师', recruiter_id: 1, student_user_id: 2,
  status: 'applied', resume_updated_at: '2026-09-01T00:00:00Z',
  created_at: '2026-09-01T00:00:00Z', updated_at: '2026-09-01T00:00:00Z', company_name: '测试企业',
}

function mountPage() {
  return mount(MyApplications, { global: { plugins: [ElementPlus] } })
}

beforeEach(() => {
  vi.mocked(jobApi.listMyApplications).mockReset()
  vi.mocked(jobApi.listMyApplications).mockResolvedValue({ items: [app], total: 1, page: 1, page_size: 20 } as any)
  vi.mocked(jobApi.withdrawApplication).mockReset()
  vi.mocked(jobApi.withdrawApplication).mockResolvedValue({ ...app, status: 'withdrawn' } as any)
})

describe('MyApplications 撤回弹窗（#452 决定 10 UI 落点）', () => {
  it('撤回弹窗的「一并撤回联系方式授权」默认不勾选', async () => {
    const wrapper = mountPage()
    await flushPromises()
    await wrapper.find('button').trigger('click')
    await flushPromises()
    const checkbox = wrapper.find('input[type="checkbox"]')
    expect((checkbox.element as HTMLInputElement).checked).toBe(false)
    expect(wrapper.text()).toContain('一并撤回对该企业的联系方式授权')
  })

  it('默认撤回调用 withdrawApplication(false)——不连带收回授权', async () => {
    const wrapper = mountPage()
    await flushPromises()
    await wrapper.find('button').trigger('click')
    await flushPromises()
    const confirmBtn = wrapper.findAll('button').find((b) => b.text().includes('确认撤回'))!
    await confirmBtn.trigger('click')
    await flushPromises()
    expect(jobApi.withdrawApplication).toHaveBeenCalledWith(1, false)
  })
})