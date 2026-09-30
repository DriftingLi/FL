/**
 * #1204 简历 notfound 语义 —— **行为级**守护（真跑 `api/request.uts` → `api/resume.uts` → 编辑页 composable）
 *
 * 病根（票面）：`api/request.uts` 的错误出口恒为 `Error(message)`、**不带 `kind`**；`getResumeApi()` 又包一层
 * `new Error(...)`，且按**后端中文文案**（`不存在` / `404`）识别 404；而简历总览页与在线简历编辑页
 * 各自按 `e['kind'] == 'notfound'` 判定 ⇒ 该分支**恒不命中**：
 *   ① 新用户（无简历）进编辑页会看到「简历加载失败」toast；
 *   ② 总览页的完善度引导卡（`resumeIncomplete`）恒不展示。
 *
 * 本文件钉住的不变量：
 *   I1 **404 的唯一出口**：`api/request.uts` 的 404 分支把状态码写成**消息前缀**
 *      （`NOT_FOUND_MESSAGE_PREFIX`），判定出口是 `api/resume.uts` 的 `isResumeNotFound()`；
 *      两者在这里是**经真实调用链**串起来的（不是镜像实现、不是抄一份字面量）。
 *   I2 notfound **之外**的错误（500 / 业务码错误 / 网络失败 / 403 / 非 Error）**不得**被判成 notfound。
 *   I3 编辑页 composable（真跑）在 404 下**不弹**「简历加载失败」，在 500 下**弹** —— 成对取证，防「恒绿」。
 *
 * 为什么不用源码文本断言：`expect(src).toContain('startsWith(NOT_FOUND_MESSAGE_PREFIX)')` 在
 * 「404 分支被摘掉、只留一句字面量」时照样绿，它回答不了 ③ 门的第一条判据
 * （「我故意弄坏被测物，它会不会红」）。所以用 `utils/utsHarness.js` 把**真的** `.uts` 跑起来，`uni.request` 由测试驱动；
 * 「必红」一侧由**注入变异源码**的两条用例给（见 describe D：真源被改成老形态时，本文件的 I1/I3 必须变红）。
 *
 * 边界（照实记）：`.uvue` 的模板/样式在 node 里跑不了 ⇒ **页面**（总览页的引导卡、编辑页壳层）不在本文件的
 * 执行面内，它靠 ①a 真机逐页截图 + `utils/resumeContract.test.js` 的接线断言兜（行为面由本文件兜，
 * 两者都不缺才不是假绿）。`getPositionsApi` 与 vue 的 `ref` / `computed` 在此按**非被测依赖**注入。
 */
const fs = require('fs');
const os = require('os');
const path = require('path');
const { loadUts, readText } = require('./utsHarness');

const ROOT = path.join(__dirname, '..');
const REQUEST_UTS = path.join(ROOT, 'api', 'request.uts');
const RESUME_UTS = path.join(ROOT, 'api', 'resume.uts');
const HELPERS_UTS = path.join(ROOT, 'api', 'helpers.uts');
const GATE_UTS = path.join(ROOT, 'api', 'refreshGate.uts');
const STORAGE_UTS = path.join(ROOT, 'utils', 'storage.uts');
const NAVIGATION_UTS = path.join(ROOT, 'utils', 'navigation.uts');
const AUTH_ROLE_UTS = path.join(ROOT, 'utils', 'authRole.uts');
const COMPOSABLE_UTS = path.join(ROOT, 'pages', 'resume', 'composables', 'useResumeEdit.uts');

const KEY_TOKEN = 'auth_token';
const KEY_USER = 'auth_user';
const KEY_ACTIVE_ROLE = 'auth_active_role';
const ROLE_STUDENT = 'student';
const ROLE_RECRUITER = 'recruiter';
const STORAGE_KEY_RESUME = 'user_resume_draft';

/** 一次 tick：让 `uni.request` 的 success/fail 回调与 Promise 链跑完 */
const tick = () => new Promise((resolve) => setTimeout(resolve, 0));

/** 假 `uni`：storage 落 Map（语义同 `utils/storage.uts`），request 只**捕获**不自动回应 */
function makeUni() {
  const kv = new Map();
  const requests = [];
  const toasts = [];
  const relaunches = [];
  return {
    kv,
    requests,
    toasts,
    relaunches,
    setStorageSync: (k, v) => { kv.set(k, v); },
    getStorageSync: (k) => (kv.has(k) ? kv.get(k) : ''),
    removeStorageSync: (k) => { kv.delete(k); },
    clearStorageSync: () => { kv.clear(); },
    getStorageInfoSync: () => ({ currentSize: 0 }),
    showToast: (o) => { toasts.push(o); },
    showLoading: () => {},
    hideLoading: () => {},
    showActionSheet: () => {},
    reLaunch: (o) => { relaunches.push(o); },
    navigateBack: (o) => { if (o && o.success) o.success({}); },
    request: (o) => { requests.push(o); },
    getSystemInfoSync: () => ({ statusBarHeight: 20, windowHeight: 800 }),
  };
}

/**
 * 装配一棵**真实**的依赖树：`helpers.uts` / `refreshGate.uts` / `storage.uts` / `authRole.uts`
 * → `request.uts` → `resume.uts`（可选：用变异源码替换其中任一层的真源）。
 * @param {{ requestSource?: string, resumeSource?: string }} [opts]
 */
function buildApp(opts = {}) {
  const uni = makeUni();
  const pages = [{ route: 'pages/resume/resume-edit' }];
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

  const resumeBindings = {
    getMapped: request.getMapped,
    requestMapped: request.requestMapped,
    del: request.del,
    uploadFile: request.uploadFile,
    toStr: helpers.toStr,
    toNumber: helpers.toNumber,
    toBool: helpers.toBool,
    errMsg: helpers.errMsg,
    // ⚠️ 读写两端共用**同一个真源常量**：这里从 request 模块回读，不抄字面量
    NOT_FOUND_MESSAGE_PREFIX: request.NOT_FOUND_MESSAGE_PREFIX,
  };
  const resume = opts.resumeSource == null
    ? loadUts(RESUME_UTS, resumeBindings)
    : loadUtsFromSource(opts.resumeSource, 'resume.uts', resumeBindings);

  /** 回应全部挂起请求；返回回应条数 */
  function respondAll(reply) {
    const pending = uni.requests.splice(0, uni.requests.length);
    pending.forEach((o) => {
      if (reply.fail) o.fail(reply.fail);
      else o.success({ statusCode: reply.statusCode, data: reply.data });
    });
    return pending.length;
  }

  /**
   * 起一个**真的**编辑页 composable（`pages/resume/composables/useResumeEdit.uts`），
   * 只把非被测依赖（vue 的响应式容器 / 岗位接口 / uni 的 toast 出口）注入。
   */
  function loadEditPage() {
    const composed = loadUts(COMPOSABLE_UTS, {
      // 非被测依赖：响应式容器（`vue` 不在 node_modules 里，本仓行为测试一律注入最小 ref/computed）
      ref: (v) => ({ value: v }),
      computed: (fn) => ({ get value() { return fn(); } }),
      setStorage: storage.setStorage,
      getStorageJSON: storage.getStorageJSON,
      goBack: loadUts(NAVIGATION_UTS, { uni, getCurrentPages: () => pages }).goBack,
      STORAGE_KEY_RESUME,
      toStr: helpers.toStr,
      toNumber: helpers.toNumber,
      toBool: helpers.toBool,
      errMsg: helpers.errMsg,
      getResumeApi: resume.getResumeApi,
      isResumeNotFound: resume.isResumeNotFound,
      saveResumeApi: resume.saveResumeApi,
      // 非被测依赖：岗位列表（与 notfound 判定无关），不起真请求以免干扰挂起队列
      getPositionsApi: () => Promise.resolve([]),
      uni,
    });
    return composed.useResumeEdit();
  }

  return { uni, request, resume, helpers, loadEditPage, respondAll };
}

/** 把变异后的源码写到临时目录再交给 harness（真源文件**一个字节都不动**） */
function loadUtsFromSource(source, fileName, bindings) {
  const dir = fs.mkdtempSync(path.join(os.tmpdir(), 'fl-1204-'));
  const file = path.join(dir, fileName);
  fs.writeFileSync(file, source);
  try {
    return loadUts(file, bindings);
  } finally {
    fs.rmSync(dir, { recursive: true, force: true });
  }
}

/** 单点字面量替换；锚点不唯一即抛（否则变异会静默落空 —— 那样「必红」用例会变成假绿） */
function mutateOnce(src, from, to) {
  const parts = src.split(from);
  if (parts.length !== 2) {
    throw new Error(`变异锚点必须唯一：${JSON.stringify(from)} 命中 ${parts.length - 1} 次`);
  }
  return parts.join(to);
}

/** 后端 `GET /resume` 对无简历用户的实际形态（404 + 中文文案） */
const NOT_FOUND_REPLY = { statusCode: 404, data: { code: 404, message: '简历不存在' } };
const SERVER_ERROR_REPLY = { statusCode: 500, data: { code: 500, message: '服务器内部错误' } };

describe('A. 404 的唯一出口：真实 request.uts → 真实 resume.uts', () => {
  test('A1：404 ⇒ isResumeNotFound 为真，且消息仍带后端文案（前缀协议单点真源）', async () => {
    const app = buildApp();
    const pending = app.resume.getResumeApi();
    await tick();
    expect(app.uni.requests.length).toBe(1);
    expect(app.uni.requests[0].url).toBe('https://example.test/api/resume');

    app.respondAll(NOT_FOUND_REPLY);
    const err = await pending.then(() => null, (e) => e);
    expect(err instanceof Error).toBe(true);
    expect(app.resume.isResumeNotFound(err)).toBe(true);
    // 前缀是**读端常量**（不是抄的字面量）：两端同值才成立
    expect(app.request.NOT_FOUND_MESSAGE_PREFIX).toBe('404:');
    expect(err.message).toBe(app.request.NOT_FOUND_MESSAGE_PREFIX + '简历不存在');
    // 展示面：`errMsg` 拿到的仍是「前缀 + 后端文案」，没被吞成空串
    expect(app.helpers.errMsg(err, '')).toContain('简历不存在');
  });

  test('A2：200 ⇒ 仍走手动映射（删掉旧 catch 没伤 happy path）', async () => {
    const app = buildApp();
    const pending = app.resume.getResumeApi();
    await tick();
    app.respondAll({
      statusCode: 200,
      data: { code: 200, message: 'ok', data: { real_name: '张三', contact_phone: '13800000000', visibility: 'open' } },
    });
    const dto = await pending;
    expect(dto.real_name).toBe('张三');
    expect(dto.contact_phone).toBe('13800000000');
    expect(dto.visibility).toBe('open');
    // 未建判据不得把成功响应误判
    expect(app.resume.isResumeNotFound(null)).toBe(false);
  });

  test('A3：500 ⇒ 不是 notfound（不得静默）', async () => {
    const app = buildApp();
    const pending = app.resume.getResumeApi();
    await tick();
    app.respondAll(SERVER_ERROR_REPLY);
    const err = await pending.then(() => null, (e) => e);
    expect(app.resume.isResumeNotFound(err)).toBe(false);
    expect(err.message).toBe('服务器内部错误');
  });

  test('A4：HTTP 200 + 业务码 500 ⇒ 不是 notfound', async () => {
    const app = buildApp();
    const pending = app.resume.getResumeApi();
    await tick();
    app.respondAll({ statusCode: 200, data: { code: 500, message: '简历服务异常' } });
    const err = await pending.then(() => null, (e) => e);
    expect(app.resume.isResumeNotFound(err)).toBe(false);
    expect(err.message).toBe('简历服务异常');
  });

  test('A5：网络失败 ⇒ 不是 notfound', async () => {
    const app = buildApp();
    const pending = app.resume.getResumeApi();
    await tick();
    app.respondAll({ fail: { errMsg: 'request:fail timeout' } });
    const err = await pending.then(() => null, (e) => e);
    expect(app.resume.isResumeNotFound(err)).toBe(false);
  });

  test('A6：403 前缀不得被 notfound 判定吃掉（两个出口互不串味）', async () => {
    const app = buildApp();
    const pending = app.resume.getResumeApi();
    await tick();
    app.respondAll({ statusCode: 403, data: { code: 403, message: '无权限访问' } });
    const err = await pending.then(() => null, (e) => e);
    expect(app.resume.isResumeNotFound(err)).toBe(false);
    expect(err.message.startsWith(app.request.FORBIDDEN_MESSAGE_PREFIX)).toBe(true);
  });

  test('A7：非 Error / 空值 fail-safe（不强转崩）', () => {
    const app = buildApp();
    expect(app.resume.isResumeNotFound(null)).toBe(false);
    expect(app.resume.isResumeNotFound({ kind: 'notfound' })).toBe(false);
    expect(app.resume.isResumeNotFound('404: 简历不存在')).toBe(false);
    expect(app.resume.isResumeNotFound(new Error(''))).toBe(false);
    // 前缀只在**开头**才算（消息里出现「404:」不算）
    expect(app.resume.isResumeNotFound(new Error('请求失败：404: 简历不存在'))).toBe(false);
  });

  test('A8：README 症状对照 —— 老形态（无前缀 Error）不得被判成 notfound', () => {
    const app = buildApp();
    // 这正是旧实现的出口形态：`new Error('RESUME_NOT_FOUND: 简历不存在')`
    expect(app.resume.isResumeNotFound(new Error('RESUME_NOT_FOUND: 简历不存在'))).toBe(false);
  });
});

describe('B. 编辑页 composable：新用户（404）不弹「简历加载失败」', () => {
  test('B1（必不红）：404 ⇒ 无「简历加载失败」toast', async () => {
    const app = buildApp();
    const vm = app.loadEditPage();
    vm.boot();
    await tick();
    expect(app.respondAll(NOT_FOUND_REPLY)).toBeGreaterThan(0);
    await tick();
    await tick();
    expect(app.uni.toasts.filter((t) => t.title === '简历加载失败')).toEqual([]);
  });

  test('B2（必红侧对照）：500 ⇒ 仍弹「简历加载失败」', async () => {
    const app = buildApp();
    const vm = app.loadEditPage();
    vm.boot();
    await tick();
    expect(app.respondAll(SERVER_ERROR_REPLY)).toBeGreaterThan(0);
    await tick();
    await tick();
    expect(app.uni.toasts.filter((t) => t.title === '简历加载失败').length).toBe(1);
  });
});

describe('C. 判别力自检：变异真源必须让本文件变红（成对取证的另一半）', () => {
  test('C1：摘掉 request.uts 404 出口的前缀（退回老形态）⇒ 404 不再被判成 notfound', async () => {
    const realRequest = readText(REQUEST_UTS);
    const mutated = mutateOnce(
      realRequest,
      'reject(new Error(NOT_FOUND_MESSAGE_PREFIX + notFoundMsg))',
      'reject(new Error(notFoundMsg))',
    );
    const app = buildApp({ requestSource: mutated });
    const pending = app.resume.getResumeApi();
    await tick();
    app.respondAll(NOT_FOUND_REPLY);
    const err = await pending.then(() => null, (e) => e);
    // 行为面变红：这正是 #1204 的术前行为
    expect(app.resume.isResumeNotFound(err)).toBe(false);
  });

  test('C2：断掉 resume.uts 读端（isResumeNotFound 恒 false）⇒ 404 也判不出来', async () => {
    const realResume = readText(RESUME_UTS);
    const mutated = mutateOnce(
      realResume,
      "return errMsg(e, '').startsWith(NOT_FOUND_MESSAGE_PREFIX)",
      'return false',
    );
    const app = buildApp({ resumeSource: mutated });
    const pending = app.resume.getResumeApi();
    await tick();
    app.respondAll(NOT_FOUND_REPLY);
    const err = await pending.then(() => null, (e) => e);
    expect(err.message.startsWith(app.request.NOT_FOUND_MESSAGE_PREFIX)).toBe(true);
    expect(app.resume.isResumeNotFound(err)).toBe(false);
  });

  test('C3：编辑页在变异树上会把 404 误报成「简历加载失败」（B1 的反面）', async () => {
    const realResume = readText(RESUME_UTS);
    const mutated = mutateOnce(
      realResume,
      "return errMsg(e, '').startsWith(NOT_FOUND_MESSAGE_PREFIX)",
      'return false',
    );
    const app = buildApp({ resumeSource: mutated });
    const vm = app.loadEditPage();
    vm.boot();
    await tick();
    app.respondAll(NOT_FOUND_REPLY);
    await tick();
    await tick();
    expect(app.uni.toasts.filter((t) => t.title === '简历加载失败').length).toBe(1);
  });
});
