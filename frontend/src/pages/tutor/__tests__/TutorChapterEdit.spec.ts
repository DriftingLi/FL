// #2920 切片 2：章节基础编辑与文件操作会话。
import { describe, expect, it, vi } from 'vitest'
import { flushPromises } from '@vue/test-utils'
import { ElMessage } from 'element-plus'
import {
  button,
  chapterFileOf,
  chapterOf,
  confirm,
  editDraft,
  getTutorApi,
  getTutorFileApi,
  libraryFileOf,
  mountPage,
  selectVideoTab
} from './TutorChapterEdit.test-utils'

const tutorApi = getTutorApi()
const tutorFileApi = getTutorFileApi()

type FileOperation = 'upload' | 'attach' | 'swap' | 'detach' | 'delete'

async function runFileOperation(wrapper: ReturnType<typeof mountPage>, operation: FileOperation) {
  if (operation === 'upload') {
    await button(wrapper, '上传视频').trigger('click')
    await wrapper.find('.upload-all-stub').trigger('click')
  } else {
    await selectVideoTab(wrapper)
  }

  if (operation === 'attach' || operation === 'swap') {
    vi.mocked(tutorFileApi.listFiles).mockResolvedValue({ files: [libraryFileOf()] })
    if (operation === 'swap') {
      await button(wrapper, '换文件').trigger('click')
    } else {
      await button(wrapper, '从文件库挂接').trigger('click')
    }
    await flushPromises()
    const action = wrapper.findAll('button').find(item => item.text().trim() === (operation === 'swap' ? '换入' : '挂接'))
    expect(action, `文件库中应有${operation === 'swap' ? '换入' : '挂接'}动作`).toBeTruthy()
    await action!.trigger('click')
  } else if (operation === 'detach') {
    await wrapper.find('button[aria-label="取消挂接"]').trigger('click')
  } else if (operation === 'delete') {
    await wrapper.find('button[aria-label="删除文件"]').trigger('click')
  }
  await flushPromises()
}

describe('TutorChapterEdit 章节编辑会话（#2920）', () => {
  it('有未保存正文时点提审只提示先保存，不确认也不调用提审 API', async () => {
    const wrapper = mountPage()
    await flushPromises()
    await editDraft(wrapper, '尚未保存的正文')

    await button(wrapper, '提交审核').trigger('click')
    await flushPromises()

    expect(ElMessage.warning).toHaveBeenCalledWith('请先保存正文后再提交审核')
    expect(confirm).not.toHaveBeenCalled()
    expect(tutorApi.submitChapterReview).not.toHaveBeenCalled()
  })

  it.each(['upload', 'attach', 'swap', 'detach', 'delete'] as const)(
    '%s 后详情返回另一份正文，文件事实刷新仍保留本地草稿',
    async operation => {
      const needsExistingFile = operation === 'swap' || operation === 'detach' || operation === 'delete'
      vi.mocked(tutorApi.getChapterDetail)
        .mockResolvedValueOnce(chapterOf({
          files: needsExistingFile
            ? [chapterFileOf({ source: operation === 'delete' ? 'chapter_file' : 'tutor_file' })]
            : []
        }))
        .mockResolvedValueOnce(chapterOf({
          content: '服务端并发修改',
          files: [chapterFileOf({ file_id: 13, file_name: '新视频.mp4', file_url: '/new.mp4' })]
        }))
      const wrapper = mountPage()
      await flushPromises()
      await editDraft(wrapper, '本地未保存草稿')

      await runFileOperation(wrapper, operation)

      expect(tutorApi.getChapterDetail).toHaveBeenCalledTimes(2)
      expect((wrapper.find('.md-editor-stub').element as HTMLTextAreaElement).value).toBe('本地未保存草稿')
      expect(button(wrapper, '保存正文').attributes('disabled')).toBeUndefined()
      if (operation === 'attach') expect(tutorFileApi.attachFile).toHaveBeenCalledWith(12, 2)
      if (operation === 'swap') expect(tutorFileApi.swapChapterFile).toHaveBeenCalledWith(2, 11, 12)
      if (operation === 'detach') expect(tutorFileApi.detachFile).toHaveBeenCalledWith(11)
      if (operation === 'delete') expect(tutorApi.deleteFile).toHaveBeenCalledWith(11)
    }
  )

  it('没有本地草稿时文件刷新采纳服务端新正文，但已保存基线不被刷新推进', async () => {
    vi.mocked(tutorApi.getChapterDetail)
      .mockResolvedValueOnce(chapterOf())
      .mockResolvedValueOnce(chapterOf({ content: '服务端新正文' }))
    const wrapper = mountPage()
    await flushPromises()

    await runFileOperation(wrapper, 'upload')

    expect((wrapper.find('.md-editor-stub').element as HTMLTextAreaElement).value).toBe('服务端新正文')
    expect(button(wrapper, '保存正文').attributes('disabled')).toBeUndefined()
    await button(wrapper, '提交审核').trigger('click')
    await flushPromises()
    expect(ElMessage.warning).toHaveBeenCalledWith('请先保存正文后再提交审核')
    expect(tutorApi.submitChapterReview).not.toHaveBeenCalled()
  })

  it('认领状态仍为 unknown 时所有可写入口禁用，且不显示认领提示条', async () => {
    vi.mocked(tutorApi.getCourseClaimants).mockReturnValue(new Promise(() => {}))
    const wrapper = mountPage()
    await flushPromises()

    expect(button(wrapper, '提交审核').attributes('disabled')).toBeDefined()
    expect(button(wrapper, '编辑信息').attributes('disabled')).toBeDefined()
    expect(button(wrapper, '保存正文').attributes('disabled')).toBeDefined()
    expect(button(wrapper, '上传视频').attributes('disabled')).toBeDefined()
    expect(button(wrapper, '从文件库挂接').attributes('disabled')).toBeDefined()
    expect(wrapper.text()).not.toContain('认领本课程后才能编辑章节')
  })

  it('unclaimed 显示认领入口并禁用写操作，认领成功后就地开放写入口', async () => {
    vi.mocked(tutorApi.getCourseClaimants).mockResolvedValue({ claimed_by_me: false, claimants: [] })
    const wrapper = mountPage()
    await flushPromises()

    expect(wrapper.text()).toContain('认领本课程后才能编辑章节')
    expect(button(wrapper, '提交审核').attributes('disabled')).toBeDefined()
    expect(button(wrapper, '编辑信息').attributes('disabled')).toBeDefined()

    await button(wrapper, '认领课程').trigger('click')
    await flushPromises()

    expect(tutorApi.claimCourse).toHaveBeenCalledWith(1)
    expect(wrapper.text()).not.toContain('认领本课程后才能编辑章节')
    expect(button(wrapper, '提交审核').attributes('disabled')).toBeUndefined()
    expect(button(wrapper, '编辑信息').attributes('disabled')).toBeUndefined()
    expect(button(wrapper, '上传视频').attributes('disabled')).toBeUndefined()
  })

  it('发布端预览读取当前编辑器快照，而不是已保存正文', async () => {
    const wrapper = mountPage()
    await flushPromises()
    await editDraft(wrapper, '当前草稿预览')

    await button(wrapper, '发布端预览').trigger('click')
    await flushPromises()

    expect(wrapper.find('.el-dialog').text()).toContain('当前草稿预览')
    expect(wrapper.find('.el-dialog').text()).not.toContain('已保存正文')
    expect(button(wrapper, '保存正文').attributes('disabled')).toBeUndefined()
  })
})
