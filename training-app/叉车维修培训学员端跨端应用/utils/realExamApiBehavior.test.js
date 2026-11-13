/**
 * #2461 历年真题 api 出口行为守护（spec #2459；票 #2461 AC⑦）
 *
 * 被测对象 `api/realExam.uts` 的四个出口 —— 用 `loadUts` 把真源码跑起来，把
 * `getMapped` / `postMapped` 换成记录器，断言的是**发给后端的 method + 路径**与
 * **从后端响应里映射出来的字段**，不断言内部实现。
 *
 * 三条要点（都来自票面口径）：
 *   ① 列卷响应是**裸数组**（后端 `httpx.Endpoint[…, []RealExamPaperDTO]`）；
 *      传输层归一后成 `{ list: [...] }`，两种形状都必须取到卷 —— 故这里两种都测。
 *   ② `year` / `source` 是 `omitempty`：缺省时端上收成 0 与空串，
 *      由展示层决定「年份未知」/「真题」（`utils/realExamPapersDisplay.uts`）。
 *   ③ 兑换结果**只映射字段**，不作「已兑换」判据（ADR-0024 禁字符串比对）。
 *
 * 最后一组是变异取证：把 entitled 映射改成硬编码 false，断言确实翻面。
 */
const fs = require('fs');
const os = require('os');
const path = require('path');

const uts = require('./utsHarness');
const { transport, envelopeBindings } = require('./requestUtsHarness');

const API_DIR = path.join(__dirname, '..', 'api');
const REAL_EXAM_UTS = path.join(API_DIR, 'realExam.uts');
const HELPERS_UTS = path.join(API_DIR, 'helpers.uts');
const ENVELOPE_UTS = path.join(API_DIR, 'envelope.uts');

const PAPER = {
  paper_id: 12,
  title: '2024 年叉车维修真题（卷一）',
  year: 2024,
  source: '内部真题',
  question_count: 50,
  duration_minutes: 60,
  entitled: true,
  price: 300,
};

/** 后端 omitempty 的极端形态：year / source 整个键不在 */
const PAPER_BARE = {
  paper_id: 13,
  title: '未标注年份的卷',
  question_count: 20,
  duration_minutes: 30,
  entitled: false,
  price: 300,
};

const PRACTICE = {
  questions: [{ id: 77, type: 'single', content: '题干', options: { A: '甲', B: '乙' }, score: 2 }],
  current_index: 3,
  total: 50,
  completed: 3,
};

const EXAM = {
  mock_exam_id: 91,
  duration: 60,
  remaining_time: 1800,
  total_score: 100,
  total_questions: 50,
  questions: [{ id: 77, type: 'single', content: '题干', options: { A: '甲' } }],
};

const REDEEM = { balance: 700, total_earned: 1000, sku: 'unlock_real_paper', ref_id: 'paper:12' };

/** 装真 envelope（getPaged 的 query 合并依赖传输层，故走 envelopeBindings） */
function loadEnvelope() {
  const h = uts.loadUts(HELPERS_UTS, {});
  return uts.loadUts(ENVELOPE_UTS, envelopeBindings({
    getMapped: () => Promise.resolve({}),
    toNumber: h.toNumber,
  }));
}

/**
 * 装载 api/realExam.uts：把 getMapped/postMapped 换成记录器。
 * @param {(url : string) => unknown} payloadByUrl 按 url 给**已解包**的 data
 */
function harness(payloadByUrl, file) {
  const calls = [];
  const mk = (method) => (url, data, mapper) => new Promise((resolve, reject) => {
    calls.push({ method: method, url: url, data: data });
    try {
      resolve(mapper(payloadByUrl(url)));
    } catch (e) {
      reject(e);
    }
  });
  const h = uts.loadUts(HELPERS_UTS, {});
  const env = loadEnvelope();
  const api = uts.loadUts(file || REAL_EXAM_UTS, {
    getMapped: mk('GET'),
    postMapped: mk('POST'),
    decodeList: env.decodeList,
    toNumber: h.toNumber,
    toStr: h.toStr,
    toBool: h.toBool,
  });
  return { calls: calls, api: api };
}

/** 经**真传输层**解包（复刻线上链路：裸数组由 transport 归一成 { list: [...] }） */
function loadThroughTransport(rawData) {
  const t = transport();
  const h = uts.loadUts(HELPERS_UTS, {});
  const env = loadEnvelope();
  const respond = (url, data, mapper) => new Promise((resolve, reject) => {
    const body = { code: 200, data: rawData };
    t.handleStatus(200, body, true, '请求失败', (v) => {
      try {
        resolve(mapper(v));
      } catch (e) {
        reject(e);
      }
    }, reject);
  });
  return uts.loadUts(REAL_EXAM_UTS, {
    getMapped: respond,
    postMapped: respond,
    decodeList: env.decodeList,
    toNumber: h.toNumber,
    toStr: h.toStr,
    toBool: h.toBool,
  });
}

/** 变异：真源码按 pairs 替换后写临时目录（判别力取证） */
function mutateRealExam(pairs) {
  let src = uts.readText(REAL_EXAM_UTS);
  for (const pair of pairs) {
    expect([pair[0], src.includes(pair[0])]).toEqual([pair[0], true]);
    src = src.split(pair[0]).join(pair[1]);
  }
  const dir = fs.mkdtempSync(path.join(os.tmpdir(), 'real-exam-api-'));
  const file = path.join(dir, 'realExam.uts');
  fs.writeFileSync(file, src);
  return file;
}

describe('路由与方法：四条出口对齐后端已注册清单（AC⑦）', () => {
  it('listPapersApi → GET /real-exam/papers（无 query，证件分区走服务端兜底）', async () => {
    const h = harness(() => [PAPER]);
    await h.api.listPapersApi();
    expect(h.calls).toEqual([{ method: 'GET', url: '/real-exam/papers', data: null }]);
  });

  it('startPaperPracticeApi → GET /real-exam/papers/:id/practice', async () => {
    const h = harness(() => PRACTICE);
    await h.api.startPaperPracticeApi(7);
    expect(h.calls).toEqual([{ method: 'GET', url: '/real-exam/papers/7/practice', data: null }]);
  });

  it('startPaperExamApi → POST /real-exam/papers/:id/exam（空 body）', async () => {
    const h = harness(() => EXAM);
    await h.api.startPaperExamApi(7);
    expect(h.calls).toEqual([{ method: 'POST', url: '/real-exam/papers/7/exam', data: null }]);
  });

  it('redeemPaperApi → POST /real-exam/papers/:id/redeem（空 body）', async () => {
    const h = harness(() => REDEEM);
    await h.api.redeemPaperApi(7);
    expect(h.calls).toEqual([{ method: 'POST', url: '/real-exam/papers/7/redeem', data: null }]);
  });
});

describe('listPapersApi：卷列表 DTO 手写映射（AC③④⑦）', () => {
  it('裸数组响应逐字段映射', async () => {
    const h = harness(() => [PAPER]);
    const papers = await h.api.listPapersApi();
    expect(papers.length).toBe(1);
    expect(papers[0]).toEqual(PAPER);
  });

  it('year / source 缺省收成 0 与空串（展示层再决定「年份未知」/「真题」）', async () => {
    const h = harness(() => [PAPER_BARE]);
    const papers = await h.api.listPapersApi();
    expect(papers[0].year).toBe(0);
    expect(papers[0].source).toBe('');
    expect(papers[0].entitled).toBe(false);
    expect(papers[0].price).toBe(300);
    expect(papers[0].title).toBe('未标注年份的卷');
    expect(papers[0].question_count).toBe(20);
    expect(papers[0].duration_minutes).toBe(30);
  });

  it('{ list: [...] } / { items: [...] } 两种信封都取到卷', async () => {
    const byList = harness(() => ({ list: [PAPER] }));
    expect((await byList.api.listPapersApi()).length).toBe(1);
    const byItems = harness(() => ({ items: [PAPER] }));
    expect((await byItems.api.listPapersApi()).length).toBe(1);
  });

  it('空响应不抛，给空数组', async () => {
    const h = harness(() => []);
    expect(await h.api.listPapersApi()).toEqual([]);
  });

  it('经真传输层：裸数组归一后端到端取到卷', async () => {
    const api = loadThroughTransport([PAPER]);
    const papers = await api.listPapersApi();
    expect(papers.length).toBe(1);
    expect(papers[0].paper_id).toBe(12);
    expect(papers[0].entitled).toBe(true);
  });
});

describe('startPaperPracticeApi：按卷练习会话 DTO（AC⑦）', () => {
  it('题目走同一 QuestionDTO 映射（id → question_id、content → title、options Map → 数组）', async () => {
    const h = harness(() => PRACTICE);
    const session = await h.api.startPaperPracticeApi(7);
    expect(session.current_index).toBe(3);
    expect(session.total).toBe(50);
    expect(session.completed).toBe(3);
    expect(session.questions.length).toBe(1);
    expect(session.questions[0].question_id).toBe(77);
    expect(session.questions[0].title).toBe('题干');
    expect(session.questions[0].type).toBe('single');
    expect(session.questions[0].options).toEqual([{ key: 'A', text: '甲' }, { key: 'B', text: '乙' }]);
  });

  it('questions 缺省给空数组（不抛）', async () => {
    const h = harness(() => ({ current_index: 0, total: 0, completed: 0 }));
    const session = await h.api.startPaperPracticeApi(7);
    expect(session.questions).toEqual([]);
  });
});

describe('startPaperExamApi：整卷考试 DTO（AC⑦）', () => {
  it('映射 mock_exam_id / duration / remaining_time 与题集', async () => {
    const h = harness(() => EXAM);
    const info = await h.api.startPaperExamApi(7);
    expect(info.mock_exam_id).toBe(91);
    expect(info.duration).toBe(60);
    expect(info.remaining_time).toBe(1800);
    expect(info.questions.length).toBe(1);
    expect(info.questions[0].question_id).toBe(77);
    expect(info.questions[0].title).toBe('题干');
  });
});

describe('redeemPaperApi：兑换结果只映射字段（ADR-0024）', () => {
  it('balance / total_earned / sku / ref_id', async () => {
    const h = harness(() => REDEEM);
    const result = await h.api.redeemPaperApi(7);
    expect(result).toEqual(REDEEM);
  });
});

describe('变异取证：entitled 映射必须有牙', () => {
  it('把 entitled 改成硬编码 false ⇒ 已解锁卷读成未解锁', async () => {
    const mutated = mutateRealExam([["entitled: toBool(obj['entitled'])", 'entitled: false']]);
    const h = harness(() => [PAPER], mutated);
    const papers = await h.api.listPapersApi();
    expect(papers[0].entitled).toBe(false);
  });
});
