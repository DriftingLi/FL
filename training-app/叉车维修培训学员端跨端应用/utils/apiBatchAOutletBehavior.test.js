/**
 * #652 T14 · 域 api 出口收紧 **行为级**测试
 *
 * ## 它在守什么（guards.md 末节三问的「行为兜底」那一半）
 *
 * 本票同时立了两个锁，分工必须说清：
 *   - 接线锁 `apiBatchAContract.test.js`（分类器判 **[接线]**）：读源码文本，守的是**出口拓扑**
 *     —— 有载荷的 GET 必须经 `getMapped`、DTO 类型住在 `types/`、allowlist 清零。
 *     它回答不了「映射出来的字段对不对」：把 `buildJobPosting` 的某行改坏，字面量都还在 ⇒ 接线锁照样绿。
 *   - **本文件**（分类器判 **[行为]**）：把 `api/job.uts` / `api/search.uts` **真跑起来**，
 *     用替身 `getMapped` **真的调用出口传进来的 mapper**，断言裸响应被映成的 DTO 逐字段正确。
 *     —— 这就是 Q3 要的「这条接线由哪个行为守护兜底行为」。
 *
 * ## 为什么必须是行为而不是再来一份源码断言
 *
 * 三件**只有跑起来才看得见**的事，正是本票改动的核心风险面：
 *   ① 出口注入的**回退缺省值**真的落进 DTO —— `searchAllApi` 的 `keyword`、`searchByTypeApi` 的
 *      `type` / `page` 是**由出口闭包传进 mapper** 的（单参写法会静默丢掉回退值）；
 *   ② **空分区 / 缺省字段塌成合法值**（null 分区 → `{items:[],total:0}`、缺 `apply_state` → `''`、
 *      缺 `forced_offline` → `false`、缺 `salary_min` → `null` 而不是 0）；
 *   ③ **请求形态逐字不变**（列表的手工 query 串、`credential_id>0` 才下发、void 两条仍走裸 `post`）。
 *
 * ## 缝与边界（写实）
 *
 * `utils/utsHarness.js` 的 `loadUts` 把 `.uts` 去类型后当 JS 执行，依赖由本文件注入（先例
 * `recruitDisplayBehavior.test.js` / `concurrent401RefreshBehavior.test.js`）。
 * `api/helpers.uts` 零依赖 ⇒ 注入**它的真实现**（不手抄 toStr/toNumber —— 手抄正是漂移起点）。
 * 渲染层（`.uvue` 模板）node 里跑不了，那是 ①a 真机门的事。
 */
const path = require('path');

const { loadUts } = require('./utsHarness');

const API_DIR = path.join(__dirname, '..', 'api');
const HELPERS_UTS = path.join(API_DIR, 'helpers.uts');
const JOB_UTS = path.join(API_DIR, 'job.uts');
const SEARCH_UTS = path.join(API_DIR, 'search.uts');

/** 真源：helpers 是零依赖纯函数，直接跑真实现 */
const helpers = () => loadUts(HELPERS_UTS, {});

/**
 * `getMapped` 替身 —— **真的调用出口传进来的 mapper**。
 * 这是「行为」与「源码文本」的分界：若被测方退化成不传 mapper（或塞个恒等函数），
 * 本替身立刻把出口原样响应吐给调用方 ⇒ 下面的逐字段断言全红。
 */
function makeGetMapped(raw, recorder) {
  return (url, params, mapper) => {
    recorder.push({ url, params });
    if (typeof mapper !== 'function') {
      throw new Error(`出口 ${url} 没有传 mapper 回调 —— 退回裸 get 了？`);
    }
    return Promise.resolve(mapper(raw));
  };
}

/** 按出口装载 job.uts：同时录下 getMapped / post 两条通路的调用形态 */
function loadJobApi(raw) {
  const getCalls = [];
  const postCalls = [];
  const mod = loadUts(JOB_UTS, {
    getMapped: makeGetMapped(raw, getCalls),
    post: (url, payload) => {
      postCalls.push({ url, payload });
      return Promise.resolve(undefined);
    },
  });
  return { mod, getCalls, postCalls };
}

/** 按出口装载 search.uts（helpers 是真源，不注入假的） */
function loadSearchApi(raw) {
  const getCalls = [];
  const h = helpers();
  const mod = loadUts(SEARCH_UTS, {
    getMapped: makeGetMapped(raw, getCalls),
    toNumber: h.toNumber,
    toStr: h.toStr,
  });
  return { mod, getCalls };
}

/** 详情裸响应（真机同形字段；null / 缺省字段是本票映射最易写错的位置） */
const JOB_DETAIL_RAW = {
  id: 42,
  title: '专业叉车维修员',
  region: '广州',
  salary_min: null,
  salary_max: null,
  salary_text: '6000-9000 元/月',
  experience_req: '',
  description: '测试',
  specialty_id: null,
  published_at: '2026-09-02',
  company_name: 'A公司',
  status: 'open',
  forced_offline: null,
  apply_state: null,
  cooldown_days: null,
};

/** 列表裸响应：两条记录，其中一条**故意缺字段**（走默认值那一支） */
const JOB_LIST_RAW = {
  items: [
    JOB_DETAIL_RAW,
    { id: 7, title: '电池维修员', company_name: 'B公司', status: 'open' },
  ],
  total: 7,
};

describe('job.uts：有载荷的 GET 出口经 getMapped，映射行为逐字段正确', () => {
  it('详情：URL 与 params 形态不变，且裸响应被映成完整 JobPosting', async () => {
    const { mod, getCalls } = loadJobApi(JOB_DETAIL_RAW);
    const got = await mod.getJobDetailApi(42);

    expect(getCalls).toEqual([{ url: '/jobs/42', params: null }]);
    // 整对象比对：任一字段被漏读 / 读错即红（接线锁做不到这一点）
    expect(got).toEqual({
      id: 42,
      title: '专业叉车维修员',
      region: '广州',
      salary_min: null,
      salary_max: null,
      salary_text: '6000-9000 元/月',
      experience_req: '',
      description: '测试',
      specialty_id: null,
      published_at: '2026-09-02',
      company_name: 'A公司',
      status: 'open',
      forced_offline: false, // 缺省 → false，不是 null、也不是 true
      apply_state: '', // 缺省 → ''，不是 null（页面按长度判可投）
      cooldown_days: null, // 数字缺省保持 null，**不是 0**（0 会被当成「当天可投」）
    });
  });

  it('列表：手工 query 串逐字保留，items 逐条经同一份 buildJobPosting', async () => {
    const { mod, getCalls } = loadJobApi(JOB_LIST_RAW);
    const got = await mod.getJobListApi({ page: 2, page_size: 10, region: '广州' });

    expect(getCalls).toEqual([{ url: '/jobs?page=2&page_size=10&region=%E5%B9%BF%E5%B7%9E', params: null }]);
    expect(got.total).toBe(7);
    expect(got.items).toHaveLength(2);
    expect(got.items[0].title).toBe('专业叉车维修员');
    // 第二条缺字段 ⇒ 走默认值那一支，证明列表与详情共用同一份映射
    expect(got.items[1]).toEqual({
      id: 7,
      title: '电池维修员',
      region: '',
      salary_min: null,
      salary_max: null,
      salary_text: '',
      experience_req: '',
      description: '',
      specialty_id: null,
      published_at: '',
      company_name: 'B公司',
      status: 'open',
      forced_offline: false,
      apply_state: '',
      cooldown_days: null,
    });
  });

  it('列表默认参数 → 裸 /jobs（不带 ?，与收紧前的拼串分支一致）', async () => {
    const { mod, getCalls } = loadJobApi(JOB_LIST_RAW);
    await mod.getJobListApi();
    expect(getCalls[0].url).toBe('/jobs');
  });

  it('列表过滤值为 0 / 空串时不下发（与服务端 nil 语义一致）', async () => {
    const { mod, getCalls } = loadJobApi(JOB_LIST_RAW);
    await mod.getJobListApi({ page: 0, page_size: 0, specialty_id: 0, region: '', experience: '' });
    expect(getCalls[0].url).toBe('/jobs');
  });

  it('举报 / 投递仍走裸 post（void 透传），不经 getMapped', async () => {
    const { mod, getCalls, postCalls } = loadJobApi(null);
    await mod.reportJobApi(42, '虚假信息');
    await mod.applyJobApi(42);

    expect(getCalls).toEqual([]); // 这两条一个 getMapped 都不该碰
    expect(postCalls.map((c) => c.url)).toEqual(['/jobs/42/report', '/jobs/42/apply']);
    expect(postCalls[0].payload).toEqual({ reason: '虚假信息' });
    expect(postCalls[1].payload).toBeUndefined(); // 投递无载荷
  });

  it('举报原因表仍是那四条（页面渲染直接吃它）', () => {
    const { mod } = loadJobApi(null);
    expect(mod.REPORT_REASONS.map((r) => r.key)).toEqual(['fake', 'illegal', 'harassment', 'other']);
  });
});

describe('search.uts：出口注入的回退缺省值真的落进 DTO', () => {
  it('全量搜索：params 含 credential，keyword 回退值来自出口而非响应', async () => {
    // 响应**故意不带 keyword 字段** —— 回退值只能由出口闭包注入
    const { mod, getCalls } = loadSearchApi({
      courses: { items: [{ type: 'course', id: 3, title: '叉车基础' }], total: 1 },
      chapters: null, // 空分区 → 塌成合法空值
    });
    const got = await mod.searchAllApi('叉车', 7);

    expect(getCalls).toEqual([
      { url: '/search', params: { keyword: '叉车', credential_id: '7' } },
    ]);
    expect(got.keyword).toBe('叉车'); // ← 单参写法会丢掉这个回退值，本条即锁它
    expect(got.courses.total).toBe(1);
    expect(got.courses.items[0].title).toBe('叉车基础');
    expect(got.chapters).toEqual({ items: [], total: 0 });
    expect(got.questions).toEqual({ items: [], total: 0 }); // 完全缺席的分区同样塌成空
    expect(got.topics).toEqual({ items: [], total: 0 });
  });

  it('credentialId=0（未声明）→ 不下发 credential_id', async () => {
    const { mod, getCalls } = loadSearchApi({});
    await mod.searchAllApi('kw');
    expect(getCalls[0].params).toEqual({ keyword: 'kw' });
    expect(getCalls[0].params).not.toHaveProperty('credential_id');
  });

  it('响应带 keyword 时以响应回显值为准（回退值不覆盖服务端真值）', async () => {
    const { mod } = loadSearchApi({ keyword: '服务端回显' });
    const got = await mod.searchAllApi('调用方给的', 0);
    expect(got.keyword).toBe('服务端回显');
  });

  it('分页搜索：type/page 回退 + 分页字段与 items 逐条映射', async () => {
    const { mod, getCalls } = loadSearchApi({
      // 故意不带 type / page，用出口注入的回退值
      total: '12',
      pages: 3,
      items: [{ type: 'chapter', id: 9, title: '第二章', snippet: '…', hit_field: 'title' }],
    });
    const got = await mod.searchByTypeApi('kw', 'chapter', 3, 5, 7);

    expect(getCalls[0].url).toBe('/search');
    expect(getCalls[0].params).toEqual({
      keyword: 'kw',
      type: 'chapter',
      page: '3',
      page_size: '5',
      credential_id: '7',
    });
    expect(got.type).toBe('chapter'); // ← 同样是出口注入的回退值
    expect(got.page).toBe(1); // 响应缺 page → 出口给的缺省 1，不是 0
    expect(got.total).toBe(12); // 数字字段按 toNumber 归一（字符串 "12" 也能读）
    expect(got.pages).toBe(3);
    expect(got.items).toHaveLength(1);
    expect(got.items[0]).toEqual({
      type: 'chapter',
      id: 9,
      title: '第二章',
      cover: '',
      summary: '',
      snippet: '…',
      hit_field: 'title',
      parent_id: 0,
    });
  });

  it('items 缺席 / items 为 null → 空数组（分页首屏不炸）', async () => {
    const { mod } = loadSearchApi({ items: null, total: 0 });
    const got = await mod.searchByTypeApi('kw', 'course');
    expect(got.items).toEqual([]);
    expect(got.total).toBe(0);
  });
});
