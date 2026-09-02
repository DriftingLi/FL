// #489 简历详情按钮状态机：none 显示申请、pending 禁用+提示、approved 无申请按钮直接明文。
import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import { mount, flushPromises } from '@vue/test-utils'
import ElementPlus from 'element-plus'

vi.mock('vue-router', () => ({ useRoute: () => ({ params: { id: '1' } }) }))
vi.mock('@/api/recruit', () => ({ recruitApi: { getResume: vi.fn(), getContact: vi.fn(), createContactRequest: vi.fn() } }))
vi.mock('@/api/client', () => ({ getValidAccessToken: vi.fn(() => Promise.resolve('tk')) }))
vi.mock('@/components/recruit/OnlineResumePdf.vue', () => ({ default: { template: '<div class="mock-pdf">PDF</div>' } }))

import { recruitApi } from '@/api/recruit'
import ResumeDetail from '../ResumeDetail.vue'

const card = { user_id: 5, real_name_masked: '张*', real_name: '张*', expected_regions: [], updated_at: '2026-09-01T00:00:00Z' }

function mountPage() { return mount(ResumeDetail, { global: { plugins: [ElementPlus] } }) }

beforeEach(() => {
  vi.mocked(recruitApi.getResume).mockReset()
  vi.mocked(recruitApi.getContact).mockReset()
  vi.mocked(recruitApi.getContact).mockRejectedValue(Object.assign(new Error('无有效授权'), { response: { status: 403 } }))
  vi.mocked(recruitApi.createContactRequest).mockReset()
  vi.stubGlobal('setInterval', vi.fn(() => 1))
  vi.stubGlobal('clearInterval', vi.fn())
})

afterEach(() => { vi.unstubAllGlobals() })

describe('ResumeDetail 按钮状态机（#489）', () => {
  it('contact_state=none：显示「申请交换联系方式」', async () => {
    vi.mocked(recruitApi.getResume).mockResolvedValue({ ...card } as any)
    const wrapper = mountPage()
    await flushPromises()
    expect(wrapper.text()).toContain('申请交换联系方式')
  })

  it('contact_state=pending：按钮禁用并提示等待处理', async () => {
    vi.mocked(recruitApi.getResume).mockResolvedValue({ ...card, contact_state: 'pending' } as any)
    const wrapper = mountPage()
    await flushPromises()
    expect(wrapper.text()).toContain('等待学员处理中')
    expect(wrapper.text()).not.toContain('申请交换联系方式')
  })

  it('contact_state=approved：不显示申请按钮，直接展示明文联系区块', async () => {
    vi.mocked(recruitApi.getResume).mockResolvedValue({ ...card, contact_state: 'approved' } as any)
    vi.mocked(recruitApi.getContact).mockResolvedValue({ real_name: '张三丰', contact_phone: '13800009999', wechat: 'wx', resume_file_url: '', photos: [], resume_certifications: [] } as any)
    const wrapper = mountPage()
    await flushPromises()
    expect(wrapper.text()).not.toContain('申请交换联系方式')
    expect(wrapper.text()).toContain('13800009999')
    expect(wrapper.text()).toContain('已授权联系信息')
  })
})
