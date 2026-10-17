/**
 * PR-E #1665 · aiAssistant 裸出口收口 + `getAiModelsApi` 吞错点删除 —— **行为级**守护（成对取证）
 *
 * ## 它在守什么
 *
 * 本票把 `api/aiAssistant.uts` 的 8 处裸 `get/post/patch().then` 出口收口至 mapper 家族
 * （`getMapped` / `postMapped` / `requestMapped`，ADR-0037 ④.5），并同批删掉
 * ADR-0037 ③ 点名的最后一处现网吞错点 `api/aiAssistant.uts` 的
 * `.catch(() => [])`（另两处 forum / mockExam 已随 PR-A / PR-D 清零）。
 * 源码文本证明不了「失败真的会上抛」——本文件把模块**真跑起来**
 * （`utsHarness.loadUts` + 替身真的回调 mapper），断言只有跑起来才看得见的语义：
 *
 *   1. **装载语义不变**（对照）—— `models` 载荷逐项经 `buildAiModel` 映射，字段读取逐字保留。
 *   2. **吞错点已删** —— 出口失败原样上抛；页面（ai-assistant / ai-settings 的 loadModels /
 *      loadData 各自 try/catch）落 `[]` 的兜底发生在 UI 侧，不在 api 层静默吞。
 *   3. **M1 必红变异** —— 把 `.catch(() => [])` 加回去，失败重新折成空列表：证明第 2 条
 *      的断言真能判红，不是恒绿。
 *
 * fail-closed 由 utsHarness 兜底：若出口退化回裸 `get`，`getMapped` 缺绑定当场红。
 * 接线面由 `envelopeAdoptionContract`（棘轮）与各 contract 文件守；本文件只兜行为。
 */
const fs = require('fs');
const os = require('os');
const path = require('path');

const { loadUts } = require('./utsHarness');

const API_DIR = path.join(__dirname, '..', 'api');
const HELPERS_UTS = path.join(API_DIR, 'helpers.uts');
const ENVELOPE_UTS = path.join(API_DIR, 'envelope.uts');
const AI_UTS = path.join(API_DIR, 'aiAssistant.uts');

/** 真源：helpers 是零依赖纯函数（手抄 toStr/toNumber 正是漂移起点） */
const helpers = () => loadUts(HELPERS_UTS, {});

/** 信封模块真实现：getAiUserModelsApi 的回退内仍可能走本域 decode 语义（缺绑定 = fail-closed） */
function envelopeFor(h) {
  return loadUts(ENVELOPE_UTS, { getMapped: () => Promise.resolve({}), toNumber: h.toNumber });
}

/**
 * 装载 aiAssistant.uts（imports：getMapped/postMapped/requestMapped/del/uploadFile +
 * renewOrExpire/AUTH_EXPIRED_MESSAGE + decodeList/decodePage + toNumber/toStr + env/storage 三常量；
 * 出口退回裸 get ⇒ 缺绑定当场红）
 * @param {object} raw 下一次 outlet 的应答载荷（经 mapper 之后再返回）
 * @param {{ reject?: Error, file?: string }} opt reject = 注入出口失败；file = 变异后的临时文件
 */
function loadAi(raw, opt = {}) {
  const h = helpers();
  const env = envelopeFor(h);
  const respond = (url, params, mapper) => {
    if (opt.reject) return Promise.reject(opt.reject);
    return Promise.resolve(mapper(raw));
  };
  const mod = loadUts(opt.file || AI_UTS, {
    getMapped: respond,
    postMapped: respond,
    requestMapped: (o, mapper) => respond(o.url, null, mapper),
    del: () => (opt.reject ? Promise.reject(opt.reject) : Promise.resolve({})),
    uploadFile: () => Promise.resolve({}),
    // SSE 401 的单点决策（#1704）：本套件不测它们，但缺绑定 = fail-closed
    renewOrExpire: () => Promise.resolve(false),
    AUTH_EXPIRED_MESSAGE: '登录已过期，请重新登录',
    decodeList: env.decodeList,
    decodePage: env.decodePage,
    toNumber: h.toNumber,
    toStr: h.toStr,
    API_BASE_URL: 'https://example.test/api',
    AI_REQUEST_TIMEOUT: 60000,
    STORAGE_KEY_TOKEN: 'auth_token',
    getStorage: () => 'access-token',
  });
  return { mod };
}

/**
 * 读真源 → 注入变异 → 落到临时目录 → 真执行（不改工作树，不进仓）；
 * 锚点断言先行：替换串不在源里就当场红 —— 变异必须真的进去了，测试才有意义。
 */
function mutateUts(file, replacements, outName) {
  let src = fs.readFileSync(file, 'utf8');
  for (const [from, to] of replacements) {
    expect([from, src.includes(from)]).toEqual([from, true]);
    src = src.split(from).join(to);
  }
  const dir = fs.mkdtempSync(path.join(os.tmpdir(), 'ai-assistant-swallow-'));
  const out = path.join(dir, outName);
  fs.writeFileSync(out, src);
  return out;
}

describe('getAiModelsApi：收口后的装载语义（PR-E #1665）', () => {
  it('对照：models 载荷逐项经 buildAiModel 映射（字段读取不变；url 逐字）', async () => {
    const { mod } = loadAi({
      models: [
        { id: 1, name: '基座模型', model: 'j/base-1', source: 'admin', description: '默认' },
        { model_id: 2, display_name: '自定义', model_name: 'u/custom', model_source: 'user' },
      ],
    });
    const got = await mod.getAiModelsApi();
    expect(got).toEqual([
      { id: 1, name: '基座模型', model: 'j/base-1', source: 'admin', description: '默认' },
      { id: 2, name: '自定义', model: 'u/custom', source: 'user', description: '' },
    ]);
  });

  it('吞错点已删：出口失败原样上抛（ADR-0037 ③ 点名清单清零；页面 try/catch 在 UI 侧兜）', async () => {
    const { mod } = loadAi({}, { reject: new Error('network down') });
    await expect(mod.getAiModelsApi()).rejects.toThrow('network down');
  });

  it('M1 必红变异：把 .catch(() => []) 加回去 → 失败重新折成空列表（断言真能判红）', async () => {
    const file = mutateUts(AI_UTS, [[
      "	})\n	// 原 `.catch(() => [])` 吞错兜底已删",
      "	}).catch((_e) : AiModel[] => { return [] as AiModel[] })\n	// 原 `.catch(() => [])` 吞错兜底已删",
    ]], 'aiAssistant.uts');
    const { mod } = loadAi({}, { file, reject: new Error('network down') });
    await expect(mod.getAiModelsApi()).resolves.toEqual([]);
  });
});
