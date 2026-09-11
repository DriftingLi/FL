// #854 / #855 / #857 页面级契约：详情页回复分页（加载更多）+ 回复卡动作分层（ADR-0042）。
//
// seam 选在**页面组件层**（mount 页面 + 只 mock 网络层），理由同 ForumPage.spec：
// 给 api 层写「参数有透传」的断言不可能失败，是无效测试。真正会被写坏的是
// 「加载更多是追加还是替换」「自己的回复有没有举报入口」这类用户可观察行为。
//
// ⋯ 菜单项不在页面层断言 DOM（EP 下拉是 teleport 的 popper），而是断言 UiMoreMenu 的 items prop：
// 菜单自身的渲染与 select 事件由 ui-components.spec 的冒烟用例守住，
// 页面该负责的决策是「按权限给出哪些项」。
import { describe, it, expect, vi, beforeEach } from 'vitest'
import { mount, flushPromises } from '@vue/test-utils'
import { epLite } from '@/test/element-lite'

vi.mock('@/api/request', () => ({
  unwrappedRequest: { get: vi.fn(), post: vi.fn(), delete: vi.fn(), put: vi.fn() }
}))

vi.mock('@/api/forum', async (importOriginal) => {
  const actual = await importOriginal<typeof import('@/api/forum')>()
  return {
    ...actual,
    forumApi: {
      getTopic: vi.fn(),
      likeTopic: vi.fn(),
      unlikeTopic: vi.fn(),
      likeReply: vi.fn(),
      unlikeReply: vi.fn(),
      deleteReply: vi.fn(),
      deleteTopic: vi.fn(),
      reportReply: vi.fn(),
      reportTopic: vi.fn()
    }
  }
})

vi.mock('vue-router', () => ({
  useRouter: () => ({ push: vi.fn(), back: vi.fn() }),
  useRoute: () => ({ query: {}, params: { topicId: '1' }, hash: '' })
}))

// 当前登录用户 user_id = 1（用于「自己的回复不显示举报」的判定）
vi.mock('@/stores/auth', () => ({
  useAuthStore: () => ({ userInfo: { user_id: 1, username: '我' } })
}))

vi.mock('@/api/favorite', () => ({
  favoriteApi: { check: vi.fn().mockResolvedValue({ favorited: false }), add: vi.fn(), remove: vi.fn() }
}))

import { forumApi, type ForumReplyItem } from '@/api/forum'
import UiMoreMenu from '@/components/ui/UiMoreMenu.vue'
import UiDialog from '@/components/ui/UiDialog.vue'
import UiInput from '@/components/ui/UiInput.vue'
import ForumDetail from '../ForumDetail.vue'

const getTopic = vi.mocked(forumApi.getTopic)

function reply(id: number, over: Partial<ForumReplyItem> = {}): ForumReplyItem {
  return {
    id,
    topic_id: 1,
    parent_id: null,
    parent_name: '',
    content: `回复${id}`,
    images: [],
    created_at: '2026-08-02T10:00:00+08:00',
    author: { user_id: 2, username: `答主${id}`, avatar_url: '' },
    can_delete: false,
    likes_count: 0,
    liked_by_me: false,
    is_accepted: false,
    ...over
  }
}

function topic(replyCount: number) {
  return {
    id: 1,
    category: 'question' as const,
    title: '问答',
    content: '内容',
    view_count: 0,
    reply_count: replyCount,
    created_at: '2026-08-01T10:00:00+08:00',
    author: { user_id: 1, username: '楼主', avatar_url: '' },
    can_delete: true,
    likes_count: 0,
    liked_by_me: false,
    accepted_reply_id: null,
    solved_at: null
  }
}

async function mountDetail() {
  const wrapper = mount(ForumDetail, {
    global: {
      plugins: [epLite()],
      // UiMoreMenu 是 el-dropdown（teleport popper），20 张卡一起挂载在 CI 双核上会超时；
      // stub 后 props 断言照常可用，菜单自身的渲染/事件由 ui-components.spec 守住。
      stubs: { ForumImageGallery: true, UiSkeleton: true, ForumComposer: true, UiMoreMenu: true }
    }
  })
  await flushPromises()
  await new Promise((r) => setTimeout(r, 0))
  await flushPromises()
  return wrapper
}

type MenuItem = { key: string; label: string }

/** 第 index 条回复卡的 ⋯ 菜单项（按回复卡定位，避免第一个命中帖子卡的菜单） */
function moreItemsOfReply(wrapper: ReturnType<typeof mount>, index = 0): MenuItem[] {
  const item = wrapper.findAll('.reply-item')[index]
  return (item.findComponent(UiMoreMenu).props('items') ?? []) as MenuItem[]
}

/** 帖子卡的 ⋯ 菜单项（同页两处卡片共用一套信息架构） */
function moreItemsOfTopic(wrapper: ReturnType<typeof mount>): MenuItem[] {
  return (wrapper.find('.topic-card').findComponent(UiMoreMenu).props('items') ?? []) as MenuItem[]
}

describe('详情页回复分页（#854）', () => {
  beforeEach(() => vi.clearAllMocks())

  it('加载更多是追加而非替换，且到底显示结束态', async () => {
    const page1 = Array.from({ length: 20 }, (_, i) => reply(i + 1))
    const page2 = Array.from({ length: 5 }, (_, i) => reply(i + 21))
    getTopic
      .mockResolvedValueOnce({ topic: topic(25), replies: page1, page: 1, pages: 2, total: 25 } as never)
      .mockResolvedValueOnce({ topic: topic(25), replies: page2, page: 2, pages: 2, total: 25 } as never)

    const wrapper = await mountDetail()
    expect(wrapper.findAll('.reply-item').length).toBe(20)

    const more = wrapper.findAll('button').find((b) => b.text().includes('加载更多'))
    expect(more).toBeTruthy()
    expect(more!.text()).toContain('剩余 5 条')
    await more!.trigger('click')
    await flushPromises()

    // 追加：25 条都在，首页的第 1 条没被替换掉
    expect(wrapper.findAll('.reply-item').length).toBe(25)
    expect(wrapper.text()).toContain('回复1')
    expect(wrapper.text()).toContain('回复25')
    // 到底：入口消失、显示结束态
    expect(wrapper.findAll('button').find((b) => b.text().includes('加载更多'))).toBeUndefined()
    expect(wrapper.text()).toContain('没有更多回复了')
    // 第二页请求带的是第 2 页
    expect(getTopic).toHaveBeenLastCalledWith(1, 'latest', 'asc', 2, 20)
  })

  it('单页即到底：不渲染加载更多入口', async () => {
    getTopic.mockResolvedValue({
      topic: topic(3), replies: [reply(1), reply(2), reply(3)], page: 1, pages: 1, total: 3
    } as never)
    const wrapper = await mountDetail()
    expect(wrapper.findAll('button').find((b) => b.text().includes('加载更多'))).toBeUndefined()
    expect(wrapper.text()).toContain('没有更多回复了')
  })

  it('切排序维度回到第 1 页重新加载', async () => {
    getTopic.mockResolvedValue({
      topic: topic(1), replies: [reply(1)], page: 1, pages: 1, total: 1
    } as never)
    const wrapper = await mountDetail()
    getTopic.mockClear()
    // 切到「热门」（分段控件的 update:modelValue）
    const hotTab = wrapper
      .findAllComponents({ name: 'UiSegmentTabs' })
      .find((c) => (c.props('options') as Array<{ value: string }>).some((o) => o.value === 'hot'))
    hotTab!.vm.$emit('update:modelValue', 'hot')
    await flushPromises()
    expect(getTopic).toHaveBeenLastCalledWith(1, 'hot', 'desc', 1, 20)
  })

  it('空回复显示空态，不显示结束态', async () => {
    getTopic.mockResolvedValue({ topic: topic(0), replies: [], page: 1, pages: 0, total: 0 } as never)
    const wrapper = await mountDetail()
    expect(wrapper.findAll('.reply-item').length).toBe(0)
    expect(wrapper.text()).not.toContain('没有更多回复了')
  })
})

describe('回复卡动作分层（#857）', () => {
  beforeEach(() => vi.clearAllMocks())

  async function mountWithReplies(replies: ForumReplyItem[]) {
    getTopic.mockResolvedValue({
      topic: topic(replies.length), replies, page: 1, pages: 1, total: replies.length
    } as never)
    return mountDetail()
  }

  it('互动动作下沉到底部操作行，治理动作不在卡片正文里', async () => {
    const wrapper = await mountWithReplies([reply(1, { can_delete: false })])
    const item = wrapper.find('.reply-item')
    expect(item.find('.reply-actions').exists()).toBe(true)
    expect(item.find('.reply-actions').text()).toContain('回复')
    expect(item.find('.reply-btn').exists()).toBe(true)
    expect(item.text()).not.toContain('举报')
    expect(item.text()).not.toContain('删除')
  })

  it('自己的回复不出现「举报」，只出现「删除」', async () => {
    const wrapper = await mountWithReplies([
      reply(1, { author: { user_id: 1, username: '我', avatar_url: '' }, can_delete: true })
    ])
    const keys = moreItemsOfReply(wrapper, 0).map((i) => i.key)
    expect(keys).not.toContain('report')
    expect(keys).toContain('delete')
  })

  it('他人的回复只出现「举报」，不出现「删除」', async () => {
    const wrapper = await mountWithReplies([reply(1, { can_delete: false })])
    const keys = moreItemsOfReply(wrapper, 0).map((i) => i.key)
    expect(keys).toContain('report')
    expect(keys).not.toContain('delete')
  })

  it('楼主自己的回复被采纳时，「✓ 已采纳」与「楼主」同时出现', async () => {
    const wrapper = await mountWithReplies([
      reply(1, { author: { user_id: 1, username: '楼主', avatar_url: '' }, is_accepted: true })
    ])
    const item = wrapper.find('.reply-item')
    expect(item.text()).toContain('楼主')
    expect(item.text()).toContain('已采纳')
  })

  it('帖子卡同页统一：举报/删除进 ⋯，点赞/收藏留在帖子操作行', async () => {
    const wrapper = await mountWithReplies([reply(1)])
    const keys = moreItemsOfTopic(wrapper).map((i) => i.key)
    expect(keys).toContain('report')
    expect(keys).toContain('delete') // topic.can_delete = true
    const topicCard = wrapper.find('.topic-card')
    expect(topicCard.find('.topic-actions').exists()).toBe(true)
    expect(topicCard.find('.topic-actions').text()).toContain('点赞')
    expect(topicCard.find('.topic-actions').text()).toContain('收藏')
    // 治理动作不在帖子卡正文里以 chip 形式出现
    expect(topicCard.find('.topic-actions').text()).not.toContain('举报')
  })

  it('采纳入口独立于互动行，只对楼主、非本人作答的回答出现', async () => {
    const wrapper = await mountWithReplies([reply(1)])
    const item = wrapper.find('.reply-item')
    expect(item.find('.reply-accept-row').exists()).toBe(true)
    expect(item.find('.reply-accept-row').text()).toContain('采纳此回答')
    expect(item.find('.reply-actions').text()).not.toContain('采纳')
  })
})

describe('举报入口（收编为 useForumReport 一处）', () => {
  beforeEach(() => vi.clearAllMocks())

  it('从 ⋯ 选「举报」打开对话框，确认后按主题/回复分别提交', async () => {
    getTopic.mockResolvedValue({
      topic: topic(1), replies: [reply(1)], page: 1, pages: 1, total: 1
    } as never)
    const wrapper = await mountDetail()

    // ⋯ 选「举报」→ 对话框打开
    const replyMenu = wrapper.findAll('.reply-item')[0].findComponent(UiMoreMenu)
    replyMenu.vm.$emit('select', 'report')
    await flushPromises()
    const dialog = wrapper.findComponent(UiDialog)
    expect(dialog.props('modelValue')).toBe(true)

    // 理由为空时提交被拦下（校验在 composable 一处，两端同口径）
    dialog.vm.$emit('confirm')
    await flushPromises()
    expect(forumApi.reportReply).not.toHaveBeenCalled()

    // 填理由后提交 → 打到回复举报端点
    wrapper.findComponent(UiInput).vm.$emit('update:modelValue', '违规内容')
    await flushPromises()
    dialog.vm.$emit('confirm')
    await flushPromises()
    expect(forumApi.reportReply).toHaveBeenCalledWith(1, '违规内容')
  })

  it('帖子卡的 ⋯ 选「举报」打的是主题举报端点', async () => {
    getTopic.mockResolvedValue({
      topic: topic(1), replies: [reply(1)], page: 1, pages: 1, total: 1
    } as never)
    const wrapper = await mountDetail()
    wrapper.find('.topic-card').findComponent(UiMoreMenu).vm.$emit('select', 'report')
    await flushPromises()
    wrapper.findComponent(UiInput).vm.$emit('update:modelValue', '主题违规')
    await flushPromises()
    wrapper.findComponent(UiDialog).vm.$emit('confirm')
    await flushPromises()
    expect(forumApi.reportTopic).toHaveBeenCalledWith(1, '主题违规')
  })
})

describe('被回复人小头像（#855）', () => {
  beforeEach(() => vi.clearAllMocks())

  it('楼中楼渲染「昵称 › 被回复人」且带头像', async () => {
    getTopic.mockResolvedValue({
      topic: topic(1),
      replies: [
        reply(1, {
          parent_id: 9,
          parent_name: '被回复的人',
          parent_avatar_url: '/static/uploads/avatar/p.png'
        })
      ],
      page: 1,
      pages: 1,
      total: 1
    } as never)
    const wrapper = await mountDetail()
    const parent = wrapper.find('.reply-parent')
    expect(parent.exists()).toBe(true)
    expect(parent.text()).toContain('被回复的人')
    expect(parent.find('.el-avatar').exists()).toBe(true)
    // 旧形态（独立引用块「回复 @某某」）已退役
    expect(wrapper.text()).not.toContain('回复 @')
  })

  it('顶层回复不渲染被回复人片段', async () => {
    getTopic.mockResolvedValue({
      topic: topic(1), replies: [reply(1)], page: 1, pages: 1, total: 1
    } as never)
    const wrapper = await mountDetail()
    expect(wrapper.find('.reply-parent').exists()).toBe(false)
  })
})
