// spec #449 T3 #452：投递确认弹窗必须写明「投递即授权…与简历是否公开无关」——
// 这条领域语义只承载在 UI（HTTP 面不可见），因此用页面 spec 守住。
import { describe, it, expect, vi, beforeEach } from 'vitest'
import { mount, flushPromises } from '@vue/test-utils'
import ElementPlus from 'element-plus'

vi.mock('@/api/job', () => ({
  jobApi: { getPublicJob: vi.fn(), applyJob: vi.fn() },
}))
vi.mock('vue-router', () => ({
  useRoute: () => ({ params: { id: '1' } }),
}))

import { jobApi } from '@/api/job'
import JobDetail from '../JobDetail.vue'

const job = {
  id: 1, recruiter_id: 1, title: '叉车维修技师', specialty_id: 1, specialty_name: '叉车维修',
  region: '江苏苏州', salary_text: '6-9K', experience_req: '2年', description: '日常维修',
  status: 'open', forced_offline: false, published_at: '2026-09-01T00:00:00Z',
  created_at: '2026-09-01T00:00:00Z', updated_at: '2026-09-01T00:00:00Z',
  company_name: '测试企业', business_scope: '叉车维修', contact_name: '王五',
}

function mountPage() {
  return mount(JobDetail, { global: { plugins: [ElementPlus] } })
}

beforeEach(() => {
  vi.mocked(jobApi.getPublicJob).mockReset()
  vi.mocked(jobApi.getPublicJob).mockResolvedValue(job as any)
  vi.mocked(jobApi.applyJob).mockReset()
  vi.mocked(jobApi.applyJob).mockResolvedValue({ id: 1 } as any)
})

describe('JobDetail 投递确认弹窗（#452 决定 1 UI 落点）', () => {
  it('投递弹窗明确写出「投递即授权该企业查看你的联系方式，与简历是否公开无关」', async () => {
    const wrapper = mountPage()
    await flushPromises()
    await wrapper.find('button').trigger('click')
    await flushPromises()
    expect(wrapper.text()).toContain('投递即授权该企业查看你的联系方式')
    expect(wrapper.text()).toContain('与简历是否公开无关')
  })

  it('确认投递调用 applyJob 并提示成功', async () => {
    const wrapper = mountPage()
    await flushPromises()
    await wrapper.find('button').trigger('click')
    await flushPromises()
    const confirmBtn = wrapper.findAll('button').find((b) => b.text().includes('确认投递'))!
    await confirmBtn.trigger('click')
    await flushPromises()
    expect(jobApi.applyJob).toHaveBeenCalledWith(1)
  })
})