/**
 * 招聘「公告」tab 接线契约（票 1941 / spec #1935 草稿 15）
 *
 * 本票把公告 tab 从空态占位接到内容精选 `notice` 类别（ADR-0045：不建第二套内容系统），
 * 并让「公告收藏」在收藏端可见（Q22：复用现行 `featured` 收藏类型，零新增类型）。
 * 这里钉住的是**接线不被悄悄拆掉**：类别写死、详情落点、分页归属、收藏计数口径。
 *
 * 装载协议本身不在这里重断：公告列表走 ADR-0027 的分页协议 `composables/usePagedList.uts`，
 * 它的代际守卫 / 空批次兜底 / 失败保留数据由 `utils/pagedListBehavior.test.js` 真跑承重
 * （见 docs/agents/guards.md：接线守护不构成 ③ 门证据）。
 */
const h = require('./contractHarness');
const read = h.read;

describe('招聘公告 tab 接线（票 1941）', () => {
  const list = read('pages/jobs/job-list.uvue');
  const panel = read('pages/jobs/components/notice-panel.uvue');
  const resume = read('pages/resume/resume.uvue');

  it('公告 tab 由模块私有组件承载，页面按 tab 切换挂载', () => {
    expect(list).toContain("import NoticePanel from './components/notice-panel.uvue'");
    expect(list).toMatch(/<notice-panel v-if="recruitTab === 'notice'" \/>/);
  });

  it('公告列表走内容精选 notice 类别，而不是自建公告接口', () => {
    expect(panel).toContain("import { getFeaturedContentsApi } from '../../../api/featured'");
    expect(panel).toMatch(/getFeaturedContentsApi\(page, NOTICE_PAGE_SIZE, 'notice'\)/);
    expect(panel).not.toContain('/recruit/notices');
  });

  it('空态只在「成功且空」时出现（协议给的 isEmpty），不是永久占位', () => {
    expect(panel).toMatch(/v-if="isEmpty"/);
    expect(panel).toContain('暂无公告');
    expect(panel).toContain("const isEmpty = paged.isEmpty");
    expect(list).not.toContain('后端公告接口未建');
  });

  it('公告卡片点进精选详情页（复用 Markdown 子集 + source + 收藏，零第二实现）', () => {
    expect(panel).toMatch(/@click="onNoticeDetail\(notice\.content_id\)"/);
    expect(panel).toContain("'/pages/featured/featured-detail?id='");
  });

  it('分页走 ADR-0027 的协议单点，不手写装载协议（棘轮：手写载体只减不增）', () => {
    expect(panel).toContain("import { usePagedList } from '../../../composables/usePagedList'");
    expect(panel).toMatch(/usePagedList<FeaturedContentItem>\(\(page : number\)/);
    expect(panel).toContain('hasMore: result.hasMore');
    // 手写协议的三件套不得回流
    expect(panel).not.toContain('noMore.value');
    expect(panel).not.toContain('noticePage.value++');
  });

  it('公告收藏在收藏端可见：计数走 featured（Q22 零新增类型），不再查恒 0 的 notice', () => {
    expect(resume).toContain("getFavoritesApi('featured', 1, 1)");
    expect(resume).not.toContain("getFavoritesApi('notice', 1, 1)");
    expect(resume).toContain('noticeFavCount.value = res.total');
  });
});
