// pr-evidence 校验逻辑的抽取式单测。
//
// 校验器内联在 .github/workflows/pr-evidence.yml 里（不 checkout 任何代码即可运行），
// 本测试把它从 workflow 中抽出来跑表驱动用例 —— 保证「守护的守护」不会静默失效。
// 运行：node --test .github/scripts/pr-evidence-check.test.mjs
// 已被 ci.yml 的 pr-evidence-selftest job 调用。
//
// 2026-09-12 修订（本文件与校验器同步改）：门证据的绑定判据从「sha 必须等于 head」放宽为
// 「**祖先 + 运行时面未变**」（动机：#624 反复被 master 前进作废）。判定要看 compare 结果，
// 故校验器改为 async 且把「取 compare 结果」抽成**注入回调 fetchCompare** ——
// 本文件里一律注桩（`cmp()` / `compareStub()`），**不发任何真实网络请求**。

import { test } from 'node:test';
import assert from 'node:assert/strict';
import { readFileSync } from 'node:fs';
import { fileURLToPath } from 'node:url';
import path from 'node:path';

const workflowPath = path.resolve(
  path.dirname(fileURLToPath(import.meta.url)),
  '..',
  'workflows',
  'pr-evidence.yml',
);

function loadValidator() {
  const src = readFileSync(workflowPath, 'utf8');
  const m = src.match(
    /\/\/ ==== PR-EVIDENCE-VALIDATOR-START ====([\s\S]*?)\/\/ ==== PR-EVIDENCE-VALIDATOR-END ====/,
  );
  assert.ok(m, '在 pr-evidence.yml 里找不到 PR-EVIDENCE-VALIDATOR 标记块');
  return new Function(`${m[1]}\nreturn validatePrEvidence;`)();
}

const validate = loadValidator();
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

test('风险分层：仅 .uts 逻辑 + 测试改动 → 免 ①②，③④齐备即绿', async () => {
  const files = [uvue('utils/foo.uts'), uvue('utils/foo.test.js')];
  const body = dropLines(['- ① ', '- ② ']);
  const r = await run({ files, body });
  assert.equal(r.lowRiskRuntime, true);
  assert.equal(r.ok, true, r.errors.join('；'));
  assert.match(r.notes.join(), /低风险运行时面/);
});

test('风险分层：含 .uvue 的改动不算低风险 → 缺 ① 行仍红', async () => {
  const files = [uvue('pages/x/x.uvue'), uvue('utils/foo.uts')];
  const body = dropLines(['- ① ']);
  const r = await run({ files, body });
  assert.equal(r.lowRiskRuntime, false);
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
  // 低风险运行时面（仅 .uts 逻辑）⇒ ①② 免；给一条「其间改了运行时面」的 ② 评论
  const files = [uvue('utils/foo.uts')];
  const body = dropLines(['- ① ', '- ② ']);
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
