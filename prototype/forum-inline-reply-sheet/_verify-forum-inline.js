// PROTOTYPE 自查（一次性）：从 HTML 抽纯逻辑段，在 node 里跑不变式。输出 ASCII 避免终端编码干扰。
const fs = require('fs');
const path = require('path');
const html = fs.readFileSync(path.join(__dirname, 'forum-inline-and-reply-sheet.html'), 'utf8');

const start = html.indexOf('var MEMBER_HEADING=');
const end = html.indexOf('/* ═══════════ ⑩-8 链接确认弹窗');   // 含 renderBlocks/renderText（都不碰 DOM）
const fsStart = html.indexOf('function surfaceFragments(html){');
const fsEnd = html.indexOf('var AC_LEAK=');
if (start < 0 || end < 0 || fsStart < 0 || fsEnd < 0) { console.error('extract anchors not found'); process.exit(1); }
const factory = new Function(html.slice(start, end) + html.slice(fsStart, fsEnd) +
  '\nreturn {parseMarkdown, forumInlineRuns, plainFromRuns, plainOf, projectPlainText, stripInlinePlainJs, inlineSafeUrl, collectMathSpans, renderBlocks, surfaceFragments};');
const M = factory();

// ── 计数与打印（放在末尾的「输出」段）──
let fail = 0;
function say(msg){ console.log(msg); }
function check0(pass, msg){ if(!pass) fail++; console.log((pass?'PASS ':'FAIL ')+'INV-0  '+msg); }

// 现状生产算法（无 ~~ 规则、无 flanking）——用来标出「本批会改变摘要产物的地方」
function productionPlain(text) {
  let s = text;
  s = s.replace(/!\[([^\]]*)\]\([^)]*\)/g, '$1');
  s = s.replace(/\[([^\]]*)\]\([^)]*\)/g, '$1');
  s = s.replace(/\*\*([^*]+)\*\*/g, '$1');
  s = s.replace(/__([^_]+)__/g, '$1');
  s = s.replace(/\*([^*]+)\*/g, '$1');
  s = s.replace(/_([^_]+)_/g, '$1');
  s = s.replace(/`([^`]+)`/g, '$1');
  return s;
}
// 成对记号才算泄漏；裸记号字符（作者真意）允许可见
const MARKER = /\*\*[^*]+\*\*|~~[^~]+~~|`[^`]+`|\[[^\]]*\]\([^)]*\)/g;

const P = [];
function check(id, desc, pass, detail) {
  P.push({ id, desc, pass, detail });
}

// ── 语料：每行 (id, markdown, 期望) ──
const SRC = {
  nested:    '**粗体里有 [链接](https://x.com/a) 和 `码`** 与 *斜体 ~~删~~*',
  bareStar:  '2 * 3 = 6，* 是乘号；a ** b 也应原样可见',
  intraWord: '变量 a_b_c 与 snake_case_name 不该变斜体',
  parenUrl:  '坏 [a](javascript:alert(1)) 与好 [维基](https://zh.wikipedia.org/wiki/x_(y))',
  math:      '价格 $a_1 + b_2$ 与 $5 到 $10 元，还有 **粗** 在旁',
  strike:    '~~这条已作废~~，但 **这条** 有效',
  basic:     '**故障码** P0420 与 [手册](https://gccsmile.com/manual) 以及 `ERR-7`',
  ol:        '3. 起步\n4. 打火\n5. 交车\n\n- 无序\n- 仍无序',
  tableFloor:'| 列A | 列B |\n| --- | --- |\n| 1 | **2** |',
  mermaid:   '```mermaid\ngraph TD; A-->B;\n```',
  task:      '- [ ] 待办\n- [x] 已完成',
  imgAlt:    '看图 ![故障铭牌](https://x.com/1.png) 结束',
  codeFence: '```\ncode **block** 内记号原样\n```',
  quote:     '> 引用里的 **粗体** 与 [链接](https://x.com/a)\n> 第二行 `码`',
  heading:   '## **粗** 标题 与 [手册](https://x.com/m)',
  tight:     '**a**b**c** 三连',
  emptyAlt:  '![](https://x.com/1.png) 空 alt 的图',
  codeMarks: '变量 `a**b` 与 反引号里的 ** 号',
  listItem:  '- **粗** 项与 [链接](https://x.com/a)\n- 普通项',
};

// ── INV-1 ⑥-1：渲染面（runs 的可见叶子文字）不得出现成对记号 ──
// 判据必须**按单个 run 判**：跨 run 拼接后，「代码内容里的 **」与「远处另一个裸 **」会被
// 成对正则连成一次误报（实测 codeMarks 用例）。⇒ 票一 AC 的判据形状应当是 per-run。
for (const [k, md] of Object.entries(SRC)) {
  const blocks = M.parseMarkdown(md, 'forum', true);
  const leaves = [];
  const walk = (runs, inCode) => runs.forEach(r => {
    const code = inCode || !!r.c;
    if (r.kids && r.kids.length) walk(r.kids, code);
    else if (!code) leaves.push(r.text);      // 代码内容按口径原样，不参与「记号泄漏」判定
  });
  blocks.forEach(b => {
    if (b.type === 'code') return;                       // 围栏内原样
    const its = b.type === 'list' ? b.items : (b.type === 'table' ? b.items : (b.text ? [b.text] : []));
    its.forEach(t => walk(M.forumInlineRuns(t), false));
  });
  const bad = leaves.filter(t => t.match(MARKER));
  check('INV-1', `render surface has no marker (per-run): ${k}`, bad.length === 0,
    (bad.length ? 'LEAK ' + JSON.stringify(bad[0]) : 'ok') + ' | leaves=' + JSON.stringify(leaves));
}

// ── INV-2 ⑩-5：摘要投影相对「生产现状」的差异集合，必须每一条都是修 bug，不许有回归 ──
// 忠实复刻生产：forum 档 fork=true 拿结构（源串完整），再用无 flanking、无 ~~ 规则的生产语义抹平
function fp(text) {
  const spans = M.collectMathSpans(text);
  const strip = t => productionPlain(t);
  if (spans.length === 0) return strip(text);
  let out = '', cur = 0;
  for (let k = 0; k + 1 < spans.length; k += 2) {
    out += strip(text.substring(cur, spans[k]));
    out += text.substring(spans[k], spans[k + 1]);
    cur = spans[k + 1];
  }
  out += strip(text.substring(cur));
  return out;
}
const diffs = [];
for (const [k, md] of Object.entries(SRC)) {
  const srcBlocks = M.parseMarkdown(md, 'forum', true);
  const newProj = M.projectPlainText(srcBlocks, true);          // 跳过围栏代码（把「代码原文入摘要」这个既有行为单列，见 INV-12）
  const oldProj = srcBlocks.filter(b => b.type !== 'code').map(b => b.type === 'divider' ? ''
    : (b.type === 'list' || b.type === 'table') ? b.items.map(fp).join(' ') : fp(b.text)).join(' ').replace(/\n/g, ' ');
  if (newProj !== oldProj) diffs.push({ k, old: oldProj, neu: newProj });
}
// 允许的漂移 = 每一条都修一个「现状丢字符 / 漏记号」的既有缺陷。
// ⚠️ 实测 6 条 ⇒ ⑩-5 的「产物逐字不变」这句**不成立**，得回写 ADR（应为「只允许等于修复集的变化」）。
const EXPECT_DRIFT = ['bareStar', 'codeMarks', 'intraWord', 'nested', 'parenUrl', 'strike'];
const got = diffs.map(d => d.k).sort();
check('INV-2', 'projection drifts == exactly the bug-fix set', JSON.stringify(got) === JSON.stringify(EXPECT_DRIFT),
  'got=' + JSON.stringify(got) + '\n' + JSON.stringify(diffs, null, 1));

// ── INV-3 ⑩-6：chapter / featured 在 fork 开关上逐字不变 ──
for (const sub of ['chapter', 'featured']) {
  const bad = [];
  Object.values(SRC).forEach(md => {
    const a = JSON.stringify(M.parseMarkdown(md, sub, false));
    const b = JSON.stringify(M.parseMarkdown(md, sub, true));
    if (a !== b) bad.push(md.slice(0, 30));
  });
  check('INV-3', `${sub} zero-regression under fork`, bad.length === 0, JSON.stringify(bad));
  // 结构面：两档的块序列（type/level/items 个数）必须与生产抹平版一致
  const st = (bs) => bs.map(b => b.type + ':' + b.level + ':' + b.items.length).join(',');
  const same = Object.values(SRC).every(md => st(M.parseMarkdown(md, sub, false)) === st(M.parseMarkdown(md, sub, true)));
  check('INV-3b', `${sub} block structure identical fork on/off`, same, 'structure-only diff');
}
const fa = JSON.stringify(M.parseMarkdown(SRC.basic, 'forum', false));
const fb = JSON.stringify(M.parseMarkdown(SRC.basic, 'forum', true));
check('INV-3', 'forum fork is NOT a no-op', fa !== fb, 'forum changed: ' + (fa !== fb));

// ── INV-4 公式跨度：投影与渲染都保住源码（含下标下划线）──
{
  const pr = M.projectPlainText(M.parseMarkdown(SRC.math, 'forum', true));
  check('INV-4', 'math span survives projection with subscripts', pr.includes('$a_1 + b_2$'), JSON.stringify(pr));
  const runs = M.forumInlineRuns(SRC.math);
  const flat = runs.map(r => r.text).join('');
  check('INV-4', 'math span not eaten by italic rule at tokenize', flat.includes('$a_1 + b_2$'), JSON.stringify(runs));
}

// ── INV-5 flanking：裸 * 与词内 _ 不产生斜体 run ──
{
  const runs = M.forumInlineRuns(SRC.bareStar);
  const ital = runs.filter(r => r.i);
  check('INV-5', 'bare asterisks produce no italic run', ital.length === 0, JSON.stringify(runs));
  const r2 = M.forumInlineRuns(SRC.intraWord);
  check('INV-5', 'intra-word underscores stay literal', r2.filter(r => r.i).length === 0, JSON.stringify(r2));
}

// ── INV-6 嵌套：粗体内部的链接/代码必须成为 kids（否则源串泄漏到渲染面）──
{
  const runs = M.forumInlineRuns(SRC.nested);
  const bold = runs.find(r => r.b);
  const kids = (bold && bold.kids) || [];
  const hasLink = kids.some(r => r.link);
  const hasCode = kids.some(r => r.c);
  check('INV-6', 'bold contains link+code as children (recursion needed)', hasLink && hasCode, JSON.stringify(runs));
  const strikeInItal = runs.some(r => r.i && (r.kids || []).some(k => k.s));
  check('INV-6', 'italic contains strike as child', strikeInItal, 'found=' + strikeInItal);
}

// ── INV-7 协议闸 + 括号内 URL 不留残括号 ──
{
  const runs = M.forumInlineRuns(SRC.parenUrl);
  const flat = runs.map(r => r.text).join('');
  check('INV-7', 'no stray ) leaked from url with parens', !/\)$|^\)/.test(flat) && !flat.includes('])'), JSON.stringify(runs));
  const links = [];
  const walkL = rs => rs.forEach(r => { if (r.link) links.push(r.link); if (r.kids) walkL(r.kids); });
  walkL(runs);
  check('INV-7', 'only http(s) became clickable', links.every(u => /^https?:\/\//i.test(u)), JSON.stringify(links));
}

// ── INV-8 D4 有序：level 槽承载有序，且起始值从 1 起（⑩-4）──
{
  const blocks = M.parseMarkdown(SRC.ol, 'forum', true);
  const lists = blocks.filter(b => b.type === 'list');
  const levels = lists.map(b => b.level);
  check('INV-8', 'ol=1 / ul=0 carried in existing level slot', JSON.stringify(levels) === '[1,0]', JSON.stringify(levels));
  check('INV-8', 'no new field on MarkdownBlock', Object.keys(lists[0]).sort().join(',') === 'items,level,text,type', JSON.stringify(Object.keys(lists[0])));
}

// ── INV-9 段落地板：未声明语法内容不消失（⑩ 的降级可读底线）──
for (const k of ['tableFloor', 'mermaid', 'task', 'imgAlt', 'codeFence']) {
  const blocks = M.parseMarkdown(SRC[k], 'forum', true);
  const vis = JSON.stringify(blocks);
  const key = { tableFloor: '列A', mermaid: 'graph TD', task: '待办', imgAlt: '故障铭牌', codeFence: 'code **block**' }[k];
  check('INV-9', `undecleared syntax still visible on floor: ${k}`, vis.includes(key), vis);
}

// ── INV-10 实现约束：新分词器只能接在论坛投影这一条路上 ──
// chapter/featured 的摘要走 #1273 的「直接用块 text」（解析时已抹平，不再二次剥）。
// 若图省事把这两档也接到 plainOf 上，会二次处理已抹平的文本 ⇒ ⑩-6 的「逐字不变」被破。
{
  const wouldDrift = [];
  for (const [k, md] of Object.entries(SRC)) {
    for (const sub of ['chapter', 'featured']) {
      const bs = M.parseMarkdown(md, sub, false);
      const prod = bs.map(b => b.type === 'divider' ? '' : (b.type === 'list' || b.type === 'table' ? b.items.join(' ') : b.text)).join(' ');
      const viaTokenizer = bs.map(b => b.type === 'divider' ? '' : (b.type === 'list' || b.type === 'table' ? b.items.map(M.plainOf).join(' ') : (b.type === 'code' ? b.text : M.plainOf(b.text)))).join(' ');
      if (prod !== viaTokenizer) wouldDrift.push(sub + '/' + k);
    }
  }
  check('INV-10', 'new tokenizer must NOT be wired into chapter/featured projection', true,
    '若接入则漂移的档位/用例（⇒ 实现必须只给论坛接）：' + JSON.stringify(wouldDrift));
}

// ── INV-12 摘要携带围栏代码原文 = 生产既有行为（不是 ⑩ 引入），票一须具名或收口 ──
{
  const bs = M.parseMarkdown(SRC.codeFence, 'forum', true);
  const withCode = M.projectPlainText(bs, false);
  const noCode = M.projectPlainText(bs, true);
  check('INV-12', 'fenced code content reaches summary (pre-existing behavior)', /\*\*block\*\*/.test(withCode) && !/\*\*block\*\*/.test(noCode),
    '带上代码块=' + JSON.stringify(withCode) + ' / 跳过代码块=' + JSON.stringify(noCode));
}

// ── INV-11 渲染面（真正画出来的 HTML）必须按元素切段判 ──
// 浏览器实测抓到两个「只按 runs 判永远看不见」的缺陷：
//   A) 引用块走 esc(blk.text) 不消费 runs ⇒ 源串 ** 与反引号被直接画出来；
//   B) run 上写 style="font:inherit" ⇒ font 简写重置 weight/style/family，加粗/斜体/等宽从未生效。
// ⇒ 票一 ⑥-1 的判据形状应当是「对渲染产物按元素切段断言」，另加一条「样式不被简写重置」的字面判据。
{
  const badRenders = [];
  for (const [k, md] of Object.entries(SRC)) {
    const out = M.renderBlocks(M.parseMarkdown(md, 'forum', true), false);
    const frags = M.surfaceFragments(out);
    const leaks = frags.filter(t => t.match(MARKER));
    if (leaks.length) badRenders.push(k + ' -> ' + JSON.stringify(leaks[0]));
    if (/font:inherit/.test(out)) badRenders.push(k + ' -> 渲染产物里出现 font:inherit（会重置 r-b/r-i/r-c）');
  }
  check('INV-11', 'rendered HTML surface is marker-clean (per element)', badRenders.length === 0, JSON.stringify(badRenders, null, 1));
  const boldOut = M.renderBlocks(M.parseMarkdown(SRC.basic, 'forum', true), false);
  check('INV-11b', 'bold/italic/code runs actually carry their style hooks', /r-b/.test(boldOut) && /r-c/.test(boldOut) && /r-l/.test(boldOut), boldOut.slice(0, 220));
}

// ── 输出 ──
// ── INV-0 整段内联脚本必须能编译（浏览器实测：一行结尾写成 ')) 而非 ']) ⇒ 整页静默白屏，
//    而只抽取部分片段的自查完全看不见。单文件原型的最高价值回归判据就是这一条。）──
{
  const sStart = html.indexOf('<script>') + '<script>'.length;
  const whole = html.slice(sStart, html.lastIndexOf('</script>'));
  let err = null;
  try { new Function(whole); } catch (e) { err = e.message; }
  check0(!err, 'whole inline script compiles' + (err ? ' -> ' + err : ''));
}

let bad = 0;
for (const p of P) {
  if (!p.pass) bad++;
  console.log((p.pass ? 'PASS ' : 'FAIL ') + p.id + '  ' + p.desc);
  console.log('      ' + String(p.detail).replace(/\n/g, '\n      '));
}
fail += bad;
say('\n' + (P.filter(p => p.pass).length) + ' pass / ' + fail + ' fail  (含 INV-0)');
process.exitCode = fail ? 1 : 0;
