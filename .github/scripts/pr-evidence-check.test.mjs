// pr-evidence 校验逻辑的单测（#1770 起不再「抽取」，直接 import 判据模块）。
//
// 校验器是 .github/scripts/lib/gate-validator.mjs —— 判据的**单一来源**：
//   · pr-evidence.yml / mobile-gates.yml 都不 checkout，改用 repos.getContent 取该文件文本 → new Function 求值；
//   · 本测试直接 import（同一个函数，不是抄一份）。
// 运行：node --test .github/scripts/pr-evidence-check.test.mjs（已由 ci.yml 的 pr-evidence-selftest job 调用）。
//
// 2026-09-12 修订（本文件与校验器同步改）：门证据的绑定判据从「sha 必须等于 head」放宽为
// 「**祖先 + 运行时面未变**」（动机：#624 反复被 master 前进作废）。判定要看 compare 结果，
// 故校验器改为 async 且把「取 compare 结果」抽成**注入回调 fetchCompare** ——
// 本文件里一律注桩（cmp() / compareStub()），**不发任何真实网络请求**。
//
// 2026-10-22（#1770）：证据主键改为 commit sha——新增 commitComments 入参的单测（commit 评论能承载门证据，
// 又没有绕过 marker/sha 绑定判据）。
// 2026-10-23（#1841）：commit 评论端点对本仓恒 404（#1838）⇒ 门脚本降级把同一份评论贴到票上，
// 票评论因此成为**第三个候选来源**。它不放宽任何判据（同 marker / 同 sha 绑定 / 同祖先判定），
// 差别只在「候选从哪儿来」——由 issueNumbersInBody 决定读哪些票（PR 正文显式关联，≤5 张）。

import { test } from 'node:test';
import assert from 'node:assert/strict';
import { readFileSync } from 'node:fs';
import { fileURLToPath } from 'node:url';
import path from 'node:path';
import { validatePrEvidence as validate, issueNumbersInBody } from './lib/gate-validator.mjs';

const SCRIPT_DIR = path.dirname(fileURLToPath(import.meta.url));
const workflowPath = path.resolve(SCRIPT_DIR, '..', 'workflows', 'pr-evidence.yml');
const validatorPath = path.resolve(SCRIPT_DIR, 'lib', 'gate-validator.mjs');
const AUTHOR = 'zhengcookie'; // 本仓库 agent 账号；PR 作者即它

const uvue = (name = 'pages/exam/exam.uvue', extra = {}) => ({
  filename: `training-app/叉车维修培训学员端跨端应用/${name}`,
  status: 'modified',
  patch: '',
  ...extra,
});
const docs = { filename: 'docs/adr/0008-移动端验收门与证据.md', status: 'added', patch: '' };

// 校验器自 2026-09-12 起是 async（要 await 注入的 fetchCompare）⇒ 用例统一 async + await
const run = async (opts) => validate({ files: [uvue()], body: '', author: AUTHOR, ...opts });

// ④ 门（2026-09-11 修订：④c 整模块为默认，dev 专属面追加 ④a）——正文一行两载体
const GATE4_LINE =
  '- ④ 本地编译门（默认 ④c 整模块；dev 面追加 ④a） — 执行人：@DriftingLi · 日期：2026-09-11 · 复测对象：exam 分支整模块 Kotlin 编译 · 结论（含产物）：KOTLIN_ALL_RESULT errors=0 classes=1261，日志 .ci-verify/kotlin-all.log';

const FULL_EVIDENCE = `## 改了什么

把 exam 页拆了。

## 验收证据

- ① Android 真机逐页截图对比 — 执行人：@DriftingLi · 日期：2026-09-11 · 复测对象：exam 入口页与交卷流程 · 结论（含产物）：逐页截图 docs/verification/exam/624/exam-home-after.jpg
- ② 微信开发者工具无报错 — 执行人：@DriftingLi · 日期：2026-09-11 · 复测对象：未命中 MP-WEIXIN 面，仅入口页 · 结论（含产物）：入口页无报错 ![入口页](https://github.com/user-attachments/assets/11111111-2222-3333-4444-555555555555)
- ③ \`npm run test:unit\` 全绿 — 结论（含产物）：https://github.com/DriftingLi/FL/actions/runs/123456789
${GATE4_LINE}
`;

test('非运行时面 PR：即使正文为空也通过（分级判据）', async () => {
  const r = await run({ files: [docs] });
  assert.equal(r.ok, true);
  assert.deepEqual(r.runtime, []);
  assert.match(r.notes.join(), /免人工门/);
});

test('运行时面 PR + 空正文：红，且提示缺段', async () => {
  const r = await run({});
  assert.equal(r.ok, false);
  assert.match(r.errors.join(), /验收证据/);
});

test('运行时面 PR + 完整四门证据：绿', async () => {
  const r = await run({ body: FULL_EVIDENCE });
  assert.equal(r.ok, true, r.errors.join('；'));
});

test('证据只写在 HTML 注释里不算（模板注释不得骗过校验）', async () => {
  const r = await run({ body: `<!--\n${FULL_EVIDENCE}\n-->` });
  assert.equal(r.ok, false);
  assert.match(r.errors.join(), /验收证据/);
});

test('占位符「待人工 / ⏳」= 缺证据：红', async () => {
  const body = FULL_EVIDENCE.replace('@DriftingLi · 日期：2026-09-11 · 复测对象：exam 入口页与交卷流程 · 结论（含产物）：逐页截图 docs/verification/exam/624/exam-home-after.jpg', '⏳ · 日期：待人工 · 复测对象：待人工 · 结论（含产物）：待人工');
  const r = await run({ body });
  assert.equal(r.ok, false);
  assert.match(r.errors.join(), /占位/);
});

test('执行人与 PR 作者同一账号：只提示、不拦（单账号仓库分不出人与 agent）', async () => {
  const body = FULL_EVIDENCE.replace('@DriftingLi', `@${AUTHOR}`);
  const r = await run({ body });
  assert.equal(r.ok, true, r.errors.join('；'));
  assert.match(r.notes.join(), /同一账号/);
});

test('日期格式不对：红', async () => {
  const body = FULL_EVIDENCE.replace('日期：2026-09-11 · 复测对象：exam 分支整模块 Kotlin 编译', '日期：今天 · 复测对象：exam 分支整模块 Kotlin 编译');
  const r = await run({ body });
  assert.equal(r.ok, false);
  assert.match(r.errors.join(), /YYYY-MM-DD/);
});

test('④ 结论没有引用编译日志：红（产物字段是硬判据）', async () => {
  const body = FULL_EVIDENCE.replace('KOTLIN_ALL_RESULT errors=0 classes=1261，日志 .ci-verify/kotlin-all.log', '已编译通过');
  const r = await run({ body });
  assert.equal(r.ok, false);
  assert.match(r.errors.join(), /kotlin-all\.log/);
});

test('③ 结论没贴 CI run 链接：红', async () => {
  const body = FULL_EVIDENCE.replace('https://github.com/DriftingLi/FL/actions/runs/123456789', 'CI 已通过');
  const r = await run({ body });
  assert.equal(r.ok, false);
  assert.match(r.errors.join(), /actions\/runs/);
});

test('新增 async function 的 patch → 不再触发 ④b（打包面判据取代代码形态判据）', async () => {
  const files = [
    uvue('utils/exam-session.uts', { patch: '@@ -1,2 +1,3 @@\n+export async function loadSession() {\n+}\n' }),
  ];
  const r = await run({ files, body: FULL_EVIDENCE });
  assert.equal(r.needs4b, false);
  assert.equal(r.ok, true, r.errors.join('；'));
});

test('新增 composable 文件 → 不再触发 ④b', async () => {
  const files = [uvue('pages/exam/composables/use-exam-session.uts', { status: 'added' })];
  const r = await run({ files, body: FULL_EVIDENCE });
  assert.equal(r.needs4b, false);
  assert.equal(r.ok, true, r.errors.join('；'));
});

test('声明「已接受未验证风险」：放行但打警告，并提示必须由人合并', async () => {
  const body = `${FULL_EVIDENCE.split('## 验收证据')[0]}## 验收证据\n\n已接受未验证风险：本地无 Android 真机。事后验证计划：合并后由维护者补跑。\n`;
  const r = await run({ body });
  assert.equal(r.ok, true, r.errors.join('；'));
  assert.match(r.notes.join(), /例外通道/);
  assert.match(r.notes.join(), /由人执行合并/);
});

test('例外通道不能替代证据段：缺段时仍红', async () => {
  const r = await run({ body: '已接受未验证风险：本地无真机。事后验证计划：由人补跑。\n' });
  assert.equal(r.ok, false);
  assert.match(r.errors.join(), /验收证据/);
});

test('training-app 下的 pages.json 也算运行时面（免得有人只改路由配置绕过）', async () => {
  const r = await run({ files: [uvue('pages.json', { status: 'modified' })], body: '' });
  assert.deepEqual(r.runtime.length, 1);
  assert.equal(r.ok, false);
});

test('未命中 MP-WEIXIN 面 → 第②门免（去掉②行也不红）', async () => {
  const body = FULL_EVIDENCE.split('\n').filter((l) => !l.startsWith('- ② ')).join('\n');
  const r = await run({ body });
  assert.equal(r.ok, true, r.errors.join('；'));
  assert.match(r.notes.join(), /第②门免/);
});

test('diff 出现 MP-WEIXIN 条件编译 → 第②门必填，缺②即红', async () => {
  const files = [uvue('pages/login/login.uvue', { patch: '@@ -1,2 +1,3 @@\n+// #ifdef MP-WEIXIN\n+// #endif\n' })];
  const body = FULL_EVIDENCE.split('\n').filter((l) => !l.startsWith('- ② ')).join('\n');
  const r = await run({ files, body });
  assert.equal(r.ok, false);
  assert.match(r.errors.join(), /②/);
});

test('manifest.json 改动 → 第②门必填', async () => {
  const files = [uvue('manifest.json', { patch: '@@ -1,2 +1,3 @@\n+"mp-weixin": {}\n' })];
  const body = FULL_EVIDENCE.split('\n').filter((l) => !l.startsWith('- ② ')).join('\n');
  const r = await run({ files, body });
  assert.equal(r.ok, false);
  assert.match(r.errors.join(), /②/);
});

test('frontend 下的同名 manifest.json 不算运行时面（不得误伤 Web 端）', async () => {
  const r = await run({ files: [{ filename: 'frontend/public/manifest.json', status: 'modified', patch: '' }] });
  assert.deepEqual(r.runtime, []);
  assert.equal(r.ok, true);
});

// -------------------- 2026-09-11 修订：④ 门 / ④b 打包面 / P1 评论 / 风险分层 --------------------

const SHA = 'a20d2e2c9f0e4b7d1c3a5f8e2b6d4a7c9e1f3b5d';
const gate4Comment = (sha) =>
  [
    '<!-- gate-evidence:④ -->',
    '**④ 本地编译门（④c 整模块 Kotlin 编译，agent 执行）**',
    `- commit: ${sha}`,
    '- 结论（含产物）：`KOTLIN_ALL_RESULT errors=0 classes=1261`；日志 `.ci-verify/kotlin-all.log`',
    '- 复现：`npm run build:kotlin-all`',
    '',
  ].join('\n');
const dropLines = (prefixes) =>
  FULL_EVIDENCE.split('\n').filter((l) => !prefixes.some((p) => l.startsWith(p))).join('\n');

// ①a 降级常态免行（ADR-0043 决策 1/8；#1859 起为校验器白名单，与窗口旧写法双白名单过渡）。
// 字段结构与窗口口径行（下方 WINDOW_LINE）一致：跳过取证不跳签收。
const DEGRADED_1A_LINE = (fields = {}) => {
  const f = {
    执行人: '@DriftingLi',
    日期: '2026-10-24',
    复测对象: '无',
    结论: '免（①a 降级发版前一次，ADR-0043）',
    ...fields,
  };
  return `- ① Android 真机逐页截图对比 — 执行人：${f.执行人} · 日期：${f.日期} · 复测对象：${f.复测对象} · 结论（含产物）：${f.结论}`;
};
/** 把 ① 行换成给定行，其余四门证据保持完整（本文件多处共用） */
const bodyWith1Line = (line) => {
  const old = FULL_EVIDENCE.split('\n').find((l) => l.startsWith('- ① '));
  return FULL_EVIDENCE.replace(old, line);
};

test('④b 打包面：pages.json 改动 → 触发 ④b，缺行即红', async () => {
  const r = await run({ files: [uvue('pages.json', { status: 'modified' })], body: FULL_EVIDENCE });
  assert.equal(r.needs4b, true);
  assert.equal(r.ok, false);
  assert.match(r.errors.join(), /④b/);
});

test('④b 打包面：新增 composables/x.uts（.uts 逻辑）→ 不触发 ④b', async () => {
  const r = await run({ files: [uvue('composables/x.uts', { status: 'added' })], body: FULL_EVIDENCE });
  assert.equal(r.needs4b, false);
  assert.equal(r.ok, true, r.errors.join('；'));
});

test('④b 打包面：.uvue 里新增 async function 的 patch → 仍不触发 ④b', async () => {
  const files = [
    uvue('pages/exam/exam.uvue', { patch: '@@ -1,2 +1,3 @@\n+export async function submit() {\n+}\n' }),
  ];
  const r = await run({ files, body: FULL_EVIDENCE });
  assert.equal(r.needs4b, false);
  assert.equal(r.ok, true, r.errors.join('；'));
});

test('④b 打包面：新增 *.aar / libs/*.jar / uni_modules 原生面改动 → 触发 ④b', async () => {
  const cases = [
    [uvue(), uvue('libs/foo.aar', { status: 'added' })],
    [uvue(), uvue('libs/foo.jar', { status: 'added' })],
    [uvue('uni_modules/uni-scan/utssdk/app-android/index.uts', { status: 'modified' })],
  ];
  for (const files of cases) {
    const r = await run({ files, body: FULL_EVIDENCE });
    assert.equal(r.needs4b, true, files.map((f) => f.filename).join(','));
    assert.equal(r.ok, false);
    assert.match(r.errors.join(), /④b/);
  }
});

test('P1：sha 匹配的 gate-evidence:④ 评论可替代正文 ④ 行（免手抄）', async () => {
  // ① 正文整行不写
  const dropped = await run({ body: dropLines(['- ④ ']), gateComments: [gate4Comment(SHA.slice(0, 7))], headSha: SHA });
  assert.equal(dropped.ok, true, dropped.errors.join('；'));
  assert.match(dropped.notes.join(), /评论/);
  assert.match(dropped.notes.join(), /sha 绑定 a20d2e2/);

  // ② 正文该行留空/写「见评论」也不算缺证据
  const blank = await run({
    body: `${dropLines(['- ④ '])}- ④（见评论） — 执行人： · 日期： · 复测对象： · 结论（含产物）：\n`,
    gateComments: [gate4Comment(SHA)],
    headSha: SHA,
  });
  assert.equal(blank.ok, true, blank.errors.join('；'));
});

test('P1：sha 不匹配的评论不算证据 → 缺 ④ 行仍红', async () => {
  const body = dropLines(['- ④ ']);
  const r = await run({ body, gateComments: [gate4Comment('deadbee')], headSha: SHA });
  assert.equal(r.ok, false);
  assert.match(r.errors.join(), /④ 本地编译门/);
});

test('P1：没给 headSha 时评论不作为证据（fail-closed）', async () => {
  const body = dropLines(['- ④ ']);
  const r = await run({ body, gateComments: [gate4Comment(SHA)] });
  assert.equal(r.ok, false);
  assert.match(r.errors.join(), /④ 本地编译门/);
});

test('P1：正文 ④ 行照写（见评论）也不受评论路径影响', async () => {
  const r = await run({ body: FULL_EVIDENCE, gateComments: [gate4Comment(SHA)], headSha: SHA });
  assert.equal(r.ok, true, r.errors.join('；'));
  assert.match(r.notes.join(), /sha 绑定 a20d2e2/);
});

// 2026-10-24 改写（#1859，ADR-0043 决策 3/8）：低风险分层退役 ⇒ 纯 .uts 不再免 ①；
// ① 行改走 ①a 降级免行（或真取证）。② 的触发面判据不变（MP-WEIXIN 面）——纯 .uts 未命中仍免 ②。
test('①a 降级常态：纯 .uts 改动不再免 ① —— ①② 行都不写 ⇒ 红（缺 ① 行）', async () => {
  const files = [uvue('utils/foo.uts'), uvue('utils/foo.test.js')];
  const body = dropLines(['- ① ', '- ② ']);
  const r = await run({ files, body });
  assert.equal(r.ok, false, '低风险分层已退役（ADR-0043 决策 8）：纯 .uts 的 ① 行不能整行不写');
  assert.match(r.errors.join(), /① Android 真机逐页截图/);
});

test('①a 降级常态：纯 .uts 改动 + ① 行写免行 ⇒ 绿（未命中 MP-WEIXIN 面 ⇒ ② 仍免）', async () => {
  const files = [uvue('utils/foo.uts'), uvue('utils/foo.test.js')];
  const body = `${dropLines(['- ② ']).replace(
    FULL_EVIDENCE.split('\n').find((l) => l.startsWith('- ① ')),
    DEGRADED_1A_LINE(),
  )}`;
  const r = await run({ files, body });
  assert.equal(r.ok, true, r.errors.join('；'));
  assert.match(r.notes.join(), /①a 降级/);
  assert.doesNotMatch(r.notes.join(), /低风险运行时面/);
});

test('①a 降级常态：.uvue 改动缺 ① 行 ⇒ 仍红（分层退役后不再有「低风险免 ①」通道）', async () => {
  const files = [uvue('pages/x/x.uvue'), uvue('utils/foo.uts')];
  const body = dropLines(['- ① ']);
  const r = await run({ files, body });
  assert.equal(r.ok, false);
  assert.match(r.errors.join(), /①/);
  assert.doesNotMatch(r.notes.join(), /低风险运行时面/);
});

// -------------------- 2026-09-12 修订：② 降为半自动门 + ② 的 sha 绑定评论（#883） --------------------

// ② 的触发面：manifest.json 改动（ADR-0008「命中 MP-WEIXIN 面」）
// 注意：manifest.json 同时命中 ④b 打包面 ⇒ 这组用例的正文需带 ④b 行（与真实 PR 一致）
const mpWeixinFiles = () => [uvue('manifest.json', { patch: '@@ -1,2 +1,3 @@\n+"mp-weixin": {}\n' })];
const GATE4B_LINE =
  '- ④b release 云打包（触及打包面时） — 执行人：人 · 日期：2026-09-11 · 复测对象：manifest.json 改动 · 结论（含产物）：cli pack 日志；发布前置条款：正式发版前必跑一次并装机自测';
const gate2Comment = (sha) =>
  [
    '<!-- gate-evidence:② -->',
    '**② 微信开发者工具无报错（半自动，agent 执行）**',
    `- commit: ${sha}`,
    '- 结论（含产物）：`MP_WEIXIN_RESULT appid=wx38c3e31b16a7ced0 pageStack=2 entry=pages/index/index errorsTotal=0 exceptionsTotal=0 shots=2 navigation=ok`；截图 `docs/verification/mp-weixin/624/entry-after.webp`；日志 `.ci-verify/mp-weixin.log`',
    '- 非等价声明：② ≠ ① 真机门，也 ≠ ④b 云打包门。',
    '- 复现：`npm run build:mp-weixin-check`',
    '',
  ].join('\n');

test('② 半自动门：② 行缺失但存在 sha 匹配的 gate-evidence:② 评论 ⇒ 绿，且 notes 提到评论', async () => {
  const body = `${dropLines(['- ② '])}\n${GATE4B_LINE}\n`;
  const r = await run({ files: mpWeixinFiles(), body, gateComments: [gate2Comment(SHA.slice(0, 7))], headSha: SHA });
  assert.equal(r.ok, true, r.errors.join('；'));
  assert.match(r.notes.join(), /评论/);
  assert.match(r.notes.join(), /sha 绑定 a20d2e2/);
  assert.match(r.notes.join(), /②/);
});

test('② 半自动门：评论 sha 与 head 不匹配 ⇒ 仍红（缺 ② 行）', async () => {
  const body = dropLines(['- ② ']);
  const r = await run({ files: mpWeixinFiles(), body, gateComments: [gate2Comment('deadbee')], headSha: SHA });
  assert.equal(r.ok, false);
  assert.match(r.errors.join(), /② 微信开发者工具无报错/);
});

test('② 半自动门：没给 headSha 时评论不作为证据（fail-closed）', async () => {
  const body = dropLines(['- ② ']);
  const r = await run({ files: mpWeixinFiles(), body, gateComments: [gate2Comment(SHA)] });
  assert.equal(r.ok, false);
  assert.match(r.errors.join(), /②/);
});

test('② 两者互不串台：只给 ④ 评论不能免 ② 行', async () => {
  const body = dropLines(['- ② ']);
  const r = await run({ files: mpWeixinFiles(), body, gateComments: [gate4Comment(SHA)], headSha: SHA });
  assert.equal(r.ok, false);
  assert.match(r.errors.join(), /②/);
});

test('② 结论须引用截图（只有自然语言结论不算产物）', async () => {
  const body = `${dropLines(['- ② '])}\n- ② 微信开发者工具无报错 — 执行人：@DriftingLi · 日期：2026-09-12 · 复测对象：入口页 pages/index/index · 结论（含产物）：入口页无报错\n${GATE4B_LINE}\n`;
  const r = await run({ files: mpWeixinFiles(), body });
  assert.equal(r.ok, false);
  assert.match(r.errors.join(), /②/);
  assert.match(r.errors.join(), /截图/);
});

test('② 结论引用 .ci-verify/entry.png **不算**产物：红（本地产物不可核验，2026-09-12 收紧）', async () => {
  const body = `${dropLines(['- ② '])}\n- ② 微信开发者工具无报错 — 执行人：@DriftingLi · 日期：2026-09-12 · 复测对象：入口页 pages/index/index · 结论（含产物）：errorsTotal=0 exceptionsTotal=0，截图 .ci-verify/entry.png\n${GATE4B_LINE}\n`;
  const r = await run({ files: mpWeixinFiles(), body });
  assert.equal(r.ok, false);
  assert.match(r.errors.join(), /②/);
  assert.match(r.errors.join(), /截图/);
});

// -------------------- 2026-09-12 修订：截图产物判据改为**结构性**（堵 .ci-verify 假绿） --------------------
// 背景：旧判据 `https?:\/\/|\.ci-verify\/\S+\.png|mp-weixin\.png|docs\/verification\/` 过宽 ——
// 在「结论」里贴个本地 png 路径就能让门**结构上被判满足**（截图从没进仓库、无人能核验）。
// 新判据只认：① GitHub 托管图片（附件直链 / Markdown 图片语法）；② 仓库内
// `docs/verification/<模块>/<PR号>/<页名>.<ext>`（PR 号必须是数字段）；③ sha 绑定门评论链接（#issuecomment-<id>）。

/** 把 ① 行的结论换掉，其余保持完整四门证据 */
const withScreenshotConclusion = (conclusion) => {
  const line = FULL_EVIDENCE.split('\n').find((l) => l.startsWith('- ① '));
  return FULL_EVIDENCE.replace(line, `- ① Android 真机逐页截图对比 — 执行人：@DriftingLi · 日期：2026-09-11 · 复测对象：exam 入口页与交卷流程 · 结论（含产物）：${conclusion}`);
};

test('结构性判据①：GitHub 附件直链 / Markdown 图片 ⇒ 绿', async () => {
  const cases = [
    '入口页截图 https://github.com/user-attachments/assets/abcdef01-2345-6789-abcd-ef0123456789',
    '入口页截图 ![entry](https://github.com/user-attachments/assets/abcdef01-2345-6789-abcd-ef0123456789)',
    '入口页截图 ![entry](https://user-images.githubusercontent.com/12345/67890-entry.png)',
    '入口页截图 ![](https://github.com/DriftingLi/FL/assets/12345/preview.webp)',
    '入口页截图 ![entry](https://anywhere.example.com/hosted/entry.png)', // Markdown 图片指向图片：算（但不可核验真伪，见「不校真伪」用例）
  ];
  for (const conclusion of cases) {
    const r = await run({ body: withScreenshotConclusion(conclusion) });
    assert.equal(r.ok, true, `${conclusion} ⇒ ${r.errors.join('；')}`);
  }
});

test('结构性判据②：仓库内 docs/verification/<模块>/<PR号>/<页名>.<ext> ⇒ 绿（jpg / webp 同样认）', async () => {
  const cases = [
    'docs/verification/exam/624/exam-home-after.jpg',
    '`docs/verification/exam/624/exam-home-after.webp`',
    '![after](docs/verification/exam/624/exam-home-after.png)',
    'https://github.com/DriftingLi/FL/blob/master/docs/verification/exam/624/exam-home-after.jpg?raw=true',
  ];
  for (const conclusion of cases) {
    const r = await run({ body: withScreenshotConclusion(conclusion) });
    assert.equal(r.ok, true, `${conclusion} ⇒ ${r.errors.join('；')}`);
  }
});

test('结构性判据③：sha 绑定门评论链接（#issuecomment-<id>）⇒ 绿', async () => {
  const cases = [
    '截图见门评论 https://github.com/DriftingLi/FL/pull/624#issuecomment-5644481824',
    '见 #issuecomment-5644481824',
  ];
  for (const conclusion of cases) {
    const r = await run({ body: withScreenshotConclusion(conclusion) });
    assert.equal(r.ok, true, `${conclusion} ⇒ ${r.errors.join('；')}`);
  }
});

test('结构性判据：裸本地产物路径 / 任意链接 ⇒ 红（旧的假绿来源）', async () => {
  const cases = [
    '.ci-verify/entry.png',
    '截图 `.ci-verify/mp-weixin-entry.png`（本地）',
    '截图 mp-weixin.png',
    '入口页无报错 https://example.com/somewhere',
    '见附件（无路径）',
    'docs/verification/exam/exam-home-after.jpg', // 缺 <PR号> 数字段：不满足 docs/verification/<模块>/<PR号>/<页名>.<ext>
    'docs/verification/exam/624/exam-home-after', // 缺扩展名
    '截图 ![entry](.ci-verify/entry.png)', // Markdown 图片语法也救不了本地产物（不在仓库里，渲染即裂图）
    '截图 ![](mp-weixin.png)',
    '入口页截图 https://example.com/a.png', // 裸 http(s) 图片直链（非 GitHub 托管）不认——要贴就贴成 Markdown 图片语法或 GitHub 附件
  ];
  for (const conclusion of cases) {
    const r = await run({ body: withScreenshotConclusion(conclusion) });
    assert.equal(r.ok, false, `${conclusion} 不该被认作截图产物`);
    assert.match(r.errors.join(), /截图/);
  }
});

test('结构性判据：② 行同样适用（裸 .ci-verify 不再假绿，GitHub 附件链接可过）', async () => {
  const red = await run({
    files: mpWeixinFiles(),
    body: `${dropLines(['- ② '])}\n- ② 微信开发者工具无报错 — 执行人：@DriftingLi · 日期：2026-09-12 · 复测对象：入口页 · 结论（含产物）：errorsTotal=0，产物 .ci-verify/entry.png\n${GATE4B_LINE}\n`,
  });
  assert.equal(red.ok, false);
  assert.match(red.errors.join(), /截图/);

  const green = await run({
    files: mpWeixinFiles(),
    body: `${dropLines(['- ② '])}\n- ② 微信开发者工具无报错 — 执行人：@DriftingLi · 日期：2026-09-12 · 复测对象：入口页 · 结论（含产物）：errorsTotal=0，截图 ![entry](https://github.com/user-attachments/assets/abcdef01-2345-6789-abcd-ef0123456789)\n${GATE4B_LINE}\n`,
  });
  assert.equal(green.ok, true, green.errors.join('；'));
});

test('结构性判据不改口径：仍只校结构、不校链接可达（不引入真伪校验）', async () => {
  // 指向仓库外的 user-images 直链一样放行——判据是「来源形态」，不校验可达性/内容
  const r = await run({ body: withScreenshotConclusion('截图 https://user-images.githubusercontent.com/00000/does-not-exist.png') });
  assert.equal(r.ok, true, r.errors.join('；'));
});

// ===========================================================================================
// 2026-09-12 修订：门证据绑定从「sha 相等」放宽为「**祖先 + 运行时面未变**」
//
// 动机（实测摩擦）：ruleset 的 strict_required_status_checks_policy（分支必须与 master 同步）
// × master 高频前进 ⇒ 每次 sync 都出新 head ⇒ 旧口径下已贴的 ④ / ② 门证据全部作废、必须重跑重贴
// （#624 反复被拖的主因）；而多数前进是无害的（只改文档/脚本/测试，运行时面逐字节未变）。
//
// 新判据三条：① commit == head ⇒ 满足；② commit 是 head 的祖先**且**其间运行时面未变 ⇒ 满足；
// ③ 其余 ⇒ 不满足。判定靠注入的 fetchCompare（量到 compare API），本文件里一律注桩。
//
// ⚠️ 注入自检：下面每个用例都断言「桩**确实被调用/未被调用**」——否则桩接错时用例会静默假绿
//    （例如把 fetchCompare 忘了传，校验器一律判不满足，写「祖先」的用例反而会红，写「不满足」的用例却会绿）。
// ===========================================================================================

const HEAD = 'bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb'; // 当前 head
const ANC = 'a111111111111111111111111111111111111111'; // 祖先：证据 commit
const anc7 = ANC.slice(0, 7);

/** 注桩：counts.calls 让用例能自检「桩真的被用了」；返回 null / 抛异常模拟 API 失败 */
const compareStub = (payload, counts = { calls: 0 }) => {
  const fn = async ({ base, head }) => {
    counts.calls += 1;
    counts.base = base;
    counts.head = head;
    if (payload instanceof Error) throw payload;
    return payload;
  };
  return { fn, counts };
};
/** 祖先：status=ahead + merge_base_commit == base，files 为「证据 commit → head」的改动集 */
const trueAncestor = (files = []) => compareStub({ status: 'ahead', mergeBaseSha: ANC, files, filesTruncated: false });

const DOC_FILES = ['docs/adr/0008-移动端验收门与证据.md', '.github/workflows/pr-evidence.yml', 'README.md'];
const UVUE_FILES = ['training-app/叉车维修培训学员端跨端应用/pages/exam/exam.uvue'];
const UTS_FILES = ['training-app/叉车维修培训学员端跨端应用/utils/exam-session.uts'];
const JSON_FILES = ['training-app/叉车维修培训学员端跨端应用/pages.json'];

/** ④ 行整行不写，只留评论：走「评论承载门证据」这条真实路径 */
const bodyWith4FromComment = () => dropLines(['- ④ ']);

test('a) 绑定判据①：证据 commit == head ⇒ 满足，且**不打** compare（旧行为回归）', async () => {
  const { fn, counts } = compareStub({ status: 'identical', mergeBaseSha: HEAD, files: [], filesTruncated: false });
  const r = await run({ body: bodyWith4FromComment(), gateComments: [gate4Comment(HEAD)], headSha: HEAD, fetchCompare: fn });
  assert.equal(r.ok, true, r.errors.join('；'));
  assert.match(r.notes.join(), /sha 绑定/);
  assert.equal(counts.calls, 0, 'commit == head 时不该调 compare API（省一次配额，也免得头事件上下文没权限时被卡）');
});

test('b) 绑定判据②：证据 commit 是 head 祖先 + 其间只有文档/脚本/测试变更 ⇒ **满足**（本次新增行为）', async () => {
  const { fn, counts } = trueAncestor(DOC_FILES);
  const r = await run({ body: bodyWith4FromComment(), gateComments: [gate4Comment(ANC)], headSha: HEAD, fetchCompare: fn });
  assert.equal(counts.calls, 1, '必须真的问了 compare —— 否则这条用例是在假绿');
  assert.equal(counts.base, ANC, 'compare 的 base 必须是**证据 commit**');
  assert.equal(counts.head, HEAD, 'compare 的 head 必须是**本次 head**');
  assert.equal(r.ok, true, r.errors.join('；'));
  const notes = r.notes.join('\n');
  assert.match(notes, /祖先/);
  assert.match(notes, new RegExp(`commit \`?${anc7}`), '放宽必须写明「是按祖先口径接受的」，不得静默放宽');
  assert.match(notes, /运行时面/);
});

test('b1) 祖先绑定时摘要行必须点名**实际绑上的旧 commit**，不得写成 head（摘要不得往「更可信」方向失真）', async () => {
  const { fn } = trueAncestor(DOC_FILES);
  const r = await run({ body: bodyWith4FromComment(), gateComments: [gate4Comment(ANC)], headSha: HEAD, fetchCompare: fn });
  assert.equal(r.ok, true, r.errors.join('；'));
  const notes = r.notes.join('\n');
  const head7 = HEAD.slice(0, 7);
  // 摘要行形如：④ 本地编译门：证据来自 PR 评论（sha 绑定 <证据 commit> —— 祖先口径，当前 head 为 <head>）。
  assert.match(notes, new RegExp(`本地编译门：证据来自 PR 评论（sha 绑定 ${anc7} `), '摘要行必须写实际绑上的 commit，而不是 head');
  assert.match(notes, /祖先口径，当前 head 为/, '祖先绑定必须显式标出「当前 head 是另一个 sha」');
  assert.doesNotMatch(notes, new RegExp(`本地编译门：证据来自 PR 评论（sha 绑定 ${head7}）`), '不得把摘要写成「绑定了 head」（旧措辞）');
});

test('b2) 祖先 + 其间**只有非运行时面**（含 .github/**、frontend/**）⇒ 满足（免重跑 churn 的主场景）', async () => {
  const files = ['.github/workflows/ci.yml', 'frontend/src/App.vue', 'scripts/foo.ps1', 'docs/x.md'];
  const { fn, counts } = trueAncestor(files);
  const r = await run({ body: bodyWith4FromComment(), gateComments: [gate4Comment(ANC)], headSha: HEAD, fetchCompare: fn });
  assert.equal(counts.calls, 1);
  assert.equal(r.ok, true, r.errors.join('；'));
});

test('c) 绑定判据②的守护边界：其间改了 .uvue / .uts / 三份 json 任一 ⇒ **不满足**，且文案要求重跑重贴', async () => {
  for (const [label, files] of [
    ['.uvue 变更', UVUE_FILES],
    ['.uts 变更', UTS_FILES],
    ['pages.json 变更', JSON_FILES],
    ['运行时面变更与其他改动混在一起', [...DOC_FILES, ...UVUE_FILES]],
  ]) {
    const { fn, counts } = trueAncestor(files);
    const r = await run({ body: bodyWith4FromComment(), gateComments: [gate4Comment(ANC)], headSha: HEAD, fetchCompare: fn });
    assert.equal(counts.calls, 1, label);
    assert.equal(r.ok, false, `${label}：运行时面变了就必须重跑，不能靠祖先放行`);
    assert.match(r.errors.join(), /④ 本地编译门/, label);
    assert.match(r.errors.join(), /运行时面文件已变更|需重跑/, label);
  }
});

test('d) 绑定判据③：证据 commit **不是** head 的祖先（compare status=diverged / behind）⇒ 不满足', async () => {
  for (const status of ['diverged', 'behind']) {
    const { fn, counts } = compareStub({ status, mergeBaseSha: 'c'.repeat(40), files: [], filesTruncated: false });
    const r = await run({ body: bodyWith4FromComment(), gateComments: [gate4Comment(ANC)], headSha: HEAD, fetchCompare: fn });
    assert.equal(counts.calls, 1, status);
    assert.equal(r.ok, false, status);
    assert.match(r.errors.join(), /④ 本地编译门/);
    assert.match(r.errors.join(), /不是当前 head 的祖先/);
  }
});

test('d2) merge_base 不是证据 commit（head 上找不到它）⇒ 不满足，fail-closed', async () => {
  const { fn } = compareStub({ status: 'ahead', mergeBaseSha: 'd'.repeat(40), files: [], filesTruncated: false });
  const r = await run({ body: bodyWith4FromComment(), gateComments: [gate4Comment(ANC)], headSha: HEAD, fetchCompare: fn });
  assert.equal(r.ok, false);
  assert.match(r.errors.join(), /不是当前 head 的祖先/);
});

test('d3) 未知 sha / compare 返回 null（GitHub 404）⇒ 不满足，且文案写明「未知 sha / API 异常」', async () => {
  const { fn, counts } = compareStub(null);
  const r = await run({ body: bodyWith4FromComment(), gateComments: [gate4Comment('deadbee')], headSha: HEAD, fetchCompare: fn });
  assert.equal(counts.calls, 1);
  assert.equal(r.ok, false);
  assert.match(r.errors.join(), /未知 sha|API 异常|compare/);
});

test('e) compare API 抛异常（超时 / 限流）⇒ **fail-closed 不满足**（绝不因异常放行）', async () => {
  const { fn, counts } = compareStub(new Error('ETIMEDOUT compare timed out'));
  const r = await run({ body: bodyWith4FromComment(), gateComments: [gate4Comment(ANC)], headSha: HEAD, fetchCompare: fn });
  assert.equal(counts.calls, 1);
  assert.equal(r.ok, false, 'API 异常必须判不满足');
  assert.match(r.errors.join(), /④ 本地编译门/);
  assert.match(r.errors.join(), /compare|API 异常|未知 sha/);
});

test('e2) 超大 diff 被 GitHub 截断（filesTruncated）⇒ fail-closed 不满足（不拿截断列表当「运行时面未变」）', async () => {
  const files = Array.from({ length: 300 }, (_, i) => `docs/x${i}.md`);
  const { fn, counts } = compareStub({ status: 'ahead', mergeBaseSha: ANC, files, filesTruncated: true });
  const r = await run({ body: bodyWith4FromComment(), gateComments: [gate4Comment(ANC)], headSha: HEAD, fetchCompare: fn });
  assert.equal(counts.calls, 1);
  assert.equal(r.ok, false);
  assert.match(r.errors.join(), /截断|300/);
});

test('e2b) **只有文件名列表、没有截断标志**的桩 ⇒ 照常按列表判据（截断与否由调用方（workflow）负判）', async () => {
  // 校验器判不了「这份 files 是不是完整」，故把「≥300 即视为截断」这条落在 workflow 侧（compareCommits 调用点）
  const files = Array.from({ length: 300 }, (_, i) => `docs/x${i}.md`);
  const { fn, counts } = compareStub({ status: 'ahead', mergeBaseSha: ANC, files, filesTruncated: false });
  const r = await run({ body: bodyWith4FromComment(), gateComments: [gate4Comment(ANC)], headSha: HEAD, fetchCompare: fn });
  assert.equal(counts.calls, 1);
  assert.equal(r.ok, true, r.errors.join('；'));
});

test('e2c) workflow 侧确实把「files ≥300 ⇒ 截断」写进了取数回调（静态自检，防止截断漏判）', () => {
  const src = readFileSync(workflowPath, 'utf8');
  assert.match(src, /COMPARE_FILES_CAP\s*=\s*300/, '缺「compare files 上限 300」的常量');
  assert.match(src, /filesTruncated:\s*fileList\.length\s*>=\s*COMPARE_FILES_CAP/, '截断标志必须按 files 数量判');
  assert.match(src, /repos\.compareCommits/, '取数回调必须走 repos.compareCommits');
});

test('e3) 完全没注入 fetchCompare（调用方忘了给）⇒ 不满足（fail-closed，不静默放行）', async () => {
  const r = await run({ body: bodyWith4FromComment(), gateComments: [gate4Comment(ANC)], headSha: HEAD });
  assert.equal(r.ok, false);
  assert.match(r.errors.join(), /④ 本地编译门/);
});

test('f) ② 走同一套判据：祖先 + 非运行时面变更 ⇒ 满足；其间改了运行时面 ⇒ 不满足', async () => {
  const body = () => `${dropLines(['- ② '])}\n${GATE4B_LINE}\n`;
  const files = mpWeixinFiles();

  const ok = trueAncestor(['docs/x.md']);
  const green = await run({ files, body: body(), gateComments: [gate2Comment(ANC)], headSha: HEAD, fetchCompare: ok.fn });
  assert.equal(ok.counts.calls, 1, '② 也必须真的问 compare');
  assert.equal(green.ok, true, green.errors.join('；'));
  assert.match(green.notes.join(), /祖先/);

  const bad = trueAncestor(UVUE_FILES);
  const red = await run({ files, body: body(), gateComments: [gate2Comment(ANC)], headSha: HEAD, fetchCompare: bad.fn });
  assert.equal(bad.counts.calls, 1);
  assert.equal(red.ok, false);
  assert.match(red.errors.join(), /② 微信开发者工具无报错/);
  assert.match(red.errors.join(), /运行时面文件已变更|需重跑/);
});

test('f2) ② 与 ④ 共用同一个 compare 结果缓存（同一 base 只查一次，省配额）', async () => {
  const counts = { calls: 0 };
  const s = compareStub({ status: 'ahead', mergeBaseSha: ANC, files: ['docs/x.md'], filesTruncated: false }, counts);
  const r = await run({
    files: mpWeixinFiles(),
    body: `${dropLines(['- ④ ', '- ② '])}\n${GATE4B_LINE}\n`,
    gateComments: [gate4Comment(ANC), gate2Comment(ANC)],
    headSha: HEAD,
    fetchCompare: s.fn,
  });
  assert.equal(r.ok, true, r.errors.join('；'));
  assert.equal(counts.calls, 1, '同一 base 的 compare 只该查一次');
});

test('g) 注入自检（防空跑假绿）：**注桩被调用**才算真验证；桩不被调用时该用例必须失败', async () => {
  // 守卫这条自检本身：桩换个「永远返回祖先且文件为空」以外的实现，用例结论会跟着变。
  const onlyRuntime = trueAncestor(UVUE_FILES);
  const green = trueAncestor([]);
  assert.equal(onlyRuntime.counts.calls, 0);
  assert.equal(green.counts.calls, 0);

  const rRed = await run({ body: bodyWith4FromComment(), gateComments: [gate4Comment(ANC)], headSha: HEAD, fetchCompare: onlyRuntime.fn });
  const rGreen = await run({ body: bodyWith4FromComment(), gateComments: [gate4Comment(ANC)], headSha: HEAD, fetchCompare: green.fn });
  assert.equal(onlyRuntime.counts.calls, 1, '红用例必须真的调用过桩');
  assert.equal(green.counts.calls, 1, '绿用例必须真的调用过桩');
  assert.equal(rRed.ok, false);
  assert.equal(rGreen.ok, true);
  // 两条路径结论相反 ⇒ 判据确实取决于 compare 的 files，而不是恒红/恒绿
  assert.notEqual(rRed.ok, rGreen.ok);
});

test('h) 放宽不放真伪：compare 结果里只有文件名判据，不看 patch/内容（仍只校绑定形态）', async () => {
  // 文件列表里带一个**非运行时面但内容可疑**的文件 → 照旧满足（校验器不读内容，与 ADR「不校真伪」一致）
  const { fn } = trueAncestor(['.github/workflows/pr-evidence.yml', 'scripts/evil.ps1']);
  const r = await run({ body: bodyWith4FromComment(), gateComments: [gate4Comment(ANC)], headSha: HEAD, fetchCompare: fn });
  assert.equal(r.ok, true, r.errors.join('；'));
});

test('i) 未绑上的门证据在**该门被跳过**时也必须留痕（notes），不得静默丢弃', async () => {
  // 纯 .uts（无 MP-WEIXIN 面）⇒ ② 免行；① 走 ①a 降级免行（低风险分层已退役，#1859）。
  // 给一条「其间改了运行时面」的 ② 评论：② 未被点亮 ⇒ 这条评论的失败被触发条件跳过，必须留痕。
  const files = [uvue('utils/foo.uts')];
  const body = bodyWith1Line(DEGRADED_1A_LINE()).split('\n').filter((l) => !l.startsWith('- ② ')).join('\n');
  const { fn } = trueAncestor(UVUE_FILES);
  const r = await run({ files, body, gateComments: [gate2Comment(ANC)], headSha: HEAD, fetchCompare: fn });
  assert.equal(r.ok, true, r.errors.join('；'));
  assert.match(r.notes.join(), /② 门证据未绑上/);
});

test('j) 候选评论里有一条绑上就算满足（老评论作废、新评论有效时不得因老评论报红）', async () => {
  const { fn, counts } = trueAncestor(['docs/x.md']);
  const r = await run({
    body: bodyWith4FromComment(),
    gateComments: [gate4Comment('deadbee'), gate4Comment(ANC)],
    headSha: HEAD,
    fetchCompare: fn,
  });
  assert.equal(counts.calls, 2, '两条候选都要判');
  assert.equal(r.ok, true, r.errors.join('；'));
  assert.match(r.notes.join(), /祖先/);
});

test('j2) 老评论（绑旧 sha）+ 新评论（绑当前 head）共存 ⇒ 认新评论，且**不打 compare**', async () => {
  const { fn, counts } = trueAncestor(['docs/x.md']);
  const r = await run({
    body: bodyWith4FromComment(),
    gateComments: [gate4Comment(ANC), gate4Comment(HEAD)],
    headSha: HEAD,
    fetchCompare: fn,
  });
  assert.equal(r.ok, true, r.errors.join('；'));
  assert.equal(counts.calls, 0, '「恰好等于 head」的候选必须优先判（既省配额，也不让 API 抖动把可用评论拖红）');
  assert.doesNotMatch(r.notes.join(), /绑定口径/, '按 sha 相等口径绑上的不必记「放宽」说明');
});

// ===========================================================================================
// 2026-09-15 修订（#1030）：② 的触发判据从「改动文本里出现过 MP-WEIXIN 字面量」
// 收窄为「**条件编译指令行**的增删（且限定运行时面文件）+ 两份 json 的文件清单」。
//
// 旧判据 `(f.patch || '').includes('MP-WEIXIN')` 判的是「文本里出现过这个字符串」，与**移动端**
// `docs/adr/0008-移动端验收门与证据.md` 的「命中 MP-WEIXIN **面**（交付形态）」不是一回事（更严），实害有二：
//   ① 误报（#1028 实测）：在 ADR/文档里写「② 免（未命中 MP-WEIXIN 面）」这句话**本身**就把门点亮，
//      改写措辞（「未命中微信小程序面」）后即消失；同族还有 `utils/useBiometric.test.js` 用字符串
//      **断言** `// #ifdef APP || MP-WEIXIN` ⇒ 改那个测试文件也会凭空要求 ②；
//   ② 反向激励：想不被误报只能**不写**这个术语 —— 而该 ADR 恰恰要求把这类口径写进文档。
// 本节的用例是这次修订的判别力证明：负例（提到 ≠ 命中）与正例（真指令增删仍必须点亮）成对存在。
// ===========================================================================================

/** 只在文本里「提到」该术语的文件（不是交付面）：patch 里那句就是 #1028 踩到的那句 */
const mentionsOnly = (filename, added) => ({
  filename,
  status: 'modified',
  patch: `@@ -86,3 +86,4 @@\n+${added}\n`,
});
const ADR_0014 = 'training-app/叉车维修培训学员端跨端应用/docs/adr/0014-移动端全局搜索落点与投影口径.md';

test('#1030 负例：ADR/文档里写「未命中 MP-WEIXIN 面」不再点亮 ②（正文缺 ② 行仍绿）', async () => {
  const files = [
    uvue(),
    mentionsOnly(ADR_0014, '> **二轮门证据**：③ CI run 全绿；**②** 免（未命中 MP-WEIXIN 面：无条件编译段、未改两份 json）。'),
  ];
  const r = await run({ files, body: dropLines(['- ② ']) });
  assert.equal(r.ok, true, r.errors.join('；'));
  assert.match(r.notes.join(), /第②门免/);
});

test('#1030 负例：*.test.js 里的指令断言字符串不算交付面（同族误报，改测试不再凭空要求 ②）', async () => {
  const files = [
    uvue(),
    mentionsOnly(
      'training-app/叉车维修培训学员端跨端应用/utils/useBiometric.test.js',
      "    const app = () => branch('// #ifdef APP || MP-WEIXIN');",
    ),
  ];
  const r = await run({ files, body: dropLines(['- ② ']) });
  assert.equal(r.ok, true, r.errors.join('；'));
  assert.match(r.notes.join(), /第②门免/);
});

test('#1030 负例：非运行时面文件里引一条真指令（.md 的代码块）也不算 —— 文件类型与行形态都要满足', async () => {
  const files = [uvue(), mentionsOnly('docs/agents/handoff-验收门-2026-09-11.md', '```js\n// #ifdef MP-WEIXIN\n```')];
  const r = await run({ files, body: dropLines(['- ② ']) });
  assert.equal(r.ok, true, r.errors.join('；'));
  assert.match(r.notes.join(), /第②门免/);
});

test('#1030 负例：运行时面文件里的散文提及 / 非行首字样不算（判据是「指令行」不是「这个词」）', async () => {
  const cases = [
    ['中文散文提及', uvue('utils/x.uts', { patch: '@@ -1,2 +1,3 @@\n+// 本模块不涉及 MP-WEIXIN 面，无需条件编译\n' })],
    ['断言字符串形态', uvue('utils/x.uts', { patch: '@@ -1,2 +1,3 @@\n+  expect(src).toContain("// #ifdef APP 与 MP-WEIXIN 分支");\n' })],
    ['没有 # 指令前缀', uvue('utils/x.uts', { patch: '@@ -1,2 +1,3 @@\n+// MP-WEIXIN 面未命中\n' })],
    // 判据只看**条件表达式**那一段：真指令行后面再写散文提及（表达式之外）不算命中。
    // 少了这条收窄，「指令行 + 同行解释」会把门凭空点亮 —— 与 #1030 要修的是同一个病。
    ['指令行 + 同行散文提及（表达式之外不判）', uvue('utils/x.uts', { patch: '@@ -1,2 +1,3 @@\n+// #ifdef APP-PLUS // 与 MP-WEIXIN 分支不同\n' })],
  ];
  for (const [label, file] of cases) {
    const r = await run({ files: [uvue(), file], body: dropLines(['- ② ']) });
    assert.equal(r.ok, true, `${label} ⇒ ${r.errors.join('；')}`);
    assert.match(r.notes.join(), /第②门免/, label);
  }
});

test('#1030 正例（判别力）：真正的条件编译指令行增删仍点亮 ②，缺 ② 行即红', async () => {
  const cases = [
    ['新增 .uvue 指令', uvue('pages/login/login.uvue', { patch: '@@ -1,2 +1,3 @@\n+// #ifdef MP-WEIXIN\n+// #endif\n' })],
    ['删除 .uvue 指令（交付形态同样变了）', uvue('pages/login/login.uvue', { patch: '@@ -1,3 +1,2 @@\n-// #ifndef MP-WEIXIN\n' })],
    ['多平台表达式 APP || MP-WEIXIN', uvue('composables/useBiometric.uts', { patch: '@@ -1,2 +1,3 @@\n+        // #ifdef APP || MP-WEIXIN\n' })],
    ['模板注释形态 <!-- #ifdef … -->', uvue('pages/login/login.uvue', { patch: '@@ -1,2 +1,3 @@\n+            <!-- #ifdef MP-WEIXIN -->\n' })],
    ['样式块注释形态 /* #ifdef … */', uvue('pages/login/login.uvue', { patch: '@@ -1,2 +1,3 @@\n+  /* #ifdef MP-WEIXIN */\n' })],
    ['pages.json 的条件编译键', uvue('pages.json', { patch: '@@ -1,2 +1,3 @@\n+  "#ifdef MP-WEIXIN": {}\n' })],
  ];
  for (const [label, file] of cases) {
    // pages.json 同时命中 ④b 打包面 ⇒ 带上 ④b 行，让红的原因只可能是 ②
    const r = await run({ files: [uvue(), file], body: `${dropLines(['- ② '])}\n${GATE4B_LINE}\n` });
    assert.equal(r.ok, false, `${label}：应要求 ② 行`);
    assert.match(r.errors.join(), /② 微信开发者工具无报错/, label);
    assert.match(r.notes.join(), /② 触发面命中/, `${label}：点亮理由要写在摘要里（#1030 的可见性诉求）`);
  }
});

test('#1030：② 未被点亮时不写「触发面命中」，点亮时摘要点名是哪个文件', async () => {
  const off = await run({ files: [uvue()], body: FULL_EVIDENCE });
  assert.match(off.notes.join(), /第②门免/);
  assert.doesNotMatch(off.notes.join(), /② 触发面命中/);

  const on = await run({ files: mpWeixinFiles(), body: `${dropLines(['- ② '])}\n${GATE4B_LINE}\n` });
  const notes = on.notes.join('\n');
  assert.match(notes, /② 触发面命中/);
  assert.match(notes, /manifest\.json/, '摘要必须点名命中文件，别让人去猜门为什么亮');
});

test('#1030 静态自检：判据不得退回「扫改动文本里的字面量」（文本判据与语义脱节的复发防线）', () => {
  const src = readFileSync(validatorPath, 'utf8');
  // 只对**代码行**判：修订说明里刻意引用了旧判据原文（`.includes('MP-WEIXIN')`）作对照，
  // 拿全文判会误报帮助文本（ADR-0008 记的 C14 就是这一坑位）。
  const codeLines = src
    .split('\n')
    .filter((l) => !/^\s*(?:\/\/|#)/.test(l))
    .join('\n');
  assert.doesNotMatch(codeLines, /\.includes\('MP-WEIXIN'\)/, '不得再按「文本里出现过 MP-WEIXIN」判 ②');
  assert.match(src, /const MP_WEIXIN_DIRECTIVE\s*=/, '缺「条件编译指令行」的显式判据');
  assert.match(src, /hasMpWeixinDirective\(f\.patch\)/, '② 的 patch 面必须走指令行判据');
  assert.match(
    src,
    /isRuntimeFile\(f\.filename\)\s*&&\s*hasMpWeixinDirective\(f\.patch\)/,
    '② 的 patch 面必须限定在运行时面文件（否则文档/测试又能点亮它）',
  );
});

// ============================================================
// 模板门行形状锁（#1403：验收证据「复述品」漂移防线）
// ------------------------------------------------------------
// 为什么建这把锁、为什么不建「门规格表」（决策语境，改门前先读）：
//   门语法有**三处真源/复述品**：① 校验器内联的 GATES（`.github/workflows/pr-evidence.yml`，
//   **权威真源**）、② 生成器 `evidence-gen.ps1` 的字符串模板、③ PR 模板手写的 5 行门。
//   #1403 实测：生成器吐「多行块」而校验器按「每门一行」取门 ⇒ ①③④ 全判「缺该行」⇒
//   移动端 PR 一开即红、逼人手工重排结构。②的漂移已被 `utils/evidenceGenContract.test.js`
//   的**真跑成对锁**堵住（生成产物喂进校验器）；③这一处**未锁**——本测试补的就是它。
//   为什么不抽「门规格表」统一驱动三处：校验器刻意**内联、不 checkout**（ADR-0008 的唯一豁免），
//   共享 lib 文件**读不到** ⇒ 表只能内联副本 + 逐字相等锁 ⇒ **校验器仍是唯一事实源、表只是它的镜像**，
//   镜像只有两个下游时它是「复述品」而非「真源」——用锁把复述品包装成真源是**假整合**（同 ADR-0020
//   「半死配置」的味道）。本仓历史门漂移都发生在**产物判据 / 触发面**，从不在**门行字段清单**，
//   故这把锁**只咬形状**（S1）不咬字段集（S3）：模板门行被改回多行、或某门行丢失 ⇒ 红；
//   真哪天加了带新必填字段的门，再补 S3 的抽取式不迟（YAGNI）。
//
// 只断言「形状可识别（无缺该行）」，**不**断言整段判绿：模板行是「待填骨架」，
//   结论/执行人冒号后本就空，校验器报「为空/占位」是**对的**——那正是它提示人来填的机制。
// ============================================================
const templatePath = path.resolve(
  path.dirname(fileURLToPath(import.meta.url)),
  '..',
  'PULL_REQUEST_TEMPLATE.md',
);

test('模板门行形状锁：PR 模板「## 验收证据」的 5 行门必须逐门被校验器识别（无「缺该行」）', async () => {
  const template = readFileSync(templatePath, 'utf8');
  // 只取「## 验收证据」段的门行（剔掉 HTML 注释与周边散文，同校验器的切段口径）——
  // 直接喂整份模板会因正文含「已接受未验证风险」+「事后验证计划」而误走例外通道（errors 被清空、
  // 转写进 notes），使下面两条断言双双失真（一个恒真、一个必假）。只留门行，测的才是「形状」本身。
  const clean = template.replace(/<!--[\s\S]*?-->/g, '');
  const head = clean.match(/^##\s*验收证据\s*$/m);
  assert.ok(head, '模板必须含「## 验收证据」段（否则本锁无从取门行）');
  const rest = clean.slice(head.index + head[0].length);
  const nxt = rest.search(/^##\s/m);
  const section = nxt === -1 ? rest : rest.slice(0, nxt);
  const gateLines = section.split('\n').filter((l) => /^\s*[-*]\s/.test(l));
  const body = `## 验收证据\n\n${gateLines.join('\n')}\n`;

  // manifest.json 同时命中 ②（MP-WEIXIN json）与 ④b（打包面）⇒ 校验器强制检查全部 5 门
  const files = mpWeixinFiles();
  const r = await run({ files, body });
  // 防空转：必须先确认真的进入「命中运行时面 → 校验门」分支——否则 runtime.length===0 会让校验器
  //   在检查门之前就早退 ok=true，上面那句「无缺该行」就成了恒真的假绿（spec §④ 更正 3）。
  assert.ok(r.runtime.length > 0, '未命中运行时面 ⇒ 门根本没被检查，这把锁是空的');
  assert.ok(r.needs4b, 'manifest.json 应点亮 ④b 打包面，否则模板 ④b 行没被检查到');
  const missing = r.errors.filter((e) => /缺该行/.test(e));
  assert.deepEqual(missing, [], `模板门行形状漂移（被改成多行 / 某门行丢失）：\n- ${missing.join('\n- ')}`);

  // 必红对照（成对，spec §⑥ 判据③「只跑通过那次不算验收」）：删掉模板 ③ 行 ⇒ 必须报「③…缺该行」，
  //   否则本锁抓不住门行丢失。若 heading 检测本身失灵，这条会连带不红——于是它把「绿用例非恒真」也一并证了。
  const withoutGate3 = `## 验收证据\n\n${gateLines.filter((l) => !/^\s*-\s*③/.test(l)).join('\n')}\n`;
  const r2 = await run({ files, body: withoutGate3 });
  assert.ok(
    r2.errors.some((e) => e.startsWith('③') && /缺该行/.test(e)),
    '删掉模板 ③ 行竟没报「缺该行」⇒ 锁恒绿、抓不住门行丢失',
  );
});

// ============================================================
// gates 结构化字段（2026-10-16 新增，移动端 ADR-0041 决策 3）
//   `.github/workflows/mobile-gates.yml` 的门证据管家从 head 取 `.github/scripts/lib/gate-validator.mjs`
//   （#1770 起；取不到才退回抽 pr-evidence.yml 的标注块）原样跑一遍，再把 `gates` 渲染成 PR 上的一条常驻评论。
//   **判据同源**靠的就是这几条断言：
//   - required 由**触发面**决定（② 看 MP-WEIXIN 面，④ 看有无运行时面），
//   - bound   由**门评论绑定**决定（等价于 upper 里 gateComment() 的 hit），
//   两者互不冒充：没有触发面时 required=false（免门不应渲染成「缺证据」），
//   有触发面但无评论时 bound=false 且 commit 为空（不得把 head 兜底成「已绑」）。
//   成对取证（spec §⑥ 判据③）：下面既有「该 true」也有「该 false」的对照，缺一半即恒绿。
// ============================================================
test('gates：非运行时面 PR ⇒ ②④ 都不 required（免门不冒充缺证据）', async () => {
  const r = await run({ files: [docs], body: '# 改了什么\n\n只改文档。\n' });
  assert.equal(r.gates['②'].required, false);
  assert.equal(r.gates['④'].required, false);
  assert.equal(r.gates['④'].bound, false);
});

test('gates：纯 .uvue 改动无门评论 ⇒ ④ required 但未绑（commit 不得兜底成 head）', async () => {
  const r = await run({ files: [uvue()], body: FULL_EVIDENCE, headSha: SHA });
  assert.equal(r.gates['④'].required, true);
  assert.equal(r.gates['④'].bound, false);
  assert.equal(r.gates['④'].commit, '');
});

test('gates：④ 门评论绑当前 head ⇒ bound 且回填实际 commit 前 7 位', async () => {
  const r = await run({
    files: [uvue()],
    body: bodyWith4FromComment(),
    headSha: SHA,
    gateComments: [gate4Comment(SHA)],
  });
  assert.equal(r.gates['④'].bound, true);
  assert.equal(r.gates['④'].commit, SHA.slice(0, 7));
  assert.equal(r.gates['④'].reason, '');
});

test('gates：② required 由 MP-WEIXIN 面点亮；纯 .uts 命中条件编译同样点亮（低风险分层已退役）', async () => {
  const hit = await run({
    files: mpWeixinFiles(),
    body: FULL_EVIDENCE,
    headSha: SHA,
    gateComments: [gate2Comment(SHA)],
  });
  assert.equal(hit.gates['②'].required, true);
  assert.equal(hit.gates['②'].bound, true);
  assert.equal(hit.gates['②'].commit, SHA.slice(0, 7));

  // 旧「低风险 .uts 面 ⇒ 免 ②」已退役（ADR-0043 决策 3/8，#1859）：
  // 纯 .uts 改动只要 diff 增删 MP-WEIXIN 条件编译指令行 ⇒ ② 照样 required（决策 3 的对称代价）。
  const utsDirective = await run({
    files: [uvue('composables/useBiometric.uts', { patch: '@@ -1,2 +1,3 @@\n+        // #ifdef APP || MP-WEIXIN\n' })],
    body: '',
    headSha: SHA,
  });
  assert.equal(utsDirective.gates['②'].required, true, '纯 .uts 命中 MP-WEIXIN 条件编译 ⇒ ② 必跑');
  assert.equal(utsDirective.gates['④'].required, true);

  // 未命中 MP-WEIXIN 面的纯 .uts ⇒ ② 仍免：触发面判的是交付形态（json / 条件编译），不是风险分层。
  const plain = await run({
    files: [{ filename: 'training-app/叉车维修培训学员端跨端应用/utils/exam-session.uts', status: 'modified', patch: '' }],
    body: '',
    headSha: SHA,
  });
  assert.equal(plain.gates['②'].required, false);
  assert.equal(plain.gates['④'].required, true);
});

// ============================================================
// ① 门窗口期口径（#1766，2026-10-22）
//   移动端 ADR-0039 / 0040 决策 1：小程序优先窗口期内真机逐页取证**不跑**，① 门「结论」如实写
//   `SKIP·待批收口`。票里判据（grill Q12 = I）：
//   - 只认**带「待批收口」字样**的结论标记；裸 `SKIP` / 其他跳过措辞一律仍走截图判据（判红）；
//   - 跳过的是「取证」不是「签收」：执行人须非占位、日期须为 YYYY-MM-DD；窗口期无实机可测，
//     故「复测对象：无」在跳过时不算占位；
//   - ②③④④b 判据一字不动（窗口口径不是万能免死金牌）；
//   - 结构化 gates['①'] 必须让「跳过（窗口口径）」与「已取证 / 还缺」可区分（跳过必须可见）。
//   2026-10-24（#1859，ADR-0043）：窗口机制已被 0043 退役，本组用例转为**过渡期双白名单**的
//   旧通道回归——新口径见下方「①a 降级常态」组；首个用新写法完整过门的运行时面 PR 出现后
//   立小票删旧通道（#1859 退出判据）。
// ============================================================
const WINDOW_LINE = (fields = {}) => {
  const f = {
    执行人: '@DriftingLi',
    日期: '2026-10-22',
    复测对象: '无',
    结论: 'SKIP·待批收口（ADR-0039 窗口口径）',
    ...fields,
  };
  return `- ① Android 真机逐页截图对比 — 执行人：${f.执行人} · 日期：${f.日期} · 复测对象：${f.复测对象} · 结论（含产物）：${f.结论}`;
};
/** 把 ① 行换成给定行，其余四门证据保持完整 —— 实现已上移至文件顶部（bodyWith1Line，#1859 起多处共用） */
const bodyWith1 = bodyWith1Line;

test('① 窗口期：结论写 SKIP·待批收口（复测对象：无）⇒ 绿，notes 明示过渡期旧写法跳过', async () => {
  const r = await run({ body: bodyWith1(WINDOW_LINE()) });
  assert.equal(r.ok, true, r.errors.join('；'));
  assert.match(r.notes.join(), /跳过（窗口口径·过渡期旧写法）/);
  assert.match(r.notes.join(), /ADR-0043 退役/, '旧通道的 notes 必须点明窗口已被 0043 退役（不得读成窗口仍有效）');

  // 成对对照：「复测对象：无」的豁免只属于窗口口径 —— 同样写「无」但结论换成真截图 ⇒ 仍按占位判红。
  const red = await run({
    body: bodyWith1(WINDOW_LINE({ 结论: '逐页截图 docs/verification/exam/624/exam-home-after.jpg' })),
  });
  assert.equal(red.ok, false, '非窗口口径的「复测对象：无」必须仍按占位判红（豁免不许外溢）');
  assert.match(red.errors.join(), /复测对象/);
});

test('① 窗口期：只认带「待批收口」的标记 —— 裸 SKIP / 跳过 / 待批 ⇒ 仍红', async () => {
  for (const conclusion of ['SKIP', '跳过（窗口期）', '待批', 'SKIP·待确认', '待人工']) {
    const r = await run({ body: bodyWith1(WINDOW_LINE({ 结论: conclusion })) });
    assert.equal(r.ok, false, `「${conclusion}」不该被认作窗口口径标记`);
    assert.match(r.errors.join(), /截图/, `「${conclusion}」应仍按截图判据报错`);
  }
});

test('① 窗口期：跳过不等于免签收 —— 执行人 / 日期仍是硬判据', async () => {
  const exec = await run({ body: bodyWith1(WINDOW_LINE({ 执行人: '待人工' })) });
  assert.equal(exec.ok, false);
  assert.match(exec.errors.join(), /执行人/);

  const date = await run({ body: bodyWith1(WINDOW_LINE({ 日期: '窗口期退出后' })) });
  assert.equal(date.ok, false);
  assert.match(date.errors.join(), /日期/);

  // 反向断言（管家渲染诚实性）：这行不合格 ⇒ 不得记成「跳过」，否则管家会替一条不合格的行渲染
  // 「⏭ 跳过（窗口口径）」，把「还没签收」显示成「按窗口口径免了」。
  assert.equal(exec.gates['①'].skipped, false, '执行人占位时不许记成跳过');
  assert.equal(date.gates['①'].skipped, false, '日期不合格时不许记成跳过');
});

test('① 窗口期：豁免只认「无 / 暂无」—— 复测对象写「待人工」仍按占位判红', async () => {
  const r = await run({ body: bodyWith1(WINDOW_LINE({ 复测对象: '待人工' })) });
  assert.equal(r.ok, false);
  assert.match(r.errors.join(), /复测对象/);
  assert.equal(r.gates['①'].skipped, false);
});

test('① 窗口期：已有截图 + 文中提到「待批收口」不得被误判成跳过（标号必须打头）', async () => {
  const r = await run({
    body: bodyWith1(WINDOW_LINE({
      复测对象: 'exam 入口页与交卷流程',
      结论: '逐页截图 docs/verification/exam/624/exam-home-after.jpg（待批收口后再补交卷页）',
    })),
  });
  assert.equal(r.ok, true, r.errors.join('；'));
  assert.equal(r.gates['①'].skipped, false, '有真截图却被渲染成「跳过」= 往更不可信方向失真');
  assert.equal(r.gates['①'].bound, true);
  assert.doesNotMatch(r.notes.join(), /跳过（窗口口径）/);
});

test('gates：① 有可核验截图但本行另有错 ⇒ ok 为红时 bound 不得为 true（管家不许显示「已取证」）', async () => {
  const r = await run({
    body: bodyWith1(WINDOW_LINE({ 执行人: '待人工', 结论: '逐页截图 docs/verification/exam/624/exam-home-after.jpg' })),
  });
  assert.equal(r.ok, false);
  assert.match(r.errors.join(), /执行人/);
  assert.equal(r.gates['①'].bound, false);
});

test('① 窗口期：标记只对 ① 生效 —— ② 结论写同样的字句仍按截图判据判红', async () => {
  const r = await run({
    files: mpWeixinFiles(),
    body: FULL_EVIDENCE.replace(/结论（含产物）：入口页无报错 [^\n]*/, '结论（含产物）：SKIP·待批收口'),
  });
  assert.equal(r.ok, false);
  assert.match(r.errors.join(), /②/);
  assert.match(r.errors.join(), /截图/);
});

test('① 窗口期：也不能替别的门兜底 —— ③ 行缺失时照样红', async () => {
  const r = await run({ body: bodyWith1(WINDOW_LINE()).replace(/^- ③ .*\n/m, '') });
  assert.equal(r.ok, false);
  assert.match(r.errors.join(), /③/);
  assert.doesNotMatch(r.errors.join(), /①/);
});

test('gates：① 区分「窗口口径跳过」/「已取证」/「还缺」三种状态（跳过必须可见）', async () => {
  const skipped = await run({ body: bodyWith1(WINDOW_LINE()) });
  assert.equal(skipped.gates['①'].required, true);
  assert.equal(skipped.gates['①'].skipped, true);
  assert.equal(skipped.gates['①'].bound, false, '跳过不是取证 ⇒ bound 必须为 false');

  const evidence = await run({ body: FULL_EVIDENCE });
  assert.equal(evidence.gates['①'].skipped, false);
  assert.equal(evidence.gates['①'].bound, true);

  const missing = await run({ body: dropLines(['- ① ']) });
  assert.equal(missing.gates['①'].required, true);
  assert.equal(missing.gates['①'].bound, false);
  assert.equal(missing.gates['①'].skipped, false);
  assert.equal(missing.ok, false);
});

test('gates：纯 .uts 面 ① 仍 required（低风险分层退役，#1859）——免行写得对才 skipped', async () => {
  const r = await run({
    files: [{ filename: 'training-app/叉车维修培训学员端跨端应用/utils/exam-session.uts', status: 'modified', patch: '' }],
    body: bodyWith1Line(DEGRADED_1A_LINE()),
  });
  assert.equal(r.gates['①'].required, true, '退役后 ① 的 required 只看运行时面命中，不再看风险分层');
  assert.equal(r.gates['①'].skipped, true);
  assert.equal(r.gates['①'].bound, false);
  assert.equal(r.ok, true, r.errors.join('；'));
});

// ============================================================
// ①a 降级常态免行（ADR-0043 决策 1/8，#1859 新白名单；与上组窗口旧写法构成双白名单过渡）
//   · 判据只认字样不认文件：结论以 `免` 开头 + 含「①a 降级发版前一次」+ 含「ADR-0043」——
//     文档包（#1858）与本判据谁先合入都不悬空；
//   · 与窗口口径同构：跳过的是「取证」不是「签收」（执行人 / 日期硬判据照走、复测对象可写「无」）；
//   · ②③④④b 一律不受影响；裸 `免` / 缺字样 ⇒ 仍走截图判据判红。
// ============================================================
test('①a 降级常态：结论写免行（复测对象：无）⇒ 绿，notes 明示「①a 降级」且 gates 记 skipped', async () => {
  const r = await run({ body: bodyWith1Line(DEGRADED_1A_LINE()) });
  assert.equal(r.ok, true, r.errors.join('；'));
  assert.match(r.notes.join(), /①a 降级/);
  assert.equal(r.gates['①'].skipped, true);
  assert.equal(r.gates['①'].bound, false, '免行不是取证 ⇒ bound 必须为 false');

  // 成对对照：「复测对象：无」的豁免只属于免行/跳过口径 —— 同样写「无」但结论换成真截图 ⇒ 仍按占位判红。
  const red = await run({
    body: bodyWith1Line(DEGRADED_1A_LINE({ 结论: '逐页截图 docs/verification/exam/624/exam-home-after.jpg' })),
  });
  assert.equal(red.ok, false, '非免行口径的「复测对象：无」必须仍按占位判红（豁免不许外溢）');
  assert.match(red.errors.join(), /复测对象/);
});

test('①a 降级常态：只认完整字样 —— 裸「免」/ 缺「降级发版前一次」/ 缺「ADR-0043」⇒ 仍红', async () => {
  for (const conclusion of [
    '免',
    '免（①a 降级）',
    '免（①a 降级发版前一次）',
    '免（低风险运行时面：仅 .uts 逻辑改动）',
    '①a 降级发版前一次，ADR-0043（不以「免」开头）',
  ]) {
    const r = await run({ body: bodyWith1Line(DEGRADED_1A_LINE({ 结论: conclusion })) });
    assert.equal(r.ok, false, `「${conclusion}」不该被认作 ①a 降级免行`);
    assert.match(r.errors.join(), /截图/, `「${conclusion}」应仍按截图判据报错`);
    assert.equal(r.gates['①'].skipped, false, `「${conclusion}」不得记成跳过`);
  }
});

test('①a 降级常态：免行不等于免签收 —— 执行人 / 日期 / 复测对象仍是硬判据', async () => {
  const exec = await run({ body: bodyWith1Line(DEGRADED_1A_LINE({ 执行人: '待人工' })) });
  assert.equal(exec.ok, false);
  assert.match(exec.errors.join(), /执行人/);
  assert.equal(exec.gates['①'].skipped, false, '执行人占位时不许记成跳过');

  const date = await run({ body: bodyWith1Line(DEGRADED_1A_LINE({ 日期: '发版前' })) });
  assert.equal(date.ok, false);
  assert.match(date.errors.join(), /日期/);
  assert.equal(date.gates['①'].skipped, false, '日期不合格时不许记成跳过');

  const target = await run({ body: bodyWith1Line(DEGRADED_1A_LINE({ 复测对象: '待人工' })) });
  assert.equal(target.ok, false);
  assert.match(target.errors.join(), /复测对象/);
});

test('①a 降级常态：免行只对 ① 生效 —— ② 结论写同样字句仍按截图判据判红', async () => {
  const r = await run({
    files: mpWeixinFiles(),
    body: FULL_EVIDENCE.replace(/结论（含产物）：入口页无报错 [^\n]*/, '结论（含产物）：免（①a 降级发版前一次，ADR-0043）'),
  });
  assert.equal(r.ok, false);
  assert.match(r.errors.join(), /②/);
  assert.match(r.errors.join(), /截图/);
});

test('①a 降级常态：也不能替别的门兜底 —— ③ 行缺失时照样红', async () => {
  const r = await run({ body: bodyWith1Line(DEGRADED_1A_LINE()).replace(/^- ③ .*\n/m, '') });
  assert.equal(r.ok, false);
  assert.match(r.errors.join(), /③/);
  assert.doesNotMatch(r.errors.join(), /①/);
});

test('静态自检：管家渲染 ① 行时必须区分「跳过（免行口径）」与 ✅/❌（跳过不许长得像已取证）', () => {
  const gatesPath = path.resolve(path.dirname(fileURLToPath(import.meta.url)), '..', 'workflows', 'mobile-gates.yml');
  const src = readFileSync(gatesPath, 'utf8');
  assert.match(src, /result\.gates\['①'\]/, '管家没读 gates[①] ⇒ ① 行的跳过状态无从渲染');
  assert.match(src, /g1\.skipped\s*\?/, '管家没按 skipped 分支渲染 ⇒ 跳过会与「已取证」混为一谈');
  assert.match(src, /跳过（①a 降级常态 \/ 窗口过渡）/, '管家渲染文案里缺少「跳过（①a 降级常态 / 窗口过渡）」标记（双白名过渡，#1859）');
  assert.match(src, /① Android 真机逐页截图/, '管家表格里没有 ① 行 ⇒ 免行跳过在 PR 上不可见');
  // 兜底不许把「算不出来」写成「免」：head 校验器读不到时会退回 base 旧版（不产 gates[①]），
  // 此时 required 必须由 runtime 现算，否则 .uvue PR 会被渲染成「免门」。
  assert.match(src, /required:\s*\(result\.runtime/, '管家兜底把 ① 默认成免门 ⇒ 旧校验器下 .uvue PR 会被错误显示为免');
});

// ============================================================
// #1770 S2：证据主键改为 commit sha —— commit 评论与 PR 评论**合并**成同一候选集
// ============================================================

test('#1770 S2 证据只贴在 commit 评论上（还没开 PR / 开 PR 后没再贴）⇒ ④ 照常绑上并判绿', async () => {
  const body = dropLines(['- ④ ']);
  const r = await run({ body, commitComments: [gate4Comment(SHA)], headSha: SHA });
  assert.equal(r.ok, true, r.errors.join('；'));
});

test('#1770 S2 commit 评论与 PR 评论合并判定：坏的那条不拖红绑得上的那条', async () => {
  const body = dropLines(['- ④ ']);
  const good = await run({ body, commitComments: [gate4Comment(SHA)], gateComments: [gate4Comment('deadbee')], headSha: SHA });
  assert.equal(good.ok, true, good.errors.join('；'));
  const bad = await run({ body, commitComments: [gate4Comment('deadbee')], gateComments: [gate4Comment('deadbee')], headSha: SHA });
  assert.equal(bad.ok, false);
  assert.match(bad.errors.join(), /④ 本地编译门/);
});

test('#1770 S2 commit 评论不绕过 marker / sha 绑定：缺 gate-evidence 标记或 commit 行一律不认', async () => {
  const body = dropLines(['- ④ ']);
  const noMarker = ['- commit: ' + SHA, '- 结论（含产物）：KOTLIN_ALL_RESULT errors=0', ''].join('\n');
  const r1 = await run({ body, commitComments: [noMarker], headSha: SHA });
  assert.equal(r1.ok, false, '没有 gate-evidence 标记的 commit 评论被当成了证据');
  const noSha = ['<!-- gate-evidence:④ -->', '- 结论（含产物）：KOTLIN_ALL_RESULT errors=0', ''].join('\n');
  const r2 = await run({ body, commitComments: [noSha], headSha: SHA });
  assert.equal(r2.ok, false, '没有 commit 行的 commit 评论被当成了证据');
});

// #1770 S1：归档目录键从 PR 号换成**票号**（同为数字段，正则不变）——
// 「还没开 PR 就该能取证」是这一票的前提，键一改就废。
test('#1770 S1 归档目录键 = 票号：docs/verification/<模块>/<票号>/ 照认，非数字键仍不认', async () => {
  const withKey = (key) =>
    FULL_EVIDENCE.replaceAll(
      'docs/verification/exam/624/exam-home-after.jpg',
      `docs/verification/exam/${key}/exam-home-after.jpg`,
    );
  const ok = await run({ body: withKey('1770') });
  assert.equal(ok.ok, true, ok.errors.join('；'));
  const bad = await run({ body: withKey('feat-1770') });
  assert.equal(bad.ok, false, '非数字目录键被当成了可核验产物（票号必须是纯数字段）');
  assert.match(bad.errors.join(), /可核验/);
});

// ============================================================
// #1841：commit 评论端点对本仓恒 404（#1838）⇒ 门脚本把**同一份**评论改贴到票上，
// 票评论因此进候选。判据一个字不放宽：同 marker、同 `commit: <sha>`、同祖先判定、同 fail-closed；
// 差别只有「候选从哪儿来」，且读哪些票由 issueNumbersInBody 定（PR 正文显式关联，≤5 张）。
// ============================================================

test('#1841 票评论承载门证据：marker + `commit: <head>` ⇒ ④ 照常绑上并判绿', async () => {
  const r = await run({ body: dropLines(['- ④ ']), issueComments: [gate4Comment(SHA)], headSha: SHA });
  assert.equal(r.ok, true, r.errors.join('；'));
});

test('#1841 票评论与 commit 评论**同构**：同一份正文换个落点，结论与 ④ 门状态逐字相同', async () => {
  const body = dropLines(['- ④ ']);
  const asCommit = await run({ body, commitComments: [gate4Comment(SHA)], headSha: SHA });
  const asIssue = await run({ body, issueComments: [gate4Comment(SHA)], headSha: SHA });
  assert.equal(asCommit.ok, true, asCommit.errors.join('；'));
  assert.deepEqual(
    { ok: asIssue.ok, gate4: asIssue.gates['④'] },
    { ok: asCommit.ok, gate4: asCommit.gates['④'] },
    '同构：同一份正文落在 commit 评论与票评论上，判定必须一致（否则「降级」就变成了「换判据」）',
  );
  // 祖先口径也照走：票评论上的绑定同样要问 compare，不能因为「票上的」就跳过祖先判定
  const { fn, counts } = trueAncestor(DOC_FILES);
  const anc = await run({ body, issueComments: [gate4Comment(ANC)], headSha: HEAD, fetchCompare: fn });
  assert.equal(counts.calls, 1, '票评论的祖先绑定也必须真问一次 compare');
  assert.equal(anc.ok, true, anc.errors.join('；'));
});

test('#1841 票评论不绕过任何判据：缺 marker / 缺 commit 行 / 绑的不是祖先 / 其后运行时面变了 ⇒ 逐条判红', async () => {
  const body = dropLines(['- ④ ']);
  const noMarker = ['- commit: ' + SHA, '- 结论（含产物）：KOTLIN_ALL_RESULT errors=0', ''].join('\n');
  const r1 = await run({ body, issueComments: [noMarker], headSha: SHA });
  assert.equal(r1.ok, false, '没有 gate-evidence 标记的票评论被当成了证据');

  const noSha = ['<!-- gate-evidence:④ -->', '- 结论（含产物）：KOTLIN_ALL_RESULT errors=0', ''].join('\n');
  const r2 = await run({ body, issueComments: [noSha], headSha: SHA });
  assert.equal(r2.ok, false, '没有 commit 行的票评论被当成了证据');

  const diverged = compareStub({ status: 'diverged', mergeBaseSha: 'c'.repeat(40), files: [], filesTruncated: false });
  const r3 = await run({ body, issueComments: [gate4Comment(ANC)], headSha: HEAD, fetchCompare: diverged.fn });
  assert.equal(r3.ok, false, '绑在非祖先 sha 上的票评论被放行了');
  assert.match(r3.errors.join(), /不是当前 head 的祖先/);

  const moved = trueAncestor(UVUE_FILES);
  const r4 = await run({ body, issueComments: [gate4Comment(ANC)], headSha: HEAD, fetchCompare: moved.fn });
  assert.equal(r4.ok, false, '绑定之后运行时面已变的票评论被放行了（降级不许顺带放宽「免重跑」）');
});

test('#1841 issueNumbersInBody：认 Closes/Refs/Resolves 与 issue 直链，剥 HTML 注释、去重、≤5 张', () => {
  assert.deepEqual(issueNumbersInBody('Closes #1841'), [1841]);
  assert.deepEqual(issueNumbersInBody('refs: #12 与 Refs #13'), [12, 13]);
  assert.deepEqual(issueNumbersInBody('见 https://github.com/DriftingLi/FL/issues/99'), [99]);
  assert.deepEqual(issueNumbersInBody('closes #7\nhttps://github.com/o/r/issues/7'), [7], '同一张票只算一次');
  assert.deepEqual(
    issueNumbersInBody('<!-- Closes #883 -->\n真关联 closes #1841'),
    [1841],
    '模板注释里的历史票号不得进候选（模板注释塞满了 #1770/#883 …）',
  );
  const many = Array.from({ length: 9 }, (_, i) => `Closes #${100 + i}`).join('\n');
  assert.equal(issueNumbersInBody(many).length, 5, '候选上限 5 张：超出即截断，不是全收');
  assert.deepEqual(issueNumbersInBody(''), []);
  assert.deepEqual(issueNumbersInBody(null), []);
});

test('#1841 判据面第三来源是票评论，且三个消费者都真的把它喂进校验器（少一路就会出现假分歧）', () => {
  const src = readFileSync(validatorPath, 'utf8');
  const start = src.indexOf('export async function validatePrEvidence(');
  assert.ok(start >= 0, '判据模块里找不到导出的校验器');
  const sig = src.slice(start, src.indexOf(') {', start));
  assert.match(sig, /issueComments/, '票评论（降级落点）必须是候选来源之一');
  assert.match(src, /export function issueNumbersInBody\(/, '「哪些票算候选」必须由判据模块决定，不许写进 workflow');

  for (const wf of ['pr-evidence.yml', 'mobile-gates.yml']) {
    const w = readFileSync(path.resolve(SCRIPT_DIR, '..', 'workflows', wf), 'utf8');
    assert.match(w, /issueComments/, `${wf} 没有把票评论传给校验器 ⇒ 降级后的证据在真门上等于没贴`);
    assert.match(w, /issueNumbersInBody/, `${wf} 没有用同一个函数决定读哪些票 ⇒ 又与判据面分叉了`);
  }
  const pre = readFileSync(path.resolve(SCRIPT_DIR, 'preflight-pr.mjs'), 'utf8');
  assert.match(pre, /issueComments/, '本地预检没同步票评论 ⇒ 会出现「本地说没绑、CI 说绿」');
});

// #1841 兼容窗口：默认分支上的旧版判据模块还没有 issueNumbersInBody 导出。
// 两个 workflow 的 evalValidator 必须容忍这一点（typeof 对未声明标识符安全），
// 退化成「票评论来源未启用」而不是把整个 job 打红 —— PR #1843 首跑就栽在这里。
test('evalValidator 兼容旧版模块：没有 issueNumbersInBody 时不报错，票来源按未启用处理', () => {
  for (const wf of ['pr-evidence.yml', 'mobile-gates.yml']) {
    const src = readFileSync(path.resolve(SCRIPT_DIR, '..', 'workflows', wf), 'utf8');
    const tail = src.match(/return \{ validatePrEvidence[^`]*?\};/);
    assert.ok(tail, `${wf} 找不到 evalValidator 的返回表达式`);
    // 用「旧版模块」+ 该返回表达式求值：旧模块只定义 validatePrEvidence。
    const api = new Function(`function validatePrEvidence() { return 'legacy'; }\n${tail[0]}`)();
    assert.equal(typeof api.validatePrEvidence, 'function', `${wf} 旧模块下主判据仍必须可用`);
    assert.equal(api.validatePrEvidence(), 'legacy');
    assert.equal(api.issueNumbersInBody, null, `${wf} 旧模块下票来源必须退化为 null（fail-closed）`);
  }
});
