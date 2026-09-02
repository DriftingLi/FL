vi.mock('element-china-area-data', () => ({
  pcTextArr: [
    { label: '江苏省', children: [{ label: '苏州市' }, { label: '南京市' }] },
    { label: '浙江省', children: [{ label: '杭州市' }] },
    { label: '北京市', children: [] },
    { label: '上海市', children: [] },
  ],
}))

// #415 简历未建空态：未建（契约内 404）时不弹报错、渲染空态引导；已建时渲染表单。
import { describe, it, expect, vi, beforeEach } from 'vitest'
import { mount, flushPromises } from '@vue/test-utils'
import ElementPlus from 'element-plus'

vi.mock('@/api/resume', () => ({
  resumeApi: { get: vi.fn(), getViewStats: vi.fn() },
}))
vi.mock('@/api/request', () => ({
  unwrappedRequest: { get: vi.fn() },
}))
vi.mock('vue-router', () => ({
  useRouter: () => ({ push: vi.fn() }),
}))

import { resumeApi } from '@/api/resume'
import { unwrappedRequest } from '@/api/request'
import ResumePage from '../ResumePage.vue'

function mountPage() {
  return mount(ResumePage, { global: { plugins: [ElementPlus] } })
}

beforeEach(() => {
  vi.mocked(unwrappedRequest.get).mockReset()
  vi.mocked(unwrappedRequest.get).mockResolvedValue({ positions: [], credentials: [] })
  vi.mocked(resumeApi.getViewStats).mockResolvedValue({ count: 0 })
})

describe('ResumePage 未建简历空态（#415）', () => {
  it('契约内 404（kind=notfound）：不弹错误提示，渲染「完善后可被招聘企业看到」引导', { timeout: 15000 }, async () => {
    const err = Object.assign(new Error('简历不存在'), { kind: 'notfound' })
    vi.mocked(resumeApi.get).mockRejectedValue(err)
    const wrapper = mountPage()
    await flushPromises()
    expect(wrapper.text()).toContain('简历尚未创建')
    expect(wrapper.text()).toContain('完善后可被招聘企业看到')
    expect(wrapper.text()).toContain('去完善')
  })

  it('已建简历：渲染编辑表单（真实姓名输入框存在）', { timeout: 15000 }, async () => {
    vi.mocked(resumeApi.get).mockResolvedValue({
      user_id: 1, real_name: '张三', contact_phone: '13800000001', wechat: '', region: '上海',
      expected_position_extra: '', expected_regions: [], salary_negotiable: false,
      available_in: '', job_nature: '', experience_years: 1, self_intro: '',
      resume_experiences: [], resume_certifications: [], resume_file_url: '', photos: [],
      visibility: 'hidden', created_at: '', updated_at: '',
    })
    const wrapper = mountPage()
    await flushPromises()
    expect(wrapper.text()).toContain('真实姓名')
    expect(wrapper.text()).not.toContain('简历尚未创建')
  })
})

describe('ResumePage 意向地区回显（#486）', () => {
  it('已存两段「省/市」数据回显为级联路径（重进不丢失）', { timeout: 15000 }, async () => {
    vi.mocked(resumeApi.get).mockResolvedValue({
      user_id: 1, real_name: '张三', contact_phone: '13800000001', wechat: '', region: '江苏省/苏州市',
      expected_position_extra: '', expected_regions: ['江苏省/苏州市', '浙江省/杭州市'], salary_negotiable: false,
      available_in: '', job_nature: '', experience_years: 1, self_intro: '',
      resume_experiences: [], resume_certifications: [], resume_file_url: '', photos: [],
      visibility: 'hidden', created_at: '', updated_at: '',
    })
    const wrapper = mountPage()
    await flushPromises()
    const vm: any = wrapper.vm
    expect(vm.expectedRegionsCascader).toEqual([['江苏省', '苏州市'], ['浙江省', '杭州市']])
    expect(vm.regionCascader).toEqual(['江苏省', '苏州市'])
  })
})
