/**
 * #2389 · 逐字输出②：分块通道接线 + 整包降级 + status + 401 重发纪律 —— 行为级守护
 *
 * 缝：`utils/utsHarness.js` 的 `loadUts`。装的是**真的** `api/aiAssistant.uts` 与**真的**
 * `utils/sseIncrementalParser.uts`（#2388），只把 `uni.request` 换成可控替身：
 * 替身返回一个带 `onChunkReceived` 的 task，测试自己决定「喂哪几块、什么时候 success」。
 *
 * 为什么必须真跑：本票的三条判据都只在**时序**里成立 ——
 *   ① 分块与 success 竞争时只收口一次（重复 onDone / 重复正文都是静默的）；
 *   ② 401 重发后旧尝试的迟到 chunk 不得再进回调（否则正文出现两遍）；
 *   ③ 分块不可用时必须退回整包解析（否则真机上直接空回答）。
 * 读源码的接线守护看不见这三条。
 */
const path = require('path');
const { loadUts } = require('./utsHarness');

const ROOT = path.join(__dirname, '..');
const HELPERS_UTS = path.join(ROOT, 'api', 'helpers.uts');
const ENVELOPE_UTS = path.join(ROOT, 'api', 'envelope.uts');
const API_UTS = path.join(ROOT, 'api', 'aiAssistant.uts');
const PARSER_UTS = path.join(ROOT, 'utils', 'sseIncrementalParser.uts');

/** 字符串 → ArrayBuffer（分块回调的载荷恒为 ArrayBuffer） */
function buf(text) {
  return new TextEncoder().encode(text).buffer;
}

/** 拼 SSE 文本（每帧以空行收尾，与后端 handler.go 的写法一致） */
function sse(frames) {
  return frames.map((f) => `event: ${f.event}\ndata: ${f.data}\n\n`).join('');
}

/**
 * 装一个真的 api/aiAssistant.uts。
 * @param {{ renew?: () => Promise<boolean>, noChunkApi?: boolean }} opt
 *   renew = 401 续期决策替身；noChunkApi = 返回**没有** onChunkReceived 的 task（老运行时 / 平台不支持）
 */
function makeApi(opt = {}) {
  const h = loadUts(HELPERS_UTS, {});
  const envelope = loadUts(ENVELOPE_UTS, {
    getMapped: () => Promise.resolve({}),
    postMapped: () => Promise.resolve({}),
    toNumber: h.toNumber,
    mergeQueryParams: () => '',
  });
  const parserMod = loadUts(PARSER_UTS, {});
  const requests = [];
  const uni = {
    request: (o) => {
      const task = opt.noChunkApi
        ? {}
        : {
          chunkCb: null,
          onChunkReceived(cb) { this.chunkCb = cb; },
        };
      requests.push({ opts: o, task });
      return task;
    },
    showToast: () => {},
    showModal: () => {},
  };
  const api = loadUts(API_UTS, {
    getMapped: () => Promise.resolve({}),
    postMapped: () => Promise.resolve({}),
    patchMapped: () => Promise.resolve({}),
    del: () => Promise.resolve({}),
    uploadFile: () => Promise.resolve({}),
    renewOrExpire: opt.renew || (() => Promise.resolve(false)),
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
    createSseParser: parserMod.createSseParser,
    decodeUtf8: parserMod.decodeUtf8,
    uni: uni,
    console: console,
  });
  return { api, requests };
}

/** 收集回调产出 */
function collector() {
  const seen = { messages: [], status: [], errors: [], done: [], restarts: 0 };
  return {
    seen,
    callbacks: {
      onMessage: (c) => seen.messages.push(c),
      onStatus: (t) => seen.status.push(t),
      onError: (m) => seen.errors.push(m),
      onDone: (d) => seen.done.push(d),
      onRestart: () => { seen.restarts += 1; },
    },
  };
}

const flush = () => new Promise((r) => setTimeout(r, 0));

describe('#2389 分块通道（行为级，真跑 api/aiAssistant.uts + 真解析器）', () => {
  it('请求真的开了分块通道：enableChunked + responseType=arraybuffer', () => {
    const { api, requests } = makeApi();
    api.streamChatApi({}, {});
    expect(requests.length).toBe(1);
    expect(requests[0].opts.enableChunked).toBe(true);
    expect(requests[0].opts.responseType).toBe('arraybuffer');
    expect(typeof requests[0].task.onChunkReceived).toBe('function');
  });

  it('逐块喂：半包 / 粘包 / 跨块 UTF-8 都由解析器承担，正文逐段到达', () => {
    const { api, requests } = makeApi();
    const { seen, callbacks } = collector();
    api.streamChatApi({}, callbacks);
    const req = requests[0];
    const text = sse([
      { event: 'message', data: '{"content":"叉车"}' },
      { event: 'message', data: '{"content":"电瓶"}' },
      { event: 'done', data: '{"content":"叉车电瓶"}' },
    ]);
    // 在 UTF-8 多字节字符中间切一刀：解析器必须自己挂起尾部字节
    const cut = text.indexOf('电') + 1;
    req.task.chunkCb({ data: buf(text.slice(0, cut)) });
    req.task.chunkCb({ data: buf(text.slice(cut)) });
    req.opts.success({ statusCode: 200, data: '' });
    expect(seen.messages).toEqual(['叉车', '电瓶']);
    expect(seen.done.length).toBe(1);
    expect(seen.errors).toEqual([]);
  });

  it('status 帧驱动等待态文案；空文案不派发', () => {
    const { api, requests } = makeApi();
    const { seen, callbacks } = collector();
    api.streamChatApi({}, callbacks);
    const req = requests[0];
    req.task.chunkCb({ data: buf(sse([
      { event: 'status', data: '{"text":"正在检索维修知识库…"}' },
      { event: 'status', data: '{"text":""}' },
      { event: 'message', data: '{"content":"答案"}' },
      { event: 'done', data: 'null' },
    ])) });
    expect(seen.status).toEqual(['正在检索维修知识库…']);
    expect(seen.messages).toEqual(['答案']);
    expect(seen.done.length).toBe(1);
  });

  it('竞争：done 已由分块收口后 success 再来，不得重放整包（正文不重复、onDone 只一次）', () => {
    const { api, requests } = makeApi();
    const { seen, callbacks } = collector();
    api.streamChatApi({}, callbacks);
    const req = requests[0];
    const full = sse([
      { event: 'message', data: '{"content":"答案"}' },
      { event: 'done', data: '{"content":"答案"}' },
    ]);
    req.task.chunkCb({ data: buf(full) });
    // 平台把同一段 body 又整包回给 success（分块与 success 竞争的真实形态）
    req.opts.success({ statusCode: 200, data: full });
    expect(seen.messages).toEqual(['答案']);
    expect(seen.done.length).toBe(1);
    expect(seen.errors).toEqual([]);
  });

  it('断流：收到分块但没有 done / error ⇒ 显式收尾「回答未完整返回」，且只收口一次', () => {
    const { api, requests } = makeApi();
    const { seen, callbacks } = collector();
    api.streamChatApi({}, callbacks);
    const req = requests[0];
    req.task.chunkCb({ data: buf(sse([{ event: 'message', data: '{"content":"半截"}' }])) });
    req.opts.success({ statusCode: 200, data: '' });
    expect(seen.messages).toEqual(['半截']);
    expect(seen.errors.length).toBe(1);
    expect(seen.errors[0]).toContain('重试');
    expect(seen.done.length).toBe(0);
  });

  it('分块不可用（task 没有 onChunkReceived）⇒ 退回整包解析，行为与从前一致', () => {
    const { api, requests } = makeApi({ noChunkApi: true });
    const { seen, callbacks } = collector();
    api.streamChatApi({}, callbacks);
    const req = requests[0];
    req.opts.success({ statusCode: 200, data: sse([
      { event: 'message', data: '{"content":"整包"}' },
      { event: 'done', data: '{"content":"整包"}' },
    ]) });
    expect(seen.messages).toEqual(['整包']);
    expect(seen.done.length).toBe(1);
    expect(seen.errors).toEqual([]);
  });

  it('整包降级也认 ArrayBuffer 响应体（responseType 生效的平台）', () => {
    const { api, requests } = makeApi({ noChunkApi: true });
    const { seen, callbacks } = collector();
    api.streamChatApi({}, callbacks);
    requests[0].opts.success({ statusCode: 200, data: buf(sse([
      { event: 'message', data: '{"content":"字节"}' },
      { event: 'done', data: 'null' },
    ])) });
    expect(seen.messages).toEqual(['字节']);
    expect(seen.done.length).toBe(1);
  });

  it('401 重发：旧尝试作废、onRestart 触发、新请求新解析器，正文不重复', async () => {
    const { api, requests } = makeApi({ renew: () => Promise.resolve(true) });
    const { seen, callbacks } = collector();
    api.streamChatApi({}, callbacks);
    const first = requests[0];
    first.opts.success({ statusCode: 401, data: {} });
    await flush();
    expect(seen.restarts).toBe(1);
    expect(requests.length).toBe(2);
    // 旧尝试的迟到 chunk：必须被丢弃（否则重发后正文出现两遍）
    first.task.chunkCb({ data: buf(sse([{ event: 'message', data: '{"content":"旧"}' }])) });
    expect(seen.messages).toEqual([]);
    // 新尝试从头再发一遍
    const second = requests[1];
    second.task.chunkCb({ data: buf(sse([
      { event: 'message', data: '{"content":"新"}' },
      { event: 'done', data: '{"content":"新"}' },
    ])) });
    second.opts.success({ statusCode: 200, data: '' });
    expect(seen.messages).toEqual(['新']);
    expect(seen.done.length).toBe(1);
    expect(seen.errors).toEqual([]);
  });

  it('401 续期失败 ⇒ 走统一登出出口文案，不重发', async () => {
    const { api, requests } = makeApi({ renew: () => Promise.resolve(false) });
    const { seen, callbacks } = collector();
    api.streamChatApi({}, callbacks);
    requests[0].opts.success({ statusCode: 401, data: {} });
    await flush();
    expect(requests.length).toBe(1);
    expect(seen.restarts).toBe(0);
    expect(seen.errors).toEqual(['登录已过期']);
  });

  it('非 2xx 与网络中断：沿用既有可行动文案', () => {
    const a = makeApi();
    const ca = collector();
    a.api.streamChatApi({}, ca.callbacks);
    a.requests[0].opts.success({ statusCode: 503, data: {} });
    expect(ca.seen.errors).toEqual(['服务暂不可用，请稍后重试']);

    const b = makeApi();
    const cb = collector();
    b.api.streamChatApi({}, cb.callbacks);
    b.requests[0].opts.fail({ errMsg: 'request:fail timeout' });
    expect(cb.seen.errors).toEqual(['回答超时了，请稍后重试']);

    const c = makeApi();
    const cc = collector();
    c.api.streamChatApi({}, cc.callbacks);
    c.requests[0].opts.fail({ errMsg: 'request:fail' });
    expect(cc.seen.errors).toEqual(['网络连接失败，请检查网络后重试']);
  });
});
