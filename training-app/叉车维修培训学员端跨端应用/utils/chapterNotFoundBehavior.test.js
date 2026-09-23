/**
 * #1268 付费章节「未解锁」语义 —— **行为级**守护（真跑 `api/request.uts` → `api/course.uts`）
 *
 * 病根（票面，跨端契约同步 #1265 第 6 条 / 第十四波 ADR-0062）：未兑换的付费章节，后端
 * `GET /course/:id/chapter/:id` 一律按「章节不存在」**404**（鉴权收回、不泄漏存在性）；
 * 而 `getChapterDetailApi` 的 `.catch` 把**所有**错误折成 `chapter_id: 0` 的空详情，
 * `pages/courses/chapter-view.uvue` 据此渲染「章节内容加载失败 **+ 重试**」⇒ 对未解锁是**误报**，
 * 且那个重试按钮必然再拿一次 404（硬死胡同）。
 *
 * 本文件钉住的不变量：
 *   I1 404 **不再被吞成空详情**：真实调用链上 `getChapterDetailApi` 必须 **reject**，
 *      且 reject 出来的错误可被 `isChapterNotFound()` 认出（前缀读写两端同值）。
 *   I2 只有 404 算「未解锁」：500 / 业务码错误 / 网络失败 / 403 / 非 Error **一律判 false**
 *      —— 否则真失败会被渲染成「需先解锁」（假引导）。
 *   I3 成功响应仍走手动映射（删掉整段 catch 没伤 happy path）。
 *
 * 为什么不用源码文本断言：`expect(src).toContain('isChapterNotFound')` 回答不了
 * 「404 到底是 resolve 还是 reject」—— 而**吞成空详情**正是本票要防的那件事，
 * 它在源码文本断言下可以照样绿（`.catch` 里留一句字面量就行）。判据三条见 `docs/agents/guards.md`。
 *
 * 分类（docs/agents/guards.md）：
 *   - **行为守护（承重）** = describe A（真实链路的 reject / resolve 与判定）+ describe C（注入变异源码的必红侧）。
 *   - **接线守护** = describe B（读 `.uvue` 源文本：分支顺序、无重试按钮、判定只有一个出口）。
 *     它不构成 ③ 门证据，其**行为兜底** = A1/C3（页面那一支只是 `chapterLocked` 的渲染，
 *     判据本身跑在 `.uts` 侧）。`.uvue` 的模板/样式在 node 里跑不了 ⇒ 页面渲染另靠 ①a 真机逐页截图。
 */
const fs = require('fs');
const os = require('os');
const path = require('path');
const { loadUts, readText } = require('./utsHarness');

const ROOT = path.join(__dirname, '..');
const REQUEST_UTS = path.join(ROOT, 'api', 'request.uts');
const COURSE_UTS = path.join(ROOT, 'api', 'course.uts');
const HELPERS_UTS = path.join(ROOT, 'api', 'helpers.uts');
const GATE_UTS = path.join(ROOT, 'api', 'refreshGate.uts');
const STORAGE_UTS = path.join(ROOT, 'utils', 'storage.uts');
const AUTH_ROLE_UTS = path.join(ROOT, 'utils', 'authRole.uts');
const CHAPTER_VIEW_UVUE = path.join(ROOT, 'pages', 'courses', 'chapter-view.uvue');

const KEY_TOKEN = 'auth_token';
const KEY_USER = 'auth_user';
const KEY_ACTIVE_ROLE = 'auth_active_role';
const ROLE_STUDENT = 'student';
const ROLE_RECRUITER = 'recruiter';

/** 后端对未兑换付费章节的实际形态（404 + 「不泄漏存在性」的文案，与真章节不存在同形） */
const NOT_FOUND_REPLY = { statusCode: 404, data: { code: 404, message: '章节不存在' } };
const SERVER_ERROR_REPLY = { statusCode: 500, data: { code: 500, message: '服务器内部错误' } };
const FORBIDDEN_REPLY = { statusCode: 403, data: { code: 403, message: '无权限访问' } };
const OK_REPLY = {
  statusCode: 200,
  data: {
    code: 200,
    message: 'ok',
    data: {
      chapter_id: 3,
      title: '液压系统概述',
      content: '正文',
      files: [{ file_id: 11, file_name: '图册', file_url: 'https://x/1.pdf' }],
      previous_chapter_id: 2,
      next_chapter_id: 4,
      study_status: 'completed',
      duration: 12,
    },
  },
};

/** 一次 tick：让 `uni.request` 的 success/fail 回调与 Promise 链跑完 */
const tick = () => new Promise((resolve) => setTimeout(resolve, 0));

/** 假 `uni`：storage 落 Map（语义同 `utils/storage.uts`），request 只**捕获**不自动回应 */
function makeUni() {
  const kv = new Map();
  const requests = [];
  const toasts = [];
  return {
    kv,
    requests,
    toasts,
    setStorageSync: (k, v) => { kv.set(k, v); },
    getStorageSync: (k) => (kv.has(k) ? kv.get(k) : ''),
    removeStorageSync: (k) => { kv.delete(k); },
    clearStorageSync: () => { kv.clear(); },
    getStorageInfoSync: () => ({ currentSize: 0 }),
    showToast: (o) => { toasts.push(o); },
    showLoading: () => {},
    hideLoading: () => {},
    reLaunch: () => {},
    request: (o) => { requests.push(o); },
    getSystemInfoSync: () => ({ statusBarHeight: 20, windowHeight: 800 }),
  };
}

/** 把变异后的源码写到临时目录再交给 harness（真源文件**一个字节都不动**） */
function loadUtsFromSource(source, fileName, bindings) {
  const dir = fs.mkdtempSync(path.join(os.tmpdir(), 'fl-1268-'));
  const file = path.join(dir, fileName);
  fs.writeFileSync(file, source);
  try {
    return loadUts(file, bindings);
  } finally {
    fs.rmSync(dir, { recursive: true, force: true });
  }
}

/** 单点字面量替换；锚点不唯一即抛（否则变异静默落空 ⇒「必红」用例变成假绿） */
function mutateOnce(src, from, to) {
  const parts = src.split(from);
  if (parts.length !== 2) {
    throw new Error(`变异锚点必须唯一：${JSON.stringify(from)} 命中 ${parts.length - 1} 次`);
  }
  return parts.join(to);
}

/**
 * 装配一棵**真实**的依赖树：`helpers.uts` / `refreshGate.uts` / `storage.uts` / `authRole.uts`
 * → `request.uts` → `course.uts`（可选：用变异源码替换其中任一层的真源）。
 * @param {{ requestSource?: string, courseSource?: string }} [opts]
 */
function buildApp(opts = {}) {
  const uni = makeUni();
  const pages = [{ route: 'pages/courses/chapter-view' }];
  const helpers = loadUts(HELPERS_UTS, {});
  const storage = loadUts(STORAGE_UTS, { uni });
  const gate = loadUts(GATE_UTS, {});
  const authRole = loadUts(AUTH_ROLE_UTS, {
    getStorage: storage.getStorage,
    setStorage: storage.setStorage,
    removeStorage: storage.removeStorage,
    STORAGE_KEY_ACTIVE_ROLE: KEY_ACTIVE_ROLE,
    ACTIVE_ROLE_STUDENT: ROLE_STUDENT,
    ACTIVE_ROLE_RECRUITER: ROLE_RECRUITER,
  });

  const requestBindings = {
    gateRefresh: gate.gateRefresh,
    API_BASE_URL: 'https://example.test/api',
    REQUEST_TIMEOUT: 15000,
    ENABLE_DEBUG_LOG: false,
    STORAGE_KEY_TOKEN: KEY_TOKEN,
    STORAGE_KEY_USER: KEY_USER,
    isRecruiterActive: authRole.isRecruiterActive,
    getStorage: storage.getStorage,
    removeStorage: storage.removeStorage,
    isContentUri: () => false,
    buildTempFilePath: () => '',
    UPLOAD_TMP_DIR: 'upload-tmp',
    uni,
    getCurrentPages: () => pages,
  };
  const request = opts.requestSource == null
    ? loadUts(REQUEST_UTS, requestBindings)
    : loadUtsFromSource(opts.requestSource, 'request.uts', requestBindings);

  const courseBindings = {
    getMapped: request.getMapped,
    post: request.post,
    toNumber: helpers.toNumber,
    toStr: helpers.toStr,
    errMsg: helpers.errMsg,
    // ⚠️ 读写两端共用**同一个真源常量**：从 request 模块回读，不抄字面量
    NOT_FOUND_MESSAGE_PREFIX: request.NOT_FOUND_MESSAGE_PREFIX,
  };
  const course = opts.courseSource == null
    ? loadUts(COURSE_UTS, courseBindings)
    : loadUtsFromSource(opts.courseSource, 'course.uts', courseBindings);

  /** 回应全部挂起请求；返回回应条数 */
  function respondAll(reply) {
    const pending = uni.requests.splice(0, uni.requests.length);
    pending.forEach((o) => {
      if (reply.fail) o.fail(reply.fail);
      else o.success({ statusCode: reply.statusCode, data: reply.data });
    });
    return pending.length;
  }

  return { uni, request, course, helpers, respondAll };
}

describe('A. 真实链路：404 不被吞成空详情，且判定只认 404（api/request.uts → api/course.uts）', () => {
  test('A1（I1）：404 ⇒ reject（不是 chapter_id:0 的空详情），且 isChapterNotFound 为真', async () => {
    const app = buildApp();
    const pending = app.course.getChapterDetailApi(7, 3);
    await tick();
    expect(app.uni.requests.length).toBe(1);
    expect(app.uni.requests[0].url).toBe('https://example.test/api/course/7/chapter/3');

    app.respondAll(NOT_FOUND_REPLY);
    const settled = await pending.then((d) => ({ ok: true, d }), (e) => ({ ok: false, e }));
    // 术前行为：`.catch` 折成空详情 ⇒ 这里会 resolve 且 `chapter_id == 0`。必须 reject。
    expect(settled.ok).toBe(false);
    expect(settled.d).toBeUndefined();
    // 判定出口：前缀是**读端常量**（两端同值才成立），后端文案仍在消息里（没被吞成空串）
    expect(app.request.NOT_FOUND_MESSAGE_PREFIX).toBe('404:');
    expect(app.course.isChapterNotFound(settled.e)).toBe(true);
    expect(settled.e.message).toBe(app.request.NOT_FOUND_MESSAGE_PREFIX + '章节不存在');
  });

  test('A2（I3）：200 ⇒ 仍走手动映射，且成功响应不会被判成未解锁', async () => {
    const app = buildApp();
    const pending = app.course.getChapterDetailApi(7, 3);
    await tick();
    app.respondAll(OK_REPLY);
    const dto = await pending;
    expect(dto.chapter_id).toBe(3);
    expect(dto.title).toBe('液压系统概述');
    expect(dto.study_status).toBe('completed');
    expect(dto.next_chapter_id).toBe(4);
    expect(dto.files.length).toBe(1);
    expect(dto.files[0].file_url).toBe('https://x/1.pdf');
    expect(app.course.isChapterNotFound(null)).toBe(false);
  });

  test('A3（I2）：500 ⇒ reject 且**不是**未解锁（否则真失败会渲染成「需先解锁」）', async () => {
    const app = buildApp();
    const pending = app.course.getChapterDetailApi(7, 3);
    await tick();
    app.respondAll(SERVER_ERROR_REPLY);
    const err = await pending.then(() => null, (e) => e);
    expect(app.course.isChapterNotFound(err)).toBe(false);
    expect(err.message).toBe('服务器内部错误');
  });

  test('A4（I2）：网络失败 ⇒ reject 且不是未解锁', async () => {
    const app = buildApp();
    const pending = app.course.getChapterDetailApi(7, 3);
    await tick();
    app.respondAll({ fail: { errMsg: 'request:fail timeout' } });
    const err = await pending.then(() => null, (e) => e);
    expect(app.course.isChapterNotFound(err)).toBe(false);
  });

  test('A5（I2）：HTTP 200 + 业务码错误 ⇒ 不是未解锁', async () => {
    const app = buildApp();
    const pending = app.course.getChapterDetailApi(7, 3);
    await tick();
    app.respondAll({ statusCode: 200, data: { code: 500, message: '课程服务异常' } });
    const err = await pending.then(() => null, (e) => e);
    expect(app.course.isChapterNotFound(err)).toBe(false);
    expect(err.message).toBe('课程服务异常');
  });

  test('A6（I2）：403 前缀不被未解锁判定吃掉（两个出口互不串味）', async () => {
    const app = buildApp();
    const pending = app.course.getChapterDetailApi(7, 3);
    await tick();
    app.respondAll(FORBIDDEN_REPLY);
    const err = await pending.then(() => null, (e) => e);
    expect(app.course.isChapterNotFound(err)).toBe(false);
    expect(err.message.startsWith(app.request.FORBIDDEN_MESSAGE_PREFIX)).toBe(true);
  });

  test('A7（I2）：非 Error / 空值 / 前缀不在开头 ⇒ fail-safe 判 false', () => {
    const app = buildApp();
    expect(app.course.isChapterNotFound(null)).toBe(false);
    expect(app.course.isChapterNotFound({ kind: 'notfound' })).toBe(false);
    expect(app.course.isChapterNotFound('404:章节不存在')).toBe(false);
    expect(app.course.isChapterNotFound(new Error(''))).toBe(false);
    // 术前形态之一：按后端中文文案匹配 ⇒ 文案一改就静默失效；这里只认前缀
    expect(app.course.isChapterNotFound(new Error('章节不存在'))).toBe(false);
    expect(app.course.isChapterNotFound(new Error('请求失败：404:章节不存在'))).toBe(false);
  });
});

describe('B. 页面接线（接线守护，行为兜底 = A1/C3）：未解锁支存在、且不给重试入口', () => {
  test('B1：未解锁支在「加载失败」支**之前**，文案是「需先解锁」，且该支内没有重试按钮', () => {
    const src = readText(CHAPTER_VIEW_UVUE);
    const locked = /<view v-else-if="chapterLocked"[\s\S]*?<\/view>/.exec(src);
    expect(locked).not.toBeNull();
    expect(locked[0]).toContain('该章节需先解锁');
    // 死胡同的判据就在这一条：未解锁支里出现 retry-btn ⇒ 渲染成「重试」，票面要防的原样复现
    expect(locked[0]).not.toContain('retry-btn');
    expect(src.indexOf('v-else-if="chapterLocked"')).toBeLessThan(src.indexOf('章节内容加载失败'));
  });

  test('B2：判定只有一个出口（来自 api/course），且每次 loadDetail 归零，防切章带着上一章的判定', () => {
    const src = readText(CHAPTER_VIEW_UVUE);
    expect(src).toContain("import { getCourseDetailApi, getChapterDetailApi, isChapterNotFound } from '../../api/course'");
    expect(src).toContain('chapterLocked.value = isChapterNotFound(e)');
    expect(src).toContain('chapterLocked.value = false');
    // 页面不得自己复制前缀判据（#1204 病根：两端各写一份判据）
    expect(src).not.toContain('NOT_FOUND_MESSAGE_PREFIX');
    expect(src).not.toContain("'404");
  });
});

describe('C. 判别力自检：变异真源必须让 A 组变红（成对取证的另一半）', () => {
  test('C1：摘掉 request.uts 404 出口的前缀（退回老形态）⇒ 404 不再被认成未解锁', async () => {
    const mutated = mutateOnce(
      readText(REQUEST_UTS),
      'reject(new Error(NOT_FOUND_MESSAGE_PREFIX + notFoundMsg))',
      'reject(new Error(notFoundMsg))',
    );
    const app = buildApp({ requestSource: mutated });
    const pending = app.course.getChapterDetailApi(7, 3);
    await tick();
    app.respondAll(NOT_FOUND_REPLY);
    const err = await pending.then(() => null, (e) => e);
    expect(app.course.isChapterNotFound(err)).toBe(false);
  });

  test('C2：断掉 course.uts 读端（isChapterNotFound 恒 false）⇒ 404 也判不出来', async () => {
    const mutated = mutateOnce(
      readText(COURSE_UTS),
      "\treturn errMsg(e, '').startsWith(NOT_FOUND_MESSAGE_PREFIX)",
      '\treturn false',
    );
    const app = buildApp({ courseSource: mutated });
    const pending = app.course.getChapterDetailApi(7, 3);
    await tick();
    app.respondAll(NOT_FOUND_REPLY);
    const err = await pending.then(() => null, (e) => e);
    expect(err.message.startsWith(app.request.NOT_FOUND_MESSAGE_PREFIX)).toBe(true);
    expect(app.course.isChapterNotFound(err)).toBe(false);
  });

  test('C3：把旧的「吞成空详情」catch 装回去 ⇒ 404 又变成 resolve(chapter_id:0)（A1 的反面 = 术前行为）', async () => {
    const mutated = mutateOnce(
      readText(COURSE_UTS),
      '\t\treturn buildChapterDetail(data)\n\t})\n}',
      '\t\treturn buildChapterDetail(data)\n\t}).catch(() : ChapterDetail => {\n\t\treturn { chapter_id: 0, title: \'\', content: \'\', files: [], previous_chapter_id: 0, next_chapter_id: 0, study_status: \'\', duration: 0 } as ChapterDetail\n\t})\n}',
    );
    const app = buildApp({ courseSource: mutated });
    const pending = app.course.getChapterDetailApi(7, 3);
    await tick();
    app.respondAll(NOT_FOUND_REPLY);
    const settled = await pending.then((d) => ({ ok: true, d }), (e) => ({ ok: false, e }));
    // 行为面变红：调用方拿到的是「成功的空详情」，页面只能显示「加载失败 + 重试」
    expect(settled.ok).toBe(true);
    expect(settled.d.chapter_id).toBe(0);
  });
});
