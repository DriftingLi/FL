// QuestionDetail（题目承载页）三态契约测试（#1101）：
// 本页原来自建 `chapterNotFound` 式的 404 布尔 + 手写 retry。第十一波收编到 useAsyncPage：
// **404（题目已下架 / 不在当前证件题库内）= 空态**、其余 = 错误态（带 retry），
// 页面对 loadError 只有 boolean 认知 —— 这条判据第一次由生产路径穿过。
import { describe, it, expect, vi, beforeEach } from 'vitest'
import { mount, flushPromises } from '@vue/test-utils'
import { epLite } from '@/test/element-lite'

vi.mock('@/api/questionBank', () => ({
  questionBankApi: { getQuestion: vi.fn() }
}))
vi.mock('@/api/practiceMode', () => ({
  practiceModeApi: { submitAnswer: vi.fn() }
}))
vi.mock('@/api/favorite', () => ({
  favoriteApi: { check: vi.fn(), add: vi.fn(), remove: vi.fn() }
}))
vi.mock('@/api/questionInteraction', () => ({
  questionInteractionApi: { listKnowledge: vi.fn() }
}))
vi.mock('vue-router', () => ({
  useRoute: () => ({ params: { id: '101' } }),
  useRouter: () => ({ back: vi.fn(), push: vi.fn() })
}))

import { questionBankApi } from '@/api/questionBank'
import { favoriteApi } from '@/api/favorite'
import { questionInteractionApi } from '@/api/questionInteraction'
import QuestionDetail from '../QuestionDetail.vue'

/** 后端 404：拦截器把 ApiErrorKind 挂在错误对象上（client.ts attachKind）。 */
function notFoundError() {
  const e = new Error('请求的资源不存在') as Error & { kind?: string }
  e.kind = 'notfound'
  return e
}

/** 服务端异常：同样带 kind，但不是 404 —— 只能进错误态。 */
function serverError() {
  const e = new Error('服务器错误') as Error & { kind?: string }
  e.kind = 'server'
  return e
}

function mountPage() {
  return mount(QuestionDetail, {
    global: {
      plugins: [epLite()],
      // 会话外围件（评论/笔记）挂载即拉自己的 API，不是本用例的关注点
      stubs: { CommentCard: true, NoteCard: true, AIExplanationCard: true }
    }
  })
}

beforeEach(() => {
  vi.mocked(questionBankApi.getQuestion).mockReset()
  vi.mocked(favoriteApi.check).mockResolvedValue({ favorited: false, favorite_id: 0 } as never)
  vi.mocked(questionInteractionApi.listKnowledge).mockResolvedValue([] as never)
})

describe('QuestionDetail 三态（#1101：404 = 空态、其余 = 错误态）', () => {
  it('404：渲染空态（明确去向），不渲染错误态的「重试」', async () => {
    vi.mocked(questionBankApi.getQuestion).mockRejectedValue(notFoundError())
    const wrapper = mountPage()
    await flushPromises()

    expect(wrapper.text()).toContain('题目不存在或已下架')
    expect(wrapper.text()).toContain('去题库看看')
    // 404 不是「可重试的失败」：错误态的重试按钮不出现
    expect(wrapper.findAll('button').some(b => b.text().includes('重试'))).toBe(false)
  })

  it('服务端异常：渲染错误态 + 重试；点重试成功后空态/错误态双双让位给内容', async () => {
    vi.mocked(questionBankApi.getQuestion).mockRejectedValueOnce(serverError())
    const wrapper = mountPage()
    await flushPromises()

    const retryBtn = wrapper.findAll('button').find(b => b.text().includes('重试'))
    expect(retryBtn).toBeTruthy()
    expect(wrapper.text()).not.toContain('题目不存在或已下架')

    vi.mocked(questionBankApi.getQuestion).mockResolvedValue({
      id: 101,
      type: 'single_choice',
      content: '叉车起升机构的检查周期是？',
      options: { A: '每日', B: '每周' },
      status: 'published'
    } as never)
    await retryBtn!.trigger('click')
    await flushPromises()

    expect(wrapper.text()).toContain('叉车起升机构的检查周期是？')
    expect(wrapper.findAll('button').some(b => b.text().includes('重试'))).toBe(false)
  })
})
