// 错题本导出（WrongQuestions.vue）契约测试：
// 点击「导出错题」→ 调用 exportWrongQuestions 拿到 Blob，并经统一 downloadBlob
// 以 wrong_questions.txt 落盘（复用唯一下载实现，F3）。
import { describe, it, expect, vi, beforeEach } from 'vitest'
import { mount, flushPromises } from '@vue/test-utils'
import ElementPlus from 'element-plus'

vi.mock('@/api/wrongQuestion', () => ({
  wrongQuestionApi: {
    getWrongQuestions: vi.fn(),
    exportWrongQuestions: vi.fn()
  }
}))

vi.mock('@/composables/useReportDownload', () => ({
  downloadBlob: vi.fn()
}))

import { wrongQuestionApi } from '@/api/wrongQuestion'
import { downloadBlob } from '@/composables/useReportDownload'
import WrongQuestions from '../WrongQuestions.vue'

beforeEach(() => {
  vi.mocked(wrongQuestionApi.getWrongQuestions).mockResolvedValue({
    items: [{ id: 1, question_id: 101, wrong_count: 2, question: { type: 'single_choice', content: '测试题目', options: { A: '选项A' } } }],
    total: 1
  })
  vi.mocked(wrongQuestionApi.exportWrongQuestions).mockResolvedValue(
    new Blob(['题目一\n题目二\n'], { type: 'text/plain; charset=utf-8' })
  )
  vi.mocked(downloadBlob).mockClear()
})

function mountPage() {
  return mount(WrongQuestions, { global: { plugins: [ElementPlus] } })
}

describe('WrongQuestions 错题导出', () => {
  it('点击导出错题：以 exportWrongQuestions 的 Blob 调用 downloadBlob，文件名为 wrong_questions.txt', async () => {
    const wrapper = mountPage()
    await flushPromises()

    const btn = wrapper.findAll('button').find(b => b.text().includes('导出错题'))
    expect(btn).toBeTruthy()
    await btn!.trigger('click')
    await flushPromises()

    expect(wrongQuestionApi.exportWrongQuestions).toHaveBeenCalled()
    expect(downloadBlob).toHaveBeenCalledTimes(1)
    const [blob, fileName] = vi.mocked(downloadBlob).mock.calls[0]
    expect(blob).toBeInstanceOf(Blob)
    expect(fileName).toBe('wrong_questions.txt')
  })
})
