// #491 编辑页：回显不丢失（#486 语义迁移至此）、保存后回预览。
import { describe, it, expect, vi, beforeEach } from 'vitest'
import { mount, flushPromises } from '@vue/test-utils'
import ElementPlus from 'element-plus'

vi.mock('element-china-area-data', () => ({
  pcTextArr: [
    { label: '江苏省', children: [{ label: '苏州市' }, { label: '南京市' }] },
    { label: '浙江省', children: [{ label: '杭州市' }] },
  ],
}))
vi.mock('@/api/resume', () => ({
  resumeApi: { get: vi.fn(), save: vi.fn(), uploadImage: vi.fn() },
}))
vi.mock('@/api/request', () => ({
  unwrappedRequest: { get: vi.fn() },
}))
vi.mock('vue-router', () => ({
  useRouter: () => ({ push: vi.fn() }),
}))

import { resumeApi } from '@/api/resume'
import { unwrappedRequest } from '@/api/request'
import ResumeEdit from '../ResumeEdit.vue'

function mountPage() {
  return mount(ResumeEdit, { global: { plugins: [ElementPlus] } })
}

beforeEach(() => {
  vi.mocked(unwrappedRequest.get).mockReset()
  vi.mocked(unwrappedRequest.get).mockResolvedValue({ positions: [], credentials: [] })
  vi.mocked(resumeApi.save).mockReset()
  vi.mocked(resumeApi.save).mockResolvedValue({} as any)
})

describe('ResumeEdit 意向地区回显（#486）', () => {
  it('已存两段「省/市」数据回显为级联路径（重进不丢失）', { timeout: 15000 }, async () => {
    vi.mocked(resumeApi.get).mockResolvedValue({
      user_id: 1, real_name: '张三', contact_phone: '13800000001', wechat: '', region: '江苏省/苏州市',
      expected_position_extra: '', expected_regions: ['江苏省/苏州市', '浙江省/杭州市'], salary_negotiable: false,
      available_in: '', job_nature: '', experience_years: 1, self_intro: '',
      resume_experiences: [], resume_certifications: [], resume_file_url: '', photos: [],
      visibility: 'hidden', created_at: '', updated_at: '',
    } as any)
    const wrapper = mountPage()
    await flushPromises()
    const vm: any = wrapper.vm
    expect(vm.expectedRegionsCascader).toEqual([['江苏省', '苏州市'], ['浙江省', '杭州市']])
    expect(vm.regionCascader).toEqual(['江苏省', '苏州市'])
  })
})

describe('ResumeEdit 预览/编辑分离（#491）', () => {
  it('编辑页显示编辑入口且不含 PDF 上传按钮（附作为只读链接）', { timeout: 15000 }, async () => {
    vi.mocked(resumeApi.get).mockResolvedValue({
      user_id: 1, real_name: '张三', contact_phone: '13800000001', wechat: '', region: '',
      expected_position_extra: '', expected_regions: [], salary_negotiable: false,
      available_in: '', job_nature: '', experience_years: 1, self_intro: '',
      resume_experiences: [], resume_certifications: [], resume_file_url: '/static/uploads/x.pdf', photos: [],
      visibility: 'hidden', created_at: '', updated_at: '',
    } as any)
    const wrapper = mountPage()
    await flushPromises()
    expect(wrapper.text()).toContain('编辑简历')
    expect(wrapper.text()).toContain('查看当前附件')
    expect(wrapper.text()).not.toContain('选择 PDF')
  })
})
