/**
 * PR-C #1663 · 具名键站点迁移 —— **行为级**守护（ADR-0037 ⑥：真执行 + 成对取证）
 *
 * ## 它在守什么
 *
 * 四个具名键站点（material 的 materials ×2 出口、student 的 records / courses、
 * forumDto 的 topics）从手写信封拆包迁到 `decodePage` / `decodeList`（数组键显式声明）。
 * 本文件把被迁模块**真跑起来**（`utsHarness.loadUts` + 替身 `getMapped` **真的回调 mapper**），
 * 断言只有跑起来才看得见的语义：
 *
 *   1. 具名键真的命中 —— 键改坏 ⇒ 空列表（成对必红变异 M1/M3/M5/M7 证明断言承重）；
 *   2. 分页字段语义 —— `page_size` 恒等于**请求** pageSize（M4/M8 锁死旧误写回潮：
 *      后端回显总页数被塞进 page_size 的那个 bug，票面②「结构上不可再表达」）；
 *      `pages` 不读后端回显、按 total/pageSize 折算（M2）；`page` 缺省 1 行为保持点；
 *   3. 票面验收「forum 回复列表装载行为不变」—— 详情 replies（`buildTopicDetail`，
 *      PR-D 不动）与「我的回复」（`getMyRepliesApi`，PR-A 已迁）两条装载路径逐字段不变；
 *   4. 失败语义 —— `getRecordsApi` / `getStudentCoursesApi` 拒绝 → 向上抛（不吞错；
 *      student 侧 mock 回退已在 PR-F #1666 / #1731 全数退役，本文件不再有 mock 兜底可锁），
 *      `getForumTopicsApi` 拒绝 → errMsg rethrow（不吞错）。
 *
 * 与接线锁的分工：`resourcesContract` / `forumContract` 守源码形状、
 * `envelopeAdoptionContract`（基线 27）按 api/ 全文双向对账三键索引读；
 * 本文件回答 guards.md 三问里「这条接线由哪个行为守护兜底行为」的那一半。
 * 载体路径写成**本文件内的字面量**：`scripts/classify-guards.mjs` 按文件自身文本判分类。
 */
const fs = require('fs');
const os = require('os');
const path = require('path');

const { loadUts, readText } = require('./utsHarness');
const { forumBodyModule } = require('./forumChainHarness');

const API_DIR = path.join(__dirname, '..', 'api');
const HELPERS_UTS = path.join(API_DIR, 'helpers.uts');
const ENVELOPE_UTS = path.join(API_DIR, 'envelope.uts');
const MATERIAL_UTS = path.join(API_DIR, 'material.uts');
const STUDENT_UTS = path.join(API_DIR, 'student.uts');
const FORUM_DTO_UTS = path.join(API_DIR, 'forumDto.uts');
const FORUM_UTS = path.join(API_DIR, 'forum.uts');

/** 真源：helpers 零依赖（手抄 toStr/toNumber 正是漂移起点） */
const helpers = () => loadUts(HELPERS_UTS, {});

/**
 * `getMapped` 替身 —— **真的调用出口传进来的 mapper**。
 * 这是「行为」与「源码文本」的分界：若被测方退化成不传 mapper（或塞个恒等函数），
 * 替身当场抛错 / 把裸响应原样吐回调用方 ⇒ 下面的逐字段断言全红。
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

/** 拒绝替身 —— 走出口的 catch 分支（mock 兑底 / errMsg rethrow 两条语义靠它触发） */
function makeRejectingGetMapped(err, recorder) {
  return (url, params) => {
    recorder.push({ url, params });
    return Promise.reject(err);
  };
}

/** 信封模块：与出口共用同一个 getMapped 替身（`getPaged` 内部走它，真回调 mapper） */
const envelopeFor = (h, gm) => loadUts(ENVELOPE_UTS, { getMapped: gm, toNumber: h.toNumber });

/** 按出口装载 material.uts（imports：getMapped / toNumber,toStr / getPaged —— PR-F #1666 起走信封出口） */
function loadMaterial(raw, calls = [], file = MATERIAL_UTS, opt = {}) {
  const h = helpers();
  const gm = opt.reject ? makeRejectingGetMapped(opt.reject, calls) : makeGetMapped(raw, calls);
  const env = envelopeFor(h, gm);
  const mod = loadUts(file, {
    getMapped: gm,
    toNumber: h.toNumber,
    toStr: h.toStr,
    getPaged: env.getPaged,
  });
  return { mod, calls };
}

/** 按出口装载 student.uts（imports 另有 `get`、decodeList 与 getPaged —— 缺一个 loadUts 就 fail-closed 抛缺绑定） */
function loadStudent(raw, calls = [], file = STUDENT_UTS, opt = {}) {
  const h = helpers();
  const gm = opt.reject ? makeRejectingGetMapped(opt.reject, calls) : makeGetMapped(raw, calls);
  const env = envelopeFor(h, gm);
  const mod = loadUts(file, {
    get: () => Promise.resolve({}),
    getMapped: gm,
    toNumber: h.toNumber,
    toStr: h.toStr,
    toBool: h.toBool,
    decodePage: env.decodePage,
    decodeList: env.decodeList,
    getPaged: env.getPaged,
  });
  return { mod, calls };
}

/** 按出口装载 forum.uts（dto/api 可换临时目录里的变异件 —— 成对取证入口） */
function loadForum(raw, calls = [], files = {}, opt = {}) {
  const h = helpers();
  const gm = opt.reject ? makeRejectingGetMapped(opt.reject, calls) : makeGetMapped(raw, calls);
  const env = envelopeFor(h, gm);
  const dtoMod = loadUts(files.dto || FORUM_DTO_UTS, { ...h, ...env });
  const mod = loadUts(files.api || FORUM_UTS, {
    getPaged: env.getPaged,
    getMapped: gm,
    post: () => Promise.resolve({}),
    del: () => Promise.resolve({}),
    uploadFile: () => Promise.resolve({}),
    postMapped: () => Promise.resolve({}),
    requestMapped: () => Promise.resolve({}),
    toNumber: h.toNumber,
    toStr: h.toStr,
    toBool: h.toBool,
    errMsg: h.errMsg,
    ...dtoMod,
    FORMAT_TEXT: forumBodyModule().FORMAT_TEXT,
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
  const dir = fs.mkdtempSync(path.join(os.tmpdir(), 'named-key-'));
  const out = path.join(dir, outName);
  fs.writeFileSync(out, src);
  return out;
}

// ===== 夹具（真机同形字段；缺字段那条专门走默认值支） =====

const MATERIAL_ONE = {
  file_id: 55,
  chapter_id: 3,
  chapter_title: '液压系统',
  course_id: 9,
  course_name: '叉车底盘',
  file_name: '液压图.pdf',
  file_url: '/files/hydraulic.pdf',
  content_type: 'application/pdf',
  file_size: 20480,
  created_at: '2026-09-01',
};
const MATERIAL_ONE_EXPECTED = { ...MATERIAL_ONE };
const MATERIAL_TWO_EXPECTED = {
  file_id: 7, // file_id 缺席 → id 回退
  chapter_id: 0,
  chapter_title: '',
  course_id: 0,
  course_name: '',
  file_name: '电池保养.docx',
  file_url: '',
  content_type: '',
  file_size: 0,
  created_at: '',
};

/** total 45 / pageSize 10 ⇒ 折算 pages 5；回显 pages 故意给 99（锁「不读后端回显」） */const MATERIAL_RAW = {
  materials: [MATERIAL_ONE, { id: 7, file_name: '电池保养.docx' }],
  total: 45,
  page: 2,
  pages: 99,
};

const RECORD_FULL = {
  record_id: 9,
  course_id: 4,
  course_name: '液压维修进阶',
  chapter_id: 2,
  chapter_title: '泵与阀',
  study_duration: 120,
  study_date: '2026-10-10',
};
const RECORD_BARE_EXPECTED = {
  record_id: 10,
  course_id: 0,
  course_name: '安全操作规程',
  chapter_id: 0,
  chapter_title: '',
  study_duration: 0,
  study_date: '',
};
const RECORDS_RAW = {
  records: [RECORD_FULL, { record_id: 10, course_name: '安全操作规程' }],
  total: 7,
  page: 1,
};

const COURSE_FULL = {
  course_id: 1,
  course_name: '叉车基础操作',
  cover: '/c1.png',
  specialty_id: 2,
  level_id: 1,
  progress: 60,
  completed_chapters: 5,
  total_chapters: 8,
  study_duration: 300,
  last_chapter_id: 3,
  last_chapter_title: '制动系统',
  last_position: 120,
  last_studied_at: '2026-10-12',
};
const COURSE_BARE_EXPECTED = {
  course_id: 2,
  course_name: '日常维护保养',
  cover: '',
  specialty_id: 0,
  level_id: 0,
  progress: 0,
  completed_chapters: 0,
  total_chapters: 0,
  study_duration: 0,
  last_chapter_id: 0,
  last_chapter_title: '',
  last_position: 0,
  last_studied_at: '',
};
/** 本端点响应无 total（后端只回 courses + continue_learning 兄弟键）⇒ 出口走 decodeList */
const COURSES_RAW = {
  courses: [COURSE_FULL, { course_id: 2, course_name: '日常维护保养' }],
  continue_learning: COURSE_FULL,
};

const TOPIC_FULL = {
  id: 15,
  title: '液压不升怎么排查',
  content: '…',
  view_count: 10,
  reply_count: 2,
  created_at: '2026-10-01',
  author: { user_id: 5, username: 'u5', nickname: '老张', avatar_url: '/a.png' },
  can_delete: false,
  scope: 'discussion',
  is_featured: true,
  likes_count: 3,
  liked: true,
  favorited: true,
  ip_province: '广东',
  ip_city: '广州',
  content_format: 'markdown',
};
const TOPIC_FULL_EXPECTED = {
  id: 15,
  title: '液压不升怎么排查',
  content: '…',
  images: [],
  author_id: 5,
  author_name: '老张', // 优先 nickname
  author_avatar: '/a.png',
  replies_count: 2, // 后端字段 reply_count
  views_count: 10, // 后端字段 view_count
  likes_count: 3,
  liked: true,
  favorited: true,
  created_at: '2026-10-01',
  scope: 'discussion',
  status: 'open', // 无采纳 → open（accepted_reply_id 派生）
  is_featured: true,
  is_experience: false,
  ip_province: '广东',
  ip_city: '广州',
  content_format: 'markdown',
};
const TOPIC_BARE_EXPECTED = {
  id: 16,
  title: '二号帖',
  content: '',
  images: [],
  author_id: 0,
  author_name: '',
  author_avatar: '',
  replies_count: 0,
  views_count: 0,
  likes_count: 0,
  liked: false,
  favorited: false,
  created_at: '',
  scope: 'general',
  status: 'open',
  is_featured: false,
  is_experience: false,
  ip_province: '',
  ip_city: '',
  content_format: 'text',
};
/** total 30 / page 3 / 回显 pages 4 —— page_size 断言恒等于请求值（不是 4，更不是 0） */
const TOPICS_RAW = {
  topics: [TOPIC_FULL, { id: 16, title: '二号帖' }],
  total: 30,
  page: 3,
  pages: 4,
};

const DETAIL_REPLY_EXPECTED = {
  id: 101,
  topic_id: 7, // topic_id 缺席 → 详情出口回填 topicId
  topic_title: '', // 详情响应不回帖子标题
  content: '先查泵',
  images: ['/i1.png'],
  author_id: 6,
  author_name: '李四',
  author_avatar: '/c.png',
  created_at: '2026-10-02',
  can_delete: false,
  likes_count: 3,
  liked: true,
  is_accepted: true,
  parent_name: '楼主',
  parent_avatar_url: '/b.png',
  ip_province: '广东',
  ip_city: '广州',
  content_format: 'markdown',
};
const DETAIL_RAW = {
  page: 1,
  pages: 1,
  total: 2,
  topic: {
    id: 7,
    title: '泵异响',
    content: '哒哒响',
    view_count: 10,
    reply_count: 2,
    created_at: '2026-10-01',
    author: { user_id: 5, username: 'u5', nickname: '老张', avatar_url: '/a.png' },
    can_delete: false,
    accepted_reply_id: 0,
  },
  replies: [
    {
      id: 101,
      content: '先查泵',
      created_at: '2026-10-02',
      parent_name: '楼主',
      parent_avatar_url: '/b.png',
      author: { user_id: 6, username: 'u6', nickname: '李四', avatar_url: '/c.png' },
      likes_count: 3,
      liked: true,
      is_accepted: true,
      content_format: 'markdown',
      images: ['/i1.png'],
      ip_province: '广东',
      ip_city: '广州',
    },
  ],
};

const MY_REPLY_EXPECTED = {
  id: 201,
  topic_id: 8,
  topic_title: '保养周期',
  content: '六个月一次',
  images: [],
  author_id: 5,
  author_name: '老张',
  author_avatar: '/a.png',
  created_at: '2026-10-03',
  can_delete: false,
  likes_count: 0,
  liked: false,
  is_accepted: false,
  parent_name: '',
  parent_avatar_url: '',
  ip_province: '',
  ip_city: '',
  content_format: 'text',
};
const MY_REPLIES_RAW = {
  replies: [
    {
      id: 201,
      topic_id: 8,
      topic_title: '保养周期',
      content: '六个月一次',
      created_at: '2026-10-03',
      author: { user_id: 5, username: 'u5', nickname: '老张', avatar_url: '/a.png' },
      likes_count: 0,
    },
  ],
  total: 4,
  page: 1,
};

// ===== 站点 1：material（materials 具名键，两出口共用 builder） =====

describe('material.uts：materials 具名键站点（PR-C #1663 → PR-F #1666 改 getPaged，真执行）', () => {
  it('列表：数组键命中、total 走信封、hasMore 由 total 折算、请求形态不变', async () => {
    const { mod, calls } = loadMaterial(MATERIAL_RAW);
    const got = await mod.getMaterialsApi(0, 2, 10);

    expect(calls).toEqual([{ url: '/materials', params: { page: '2', page_size: '10' } }]);
    expect(got).toEqual({
      items: [MATERIAL_ONE_EXPECTED, MATERIAL_TWO_EXPECTED],
      total: 45,
      hasMore: true, // 2*10 < 45
    });
  });

  it('列表：末页 hasMore=false（5*10 ≥ 45）—— 到底判据由 total 折算，不读回显 pages=99', async () => {
    const { mod } = loadMaterial(MATERIAL_RAW);
    const got = await mod.getMaterialsApi(0, 5, 10);
    expect(got.hasMore).toBe(false);
  });

  it('列表：courseId>0 才下发 course_id（请求形态逐字不变）', async () => {
    const { mod, calls } = loadMaterial(MATERIAL_RAW);
    await mod.getMaterialsApi(3, 1, 20);
    expect(calls[0].params).toEqual({ page: '1', page_size: '20', course_id: '3' });
  });

  it('列表：响应缺 total → total 回落条数、hasMore 按条数兜底', async () => {
    const { mod } = loadMaterial({ materials: [{ id: 1, file_name: 'a.pdf' }] });
    const got = await mod.getMaterialsApi(0, 1, 10);
    expect(got.total).toBe(1);
    expect(got.hasMore).toBe(false); // 1 != pageSize 10
    expect(got.items[0].file_id).toBe(1); // id 回退
  });

  it('学员资料别名出口：同一具名键，URL 与 params 不变', async () => {
    const { mod, calls } = loadMaterial(MATERIAL_RAW);
    const got = await mod.getStudentMaterialsApi(3, 10);
    expect(calls).toEqual([{ url: '/student/materials', params: { page: '3', page_size: '10' } }]);
    expect(got.total).toBe(45);
    expect(got.items).toHaveLength(2);
  });

  it('必红 M1：数组键改坏（materials→items）⇒ 空列表 + total 归零 —— 具名键断言承重', async () => {
    const file = mutateUts(MATERIAL_UTS, [["['materials']", "['items']"]], 'material.uts');
    const { mod } = loadMaterial(MATERIAL_RAW, [], file);
    const got = await mod.getMaterialsApi(0, 2, 10);
    expect(got.items).toEqual([]); // 绿用例的逐字段 toEqual 在此必红
    expect(got.total).toBe(0);
  });
});

// ===== 站点 2/3：student（records / courses 具名键） =====

describe('student.uts：records / courses 具名键站点（PR-C #1663，真执行）', () => {
  it('学习记录：records 键命中、total 走信封、返回 Page<StudyRecord>（PR-F #1666 页面侧重命名）', async () => {
    const { mod, calls } = loadStudent(RECORDS_RAW);
    const got = await mod.getRecordsApi(2, 15);

    expect(calls).toEqual([{ url: '/student/records', params: { page: '2', page_size: '15' } }]);
    expect(got).toEqual({
      items: [RECORD_FULL, RECORD_BARE_EXPECTED],
      total: 7,
      hasMore: false, // 2*15 = 30 ≥ 7
    });
  });

  it('学习记录：日期区间空串不下发、非空逐字下发', async () => {
    const { mod, calls } = loadStudent(RECORDS_RAW);
    await mod.getRecordsApi(1, 10);
    await mod.getRecordsApi(1, 10, '2026-10-01', '2026-10-15');
    expect(calls[0].params).toEqual({ page: '1', page_size: '10' });
    expect(calls[1].params).toEqual({
      page: '1',
      page_size: '10',
      start_date: '2026-10-01',
      end_date: '2026-10-15',
    });
  });

  it('学习记录：请求拒绝 → 向上抛（PR-F #1666 删掉 mock 回填，吞错点清零）', async () => {
    const { mod } = loadStudent(null, [], STUDENT_UTS, { reject: new Error('网络断了') });
    await expect(mod.getRecordsApi(1, 20)).rejects.toThrow('获取学习记录失败');
  });

  it('我的课程：courses 键命中（decodeList，本端点无 total）、continue_learning 兄弟键手读保留', async () => {
    const { mod, calls } = loadStudent(COURSES_RAW);
    const got = await mod.getStudentCoursesApi();

    expect(calls).toEqual([{ url: '/student/courses', params: null }]);
    expect(got.courses).toEqual([COURSE_FULL, COURSE_BARE_EXPECTED]);
    expect(got.continue_learning).toEqual(COURSE_FULL);
  });

  it('我的课程：continue_learning 缺席 → null（不炸首屏）', async () => {
    const { mod } = loadStudent({ courses: [] });
    const got = await mod.getStudentCoursesApi();
    expect(got.courses).toEqual([]);
    expect(got.continue_learning).toBeNull();
  });

  it('必红 M3：records 键改坏 ⇒ 空页', async () => {
    const file = mutateUts(STUDENT_UTS, [["['records']", "['items']"]], 'student.uts');
    const { mod } = loadStudent(RECORDS_RAW, [], file);
    const got = await mod.getRecordsApi(2, 15);
    expect(got.items).toEqual([]);
    expect(got.total).toBe(0);
  });

  it('必红 M5：courses 键改坏 ⇒ 空列表', async () => {
    const file = mutateUts(STUDENT_UTS, [["['courses']", "['items']"]], 'student.uts');
    const { mod } = loadStudent(COURSES_RAW, [], file);
    const got = await mod.getStudentCoursesApi();
    expect(got.courses).toEqual([]);
  });

  it('必红 M6：把 rethrow 改回返回空页 ⇒ 拒绝被吞（绿用例断言的是向上抛）', async () => {
    const file = mutateUts(
      STUDENT_UTS,
      [['throw new Error(msg)', "return { items: [] as StudyRecord[], total: 0, hasMore: false } as Page<StudyRecord>"]],
      'student.uts'
    );
    const { mod } = loadStudent(null, [], file, { reject: new Error('网络断了') });
    const got = await mod.getRecordsApi(1, 20);
    expect(got).toEqual({ items: [], total: 0, hasMore: false });
  });
});

// ===== 站点 4：forumDto topics（票面②：pages→page_size 误写随迁移消亡） =====

describe('forum：topics 具名键站点 + 回显页数不参与判据（PR-C #1663 / PR-F #1666，真执行）', () => {
  it('帖子列表：topics 键命中、hasMore 由 total 折算（回显 pages=4 塞不进来）', async () => {
    const { mod, calls } = loadForum(TOPICS_RAW);
    const got = await mod.getForumTopicsApi(2, 5, 'general', 'latest', 'desc', 'question', 'resolved', '', '');

    expect(calls).toEqual([
      {
        url: '/forum/topics',
        params: { scope: 'general', sort: 'latest', order: 'desc', page: '2', page_size: '5', category: 'question', solved: 'solved' },
      },
    ]);
    expect(got).toEqual({
      items: [TOPIC_FULL_EXPECTED, TOPIC_BARE_EXPECTED],
      total: 30,
      hasMore: true, // 2*5 < 30
    });
  });

  it('我的帖子：同一 builder、params 仅 page/page_size、末页 hasMore=false', async () => {
    const { mod, calls } = loadForum(TOPICS_RAW);
    const got = await mod.getMyTopicsApi(3, 10);
    expect(calls).toEqual([{ url: '/forum/my-topics', params: { page: '3', page_size: '10' } }]);
    expect(got.total).toBe(30);
    expect(got.hasMore).toBe(false); // 3*10 ≥ 30；回显 pages=4 若被读会误判「还有更多」
    expect(got.items).toHaveLength(2);
  });

  it('帖子列表：请求拒绝 → errMsg rethrow（不吞错，交页面 catch→toast）', async () => {
    const { mod } = loadForum(null, [], {}, { reject: new Error('boom') });
    await expect(mod.getForumTopicsApi()).rejects.toThrow();
  });

  it('必红 M7：topics 键改坏 ⇒ 空列表', async () => {
    const dtoFile = mutateUts(FORUM_DTO_UTS, [["['topics']", "['items']"]], 'forumDto.uts');
    const { mod } = loadForum(TOPICS_RAW, [], { dto: dtoFile });
    const got = await mod.getMyTopicsApi(3, 10);
    expect(got.items).toEqual([]);
  });

  it('必红 M8：hasMore 改回读后端回显页数 ⇒ 末页误判「还有更多」', async () => {
    const dtoFile = mutateUts(
      FORUM_DTO_UTS,
      [['hasMore: paged.hasMore,', "hasMore: toNumber(obj['pages'], 0) > page,"]],
      'forumDto.uts'
    );
    const { mod } = loadForum(TOPICS_RAW, [], { dto: dtoFile });
    const got = await mod.getMyTopicsApi(3, 10);
    expect(got.hasMore).toBe(true); // 回显 pages=4 > 3；绿用例断言的是 false
  });
});

// ===== 票面验收：forum 回复列表装载行为不变（两条装载路径逐字段不变） =====

describe('forum 回复列表装载行为不变（票面验收，列表 PR-C #1663 / 详情回复 PR-D #1664）', () => {
  it('详情回复：topic 子对象 + 顶层 replies + 分页三元组逐字段不变（PR-D 迁 decodePage 后同解）', async () => {
    const { mod, calls } = loadForum(DETAIL_RAW);
    const got = await mod.getForumTopicDetailApi(7, 'latest', 'asc', 1, 20);

    expect(calls).toEqual([
      { url: '/forum/topics/7', params: { sort: 'latest', order: 'asc', page: '1', page_size: '20' } },
    ]);
    expect(got).toEqual({
      id: 7,
      title: '泵异响',
      content: '哒哒响',
      images: [],
      author_id: 5,
      author_name: '老张',
      author_avatar: '/a.png',
      replies_count: 2,
      views_count: 10,
      likes_count: 0, // 无 likes_count/like_count → 0
      liked: false,
      favorited: false,
      created_at: '2026-10-01',
      can_delete: false,
      replies: [DETAIL_REPLY_EXPECTED],
      hasMore: false, // 1*20 ≥ total 2；PR-F #1666 起 page/pages 换成 hasMore
      total: 2,
      scope: 'general',
      category: 'discussion',
      is_experience: false,
      status: 'open',
      accepted_reply_id: null,
      ip_province: '',
      ip_city: '',
      content_format: 'text',
    });
  });

  it('我的回复：PR-A getPaged 形态装载不变（Page 三字段 + replies 键）', async () => {
    const { mod, calls } = loadForum(MY_REPLIES_RAW);
    const got = await mod.getMyRepliesApi(1, 2);

    expect(calls).toEqual([{ url: '/forum/my-replies', params: { page: '1', page_size: '2' } }]);
    expect(got).toEqual({ items: [MY_REPLY_EXPECTED], total: 4, hasMore: true });
  });
});
