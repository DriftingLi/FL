/**
 * #2406 随机练习取不到题目 —— **行为级**守护（站点级取键兜底）
 *
 * ## 它在守什么
 *
 * 后端 `GET /practice-mode/free` 的 `data` 是**裸数组**（`array<QuestionDTO>`，见 swagger.json）。
 * 自 #2404 起传输层 `handleStatus` 显式判形并把裸数组归一为 `{list:[...]}`，两种运行时
 * （App / 小程序）拿到的是**同一形状**。而本站点当初只写了一个候选键 `['questions']`：
 * `{list}` 里没有 `questions` ⇒ `extractArray` 全落空 ⇒ 打 `[envelope] no-hit` 并返回空数组 ⇒
 * 页面无题可答、也不抛错（所以连 toast 都没有，与「两端都空」的现象一致）。
 *
 * 本文件把**真链路**跑起来（真 `envelope.uts` 的 `decodeList` + 真 `practice.uts` 的 mapper），
 * 断言只有跑起来才看得见的语义：
 *
 *   1. `{list:[...]}`（传输层归一后的**真实**形状）⇒ 取到题目（修复前必红）；
 *   2. `{items:[...]}` ⇒ 取到题目（修复前必红）；
 *   3. 裸数组直喂 mapper ⇒ 取到题目（#2404 的 `extractArray` 第二道兜底，**已绿**，只作不回归断言）；
 *   4. 后端裸数组走**真** `handleStatus` ⇒ 取到题目（端到端复刻「随机练习」这条症状）；
 *   5. 本域语义键优先：`questions` 与兜底键同时在场时取 `questions`；
 *   6. 同域两条对象信封路径（`/practice-mode/sequential`、`/tag`）行为零 diff；
 *   7. M1 必红变异：候选键退回只剩 `questions` ⇒ 1、2 两条重新落空 —— 证明断言真能判红。
 *
 * 为什么**不能**只靠「裸数组」那条当红用例（#2406 验收口径 Q2 已改写）：`extractArray` 自 #2404
 * 起对裸数组直接返回，不动 `practice.uts` 它也绿 ⇒ 它没有鉴别力，只能作不回归断言。
 */
const fs = require('fs');
const os = require('os');
const path = require('path');

const uts = require('./utsHarness');
const { transport, envelopeBindings } = require('./requestUtsHarness');

const API_DIR = path.join(__dirname, '..', 'api');
const PRACTICE_UTS = path.join(API_DIR, 'practice.uts');
const HELPERS_UTS = path.join(API_DIR, 'helpers.uts');
const ENVELOPE_UTS = path.join(API_DIR, 'envelope.uts');

/** 信封真件（decodeList 的取键与裸数组兜底都在这里） */
function loadEnvelope() {
  const h = uts.loadUts(HELPERS_UTS, {});
  return uts.loadUts(ENVELOPE_UTS, envelopeBindings({ getMapped: () => Promise.resolve({}), toNumber: h.toNumber }));
}

/** 两条 `/practice-mode/free` 题目样本（字段取后端 QuestionDTO 的真名） */
function sampleQuestions() {
  return [
    { id: 101, type: 'single', content: '叉车起升无力的首要排查项是？', options: { A: '液压油位', B: '轮胎气压' }, answer: 'A', explanation: '先查油位' },
  ];
}

/**
 * 装载 `api/practice.uts`：`getMapped` 把**已经解包**的 data 直接交给 mapper。
 * 用来精确复刻「传输层归一之后 mapper 收到什么形状」。
 * @param {unknown} decoded 传输层归一后的形状（`{list:[...]}` / `{items:[...]}` / 裸数组）
 * @param {string} [practiceFile] 变异副本路径（缺省真件）
 */
function loadPractice(decoded, practiceFile) {
  const h = uts.loadUts(HELPERS_UTS, {});
  const env = loadEnvelope();
  const respond = (url, params, mapper) => new Promise((resolve, reject) => {
    try {
      resolve(mapper(decoded));
    } catch (e) {
      reject(e);
    }
  });
  return uts.loadUts(practiceFile || PRACTICE_UTS, {
    get: () => Promise.resolve({}),
    post: () => Promise.resolve({}),
    getMapped: respond,
    postMapped: respond,
    decodeList: env.decodeList,
    decodePage: env.decodePage,
    toNumber: h.toNumber,
    toStr: h.toStr,
    toBool: h.toBool,
  });
}

/**
 * 装载 `api/practice.uts` 并走**真** `handleStatus`：入参是后端 `data` 的**原始**形态，
 * 传输层归一（裸数组 → `{list}`）真的发生，再交给 mapper。这就是线上链路。
 * @param {unknown} rawData 后端 `data` 字段原样（裸数组 / 对象）
 */
function loadPracticeThroughTransport(rawData) {
  const h = uts.loadUts(HELPERS_UTS, {});
  const env = loadEnvelope();
  const t = transport();
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
  return uts.loadUts(PRACTICE_UTS, {
    get: () => Promise.resolve({}),
    post: () => Promise.resolve({}),
    getMapped: respond,
    postMapped: respond,
    decodeList: env.decodeList,
    decodePage: env.decodePage,
    toNumber: h.toNumber,
    toStr: h.toStr,
    toBool: h.toBool,
  });
}

/** 跑一段被测调用并收集 `console.warn`（`no-hit` 是有效信号，不该在本站点被静默吞掉） */
async function withWarnings(fn) {
  const warns = [];
  const origWarn = console.warn;
  console.warn = (m) => { warns.push(m); };
  try {
    const value = await fn();
    return { value, warns };
  } finally {
    console.warn = origWarn;
  }
}

describe('getFreePracticeApi：站点级取键兜底（#2406）', () => {
  test('{list:[...]}（传输层归一后的真实形状）⇒ 取到题目', async () => {
    const api = loadPractice({ list: sampleQuestions() });
    const { value, warns } = await withWarnings(() => api.getFreePracticeApi());
    expect(value.length).toBe(1);
    expect(value[0].question_id).toBe(101);
    expect(value[0].title).toBe('叉车起升无力的首要排查项是？');
    expect(warns).toEqual([]);
  });

  test('{items:[...]} ⇒ 取到题目', async () => {
    const api = loadPractice({ items: sampleQuestions() });
    const { value, warns } = await withWarnings(() => api.getFreePracticeApi());
    expect(value.length).toBe(1);
    expect(value[0].question_id).toBe(101);
    expect(warns).toEqual([]);
  });

  test('裸数组直喂 mapper ⇒ 取到题目（#2404 第二道兜底，不回归断言）', async () => {
    const api = loadPractice(sampleQuestions());
    const { value, warns } = await withWarnings(() => api.getFreePracticeApi());
    expect(value.length).toBe(1);
    expect(warns).toEqual([]);
  });

  test('真链路：后端裸数组经 handleStatus 归一 ⇒ 取到题目（「随机练习」端到端）', async () => {
    const api = loadPracticeThroughTransport(sampleQuestions());
    const { value, warns } = await withWarnings(() => api.getFreePracticeApi());
    expect(value.length).toBe(1);
    expect(value[0].question_id).toBe(101);
    expect(warns).toEqual([]);
  });

  test('本域语义键优先：questions 与兜底键同时在场时取 questions', async () => {
    const api = loadPractice({ questions: sampleQuestions(), list: [], items: [] });
    const { value } = await withWarnings(() => api.getFreePracticeApi());
    expect(value.length).toBe(1);
    expect(value[0].question_id).toBe(101);
  });
});

describe('同域对象信封路径零 diff（sequential / tag 后端 data 是对象）', () => {
  test('getSequentialPracticeApi：{questions:[...]} ⇒ 取到题目', async () => {
    const api = loadPractice({ questions: sampleQuestions(), total: 1 });
    const got = await api.getSequentialPracticeApi();
    expect(got.length).toBe(1);
    expect(got[0].question_id).toBe(101);
  });

  test('getTagPracticeApi：{questions:[...]} ⇒ 取到题目', async () => {
    const api = loadPractice({ questions: sampleQuestions(), total: 1 });
    const got = await api.getTagPracticeApi(7);
    expect(got.length).toBe(1);
    expect(got[0].question_id).toBe(101);
  });
});

/** 把 `api/practice.uts` 的候选键退回只剩 `questions`（#2406 修复前的形态） */
function practiceWithoutFallbackKeys() {
  const src = uts.readText(PRACTICE_UTS);
  const anchor = "['questions', 'items', 'list']";
  expect([anchor, src.includes(anchor)]).toEqual([anchor, true]);
  const dir = fs.mkdtempSync(path.join(os.tmpdir(), 'practice-free-'));
  const out = path.join(dir, 'practice.uts');
  fs.writeFileSync(out, src.split(anchor).join("['questions']"));
  return out;
}

describe('M1 必红变异：候选键退回只剩 questions ⇒ 兜底形状重新落空', () => {
  test('{list} / {items} 在该变异下均取不到题目（证明上面两条断言有鉴别力）', async () => {
    const broken = practiceWithoutFallbackKeys();
    const byList = await withWarnings(() => loadPractice({ list: sampleQuestions() }, broken).getFreePracticeApi());
    const byItems = await withWarnings(() => loadPractice({ items: sampleQuestions() }, broken).getFreePracticeApi());
    expect(byList.value).toEqual([]);
    expect(byItems.value).toEqual([]);
    expect(byList.warns.length + byItems.warns.length).toBeGreaterThan(0);
  });
});
