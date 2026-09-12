// 章节正文（发布端）渲染契约（ADR-0046 / #901 #903）。
//
// seam 同 ChapterDiscussion.spec：mount 真实页面，只 mock 网络层与重子组件。
// 这条用例守的是**页面真的挂了发布端渲染单点**——只测 PublishMarkdown 不能防住
// 「有人把 ChapterView 改回 v-html + marked」这种回退。
import { describe, it, expect, vi, beforeEach } from 'vitest'
import { mount, flushPromises } from '@vue/test-utils'
import { epLite } from '@/test/element-lite'

vi.mock('@/api/course', () => ({
  courseApi: {
    getChapterDetail: vi.fn(),
    updateProgress: vi.fn()
  }
}))

vi.mock('@/api/student', () => ({
  studentApi: { getStudentCourseDetail: vi.fn() }
}))

vi.mock('@/stores/course', () => ({
  useCourseStore: () => ({ loadCourse: vi.fn(), courseInfo: null, chapters: [] })
}))

vi.mock('@/stores/credential', () => ({
  useCredentialStore: () => ({ current: null })
}))

vi.mock('@/composables/useStudyTracker', () => ({
  REPORT_THRESHOLD_SECONDS: 60,
  useStudyTracker: () => ({
    studySeconds: { value: 0 },
    isStudying: { value: false },
    begin: vi.fn(),
    stop: vi.fn(),
    pause: vi.fn(),
    resume: vi.fn(),
    reportIncremental: vi.fn().mockResolvedValue(0)
  })
}))

vi.mock('vue-router', () => ({
  useRoute: () => ({ params: { courseId: '1', chapterId: '2' } }),
  useRouter: () => ({ push: vi.fn() })
}))

import { courseApi } from '@/api/course'
import { studentApi } from '@/api/student'
import ChapterView from '../ChapterView.vue'

const CHAPTER_CONTENT = [
  '# 扭矩计算',
  '',
  '| 参数 | 值 |',
  '| --- | --- |',
  '| 扭矩 | 100 |',
  '',
  '扭矩 $T = 9550P/n$ 时按此取值',
  '',
  '```js',
  'const a = 1',
  '```'
].join('\n')

function mountPage() {
  return mount(ChapterView, {
    global: {
      plugins: [epLite()],
      stubs: {
        ChapterDiscussion: true,
        VideoPlayer: true,
        DocumentViewer: true,
        PptViewer: true,
        ImageViewer: true
      }
    }
  })
}

beforeEach(() => {
  vi.mocked(courseApi.getChapterDetail).mockResolvedValue({
    chapter_id: 2,
    title: '扭矩计算',
    content: CHAPTER_CONTENT,
    files: []
  })
  vi.mocked(studentApi.getStudentCourseDetail).mockResolvedValue({ chapters: [] } as never)
})

describe('ChapterView 发布端渲染（#901 / #903）', () => {
  it('正文里的表格渲染成真正的表格元素', async () => {
    const w = mountPage()
    await flushPromises()
    expect(w.find('.markdown-body table').exists()).toBe(true)
    expect(w.text()).toContain('扭矩')
  })

  it('公式渲染成公式，而不是 `$...$` 源码', async () => {
    const w = mountPage()
    await flushPromises()
    expect(w.find('.markdown-body .katex').exists()).toBe(true)
    expect(w.find('.markdown-body').text()).not.toContain('$T = 9550P/n$')
  })

  it('代码块保留语法高亮（换渲染单点不许丢能力）', async () => {
    const w = mountPage()
    await flushPromises()
    expect(w.find('.markdown-body .hljs-keyword').exists()).toBe(true)
  })
})
