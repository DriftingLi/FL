// #889 / #890 页面级契约：IP 属地展示与发布前披露（ADR-0045）。
//
// seam 同批一的 ForumReplyRedesign.spec：mount 页面 + 只 mock 网络层。
// 属地是**发布那一刻的快照**，所以这里断言的是「有就渲染、没有就整段不渲染」，
// 而不是「某个 IP 该显示哪个省」——后者属于库的测试口径（见后端 geolocation 包）。
import { describe, it, expect, vi, beforeEach } from 'vitest'
import { mount, flushPromises } from '@vue/test-utils'
import { epLite } from '@/test/element-lite'

// markstream 的 CSS 导入在 vitest 下无意义，替身掉
vi.mock('markstream-vue/index.css', () => ({}))

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

vi.mock('@/stores/auth', () => ({
  useAuthStore: () => ({ userInfo: { user_id: 1, username: '我' } })
}))

vi.mock('@/api/favorite', () => ({
  favoriteApi: { check: vi.fn().mockResolvedValue({ favorited: false }), add: vi.fn(), remove: vi.fn() }
}))

vi.mock('@/composables/useConfirm', () => ({
  useConfirm: () => ({
    confirm: vi.fn().mockResolvedValue('confirm'),
    confirmDanger: vi.fn().mockResolvedValue('confirm'),
    prompt: vi.fn().mockResolvedValue({ value: '' })
  })
}))

import { forumApi, type ForumReplyItem } from '@/api/forum'
import { FORUM_REGION_NOTICE } from '@/utils/forumDisplay'
import ForumPostForm from '@/components/student/ForumPostForm.vue'
import ForumDetail from '../ForumDetail.vue'

const getTopic = vi.mocked(forumApi.getTopic)

/** 回复桩数据：属地按需给（空 = 历史回复 / 内网来源）。 */
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

/** 帖子桩数据；属地按需给。 */
function topic(over: Record<string, unknown> = {}) {
  return {
    id: 1,
    category: 'discussion' as const,
    title: '讨论',
    content: '内容',
    view_count: 0,
    reply_count: 1,
    created_at: '2026-08-01T10:00:00+08:00',
    author: { user_id: 1, username: '楼主', avatar_url: '' },
    can_delete: true,
    likes_count: 0,
    liked_by_me: false,
    accepted_reply_id: null,
    solved_at: null,
    ...over
  }
}

/** 详情页挂载：**不 stub ForumComposer** —— 披露提示就在那个共享组件里。 */
async function mountDetail(tp: Record<string, unknown>, replies: ForumReplyItem[]) {
  getTopic.mockResolvedValue({
    topic: tp,
    replies,
    page: 1,
    pages: 1,
    total: replies.length
  } as never)
  const wrapper = mount(ForumDetail, {
    global: {
      plugins: [epLite()],
      stubs: { ForumImageGallery: true, UiSkeleton: true, UiMoreMenu: true, UiErrorState: true }
    }
  })
  await flushPromises()
  await new Promise((r) => setTimeout(r, 0))
  await flushPromises()
  return wrapper
}

describe('属地展示（#889 / #890，ADR-0045）', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    getTopic.mockReset()
  })

  it('有属地：帖子作者行与回复署名行都出现「时间 · 属地」', async () => {
    const wrapper = await mountDetail(
      topic({ ip_province: '江苏省', ip_city: '南京市' }),
      [reply(1, { ip_province: '广东省', ip_city: '深圳市' })]
    )

    // 帖子作者行：时间与属地同一行
    const topicTime = wrapper.find('.topic-time')
    expect(topicTime.text()).toContain('·')
    expect(topicTime.text()).toContain('南京市')

    // 回复署名行：属地接在相对时间之后
    const region = wrapper.find('.reply-region')
    expect(region.exists()).toBe(true)
    expect(region.text()).toBe('· 深圳市')
  })

  it('无属地：作者行与署名行都不出现那一段（连分隔符也没有）', async () => {
    const wrapper = await mountDetail(topic(), [reply(1)])

    expect(wrapper.find('.reply-region').exists()).toBe(false)
    // 作者行只有时间，没有「·」分隔符
    expect(wrapper.find('.topic-time').text()).not.toContain('·')
    // 也不该出现任何「未知」类占位
    expect(wrapper.text()).not.toContain('未知')
  })

  it('市为空时退到省（只显示一级，避免窄屏挤压）', async () => {
    const wrapper = await mountDetail(
      topic({ ip_province: '江苏省', ip_city: '' }),
      [reply(1, { ip_province: '广东省', ip_city: '' })]
    )
    expect(wrapper.find('.topic-time').text()).toContain('江苏省')
    expect(wrapper.find('.reply-region').text()).toBe('· 广东省')
  })

  it('回复输入区有一行发布前披露', async () => {
    const wrapper = await mountDetail(topic(), [reply(1)])
    const notice = wrapper.find('.forum-region-notice')
    expect(notice.exists()).toBe(true)
    expect(notice.text()).toBe(FORUM_REGION_NOTICE)
  })
})

describe('发帖表单的发布前披露（#890）', () => {
  beforeEach(() => vi.clearAllMocks())

  it('披露提示在场且是功能性提示而非装饰', () => {
    const wrapper = mount(ForumPostForm, {
      props: { category: 'discussion' },
      global: { plugins: [epLite()] }
    })
    const notice = wrapper.find('.forum-region-notice')
    expect(notice.exists()).toBe(true)
    expect(notice.text()).toBe(FORUM_REGION_NOTICE)
  })
})
