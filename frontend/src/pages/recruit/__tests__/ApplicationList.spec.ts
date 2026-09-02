// #490 投递列表详情抽屉：内嵌在线简历 PDF + 明文联系方式 + 标记不合适；旧指引文案已移除。
import { describe, it, expect, vi, beforeEach } from 'vitest'
import { mount, flushPromises } from '@vue/test-utils'
import ElementPlus from 'element-plus'

vi.mock('vue-router', () => ({
  useRoute: () => ({ params: { id: '1' } }),
}))
vi.mock('@/api/job', () => ({
  jobApi: {
    listJobApplications: vi.fn(),
    getApplicationDetail: vi.fn(),
    rejectApplication: vi.fn(),
  },
}))
vi.mock('@/api/recruit', () => ({
  recruitApi: { getContact: vi.fn() },
}))
vi.mock('@/components/recruit/OnlineResumePdf.vue', () => ({
  default: { template: '<div class="mock-pdf">PDF-PLACEHOLDER</div>' },
}))
vi.mock('@/api/client', () => ({
  getValidAccessToken: vi.fn(() => Promise.resolve('tk')),
}))

import { jobApi } from '@/api/job'
import { recruitApi } from '@/api/recruit'
import ApplicationList from '../ApplicationList.vue'

const appItem = {
  id: 10,
  job_posting_id: 1,
  student_user_id: 5,
  student_real_name_masked: '张*',
  status: 'applied',
  employer_viewed_at: null,
  created_at: '2026-09-01T00:00:00Z',
  resume_updated_at_snapshot: '2026-09-01T00:00:00Z',
  student_resume_updated_at: '2026-09-01T00:00:00Z',
}

function mountPage() {
  return mount(ApplicationList, { global: { plugins: [ElementPlus] } })
}

beforeEach(() => {
  vi.mocked(jobApi.listJobApplications).mockReset()
  vi.mocked(jobApi.listJobApplications).mockResolvedValue({ items: [appItem], total: 1, unread_count: 1, job_title: '测试职位' } as any)
  vi.mocked(jobApi.getApplicationDetail).mockReset()
  vi.mocked(jobApi.getApplicationDetail).mockResolvedValue({ ...appItem, employer_viewed_at: '2026-09-02T00:00:00Z' } as any)
  vi.mocked(jobApi.rejectApplication).mockReset()
  vi.mocked(recruitApi.getContact).mockReset()
  vi.mocked(recruitApi.getContact).mockResolvedValue({ real_name: '张三丰', contact_phone: '13800009999', wechat: 'wx', resume_file_url: '/static/x.pdf' } as any)
})

describe('ApplicationList 投递详情抽屉（#490）', () => {
  it('打开抽屉即加载明文联系方式并内嵌 PDF；旧指引文案已移除', async () => {
    const wrapper = mountPage()
    await flushPromises()
    const viewBtn = wrapper.findAll('button').find((b) => b.text().includes('查看'))!
    await viewBtn.trigger('click')
    await flushPromises()
    expect(jobApi.getApplicationDetail).toHaveBeenCalledWith(10)
    expect(recruitApi.getContact).toHaveBeenCalledWith(5)
    const text = wrapper.text()
    expect(text).toContain('13800009999')
    expect(text).toContain('张三丰')
    expect(text).not.toContain('请通过简历库的「申请交换联系方式」取得')
  })

  it('抽屉内可直接标记不合适', async () => {
  vi.mocked(jobApi.rejectApplication).mockResolvedValue({} as any)
    const wrapper = mountPage()
    await flushPromises()
    await wrapper.findAll('button').find((b) => b.text().includes('查看'))!.trigger('click')
    await flushPromises()
    const rejectBtn = wrapper.findAll('button').find((b) => b.text().includes('标记不合适'))!
    await rejectBtn.trigger('click')
    await flushPromises()
    expect(jobApi.rejectApplication).toHaveBeenCalledWith(10)
  })
})
