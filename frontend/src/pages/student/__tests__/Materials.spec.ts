// Materials 学习资料页双 tab（#517）：课程资料 / 学员投稿切换。
// 第十四波 B 票 10（ADR-0062 决策 10）：课程筛选选项此前是挂在 onMounted 上的旁路装载流，
// 切证件后选项仍是旧证件的课程 ⇒ 拿旧证件的 course_id 去过滤新证件 = 空列表；
// 现在它声明进 useAsyncPage 的 facets，与列表同一处判据。
import { describe, it, expect, vi, beforeEach } from 'vitest'
import { mount, flushPromises } from '@vue/test-utils'
import { epLite } from '@/test/element-lite'
import { createPinia, setActivePinia } from 'pinia'

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

import { materialApi } from '@/api/material'
import { courseApi } from '@/api/course'
import { useCredentialStore } from '@/stores/credential'
import type { CredentialDict } from '@/api/credential'
import UiPagination from '@/components/ui/UiPagination.vue'
import Materials from '../Materials.vue'

function credentialOf(id: number): CredentialDict {
  return { id, code: `C${id}`, name: `证件${id}`, description: '', category: 'special_operation', level: null, sort_order: 0, status: 1, created_at: '', updated_at: '' }
}

function materialOf(fileId: number) {
  return {
    file_id: fileId,
    file_name: `液压图 ${fileId}.pdf`,
    file_url: `/f/${fileId}.pdf`,
    file_size: 1024,
    content_type: 'document',
    course_id: 1,
    course_name: '叉车液压系统',
    chapter_id: 2,
    chapter_title: '液压原理',
    created_at: '2026-09-01T10:00:00+08:00'
  }
}

beforeEach(() => {
  // Materials 读当前证件（课程选项按证件分区 + 投稿 tab 跟随），需要激活的 pinia
  setActivePinia(createPinia())
  vi.mocked(materialApi.list).mockResolvedValue({ materials: [], total: 0, page: 1, pages: 1 })
  vi.mocked(courseApi.getCourses).mockResolvedValue({ courses: [], total: 0, page: 1, pages: 0 })
})

describe('Materials 学习资料页（#517 双 tab）', () => {
  it('渲染两个 tab 与默认「课程资料」', async () => {
    const w = mount(Materials, { global: { plugins: [epLite()] } })
    await flushPromises()
    expect(w.text()).toContain('学习资料')
    expect(w.text()).toContain('课程资料')
    expect(w.text()).toContain('学员投稿')
  })

  it('切到「学员投稿」渲染投稿 tab（stub 子组件）', async () => {
    const w = mount(Materials, {
      global: {
        plugins: [epLite()],
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

describe('Materials 课程选项 facet 随证件重装（ADR-0062 票 10）', () => {
  it('facet 首装随列表一次；切证件后与列表一起重装，读的是新证件', async () => {
    setActivePinia(createPinia())
    const store = useCredentialStore()
    store.current = credentialOf(1)
    vi.mocked(courseApi.getCourses).mockClear()
    vi.mocked(materialApi.list).mockClear()

    mount(Materials, { global: { plugins: [epLite()] } })
    await flushPromises()
    expect(courseApi.getCourses).toHaveBeenCalledTimes(1)
    expect(courseApi.getCourses).toHaveBeenLastCalledWith(expect.objectContaining({ credential_id: 1 }))

    store.current = credentialOf(2)
    await flushPromises()
    // 「选项 facet」与「资料列表」同批重装：不留下「旧证件的 course_id + 新证件的列表」那种错配
    expect(courseApi.getCourses).toHaveBeenCalledTimes(2)
    expect(courseApi.getCourses).toHaveBeenLastCalledWith(expect.objectContaining({ credential_id: 2 }))
    expect(materialApi.list).toHaveBeenCalledTimes(2)
  })

  it('翻页不重复拉课程选项（facet 不并入列表 loader，判据仍只有一处）', async () => {
    setActivePinia(createPinia())
    useCredentialStore().current = credentialOf(1)
    vi.mocked(materialApi.list).mockResolvedValue({
      materials: [materialOf(1)],
      total: 45,
      page: 1,
      pages: 3
    })
    vi.mocked(courseApi.getCourses).mockClear()
    const w = mount(Materials, { global: { plugins: [epLite()] } })
    await flushPromises()
    expect(courseApi.getCourses).toHaveBeenCalledTimes(1)

    const pag = w.findComponent(UiPagination)
    expect(pag.exists()).toBe(true)
    pag.vm.$emit('update:currentPage', 2)
    pag.vm.$emit('current-change', 2)
    await flushPromises()

    expect(materialApi.list).toHaveBeenLastCalledWith(expect.objectContaining({ page: 2 }))
    expect(courseApi.getCourses).toHaveBeenCalledTimes(1)
  })
})
