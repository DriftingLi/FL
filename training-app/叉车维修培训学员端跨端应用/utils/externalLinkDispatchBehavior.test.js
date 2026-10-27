/**
 * 外链点击编排（`utils/externalLinkDispatch.uts`）· **行为级**套件（票 #2035；ADR-0025 ⑩-8/⑩-9）
 *
 * 为什么这是 ③ 门承重证据而不是接线：`docs/agents/guards.md` 判据 ① —— 把编排改坏（摘掉空链接闸 /
 * 摘掉确认闸），本套件必须判红；组 ④ 用注入变异的副本**实测**这条有牙性，不是口头承诺。
 *
 * ## 缝与口径（写实，不假装）
 * - 缝 = `utils/utsHarness.js` 的 `loadUts`（先例 `.forumBodyBehavior` / `.markdownInlineBehavior`）：
 *   具名 import 被剥除，`forumLinkDomain` / `openExternalBrowser` / `uni` 全以假实现注入 ——
 *   本套件验的是**编排**（弹窗次数与字样 / 确认闸 / 副作用落点），不是任何真实端效果。
 * - `#ifdef` 在 node 缝里是惰性注释 ⇒ confirm 后**两支同时执行**（直开与复制都会记录到）。
 *   端分叉的**排他性**是编译期事实，不在行为缝射程，由 `systemBrowserWiringContract.test.js` 钉
 *   区域归属（guards.md 判据 ② 的口径在这里明写，免得读者把双支同响当病灶）。
 * - 域名投影本身自有行为面（`markdownInlineBehavior` ④/域名组、`forumBodyBehavior`）：
 *   这里注入 marker 函数，断言只落在「被透传进弹窗文案」，不重测投影算法。
 *
 * ## 成对取证（③ 门判据 ③）
 * 组 ①③ 是真源（必不红），组 ④ 是两条坏实现副本（必红）：摘空链接闸 ⇒ 空链接也弹；
 * 摘确认闸 ⇒ 取消也执行副作用。各配一条与真源相反的观测，证明 ①③ 的判据有牙。
 */
const fs = require('fs');
const os = require('os');
const path = require('path');

const { loadUts, readText } = require('./utsHarness');

const DISPATCH_UTS = path.join(__dirname, 'externalLinkDispatch.uts');

/** 假端环境：记录全部副作用调用；showModal 按 confirm 旗回调 success（同步回调，与真端时序一致） */
function fakeUni(opts) {
  const rec = { modals: [], opened: [], clipboard: [], toasts: [] };
  const uni = {
    showModal: (o) => {
      rec.modals.push({ title: o.title, content: o.content, confirmText: o.confirmText, cancelText: o.cancelText });
      o.success({ confirm: opts.confirm === true });
    },
    setClipboardData: (o) => { rec.clipboard.push(o.data); o.success(); },
    showToast: (o) => { rec.toasts.push(o.title); },
  };
  return {
    rec,
    bindings: {
      uni,
      forumLinkDomain: (u) => 'DOM(' + u + ')',
      openExternalBrowser: (u) => { rec.opened.push(u); },
    },
  };
}

/** 真执行一次点击（每次全新模块实例，模块级状态不串） */
const tap = (link, opts) => {
  const f = fakeUni(opts);
  loadUts(DISPATCH_UTS, f.bindings).dispatchExternalLinkTap(link);
  return f.rec;
};

// ===== ① 空链接闸 =====

describe('① 空链接 ⇒ 零副作用：连弹窗都不许出（UGC 正文不被脏 run 惊扰）', () => {
  it('confirm=true 的空链接仍然全程静默（闸在弹窗之前，不在确认之后）', () => {
    const rec = tap('', { confirm: true });
    expect([rec.modals.length, rec.opened.length, rec.clipboard.length, rec.toasts.length]).toEqual([0, 0, 0, 0]);
  });
});

// ===== ② 确认弹窗面（⑩-8 摩擦面本身） =====

describe('② 弹窗字样与按钮是 ⑩-8 的口径：显示目标域名、打开/取消具名', () => {
  const rec = tap('https://e.com/m', { confirm: false });
  it('恰好一次弹窗；正文 = 前缀 + 域名投影（透传）；按钮具名', () => {
    expect(rec.modals.length).toBe(1);
    const m = rec.modals[0];
    expect([m.title, m.content]).toEqual(['打开外部链接', '即将离开应用访问：DOM(https://e.com/m)']);
    expect([m.confirmText, m.cancelText]).toEqual(['打开', '取消']);
  });
});

// ===== ③ 确认闸与副作用落点 =====

describe('③ 取消 ⇒ 零副作用；确认 ⇒ 两支各自的落点收到原链接', () => {
  it('取消：弹窗出过，但既没直开也没复制', () => {
    const rec = tap('https://e.com/a', { confirm: false });
    expect([rec.modals.length, rec.opened.length, rec.clipboard.length, rec.toasts.length]).toEqual([1, 0, 0, 0]);
  });
  it('确认：直开支收到原串；复制支收到原串并弹「链接已复制」（node 缝双支同执，口径见文件头）', () => {
    const rec = tap('https://e.com/a', { confirm: true });
    expect(rec.opened).toEqual(['https://e.com/a']);
    expect(rec.clipboard).toEqual(['https://e.com/a']);
    expect(rec.toasts).toEqual(['链接已复制']);
  });
});

// ===== ④ 成对取证：判据有牙 =====

/** 读真源 → 注入变异 → 落临时目录 → 真执行（不改工作树，不进仓；先例 markdownInlineBehavior） */
function loadMutated(replacements) {
  let src = readText(DISPATCH_UTS);
  for (const [from, to] of replacements) {
    expect([from, src.includes(from)]).toEqual([from, true]);  // 锚点失效即红：变异必须真的进去了
    src = src.split(from).join(to);
  }
  const dir = fs.mkdtempSync(path.join(os.tmpdir(), 'ext-link-'));
  const file = path.join(dir, 'externalLinkDispatch.uts');
  fs.writeFileSync(file, src);
  return file;
}

describe('④ 成对取证（③ 门判据③）：组 ①③ 的判据有牙', () => {
  it('必不红（对照组）：真源上「取消零副作用」成立 —— 变异判红之前先证明夹具没坏', () => {
    const rec = tap('https://e.com/a', { confirm: false });
    expect([rec.opened.length, rec.clipboard.length]).toEqual([0, 0]);
  });
  it('必红 · 摘掉空链接闸 ⇒ 空链接也弹窗（组 ① 在真源上正是判它 0 次）', () => {
    const file = loadMutated([['    if (link.length == 0) return', '    if (link.length < 0) return']]);
    const f = fakeUni({ confirm: false });
    loadUts(file, f.bindings).dispatchExternalLinkTap('');
    expect(f.rec.modals.length).toBe(1);   // 真源上是 0 —— 组 ① 断言正是这里会红
  });
  it('必红 · 摘掉确认闸 ⇒ 取消也执行副作用（组 ③ 在真源上正是判它 0 次）', () => {
    const file = loadMutated([['            if (!res.confirm) return', '            if (res.confirm == null) return']]);
    const f = fakeUni({ confirm: false });
    loadUts(file, f.bindings).dispatchExternalLinkTap('https://e.com/a');
    expect(f.rec.opened.length).toBe(1);   // 真源上是 0 —— 组 ③ 断言正是这里会红
  });
});
