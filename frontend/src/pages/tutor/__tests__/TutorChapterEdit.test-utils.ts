import { afterEach, beforeEach, expect, vi } from 'vitest'
import { flushPromises, mount, type VueWrapper } from '@vue/test-utils'
import { defineComponent, h } from 'vue'
import { ElMessage } from 'element-plus'
import { epLite } from '@/test/element-lite'

const { confirm, confirmDanger } = vi.hoisted(() => ({
  confirm: vi.fn(),
  confirmDanger: vi.fn()
}))

vi.mock('@/composables/useConfirm', () => ({
  useConfirm: () => ({ confirm, confirmDanger })
}))

vi.mock('@/api/tutor', () => ({
  tutorApi: {
    getChapterDetail: vi.fn(),
    getCourseChapters: vi.fn(),
    getCourseClaimants: vi.fn(),
    claimCourse: vi.fn(),
    updateChapter: vi.fn(),
    submitChapterReview: vi.fn(),
    deleteFile: vi.fn()
  }
}))

vi.mock('@/api/tutorfile', () => ({
  tutorFileApi: {
    listFiles: vi.fn(),
    attachFile: vi.fn(),
    detachFile: vi.fn(),
    swapChapterFile: vi.fn()
  }
}))

vi.mock('@/stores/credential', () => ({
  useCredentialStore: () => ({ current: null })
}))

const routeState = vi.hoisted(() => ({
  params: { courseId: '1', chapterId: '2' },
  route: null as { params: { courseId: string; chapterId: string } } | null,
  push: vi.fn()
}))

vi.mock('vue-router', async () => {
  const { reactive } = await import('vue')
  const route = reactive(routeState)
  routeState.route = route
  return {
    useRoute: () => route,
    useRouter: () => ({ push: routeState.push })
  }
})

import { tutorApi, type TutorChapterDetail } from '@/api/tutor'
import { tutorFileApi, type FileDTO } from '@/api/tutorfile'
import type { ChapterFile } from '@/api/course'
import TutorChapterEdit from '../TutorChapterEdit.vue'

export { confirm, routeState }
export type { TutorChapterDetail }

export function getTutorApi() {
  return tutorApi
}

export function getTutorFileApi() {
  return tutorFileApi
}

const MarkdownEditorStub = defineComponent({
  name: 'MarkdownEditor',
  props: { modelValue: { type: String, default: '' } },
  emits: ['update:modelValue'],
  setup(props, { expose, emit }) {
    expose({ getValue: () => props.modelValue })
    return () => h('textarea', {
      class: 'md-editor-stub',
      value: props.modelValue,
      onInput: (event: Event) => emit('update:modelValue', (event.target as HTMLTextAreaElement).value)
    })
  }
})

const FileUploadStub = defineComponent({
  name: 'FileUpload',
  emits: ['upload-all'],
  setup(_, { emit }) {
    return () => h('button', {
      class: 'upload-all-stub',
      onClick: () => emit('upload-all', { total: 1, success: 1, failed: 0 })
    }, '完成上传')
  }
})

export function chapterOf(overrides: Partial<TutorChapterDetail> = {}): TutorChapterDetail {
  return {
    chapter_id: 2,
    course_id: 1,
    title: '扭矩计算',
    content: '已保存正文',
    content_type: 'markdown',
    description: '',
    duration: 30,
    file_url: '',
    files: [],
    next_chapter_id: null,
    previous_chapter_id: null,
    order_num: 1,
    resume_position: 0,
    review_status: 'draft',
    reject_reason: '',
    created_at: '',
    ...overrides
  }
}

export function chapterFileOf(overrides: Partial<ChapterFile> = {}): ChapterFile {
  return {
    chapter_id: 2,
    content_type: 'video',
    created_at: '',
    file_id: 11,
    file_name: '原视频.mp4',
    file_size: 1024,
    file_url: '/old.mp4',
    source: 'chapter_file',
    ...overrides
  }
}

export function libraryFileOf(overrides: Partial<FileDTO> = {}): FileDTO {
  return {
    chapter_id: 0,
    chapter_title: '',
    content_type: 'video',
    created_at: '',
    file_id: 12,
    file_name: '库视频.mp4',
    file_size: 2048,
    file_url: '/library.mp4',
    ...overrides
  }
}

export function mountPage() {
  const page = mount(TutorChapterEdit, {
    global: {
      plugins: [epLite()],
      stubs: {
        MarkdownEditor: MarkdownEditorStub,
        FileUpload: FileUploadStub,
        VideoPlayer: true,
        DocumentViewer: true,
        PptViewer: true
      }
    }
  })
  mountedPages.push(page)
  return page
}

export function button(wrapper: VueWrapper, text: string) {
  const found = wrapper.findAll('button').find(item => item.text().includes(text))
  expect(found, `页面应有「${text}」按钮`).toBeTruthy()
  return found!
}

export async function editDraft(wrapper: VueWrapper, content: string) {
  await wrapper.find('.md-editor-stub').setValue(content)
}

export async function selectVideoTab(wrapper: VueWrapper) {
  const tab = wrapper.findAll('.el-tabs__item').find(item => item.text().includes('视频'))
  expect(tab, '应有视频文件 Tab').toBeTruthy()
  await tab!.trigger('click')
  await flushPromises()
}

const mountedPages: VueWrapper[] = []

beforeEach(() => {
  vi.restoreAllMocks()
  routeState.params = { courseId: '1', chapterId: '2' }
  routeState.push.mockReset()
  confirm.mockReset().mockResolvedValue('confirm')
  confirmDanger.mockReset().mockResolvedValue('confirm')
  vi.spyOn(ElMessage, 'success').mockImplementation(() => undefined as never)
  vi.spyOn(ElMessage, 'warning').mockImplementation(() => undefined as never)

  const getChapterDetail = vi.mocked(tutorApi.getChapterDetail)
  getChapterDetail.mockReset().mockImplementation(async () => chapterOf({
    review_status: getChapterDetail.mock.calls.length > 1 ? 'pending' : 'draft'
  }))
  vi.mocked(tutorApi.getCourseChapters).mockResolvedValue({ course: { name: '叉车维修' }, chapters: [] } as never)
  vi.mocked(tutorApi.getCourseClaimants).mockResolvedValue({ claimed_by_me: true, claimants: [] })
  vi.mocked(tutorApi.claimCourse).mockResolvedValue({ course_id: 1, claimed: true })
  vi.mocked(tutorApi.updateChapter).mockResolvedValue(chapterOf())
  vi.mocked(tutorApi.submitChapterReview).mockResolvedValue(chapterOf({ review_status: 'pending' }))
  vi.mocked(tutorApi.deleteFile).mockResolvedValue({ file_id: 11, deleted: true })
  vi.mocked(tutorFileApi.listFiles).mockResolvedValue({ files: [] })
  vi.mocked(tutorFileApi.attachFile).mockResolvedValue(null)
  vi.mocked(tutorFileApi.detachFile).mockResolvedValue(null)
  vi.mocked(tutorFileApi.swapChapterFile).mockResolvedValue(null)
})

afterEach(() => {
  mountedPages.splice(0).forEach(page => page.unmount())
})
