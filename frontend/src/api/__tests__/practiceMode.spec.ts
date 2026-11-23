import { beforeEach, describe, expect, it, vi } from 'vitest'
import { unwrappedRequest } from '@/api/request'
import { practiceModeApi } from '../practiceMode'

vi.mock('@/api/request', () => ({
  unwrappedRequest: { get: vi.fn(), post: vi.fn() }
}))

beforeEach(() => { vi.clearAllMocks() })

describe('practiceModeApi', () => {
  it('开始练习时发送本次分类，提交时只使用服务端会话标识', async () => {
    const input = { category_id: 7, mode: 'tag:2' }
    await practiceModeApi.startCategoryPractice(input)
    expect(unwrappedRequest.post).toHaveBeenCalledWith('/practice-mode/attempts', input, { headers: { 'X-Silent': 'true' } })
    const answer = { attempt_key: 'attempt', question_id: 10, user_answer: ['A', 'B'] }
    await practiceModeApi.submitCategoryAnswer(answer)
    expect(unwrappedRequest.post).toHaveBeenLastCalledWith('/practice-mode/attempts/answer', answer)
  })

  it('按卷进度继续用试卷号定位', async () => {
    await practiceModeApi.saveProgress(1, 'paper:7', 5, {})
    expect(unwrappedRequest.post).toHaveBeenCalledWith('/practice-mode/progress', {
      index: 1, practice_mode: 'paper:7', total: 5, answers_state: {}
    })
    await practiceModeApi.getProgress('paper:7')
    expect(unwrappedRequest.get).toHaveBeenCalledWith('/practice-mode/progress', { params: { mode: 'paper:7', category_id: 'all' } })
  })

  it('练习历史默认全部，也支持按历史未分类查找', async () => {
    await practiceModeApi.getHistory({ page: 2 })
    expect(unwrappedRequest.get).toHaveBeenCalledWith('/practice-mode/history', { params: { page: 2, category_id: 'all' } })
    await practiceModeApi.getHistory({ category_id: 'unclassified' })
    expect(unwrappedRequest.get).toHaveBeenLastCalledWith('/practice-mode/history', { params: { category_id: 'unclassified' } })
  })
})
