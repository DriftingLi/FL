// pr-evidence 校验逻辑的抽取式单测。
//
// 校验器内联在 .github/workflows/pr-evidence.yml 里（不 checkout 任何代码即可运行），
// 本测试把它从 workflow 中抽出来跑表驱动用例 —— 保证「守护的守护」不会静默失效。
// 运行：node --test .github/scripts/pr-evidence-check.test.mjs
// 已被 ci.yml 的 pr-evidence-selftest job 调用。

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

const run = (opts) => validate({ files: [uvue()], body: '', author: AUTHOR, ...opts });

// ④ 门（2026-09-11 修订：④c 整模块为默认，dev 专属面追加 ④a）——正文一行两载体
const GATE4_LINE =
  '- ④ 本地编译门（默认 ④c 整模块；dev 面追加 ④a） — 执行人：@DriftingLi · 日期：2026-09-11 · 复测对象：exam 分支整模块 Kotlin 编译 · 结论（含产物）：KOTLIN_ALL_RESULT errors=0 classes=1261，日志 .ci-verify/kotlin-all.log';

const FULL_EVIDENCE = `## 改了什么

把 exam 页拆了。

## 验收证据

- ① Android 真机逐页截图对比 — 执行人：@DriftingLi · 日期：2026-09-11 · 复测对象：exam 入口页与交卷流程 · 结论（含产物）：逐页截图 docs/verification/exam/exam-home-after.jpg
- ② 微信开发者工具无报错 — 执行人：@DriftingLi · 日期：2026-09-11 · 复测对象：未命中 MP-WEIXIN 面，仅入口页 · 结论（含产物）：入口页无报错 https://github.com/DriftingLi/FL/blob/master/docs/verification/exam/entry-after.jpg
- ③ \`npm run test:unit\` 全绿 — 结论（含产物）：https://github.com/DriftingLi/FL/actions/runs/123456789
${GATE4_LINE}
`;

test('非运行时面 PR：即使正文为空也通过（分级判据）', () => {
  const r = run({ files: [docs] });
  assert.equal(r.ok, true);
  assert.deepEqual(r.runtime, []);
  assert.match(r.notes.join(), /免人工门/);
});

test('运行时面 PR + 空正文：红，且提示缺段', () => {
  const r = run({});
  assert.equal(r.ok, false);
  assert.match(r.errors.join(), /验收证据/);
});

test('运行时面 PR + 完整四门证据：绿', () => {
  const r = run({ body: FULL_EVIDENCE });
  assert.equal(r.ok, true, r.errors.join('；'));
});

test('证据只写在 HTML 注释里不算（模板注释不得骗过校验）', () => {
  const r = run({ body: `<!--\n${FULL_EVIDENCE}\n-->` });
  assert.equal(r.ok, false);
  assert.match(r.errors.join(), /验收证据/);
});

test('占位符「待人工 / ⏳」= 缺证据：红', () => {
  const body = FULL_EVIDENCE.replace('@DriftingLi · 日期：2026-09-11 · 复测对象：exam 入口页与交卷流程 · 结论（含产物）：逐页截图 docs/verification/exam/exam-home-after.jpg', '⏳ · 日期：待人工 · 复测对象：待人工 · 结论（含产物）：待人工');
  const r = run({ body });
  assert.equal(r.ok, false);
  assert.match(r.errors.join(), /占位/);
});

test('执行人与 PR 作者同一账号：只提示、不拦（单账号仓库分不出人与 agent）', () => {
  const body = FULL_EVIDENCE.replace('@DriftingLi', `@${AUTHOR}`);
  const r = run({ body });
  assert.equal(r.ok, true, r.errors.join('；'));
  assert.match(r.notes.join(), /同一账号/);
});

test('日期格式不对：红', () => {
  const body = FULL_EVIDENCE.replace('日期：2026-09-11 · 复测对象：exam 分支整模块 Kotlin 编译', '日期：今天 · 复测对象：exam 分支整模块 Kotlin 编译');
  const r = run({ body });
  assert.equal(r.ok, false);
  assert.match(r.errors.join(), /YYYY-MM-DD/);
});

test('④ 结论没有引用编译日志：红（产物字段是硬判据）', () => {
  const body = FULL_EVIDENCE.replace('KOTLIN_ALL_RESULT errors=0 classes=1261，日志 .ci-verify/kotlin-all.log', '已编译通过');
  const r = run({ body });
  assert.equal(r.ok, false);
  assert.match(r.errors.join(), /kotlin-all\.log/);
});

test('③ 结论没贴 CI run 链接：红', () => {
  const body = FULL_EVIDENCE.replace('https://github.com/DriftingLi/FL/actions/runs/123456789', 'CI 已通过');
  const r = run({ body });
  assert.equal(r.ok, false);
  assert.match(r.errors.join(), /actions\/runs/);
});

test('新增 async function 的 patch → 不再触发 ④b（打包面判据取代代码形态判据）', () => {
  const files = [
    uvue('utils/exam-session.uts', { patch: '@@ -1,2 +1,3 @@\n+export async function loadSession() {\n+}\n' }),
  ];
  const r = run({ files, body: FULL_EVIDENCE });
  assert.equal(r.needs4b, false);
  assert.equal(r.ok, true, r.errors.join('；'));
});

test('新增 composable 文件 → 不再触发 ④b', () => {
  const files = [uvue('pages/exam/composables/use-exam-session.uts', { status: 'added' })];
  const r = run({ files, body: FULL_EVIDENCE });
  assert.equal(r.needs4b, false);
  assert.equal(r.ok, true, r.errors.join('；'));
});

test('声明「已接受未验证风险」：放行但打警告，并提示必须由人合并', () => {
  const body = `${FULL_EVIDENCE.split('## 验收证据')[0]}## 验收证据\n\n已接受未验证风险：本地无 Android 真机。事后验证计划：合并后由维护者补跑。\n`;
  const r = run({ body });
  assert.equal(r.ok, true, r.errors.join('；'));
  assert.match(r.notes.join(), /例外通道/);
  assert.match(r.notes.join(), /由人执行合并/);
});

test('例外通道不能替代证据段：缺段时仍红', () => {
  const r = run({ body: '已接受未验证风险：本地无真机。事后验证计划：由人补跑。\n' });
  assert.equal(r.ok, false);
  assert.match(r.errors.join(), /验收证据/);
});

test('training-app 下的 pages.json 也算运行时面（免得有人只改路由配置绕过）', () => {
  const r = run({ files: [uvue('pages.json', { status: 'modified' })], body: '' });
  assert.deepEqual(r.runtime.length, 1);
  assert.equal(r.ok, false);
});

test('未命中 MP-WEIXIN 面 → 第②门免（去掉②行也不红）', () => {
  const body = FULL_EVIDENCE.split('\n').filter((l) => !l.startsWith('- ② ')).join('\n');
  const r = run({ body });
  assert.equal(r.ok, true, r.errors.join('；'));
  assert.match(r.notes.join(), /第②门免/);
});

test('diff 出现 MP-WEIXIN 条件编译 → 第②门必填，缺②即红', () => {
  const files = [uvue('pages/login/login.uvue', { patch: '@@ -1,2 +1,3 @@\n+// #ifdef MP-WEIXIN\n+// #endif\n' })];
  const body = FULL_EVIDENCE.split('\n').filter((l) => !l.startsWith('- ② ')).join('\n');
  const r = run({ files, body });
  assert.equal(r.ok, false);
  assert.match(r.errors.join(), /②/);
});

test('manifest.json 改动 → 第②门必填', () => {
  const files = [uvue('manifest.json', { patch: '@@ -1,2 +1,3 @@\n+"mp-weixin": {}\n' })];
  const body = FULL_EVIDENCE.split('\n').filter((l) => !l.startsWith('- ② ')).join('\n');
  const r = run({ files, body });
  assert.equal(r.ok, false);
  assert.match(r.errors.join(), /②/);
});

test('frontend 下的同名 manifest.json 不算运行时面（不得误伤 Web 端）', () => {
  const r = run({ files: [{ filename: 'frontend/public/manifest.json', status: 'modified', patch: '' }] });
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

test('④b 打包面：pages.json 改动 → 触发 ④b，缺行即红', () => {
  const r = run({ files: [uvue('pages.json', { status: 'modified' })], body: FULL_EVIDENCE });
  assert.equal(r.needs4b, true);
  assert.equal(r.ok, false);
  assert.match(r.errors.join(), /④b/);
});

test('④b 打包面：新增 composables/x.uts（.uts 逻辑）→ 不触发 ④b', () => {
  const r = run({ files: [uvue('composables/x.uts', { status: 'added' })], body: FULL_EVIDENCE });
  assert.equal(r.needs4b, false);
  assert.equal(r.ok, true, r.errors.join('；'));
});

test('④b 打包面：.uvue 里新增 async function 的 patch → 仍不触发 ④b', () => {
  const files = [
    uvue('pages/exam/exam.uvue', { patch: '@@ -1,2 +1,3 @@\n+export async function submit() {\n+}\n' }),
  ];
  const r = run({ files, body: FULL_EVIDENCE });
  assert.equal(r.needs4b, false);
  assert.equal(r.ok, true, r.errors.join('；'));
});

test('④b 打包面：新增 *.aar / libs/*.jar / uni_modules 原生面改动 → 触发 ④b', () => {
  const cases = [
    [uvue(), uvue('libs/foo.aar', { status: 'added' })],
    [uvue(), uvue('libs/foo.jar', { status: 'added' })],
    [uvue('uni_modules/uni-scan/utssdk/app-android/index.uts', { status: 'modified' })],
  ];
  for (const files of cases) {
    const r = run({ files, body: FULL_EVIDENCE });
    assert.equal(r.needs4b, true, files.map((f) => f.filename).join(','));
    assert.equal(r.ok, false);
    assert.match(r.errors.join(), /④b/);
  }
});

test('P1：sha 匹配的 gate-evidence:④ 评论可替代正文 ④ 行（免手抄）', () => {
  // ① 正文整行不写
  const dropped = run({ body: dropLines(['- ④ ']), gateComments: [gate4Comment(SHA.slice(0, 7))], headSha: SHA });
  assert.equal(dropped.ok, true, dropped.errors.join('；'));
  assert.match(dropped.notes.join(), /评论/);
  assert.match(dropped.notes.join(), /sha 绑定 a20d2e2/);

  // ② 正文该行留空/写「见评论」也不算缺证据
  const blank = run({
    body: `${dropLines(['- ④ '])}- ④（见评论） — 执行人： · 日期： · 复测对象： · 结论（含产物）：\n`,
    gateComments: [gate4Comment(SHA)],
    headSha: SHA,
  });
  assert.equal(blank.ok, true, blank.errors.join('；'));
});

test('P1：sha 不匹配的评论不算证据 → 缺 ④ 行仍红', () => {
  const body = dropLines(['- ④ ']);
  const r = run({ body, gateComments: [gate4Comment('deadbee')], headSha: SHA });
  assert.equal(r.ok, false);
  assert.match(r.errors.join(), /④ 本地编译门/);
});

test('P1：没给 headSha 时评论不作为证据（fail-closed）', () => {
  const body = dropLines(['- ④ ']);
  const r = run({ body, gateComments: [gate4Comment(SHA)] });
  assert.equal(r.ok, false);
  assert.match(r.errors.join(), /④ 本地编译门/);
});

test('P1：正文 ④ 行照写（见评论）也不受评论路径影响', () => {
  const r = run({ body: FULL_EVIDENCE, gateComments: [gate4Comment(SHA)], headSha: SHA });
  assert.equal(r.ok, true, r.errors.join('；'));
  assert.match(r.notes.join(), /sha 绑定 a20d2e2/);
});

test('风险分层：仅 .uts 逻辑 + 测试改动 → 免 ①②，③④齐备即绿', () => {
  const files = [uvue('utils/foo.uts'), uvue('utils/foo.test.js')];
  const body = dropLines(['- ① ', '- ② ']);
  const r = run({ files, body });
  assert.equal(r.lowRiskRuntime, true);
  assert.equal(r.ok, true, r.errors.join('；'));
  assert.match(r.notes.join(), /低风险运行时面/);
});

test('风险分层：含 .uvue 的改动不算低风险 → 缺 ① 行仍红', () => {
  const files = [uvue('pages/x/x.uvue'), uvue('utils/foo.uts')];
  const body = dropLines(['- ① ']);
  const r = run({ files, body });
  assert.equal(r.lowRiskRuntime, false);
  assert.equal(r.ok, false);
  assert.match(r.errors.join(), /①/);
  assert.doesNotMatch(r.notes.join(), /低风险运行时面/);
});
