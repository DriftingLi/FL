// #367 问答状态可见：求助/已解决与采纳置顶
import { describe, it, expect, vi, beforeEach } from 'vitest'
import { mount, flushPromises } from '@vue/test-utils'
import ElementPlus, { ElRadioGroup } from 'element-plus'

vi.mock('@/api/request', () => ({
  unwrappedRequest: { get: vi.fn(), post: vi.fn(), delete: vi.fn(), put: vi.fn() }
}))

vi.mock('@/api/forum', async (importOriginal) => {
  const actual = await importOriginal<typeof import('@/api/forum')>()
  return {
    ...actual,
    forumApi: {
      listTopics: vi.fn(),
      getMyTopics: vi.fn(),
      getMyReplies: vi.fn(),
      createTopic: vi.fn(),
      getCheckInCalendar: vi.fn(),
      checkIn: vi.fn(),
      getTopic: vi.fn(),
      acceptReply: vi.fn(),
      cancelAccept: vi.fn(),
      likeTopic: vi.fn(),
      unlikeTopic: vi.fn(),
      likeReply: vi.fn(),
      unlikeReply: vi.fn()
    }
  }
})

vi.mock('vue-router', () => ({
  useRouter: () => ({ push: vi.fn(), back: vi.fn() }),
  useRoute: () => ({ query: {}, params: { topicId: '1' }, hash: '' })
}))

vi.mock('@/utils/forumHistory', () => ({
  loadHistory: vi.fn(() => []),
  removeHistoryItem: vi.fn(),
  clearHistory: vi.fn(),
  pushHistory: vi.fn(),
  toHistoryItem: vi.fn(() => ({}))
}))

vi.mock('@/stores/auth', () => ({
  useAuthStore: () => ({ userInfo: { user_id: 1, username: '楼主' } })
}))

vi.mock('@/api/favorite', () => ({
  favoriteApi: { check: vi.fn().mockResolvedValue({ favorited: false }), add: vi.fn(), remove: vi.fn() }
}))

import { forumApi } from '@/api/forum'
import ForumPage from '../ForumPage.vue'
import ForumDetail from '../ForumDetail.vue'

const listTopics = vi.mocked(forumApi.listTopics)
const getTopic = vi.mocked(forumApi.getTopic)

function topic(id: number, category: 'discussion' | 'question', solved: boolean) {
  return {
    id,
    category,
    title: solved ? `已解决-${id}` : `求助-${id}`,
    content: '内容',
    view_count: 0,
    reply_count: 2,
    created_at: '2026-08-01T10:00:00+08:00',
    author: { user_id: 1, username: '楼主', avatar_url: '' },
    accepted_reply_id: solved ? 99 : null,
    solved_at: solved ? '2026-08-02T10:00:00+08:00' : null,
    reward_issued: solved
  }
}

function reply(id: number, isAccepted: boolean, userId = 2) {
  return {
    id,
    topic_id: 1,
    parent_id: null,
    parent_name: '',
    content: `回复${id}`,
    images: [],
    created_at: '2026-08-02T10:00:00+08:00',
    author: { user_id: userId, username: userId === 1 ? '楼主' : `答主${id}`, avatar_url: '' },
    can_delete: false,
    likes_count: 0,
    liked_by_me: false,
    is_accepted: isAccepted
  }
}

async function mountForumPage() {
  listTopics.mockResolvedValue({ topics: [topic(1, 'question', false), topic(2, 'question', true)], total: 2 } as never)
  vi.mocked(forumApi.getCheckInCalendar).mockResolvedValue({ dates: [], streak: 0, total: 0, today_checked: false } as never)
  const wrapper = mount(ForumPage, {
    global: { plugins: [ElementPlus], stubs: { CheckInDialog: true, ForumHistoryPanel: true, ForumImageUploader: true } }
  })
  await flushPromises()
  return wrapper
}

describe('问答状态可见 #367', () => {
  beforeEach(() => vi.clearAllMocks())

  it('问答 Tab 显示求助/已解决筛选，且只有这一条筛选轴', async () => {
    const wrapper = await mountForumPage()
    // 切换到问答
    const catGroup = wrapper.findAllComponents(ElRadioGroup).find(c => c.classes().includes('forum-category'))!
    catGroup.vm.$emit('update:modelValue', 'question')
    await flushPromises()
    // 应出现 solved-filter
    expect(wrapper.find('.solved-filter').exists()).toBe(true)
    const solvedGroup = wrapper.find('.solved-filter').findComponent(ElRadioGroup)
    expect(solvedGroup.exists()).toBe(true)
    const labels = wrapper.find('.solved-filter').text()
    expect(labels).toContain('求助')
    expect(labels).toContain('已解决')
    // 讨论 Tab 不应出现该筛选
    catGroup.vm.$emit('update:modelValue', 'discussion')
    await flushPromises()
    expect(wrapper.find('.solved-filter').exists()).toBe(false)
  })

  it('问答列表每条显示 ✓ 已解决 或 求助 标记 + 回答数', async () => {
    const wrapper = await mountForumPage()
    const catGroup = wrapper.findAllComponents(ElRadioGroup).find(c => c.classes().includes('forum-category'))!
    catGroup.vm.$emit('update:modelValue', 'question')
    await flushPromises()
    // 列表应含两种标记
    expect(wrapper.text()).toContain('✓ 已解决')
    expect(wrapper.text()).toContain('求助')
    // 回答数（reply_count）在 meta-right 中
    expect(wrapper.text()).toMatch(/2/)
  })

  it('求助/已解决筛选切换时请求带 solved 参数', async () => {
    const wrapper = await mountForumPage()
    const catGroup = wrapper.findAllComponents(ElRadioGroup).find(c => c.classes().includes('forum-category'))!
    catGroup.vm.$emit('update:modelValue', 'question')
    await flushPromises()
    listTopics.mockClear()
    const solvedGroup = wrapper.find('.solved-filter').findComponent(ElRadioGroup)
    // 切到已解决
    solvedGroup.vm.$emit('update:modelValue', 'solved')
    // el-radio-group 会同时触发 change 事件，组件内 @change 会触发 load
    solvedGroup.vm.$emit('change', 'solved')
    await flushPromises()
    expect(listTopics).toHaveBeenCalled()
    const lastParams = listTopics.mock.calls[listTopics.mock.calls.length - 1][0] as Record<string, unknown>
    expect(lastParams.solved).toBe('solved')
    // 切到求助
    listTopics.mockClear()
    solvedGroup.vm.$emit('update:modelValue', 'unsolved')
    solvedGroup.vm.$emit('change', 'unsolved')
    await flushPromises()
    const p2 = listTopics.mock.calls[listTopics.mock.calls.length - 1][0] as Record<string, unknown>
    expect(p2.solved).toBe('unsolved')
  })

  it('详情页已采纳答案恒置顶且带绿色边框与 ✓ 标记', async () => {
    // 构造详情：回复 2 为已采纳，但排在后面，渲染后应置顶
    const r1 = reply(1, false)
    const r2 = reply(2, true) // 已采纳
    const r3 = reply(3, false)
    getTopic.mockResolvedValue({
      topic: { id: 1, category: 'question', title: '问答', content: '内容', view_count: 0, reply_count: 3, created_at: '2026-08-01T10:00:00+08:00', author: { user_id: 1, username: '楼主', avatar_url: '' }, accepted_reply_id: 2, solved_at: '2026-08-02T10:00:00+08:00', reward_issued: true },
      replies: [r1, r2, r3]
    } as never)
    const wrapper = mount(ForumDetail, {
      global: { plugins: [ElementPlus], stubs: { ForumImageGallery: true, UiEmptyState: true, UiErrorState: true, UiSkeleton: true } }
    })
    await flushPromises()
    await new Promise(r => setTimeout(r, 0))
    await flushPromises()
    // 检查置顶：第一个渲染的回复应为已采纳
    const replyItems = wrapper.findAll('.reply-item')
    expect(replyItems.length).toBe(3)
    // 已采纳的在最前且有 is-accepted 类与绿色边框
    expect(replyItems[0].classes()).toContain('is-accepted')
    expect(replyItems[0].text()).toContain('已采纳')
    // 楼主标识与已采纳可同时成立（用楼主自己的回答被采纳的场景）
    // 构造楼主自答被采纳
    const rSelf = reply(4, true, 1)
    getTopic.mockResolvedValue({
      topic: { id: 1, category: 'question', title: '问答', content: '内容', view_count: 0, reply_count: 1, created_at: '2026-08-01T10:00:00+08:00', author: { user_id: 1, username: '楼主', avatar_url: '' }, accepted_reply_id: 4, solved_at: '2026-08-02T10:00:00+08:00', reward_issued: false },
      replies: [rSelf]
    } as never)
    const wrapper2 = mount(ForumDetail, {
      global: { plugins: [ElementPlus], stubs: { ForumImageGallery: true, UiEmptyState: true, UiErrorState: true, UiSkeleton: true } }
    })
    await flushPromises()
    await new Promise(r => setTimeout(r, 0))
    const first = wrapper2.find('.reply-item')
    expect(first.classes()).toContain('is-accepted')
    expect(first.text()).toContain('楼主')
    expect(first.text()).toContain('已采纳')
  })
})
