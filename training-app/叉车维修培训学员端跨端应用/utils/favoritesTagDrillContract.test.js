/**
 * 「我的收藏」题目类型内按题库类别下钻（#1947）—— 断言**行为**，不断言源码片段。
 *
 * ## 这一层为什么存在
 *
 * Q25 定案（spec #1935 / 报告 §8）：**不推翻内容类型 tab 结构**，只在「题目」类型内部加一层
 * 类别下钻。后端 #2067（PR #2085）已把两个前提落地：
 *   - `GET /favorites?tag_id=<id>` —— 仅 `target_type=question` 生效，**传其它类型 400**；
 *   - 条目带 `tags[]`（本票的定案改为字典驱动，故不消费它）。
 *
 * ## 两条缝
 *
 * 1. **api 层**（`api/favorite.uts`）：经 `loadUts` 真跑模块，桩掉 `getMapped` 只观察它拿到的
 *    查询参数与映射结果 —— 覆盖「`tag_id` 不得越界到非 question」这条**会 400** 的硬约束。
 * 2. **纯决策层**（`utils/favoriteTagDrill.uts`）：`favoriteTagChips` 决定「这一层有没有、有哪些项、
 *    什么顺序」。页面只剩接线（.uvue 在 node 里跑不了，故决策必须搬进 .uts）。
 *
 * 为什么决策必须搬出去（先例 #1237）：内联在 .uvue 里时只能被「取函数体 + new Function」那类
 * 自搓执行器跑，按 `docs/agents/guards.md` 的分类器判据落不进**行为守护** ⇒ 这一面没有 ③ 承重证据。
 *
 * ## 成对断言
 *
 * 必红：把 `getFavoritesApi` 里的 `targetType == 'question'` 去掉（变成任何类型都带 tag_id）
 * ⇒ 第一组判红；把 `favoriteTagChips` 的类型门去掉（任何类型都返回类别）⇒ 第二组判红。
 */
const path = require('path');

const { readText, loadUts } = require('./utsHarness');
const ROOT = path.join(__dirname, '..');
const API = 'api/favorite.uts';
const ENVELOPE = 'api/envelope.uts';
const HELPERS = 'api/helpers.uts';
const DRILL = 'utils/favoriteTagDrill.uts';
const PAGE = 'pages/profile/favorites.uvue';
const read = (rel) => readText(path.join(ROOT, rel));

/** 收藏列表响应夹具（形状照 swagger `favorite.FavoritePageResult`：数组键是 `favorites`） */
const pageData = (items) => ({ favorites: items, total: items.length });

/**
 * 装一个 api 模块：`getMapped` 用桩，其余（helpers / decodePage）都是**真源**
 * ⇒ 断言里看到的分页与映射就是生产跑的同一段代码。
 */
function loadApi() {
  const helpers = loadUts(path.join(ROOT, HELPERS), {});
  const envelope = loadUts(path.join(ROOT, ENVELOPE), {
    // envelope 顶层 import 了这两个，只有 getPaged 用得到；decodePage 用不到，但绑定必须齐（fail-closed）
    getMapped: () => Promise.reject(new Error('unused')),
    mergeQueryParams: (base, extra) => Object.assign({}, base, extra),
    toNumber: helpers.toNumber,
  });
  const seen = { url: '', params: null, data: pageData([]) };
  const api = loadUts(path.join(ROOT, API), {
    get: () => Promise.reject(new Error('unused')),
    getMapped: (url, params, mapper) => {
      seen.url = url;
      seen.params = params;
      return Promise.resolve(mapper(seen.data));
    },
    post: () => Promise.reject(new Error('unused')),
    postMapped: () => Promise.reject(new Error('unused')),
    del: () => Promise.reject(new Error('unused')),
    toNumber: helpers.toNumber,
    toStr: helpers.toStr,
    decodePage: envelope.decodePage,
  });
  return { api, seen };
}

/** 题库类别字典夹具（照 `TagItem`：`id` 是类别 id，与收藏条目的 `tag_id` 同一空间） */
const tag = (id, name, sortOrder) => ({
  id, code: 't' + id, name, description: '', sort_order: sortOrder, status: 1, question_count: 3,
});

const tags = () => [tag(10, '法规', 1), tag(11, '结构', 2), tag(12, '液压', 3)];

describe('#1947 收藏列表：tag_id 只在「题目」类型下发', () => {
  it('question + tagId>0 ⇒ 查询串带 tag_id', async () => {
    const { api, seen } = loadApi();
    await api.getFavoritesApi('question', 1, 20, 7);
    expect(seen.url).toBe('/favorites');
    expect(seen.params.target_type).toBe('question');
    expect(seen.params.tag_id).toBe('7');
  });

  it('非 question + tagId>0 ⇒ **不得**下发 tag_id（后端对其它类型传 tag_id 直接 400）', async () => {
    for (const t of ['course', 'chapter', 'topic', 'featured', 'job', '']) {
      const { api, seen } = loadApi();
      await api.getFavoritesApi(t, 1, 20, 7);
      expect(Object.prototype.hasOwnProperty.call(seen.params, 'tag_id')).toBe(false);
    }
  });

  it('tagId 缺省 / 0（「全部」）⇒ 不下发 tag_id，也不下发空串', async () => {
    for (const args of [[], [0]]) {
      const { api, seen } = loadApi();
      await api.getFavoritesApi('question', 1, 20, ...args);
      expect(Object.prototype.hasOwnProperty.call(seen.params, 'tag_id')).toBe(false);
    }
  });

  it('既有参数不回归：target_type / page / page_size 口径照旧（空类型不下发 target_type）', async () => {
    const { api, seen } = loadApi();
    await api.getFavoritesApi('question', 2, 10, 0);
    expect(seen.params.target_type).toBe('question');
    expect(seen.params.page).toBe('2');
    expect(seen.params.page_size).toBe('10');

    const bare = loadApi();
    await bare.api.getFavoritesApi('', 1, 20);
    expect(Object.prototype.hasOwnProperty.call(bare.seen.params, 'target_type')).toBe(false);
  });

  it('映射不回归：favorites 数组 → items，total 照旧（本票不消费条目 tags[]）', async () => {
    const { api, seen } = loadApi();
    seen.data = pageData([
      { favorite_id: 1, target_type: 'question', target_id: 9, title: '液压题', created_at: '2026-10-01' },
      { favorite_id: 2, target_type: 'question', target_id: 10, title: '法规题', created_at: '2026-10-02' },
    ]);
    const res = await api.getFavoritesApi('question', 1, 20, 7);
    expect(res.items.length).toBe(2);
    expect(res.items[0].title).toBe('液压题');
    expect(res.items[1].target_id).toBe(10);
    expect(res.total).toBe(2);
  });
});

describe('#1947 二级类别 chip 行：只有「题目」类型才有这一层', () => {
  const chips = (type, dict) => loadUts(path.join(ROOT, DRILL), {}).favoriteTagChips(type, dict);

  it('question ⇒ 首项是「全部」（tag_id=0），其后是各类别名', () => {
    const c = chips('question', tags());
    expect(c[0].tag_id).toBe(0);
    expect(c[0].name).toBe('全部');
    expect(c.slice(1).map((x) => x.name)).toEqual(['法规', '结构', '液压']);
    expect(c.slice(1).map((x) => x.tag_id)).toEqual([10, 11, 12]);
  });

  it('顺序 = 字典顺序，客户端**不重排**（sort_order 是后端的单点）', () => {
    // 夹具故意让「按名字排」与「后端顺序」不同：法(U+6CD5) < 液(U+6DB2) < 结(U+7ED3)。
    // 后端 /tags 的排序真源是 catalog_specs.go:167（questionTagCatalogSpec 的 OrderBy "sort_order ASC, id ASC"，
    // 经 catalog_service.go 的 catalogList 落地）⇒ 客户端再排一次就是第二份排序知识，且会把这组改成
    expect(chips('question', tags()).slice(1).map((x) => x.name)).toEqual(['法规', '结构', '液压']);
  });

  it('类别字典为空 ⇒ 空数组（只有「全部」的栏没有意义，整行不渲染）', () => {
    expect(chips('question', [])).toEqual([]);
  });

  it('其余五个类型 ⇒ 空数组（这一层不越界到课程/章节/帖子/资讯/职位）', () => {
    for (const t of ['course', 'chapter', 'topic', 'featured', 'job', '']) {
      expect(chips(t, tags())).toEqual([]);
    }
  });
});

describe('#1947 接线：页面真的把这一层挂上去了', () => {
  it('页面 import 决策模块并渲染 chip 行', () => {
    const page = read(PAGE);
    expect(page).toContain("import { favoriteTagChips } from '../../utils/favoriteTagDrill'");
    expect(page).toContain('favoriteTagChips(');
  });

  /** 取 `marker` 起、到 `</script>` 或 `closeMarker` 之前的那段源码，并剥掉 // 注释（防散文误伤） */
  const codeFrom = (marker, closeMarker) => {
    const page = read(PAGE);
    const start = page.indexOf(marker);
    expect(start).toBeGreaterThan(-1);
    const end = closeMarker ? page.indexOf(closeMarker, start) : page.indexOf('</script>', start);
    expect(end).toBeGreaterThan(start);
    return page.slice(start, end).replace(/\/\/[^\n]*/g, '');
  };

  it('类别字典挂在 onShow 上（与练习页同节拍）：换证件后 chip 行不会停在旧证件的类别', () => {
    // 只挂 onLoad 的话，学员在别的页换了目标证件，本页 chip 行仍是旧证件的类别 —— 列表按新
    // 证件的题走、chip 却按旧证件的类目，两边口径脱节。练习页的节拍也是每次 onShow 重取。
    expect(codeFrom('onShow(', null)).toContain('loadTagChips()');
  });

  it('重置不被在途请求吞掉：放行判据必须排除 reset，且过期响应要丢弃', () => {
    // 原判据 `if (loading.value) return` 在 reset 之前 ⇒ 连点两个类别时后一次重置被丢弃，
    // chip 高亮 B 而列表还是 A，随后 loadMore 还会把 B 的第 2 页拼到 A 的第 1 页。
    const code = codeFrom('async function loadFavorites', null);
    expect(code).toContain('if (!reset && loading.value) return');
    expect(code).toContain('if (seq != loadSeq.value) return');
  });

  it('点类别必须真的重拉列表（只改 tagId 不改列表 = chip 动了、内容没动）', () => {
    const code = codeFrom('function onTagClick', 'function onTypeClick');
    expect(code).toContain('tagId.value = val');
    expect(code).toContain('loadFavorites(true)');
  });

  it('页面把当前类别作为第四个参数传给列表接口（漏传 = 点类别没反应）', () => {
    expect(read(PAGE)).toMatch(/getFavoritesApi\(currentType\.value,[^)]*tagId\.value/);
  });

  it('模块导出 favoriteTagChips（loadUts 回读名，改名即判红）', () => {
    expect(read(DRILL)).toContain('export function favoriteTagChips(');
  });
});
