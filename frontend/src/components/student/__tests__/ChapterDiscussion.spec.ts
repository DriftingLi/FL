// #858 页面级契约：章节讨论回复区与帖子详情功能对齐（点赞 / 举报 / ⋯），但保持紧凑密度。
//
// seam 同 ForumReplyRedesign.spec：mount 组件 + 只 mock 网络层。
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
      listTopics: vi.fn(),
      getTopic: vi.fn(),
      likeReply: vi.fn(),
      unlikeReply: vi.fn(),
      replyTopic: vi.fn(),
      deleteReply: vi.fn(),
      deleteTopic: vi.fn(),
      reportReply: vi.fn(),
      reportTopic: vi.fn()
    }
  }
})

vi.mock('@/stores/auth', () => ({
  useAuthStore: () => ({ userInfo: { user_id: 1, username: '我' } })
}))

// 章节讨论新增了「查看全部」跳详情（截断提示的去向）
vi.mock('vue-router', () => ({
  useRouter: () => ({ push: vi.fn() })
}))

import { forumApi, type ForumReplyItem } from '@/api/forum'
import UiMoreMenu from '@/components/ui/UiMoreMenu.vue'
import ForumReplyCard from '../ForumReplyCard.vue'
import ChapterDiscussion from '../ChapterDiscussion.vue'

const listTopics = vi.mocked(forumApi.listTopics)
const getTopic = vi.mocked(forumApi.getTopic)
const likeReply = vi.mocked(forumApi.likeReply)

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

/** 挂载并展开第一个章节帖（章节讨论是「列表 → 点击展开详情」两级加载） */
async function mountExpanded(replies: ForumReplyItem[]) {
  listTopics.mockResolvedValue({
    topics: [
      {
        id: 1,
        chapter_id: 1,
        category: 'discussion',
        title: '章节讨论帖',
        content: '内容',
        view_count: 0,
        reply_count: replies.length,
        created_at: '2026-08-01T10:00:00+08:00',
        author: { user_id: 1, username: '楼主', avatar_url: '' }
      }
    ],
    total: 1
  } as never)
  getTopic.mockResolvedValue({
    topic: {
      id: 1,
      chapter_id: 1,
      category: 'discussion',
      title: '章节讨论帖',
      content: '内容',
      view_count: 0,
      reply_count: replies.length,
      created_at: '2026-08-01T10:00:00+08:00',
      author: { user_id: 1, username: '楼主', avatar_url: '' }
    },
    replies,
    page: 1,
    pages: 1,
    total: replies.length
  } as never)

  const wrapper = mount(ChapterDiscussion, {
    props: { chapterId: 1 },
    global: {
      plugins: [epLite()],
      stubs: { ForumImageGallery: true, ForumPostForm: true, ForumComposer: true }
    }
  })
  await flushPromises()
  // 展开第一个帖子（列表项是可点击容器）
  await wrapper.findAll('.cursor-pointer')[0].trigger('click')
  await flushPromises()
  await new Promise((r) => setTimeout(r, 0))
  await flushPromises()
  return wrapper
}

function moreItemsOfReply(wrapper: ReturnType<typeof mount>, index = 0) {
  const item = wrapper.findAll('.reply-item')[index]
  return (item.findComponent(UiMoreMenu).props('items') ?? []) as Array<{ key: string; label: string }>
}

describe('章节讨论正文按声明格式渲染（#878 / ADR-0044）', () => {
  beforeEach(() => vi.clearAllMocks())

  /** 展开第一个章节帖，正文与格式可指定 */
  async function mountWithContent(content: string, format?: string) {
    const baseTopic = {
      id: 1,
      chapter_id: 1,
      category: 'discussion',
      title: '章节讨论帖',
      content,
      view_count: 0,
      reply_count: 0,
      created_at: '2026-08-01T10:00:00+08:00',
      author: { user_id: 1, username: '楼主', avatar_url: '' }
    };
    const withFormat = format === undefined ? baseTopic : { ...baseTopic, content_format: format };
    listTopics.mockResolvedValue({ topics: [withFormat], total: 1 } as never);
    getTopic.mockResolvedValue({
      topic: withFormat,
      replies: [],
      page: 1,
      pages: 1,
      total: 0
    } as never);

    const wrapper = mount(ChapterDiscussion, {
      props: { chapterId: 1 },
      global: {
        plugins: [epLite()],
        stubs: { ForumImageGallery: true, ForumPostForm: true, ForumComposer: true }
      }
    });
    await flushPromises();
    await wrapper.findAll('.cursor-pointer')[0].trigger('click')
    await flushPromises();
    await new Promise((r) => setTimeout(r, 0));
    await flushPromises();
    return wrapper;
  }

  it('markdown 帖渲染出结构，纯文本帖原样显示（与详情页同口径）', async () => {
    const md = await mountWithContent('## 排查步骤', 'markdown')
    expect(md.html()).toMatch(/<h2/i)

    const txt = await mountWithContent('## 排查步骤', 'text')
    expect(txt.html()).not.toMatch(/<h2/i)
    expect(txt.text()).toContain('## 排查步骤')
  })
})

describe('章节讨论回复区对齐（#858）', () => {
  beforeEach(() => vi.clearAllMocks())

  it('复用共享回复卡并保持紧凑密度', async () => {
    const wrapper = await mountExpanded([reply(1)])
    const cards = wrapper.findAllComponents(ForumReplyCard)
    expect(cards.length).toBe(1)
    expect(cards[0].props('density')).toBe('compact')
    // 紧凑档头像 26px（详情页是 38px）
    expect(wrapper.find('.reply-item .el-avatar').attributes('style')).toContain('26px')
  })

  it('回复可点赞（调 likeReply）', async () => {
    likeReply.mockResolvedValue({ likes_count: 1, liked: true } as never)
    const wrapper = await mountExpanded([reply(1)])
    const likeBtn = wrapper.find('.reply-item .reply-actions button:last-child')
    await likeBtn.trigger('click')
    await flushPromises()
    expect(likeReply).toHaveBeenCalledWith(1)
  })

  it('他人的回复：⋯ 里出现「举报」', async () => {
    const wrapper = await mountExpanded([reply(1)])
    const keys = moreItemsOfReply(wrapper, 0).map((i) => i.key)
    expect(keys).toContain('report')
  })

  it('自己的回复：⋯ 里不出现「举报」，按权限出现「删除」', async () => {
    const wrapper = await mountExpanded([
      reply(1, { author: { user_id: 1, username: '我', avatar_url: '' }, can_delete: true })
    ])
    const keys = moreItemsOfReply(wrapper, 0).map((i) => i.key)
    expect(keys).not.toContain('report')
    expect(keys).toContain('delete')
  })

  it('章节讨论无采纳概念：不渲染采纳入口', async () => {
    // 当前用户是楼主（topic.author.user_id = 1 且 authStore.user_id = 1），
    // 但章节帖不是问答帖 —— 采纳入口不该出现。
    const wrapper = await mountExpanded([reply(1)])
    expect(wrapper.find('.reply-accept-row').exists()).toBe(false)
  })

  it('回复超过一页时给出可见的截断提示与去向（不静默丢弃）', async () => {
    listTopics.mockResolvedValue({
      topics: [
        {
          id: 1,
          chapter_id: 1,
          category: 'discussion',
          title: '章节讨论帖',
          content: '内容',
          view_count: 0,
          reply_count: 150,
          created_at: '2026-08-01T10:00:00+08:00',
          author: { user_id: 1, username: '楼主', avatar_url: '' }
        }
      ],
      total: 1
    } as never)
    getTopic.mockResolvedValue({
      topic: {
        id: 1,
        chapter_id: 1,
        category: 'discussion',
        title: '章节讨论帖',
        content: '内容',
        view_count: 0,
        reply_count: 150,
        created_at: '2026-08-01T10:00:00+08:00',
        author: { user_id: 1, username: '楼主', avatar_url: '' }
      },
      replies: [reply(1)],
      page: 1,
      pages: 2,
      total: 150
    } as never)

    const wrapper = mount(ChapterDiscussion, {
      props: { chapterId: 1 },
      global: {
        plugins: [epLite()],
        stubs: { ForumImageGallery: true, ForumPostForm: true, ForumComposer: true }
      }
    })
    await flushPromises()
    await wrapper.findAll('.cursor-pointer')[0].trigger('click')
    await flushPromises()
    await new Promise((r) => setTimeout(r, 0))
    await flushPromises()

    expect(wrapper.text()).toContain('仅显示前 1 条回复')
    expect(wrapper.text()).toContain('查看全部')
  })

  it('楼中楼显示被回复人', async () => {
    const wrapper = await mountExpanded([
      reply(1, { parent_id: 9, parent_name: '上游', parent_avatar_url: '/static/uploads/avatar/p.png' })
    ])
    const parent = wrapper.find('.reply-parent')
    expect(parent.exists()).toBe(true)
    expect(parent.text()).toContain('上游')
  })
})
