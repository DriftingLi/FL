/**
 * #1944 浏览课程上报 —— **行为级**守护（真跑 api/transport.uts → api/request.uts → api/course.uts）
 *
 * 契约（后端票 #2065 定稿，2026-10-28 同步给移动端的原文）：点开课程详情那一刻
 * `POST /api/course/{course_id}/browse`，无请求体，后端按（学员, 课程, 当日）去重。
 * 三件事只在真实链路上跑得出来，源码文本断言一条都答不了：
 *   I1 请求形态：POST + url 落在 /course/<id>/browse（course_id 走路径，不进 body）。
 *   I2 幂等回包**不是失败**：`recorded: false` 必须 resolve(false) 且端上**不再发第二个请求**
 *      —— 「当失败重试」正是票面点名禁止的那件事，它在文本断言下可以照样绿。
 *   I3 上报失败不得折成成功：404 一律 reject（课程未发布 / 未挂载），但**不弹 toast**
 *      （silent=true：旁路上报不该给正在浏览课程的用户制造一次他没触发的报错，
 *      口径同移动端 ADR-0031 决策 3 的余额预读）。
 *
 * 分类（docs/agents/guards.md）：
 *   - **行为守护（承重）** = describe A（真实链路的请求形态与 resolve/reject）+ describe C（变异必红侧）。
 *   - 接线侧（页面只在 onLoad 发一次、任务中心图标与落点）落在 coursesContract.test.js 与
 *     pointsRealApiContract.test.js，**不构成 ③ 门证据**，其兜底 = 本文件 A1/A2/C1 + ② 门。
 *
 * 装配同 utils/chapterNotFoundBehavior.test.js（#1268 先例）：真源一个字节不动，变异侧写临时文件。
 */
const fs = require('fs');
const os = require('os');
const path = require('path');
const { loadUts, readText } = require('./utsHarness');
const { transport, transportAliases, envelopeBindings } = require('./requestUtsHarness');

jest.setTimeout(120000);

const ROOT = path.join(__dirname, '..');
const REQUEST_UTS = path.join(ROOT, 'api', 'request.uts');
const COURSE_UTS = path.join(ROOT, 'api', 'course.uts');
const HELPERS_UTS = path.join(ROOT, 'api', 'helpers.uts');
const GATE_UTS = path.join(ROOT, 'api', 'refreshGate.uts');
const STORAGE_UTS = path.join(ROOT, 'utils', 'storage.uts');
const AUTH_ROLE_UTS = path.join(ROOT, 'utils', 'authRole.uts');

const KEY_TOKEN = 'auth_token';
const KEY_USER = 'auth_user';
const KEY_ACTIVE_ROLE = 'auth_active_role';
const BASE_URL = 'https://example.test/api';

/** 后端三种真实回包形态（#2065 定稿评论逐字对应） */
const RECORDED_TRUE = { statusCode: 200, data: { code: 200, message: 'ok', data: { recorded: true } } };
const RECORDED_FALSE = { statusCode: 200, data: { code: 200, message: 'ok', data: { recorded: false } } };
const NOT_FOUND_REPLY = { statusCode: 404, data: { code: 404, message: 'not found' } };

/** 一次 tick：让 uni.request 的 success/fail 回调与 Promise 链跑完 */
const tick = () => new Promise((resolve) => setTimeout(resolve, 0));

/** 假 uni：request 只**捕获**不自动回应；showToast 落数组（silent 判据要看它） */
function makeUni() {
  const kv = new Map([[KEY_TOKEN, 'test-token'], [KEY_USER, JSON.stringify({ role: 'student' })], [KEY_ACTIVE_ROLE, 'student']]);
  const requests = [];
  const toasts = [];
  return {
    kv, requests, toasts,
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

/** 把变异后的源码写到临时目录再交给 harness（真源不动） */
function loadUtsFromSource(source, fileName, bindings) {
  const dir = fs.mkdtempSync(path.join(os.tmpdir(), 'fl-1944-'));
  const file = path.join(dir, fileName);
  fs.writeFileSync(file, source);
  try {
    return loadUts(file, bindings);
  } finally {
    fs.rmSync(dir, { recursive: true, force: true });
  }
}

/** 单点字面量替换；锚点不唯一即抛（否则变异静默落空 ⇒「必红」用例变假绿） */
function mutateOnce(src, from, to) {
  const parts = src.split(from);
  if (parts.length !== 2) throw new Error(`变异锚点必须唯一：${JSON.stringify(from)} 命中 ${parts.length - 1} 次`);
  return parts.join(to);
}

/**
 * 装配一棵真实的依赖树：helpers / refreshGate / storage / authRole / transport → request → course。
 * @param {{ courseSource?: string }} [opts] 用变异源码替换 course.uts 真源
 */
function buildApp(opts = {}) {
  const uni = makeUni();
  const helpers = loadUts(HELPERS_UTS, {});
  const envelope = loadUts(path.join(ROOT, 'api', 'envelope.uts'), envelopeBindings({
    getMapped: () => Promise.resolve({}),
    toNumber: helpers.toNumber,
  }));
  const storage = loadUts(STORAGE_UTS, { uni });
  const gate = loadUts(GATE_UTS, {});
  const authRole = loadUts(AUTH_ROLE_UTS, {
    getStorage: storage.getStorage,
    setStorage: storage.setStorage,
    removeStorage: storage.removeStorage,
    STORAGE_KEY_ACTIVE_ROLE: KEY_ACTIVE_ROLE,
    ACTIVE_ROLE_STUDENT: 'student',
    ACTIVE_ROLE_RECRUITER: 'recruiter',
  });
  const transportMod = transport({ uni, getStorage: storage.getStorage, API_BASE_URL: BASE_URL, STORAGE_KEY_TOKEN: KEY_TOKEN });
  const request = loadUts(REQUEST_UTS, {
    ...transportAliases(transportMod),
    gateRefresh: gate.gateRefresh,
    API_BASE_URL: BASE_URL,
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
    getCurrentPages: () => [{ route: 'pages/courses/course-detail' }],
  });
  const courseBindings = {
    getMapped: request.getMapped,
    postMapped: request.postMapped,
    post: request.post,
    decodePage: envelope.decodePage,
    decodeList: envelope.decodeList,
    toNumber: helpers.toNumber,
    toNumberOrNull: helpers.toNumberOrNull,
    toBool: helpers.toBool,
    toStr: helpers.toStr,
    errMsg: helpers.errMsg,
    NOT_FOUND_MESSAGE_PREFIX: request.NOT_FOUND_MESSAGE_PREFIX,
  };
  const course = opts.courseSource == null
    ? loadUts(COURSE_UTS, courseBindings)
    : loadUtsFromSource(opts.courseSource, 'course.uts', courseBindings);

  /** 回应全部挂起请求 */
  function respondAll(reply) {
    const pending = uni.requests.splice(0, uni.requests.length);
    pending.forEach((o) => {
      if (reply.fail) o.fail(reply.fail);
      else o.success({ statusCode: reply.statusCode, data: reply.data });
    });
    return pending.length;
  }
  return { uni, request, course, respondAll };
}

const COURSE_SRC = readText(COURSE_UTS);

describe('#1944 A：真实链路上的上报形态与幂等回包（行为守护·承重）', () => {
  it('A1 点开即上报：POST 打 /course/<id>/browse，course_id 不进 body，解析出 recorded=true', async () => {
    const app = buildApp();
    let resolved = null;
    app.course.recordCourseBrowseApi(7).then((v) => { resolved = v; });
    await tick();
    expect(app.uni.requests.length).toBe(1);
    const sent = app.uni.requests[0];
    expect(sent.method).toBe('POST');
    expect(sent.url).toBe(BASE_URL + '/course/7/browse');
    expect(sent.data == null).toBe(true);
    expect(app.respondAll(RECORDED_TRUE)).toBe(1);
    await tick();
    expect(resolved).toBe(true);
  });

  it('A2 recorded=false 是幂等回包：resolve(false) 且**不发第二个请求**（别当失败重试）', async () => {
    const app = buildApp();
    let resolved = null;
    app.course.recordCourseBrowseApi(7).then((v) => { resolved = v; });
    await tick();
    app.respondAll(RECORDED_FALSE);
    await tick();
    expect(resolved).toBe(false);
    expect(app.uni.requests.length).toBe(0); // 重试会把「今天已记过」变成第二次请求
  });

  it('A3 404 上抛不折成成功，且静默（silent=true：旁路上报不得弹用户没触发的报错）', async () => {
    const app = buildApp();
    let rejected = null;
    app.course.recordCourseBrowseApi(7).catch((e) => { rejected = e; });
    await tick();
    app.respondAll(NOT_FOUND_REPLY);
    await tick();
    expect(rejected instanceof Error).toBe(true);
    expect(String(rejected.message)).toMatch(/404/);
    expect(app.uni.toasts).toEqual([]);
  });
});

describe('#1944 C：变异必红侧（证明 A 组钉的是行为，不是字面量）', () => {
  it('C1 把 recorded 判定改反 → recorded=false 会被解析成 true（A2 必红）', async () => {
    const mutated = mutateOnce(COURSE_SRC, "data['recorded'] == true", "data['recorded'] != true");
    const app = buildApp({ courseSource: mutated });
    let resolved = null;
    app.course.recordCourseBrowseApi(7).then((v) => { resolved = v; });
    await tick();
    app.respondAll(RECORDED_FALSE);
    await tick();
    expect(resolved).toBe(true); // 变异生效的证据：与 A2 的 false 相反
  });

  it('C2 摘掉 silent → 404 时 toast 出现（A3 的静默断言有红能力）', async () => {
    const mutated = mutateOnce(COURSE_SRC, 'silent: true', 'silent: false');
    const app = buildApp({ courseSource: mutated });
    app.course.recordCourseBrowseApi(7).catch(() => { });
    await tick();
    app.respondAll(NOT_FOUND_REPLY);
    await tick();
    expect(app.uni.toasts.length).toBeGreaterThan(0);
  });
});
