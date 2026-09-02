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
  resumeApi: { get: vi.fn(), getViewStats: vi.fn(), listContactRequests: vi.fn(), approveContactRequest: vi.fn(), rejectContactRequest: vi.fn(), revokeContactRequest: vi.fn() },
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
  vi.mocked(resumeApi.listContactRequests).mockResolvedValue({ items: [], total: 0 } as any)
})

describe('ResumePage 未建简历空态（#415）', () => {
  it('契约内 404（kind=notfound）：不弹错误提示，渲染「完善后可被招聘企业看到」引导', { timeout: 15000 }, async () => {
    const err = Object.assign(new Error('简历不存在'), { kind: 'notfound' })
    vi.mocked(resumeApi.get).mockRejectedValue(err)
    const wrapper = mountPage()
    await flushPromises()
    expect(wrapper.text()).toContain('简历尚未创建')
    expect(wrapper.text()).toContain('完善后可被招聘企业看到')
    expect(wrapper.text()).toContain('去填写')
  })

  it('已建简历：预览页展示编辑按钮/我的联系方式/内嵌 PDF（#491）', { timeout: 15000 }, async () => {
    vi.mocked(resumeApi.get).mockResolvedValue({
      user_id: 1, real_name: '张三', contact_phone: '13800000001', wechat: 'wx_zhang', region: '上海',
      expected_position_extra: '', expected_regions: [], salary_negotiable: false,
      available_in: '', job_nature: '', experience_years: 1, self_intro: '',
      resume_experiences: [], resume_certifications: [], resume_file_url: '/static/uploads/x.pdf', photos: [],
      visibility: 'open', created_at: '', updated_at: '',
    })
    const wrapper = mountPage()
    await flushPromises()
    expect(wrapper.text()).toContain('编辑简历')
    expect(wrapper.text()).toContain('我的联系方式（企业授权后可见）')
    expect(wrapper.text()).toContain('张三')
    expect(wrapper.text()).toContain('13800000001')
    expect(wrapper.text()).toContain('查看上传简历')
    expect(wrapper.text()).not.toContain('简历尚未创建')
  })
})

describe('ResumePage 企业联系方式透出（#487）', () => {
  it('approved 申请就地展开企业电话/微信，pending 不透出', { timeout: 15000 }, async () => {
    vi.mocked(resumeApi.get).mockResolvedValue({
      user_id: 1, real_name: '张三', contact_phone: '13800000001', wechat: '', region: '江苏省/苏州市',
      expected_position_extra: '', expected_regions: [], salary_negotiable: false,
      available_in: '', job_nature: '', experience_years: 1, self_intro: '',
      resume_experiences: [], resume_certifications: [], resume_file_url: '', photos: [],
      visibility: 'hidden', created_at: '', updated_at: '',
    })
    vi.mocked(resumeApi.listContactRequests).mockResolvedValue({
      items: [
        { id: 1, company_name: '测试企业A', contact_name: '王五', message: '想联系', status: 'approved', created_at: '2026-01-01', contact_phone: '13800001111', contact_email: 'a@example.com', wechat: 'wx_a' },
        { id: 2, company_name: '测试企业B', contact_name: '赵六', message: '考虑中', status: 'pending', created_at: '2026-01-02' },
      ],
      total: 2,
    })
    const wrapper = mountPage()
    await flushPromises()
    const text = wrapper.text()
    expect(text).toContain('13800001111')
    expect(text).toContain('wx_a')
    expect(text).toContain('测试企业A')
    // pending 项显示企业名但不透出电话（无 contact_phone 字段）
    expect(text).toContain('测试企业B')
    expect(text).not.toContain('wx_b')
  })
})
