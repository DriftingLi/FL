// 章节收藏入口（#1132）：章节页可收藏 / 取消收藏，进页回填既有状态。
//
// 为什么值得单独立锁：`chapter` 在收藏表里曾是**没有创建点的类型**（全仓零入口、生产 0 行），
// 而收藏页两端都把它当一等公民展示 ⇒ 入口消失不会有任何东西变红。本 spec 把「入口在」钉住。
//
// seam 同 ChapterContentView.spec：mount 真实页面，只 mock 网络层与重子组件。
import { describe, it, expect, vi, beforeEach } from 'vitest'
import { mount, flushPromises } from '@vue/test-utils'
import { epLite } from '@/test/element-lite'

vi.mock('@/api/course', () => ({
  courseApi: { getChapterDetail: vi.fn(), updateProgress: vi.fn() }
}))

vi.mock('@/api/student', () => ({
  studentApi: { getStudentCourseDetail: vi.fn() }
}))

vi.mock('@/api/favorite', () => ({
  favoriteApi: { check: vi.fn(), add: vi.fn(), remove: vi.fn() }
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
import { favoriteApi } from '@/api/favorite'
import ChapterView from '../ChapterView.vue'

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

/** 收藏药丸（UiActionChip 的根是 button，label 渲染成 span） */
function chip(w: ReturnType<typeof mountPage>) {
  const found = w.findAll('button').find((b) => b.text().includes('收藏'))
  if (!found) throw new Error('未找到章节收藏入口')
  return found
}

beforeEach(() => {
  vi.mocked(courseApi.getChapterDetail).mockResolvedValue({
    chapter_id: 2,
    title: '扭矩计算',
    content: '',
    files: [],
    content_type: 'markdown',
    course_id: 1,
    created_at: '',
    description: '',
    duration: 0,
    file_url: '',
    next_chapter_id: null,
    order_num: 0,
    previous_chapter_id: null,
    study_status: ''
  } as never)
  vi.mocked(studentApi.getStudentCourseDetail).mockResolvedValue({ chapters: [] } as never)
  vi.mocked(favoriteApi.check).mockReset()
  vi.mocked(favoriteApi.add).mockReset()
  vi.mocked(favoriteApi.remove).mockReset()
  vi.mocked(favoriteApi.check).mockResolvedValue({ favorited: false, favorite_id: 0 })
  vi.mocked(favoriteApi.add).mockResolvedValue({ favorite_id: 9 } as never)
  vi.mocked(favoriteApi.remove).mockResolvedValue(null as never)
})

describe('章节收藏入口（#1132）', () => {
  it('进页按当前章节查收藏状态（目标类型 = chapter，目标 = 章节 id）', async () => {
    const w = mountPage()
    await flushPromises()
    expect(favoriteApi.check).toHaveBeenCalledWith({ target_type: 'chapter', target_id: 2 })
    expect(chip(w).text()).toContain('收藏')
    expect(chip(w).text()).not.toContain('已收藏')
  })

  it('已收藏时回填为「已收藏」态', async () => {
    vi.mocked(favoriteApi.check).mockResolvedValue({ favorited: true, favorite_id: 7 })
    const w = mountPage()
    await flushPromises()
    expect(chip(w).text()).toContain('已收藏')
  })

  it('点击收藏：add(chapter, 章节 id) 并切到已收藏态', async () => {
    const w = mountPage()
    await flushPromises()
    await chip(w).trigger('click')
    await flushPromises()
    expect(favoriteApi.add).toHaveBeenCalledWith({ target_type: 'chapter', target_id: 2 })
    expect(chip(w).text()).toContain('已收藏')
  })

  it('已收藏时点击：按 favorite_id 取消收藏并切回未收藏态', async () => {
    vi.mocked(favoriteApi.check).mockResolvedValue({ favorited: true, favorite_id: 7 })
    const w = mountPage()
    await flushPromises()
    await chip(w).trigger('click')
    await flushPromises()
    expect(favoriteApi.remove).toHaveBeenCalledWith(7)
    expect(chip(w).text()).not.toContain('已收藏')
  })

  it('查收藏状态失败不阻断页面（仍渲染章节正文入口）', async () => {
    vi.mocked(favoriteApi.check).mockRejectedValue(new Error('network'))
    const w = mountPage()
    await flushPromises()
    expect(chip(w).text()).toContain('收藏')
    expect(courseApi.getChapterDetail).toHaveBeenCalled()
  })
})
