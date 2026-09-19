/**
 * 空态可执行性契约（#1062）—— 源码契约测试缝（读源文本、断言语义；与 aiFeatureEntryWiringContract /
 * aiProGateContract 同款）。
 *
 * 钉的不变式是**文案的可执行性**，不是某一句原文：平台模型为空时，通用页给学员的那句话必须
 * **指向一个真实可走的动作** —— 既要有对所有人都成立的路径（联系管理员），点名的应用内入口
 * 也必须在 UI 里真实存在。
 *
 * 为什么要钉（来源 #1062 ← #1044 的 ❓Q2 裁定「要硬化」）：#1044 终态裁定「通用对话接受不可用」
 * ⇒ 平台模型为空是**长期状态**，那句 toast 是学员**长期**看到的文案。改前它是
 * `暂无可用的 AI 模型` —— 学员既不知道发生了什么，也没有任何入口可走（#1044 原话：
 * 「提示语不指向任何可执行动作」）。
 *
 * 为什么门槛是「联系管理员 + 真实入口名」两条、而不是钉死整句原文（#1062 票面「建议（推荐做）」
 * 那段的要求：别让守护退化成文案快照）：自定义模型的入口与配置面**都在专业版门内** —— 未解锁时
 * `RIGHT_MENU_ITEMS` 过滤掉 `custom-models`（`ai-assistant.uvue` 的 `rightMenuItems`）、设置页的
 * user / custom 来源整行不渲染（#1024）⇒ 只写「去配置自定义模型」对未解锁学员是死路一条。
 * 文案因此必须**先**给出对谁都可执行的路径；这与 ADR-0008「守护从断言源码文本改为断言行为」同向。
 * 措辞（措辞可改）与这两条语义（语义不可撤）分开对待。
 *
 * 断言纪律沿用 #921 评审修正：函数体一律用 `fnBody()` 限在函数体内（大括号配平），
 * 且「检测器有没有牙」用手工变形样本自证（④）。
 *
 * 已知边界（如实声明）：契约测试只钉客户端这一半；真机门与本地编译门（移动端 `docs/adr/0008`
 * 的 ① / ④）另行执行并在 PR 证据段如实记录。
 */
const path = require('path');

/** 读源码一律经共享读者归一 EOL（ADR-0019）：与检出平台无关，Windows CRLF 也免疫。 */
const { readText } = require('./utsHarness');
const ROOT = path.join(__dirname, '..');
const read = (p) => readText(path.join(ROOT, p));

const PAGE = read('pages/ai-assistant/ai-assistant.uvue');
const CONSTANTS = read('pages/ai-assistant/ai-assistant-constants.uts');

/** 取某个函数体（大括号配平，切到闭合处为止）——防止切片越界到文件尾造成假绿（沿用 #921 的 fnBody）。 */
function fnBody(src, sig) {
  const start = src.indexOf(sig);
  if (start < 0) throw new Error(`找不到源码片段: ${sig}`);
  const open = src.indexOf('{', start);
  if (open < 0) throw new Error(`片段后找不到 '{': ${sig}`);
  let depth = 0;
  for (let i = open; i < src.length; i++) {
    if (src[i] === '{') depth++;
    else if (src[i] === '}') {
      depth--;
      if (depth === 0) return src.slice(open, i + 1);
    }
  }
  throw new Error(`大括号未配平: ${sig}`);
}

/**
 * 「平台模型为空」那道守卫弹的 toast 文案。
 * 锚点取**守卫条件**（`models.value.length == 0`）、不取文案本身 —— 否则每次改文案都要改守护，
 * 守护会退化成文案快照（#1062 票面「建议（推荐做）」那段）。
 */
function platformModelGuardMessage(src) {
  const fn = fnBody(src, 'function onInputSend');
  const m = /if \(models\.value\.length == 0[\s\S]*?uni\.showToast\(\{ title: '([^']*)'/.exec(fn);
  return m ? m[1] : null;
}

/** 右侧菜单里「对话设置」那一项的 label（真源：常量表 `RIGHT_MENU_ITEMS`）。 */
function settingsMenuLabel(src) {
  const m = /key: 'settings',\s*label: '([^']+)'/.exec(src);
  return m ? m[1] : null;
}

/**
 * 文案是否「指向可执行动作」：① 给出对所有人都成立的路径（联系管理员）；
 * ② 点名的应用内入口带书名号，且该名字**真实存在**（否则改名即静默失效）。
 */
function pointsToAction(msg, label) {
  if (typeof msg !== 'string' || msg.length === 0) return false;
  if (msg.indexOf('联系管理员') < 0) return false;
  if (!label || label.length === 0) return false;
  return msg.indexOf(`「${label}」`) >= 0;
}

describe('平台模型为空的空态可执行性契约（#1062）', () => {
  const label = settingsMenuLabel(CONSTANTS);

  it('① 前置：文案点名的入口在右侧菜单里真实存在（「对话设置」= settings 项的 label）', () => {
    expect(label).toBe('对话设置');
  });

  it('② 通用页「平台模型为空」守卫的文案指向可执行动作（管理员路径 + 真实入口名）', () => {
    const msg = platformModelGuardMessage(PAGE);
    expect(msg).toBeTruthy();
    // 判据逐条给：失败时能一眼看出缺的是哪一半
    expect(msg).toContain('联系管理员');
    expect(msg).toContain(`「${label}」`);
    expect(pointsToAction(msg, label)).toBe(true);
  });

  it('③ 旧文案（不可执行的裸提示）不得残留', () => {
    expect(platformModelGuardMessage(PAGE)).not.toBe('暂无可用的 AI 模型');
  });

  it('④ 字数上限：文本区是**两行 × 18 字**，超了末字会被吞（①a 真机实测）', () => {
    // 实测（2026-09-16，Redmi 23049RAD8C / b32d8398）：首版 37 字的文案，末字「型」**不渲染**
    // （OCR 三帧一致，每行恰好 18 字）⇒ 上限 36，留余量按 ≤36 判。
    // 这条是「指向可执行动作」的**必要配套**：字数超了，句子就会被吃掉结尾 —— 同样不可执行。
    expect(platformModelGuardMessage(PAGE).length).toBeLessThanOrEqual(36);
  });

  it('⑤ 检测器自检（变异）：把文案换回旧裸提示后必须判红 —— 检测器有牙', () => {
    const mutated = PAGE.replace(
      /(if \(models\.value\.length == 0[\s\S]*?uni\.showToast\(\{ title: ')[^']*(')/,
      '$1暂无可用的 AI 模型$2'
    );
    // 注入自检：替换必须真的落到守卫那句上，否则本条自检本身是假绿
    expect(mutated).not.toBe(PAGE);
    const msg = platformModelGuardMessage(mutated);
    expect(msg).toBe('暂无可用的 AI 模型');
    expect(pointsToAction(msg, label)).toBe(false);
  });
});
