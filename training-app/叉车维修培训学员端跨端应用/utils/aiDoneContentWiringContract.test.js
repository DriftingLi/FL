/**
 * #2310 · `done.content` 的**接线契约**（读源文本、断言语义）
 *
 * 行为那半在 `utils/aiDoneContentBehavior.test.js`（端到端真执行）。这里守的是
 * 「接线没断」：`StreamChatCallbacks.onDone` 必须带可空载荷参数（改回 `() => void`
 * 就等于又把后端发的 content 挡在门外），且 `useAiChat` 的 done 分支必须读
 * `data['content']` 并整体替换、不是追加。
 *
 * 断言纪律（沿用 #921 评审修正）：函数体一律限在大括号配平的切片内；
 * 每条「检测器有没有牙」用手工变异样本自证 —— 存在性断言是本仓踩过的假绿。
 */
const path = require('path');

const { readText } = require('./utsHarness');
const ROOT = path.join(__dirname, '..');
const read = (p) => readText(path.join(ROOT, p));

const API = read('api/aiAssistant.uts');
const CHAT = read('composables/useAiChat.uts');

/** 取函数体（大括号配平）—— 防止切片越界到文件尾造成假绿 */
function fnBody(src, sig) {
  const start = src.indexOf(sig);
  if (start < 0) throw new Error('找不到源码片段: ' + sig);
  const open = src.indexOf('{', start);
  if (open < 0) throw new Error('片段后找不到 { : ' + sig);
  let depth = 0;
  for (let i = open; i < src.length; i++) {
    if (src[i] === '{') depth += 1;
    else if (src[i] === '}') {
      depth -= 1;
      if (depth === 0) return src.slice(open, i + 1);
    }
  }
  throw new Error('大括号未配平: ' + sig);
}

describe('#2310 done.content 接线契约', () => {
  it('StreamChatCallbacks.onDone 带可空 UTSJSONObject 载荷（不是无参回调）', () => {
    const t = fnBody(API, 'export type StreamChatCallbacks');
    expect(t).toMatch(/onDone \?: \(data : UTSJSONObject \| null\) => void/);
    expect(t).not.toMatch(/onDone \?: \(\) => void/);
  });

  it('done 分支解析 data 并把载荷交给 onDone（不是只调 onDone()）', () => {
    const t = fnBody(API, 'function handleStreamSSEEvent');
    const done = t.slice(t.indexOf("event == 'done'"));
    expect(done).toMatch(/JSON\.parse\(data\)/);
    expect(done).toMatch(/callbacks\.onDone\(payload\)/);
  });

  it('useAiChat 的 done 回调读 data[content] 并整体替换 assistantMsg.content', () => {
    const t = fnBody(CHAT, 'async function doStreamChat');
    expect(t).toMatch(/onDone: \(data : UTSJSONObject \| null\) =>/);
    expect(t).toMatch(/data\['content'\]/);
    expect(t).toMatch(/assistantMsg\.content = full/);
    // 是替换不是追加
    expect(t).not.toMatch(/assistantMsg\.content \+=.*full/);
  });

  it('30 秒固定兜底计时器已删（等待态只在请求结束时收尾，#2298）', () => {
    const t = fnBody(CHAT, 'async function doStreamChat');
    expect(t).not.toMatch(/30000/);
  });

  it('检测器有牙：把 onDone 改回无参回调必须判红', () => {
    const t = fnBody(API, 'export type StreamChatCallbacks');
    const broken = t.replace(
      'onDone ?: (data : UTSJSONObject | null) => void',
      'onDone ?: () => void'
    );
    expect(broken).not.toBe(t); // 变异必须真落上
    expect(broken).not.toMatch(/onDone \?: \(data : UTSJSONObject \| null\) => void/);
  });
});
