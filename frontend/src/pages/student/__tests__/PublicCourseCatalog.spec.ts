import { flushPromises, mount } from '@vue/test-utils'
import { createPinia } from 'pinia'
import { describe, expect, it, vi } from 'vitest'
import { courseApi } from '@/api/course'
import { trainingApi } from '@/api/training'
import { testRouter } from '@/test/router'
import PublicCourseCatalog from '../PublicCourseCatalog.vue'

vi.mock('@/api/course', () => ({ courseApi: { getPublicCourses: vi.fn() } }))
vi.mock('@/api/training', () => ({ trainingApi: { getCategories: vi.fn() } }))

describe('学员课程目录', () => {
  it('从地址读取一级分类，切换分类后回到第一页', async () => {
    vi.mocked(trainingApi.getCategories).mockResolvedValue({ categories: [{ id: 4, name: '维修' }, { id: 5, name: '安全' }] })
    vi.mocked(courseApi.getPublicCourses).mockResolvedValue({
      courses: [{ course_id: 17, name: '电路维修', description: '', cover_image: '', category: { id: 4, name: '维修' } }],
      page: 1, pages: 1, total: 1
    })
    const router = testRouter()
    await router.push('/training/courses?category_id=4')
    const wrapper = mount(PublicCourseCatalog, {
      global: { plugins: [router, createPinia()], stubs: { UiSelect: true, UiTag: true } }
    })
    try {
      await flushPromises()
      expect(courseApi.getPublicCourses).toHaveBeenLastCalledWith({ category_id: 4, page: 1, filter: undefined })
      expect(wrapper.get('a[href="/training/course/17"]').text()).toContain('电路维修')

      wrapper.getComponent({ name: 'UiSelect' }).vm.$emit('update:modelValue', 5)
      await flushPromises()
      expect(router.currentRoute.value.query.category_id).toBe('5')
      expect(courseApi.getPublicCourses).toHaveBeenLastCalledWith({ category_id: 5, page: 1, filter: undefined })
    } finally {
      wrapper.unmount()
    }
  })
})
