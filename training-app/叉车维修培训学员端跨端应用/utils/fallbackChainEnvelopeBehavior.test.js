/**
 * PR-D #1664 · 兜底链收敛 + 四类异形处置 —— **行为级**守护（ADR-0037 ⑥：真执行 + 成对取证）
 *
 * ## 它在守什么
 *
 * 本票把 15 处棘轮载体（手写回退链 / 具名键 / 四类异形）从手写信封拆包迁到
 * `decodePage`（ADR-0037 ④.4）。源码文本只能证明「字面量出现过」，证明不了
 * 「键序真的按声明的顺序回退」「total 缺失时真的回落 items.length」「吞错分支真的没了」。
 * 本文件把被迁的七个模块**真跑起来**（`utsHarness.loadUts` + 替身 `getMapped` / `get`
 * **真的回调 mapper**），断言只有跑起来才看得见的语义：
 *
 *   1. **回退链语义**（M1/M2）—— 原 `data['list'] ?? data['items']` 与 exams→items→records
 *      三级 if-else 收敛为一次 keys 声明后，键序即回退序；把 keys 改坏 / 改序立刻红。
 *   2. **具名键不放宽**（M3）—— favorite 只认 `favorites`，**不**因模块默认键序而凭空接受 `items`。
 *   3. **侧计数器与冗余标题保留手读**（M4/M5）—— `unread_count` / `job_title` 不属于信封三字段，
 *      迁移不得把它们卷进空页通道。
 *   4. **`page_size` 恒等于请求值**（M6）—— 旧误读「响应回显的每页大小」结构上消亡（票面②）。
 *   5. **`pages` 不读后端回显**（M7）—— 按 total / 请求 pageSize 折算；后端回显一个更大的数也不作数。
 *   6. **吞错点已删**（M8）—— `getMockExamHistoryApi` 失败原样上抛，不再合成空页。
 *   7. **嵌套父列表 / 不回显端点**（F/G 组）—— forumDto 详情与 recruit 简历库装载逐字段同解。
 *
 * 与接线锁的分工：`forumContract` / `profileContract` / `dashboardContract` / `apiBatchAContract`
 * 守源码形状；`envelopeAdoptionContract`（基线 12）按 api/ 全文双向对账三键索引读；
 * 本文件回答 guards.md 三问里「这条接线由哪个行为守护兜底行为」的那一半。
 * 载体路径写成**本文件内的字面量**：`scripts/classify-guards.mjs` 按文件自身文本判分类。
 */
const fs = require('fs');
const os = require('os');
const path = require('path');

const { loadUts, readText } = require('./utsHarness');

const API_DIR = path.join(__dirname, '..', 'api');
const HELPERS_UTS = path.join(API_DIR, 'helpers.uts');
const ENVELOPE_UTS = path.join(API_DIR, 'envelope.uts');
const QUESTION_INTERACTION_UTS = path.join(API_DIR, 'questionInteraction.uts');
const MOCK_EXAM_UTS = path.join(API_DIR, 'mockExam.uts');
const FAVORITE_UTS = path.join(API_DIR, 'favorite.uts');
const NOTIFICATION_UTS = path.join(API_DIR, 'notification.uts');
const SEARCH_UTS = path.join(API_DIR, 'search.uts');
const FORUM_DTO_UTS = path.join(API_DIR, 'forumDto.uts');
const RECRUIT_UTS = path.join(API_DIR, 'recruit.uts');

/** 真源：helpers 是零依赖纯函数（手抄 toStr/toNumber 正是漂移起点） */
const helpers = () => loadUts(HELPERS_UTS, {});

/**
 * 出口替身 —— **真的调用出口传进来的 mapper**（裸 `get` 与 `getMapped` 两条通路同形）。
 * 若被测方退化成不传 mapper，替身当场抛错 ⇒ 下面的逐字段断言全红。
 */
function makeResponder(raw, recorder, via) {
  return (url, params, mapper) => {
    recorder.push({ url, params });
    if (via === 'mapped' && typeof mapper !== 'function') {
      throw new Error(`出口 ${url} 没有传 mapper 回调 —— 退回裸 get 了？`);
    }
    if (via === 'bare') return Promise.resolve(raw);
    return Promise.resolve(mapper(raw));
  };
}

/** 拒绝替身 —— 触发「不吞错」那条路径（M8 靠它证明 catch 已被拆掉） */
function makeRejecting(recorder) {
  return (url, params) => {
    recorder.push({ url, params });
    return Promise.reject(new Error('network down'));
  };
}

/** 信封模块：与被测域共用同一份真实现（缺绑定 = fail-closed） */
const envelopeFor = (h, gm) => loadUts(ENVELOPE_UTS, { getMapped: gm, toNumber: h.toNumber });

/** 装载 questionInteraction.uts（imports：get/post/put/del + toNumber/toStr/toBool + decodeList/decodePage） */
function loadQuestionInteraction(raw, calls = [], file = QUESTION_INTERACTION_UTS, opt = {}) {
  const h = helpers();
  const get = opt.reject ? makeRejecting(calls) : makeResponder(raw, calls, 'bare');
  const env = envelopeFor(h, makeResponder({}, calls, 'mapped'));
  const mod = loadUts(file, {
    get,
    post: get,
    put: get,
    del: get,
    toNumber: h.toNumber,
    toStr: h.toStr,
    toBool: h.toBool,
    decodeList: env.decodeList,
    decodePage: env.decodePage,
  });
  return { mod, calls };
}

/** 装载 mockExam.uts（imports：getMapped/post/postMapped + toNumber/toStr/toBool + decodePage） */
function loadMockExam(raw, calls = [], file = MOCK_EXAM_UTS, opt = {}) {
  const h = helpers();
  const gm = opt.reject ? makeRejecting(calls) : makeResponder(raw, calls, 'mapped');
  const env = envelopeFor(h, gm);
  const mod = loadUts(file, {
    getMapped: gm,
    post: gm,
    postMapped: gm,
    toNumber: h.toNumber,
    toStr: h.toStr,
    toBool: h.toBool,
    decodePage: env.decodePage,
  });
  return { mod, calls };
}

/** 装载 favorite.uts */
function loadFavorite(raw, calls = [], file = FAVORITE_UTS, opt = {}) {
  const h = helpers();
  const gm = opt.reject ? makeRejecting(calls) : makeResponder(raw, calls, 'mapped');
  const env = envelopeFor(h, gm);
  const mod = loadUts(file, {
    get: gm,
    getMapped: gm,
    post: gm,
    postMapped: gm,
    del: gm,
    toNumber: h.toNumber,
    toStr: h.toStr,
    decodePage: env.decodePage,
  });
  return { mod, calls };
}

/** 装载 notification.uts */
function loadNotification(raw, calls = [], file = NOTIFICATION_UTS, opt = {}) {
  const h = helpers();
  const gm = opt.reject ? makeRejecting(calls) : makeResponder(raw, calls, 'mapped');
  const env = envelopeFor(h, gm);
  const mod = loadUts(file, {
    getMapped: gm,
    post: gm,
    toNumber: h.toNumber,
    toStr: h.toStr,
    toBool: h.toBool,
    decodePage: env.decodePage,
  });
  return { mod, calls };
}

/** 装载 search.uts */
function loadSearch(raw, calls = [], file = SEARCH_UTS, opt = {}) {
  const h = helpers();
  const gm = opt.reject ? makeRejecting(calls) : makeResponder(raw, calls, 'mapped');
  const env = envelopeFor(h, gm);
  const mod = loadUts(file, {
    getMapped: gm,
    toNumber: h.toNumber,
    toStr: h.toStr,
    decodePage: env.decodePage,
  });
  return { mod, calls };
}

/** 装载 forumDto.uts（纯构造层，零请求出口） */
function loadForumDto(file = FORUM_DTO_UTS) {
  const h = helpers();
  const env = envelopeFor(h, makeResponder({}, [], 'mapped'));
  return loadUts(file, { ...h, decodePage: env.decodePage });
}

/** 装载 recruit.uts（绑定表与 recruiterResumeBehavior 同源：本域 imports 一项不缺） */
function loadRecruit(raw, calls = [], file = RECRUIT_UTS, opt = {}) {
  const h = helpers();
  const respond = opt.reject ? makeRejecting(calls) : makeResponder(raw, calls, 'bare');
  const env = envelopeFor(h, makeResponder({}, calls, 'mapped'));
  const mod = loadUts(file, {
    get: respond,
    post: respond,
    put: respond,
    decodePage: env.decodePage,
    API_BASE_URL: 'https://example.test/api',
    FORBIDDEN_MESSAGE_PREFIX: '没有权限：',
    toBool: h.toBool,
    toNumber: h.toNumber,
    toNumberOrNull: h.toNumberOrNull,
    toStr: h.toStr,
    errMsg: h.errMsg,
    STORAGE_KEY_TOKEN: 'auth_token',
    getStorage: () => 'recruiter-access-token',
    uni: { showToast: () => {}, downloadFile: () => {}, openDocument: () => {} },
  });
  return { mod, calls };
}

/**
 * 读真源 → 注入变异 → 落到临时目录 → 真执行（不改工作树，不进仓）。
 * 锚点断言先行：替换串不在源里就当场红 —— 变异必须真的进去了，测试才有意义。
 */
function mutateUts(file, replacements, outName) {
  let src = readText(file);
  for (const [from, to] of replacements) {
    expect([from, src.includes(from)]).toEqual([from, true]);
    src = src.split(from).join(to);
  }
  const dir = fs.mkdtempSync(path.join(os.tmpdir(), 'fallback-chain-'));
  const out = path.join(dir, outName);
  fs.writeFileSync(out, src);
  return out;
}

// ===== 夹具（真机同形字段） =====

const COMMENT_A = { id: 11, question_id: 3, user_id: 7, username: '老王', avatar_url: '/a.png', content: '先查油位', created_at: '2026-10-01', can_delete: true };
const COMMENT_B = { id: 12, question_id: 3, user_id: 8, username: '小李', avatar_url: '', content: '再看滤芯', created_at: '2026-10-02', can_delete: false };
const COMMENT_A_EXPECTED = { id: 11, question_id: 3, user_id: 7, username: '老王', avatar_url: '/a.png', content: '先查油位', created_at: '2026-10-01', can_delete: true };

const EXAM_ROW = { id: 21, status: 'completed', duration: 30, remaining_time: 100, result: { total_score: 88, max_score: 100, correct_count: 22, total_questions: 25, accuracy: 88 }, created_at: '2026-10-03' };
const FAV_ROW = { favorite_id: 5, target_type: 'course', target_id: 9, course_id: 0, title: '液压系统', cover: '/c.png', created_at: '2026-10-04' };
const NOTE_ROW = { id: 2, type: 'system', title: 'T', content: 'C', link: '', payload: {}, is_read: false, read_at: '', created_at: '2026-10-05' };
const SEARCH_ROW = { type: 'course', id: 1, title: '叉车基础', cover: '', summary: '', snippet: '', hit_field: '', parent_id: 0 };

// ===== A. 手写回退链：getCommentsApi 的 list ?? items =====

describe('A. getCommentsApi：手写回退链收敛为模块默认键序（PR-B 同口径）', () => {
  it('载荷用 list 键 → 命中（回退链没被迁丢），total 取后端值', async () => {
    const { mod } = loadQuestionInteraction({ list: [COMMENT_A, COMMENT_B], total: 9 });
    const got = await mod.getCommentsApi(3);
    expect(got.items).toEqual([COMMENT_A_EXPECTED, { ...COMMENT_B, avatar_url: '', can_delete: false }]);
    expect(got.total).toBe(9);
  });

  it('载荷只有 items 键 → 同样命中；total 缺失时回落 items.length（旧手写法同值）', async () => {
    const { mod } = loadQuestionInteraction({ items: [COMMENT_A] });
    const got = await mod.getCommentsApi(3);
    expect(got.items.length).toBe(1);
    expect(got.total).toBe(1);
  });

  it('M1 必红：把默认键序改成只认 items → list 载荷解不出', async () => {
    const file = mutateUts(QUESTION_INTERACTION_UTS, [['(obj : UTSJSONObject) : QuestionComment => buildComment(obj))', "(obj : UTSJSONObject) : QuestionComment => buildComment(obj), ['items'])"]], 'questionInteraction.uts');
    const { mod } = loadQuestionInteraction({ list: [COMMENT_A], total: 9 }, [], file);
    const got = await mod.getCommentsApi(3);
    expect(got.items).toEqual([]);
    expect(got.total).toBe(0);
  });
});

// ===== B. 三级回退链 + 吞错删除：mockExam =====

describe('B. mockExam 历史：三级 if-else 收敛为一次 keys 声明，吞错分支已删', () => {
  it('exams 优先于 records（键序即回退序）；page_size 恒等于请求值', async () => {
    const { mod, calls } = loadMockExam({ exams: [EXAM_ROW], records: [EXAM_ROW, EXAM_ROW], total: 7 });
    const got = await mod.getMockExamHistoryApi(2, 5);
    expect(got.items.length).toBe(1);
    expect(got.total).toBe(7);
    expect(got.page).toBe(1); // 载荷无回显页码 → 缺省 1（行为保持点）
    expect(got.page_size).toBe(5); // 请求值，不是响应回显
    expect(calls[0].url).toBe('/mock-exam/history');
    expect(calls[0].params).toEqual({ page: '2', page_size: '5' });
  });

  it('载荷只有 records → 第三级键仍命中；total 缺失回落 items.length', async () => {
    const { mod } = loadMockExam({ records: [EXAM_ROW] });
    const got = await mod.getMockExamHistoryApi();
    expect(got.items.length).toBe(1);
    expect(got.total).toBe(1);
  });

  it('失败原样上抛（吞错空页已随 PR-D 删除，ADR-0037 ④.4）', async () => {
    const { mod } = loadMockExam({}, [], undefined, { reject: true });
    await expect(mod.getMockExamHistoryApi(1, 10)).rejects.toThrow('network down');
  });

  it('M2 必红：keys 改成 records 优先 → 同载荷解出两条', async () => {
    const file = mutateUts(MOCK_EXAM_UTS, [["['exams', 'items', 'records']", "['records', 'items', 'exams']"]], 'mockExam.uts');
    const { mod } = loadMockExam({ exams: [EXAM_ROW], records: [EXAM_ROW, EXAM_ROW], total: 7 }, [], file);
    const got = await mod.getMockExamHistoryApi(2, 5);
    expect(got.items.length).toBe(2);
  });

  it('M3 必红：把吞错 .catch 加回来 → 失败不再上抛', async () => {
    const file = mutateUts(MOCK_EXAM_UTS, [[
      "buildMockExamHistoryResult(data, 'mock-exam/history', page, pageSize))",
      "buildMockExamHistoryResult(data, 'mock-exam/history', page, pageSize)).catch(() => Promise.resolve({ items: [], total: 0, page: page, page_size: pageSize }))",
    ]], 'mockExam.uts');
    const { mod } = loadMockExam({}, [], file, { reject: true });
    await expect(mod.getMockExamHistoryApi(1, 10)).resolves.toEqual({ items: [], total: 0, page: 1, page_size: 10 });
  });
});

// ===== C. 具名键站点：favorite =====

describe('C. favorite：具名键 favorites 显式声明，不因默认键序而放宽', () => {
  it('favorites 命中；page_size 取请求值、page 取回显值', async () => {
    const { mod } = loadFavorite({ favorites: [FAV_ROW], total: 31, page: 4 });
    const got = await mod.getFavoritesApi('course', 4, 7);
    expect(got.items.length).toBe(1);
    expect(got.total).toBe(31);
    expect(got.page).toBe(4);
    expect(got.page_size).toBe(7);
  });

  it('载荷用 items 键 → 解不出（具名键站点不借默认兜底扩面）', async () => {
    const { mod } = loadFavorite({ items: [FAV_ROW], total: 31 });
    const got = await mod.getFavoritesApi();
    expect(got.items).toEqual([]);
    expect(got.total).toBe(0);
  });

  it('M4 必红：keys 放宽为默认 items → 上一用例的载荷反而被解出，等于凭空扩面', async () => {
    const file = mutateUts(FAVORITE_UTS, [["(obj : UTSJSONObject) : FavoriteItem => buildFavoriteItem(obj), ['favorites'])", '(obj : UTSJSONObject) : FavoriteItem => buildFavoriteItem(obj))']], 'favorite.uts');
    const { mod } = loadFavorite({ items: [FAV_ROW], total: 31 }, [], file);
    const got = await mod.getFavoritesApi();
    expect(got.items.length).toBe(1);
  });
});

// ===== D. 侧计数器：notification =====

describe('D. notification：信封三字段之外，unread_count 保留手读', () => {
  it('items/total 走信封，page_size 取请求值，unread_count 逐值不变', async () => {
    const { mod } = loadNotification({ items: [NOTE_ROW], total: 12, page: 2, unread_count: 3 });
    const got = await mod.getNotificationsApi(2, 6);
    expect(got.items.length).toBe(1);
    expect(got.total).toBe(12);
    expect(got.page).toBe(2);
    expect(got.page_size).toBe(6);
    expect(got.unread_count).toBe(3);
  });

  it('载荷没有 unread_count → 手读缺省 0（空页通道不该把它一起吞掉）', async () => {
    const { mod } = loadNotification({ items: [NOTE_ROW], total: 1 });
    const got = await mod.getNotificationsApi();
    expect(got.items.length).toBe(1);
    expect(got.unread_count).toBe(0);
  });

  it('M5 必红：把侧计数器改成常量 → 未读角标静默归零', async () => {
    const file = mutateUts(NOTIFICATION_UTS, [["unread_count: toNumber(data['unread_count']),", 'unread_count: 0,']], 'notification.uts');
    const { mod } = loadNotification({ items: [NOTE_ROW], total: 1, unread_count: 3 }, [], file);
    const got = await mod.getNotificationsApi();
    expect(got.unread_count).toBe(0);
  });
  it('M6 必红：page_size 改读响应回显 → 请求值不再恒等（票面②「结构上不可再表达」的反证）', async () => {
    const file = mutateUts(NOTIFICATION_UTS, [['page_size: pageSize,', "page_size: toNumber(data['page']),"]], 'notification.uts');
    const { mod } = loadNotification({ items: [NOTE_ROW], total: 12, page: 2, page_size: 99 }, [], file);
    const got = await mod.getNotificationsApi(2, 6);
    expect(got.page_size).toBe(2);
  });
});

// ===== E. 四类异形之一：search 五分区 + 分页 pages =====

describe('E. search：五分区各自解码，pages 由 total 折算不读回显', () => {
  it('分区独立：null 分区塌成空、缺 total 的分区回落 items.length', async () => {
    const { mod } = loadSearch({
      courses: { items: [SEARCH_ROW], total: 5 },
      chapters: null,
      questions: { items: [SEARCH_ROW, SEARCH_ROW] },
    });
    const got = await mod.searchAllApi('叉车');
    expect(got.courses.total).toBe(5);
    expect(got.courses.items.length).toBe(1);
    expect(got.chapters).toEqual({ items: [], total: 0 });
    expect(got.questions.total).toBe(2);
    expect(got.contents.items).toEqual([]);
    expect(got.keyword).toBe('叉车'); // 载荷无回显 → 出口注入的关键词落进 DTO
  });

  it('分页搜索：pages = ceil(total / 请求 pageSize)，后端回显更大的数也不作数', async () => {
    const { mod } = loadSearch({ items: [SEARCH_ROW], total: 12, pages: 99, page: 2 });
    const got = await mod.searchByTypeApi('叉车', 'chapter', 2, 5);
    expect(got.total).toBe(12);
    expect(got.pages).toBe(3); // floor((12+5-1)/5)，不是回显的 99
    expect(got.page).toBe(2);
    expect(got.type).toBe('chapter');
  });

  it('M7 必红：pages 改回索引读后端回显 → 现网回显值直接进 DTO', async () => {
    const file = mutateUts(SEARCH_UTS, [["pages: Math.floor((paged.total + pageSize - 1) / pageSize),", "pages: toNumber(data['pages'], 1),"]], 'search.uts');
    const { mod } = loadSearch({ items: [SEARCH_ROW], total: 12, pages: 99, page: 2 }, [], file);
    const got = await mod.searchByTypeApi('叉车', 'chapter', 2, 5);
    expect(got.pages).toBe(99);
  });
});

// ===== F. 四类异形之二：嵌套父列表 forumDto.buildTopicDetail =====

describe('F. buildTopicDetail：replies 与三元组走信封，pages 折算', () => {
  const TOPIC = { id: 7, title: '泵异响', content: '哒哒响', reply_count: 45, view_count: 10 };

  it('详情载荷：replies 命中、total 取后端值、pages 由 total/pageSize 折算', () => {
    const d = loadForumDto();
    const got = d.buildTopicDetail({ topic: TOPIC, replies: [{ id: 31, content: '换密封圈' }], page: 2, pages: 9, total: 45 }, '/forum/topics/7', 2, 20);
    expect(got.replies.length).toBe(1);
    expect(got.total).toBe(45);
    expect(got.pages).toBe(3); // floor((45+19)/20)，不是回显的 9
    expect(got.page).toBe(2);
  });

  it('采纳形态（无 replies / 无三元组）→ 空页 + 0 条 0 页，不退化成「还有更多」', () => {
    const d = loadForumDto();
    const got = d.buildTopicDetail({ topic: TOPIC }, '/forum/topics/7/accept', 1, 20);
    expect(got.replies).toEqual([]);
    expect(got.total).toBe(0);
    expect(got.pages).toBe(0);
    expect(got.page).toBe(1);
  });

  it('M8 必红：数组键写成 items → 详情的 replies 解不出', () => {
    const file = mutateUts(FORUM_DTO_UTS, [["(reply : UTSJSONObject) : ForumReply => buildReply(reply, topicId), ['replies'])", "(reply : UTSJSONObject) : ForumReply => buildReply(reply, topicId), ['items'])"]], 'forumDto.uts');
    const d = loadForumDto(file);
    const got = d.buildTopicDetail({ topic: TOPIC, replies: [{ id: 31, content: '换密封圈' }], total: 45 }, '/forum/topics/7', 1, 20);
    expect(got.replies).toEqual([]);
  });
});

// ===== G. 四类异形之三/四：recruit 侧计数器 + 不回显端点 =====

describe('G. recruit：投递列表保留 unread_count/job_title，简历库不回显 total 时回落', () => {
  it('投递列表：total 走信封、page_size 取请求值、侧计数器逐值不变', async () => {
    const { mod } = loadRecruit({ items: [{ id: 1, job_title: '叉车维修员' }], total: 8, page: 2, unread_count: 3, job_title: '液压专班' });
    const got = await mod.getRecruitJobApplicationsApi(9, 2, 7);
    expect(got.items.length).toBe(1);
    expect(got.total).toBe(8);
    expect(got.page).toBe(2);
    expect(got.page_size).toBe(7);
    expect(got.unread_count).toBe(3);
    expect(got.job_title).toBe('液压专班');
  });

  it('简历库（端点不回显分页字段）：total 缺失回落 items.length，pageSize 与本域常量同源', async () => {
    // filters 的字符串维度在 UTS 里由类型保证非空，JS 侧必须补齐（否则 buildFilterParams 的 .trim() 炸）
    const { mod, calls } = loadRecruit({ items: [{ user_id: 3 }, { user_id: 4 }] });
    const got = await mod.getRecruitResumesApi(mod.emptyRecruitFilters(), 1);
    expect(got.items.length).toBe(2);
    expect(got.total).toBe(2);
    expect(calls[0].params).toEqual({ page: '1', page_size: '20' }); // 与 decodePage 的 RECRUIT_PAGE_SIZE 同源
  });
});
