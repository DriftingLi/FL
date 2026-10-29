/**
 * 首页新闻板块取数口径（#1939）—— 行为腿
 *
 * 断言**产物**（合并后的顺序 / 截断 / 去重），不是源码片段：loadUts 真执行 utils/dashboardNews.uts。
 * 类别字典映射（考情=exam / 行业新闻=industry / 政策法规=news）是与后端 #2063 六态的契约，
 * 写死在这里当判据：后端换字典或映射写错，这条先红。
 */
const path = require('path');
const { loadUts } = require('./utsHarness');

const FILE = path.join(__dirname, 'dashboardNews.uts');

function item(id, publishedAt, title) {
  return {
    content_id: id,
    title: title || 't' + id,
    category: '',
    category_label: '',
    summary: '',
    cover_image: '',
    source: '',
    status: 'published',
    sort_order: 0,
    view_count: 0,
    published_at: publishedAt,
    created_at: '',
  };
}

const mod = loadUts(FILE, {});

describe('首页新闻板块取数口径（#1939）', () => {
  it('类别范围 = 考情 + 行业新闻 + 政策法规（exam / industry / news）', () => {
    expect(mod.DASHBOARD_NEWS_CATEGORIES).toEqual(['exam', 'industry', 'news']);
  });

  it('三个类别的条目合并后按发布时间倒序（不按类别分组、不按请求顺序）', () => {
    const out = mod.mergeDashboardNews(
      [
        [item(1, '2026-10-01T09:00:00+08:00')],
        [item(2, '2026-10-20T12:00:00+08:00'), item(3, '2026-09-05T08:00:00+08:00')],
        [item(4, '2026-10-24T18:30:00+08:00')],
      ],
      5
    );
    expect(out.map((x) => x.content_id)).toEqual([4, 2, 1, 3]);
  });

  it('取前 N：板块只有 5 个卡片位，多出来的丢掉而不是悄悄堆在末尾', () => {
    const out = mod.mergeDashboardNews(
      [[item(1, '2026-10-01T00:00:00+08:00'), item(2, '2026-10-02T00:00:00+08:00')],
       [item(3, '2026-10-03T00:00:00+08:00')]],
      2
    );
    expect(out.map((x) => x.content_id)).toEqual([3, 2]);
  });

  it('缺发布时间的内容垫底：不会被当成「最新」霸占第一条', () => {
    const out = mod.mergeDashboardNews(
      [[item(1, ''), item(2, '2026-10-01T00:00:00+08:00')]],
      5
    );
    expect(out.map((x) => x.content_id)).toEqual([2, 1]);
  });

  it('同 content_id 只出现一次（类别请求若被后端忽略也不会重复出卡）', () => {
    const out = mod.mergeDashboardNews(
      [[item(7, '2026-10-01T00:00:00+08:00')], [item(7, '2026-10-01T00:00:00+08:00')]],
      5
    );
    expect(out.length).toBe(1);
  });

  it('limit <= 0 表示不限（调用方显式要全量时不截断）', () => {
    const out = mod.mergeDashboardNews(
      [[item(1, '2026-10-01T00:00:00+08:00'), item(2, '2026-10-02T00:00:00+08:00')]],
      0
    );
    expect(out.length).toBe(2);
  });
});
