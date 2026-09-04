// Materials 学习资料页双 tab（#517）：课程资料 / 学员投稿切换。
import { describe, it, expect, vi, beforeEach } from 'vitest'
import { mount, flushPromises } from '@vue/test-utils'
import ElementPlus from 'element-plus'

vi.mock('@/api/material', () => ({
  materialApi: { list: vi.fn() }
}))
vi.mock('@/api/course', () => ({
  courseApi: { getCourses: vi.fn() }
}))
vi.mock('@/api/contribution', () => ({
  contributionApi: {
    listPublic: vi.fn(),
    listMine: vi.fn(),
    uploadFile: vi.fn(),
    create: vi.fn(),
    download: vi.fn(),
    withdraw: vi.fn(),
    report: vi.fn()
  }
}))
vi.mock('@/stores/credential', () => ({
  useCredentialStore: () => ({ current: { id: 1 } })
}))

import { materialApi } from '@/api/material'
import { courseApi } from '@/api/course'
import Materials from '../Materials.vue'

beforeEach(() => {
  vi.mocked(materialApi.list).mockResolvedValue({ materials: [], total: 0, page: 1, pages: 1 })
  vi.mocked(courseApi.getCourses).mockResolvedValue({ courses: [], total: 0 })
})

describe('Materials 学习资料页（#517 双 tab）', () => {
  it('渲染两个 tab 与默认「课程资料」', async () => {
    const w = mount(Materials, { global: { plugins: [ElementPlus] } })
    await flushPromises()
    expect(w.text()).toContain('学习资料')
    expect(w.text()).toContain('课程资料')
    expect(w.text()).toContain('学员投稿')
  })

  it('切到「学员投稿」渲染投稿 tab（stub 子组件）', async () => {
    const w = mount(Materials, {
      global: {
        plugins: [ElementPlus],
        stubs: { ContributionTab: { template: '<div class="contrib-tab-stub">学员投稿区</div>' } }
      }
    })
    await flushPromises()
    const tabs = w.findAll('button')
    const contribTab = tabs.find((b) => b.text().includes('学员投稿'))
    expect(contribTab).toBeTruthy()
    await contribTab!.trigger('click')
    await flushPromises()
    expect(w.find('.contrib-tab-stub').exists()).toBe(true)
  })
})
