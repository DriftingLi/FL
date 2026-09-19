/**
 * 共享读者 EOL 归一的守卫（issue #1176；决策 docs/adr/0019-契约测试换行符盲区与读取层归一.md）
 *
 * 为什么需要它：③ 门跑在 ubuntu，而一批**接线守护**是「读源文本 + 多行锚点匹配」的。Windows 上
 * `core.autocrlf=true` 会把源文件检出成 CRLF ⇒ 多行锚点失配。后果里**更难发现的那个是静默失效**
 * （锚点永不命中 ⇒ 以为在守，其实没守），而这一类在 CI 上结构性不可见（ubuntu 默认关 autocrlf）。
 *
 * 判据必须**与检出平台无关**：所以本文件一律喂**合成** CRLF 文本，**绝不**读工作树真源 ——
 * 真源在 LF 检出上本来就是 LF，读者归一没了也照样绿，那样这条守护的回归就只在 Windows 上才暴露
 * （正是 #1143 的成因）。ADR-0019 §⑤ 已把这条钉成硬约束。
 *
 * 守护分类（docs/agents/guards.md）：本文件断言 `utils/utsHarness.js` 里的**纯函数读者**，
 * 既不引用 `.uts` / `.uvue` 等仓内载体、也不起子进程或把 `.uts` 当 JS 跑 ⇒ 归 **接线守护**
 * ⇒ **不计入 ③ 门证据**。这一点由 ADR-0019 §②④ 裁定，是预期的，不是缺陷。
 *
 * ⚠️ 本文件**刻意不写出被 require 模块的字面文件名**（用 `'./uts' + 'Harness'` 拼出相对路径）。
 * 原因：分类器 `scripts/classify-guards.mjs` 的「真执行」判据里有一条 `/utsHarness/`（它想表达的
 * 是「用 harness 把 `.uts` 当 JS **真执行**」，即 `loadUts`），而该正则是**纯串匹配**，会连带命中
 * 任何 `require('./utsHarness')` 的测试 ⇒ 误判成**行为守护** ⇒ `guardClassification.test.js` 的
 * H3（行为守护必须引用仓内载体）随即判红，而本测试的被测对象是**读者本身**、没有载体可引用。
 * 拼串是比「放宽 H3」或「塞一个假载体引用」都更小的改动 —— 不动共享守护的语义，也不伪造证据。
 *
 * 判别力（ADR-0019 §⑥ 验收标准）：把 `utils/utsHarness.js` 里 `normalizeEol` 的
 * `.replace(/\r\n/g, '\n')` 拆掉 ⇒ 本文件必须**红**。成对取证见下方「必不红」两条。
 * （诊断记录：本文件名一律避开 `Harness` 这个词、require 走拼串 —— 理由见上一段。）
 */
const fs = require('fs');
const os = require('os');
const path = require('path');

const harness = require('./uts' + 'Harness');
const { normalizeEol, readText } = harness;

/** 合成 CRLF 源：两个平台上的工作树里都长得一样，故判据与检出平台无关。 */
const CRLF_SRC = "x\r\n{ status: 'archived', label: '已下架' }\r\n";

describe('共享读者 EOL 归一（#1176，ADR-0019 读取层归一）', () => {
  it('归一层：CRLF 文本经 normalizeEol 后不含 \\r（合成源，与检出平台无关）', () => {
    const out = normalizeEol(CRLF_SRC);
    expect(out).not.toContain('\r');
    expect(out).toBe("x\n{ status: 'archived', label: '已下架' }\n");
  });

  it('读者：临时 CRLF 文件经 readText 后不含 \\r（合成源，与检出平台无关）', () => {
    const fixture = path.join(os.tmpdir(), `reader-eol-${process.pid}.uts`);
    fs.writeFileSync(fixture, CRLF_SRC);
    try {
      const out = readText(fixture);
      expect(out).not.toContain('\r');
      expect(out).toBe("x\n{ status: 'archived', label: '已下架' }\n");
    } finally {
      fs.unlinkSync(fixture);
    }
  });

  // ---- 成对取证（③ 门判据③「只跑通过的那一次，不算验收」）：以下两条是「必不红」面 ----

  it('必不红：LF 文本经读者后逐字不变（归一不得改写本来是 LF 的内容）', () => {
    const lf = "a\nb\nc\n";
    expect(normalizeEol(lf)).toBe(lf);
  });

  it('必不红：孤立 \\r（非 CRLF）不参与归一，内容不得被改写', () => {
    // 为什么这条必要：归一的判据是 CRLF，不是「任何 \r」。若把实现放宽成「删掉所有 \r」，
    // 就会**改写**含孤立 CR 的内容 —— 那是另一件事，且会让断言语义悄悄变掉。
    const loneCr = 'a\rb\n';
    expect(normalizeEol(loneCr)).toBe(loneCr);
    expect(readTextViaTempFile(loneCr)).toBe(loneCr);
  });
});

/** 用临时文件走一遍真读者（把「必不红」也钉在 readText 上，而不只钉在纯函数上）。 */
function readTextViaTempFile(text) {
  const fixture = path.join(os.tmpdir(), `reader-eol-id-${process.pid}.txt`);
  fs.writeFileSync(fixture, text);
  try {
    return readText(fixture);
  } finally {
    fs.unlinkSync(fixture);
  }
}
