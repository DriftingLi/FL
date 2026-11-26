// 管理端论坛的正文渲染（#880 / ADR-0044）。
//
// 治理面必须看**实际展示效果**：管理员判断违规依据的是学员最终看到的样子，
// 而不是 markdown 源串。raw HTML 在 escape 策略下以文本形式可见，
// 所以「渲染版」不会把藏起来的标记掩盖掉。
import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import { mount, flushPromises, type VueWrapper } from '@vue/test-utils'
import { ElMessage } from 'element-plus'
import { epLite } from '@/test/element-lite'

const { confirmMock, confirmDangerMock } = vi.hoisted(() => ({
  confirmMock: vi.fn(),
  confirmDangerMock: vi.fn()
}))

vi.mock('@/composables/useConfirm', () => ({
  useConfirm: () => ({ confirm: confirmMock, confirmDanger: confirmDangerMock })
}))

// 页面从 barrel 引 ElMessage；epLite 走 element-plus/es 子路径，不受此 mock 影响。
vi.mock('element-plus', () => ({ ElMessage: { success: vi.fn() } }))

vi.mock('@/api/request', () => ({
  unwrappedRequest: { get: vi.fn(), post: vi.fn(), delete: vi.fn(), put: vi.fn() }
}))
vi.mock('markstream-vue/index.css', () => ({}))

vi.mock('@/api/forum', async (importOriginal) => {
  const actual = await importOriginal<typeof import('@/api/forum')>()
  return {
    ...actual,
    adminForumApi: {
      listTopics: vi.fn(),
      getTopic: vi.fn(),
      deleteTopic: vi.fn(),
      deleteReply: vi.fn(),
      listReports: vi.fn(),
      handleReport: vi.fn(),
      featureTopic: vi.fn(),
      unfeatureTopic: vi.fn(),
      designateExperience: vi.fn(),
      revokeExperience: vi.fn()
    }
  }
})

import { adminForumApi } from '@/api/forum'
import ForumManage from '../ForumManage.vue'

const listTopics = vi.mocked(adminForumApi.listTopics)
const getTopic = vi.mocked(adminForumApi.getTopic)
const deleteReplyMock = vi.mocked(adminForumApi.deleteReply)

const topicActionCases = [
  {
    key: 'designateExperience',
    request: vi.mocked(adminForumApi.designateExperience),
    confirmation: confirmMock,
    otherConfirmation: confirmDangerMock,
    message: '认定后「管理端看正文」将进入备考经验区并带上精选标识，帖主获 +30 分（每帖仅一次；已加精的帖子不重复发放）。',
    title: '认定备考经验',
    type: 'info',
    successMessage: '已认定为备考经验',
    errorLabel: '认定备考经验'
  },
  {
    key: 'revokeExperience',
    request: vi.mocked(adminForumApi.revokeExperience),
    confirmation: confirmMock,
    otherConfirmation: confirmDangerMock,
    message: '确定取消「管理端看正文」的备考经验认定？精选位保留，已发放积分不收回。',
    title: '取消经验认定',
    type: 'info',
    successMessage: '已取消经验认定',
    errorLabel: '取消经验认定'
  },
  {
    key: 'featureTopic',
    request: vi.mocked(adminForumApi.featureTopic),
    confirmation: confirmMock,
    otherConfirmation: confirmDangerMock,
    message: '加精后「管理端看正文」将带上精选标识，帖主获 +30 分（每帖仅一次）。',
    title: '加精帖子',
    type: 'info',
    successMessage: '已加精',
    errorLabel: '加精'
  },
  {
    key: 'unfeatureTopic',
    request: vi.mocked(adminForumApi.unfeatureTopic),
    confirmation: confirmMock,
    otherConfirmation: confirmDangerMock,
    message: '确定取消「管理端看正文」的精选标识？',
    title: '取消精选',
    type: 'info',
    successMessage: '已取消精选',
    errorLabel: '取消精选'
  },
  {
    key: 'delete',
    request: vi.mocked(adminForumApi.deleteTopic),
    confirmation: confirmDangerMock,
    otherConfirmation: confirmMock,
    message: '确定删除帖子「管理端看正文」？删除后不可恢复。',
    title: '删除帖子',
    type: 'warning',
    successMessage: '已删除',
    errorLabel: '删除'
  }
] as const

function adminTopic(over: Record<string, unknown> = {}) {
  return {
    id: 1,
    category: 'discussion',
    title: '管理端看正文',
    content: '## 排查步骤',
    content_format: 'markdown',
    images: [],
    view_count: 0,
    reply_count: 0,
    created_at: '2026-08-01T10:00:00+08:00',
    author: { user_id: 2, username: '学员', avatar_url: '' },
    ...over
  }
}

function adminReply(over: Record<string, unknown> = {}) {
  return {
    id: 9,
    topic_id: 1,
    parent_id: null,
    parent_name: '',
    content: '先量 `E01` 电压',
    content_format: 'markdown',
    images: [],
    created_at: '2026-08-02T10:00:00+08:00',
    author: { user_id: 3, username: '答主', avatar_url: '' },
    ...over
  }
}

interface ManageVm {
  onTopicAction(key: string, row: ReturnType<typeof adminTopic>): void
  deleteReply(reply: ReturnType<typeof adminReply>): Promise<void>
  replyPages: Record<number, { replies: Array<{ id: number }>; page: number; pages: number; total: number }>
}

const wrappers: VueWrapper[] = []

function vmOf(wrapper: VueWrapper): ManageVm {
  return wrapper.vm as unknown as ManageVm
}

async function mountTopics(topic = adminTopic()) {
  listTopics.mockResolvedValue({ items: [topic], total: 1 } as never)
  const wrapper = mount(ForumManage, {
    global: { plugins: [epLite()], stubs: { ForumImageGallery: true } }
  })
  wrappers.push(wrapper)
  await flushPromises()
  return wrapper
}

async function openReplies(
  wrapper: VueWrapper,
  topic: ReturnType<typeof adminTopic>,
  replies: Array<ReturnType<typeof adminReply>>,
  total = replies.length,
  pages = 1
) {
  getTopic.mockResolvedValue({ topic, replies, page: 1, pages, total } as never)
  await wrapper.get('button.title-text').trigger('click')
  await flushPromises()
  await new Promise(resolve => setTimeout(resolve, 0))
  await flushPromises()
}

async function mountManage(topic: ReturnType<typeof adminTopic>) {
  const wrapper = await mountTopics(topic)
  await openReplies(wrapper, topic, [])
  return wrapper
}

beforeEach(() => {
  vi.clearAllMocks()
  confirmMock.mockResolvedValue('confirm')
  confirmDangerMock.mockResolvedValue('delete')
  deleteReplyMock.mockResolvedValue(undefined as never)
  for (const action of topicActionCases) action.request.mockResolvedValue(undefined as never)
  vi.spyOn(console, 'error').mockImplementation(() => {})
})

afterEach(() => {
  for (const wrapper of wrappers.splice(0)) wrapper.unmount()
  document.body.replaceChildren()
  vi.restoreAllMocks()
})

describe('管理端正文渲染（#880 / ADR-0044）', () => {
  it('markdown 帖在详情抽屉里渲染成展示效果', async () => {
    const w = await mountManage(adminTopic());
    expect(w.find('.topic-content h2').exists()).toBe(true);
  });

  it('markdown 回复在详情抽屉里也渲染', async () => {
    const topic = adminTopic()
    const w = await mountTopics(topic)
    await openReplies(w, topic, [adminReply()])
    expect(w.find('.reply-content code').exists()).toBe(true)
  });

  it('纯文本帖仍按纯文本渲染（不误伤存量）', async () => {
    const w = await mountManage(adminTopic({ content: '## 原样', content_format: 'text' }));
    expect(w.find('.topic-content h2').exists()).toBe(false);
    expect(w.find('.topic-content').text()).toContain('## 原样');
  });
});

describe('回复加载错误', () => {
  it('首页失败显示错误，重试成功后恢复空态', async () => {
    const topic = adminTopic()
    getTopic.mockRejectedValueOnce(new Error('load failed'))
    const w = await mountTopics(topic)

    await w.get('button.title-text').trigger('click')
    await flushPromises()

    expect(w.text()).toContain('回复加载失败')
    expect(w.text()).not.toContain('暂无回复')

    getTopic.mockResolvedValueOnce({ topic, replies: [], page: 1, pages: 0, total: 0 } as never)
    const retry = w.findAll('button').find(button => button.text() === '重试')
    expect(retry).toBeDefined()
    await retry!.trigger('click')
    await flushPromises()

    expect(getTopic).toHaveBeenLastCalledWith(topic.id, 1, 20)
    expect(w.text()).toContain('暂无回复')
    expect(w.text()).not.toContain('回复加载失败')
  })

  it('续页失败保留已有回复，重试后清错并只追加一次', async () => {
    const topic = adminTopic()
    const first = adminReply()
    const next = adminReply({ id: 10, content: '服务端续页' })
    const w = await mountTopics(topic)
    await openReplies(w, topic, [first], 2, 2)

    getTopic.mockRejectedValueOnce(new Error('load more failed'))
    await w.get('.reply-more button').trigger('click')
    await flushPromises()

    expect(w.text()).toContain('答主')
    expect(w.findAll('.reply-error')).toHaveLength(1)

    const callsBeforeRetry = getTopic.mock.calls.length
    getTopic.mockResolvedValueOnce({ topic, replies: [next], page: 2, pages: 2, total: 2 } as never)
    await w.get('.reply-more button').trigger('click')
    await flushPromises()

    expect(getTopic).toHaveBeenCalledTimes(callsBeforeRetry + 1)
    expect(getTopic).toHaveBeenLastCalledWith(topic.id, 2, 20)
    expect(w.text()).toContain('服务端续页')
    expect(w.findAll('.reply-error')).toHaveLength(0)
  })
})

describe('论坛管理操作', () => {
  it.each(topicActionCases)('$key 确认后请求、提示成功并刷新列表', async action => {
    const topic = adminTopic()
    const w = await mountTopics(topic)
    const vm = vmOf(w)

    vm.onTopicAction(action.key, topic)
    await flushPromises()

    expect(action.confirmation).toHaveBeenCalledWith(action.message, action.title, { type: action.type })
    expect(action.otherConfirmation).not.toHaveBeenCalled()
    expect(action.request).toHaveBeenCalledWith(topic.id)
    expect(ElMessage.success).toHaveBeenCalledWith(action.successMessage)
    expect(listTopics).toHaveBeenCalledTimes(2)
  })

  it.each(topicActionCases)('$key 请求失败时不发成功 toast、不刷新', async action => {
    const topic = adminTopic()
    const w = await mountTopics(topic)
    const error = new Error('request failed')
    action.request.mockRejectedValue(error)

    vmOf(w).onTopicAction(action.key, topic)
    await flushPromises()

    expect(action.request).toHaveBeenCalledWith(topic.id)
    expect(ElMessage.success).not.toHaveBeenCalled()
    expect(listTopics).toHaveBeenCalledTimes(1)
    expect(console.error).toHaveBeenCalledWith(`${action.errorLabel}失败:`, error)
  })

  it.each(topicActionCases)('$key 取消确认后不发请求、不刷新', async action => {
    const topic = adminTopic()
    const w = await mountTopics(topic)
    action.confirmation.mockRejectedValue(new Error('cancel'))

    vmOf(w).onTopicAction(action.key, topic)
    await flushPromises()

    expect(action.request).not.toHaveBeenCalled()
    expect(ElMessage.success).not.toHaveBeenCalled()
    expect(listTopics).toHaveBeenCalledTimes(1)
  })

  it('未知 action 保持 no-op', async () => {
    const topic = adminTopic()
    const w = await mountTopics(topic)
    const vm = vmOf(w)

    vm.onTopicAction('unknown', topic)
    vm.onTopicAction('toString', topic)
    await flushPromises()

    expect(confirmMock).not.toHaveBeenCalled()
    expect(confirmDangerMock).not.toHaveBeenCalled()
    for (const action of topicActionCases) expect(action.request).not.toHaveBeenCalled()
    expect(ElMessage.success).not.toHaveBeenCalled()
    expect(listTopics).toHaveBeenCalledTimes(1)
  })

  it('删帖成功先清回复缓存，再刷新列表', async () => {
    const topic = adminTopic()
    const reply = adminReply()
    const w = await mountTopics(topic)
    await openReplies(w, topic, [reply])
    const vm = vmOf(w)
    expect(vm.replyPages[topic.id].replies.map(item => item.id)).toEqual([reply.id])
    let cachedTopicCountAtRefresh: number | undefined
    listTopics.mockImplementationOnce(async () => {
      cachedTopicCountAtRefresh = Object.keys(vm.replyPages).length
      return { items: [topic], total: 1 } as never
    })

    vm.onTopicAction('delete', topic)
    await flushPromises()

    expect(cachedTopicCountAtRefresh).toBe(0)
    expect(vm.replyPages[topic.id]).toBeUndefined()
    expect(ElMessage.success).toHaveBeenCalledWith('已删除')
    expect(listTopics).toHaveBeenCalledTimes(2)
  })

  it('删帖失败保留回复缓存，不执行成功回调', async () => {
    const topic = adminTopic()
    const reply = adminReply()
    const w = await mountTopics(topic)
    await openReplies(w, topic, [reply])
    const vm = vmOf(w)
    vi.mocked(adminForumApi.deleteTopic).mockRejectedValue(new Error('delete failed'))

    vm.onTopicAction('delete', topic)
    await flushPromises()

    expect(vm.replyPages[topic.id].replies.map(item => item.id)).toEqual([reply.id])
    expect(ElMessage.success).not.toHaveBeenCalled()
    expect(listTopics).toHaveBeenCalledTimes(1)
  })

  it('删回复后恢复首页，忽略删除前发出的续页响应', async () => {
    const topic = adminTopic()
    const parent = adminReply({ content: '待删除的父回复' })
    const child = adminReply({ id: 10, parent_id: parent.id, content: '待删除的子回复' })
    const replacement = adminReply({ id: 11, content: '服务端新首页' })
    const next = adminReply({ id: 12, content: '服务端新续页' })
    const w = await mountTopics(topic)
    await openReplies(w, topic, [parent, child], 5, 2)
    let resolveOldPage!: (value: Awaited<ReturnType<typeof adminForumApi.getTopic>>) => void
    getTopic.mockImplementationOnce(() => new Promise(resolve => { resolveOldPage = resolve }))
    await w.get('.reply-more button').trigger('click')
    getTopic.mockResolvedValueOnce({ topic, replies: [replacement], page: 1, pages: 2, total: 3 } as never)

    await w.get('.reply-delete').trigger('click')
    await flushPromises()
    resolveOldPage({ topic, replies: [child], page: 2, pages: 2, total: 5 } as never)
    await flushPromises()

    expect(confirmDangerMock).toHaveBeenCalledWith(
      '确定删除这条回复？删除后不可恢复。', '删除回复', { type: 'warning' }
    )
    expect(deleteReplyMock).toHaveBeenCalledWith(parent.id)
    expect(getTopic.mock.calls).toEqual([[topic.id, 1, 20], [topic.id, 2, 20], [topic.id, 1, 20]])
    expect(w.text()).toContain('服务端新首页')
    expect(w.text()).not.toContain('待删除的父回复')
    expect(w.text()).not.toContain('待删除的子回复')
    expect(w.get('.reply-more').text()).toContain('剩余 2 条')

    getTopic.mockResolvedValueOnce({ topic, replies: [next], page: 2, pages: 2, total: 3 } as never)
    await w.get('.reply-more button').trigger('click')
    await flushPromises()
    expect(getTopic).toHaveBeenLastCalledWith(topic.id, 2, 20)
    expect(w.text()).toContain('服务端新续页')
    expect(ElMessage.success).toHaveBeenCalledWith('已删除')
    expect(listTopics).toHaveBeenCalledTimes(2)
  })

  it('删除后恢复首页失败时，可重试首页', async () => {
    const topic = adminTopic()
    const w = await mountTopics(topic)
    await openReplies(w, topic, [adminReply()], 2, 2)
    getTopic.mockRejectedValueOnce(new Error('reload failed'))
    await w.get('.reply-delete').trigger('click')
    await flushPromises()

    expect(w.text()).toContain('回复加载失败')
    getTopic.mockResolvedValueOnce({ topic, replies: [], page: 1, pages: 0, total: 0 } as never)
    const retry = w.findAll('button').find(button => button.text() === '重试')
    expect(retry).toBeDefined()
    await retry!.trigger('click')
    await flushPromises()

    expect(getTopic).toHaveBeenLastCalledWith(topic.id, 1, 20)
    expect(w.text()).toContain('暂无回复')
  })

  it('删回复失败不更新缓存、不发成功 toast、不刷新', async () => {
    const topic = adminTopic()
    const reply = adminReply()
    const w = await mountTopics(topic)
    await openReplies(w, topic, [reply], 3, 2)
    const vm = vmOf(w)
    deleteReplyMock.mockRejectedValue(new Error('delete failed'))

    await vm.deleteReply(reply)
    await flushPromises()

    expect(vm.replyPages[topic.id].replies.map(item => item.id)).toEqual([reply.id])
    expect(vm.replyPages[topic.id]).toMatchObject({ page: 1, pages: 2, total: 3 })
    expect(ElMessage.success).not.toHaveBeenCalled()
    expect(listTopics).toHaveBeenCalledTimes(1)
  })

  it('取消删回复不发请求，也不更新缓存或刷新', async () => {
    const topic = adminTopic()
    const reply = adminReply()
    const w = await mountTopics(topic)
    await openReplies(w, topic, [reply], 3, 2)
    const vm = vmOf(w)
    confirmDangerMock.mockRejectedValue(new Error('cancel'))

    await vm.deleteReply(reply)
    await flushPromises()

    expect(deleteReplyMock).not.toHaveBeenCalled()
    expect(vm.replyPages[topic.id].replies.map(item => item.id)).toEqual([reply.id])
    expect(vm.replyPages[topic.id]).toMatchObject({ page: 1, pages: 2, total: 3 })
    expect(ElMessage.success).not.toHaveBeenCalled()
    expect(listTopics).toHaveBeenCalledTimes(1)
  })
})
