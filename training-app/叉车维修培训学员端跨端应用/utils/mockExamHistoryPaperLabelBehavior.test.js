/**
 * 规格 #2459「历年真题接入」用户故事 22 的收尾守护：整卷考试在模考历史里带「真题卷」标。
 *
 * 后端 MockExamHistoryItemDTO.paper_id 是 omitempty：按卷开考的条目带卷 id，
 * 随机模考的响应里**没有这个键**（随机模考与真题卷考试共用同一张 mock_exam 表）。
 * 故这里断言的是两件事：
 *   ① 有卷 → 映射出 paper_id；无键 → 收成 0（页面据此不渲染标签）；
 *   ② 历史页确实以 paper_id 为准渲染「真题卷」标，且不改既有「通过 / 未通过」标。
 *
 * 用真源码跑（utsHarness），只替掉传输层，断言的是出口行为不是内部实现。
 */
const fs = require('fs');
const path = require('path');

const uts = require('./utsHarness');
const { envelopeBindings } = require('./requestUtsHarness');

const API_DIR = path.join(__dirname, '..', 'api');
const HELPERS_UTS = path.join(API_DIR, 'helpers.uts');
const ENVELOPE_UTS = path.join(API_DIR, 'envelope.uts');
const MOCK_EXAM_UTS = path.join(API_DIR, 'mockExam.uts');
const PAGE_UTS = path.join(__dirname, '..', 'pages', 'profile', 'mock-exam-records.uvue');

/** 装载 api/mockExam.uts：getPaged 换成直接喂 mapper 的桩（页/大小/键位都按真调用形状取）。 */
function loadHistory(rawItems) {
  const h = uts.loadUts(HELPERS_UTS, {});
  const env = uts.loadUts(ENVELOPE_UTS, envelopeBindings({
    getMapped: () => Promise.resolve({}),
    toNumber: h.toNumber,
  }));
  return uts.loadUts(MOCK_EXAM_UTS, {
    getMapped: () => Promise.resolve({}),
    post: () => Promise.resolve({}),
    postMapped: () => Promise.resolve({}),
    toNumber: h.toNumber,
    toStr: h.toStr,
    toBool: h.toBool,
    decodePage: env.decodePage,
    getPaged: (...args) => {
      const mapper = args.find((a) => typeof a === 'function');
      return Promise.resolve({
        items: rawItems.map((it) => mapper(it)),
        total: rawItems.length,
        page: 1,
        page_size: 10,
      });
    },
  });
}

const RAW_PAPER_EXAM = { id: 7, status: 'completed', duration: 60, remaining_time: 3000, paper_id: 12 };
const RAW_RANDOM_EXAM = { id: 8, status: 'completed', duration: 90, remaining_time: 1200 };

describe('模考历史 paper_id（规格 #2459 US22，真题卷来源标识）', () => {
  it('按卷开考的条目：paper_id 映射为卷 id', async () => {
    const mod = loadHistory([RAW_PAPER_EXAM]);
    const got = await mod.getMockExamHistoryApi(1, 10);
    expect(got.items[0].paper_id).toBe(12);
    expect(got.items[0].mock_exam_id).toBe(7);
  });

  it('随机模考的条目：响应无 paper_id 键 ⇒ 收成 0', async () => {
    const mod = loadHistory([RAW_RANDOM_EXAM]);
    const got = await mod.getMockExamHistoryApi(1, 10);
    expect(got.items[0].paper_id).toBe(0);
  });

  it('历史页以 paper_id 为准渲染「真题卷」标（0 不渲染）', () => {
    const src = fs.readFileSync(PAGE_UTS, 'utf8');
    expect(src).toContain('v-if="item.paper_id > 0"');
    expect(src).toContain('真题卷');
    // 既有通过 / 未通过标不变（同一行共存，不是替换）
    expect(src).toContain("item.passed ? '通过' : '未通过'");
  });
});
