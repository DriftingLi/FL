/**
 * 整卷考试入口的行为级测试（#2464，行为腿 / ③ 门第二腿）
 *
 * 票面判据是**行为**，不是文本：列表页点「模拟考试」后，考试页必须
 *   ① 认出卷参数就调「开整卷考试」出口（题集与卷时长由后端按卷给定）；
 *   ② 不再探「未完成考试」—— 卷考题集固定，没有「重新组卷」这个分支可用；
 *   ③ 后端拒绝（未解锁）时把后端原文交到 loadError，不残留半个考试。
 * 只读源码文本能证明「有一句 startPaperExamApi(paperId.value)」，证明不了
 * 「带卷参数时历史探活真的没发」「拒绝后 timer 真的没起来」—— 那只有跑一遍才看得见。
 *
 * 缝：utils/utsHarness.js 的 loadUts。注入 ref / computed（最小盒）与六个取数函数替身；
 * 被测的状态机（loadExam / boot / applyOptions / 计时与交卷）全部来自真源。
 */
const path = require('path');
const { loadUts } = require('./utsHarness');

const SESSION_UTS = path.join(__dirname, '..', 'pages', 'exam', 'composables', 'useMockExamSession.uts');

/** 最小 ref：composable 只用到 `.value` 读写 */
function miniRef(initial) {
  return { value: initial };
}

/** 最小 computed：只读 getter（session 里的 computed 只被读） */
function miniComputed(fn) {
  return {
    get value() {
      return fn();
    },
  };
}

const PAPER_QUESTIONS = [
  { question_id: 11, type: 'single', title: '卷题一', options: [{ key: 'A', text: '甲' }], score: 5 },
  { question_id: 12, type: 'single', title: '卷题二', options: [{ key: 'A', text: '乙' }], score: 5 },
];

/** 后端 StartPaperExam 的返回形状（mockexam.MockExamStartDTO） */
function paperExamInfo() {
  return { mock_exam_id: 501, questions: PAPER_QUESTIONS, duration: 60, remaining_time: 3600 };
}

/** 随机组卷的返回形状（同一 DTO，题集由后端随机） */
function randomExamInfo() {
  return { mock_exam_id: 601, questions: PAPER_QUESTIONS, duration: 90, remaining_time: 5400 };
}

const flush = () => new Promise((r) => setTimeout(r, 0));

/**
 * 装载被测 composable（真身）。
 * @param {{paperFails?:boolean, history?:any[], startInfo?:any}} scenario
 */
function loadSession(scenario) {
  const calls = { history: 0, start: [], paper: [], resume: [], intervals: 0 };
  const toasts = [];
  const modals = [];
  const mod = loadUts(SESSION_UTS, {
    ref: miniRef,
    computed: miniComputed,
    startMockExamApi: (id, duration) => {
      calls.start.push([id, duration]);
      return Promise.resolve(scenario.startInfo || randomExamInfo());
    },
    resumeMockExamApi: (id) => {
      calls.resume.push(id);
      return Promise.resolve(scenario.startInfo || randomExamInfo());
    },
    saveMockExamApi: () => Promise.resolve({}),
    submitMockExamApi: () => Promise.resolve({}),
    getMockExamHistoryApi: () => {
      calls.history += 1;
      return Promise.resolve({ items: scenario.history || [] });
    },
    startPaperExamApi: (paperId) => {
      calls.paper.push(paperId);
      // 未解锁：后端 ErrRealPaperNotRedeemed 的原文（backend/internal/realexam/service.go:60）
      if (scenario.paperFails) return Promise.reject(new Error('请先兑换该真题卷'));
      return Promise.resolve(scenario.paperInfo || paperExamInfo());
    },
    getTypeName: () => '单选题',
    isMultiChoiceType: () => false,
    // 计时器替身：只判「有没有起来」，不让它真的跑（否则倒计时会在断言之间偷偷减 1）
    setInterval: () => {
      calls.intervals += 1;
      return 1;
    },
    clearInterval: () => {},
    uni: {
      showToast: (opts) => toasts.push(opts.title),
      showModal: (opts) => modals.push(opts),
      showLoading: () => {},
      hideLoading: () => {},
      redirectTo: () => {},
    },
  });
  return { session: mod.useMockExamSession(), calls, toasts, modals };
}

describe('整卷考试入口（#2464）', () => {
  it('applyOptions 收卷参数：paperId 落地，mode 仍是 new（卷考没有 resume 语义）', () => {
    const { session } = loadSession({});
    session.applyOptions({ paper: '77' });
    expect(session.paperId.value).toBe(77);
    expect(session.mode.value).toBe('new');
  });

  it('带卷参数进页即开考：只调开整卷考试出口，题集与时长用后端给的，不查历史、不随机组卷', async () => {
    const { session, calls } = loadSession({});
    session.applyOptions({ paper: '77' });
    await session.boot();
    // boot 的卷考分支沿用既有 `loadExam()` 不 await 的写法（与尾部同形），等一拍让装载落定
    await flush();
    expect(calls.paper).toEqual([77]);
    expect(calls.history).toBe(0);
    expect(calls.start).toEqual([]);
    expect(session.mockExamId.value).toBe(501);
    expect(session.questions.value.length).toBe(2);
    expect(session.remainingTime.value).toBe(3600); // 卷时长由后端给定，端上不传 90 分钟
    expect(session.loadError.value).toBe('');
    expect(calls.intervals).toBe(1);
  });

  it('后端拒绝（未解锁）时把后端原文交到 loadError，不残留半个考试、不起计时', async () => {
    const { session, calls } = loadSession({ paperFails: true });
    session.applyOptions({ paper: '77' });
    await session.boot();
    await flush();
    expect(session.loadError.value).toContain('请先兑换该真题卷');
    expect(session.mockExamId.value).toBe(0);
    expect(calls.intervals).toBe(0);
  });

  it('无卷参数：未完成询问仍在，选「开始新考试」⇒ 随机组卷（既有行为不变）', async () => {
    const { session, calls, modals } = loadSession({ history: [{ mock_exam_id: 88, status: 'in_progress' }] });
    await session.boot();
    expect(calls.history).toBe(1);
    expect(calls.paper).toEqual([]);
    expect(modals.length).toBe(1);
    expect(modals[0].content).toContain('继续上一场考试');
    modals[0].success({ confirm: false, cancel: true });
    await flush();
    expect(calls.start).toEqual([[0, 90]]);
    expect(calls.resume).toEqual([]);
  });

  it('无卷参数：选「继续考试」⇒ 续考既有链路（未完成询问的确认分支不被卷参数挤掉）', async () => {
    const { session, calls, modals } = loadSession({ history: [{ mock_exam_id: 88, status: 'in_progress' }] });
    await session.boot();
    modals[0].success({ confirm: true, cancel: false });
    await flush();
    expect(session.mode.value).toBe('resume');
    expect(calls.resume).toEqual([88]);
    expect(calls.paper).toEqual([]);
  });

  it('无卷参数且无历史：直接随机组卷（原口径，卷参数不介入）', async () => {
    const { session, calls } = loadSession({});
    await session.boot();
    expect(calls.history).toBe(1);
    expect(calls.start).toEqual([[0, 90]]);
    expect(calls.paper).toEqual([]);
  });
});
