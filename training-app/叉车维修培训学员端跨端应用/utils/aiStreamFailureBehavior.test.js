/**
 * #2298 · 流式通道的**失败面** —— 行为级守护（真跑 `api/aiAssistant.uts`）
 *
 * ## 它在守什么
 *
 * 票面验收第 3 条与第 5 条各有一个真缺口，读源码只能证明「有一行文案 / 有一行收尾」，
 * 证明不了「学员真的收到那句可行动的文案」「等待态真的停了」：
 *
 * 1. **截断**：`uni.request` 是整包回调，200 也可能只收到半截流（连接中途断、body 为空）
 *    —— 既没有 `done` 也没有 `error`。收口前这种响应**没有任何终止回调**，
 *    `useAiChat` 的等待态会一直转（`onDone` / `onError` 都不触发）。判据：必须显式走一次 `onError`。
 * 2. **失败文案**：旧实现在 `fail` 里把平台 `errMsg`（`request:fail ...`）直接当文案上屏，
 *    学员读到的是一串英文，读不出下一步做什么。判据：超时 / 网络失败 / 服务不可用
 *    三类各有自己的中文可行动文案，且**原始 errMsg 不上屏**（只进日志）。
 *
 * ## 为什么必须真跑
 *
 * 文案与收尾都发生在「解析完这一整包文本之后」的分支里。用 `utsHarness` 把被测模块真装起来、
 * 让替身只回包，断言的是**回调产出**（`onError` 收到了什么），不是源码里的字面量。
 * 末组注入变异（删掉截断收尾），证明第 1 条判据有牙。
 */
const fs = require('fs');
const os = require('os');
const path = require('path');

const { loadUts, readText } = require('./utsHarness');

const ROOT = path.join(__dirname, '..');
const HELPERS_UTS = path.join(ROOT, 'api', 'helpers.uts');
const ENVELOPE_UTS = path.join(ROOT, 'api', 'envelope.uts');
const API_UTS = path.join(ROOT, 'api', 'aiAssistant.uts');

/**
 * 装一个真的 `api/aiAssistant.uts`，`uni.request` 由 `respond(opts)` 决定怎么回包。
 * 依赖清单与 `aiDoneContentBehavior.test.js` 保持一致（`utsHarness` 对新增/减少 import fail-closed）。
 */
function loadApi(respond, apiFile) {
  const h = loadUts(HELPERS_UTS, {});
  const envelope = loadUts(ENVELOPE_UTS, {
    getMapped: () => Promise.resolve({}),
    postMapped: () => Promise.resolve({}),
    toNumber: h.toNumber,
    mergeQueryParams: () => '',
  });
  const uni = {
    request: (o) => {
      respond(o);
      return {};
    },
    showToast: () => {},
    showModal: () => {},
  };
  return loadUts(apiFile || API_UTS, {
    getMapped: () => Promise.resolve({}),
    postMapped: () => Promise.resolve({}),
    patchMapped: () => Promise.resolve({}),
    del: () => Promise.resolve({}),
    uploadFile: () => Promise.resolve({}),
    renewOrExpire: () => Promise.resolve(false),
    AUTH_EXPIRED_MESSAGE: '登录已过期',
    decodeList: envelope.decodeList,
    decodePage: envelope.decodePage,
    toNumber: h.toNumber,
    toStr: h.toStr,
    toBool: h.toBool,
    API_BASE_URL: 'https://example.test/api',
    AI_REQUEST_TIMEOUT: 330000,
    STORAGE_KEY_TOKEN: 'auth_token',
    getStorage: () => 'access-token',
    createSseParser: loadUts(path.join(ROOT, 'utils', 'sseIncrementalParser.uts'), {}).createSseParser,
    uni: uni,
    console: console,
  });
}

/** 跑一轮流式对话，收集三类回调的产出 */
function runStream(respond, apiFile) {
  const api = loadApi(respond, apiFile);
  const seen = { messages: [], errors: [], done: [] };
  api.streamChatApi({}, {
    onMessage: (chunk) => seen.messages.push(chunk),
    onError: (msg) => seen.errors.push(msg),
    onDone: (data) => seen.done.push(data),
  });
  return seen;
}

/** 200 回包（SSE 文本原样） */
const okStream = (text) => (o) => o.success({ statusCode: 200, data: text });

/** 非 2xx 回包 */
const statusStream = (statusCode, data) => (o) => o.success({ statusCode, data });

/** 传输层失败（`fail` 回调；平台原始 errMsg 走这里） */
const failStream = (errMsg) => (o) => o.fail({ errMsg });

describe('#2298 流式失败面（行为级，真跑 api/aiAssistant.uts）', () => {
  describe('验收第 5 条：请求「结束」必须真的收尾', () => {
    it('200 但流里没有 done / error（截断）⇒ 显式走一次 onError，等待态才停得下来', () => {
      const seen = runStream(okStream('event: message\ndata: {"content":"半句话"}\n\n'));
      expect(seen.messages).toEqual(['半句话']);
      expect(seen.done.length).toBe(0);
      expect(seen.errors.length).toBe(1);
      expect(seen.errors[0]).toContain('重试');
    });

    it('200 且 body 为空 ⇒ 同样收尾（不是静默成功）', () => {
      const seen = runStream(okStream(''));
      expect(seen.done.length).toBe(0);
      expect(seen.errors.length).toBe(1);
    });

    it('正常的 done 收尾 ⇒ 不叠一条「未完整」错误', () => {
      const seen = runStream(okStream(
        'event: message\ndata: {"content":"答完"}\n\n' +
        'event: done\ndata: {"content":"答完"}\n\n'
      ));
      expect(seen.done.length).toBe(1);
      expect(seen.errors.length).toBe(0);
    });

    it('error 帧收尾 ⇒ 只报后端那句一次，不追一条「未完整」', () => {
      const seen = runStream(okStream(
        'event: message\ndata: {"content":"半句"}\n\n' +
        'event: error\ndata: {"message":"上游诊断助手超时"}\n\n'
      ));
      expect(seen.errors).toEqual(['上游诊断助手超时']);
    });
  });

  describe('验收第 3 条：三类失败各有可行动文案，原始 errMsg 不上屏', () => {
    it('超时 ⇒ 说清「稍后重试」', () => {
      const seen = runStream(failStream('request:fail timeout'));
      expect(seen.errors.length).toBe(1);
      expect(seen.errors[0]).toContain('超时');
      expect(seen.errors[0]).toContain('重试');
      expect(seen.errors[0]).not.toContain('request:fail');
    });

    it('网络失败 ⇒ 指到「检查网络」，不透出 request:fail', () => {
      const seen = runStream(failStream('request:fail'));
      expect(seen.errors.length).toBe(1);
      expect(seen.errors[0]).toContain('网络');
      expect(seen.errors[0]).toContain('重试');
      expect(seen.errors[0]).not.toContain('request:fail');
    });

    it('服务不可用（5xx 且无 message）⇒ 说清「稍后重试」而不是「请求失败 (503)」', () => {
      const seen = runStream(statusStream(503, ''));
      expect(seen.errors.length).toBe(1);
      expect(seen.errors[0]).toContain('服务');
      expect(seen.errors[0]).toContain('稍后重试');
    });

    it('5xx 带后端 message ⇒ 用后端那句（后端更知道发生了什么）', () => {
      const seen = runStream(statusStream(502, { message: '诊断助手未配置' }));
      expect(seen.errors).toEqual(['诊断助手未配置']);
    });
  });

  describe('检测器有牙：删掉截断收尾，第 1 条用例当场红', () => {
    it('变异副本（只 parse 不收尾）⇒ 截断时 onError 一次都不触发', () => {
      const src = readText(API_UTS);
      const anchor = '\t\t\tconst terminated = parseStreamSSE(raw, callbacks)';
      expect(src.includes(anchor)).toBe(true);
      const start = src.indexOf(anchor);
      const block = src.slice(start, src.indexOf('\t\t},', start));
      expect(block).toContain('STREAM_MSG_INCOMPLETE'); // 变异锚点必须真的在源里
      const broken = src.replace(block, '\t\t\tparseStreamSSE(raw, callbacks)\n');
      expect(broken).not.toBe(src);

      const dir = fs.mkdtempSync(path.join(os.tmpdir(), 'ai-stream-failure-mut-'));
      const file = path.join(dir, 'aiAssistant.uts');
      fs.writeFileSync(file, broken);

      const seen = runStream(okStream('event: message\ndata: {"content":"半句话"}\n\n'), file);
      expect(seen.errors.length).toBe(0); // 坏实现：截断后没有任何回调
    });
  });
});
