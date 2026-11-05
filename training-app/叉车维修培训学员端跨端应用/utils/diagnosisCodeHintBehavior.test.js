/**
 * 故障码体检的**五态映射与防抖** · 行为级测试（#2297，行为腿）
 *
 * 票面两条判据要求「真执行」：
 *   ① 「状态 → 展示什么」的映射有真执行的行为测试 —— 读源码只能证明「有个 status == 'known'
 *      分支」，证明不了 unknown / none 真的**整行不渲染**；那种「多渲染一行」的回归只有把五种
 *      状态各喂一遍才看得见（#1273 先例：断言源码字面量 ⇒ 实现改了照样绿）。
 *   ② 防抖有行为层用例 —— 判据是「连续敲键不逐字发请求」，只有真起计时器、真数请求次数才能观测。
 *
 * 缝：`utils/utsHarness.js` 的 `loadUts`（把 `.uts` 去掉类型当 JS 真跑）。注入两样替身：
 *   ① `ref` —— 最小 reactive 盒（本仓没有 `vue` npm 包，composable 只用到 `ref(x).value`）；
 *   ② 取数函数 —— 由用例按场景给结果、抛错或挂起。
 * 被测的**映射与状态机不替身**：来自 `utils/diagnosisCodeHint.uts` 与
 * `composables/useDiagnosisCodeHint.uts` 的真实导出（手抄镜像会让「真源改了而镜子没改」静默变绿）。
 *
 * 计时器用 jest 假计时器：composable 里的 `setTimeout`/`clearTimeout` 解析到 Node 全局，
 * 假计时器一装就接管 —— 于是「250ms 前一发不发」「连敲只发最后一发」都是**数出来的**。
 */
const path = require('path');
const { loadUts, readText } = require('./utsHarness');

const HINT_UTS = path.join(__dirname, 'diagnosisCodeHint.uts');
const COMPOSABLE_UTS = path.join(__dirname, '..', 'composables', 'useDiagnosisCodeHint.uts');
const PAGE_UVUE = path.join(__dirname, '..', 'pages', 'ai-assistant', 'ai-feature.uvue');
const INPUT_UVUE = path.join(__dirname, '..', 'components', 'ai-chat', 'ai-chat-input.uvue');
const API_UTS = path.join(__dirname, '..', 'api', 'aiAssistant.uts');

/** 最小 ref：composable 只用到 `.value` 读写 */
function miniRef(initial) {
  return { value: initial };
}

/** 装载真源：映射模块 + 被它依赖的 composable（映射与常量全部来自真源导出） */
function loadHintModule() {
  const hint = loadUts(HINT_UTS, {});
  const mod = loadUts(COMPOSABLE_UTS, {
    ref: miniRef,
    hintViewOf: hint.hintViewOf,
    emptyHintView: hint.emptyHintView,
    CODE_HINT_DEBOUNCE_MS: hint.CODE_HINT_DEBOUNCE_MS,
    CODE_HINT_MIN_LEN: hint.CODE_HINT_MIN_LEN,
  });
  return { hint, mod };
}

/** 固定返回同一结果的取数替身，顺带记下每次查询的 `q` */
function fixedFetch(calls, result) {
  return (q) => {
    calls.push(q);
    return Promise.resolve(result);
  };
}

/** 冲掉 `.then/.catch` 的微任务（一发查询回来要跨两层） */
async function flush() {
  await Promise.resolve();
  await Promise.resolve();
  await Promise.resolve();
}

const KNOWN = {
  status: 'known',
  message: 'E01 主接触器故障：检查接触器线圈与线束',
  fix: 'E01',
  samples: [],
};

beforeEach(() => {
  jest.useFakeTimers();
});

afterEach(() => {
  jest.useRealTimers();
});

describe('五态 → 展示什么（纯函数真执行）', () => {
  const { hint } = loadHintModule();

  test('known：一行说明 + 可点追问（追问句用规范码，不带输入里的手误）', () => {
    const v = hint.hintViewOf(KNOWN, 'e01');
    expect(v.visible).toBe(true);
    expect(v.kind).toBe('known');
    expect(v.text).toBe(KNOWN.message);
    expect(v.followUp).toBe('E01 这个故障码是什么故障、怎么排查？');
  });

  test('known 但上游没给 message/fix：用学员输入的原文兜住，不渲染空行', () => {
    const v = hint.hintViewOf({ status: 'known', message: '', fix: '', samples: [] }, ' E41 ');
    expect(v.visible).toBe(true);
    expect(v.text).toBe('已收录这个故障码：E41');
    expect(v.followUp).toBe('E41 这个故障码是什么故障、怎么排查？');
  });

  test('typo：指出该改成什么（message 不含 fix 时补上）', () => {
    const v = hint.hintViewOf({ status: 'typo', message: '没有这个码', fix: 'E01', samples: [] }, 'EO1');
    expect(v.visible).toBe(true);
    expect(v.kind).toBe('typo');
    expect(v.text).toBe('没有这个码，应该是 E01');
    expect(v.followUp).toBe('');
  });

  test('typo：message 已经含 fix 时不重复「应该是 E01」', () => {
    const v = hint.hintViewOf({ status: 'typo', message: '你是不是想输 E01？', fix: 'E01', samples: [] }, 'EO1');
    expect(v.text).toBe('你是不是想输 E01？');
  });

  test('typo：只有 fix 没有 message 时也能说清', () => {
    const v = hint.hintViewOf({ status: 'typo', message: '', fix: 'E01', samples: [] }, 'EO1');
    expect(v.text).toBe('是不是要输入 E01？');
  });

  test('format：message + 最多 3 条写法示例', () => {
    const v = hint.hintViewOf(
      { status: 'format', message: '格式不对', fix: '', samples: ['E01', 'E41', 'E101', 'E999'] },
      'e01x',
    );
    expect(v.visible).toBe(true);
    expect(v.kind).toBe('format');
    expect(v.text).toBe('格式不对，可这样填：E01 / E41 / E101');
  });

  test('format：没有 samples 时只给 message；没有 message 时有默认句子', () => {
    expect(hint.hintViewOf({ status: 'format', message: '请按 E01 的写法填', fix: '', samples: [] }, 'x').text)
      .toBe('请按 E01 的写法填');
    expect(hint.hintViewOf({ status: 'format', message: '', fix: '', samples: [] }, 'x').text)
      .toBe('格式不对');
  });

  test('unknown / none / 未来新增态 / 取数失败：整行不渲染', () => {
    [
      { status: 'unknown', message: '', fix: '', samples: [] },
      { status: 'none', message: '', fix: '', samples: [] },
      { status: 'something-new', message: '新态', fix: '', samples: [] },
      null,
    ].forEach((h) => {
      const v = hint.hintViewOf(h, 'E01');
      expect(v.visible).toBe(false);
      expect(v.text).toBe('');
      expect(v.followUp).toBe('');
    });
  });
});

describe('防抖与过期丢弃（真起计时器）', () => {
  test('停手 250ms 才发查询；连敲 3 次只发最后一发', async () => {
    const { mod } = loadHintModule();
    const calls = [];
    const state = mod.useDiagnosisCodeHint(fixedFetch(calls, KNOWN));

    state.onInput('E');
    state.onInput('E0');
    state.onInput('E01');
    expect(calls).toEqual([]); // 还没到点，一次都没发

    jest.advanceTimersByTime(249);
    expect(calls).toEqual([]);

    jest.advanceTimersByTime(1);
    await flush();
    expect(calls).toEqual(['E01']); // 逐字敲 = 只发最后一发
    expect(state.view.value.visible).toBe(true);
    expect(state.view.value.kind).toBe('known');
  });

  test('太短（< 2 字）不发查询，也不渲染', async () => {
    const { mod } = loadHintModule();
    const calls = [];
    const state = mod.useDiagnosisCodeHint(fixedFetch(calls, KNOWN));

    state.onInput('E');
    jest.advanceTimersByTime(1000);
    await flush();
    expect(calls).toEqual([]);
    expect(state.view.value.visible).toBe(false);
  });

  test('输入一变，旧提示立刻消失（不能拿上一段的说明解释这一段）', async () => {
    const { mod } = loadHintModule();
    const calls = [];
    const state = mod.useDiagnosisCodeHint(fixedFetch(calls, KNOWN));

    state.onInput('E01');
    jest.advanceTimersByTime(250);
    await flush();
    expect(state.view.value.visible).toBe(true);

    state.onInput('E012');
    expect(state.view.value.visible).toBe(false);
  });

  test('清空输入 / clear()：提示立即消失，未到点的那一发作废', async () => {
    const { mod } = loadHintModule();
    const calls = [];
    const state = mod.useDiagnosisCodeHint(fixedFetch(calls, KNOWN));

    state.onInput('E01');
    jest.advanceTimersByTime(250);
    await flush();
    expect(state.view.value.visible).toBe(true);

    state.onInput('');
    expect(state.view.value.visible).toBe(false);

    state.onInput('E01');
    state.clear();
    jest.advanceTimersByTime(1000);
    await flush();
    expect(calls).toEqual(['E01']); // 只有第一次那发，clear 后不再补发
  });

  test('在途结果乱序返回：只认最新一句的答案', async () => {
    const { mod } = loadHintModule();
    const calls = [];
    const pending = [];
    const state = mod.useDiagnosisCodeHint((q) => {
      calls.push(q);
      return new Promise((resolve) => {
        pending.push({ q, resolve });
      });
    });

    state.onInput('E01');
    jest.advanceTimersByTime(250); // 第 1 发在途
    state.onInput('E012');
    jest.advanceTimersByTime(250); // 第 2 发在途
    expect(calls).toEqual(['E01', 'E012']);

    // 新的一发先回来：展示它
    pending[1].resolve({ status: 'known', message: 'E012 说明', fix: 'E012', samples: [] });
    await flush();
    expect(state.view.value.text).toBe('E012 说明');

    // 旧的一发后回来：必须被丢掉，不能把 E01 的说明盖到 E012 上
    pending[0].resolve({ status: 'known', message: 'E01 说明', fix: 'E01', samples: [] });
    await flush();
    expect(state.view.value.text).toBe('E012 说明');
  });

  test('查询失败静默：不抛、不渲染，输入不受影响', async () => {
    const { mod } = loadHintModule();
    const state = mod.useDiagnosisCodeHint(() => Promise.reject(new Error('502')));

    state.onInput('E01');
    jest.advanceTimersByTime(250);
    await flush();
    expect(state.view.value.visible).toBe(false);
  });

  test('dispose() 取消未到点的计时器（页面卸载后不再落地）', async () => {
    const { mod } = loadHintModule();
    const calls = [];
    const state = mod.useDiagnosisCodeHint(fixedFetch(calls, KNOWN));

    state.onInput('E01');
    state.dispose();
    jest.advanceTimersByTime(1000);
    await flush();
    expect(calls).toEqual([]);
  });
});

describe('接线（行为测试自己搭依赖，接线断了它照样绿 ⇒ 这里守接线本身）', () => {
  test('页面：诊断页才挂提示行，点行回填追问句，卸载时 dispose', () => {
    const page = readText(PAGE_UVUE);
    expect(page).toContain('useDiagnosisCodeHint((q : string) : Promise<DiagnosisCodeHint> => checkDiagnosisCodeApi(q))');
    expect(page).toContain('v-if="isDiagnosis && codeHintView.visible"');
    expect(page).toContain('@click="onCodeHintTap"');
    expect(page).toContain('@text-change="onDraftInput"');
    expect(page).toContain('codeHintView.followUp.length > 0');
    expect(page).toContain('onUnload(() => {');
    expect(page).toContain('disposeCodeHint()');
    // 「把内容拼进输入框」的落点：追问句写回 draftText（替换，不自动发送）
    expect(page).toContain('draftText.value = followUp');
  });

  test('输入组件：每次输入都把当前内容抛给页面', () => {
    const input = readText(INPUT_UVUE);
    expect(input).toContain("emit('text-change', inputText.value)");
    expect(input).toContain('text-change');
  });

  test('API：走后端代理的 code-hint 端点', () => {
    const api = readText(API_UTS);
    expect(api).toContain("'/ai-assistant/diagnosis/code-hint'");
    expect(api).toContain('checkDiagnosisCodeApi');
  });
});
