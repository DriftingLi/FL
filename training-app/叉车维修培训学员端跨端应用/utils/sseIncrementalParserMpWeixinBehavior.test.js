/**
 * #2457 · 微信小程序（mp-weixin）没有 `TextDecoder` —— 解析器与整包降级必须照常工作
 *
 * 报障：体验版智能维修诊断无回答，`ReferenceError: TextDecoder is not defined`
 * 抛在 `createSseParser()` 里（`utils/sseIncrementalParser.uts` 的类字段 `new TextDecoder()`）。
 * 同一平台上 `api/aiAssistant.uts` 的 `bodyToText()` 也用了 `TextDecoder`，被 catch 吞掉后
 * 返回空串，整包降级路会误报「回答未完整返回」。
 *
 * 为什么必须真跑：Node 里 `TextDecoder` 是全局，既有行为测试**测不出** mp-weixin 的缺失。
 * 本套件在装载前删掉 `globalThis.TextDecoder`，把小程序运行时的形状搬进 Node。
 */
const path = require('path');
const { loadUts } = require('./utsHarness');

const ROOT = path.join(__dirname, '..');
const PARSER_UTS = path.join(ROOT, 'utils', 'sseIncrementalParser.uts');
const API_UTS = path.join(ROOT, 'api', 'aiAssistant.uts');
const HELPERS_UTS = path.join(ROOT, 'api', 'helpers.uts');
const ENVELOPE_UTS = path.join(ROOT, 'api', 'envelope.uts');

/** 字符串 → ArrayBuffer（不依赖 TextEncoder：小程序同样没有它） */
function bytes(text) {
  const b = Buffer.from(text, 'utf8');
  return new Uint8Array(b).buffer;
}

/** 拼 SSE 文本（每帧以空行收尾，与后端 handler.go 的写法一致） */
function sse(frames) {
  return frames.map((f) => `event: ${f.event}\ndata: ${f.data}\n\n`).join('');
}

/**
 * 装一个真的 api/aiAssistant.uts（整包降级路：task 没有 onChunkReceived）。
 */
function makeApi() {
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
      const task = {};
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
    createSseParser: parserMod.createSseParser,
    decodeUtf8: parserMod.decodeUtf8,
    uni: uni,
    console: console,
  });
  return { api, requests };
}

describe('#2457 mp-weixin 无 TextDecoder', () => {
  let savedTextDecoder;

  beforeEach(() => {
    savedTextDecoder = globalThis.TextDecoder;
    delete globalThis.TextDecoder;
  });

  afterEach(() => {
    globalThis.TextDecoder = savedTextDecoder;
  });

  it('删除 TextDecoder 后 createSseParser 仍可构造并解析帧（#2432 崩溃点）', () => {
    const mod = loadUts(PARSER_UTS, {});
    const parser = mod.createSseParser();
    const frames = parser.feed(bytes('event: message\ndata: {"content":"叉车"}\n\n'));
    expect(frames).toEqual([{ event: 'message', data: '{"content":"叉车"}' }]);
  });

  it('跨块切在多字节字符中间：尾部挂起 + 自研解码器仍拼回原字符', () => {
    const mod = loadUts(PARSER_UTS, {});
    const parser = mod.createSseParser();
    const text = sse([{ event: 'message', data: '{"content":"电瓶"}' }]);
    const cut = text.indexOf('电') + 1;
    const first = parser.feed(bytes(text.slice(0, cut)));
    const second = parser.feed(bytes(text.slice(cut)));
    expect(first).toEqual([]);
    expect(second).toEqual([{ event: 'message', data: '{"content":"电瓶"}' }]);
  });

  it('整包降级路在无 TextDecoder 时仍能读出正文（bodyToText 第二处缺陷）', () => {
    const { api, requests } = makeApi();
    const seen = { messages: [], errors: [], done: 0 };
    api.streamChatApi({}, {
      onMessage: (c) => seen.messages.push(c),
      onError: (m) => seen.errors.push(m),
      onDone: () => { seen.done += 1; },
    });
    requests[0].opts.success({
      statusCode: 200,
      data: bytes(sse([
        { event: 'message', data: '{"content":"整包"}' },
        { event: 'done', data: '{"content":"整包"}' },
      ])),
    });
    expect(seen.messages).toEqual(['整包']);
    expect(seen.errors).toEqual([]);
    expect(seen.done).toBe(1);
  });
});

describe('#2457 decodeUtf8 口径', () => {
  it('1/2/3/4 字节与代理对往返一致', () => {
    const mod = loadUts(PARSER_UTS, {});
    const samples = ['', 'abc', '叉车电瓶', 'éüñ', '😀🎉', '中\u0000文', '\u{10FFFF}', '\u007f\u0080\u07ff\u0800\uffff'];
    for (const s of samples) {
      expect(mod.decodeUtf8(new Uint8Array(bytes(s)))).toBe(s);
    }
  });

  it('与 Node TextDecoder 对合法 UTF-8 逐字节一致（抽样）', () => {
    const mod = loadUts(PARSER_UTS, {});
    const ref = new TextDecoder();
    const enc = new TextEncoder();
    const pool = ['a', 'Z', '0', ' ', '\n', '叉', '车', '电', '瓶', '😀', '🎉', 'é', 'ü', '中', '文', '\u0000', '\u007f', '\u0080', '\u07ff', '\u0800', '\uffff', '\u{10000}', '\u{10ffff}'];
    for (let t = 0; t < 200; t++) {
      let s = '';
      const n = 1 + Math.floor(Math.random() * 100);
      for (let i = 0; i < n; i++) s += pool[Math.floor(Math.random() * pool.length)];
      const b = enc.encode(s);
      expect(mod.decodeUtf8(b)).toBe(ref.decode(b));
    }
  });

  it('非法序列不抛，产出 U+FFFD', () => {
    const mod = loadUts(PARSER_UTS, {});
    const bad = [
      [0x80], [0xbf], [0xc0, 0x80], [0xc1, 0xbf], [0xe0, 0x80, 0x80],
      [0xed, 0xa0, 0x80], [0xf4, 0x90, 0x80, 0x80], [0xf5, 0x80, 0x80, 0x80],
      [0xe4, 0xb8], [0xf0, 0x9f, 0x98], [0xff], [0xfe],
    ];
    for (const b of bad) {
      const out = mod.decodeUtf8(new Uint8Array(b));
      expect(typeof out).toBe('string');
      expect(out).toContain('\uFFFD');
    }
  });
});
