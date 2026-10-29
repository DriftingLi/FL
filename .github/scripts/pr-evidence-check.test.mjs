// pr-evidence 校验器的单测（A1 形态，#2100）。
//
// 被测对象是 .github/scripts/lib/gate-validator.mjs —— 判据的**单一来源**：
//   · pr-evidence.yml 不 checkout，改用 repos.getContent 取该文件文本 → new Function 求值；
//   · 本测试直接 import（同一个函数，不是抄一份）。
// 运行：node --test .github/scripts/pr-evidence-check.test.mjs（已由 ci.yml 的 pr-evidence-selftest job 调用）。
//
// 2026-10-29（A1，票 #2100）本文件随判据大幅收缩：候选集 / marker / 展示副本 / ① / ③ / ④b / 字段签核
// 的用例随那些判据一起删除 —— 留着它们等于给死代码上保护，比没有测试更坏（改判据时会以为"测试挡住了"）。
// 保留并加强的是三类：**触发面**（该跑没跑）、**sha 绑定**（7 位前缀 + 祖先口径 + fail-closed）、
// **反回流静态自检**（别把退役机制悄悄请回来）。
//
// 比较一律注桩（cmp()），**不发任何真实网络请求**。

import { test } from 'node:test';
import assert from 'node:assert/strict';
import { readFileSync } from 'node:fs';
import { fileURLToPath } from 'node:url';
import path from 'node:path';
import { validatePrEvidence as validate, GATE_EVIDENCE } from './lib/gate-validator.mjs';

const SCRIPT_DIR = path.dirname(fileURLToPath(import.meta.url));
const REPO_ROOT = path.resolve(SCRIPT_DIR, '..', '..');
const workflowPath = path.join(REPO_ROOT, '.github', 'workflows', 'pr-evidence.yml');
const validatorPath = path.join(REPO_ROOT, '.github', 'scripts', 'lib', 'gate-validator.mjs');
const templatePath = path.join(REPO_ROOT, '.github', 'PULL_REQUEST_TEMPLATE.md');

const HEAD = 'deadbeef1234567890abcdef1234567890abcdef';
const HEAD7 = HEAD.slice(0, 7);
const OLD = '0123456789abcdef0123456789abcdef01234567'; // head 的祖先
const OLD7 = OLD.slice(0, 7);

const uvue = (name = 'pages/exam/exam.uvue', extra = {}) => ({
  filename: `training-app/叉车维修培训学员端跨端应用/${name}`,
  status: 'modified',
  patch: '',
  ...extra,
});
const docs = { filename: 'docs/adr/0008-移动端验收门与证据.md', status: 'added', patch: '' };

// 门自带的机读行（原样贴进正文，判据只认 token 与 sha7）
const MP = 'MP_WEIXIN_RESULT appid=wx1234567890abcdef pageStack=2 entry=pages/index/index errorsTotal=0 exceptionsTotal=0 shots=2 navigation=ok';
const KOTLIN = 'KOTLIN_ALL_RESULT errors=0 classes=1261 files=88 freshness=fresh';
const COMPILE = 'COMPILE_RESULT errors=0 clean=true';

const body = (...gateLines) => `## 改了什么\n\n把 exam 页拆了。\n\n## 验收证据\n\n${gateLines.join('\n')}\n`;
const g2 = (sha7, reading = MP) => `- ② 跑于 \`${sha7}\` · \`${reading}\``;
const g4 = (sha7, reading = KOTLIN) => `- ④ 跑于 \`${sha7}\` · \`${reading}\``;
// 运行时面 + 未命中 MP-WEIXIN 面 ⇒ ② 免，只有 ④ 必填
const onlyG4 = (sha7 = HEAD7) => body(g4(sha7));
// 命中 MP-WEIXIN 面（改 manifest.json）⇒ ②④ 都必填
const bothGates = (sha7 = HEAD7) => body(g2(sha7), g4(sha7));
const mpFiles = [uvue(), { filename: 'training-app/叉车维修培训学员端跨端应用/manifest.json', status: 'modified', patch: '' }];

// 注桩：默认「sha 已是 head 祖先 + 其间未动运行时面」
function cmp({ status = 'ahead', mergeBaseSha = OLD, files = ['docs/x.md'], filesTruncated = false } = {}) {
  const calls = [];
  const fn = async ({ base, head }) => {
    calls.push({ base, head });
    return { status, mergeBaseSha, files, filesTruncated };
  };
  fn.calls = calls;
  return fn;
}

const run = async (opts = {}) => validate({ files: [uvue()], body: onlyG4(), headSha: HEAD, ...opts });

/* ─────────────── 分级判据与段结构 ─────────────── */

test('非运行时面 PR：正文为空也通过（分级判据）', async () => {
  const r = await validate({ files: [docs], body: '', headSha: HEAD });
  assert.equal(r.ok, true);
  assert.deepEqual(r.runtime, []);
});

test('运行时面 PR + 空正文：红，且提示缺段', async () => {
  const r = await run({ body: '' });
  assert.equal(r.ok, false);
  assert.match(r.errors[0], /验收证据/);
});

test('A1 完整形状（②+④ 两行）⇒ 绿', async () => {
  const r = await run({ files: mpFiles, body: bothGates() });
  assert.deepEqual(r.errors, []);
  assert.equal(r.ok, true);
  assert.equal(r.gates['②'].bound, true);
  assert.equal(r.gates['④'].bound, true);
});

test('证据只写在 HTML 注释里不算（模板注释不得骗过校验）', async () => {
  const r = await run({ body: `## 验收证据\n\n<!-- ${g4(HEAD7)} -->\n` });
  assert.equal(r.ok, false);
  assert.ok(r.errors.some((e) => e.includes('缺该行')));
});

test('未命中运行时面 ⇒ 不要求任何门行，也不要求段', async () => {
  const r = await validate({ files: [docs, { filename: 'frontend/src/a.vue', status: 'modified', patch: '' }], body: '', headSha: HEAD });
  assert.equal(r.ok, true);
  assert.match(r.notes.join(), /未命中运行时面/);
});

/* ─────────────── 触发面：哪道门必填 ─────────────── */

test('未命中 MP-WEIXIN 面 ⇒ ② 免：只写 ④ 行即绿', async () => {
  const r = await run({ body: onlyG4() });
  assert.equal(r.ok, true);
  assert.equal(r.gates['②'].required, false);
});

test('diff 里增删真正的 MP-WEIXIN 条件编译指令行 ⇒ ② 必填，缺行即红', async () => {
  const patch = '@@ -1,2 +1,3 @@\n+// #ifdef MP-WEIXIN\n const a = 1;\n+// #endif\n';
  const r = await run({ files: [uvue('pages/a/a.uvue', { patch })], body: onlyG4() });
  assert.equal(r.gates['②'].required, true);
  assert.equal(r.ok, false);
  assert.ok(r.errors.some((e) => e.startsWith('②')));
});

test('#1030 负例：文档里写「未命中 MP-WEIXIN 面」这句话本身不点亮 ②', async () => {
  const r = await run({
    files: [uvue('pages/a/a.uvue', { patch: '+在 ADR 里写：未命中 MP-WEIXIN 面，故免 ②\n' })],
    body: onlyG4(),
  });
  assert.equal(r.gates['②'].required, false);
  assert.equal(r.ok, true);
});

test('#1030 负例：运行时面文件里的散文提及（非行首指令）不点亮 ②', async () => {
  const patch = '+// 与 MP-WEIXIN 分支不同，这里走 APP-PLUS\n';
  const r = await run({ files: [uvue('pages/a/a.uvue', { patch })], body: onlyG4() });
  assert.equal(r.gates['②'].required, false);
});

test('manifest.json 改动 ⇒ ② 必填；frontend 下同名 json 不算运行时面', async () => {
  const hit = await run({ files: mpFiles, body: onlyG4() });
  assert.equal(hit.gates['②'].required, true);
  const miss = await run({
    files: [uvue(), { filename: 'frontend/manifest.json', status: 'modified', patch: '' }],
    body: onlyG4(),
  });
  assert.equal(miss.gates['②'].required, false);
});

test('④a 行（COMPILE_RESULT）可满足 ④；④b 行不能顶替 ④', async () => {
  const ok = await run({ body: body(g4(HEAD7, COMPILE)) });
  assert.equal(ok.ok, true, ok.errors.join());
  const bad = await run({ body: body(`- ④b 跑于 \`${HEAD7}\` · release 打包通过`) });
  assert.equal(bad.ok, false);
  assert.ok(bad.errors.some((e) => e.includes('缺该行')));
});

test('④b 打包面命中：不判红，但 notes 里必须留发版提醒', async () => {
  const files = [uvue(), { filename: 'training-app/叉车维修培训学员端跨端应用/pages.json', status: 'modified', patch: '' }];
  const r = await run({ files, body: onlyG4() });
  assert.equal(r.needs4b, true);
  assert.equal(r.ok, true);
  assert.match(r.notes.join(), /④b 打包面命中[\s\S]*发版前必须跑/);
});

/* ─────────────── 行形态：读数行 + 跑于 ─────────────── */

test('② 行缺 MP_WEIXIN_RESULT 读数行 ⇒ 红（禁止「已验证」式自然语言）', async () => {
  const r = await run({ files: mpFiles, body: body(`- ② 跑于 \`${HEAD7}\` · 入口页无报错`, g4()) });
  assert.equal(r.ok, false);
  assert.ok(r.errors.some((e) => e.includes('MP_WEIXIN_RESULT')));
});

test('④ 行缺读数行 ⇒ 红', async () => {
  const r = await run({ body: body(`- ④ 跑于 \`${HEAD7}\` · 编译通过`) });
  assert.equal(r.ok, false);
  assert.ok(r.errors.some((e) => e.includes('KOTLIN_ALL_RESULT')));
});

test('缺 `跑于 <sha7>` ⇒ 红（读数行在也不行：没有绑定就没有证据）', async () => {
  const r = await run({ body: body(`- ④ · \`${KOTLIN}\``) });
  assert.equal(r.ok, false);
  assert.ok(r.errors.some((e) => e.includes('跑于')));
});

test('`跑于` 后面不是 sha（如写分支名）⇒ 判成缺 sha7，不静默通过', async () => {
  const r = await run({ body: body(`- ④ 跑于 master · \`${KOTLIN}\``) });
  assert.equal(r.ok, false);
  assert.ok(r.errors.some((e) => e.includes('跑于')));
});

/* ─────────────── sha 绑定：等于 head / 祖先口径 / fail-closed ─────────────── */

test('sha7 等于 head 前 7 位 ⇒ 满足，且**不打** compare', async () => {
  const stub = cmp();
  const r = await run({ body: onlyG4(HEAD7), fetchCompare: stub });
  assert.equal(r.ok, true);
  assert.equal(stub.calls.length, 0, '恰好等于 head 时不该花配额去 compare');
  assert.equal(r.gates['④'].commit, HEAD7);
});

test('sha7 是 head 祖先 + 其间只有非运行时面变更 ⇒ 满足（7 位前缀必须能绑上 40 位 merge_base）', async () => {
  const r = await run({ body: onlyG4(OLD7), fetchCompare: cmp({ mergeBaseSha: OLD }) });
  assert.equal(r.ok, true, r.errors.join());
  assert.match(r.notes.join(), /祖先/);
  assert.equal(r.gates['④'].commit, OLD7);
});

test('sha7 是祖先，但其间运行时面变了 ⇒ 不满足，文案要求改 sha7', async () => {
  const r = await run({
    body: onlyG4(OLD7),
    fetchCompare: cmp({ mergeBaseSha: OLD, files: ['training-app/叉车维修培训学员端跨端应用/pages/b.uvue'] }),
  });
  assert.equal(r.ok, false);
  assert.match(r.errors.join(), /需重跑/);
});

test('merge_base 不是它（不是祖先 / head 上找不到）⇒ 不满足', async () => {
  const r = await run({ body: onlyG4(OLD7), fetchCompare: cmp({ mergeBaseSha: 'ffffffffffffffffffffffffffffffffffffffff' }) });
  assert.equal(r.ok, false);
  assert.match(r.errors.join(), /不是当前 head 的祖先/);
});

test('compare 返回 null（未知 sha / API 异常）⇒ fail-closed 不满足，文案写明', async () => {
  const r = await run({ body: onlyG4(OLD7), fetchCompare: async () => null });
  assert.equal(r.ok, false);
  assert.match(r.errors.join(), /fail-closed/);
});

test('compare 抛异常（超时 / 限流）⇒ 仍 fail-closed，不外抛', async () => {
  const r = await run({ body: onlyG4(OLD7), fetchCompare: async () => { throw new Error('rate limited'); } });
  assert.equal(r.ok, false);
});

test('超大 diff 被截断（filesTruncated）⇒ fail-closed，不拿截断列表当「没改」', async () => {
  const r = await run({ body: onlyG4(OLD7), fetchCompare: cmp({ mergeBaseSha: OLD, filesTruncated: true }) });
  assert.equal(r.ok, false);
  assert.match(r.errors.join(), /截断/);
});

test('完全没注入 fetchCompare ⇒ 不满足（调用方忘了给也不静默放行）', async () => {
  const r = await run({ body: onlyG4(OLD7) });
  assert.equal(r.ok, false);
});

test('没给 headSha ⇒ 不满足（fail-closed）', async () => {
  const r = await run({ body: onlyG4(OLD7), headSha: '' });
  assert.equal(r.ok, false);
});

test('②④ 共用同一份 compare 缓存（同一 sha 只查一次，省配额）', async () => {
  const stub = cmp({ mergeBaseSha: OLD });
  const r = await run({ files: mpFiles, body: bothGates(OLD7), fetchCompare: stub });
  assert.equal(r.ok, true, r.errors.join());
  assert.equal(stub.calls.length, 1, '两个门同一个 sha，必须只打一次 compare');
});

test('注入自检：桩不被调用时该判定必须失败（防空跑假绿）', async () => {
  let called = 0;
  const spy = async () => { called += 1; return { status: 'ahead', mergeBaseSha: OLD, files: [], filesTruncated: false }; };
  const r = await run({ body: onlyG4(OLD7), fetchCompare: spy });
  assert.equal(r.ok, true);
  assert.equal(called, 1, '祖先口径必须真的走 compare —— 桩没被调用说明判据空转了');
});

/* ─────────────── 例外通道（2026-10-29 review 收窄：必须点名门） ─────────────── */

test('例外通道：点名 ④ ⇒ 放行，且把被豁免的缺项原样带进 notes', async () => {
  const r = await run({ body: `## 验收证据\n\n- 例外：已接受未验证风险 ④\n` });
  assert.equal(r.ok, true);
  assert.deepEqual(r.errors, []);
  assert.match(r.notes.join(), /例外通道/);
  assert.match(r.notes.join(), /已豁免/);
  assert.match(r.notes.join(), /必须由人执行合并/);
});

test('P0-2 锁：只写「已接受未验证风险」不点名门 ⇒ 不放行（五个字换不来豁免）', async () => {
  const r = await run({ body: `## 验收证据\n\n已接受未验证风险\n` });
  assert.equal(r.ok, false);
  assert.ok(r.errors.length > 0, '没点名门就把全部 errors 清空 —— 通道宽到没有意义');
});

test('P0-2 锁：否认句不算声明（「不适用『已接受未验证风险』」曾被当成声明）', async () => {
  const r = await run({ body: `## 验收证据\n\n本 PR 不适用「已接受未验证风险」通道，证据齐全。\n` });
  assert.equal(r.ok, false);
});

test('P0-2 锁：例外只清被点名的门 —— 点名 ② 救不了 ④ 的缺项', async () => {
  // 命中 MP-WEIXIN 面 ⇒ ②④ 都必填，这里两行都没写
  const r = await run({ files: mpFiles, body: `## 验收证据\n\n- 例外：已接受未验证风险 ②\n` });
  assert.equal(r.ok, false);
  assert.ok(r.errors.some((e) => e.includes('④')), `④ 的缺项必须照红，实际：${r.errors.join(' / ')}`);
  assert.match(r.notes.join(), /只覆盖了/);
});

test('例外声明写在段外不算（背景叙述 / 引用的旧例子不得触发）', async () => {
  const r = await run({ body: `## 改了什么\n\n上一轮用过「已接受未验证风险 ④」。\n\n## 验收证据\n` });
  assert.equal(r.ok, false);
  assert.ok(r.errors.length > 0);
});

test('例外通道不能替代证据段：缺 `## 验收证据` 仍红', async () => {
  const r = await run({ body: '## 改了什么\n\n- 例外：已接受未验证风险 ④\n' });
  assert.equal(r.ok, false);
  assert.match(r.errors[0], /验收证据/);
});

test('点名了但该门没缺项 ⇒ 记 note（多写了），不假装豁免了东西', async () => {
  const r = await run({ body: `${bothGates()}- 例外：已接受未验证风险 ②\n` });
  assert.equal(r.ok, true);
  assert.match(r.notes.join(), /没有缺项/);
});

/* ─────────────── 结构化字段（预检/渲染消费） ─────────────── */

test('gates 结构化字段：required / bound / commit / why', async () => {
  const r = await run({ files: mpFiles, body: bothGates() });
  assert.deepEqual(Object.keys(r.gates).sort(), ['②', '④'].sort());
  for (const k of ['②', '④']) {
    assert.equal(r.gates[k].required, true);
    assert.equal(r.gates[k].bound, true);
    assert.equal(r.gates[k].commit, HEAD7);
  }
  assert.ok(Array.isArray(r.gates['④'].why));
});

test('未绑上时 gates.bound 必须 false（不许把「有行」当「已取证」）', async () => {
  const r = await run({ body: onlyG4(OLD7) });
  assert.equal(r.gates['④'].bound, false);
  assert.match(r.gates['④'].reason, /fail-closed|祖先|不认识|取不到/);
});

/* ─────────────── 形状单一来源与模板锁 ─────────────── */

test('描述符是严格 JSON 可解析（PowerShell 侧用 ConvertFrom-Json 读同一段字节）', () => {
  const src = readFileSync(validatorPath, 'utf8');
  const m = src.match(/\/\/ ===== GATE_EVIDENCE_DESCRIPTOR BEGIN =====([\s\S]*?)\/\/ ===== GATE_EVIDENCE_DESCRIPTOR END =====/);
  assert.ok(m, '找不到描述符 marker：PowerShell 侧会直接抛错');
  const json = m[1].trim().replace(/^export\s+const\s+GATE_EVIDENCE\s*=\s*/, '').replace(/;\s*$/, '');
  const parsed = JSON.parse(json);
  assert.equal(parsed.heading, GATE_EVIDENCE.heading);
  assert.deepEqual(parsed.gates.map((g) => g.key), ['②', '④']);
});

test('P0-1 锁：模板原样（只换 sha7）不得判绿 —— 否则「照抄模板」就等于有证据', async () => {
  const tpl = readFileSync(templatePath, 'utf8');
  const section = (tpl.split('## 验收证据')[1] || '').replace(/<!--[\s\S]*?-->/g, '');
  // 最接近「照抄模板、只改 sha」的形态：占位 sha 换成真 head 前 7 位，读数行原样不动。
  // 2026-10-29 review 实测：改之前这一形态 ok=true（token 只判 line.includes）。
  const filled = section.replace(/<sha7>/g, HEAD7);
  const r = await run({ files: mpFiles, body: `## 验收证据\n${filled}` });
  assert.equal(r.ok, false, '模板原样被判绿：占位读数被当成真读数（P0-1）');
  assert.ok(
    r.errors.some((e) => /没有实际读数/.test(e)),
    `应报「没有实际读数」，实际：${r.errors.join(' / ')}`,
  );
});

test('模板必须给出可照抄的门行形状（光堵假绿不够，人得知道写什么）', () => {
  const tpl = readFileSync(templatePath, 'utf8');
  const section = (tpl.split('## 验收证据')[1] || '').replace(/<!--[\s\S]*?-->/g, '');
  for (const gate of GATE_EVIDENCE.gates) {
    const line = section.split('\n').find((l) => l.replace(/^\s*[-*]\s+/, '').startsWith(gate.key));
    assert.ok(line, `模板里没有 ${gate.key} 行 —— 照模板写的人不知道要写哪一行`);
    assert.ok(gate.tokens.some((t) => line.includes(t)), `模板的 ${gate.key} 行没提示读数行 token`);
    assert.match(line, /跑于/, `模板的 ${gate.key} 行没写 \`跑于 <sha7>\``);
  }
});

/* ─────────────── 反回流静态自检 ─────────────── */

test('判据模块不得把退役机制请回来（评论候选 / marker / 展示副本 / issueNumbersInBody）', () => {
  const src = readFileSync(validatorPath, 'utf8');
  for (const banned of [/gateComments/, /commitComments/, /issueComments/, /gate-evidence:/, /issueNumbersInBody/, /exemptions/]) {
    assert.doesNotMatch(src, banned, `判据里出现了已退役的东西：${banned}`);
  }
});

test('W1：CI 的 compare 取数必须并 previous_filename（改名是运行时面改动的一种形态）', () => {
  const wf = readFileSync(workflowPath, 'utf8');
  // `a.uvue` → `a.ts` 时 compare 只报 filename=a.ts；不并 previous_filename ⇒ 判「运行时面没动」⇒ fail-open
  assert.match(wf, /previous_filename/, 'compare 取数没并 previous_filename：改名成 .ts 可绕过祖先判定');
  assert.match(wf, /f\.filename/, '取数形态变了，这条断言要跟着改');
});

test('W2：compare 不回 files（非 300 截断）也必须按截断处理，不能当「没改动」', () => {
  const wf = readFileSync(workflowPath, 'utf8');
  assert.match(wf, /raw === null/, 'compare 不回 files 的形态没被识别 ⇒ 又一次 fail-open');
  assert.match(wf, /filesTruncated:\s*truncated|filesTruncated:/, 'filesTruncated 的产出形态变了');
});

test('CI 同构：剥 export + new Function 求值必须能跑通（import 绿不代表 CI 绿）', async () => {
  // pr-evidence.yml 不 checkout，用 repos.getContent 取单文件文本再 new Function 求值。
  // 这条路径与本地 import 不同：文件里一旦出现 import / 顶层 await，本地照样绿、CI 当场炸。
  const src = readFileSync(validatorPath, 'utf8');
  const api = new Function(`${src.replace(/^export /gm, '')}\nreturn { validatePrEvidence };`)();
  const green = await api.validatePrEvidence({ files: [uvue()], body: onlyG4(), headSha: HEAD });
  assert.equal(green.ok, true);
  // 同一条路径上，「照抄模板」也必须红 —— 两个加载路径的结论要一致
  const tpl = readFileSync(templatePath, 'utf8');
  const filled = (tpl.split('## 验收证据')[1] || '').replace(/<!--[\s\S]*?-->/g, '').replace(/<sha7>/g, HEAD7);
  const red = await api.validatePrEvidence({ files: mpFiles, body: `## 验收证据\n${filled}`, headSha: HEAD });
  assert.equal(red.ok, false);
});

test('workflow 不再读评论，也不再要 issues 权限（A1：证据只在正文）', () => {
  // 剥 YAML 注释：头部注释里正解释「issues: read 已随之删除」，不剥会把说明当成配置。
  const wf = readFileSync(workflowPath, 'utf8').split('\n').filter((l) => !/^\s*#/.test(l)).join('\n');
  for (const banned of [/listComments/, /listCommitComments/, /issues:\s*read/, /issueNumbersInBody/]) {
    assert.doesNotMatch(wf, banned, `workflow 里还留着读评论的路径：${banned}`);
  }
  assert.match(wf, /getContent/, 'workflow 必须仍从默认分支取判据（PR 不能改自己的判据）');
});

test('workflow 仍把「files ≥300 ⇒ 截断」写进取数回调（截断漏判会放行旧证据）', () => {
  const wf = readFileSync(workflowPath, 'utf8');
  assert.match(wf, /COMPARE_FILES_CAP\s*=\s*300/);
  // 判的是**原始**文件数：展开 previous_filename 之后一次改名会凭空多一条，
  // 拿展开后的条数比 300 会把没截断的判成截断（多要一次重跑，方向安全但白付）。
  assert.match(wf, /raw\.length\s*>=\s*COMPARE_FILES_CAP/);
});

test('② 判据不得退回「扫改动文本里的字面量」（文本判据与语义脱节的复发防线）', () => {
  const src = readFileSync(validatorPath, 'utf8');
  assert.doesNotMatch(src, /includes\(['"]MP-WEIXIN['"]\)/, '又把「文本里出现过这个词」当触发面了（#1030）');
  assert.match(src, /MP_WEIXIN_DIRECTIVE/);
});

test('删掉的门不得有任何残留判据（① / ③ / ④b 的行判定）', () => {
  const src = readFileSync(validatorPath, 'utf8');
  assert.doesNotMatch(src, /gate\.key === '①'/);
  assert.doesNotMatch(src, /gate\.key === '③'/);
  assert.deepEqual(GATE_EVIDENCE.gates.map((g) => g.key), ['②', '④'], '描述符只留两道门');
});
