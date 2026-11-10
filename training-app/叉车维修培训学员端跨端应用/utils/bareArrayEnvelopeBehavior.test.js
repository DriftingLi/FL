/**
 * #2404 裸数组信封归一 —— **行为级**守护（成对取证）
 *
 * ## 它在守什么
 *
 * 后端有 5 个 GET 端点 `data` 是**裸数组**（`/ai-assistant/sessions`、`/sessions/{id}/messages`、
 * `/models`、`/user-models`、`/practice-mode/free`）。传输层 `handleStatus` 曾用
 * 「`data as UTSJSONObject` 抛异常」来识别裸数组：
 *
 *   - App（Kotlin）：`as` 是运行时强转，数组抛 ClassCastException ⇒ 进 catch 包成 `{list:[...]}`；
 *   - 小程序 / H5（JS）：`as` 被**完全擦除**，裸数组原样透传 ⇒ 消费方按信封取键恒落空。
 *
 * 于是「App 端能查到历史对话、小程序端查不到」。本文件把**真链路**跑起来
 * （真 `transport.uts` 的 `handleStatus` → 真 `aiAssistant.uts` 的 mapper），断言只有跑起来
 * 才看得见的语义：
 *
 *   1. **传输层归一**：裸数组在两种运行时产出同一形状 `{list:[...]}`（保留 App 语义，消费方零改动）；
 *   2. **非数组不变**：对象原样透传、`data: null` 仍归一为 `{}`（移动端 ADR-0037）；
 *   3. **四条消费路径**：sessions / messages / models / user-models 均取到元素（修复前必红）；
 *   4. **第二道兜底**：`extractArray` 收到裸数组时直接返回（传输层归一失效也不至于恒空）；
 *   5. **日志净化同源**：`sanitizeForLog` 的「数组 / 对象」判定同样不靠强转抛异常；
 *   6. **M1 必红变异**：把 `handleStatus` 改回「cast 抛异常判形」，四条路径重新变空 —— 证明断言真能判红。
 *
 * 形态判定的原语是 `Array.isArray`：官方 `docs/uts/README.md`「类型判断」明确给出
 * `Array.isArray(a1) // 返回true` 与 `a1 instanceof Array // 返回true`，且 App 平台自 3.90 起
 * 提供 `Array.isArray`（`docs/release.md`）。`instanceof UTSJSONObject` 不可用 ——
 * JS 运行时里 `uni.request` 的 `res.data` 是**纯 JS 对象**，不是 UTSJSONObject 实例。
 */
const fs = require('fs');
const os = require('os');
const path = require('path');

const uts = require('./utsHarness');
const { transport, envelopeBindings } = require('./requestUtsHarness');

const API_DIR = path.join(__dirname, '..', 'api');
const TRANSPORT_UTS = path.join(API_DIR, 'transport.uts');
const ENVELOPE_UTS = path.join(API_DIR, 'envelope.uts');
const HELPERS_UTS = path.join(API_DIR, 'helpers.uts');
const AI_UTS = path.join(API_DIR, 'aiAssistant.uts');
const SSE_UTS = path.join(__dirname, 'sseIncrementalParser.uts');

/** 传输层真件（handleStatus / sanitizeForLog 都在这里） */
function loadTransport() {
  return transport();
}

/** 信封真件（decodeList / decodePage 的第二道兜底） */
function loadEnvelope() {
  const h = uts.loadUts(HELPERS_UTS, {});
  return uts.loadUts(ENVELOPE_UTS, envelopeBindings({ getMapped: () => Promise.resolve({}), toNumber: h.toNumber }));
}

/**
 * 走**真** handleStatus 解包：模拟 `api/request.uts` 的 `request()` 成功段
 * （`body` = `uni.request` 的 `res.data`，即后端原始 JSON）。
 */
function resolveThroughStatus(t, data) {
  const body = { code: 200, data: data };
  let out = null;
  let err = null;
  const handled = t.handleStatus(200, body, true, '请求失败', (v) => { out = v; }, (e) => { err = e; });
  return { handled: handled, out: out, err: err };
}

/**
 * 装载 `api/aiAssistant.uts`：出口 `getMapped` 走**真 handleStatus** 再交给 mapper，
 * 即 `request()` 的真实链路（不是把已解包对象直接喂 mapper —— 那样测不到本票的 bug）。
 * @param {unknown} rawData 后端 `data` 字段原样（裸数组 / 对象 / null）
 * @param {object} [transportModule] 变异后的传输层（缺省真件）
 * @param {string} [aiFile] 变异后的 aiAssistant.uts 路径（缺省真件）
 */
function loadAi(rawData, transportModule, aiFile) {
  const t = transportModule || loadTransport();
  const h = uts.loadUts(HELPERS_UTS, {});
  const env = loadEnvelope();
  const respond = (url, params, mapper) => new Promise((resolve, reject) => {
    const body = { code: 200, data: rawData };
    t.handleStatus(200, body, true, '请求失败', (v) => {
      try {
        resolve(mapper(v));
      } catch (e) {
        reject(e);
      }
    }, reject);
  });
  return uts.loadUts(aiFile || AI_UTS, {
    getMapped: respond,
    postMapped: respond,
    patchMapped: (url, data, mapper) => respond(url, data, mapper),
    del: () => Promise.resolve({}),
    uploadFile: () => Promise.resolve({}),
    renewOrExpire: () => Promise.resolve(false),
    AUTH_EXPIRED_MESSAGE: '登录已过期，请重新登录',
    decodeList: env.decodeList,
    decodePage: env.decodePage,
    toNumber: h.toNumber,
    toStr: h.toStr,
    toBool: h.toBool,
    API_BASE_URL: 'https://example.test/api',
    AI_REQUEST_TIMEOUT: 60000,
    STORAGE_KEY_TOKEN: 'auth_token',
    getStorage: () => 'access-token',
    createSseParser: uts.loadUts(SSE_UTS, {}).createSseParser,
  });
}

/** 读真源 → 注入变异 → 落临时目录 → 真执行（锚点不在源里即红） */
function mutateTransport(replacements) {
  let src = uts.readText(TRANSPORT_UTS);
  for (const pair of replacements) {
    expect([pair[0], src.includes(pair[0])]).toEqual([pair[0], true]);
    src = src.split(pair[0]).join(pair[1]);
  }
  const dir = fs.mkdtempSync(path.join(os.tmpdir(), 'bare-array-'));
  const out = path.join(dir, 'transport.uts');
  fs.writeFileSync(out, src);
  return uts.loadUts(out, {
    API_BASE_URL: 'http://127.0.0.1:8080',
    STORAGE_KEY_TOKEN: 'auth_token',
    getStorage: () => '',
    uni: { showToast: () => {} },
  });
}

/** 同上，作用于 `api/aiAssistant.uts`（第二道兜底的变异取证） */
function mutateAi(replacements) {
  let src = uts.readText(AI_UTS);
  for (const pair of replacements) {
    expect([pair[0], src.includes(pair[0])]).toEqual([pair[0], true]);
    src = src.split(pair[0]).join(pair[1]);
  }
  const dir = fs.mkdtempSync(path.join(os.tmpdir(), 'bare-array-ai-'));
  const out = path.join(dir, 'aiAssistant.uts');
  fs.writeFileSync(out, src);
  return out;
}

describe('handleStatus：裸数组归一（#2404 传输层）', () => {
  test('裸数组 ⇒ {list:[...]}（保留 App 语义，消费方零改动）', () => {
    const t = loadTransport();
    const arr = [{ id: 1 }, { id: 2 }];
    const r = resolveThroughStatus(t, arr);
    expect(r.handled).toBe(true);
    expect(r.err).toBe(null);
    expect(Array.isArray(r.out)).toBe(false);
    expect(r.out.list).toEqual(arr);
  });

  test('空数组同样归一为 {list:[]}（不是 {}、也不是 null）', () => {
    const t = loadTransport();
    const r = resolveThroughStatus(t, []);
    expect(r.out.list).toEqual([]);
  });

  test('对象响应行为不变：原样透传、不包 list', () => {
    const t = loadTransport();
    const obj = { levels: [{ id: 1 }], total: 1 };
    const r = resolveThroughStatus(t, obj);
    expect(r.out).toEqual(obj);
    expect(r.out.list).toBe(undefined);
  });

  test('data: null 仍归一为 {}（移动端 ADR-0037）', () => {
    const t = loadTransport();
    const r = resolveThroughStatus(t, null);
    expect(r.out).toEqual({});
  });
});

describe('sanitizeForLog：数组 / 对象判定不靠强转抛异常（#2404 同源缺陷）', () => {
  test('数组请求体原样返回（不被打码、也不被折成数字键对象）', () => {
    const t = loadTransport();
    const arr = [{ password: 'p' }, { password: 'q' }];
    const out = t.sanitizeForLog(arr);
    expect(Array.isArray(out)).toBe(true);
    expect(out).toEqual(arr);
  });

  test('对象请求体照旧打码（数组分支不误伤对象）', () => {
    const t = loadTransport();
    const out = t.sanitizeForLog({ username: 'alice', password: 'P@ssw0rd' });
    expect(out.username).toBe('alice');
    expect(out.password).toBe('***');
  });

  test('null 原样返回 null', () => {
    const t = loadTransport();
    expect(t.sanitizeForLog(null)).toBe(null);
  });
});

describe('extractArray：第二道兜底（data 本身是数组时直接返回）', () => {
  test('decodeList 收到裸数组 ⇒ 逐项映射，不再打 no-hit', () => {
    const env = loadEnvelope();
    const warns = [];
    const origWarn = console.warn;
    console.warn = (m) => { warns.push(m); };
    try {
      const got = env.decodeList([{ id: 1 }, { id: 2 }], '/x', (o) => ({ id: o['id'] }), ['sessions']);
      expect(got).toEqual([{ id: 1 }, { id: 2 }]);
      expect(warns).toEqual([]);
    } finally {
      console.warn = origWarn;
    }
  });

  test('decodePage 收到裸数组 ⇒ items 全在、total 回落 items.length', () => {
    const env = loadEnvelope();
    const got = env.decodePage([{ id: 1 }, { id: 2 }], '/x', 1, 10, (o) => ({ id: o['id'] }), ['items']);
    expect(got.items).toEqual([{ id: 1 }, { id: 2 }]);
    expect(got.total).toBe(2);
    expect(got.hasMore).toBe(false);
  });

  test('对象信封取键顺序零 diff（具名键与 list 兜底都照旧）', () => {
    const env = loadEnvelope();
    const byKey = env.decodeList({ sessions: [{ id: 7 }] }, '/x', (o) => ({ id: o['id'] }), ['sessions']);
    expect(byKey).toEqual([{ id: 7 }]);
    const byList = env.decodeList({ list: [{ id: 8 }] }, '/x', (o) => ({ id: o['id'] }), ['items', 'list']);
    expect(byList).toEqual([{ id: 8 }]);
  });
});

describe('四条消费路径：裸数组端到端取到元素（#2404 验收）', () => {
  test('getAiSessionsApi：/ai-assistant/sessions 裸数组 ⇒ 会话列表非空', async () => {
    const mod = loadAi([
      { id: 11, title: '液压系统异响', model_source: 'admin', model_id: 3, created_at: '2026-11-01', updated_at: '2026-11-02' },
    ]);
    const got = await mod.getAiSessionsApi('');
    expect(got.length).toBe(1);
    expect(got[0].id).toBe(11);
    expect(got[0].title).toBe('液压系统异响');
  });

  test('getAiSessionMessagesApi：/sessions/{id}/messages 裸数组 ⇒ 消息列表非空', async () => {
    const mod = loadAi([
      { id: 21, role: 'user', content: '叉车起升无力', created_at: '2026-11-01' },
      { id: 22, role: 'assistant', content: '先查液压油位', created_at: '2026-11-01' },
    ]);
    const got = await mod.getAiSessionMessagesApi(11);
    expect(got.length).toBe(2);
    expect(got[0].content).toBe('叉车起升无力');
    expect(got[1].role).toBe('assistant');
  });

  test('getAiModelsApi：/ai-assistant/models 裸数组 ⇒ 模型列表非空', async () => {
    const mod = loadAi([{ id: 1, name: '基座模型', model: 'j/base-1', source: 'admin' }]);
    const got = await mod.getAiModelsApi();
    expect(got.length).toBe(1);
    expect(got[0].model).toBe('j/base-1');
  });

  test('getAiUserModelsApi：/ai-assistant/user-models 裸数组 ⇒ 自定义模型列表非空', async () => {
    const mod = loadAi([{ id: 5, name: '自定义', base_url: 'https://api.test', model: 'u/custom', api_key: 'sk-x' }]);
    const got = await mod.getAiUserModelsApi();
    expect(got.length).toBe(1);
    expect(got[0].name).toBe('自定义');
    expect(got[0].base_url).toBe('https://api.test');
  });
});

/** 把 handleStatus 改回「cast 抛异常判形」（#2404 修复前的形态） */
function transportWithCastThrow() {
  return mutateTransport([[
    `    if (Array.isArray(data)) {
        // 保留 App 语义：裸数组统一包装为 { list: [...] }，消费方零改动
        resolve({ list: data } as UTSJSONObject)
        return true
    }
    resolve(data as UTSJSONObject)`,
    `    try {
        resolve(data as UTSJSONObject)
    } catch (e) {
        resolve({ list: data } as UTSJSONObject)
    }`,
  ]]);
}

/** 把 aiAssistant.uts 的第二道兜底摘掉（#2404 修复前的形态） */
function aiWithoutArrayGuard() {
  return mutateAi([[
    `	if (Array.isArray(data)) {
		const anyData : any = data
		return anyData as Array<UTSJSONObject>
	}
`,
    '',
  ]]);
}

describe('M1 必红变异：两层都退回修复前形态 ⇒ 四条路径重新变空', () => {
  test('裸数组原样透传 + 无第二道兜底时，sessions / messages / models / user-models 全部取不到元素', async () => {
    const brokenTransport = transportWithCastThrow();
    const brokenAi = aiWithoutArrayGuard();

    const sessions = await loadAi([{ id: 11, title: 't' }], brokenTransport, brokenAi).getAiSessionsApi('');
    const messages = await loadAi([{ id: 21, role: 'user', content: 'c' }], brokenTransport, brokenAi).getAiSessionMessagesApi(11);
    const models = await loadAi([{ id: 1, name: 'm', model: 'j/b' }], brokenTransport, brokenAi).getAiModelsApi();
    const userModels = await loadAi([{ id: 5, name: 'u', model: 'u/c' }], brokenTransport, brokenAi).getAiUserModelsApi();

    expect(sessions).toEqual([]);
    expect(messages).toEqual([]);
    expect(models).toEqual([]);
    expect(userModels).toEqual([]);
  });
});

describe('M2 纵深：传输层归一失效时，第二道兜底仍能取到元素', () => {
  test('只退回传输层（aiAssistant 的数组判定保留）⇒ 四条路径照常非空', async () => {
    const brokenTransport = transportWithCastThrow();

    const sessions = await loadAi([{ id: 11, title: 't' }], brokenTransport).getAiSessionsApi('');
    const messages = await loadAi([{ id: 21, role: 'user', content: 'c' }], brokenTransport).getAiSessionMessagesApi(11);
    const models = await loadAi([{ id: 1, name: 'm', model: 'j/b' }], brokenTransport).getAiModelsApi();
    const userModels = await loadAi([{ id: 5, name: 'u', model: 'u/c' }], brokenTransport).getAiUserModelsApi();

    expect(sessions.length).toBe(1);
    expect(messages.length).toBe(1);
    expect(models.length).toBe(1);
    expect(userModels.length).toBe(1);
  });
});
