/**
 * 请求基座 mapper-callback 泛型出口契约测试（TDD: RED → GREEN 钉住行为契约）
 *
 * request.uts 无法在 jest 中 import（.uts 非 JS 模块），沿用项目既有的
 * 「源码契约测试」缝：读取源文本，断言函数体的外部行为语义。
 * 先例：utils/secureStorage.test.js、utils/aiChatModesContract.test.js。
 *
 * 钉住的行为契约（对应 Issue #639 / refactor epic #638）：
 * 1) 出口签名：requestMapped<T>(options, map) —— mapper-callback 形态，
 *    域 api 传入自己的 DTO 映射函数（与 ADR-0003 手动 JSON 映射同构）
 * 2) 拆包后映射：mapper 收到的是 ApiResponse 拆包后的 data（非信封）
 * 3) 失败传播：映射函数抛错不被静默吞掉，沿 Promise 链 reject
 * （切片 2/3 的契约用例随后续 commit 追加）
 */
const path = require('path');

/** 读源码一律经共享读者归一 EOL（ADR-0019）：与检出平台无关，Windows CRLF 也免疫。 */
const { readText } = require('./utsHarness');
const REQ_PATH = path.join(__dirname, '..', 'api', 'request.uts');
const TRANSPORT_PATH = path.join(__dirname, '..', 'api', 'transport.uts');
const src = readText(REQ_PATH);
const transportSrc = readText(TRANSPORT_PATH);

/**
 * 提取函数体：从函数声明到顶层级 "\n}"（先例：quickLoginContract）。
 * ⚠️ 必须带左括号：`fnBody('get')` 若只匹配 `function get`，会先命中 `function getMapped`
 *    —— 于是断言跑在错的函数体上、照样绿（假绿的形态之一，#1734 自查时抓到）。
 */
function fnBody(name) {
  // 泛型出口写作 `function name<T>(`，非泛型写作 `function name(`：两种都要精确到「名字后面紧跟
  // ( 或 <」，否则 `function get` 会先命中 `function getMapped`。
  const start = src.search(new RegExp('function\\s+' + name + '\\s*(?:<[^>]*>)?\\s*\\('));
  if (start === -1) throw new Error('未找到 ' + name);
  return src.slice(start, src.indexOf('\n}', start));
}

describe('出口签名契约（requestMapped）', () => {
  it('导出泛型出口 requestMapped<T>：RequestOptions + mapper 回调，返回 Promise<T>', () => {
    const sig = /export\s+function\s+requestMapped\s*<\s*T\s*>\s*\(\s*options\s*:\s*RequestOptions\s*,\s*map\s*:\s*\(\s*data\s*:\s*UTSJSONObject\s*\)\s*=>\s*T\s*\)\s*:\s*Promise\s*<\s*T\s*>/;
    expect(sig.test(src)).toBe(true);
  });
});

describe('拆包后映射契约', () => {
  it('出口复用底层 request 的解包结果，mapper 收到 data 而非信封', () => {
    const body = fnBody('requestMapped');
    expect(body).toContain('request<UTSJSONObject>(options)');
    expect(body).toMatch(/\.then\s*\(/);
    expect(body).toMatch(/map\s*\(\s*data\s*\)/);
  });

  it('出口内不二次解包：不得再取信封 data 字段（mapper 收到的已是拆包结果）', () => {
    const body = fnBody('requestMapped');
    expect(body).not.toContain("['data']");
    expect(body).not.toContain('["data"]');
  });
});

describe('失败传播契约', () => {
  it('映射函数抛错不被静默吞掉：出口函数体无 try/catch，异常沿 Promise 链 reject', () => {
    const body = fnBody('requestMapped');
    expect(body).not.toMatch(/\btry\s*\{/);
    expect(body).not.toMatch(/\bcatch\s*\(/);
  });
});

/**
 * #1734 问题 2：出口家族补齐 PUT/PATCH 便捷面后，四个 mapped 快捷出口必须**同形**
 * （同一 opts.url / opts.method / opts.data 通路 + 同一 requestMapped 委托）。
 * 形态漂移 = 请求形态漂移 = 后端看到的 body/method 变了，这是本票最容易悄悄改坏语义的地方。
 */
describe('mapped 出口家族同形契约（#1734 问题 2）', () => {
  const METHODS = { getMapped: 'GET', postMapped: 'POST', putMapped: 'PUT', patchMapped: 'PATCH' };
  it('四个便捷出口都已导出，签名同 postMapped（url / data / map / options）', () => {
    for (const name of Object.keys(METHODS)) {
      const sig = new RegExp('export\\s+function\\s+' + name + '\\s*<\\s*T\\s*>\\s*\\(\\s*url\\s*:\\s*string\\s*(,[\\s\\S]*)\\)\\s*:\\s*Promise\\s*<\\s*T\\s*>');
      expect(sig.test(src)).toBe(true);
    }
  });

  it('每个便捷出口都只设自己那一个 method，并委托 requestMapped（不各抄一份传输逻辑）', () => {
    for (const [name, method] of Object.entries(METHODS)) {
      const body = fnBody(name);
      expect(body).toContain("opts.method = '" + method + "'");
      // 反向：不得顺手把别的 method 也写进来（换轨 = 请求打到错误的方法上）
      for (const other of Object.values(METHODS)) {
        if (other !== method) expect(body).not.toContain("opts.method = '" + other + "'");
      }
      expect(body).toContain('requestMapped<T>(opts, map)');
      expect(body).not.toContain("['data']");
      expect(body).not.toMatch(/\btry\s*\{/);
    }
  });

  it('putMapped / patchMapped 带 data 载荷（GET 便捷面没有 data，POST/PUT/PATCH 必须有）', () => {
    for (const name of ['postMapped', 'putMapped', 'patchMapped']) {
      expect(fnBody(name)).toContain('opts.data = data');
    }
  });
});

/**
 * #1734 问题 3：query 序列化三处同构（getMapped / get / envelope.getPaged）收一为
 * transport.buildQuery + withQuery + mergeQueryParams。这里钉「收一没被拆回去」：
 * 任何一份 for...in 手写序列化重新出现，本用例就红。
 */
describe('query 序列化单点契约（#1734 问题 3）', () => {
  it('序列化只发生在 transport 的 buildQuery 里', () => {
    const i = transportSrc.indexOf('export function buildQuery');
    expect(i).toBeGreaterThan(-1);
    const body = transportSrc.slice(i, transportSrc.indexOf('\n}', i));
    expect(body).toContain('encodeURIComponent');
    expect(body).toContain('pairs.join');
  });

  it('request.uts 与 envelope.uts 内不再各抄 for...in 拼 query', () => {
    const strip = (text) => text.replace(/\/\*[\s\S]*?\*\//g, '').replace(/^\s*\/\/.*$/gm, '');
    for (const [where, text] of [['request.uts', strip(src)], ['envelope.uts', strip(readText(path.join(__dirname, '..', 'api', 'envelope.uts')))]]) {
      expect(text).not.toMatch(/encodeURIComponent/);
      expect(text).not.toMatch(/for\s*\(\s*const\s+\w+\s+in\s+params\s*\)/);
    }
  });

  it('getMapped 与 get 都经 withQuery（两轴拆分后 query 只有一个入口）', () => {
    expect(fnBody('getMapped')).toContain('withQuery(');
    expect(fnBody('get')).toContain('withQuery(');
  });

  it('envelope.getPaged 的 query 合并走 mergeQueryParams（原第三份同构循环已消亡）', () => {
    const envSrc = readText(path.join(__dirname, '..', 'api', 'envelope.uts'));
    const i = envSrc.indexOf('export function getPaged');
    expect(i).toBeGreaterThan(-1);
    const body = envSrc.slice(i, envSrc.indexOf('\n}', i));
    expect(body).toContain('mergeQueryParams(params, query)');
    expect(body).not.toMatch(/for\s*\(\s*const\s+k\s+in\s+query\s*\)/);
  });
});

/**
 * #1734 问题 1：两轴 seam —— 传输层不得反向依赖续期/重试编排。
 * （正向依赖是允许的，request.uts 依赖 transport.uts；反向会让传输层无法独立测试。）
 */
describe('两轴 seam 契约（#1734 问题 1）', () => {
  it('transport.uts 的 import 面与代码面都不含续期编排件（注释里提及不算，故先剥注释）', () => {
    const strip = (text) => text.replace(/\/\*[\s\S]*?\*\//g, '').replace(/^\s*\/\/.*$/gm, '');
    const code = strip(transportSrc);
    expect(code).not.toMatch(/from '\.\/refreshGate'/);
    expect(code).not.toMatch(/from '\.\/request'/);
    expect(code).not.toMatch(/renewOrExpire|handleUnauthorized|gateRefresh|uni\.reLaunch|isHandlingUnauthorized/);
  });

  it("request.uts 仍是消费方口径：前缀常量从 transport 再导出，字面量单点不在本文件", () => {
    expect(src).toContain("export const FORBIDDEN_MESSAGE_PREFIX = TRANSPORT_FORBIDDEN_PREFIX");
    expect(src).toContain("export const NOT_FOUND_MESSAGE_PREFIX = TRANSPORT_NOT_FOUND_PREFIX");
    expect(src).not.toMatch(/const FORBIDDEN_MESSAGE_PREFIX = '403:'/);
    expect(transportSrc).toMatch(/export const FORBIDDEN_MESSAGE_PREFIX = '403:'/);
  });
});
