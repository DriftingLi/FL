// #2920 切片 2：章节审核快照与保存竞态会话。
import { describe, expect, it, vi } from 'vitest'
import { flushPromises } from '@vue/test-utils'
import { nextTick } from 'vue'
import { ElMessage } from 'element-plus'
import type { TutorChapterDetail } from '@/api/tutor'
import {
  button,
  chapterFileOf,
  chapterOf,
  confirm,
  editDraft,
  getTutorApi,
  mountPage,
  routeState
} from './TutorChapterEdit.test-utils'

const tutorApi = getTutorApi()

describe('TutorChapterEdit 审核快照与保存竞态（#2920）', () => {
  it('保存后已保存基线更新、保存按钮复位，随后可确认并调用提审 API', async () => {
    const wrapper = mountPage()
    await flushPromises()
    await editDraft(wrapper, '已保存的新正文')

    await button(wrapper, '保存正文').trigger('click')
    await flushPromises()

    expect(tutorApi.updateChapter).toHaveBeenCalledWith(2, { content: '已保存的新正文' })
    expect(button(wrapper, '保存正文').attributes('disabled')).toBeDefined()

    await button(wrapper, '提交审核').trigger('click')
    await flushPromises()

    expect(confirm).toHaveBeenCalledTimes(1)
    expect(confirm).toHaveBeenCalledWith(
      '提交后将冻结已保存正文与文件清单进入审核（后续改动不影响在审版本），确认提交？',
      '提交审核'
    )
    expect(tutorApi.submitChapterReview).toHaveBeenCalledWith(2)
    expect(wrapper.text()).toContain('审核中')
  })

  it('保存请求期间继续输入时，只把提交时的正文快照标为已保存', async () => {
    let finishSave!: (value: TutorChapterDetail) => void
    vi.mocked(tutorApi.updateChapter).mockImplementationOnce(() => new Promise(resolve => {
      finishSave = resolve
    }))
    const wrapper = mountPage()
    await flushPromises()
    await editDraft(wrapper, '提交保存的正文')

    await button(wrapper, '保存正文').trigger('click')
    await flushPromises()
    expect(tutorApi.updateChapter).toHaveBeenCalledWith(2, { content: '提交保存的正文' })
    await editDraft(wrapper, '保存期间的新草稿')

    finishSave(chapterOf({ content: '提交保存的正文' }))
    await flushPromises()

    expect((wrapper.find('.md-editor-stub').element as HTMLTextAreaElement).value).toBe('保存期间的新草稿')
    expect((wrapper.vm as unknown as { chapterDetail: TutorChapterDetail }).chapterDetail.content).toBe('提交保存的正文')
    expect(button(wrapper, '保存正文').attributes('disabled')).toBeUndefined()
    await button(wrapper, '提交审核').trigger('click')
    await flushPromises()
    expect(ElMessage.warning).toHaveBeenCalledWith('请先保存正文后再提交审核')
    expect(tutorApi.submitChapterReview).not.toHaveBeenCalled()
  })

  it('提审 API 成功后重新拉取详情，只展示服务端返回的在审快照', async () => {
    vi.mocked(tutorApi.getChapterDetail)
      .mockResolvedValueOnce(chapterOf({ files: [chapterFileOf({ file_name: '本地文件.mp4' })] }))
      .mockResolvedValueOnce(chapterOf({
        content: '服务端工作副本',
        review_status: 'pending',
        reject_reason: '',
        review_snapshot: {
          chapter_id: 2,
          chapter_title: '扭矩计算',
          course_id: 1,
          course_name: '叉车维修',
          content: '服务端冻结正文',
          content_type: 'markdown',
          file_url: '',
          submitted_by: 9,
          frozen_at: '2026-11-01T10:00:00+08:00',
          files: [chapterFileOf({ file_name: '服务端冻结文件.pdf' })]
        }
      }))
    const wrapper = mountPage()
    await flushPromises()
    await editDraft(wrapper, '本地提交正文')
    await button(wrapper, '保存正文').trigger('click')
    await flushPromises()

    await button(wrapper, '提交审核').trigger('click')
    await flushPromises()

    expect(tutorApi.submitChapterReview).toHaveBeenCalledWith(2)
    expect(tutorApi.getChapterDetail).toHaveBeenCalledTimes(2)
    expect(tutorApi.getChapterDetail).toHaveBeenLastCalledWith(2)
    expect(wrapper.text()).toContain('审核中')
    expect(wrapper.get('.review-snapshot').text()).toContain('服务端冻结文件.pdf')
    expect(wrapper.get('.review-snapshot').text()).not.toContain('本地文件.mp4')
    await button(wrapper, '预览在审快照').trigger('click')
    await flushPromises()
    expect(wrapper.get('.el-dialog').text()).toContain('服务端冻结正文')
    expect(wrapper.get('.el-dialog').text()).not.toContain('本地提交正文')
  })

  it('提审成功后快照刷新失败时保持 pending、清空快照并提示刷新失败', async () => {
    const refreshError = new Error('detail refresh failed')
    const consoleError = vi.spyOn(console, 'error').mockImplementation(() => undefined)
    vi.mocked(tutorApi.getChapterDetail)
      .mockResolvedValueOnce(chapterOf())
      .mockRejectedValueOnce(refreshError)
    const wrapper = mountPage()
    await flushPromises()

    await button(wrapper, '提交审核').trigger('click')
    await flushPromises()

    expect(tutorApi.submitChapterReview).toHaveBeenCalledWith(2)
    expect(tutorApi.getChapterDetail).toHaveBeenCalledTimes(2)
    expect((wrapper.vm as unknown as { chapterDetail: TutorChapterDetail }).chapterDetail.review_status).toBe('pending')
    expect((wrapper.vm as unknown as { reviewSnapshot: unknown }).reviewSnapshot).toBeNull()
    expect(wrapper.find('.review-snapshot').exists()).toBe(false)
    expect(wrapper.text()).toContain('在审快照不可用')
    expect(ElMessage.warning).toHaveBeenCalledWith('已提交审核，但在审快照刷新失败，请刷新页面重试')
    expect(consoleError).toHaveBeenCalledWith(
      '[chapter editing session] 提审成功但在审快照刷新失败。',
      refreshError
    )
  })

  it('提审期间继续输入的草稿不会被详情刷新覆盖', async () => {
    let finishRefresh!: (value: TutorChapterDetail) => void
    vi.mocked(tutorApi.getChapterDetail)
      .mockResolvedValueOnce(chapterOf())
      .mockImplementationOnce(() => new Promise(resolve => {
        finishRefresh = resolve
      }))
    const wrapper = mountPage()
    await flushPromises()
    await editDraft(wrapper, '本地提交正文')
    await button(wrapper, '保存正文').trigger('click')
    await flushPromises()

    await button(wrapper, '提交审核').trigger('click')
    await flushPromises()
    expect(tutorApi.getChapterDetail).toHaveBeenCalledTimes(2)
    await editDraft(wrapper, '提审期间的新草稿')

    finishRefresh(chapterOf({
      content: '服务端并发工作副本',
      review_status: 'pending',
      review_snapshot: {
        chapter_id: 2,
        chapter_title: '扭矩计算',
        course_id: 1,
        course_name: '叉车维修',
        content: '服务端冻结正文',
        content_type: 'markdown',
        file_url: '',
        submitted_by: 9,
        frozen_at: '2026-11-01T10:00:00+08:00',
        files: []
      }
    }))
    await flushPromises()

    expect((wrapper.find('.md-editor-stub').element as HTMLTextAreaElement).value).toBe('提审期间的新草稿')
    expect((wrapper.vm as unknown as { chapterDetail: TutorChapterDetail }).chapterDetail.content).toBe('本地提交正文')
    expect(button(wrapper, '保存正文').attributes('disabled')).toBeUndefined()
    expect(wrapper.get('.review-snapshot').text()).toContain('在审快照')
    await button(wrapper, '预览在审快照').trigger('click')
    await flushPromises()
    expect(wrapper.get('.el-dialog').text()).toContain('服务端冻结正文')
    expect(wrapper.get('.el-dialog').text()).not.toContain('提审期间的新草稿')
  })

  it('在审章节分别展示真实在审快照与工作副本，不用工作正文冒充快照', async () => {
    const pending: TutorChapterDetail = {
      ...chapterOf({ review_status: 'pending', content: '审中的工作副本' }),
      review_snapshot: {
        chapter_id: 2,
        chapter_title: '扭矩计算',
        course_id: 1,
        course_name: '叉车维修',
        content: '提交时冻结的正文',
        content_type: 'markdown',
        file_url: '',
        submitted_by: 9,
        frozen_at: '2026-11-01T10:00:00+08:00',
        files: [chapterFileOf({ file_name: '送审时冻结的文档.pdf' })]
      }
    }
    vi.mocked(tutorApi.getChapterDetail).mockResolvedValue(pending)
    const wrapper = mountPage()
    await flushPromises()

    expect((wrapper.find('.md-editor-stub').element as HTMLTextAreaElement).value).toBe('审中的工作副本')
    expect(wrapper.text()).toContain('工作副本')
    const snapshot = wrapper.get('.review-snapshot')
    expect(snapshot.text()).toContain('在审快照')
    expect(snapshot.text()).toContain('冻结文件清单（1）')
    expect(snapshot.text()).toContain('送审时冻结的文档.pdf')
    expect(snapshot.text()).not.toContain('审中的工作副本')

    await button(wrapper, '预览在审快照').trigger('click')
    await flushPromises()

    const preview = wrapper.get('.el-dialog')
    expect(preview.text()).toContain('在审快照预览')
    expect(preview.text()).toContain('提交时冻结的正文')
    expect(preview.text()).not.toContain('审中的工作副本')
  })

  it('在审状态缺少真实快照时明确提示不可用，不把工作副本伪造成快照', async () => {
    vi.mocked(tutorApi.getChapterDetail).mockResolvedValue(chapterOf({
      review_status: 'pending',
      content: '仅有工作副本'
    }))
    const wrapper = mountPage()
    await flushPromises()

    expect((wrapper.find('.md-editor-stub').element as HTMLTextAreaElement).value).toBe('仅有工作副本')
    expect((wrapper.vm as unknown as { reviewSnapshot: unknown }).reviewSnapshot).toBeNull()
    expect(wrapper.find('.review-snapshot').exists()).toBe(false)
    expect(wrapper.text()).toContain('在审快照不可用')
  })

  it('提审后继续编辑只改变工作副本，在审状态与重复提审禁用态保持独立', async () => {
    const wrapper = mountPage()
    await flushPromises()
    await editDraft(wrapper, '提交审核的正文')
    await button(wrapper, '保存正文').trigger('click')
    await flushPromises()
    await button(wrapper, '提交审核').trigger('click')
    await flushPromises()

    await editDraft(wrapper, '在审后的工作副本')

    expect(wrapper.text()).toContain('审核中')
    expect(button(wrapper, '提交审核').attributes('disabled')).toBeDefined()
    expect(button(wrapper, '保存正文').attributes('disabled')).toBeUndefined()
    expect(tutorApi.submitChapterReview).toHaveBeenCalledTimes(1)
  })

  it('切章建立全新会话，上一章未保存草稿不会带入下一章', async () => {
    vi.mocked(tutorApi.getChapterDetail).mockImplementation(async chapterId => chapterOf({
      chapter_id: chapterId,
      content: chapterId === 2 ? '第二章已保存正文' : '第三章已保存正文'
    }))
    const wrapper = mountPage()
    await flushPromises()
    await editDraft(wrapper, '第二章未保存草稿')

    routeState.route!.params.chapterId = '3'
    await nextTick()
    await flushPromises()

    expect(tutorApi.getChapterDetail).toHaveBeenLastCalledWith(3)
    expect((wrapper.find('.md-editor-stub').element as HTMLTextAreaElement).value).toBe('第三章已保存正文')
    expect(button(wrapper, '保存正文').attributes('disabled')).toBeDefined()
  })

  it('页面卸载后重新挂载不会恢复旧草稿', async () => {
    const wrapper = mountPage()
    await flushPromises()
    await editDraft(wrapper, '只存在于当前页面的草稿')
    wrapper.unmount()

    const remounted = mountPage()
    await flushPromises()

    expect((remounted.find('.md-editor-stub').element as HTMLTextAreaElement).value).toBe('已保存正文')
    expect(button(remounted, '保存正文').attributes('disabled')).toBeDefined()
  })
})
