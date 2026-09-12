// 管理端论坛的正文渲染（#880 / ADR-0044）。
//
// 治理面必须看**实际展示效果**：管理员判断违规依据的是学员最终看到的样子，
// 而不是 markdown 源串。raw HTML 在 escape 策略下以文本形式可见，
// 所以「渲染版」不会把藏起来的标记掩盖掉。
import { describe, it, expect, vi, beforeEach } from 'vitest'
import { mount, flushPromises } from '@vue/test-utils'
import { epLite } from '@/test/element-lite'

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

async function mountManage(topic: ReturnType<typeof adminTopic>) {
  listTopics.mockResolvedValue({ topics: [topic], total: 1 } as never);
  getTopic.mockResolvedValue({
    topic,
    replies: [],
    page: 1,
    pages: 1,
    total: 0
  } as never);
  const w = mount(ForumManage, {
    global: { plugins: [epLite()], stubs: { ForumImageGallery: true } }
  });
  await flushPromises();
  // 触发展开：el-table 的 expand-change 是页面接线入口
  const table = w.findComponent({ name: "ElTable" });
  table.vm.$emit("expand-change", topic, [topic]);
  await flushPromises();
  await new Promise((r) => setTimeout(r, 0));
  await flushPromises();
  return w;
}

describe('管理端正文渲染（#880 / ADR-0044）', () => {
  beforeEach(() => vi.clearAllMocks());

  it('markdown 帖在展开面板里渲染成展示效果', async () => {
    const w = await mountManage(adminTopic());
    expect(w.find('.topic-content h2').exists()).toBe(true);
  });

  it('纯文本帖仍按纯文本渲染（不误伤存量）', async () => {
    const w = await mountManage(adminTopic({ content: '## 原样', content_format: 'text' }));
    expect(w.find('.topic-content h2').exists()).toBe(false);
    expect(w.find('.topic-content').text()).toContain('## 原样');
  });
});
