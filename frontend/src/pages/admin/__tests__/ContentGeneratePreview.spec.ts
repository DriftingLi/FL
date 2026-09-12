// 管理端 AI 生成内容的预览（ADR-0046 / #903）。
//
// 现状问题：这条预览走的是裸 marked.parse + v-html，**连代码高亮都没有**，
// 于是「AI 生成的正文能不能直接采用」在管理端看不出来。
// 本用例把它钉在与发布端同源：预览里的代码块必须带 hljs 高亮。
import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import { mount, flushPromises } from '@vue/test-utils'
import { epLite } from '@/test/element-lite'

vi.mock('@/api/admin', () => ({
  adminApi: {
    getCourses: vi.fn(),
    getCourseDetail: vi.fn(),
    generateContent: vi.fn(),
    getGenerateStatus: vi.fn()
  }
}))

import { adminApi } from '@/api/admin'
import ContentGenerate from '../ContentGenerate.vue'

const GENERATED = ['## 故障排查', '', '```js', 'const code = "E01"', '```'].join('\n')

let wrapper: ReturnType<typeof mount> | null = null

beforeEach(() => {
  vi.mocked(adminApi.getCourses).mockResolvedValue({ courses: [{ course_id: 1, name: '叉车维修' }] } as never)
  vi.mocked(adminApi.getCourseDetail).mockResolvedValue({ chapters: [{ chapter_id: 11, title: '第一章', content: '' }] } as never)
  vi.mocked(adminApi.generateContent).mockResolvedValue({
    task_id: 't1',
    status: 'completed',
    total: 1,
    completed: 1,
    results: [{ chapter_id: 11, title: '第一章', status: 'success', content: GENERATED }]
  } as never)
})

afterEach(() => {
  wrapper?.unmount()
  wrapper = null
})

describe('ContentGenerate 预览（#903）', () => {
  it('预览与发布端同源：代码块有语法高亮，标题渲染成标题', async () => {
    wrapper = mount(ContentGenerate, { global: { plugins: [epLite()] } })
    await flushPromises()

    const select = wrapper.findComponent({ name: 'ElSelect' })
    select.vm.$emit('update:modelValue', 1)
    select.vm.$emit('change', 1)
    await flushPromises()

    const checkboxGroup = wrapper.findComponent({ name: 'UiCheckboxGroup' })
    checkboxGroup.vm.$emit('update:modelValue', [11])
    await flushPromises()

    const generateButton = wrapper.findAll('button').find((b) => b.text().includes('开始生成'))
    expect(generateButton).toBeTruthy()
    await generateButton!.trigger('click')
    await flushPromises()

    const previewButton = wrapper.findAll('button').find((b) => b.text() === '预览')
    expect(previewButton).toBeTruthy()
    await previewButton!.trigger('click')
    await flushPromises()

    const dialog = wrapper.find('.el-dialog')
    expect(dialog.exists()).toBe(true)
    expect(dialog.find('h2').exists()).toBe(true)
    expect(dialog.find('.hljs-keyword').exists()).toBe(true)
    expect(dialog.text()).toContain('故障排查')
  })
})
