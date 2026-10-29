/**
 * 考情资讯页 tab / chips 口径（#1940）—— 行为腿
 *
 * 口径来源：移动端 `docs/adr/0045-移动端内容消费复用内容精选域.md`「修订（2026-10-28，#1940 裁定）」段。
 * 断言**产物**（映射结果、去重结果），不是源码片段：loadUts 真执行 utils/examInfoTabs.uts。
 * 类别字典（industry / news / exam）是与后端 #2063 六态的契约，写死在这里当判据 ——
 * 后端换值或映射写错，这条先红。
 */
const path = require('path');
const { loadUts } = require('./utsHarness');

const FILE = path.join(__dirname, 'examInfoTabs.uts');

function item(id) {
  return {
    content_id: id,
    title: 't' + id,
    category: '',
    category_label: '',
    summary: '',
    cover_image: '',
    source: '',
    status: 'published',
    sort_order: 0,
    view_count: 0,
    published_at: '2026-10-01T00:00:00+08:00',
    created_at: '',
  };
}

const mod = loadUts(FILE, {});

describe('考情资讯页 tab / chips 口径（#1940）', () => {
  it('主 tab 收敛为原型四 tab，顺序固定', () => {
    expect(mod.EXAM_INFO_TABS).toEqual(['新闻资讯', '考试动态', '报名入口', '成绩查询']);
  });

  it('原页面多出的三个假入口/tab 不再出现（招聘信息 / 相关问题 / 岗位表）', () => {
    for (const gone of ['招聘信息', '相关问题', '岗位表']) {
      expect(mod.EXAM_INFO_TABS).not.toContain(gone);
    }
  });

  it('tab→类别：新闻资讯 = 行业新闻 + 政策法规，其余三 tab = 考情', () => {
    expect(mod.tabCategories(0)).toEqual(['industry', 'news']);
    expect(mod.tabCategories(1)).toEqual(['exam']);
    expect(mod.tabCategories(2)).toEqual(['exam']);
    expect(mod.tabCategories(3)).toEqual(['exam']);
  });

  it('chips 只在「新闻资讯」tab 出现（其余 tab 自身已是类别）', () => {
    expect(mod.showsFilters(0)).toBe(true);
    expect(mod.showsFilters(1)).toBe(false);
    expect(mod.showsFilters(2)).toBe(false);
    expect(mod.showsFilters(3)).toBe(false);
  });

  it('报名入口 / 成绩查询带说明文案，新闻资讯 / 考试动态不带', () => {
    expect(mod.tabHint(0)).toBe('');
    expect(mod.tabHint(1)).toBe('');
    expect(String(mod.tabHint(2)).length).toBeGreaterThan(0);
    expect(mod.tabHint(3)).toBe(mod.tabHint(2));
  });

  it('四个 chip 的请求参数：最新=latest / 热点=hot / 政策法规=news / 全部=不筛不排', () => {
    expect(mod.EXAM_INFO_FILTERS).toEqual(['最新资讯', '热点关注', '政策法规', '全部']);
    expect(mod.filterQuery(0)).toEqual({ category: '', sort: 'latest' });
    expect(mod.filterQuery(1)).toEqual({ category: '', sort: 'hot' });
    expect(mod.filterQuery(2)).toEqual({ category: 'news', sort: '' });
    expect(mod.filterQuery(3)).toEqual({ category: '', sort: '' });
  });

  it('分页追加按 content_id 去重：翻页重叠的同一篇只留一条（保留先来的位置）', () => {
    const page1 = [item(1), item(2)];
    const page2 = [item(2), item(3)];
    const out = mod.appendUnique(page1, page2);
    expect(out.map((x) => x.content_id)).toEqual([1, 2, 3]);
  });

  it('分页追加不改写已有顺序、也不丢字段（渲染依赖 category_label / published_at）', () => {
    const first = item(9);
    first.title = '旧的一条';
    first.category_label = '考情';
    const out = mod.appendUnique([first], [item(10)]);
    expect(out[0].title).toBe('旧的一条');
    expect(out[0].category_label).toBe('考情');
    expect(out[1].content_id).toBe(10);
  });

  it('空入参是恒等操作（首屏 / 末页都不会凭空造条目）', () => {
    expect(mod.appendUnique([], [])).toEqual([]);
    expect(mod.appendUnique([item(1)], []).map((x) => x.content_id)).toEqual([1]);
    expect(mod.appendUnique([], [item(2)]).map((x) => x.content_id)).toEqual([2]);
  });
});
