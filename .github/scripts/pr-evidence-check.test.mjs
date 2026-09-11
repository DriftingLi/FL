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

const FULL_EVIDENCE = `## 改了什么

把 exam 页拆了。

## 验收证据

- ① Android 真机逐页截图对比 — 执行人：@DriftingLi · 日期：2026-09-11 · 复测对象：exam 入口页与交卷流程 · 结论（含产物）：逐页截图 docs/verification/exam/exam-home-after.jpg
- ② 微信开发者工具无报错 — 执行人：@DriftingLi · 日期：2026-09-11 · 复测对象：未命中 MP-WEIXIN 面，仅入口页 · 结论（含产物）：入口页无报错 https://github.com/DriftingLi/FL/blob/master/docs/verification/exam/entry-after.jpg
- ③ \`npm run test:unit\` 全绿 — 结论（含产物）：https://github.com/DriftingLi/FL/actions/runs/123456789
- ④a dev 全量编译（\`npm run build:compile\`） — 执行人：@DriftingLi · 日期：2026-09-11 · 复测对象：exam 分支全量编译 · 结论（含产物）：ERROR 0 处，日志 .ci-verify/build.log
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
  const body = FULL_EVIDENCE.replace('日期：2026-09-11 · 复测对象：exam 分支全量编译', '日期：今天 · 复测对象：exam 分支全量编译');
  const r = run({ body });
  assert.equal(r.ok, false);
  assert.match(r.errors.join(), /YYYY-MM-DD/);
});

test('④a 结论没有引用 build.log：红（产物字段是硬判据）', () => {
  const body = FULL_EVIDENCE.replace('ERROR 0 处，日志 .ci-verify/build.log', '已编译通过');
  const r = run({ body });
  assert.equal(r.ok, false);
  assert.match(r.errors.join(), /build\.log/);
});

test('③ 结论没贴 CI run 链接：红', () => {
  const body = FULL_EVIDENCE.replace('https://github.com/DriftingLi/FL/actions/runs/123456789', 'CI 已通过');
  const r = run({ body });
  assert.equal(r.ok, false);
  assert.match(r.errors.join(), /actions\/runs/);
});

test('新增 async function → 触发 ④b，缺行即红', () => {
  const files = [
    uvue('utils/exam-session.uts', { patch: '@@ -1,2 +1,3 @@\n+export async function loadSession() {\n+}\n' }),
  ];
  const r = run({ files, body: FULL_EVIDENCE });
  assert.equal(r.needs4b, true);
  assert.equal(r.ok, false);
  assert.match(r.errors.join(), /④b/);
});

test('新增 composable 文件 → 触发 ④b，补齐后绿', () => {
  const files = [uvue('pages/exam/composables/use-exam-session.uts', { status: 'added' })];
  const missing = run({ files, body: FULL_EVIDENCE });
  assert.equal(missing.needs4b, true);
  assert.equal(missing.ok, false);

  const completed = run({
    files,
    body: `${FULL_EVIDENCE}- ④b release 云打包 — 执行人：@DriftingLi · 日期：2026-09-11 · 复测对象：收口 PR · 结论（含产物）：cli pack 日志见 https://github.com/DriftingLi/FL/actions/runs/987654321\n`,
  });
  assert.equal(completed.ok, true, completed.errors.join('；'));
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

test('frontend 下的同名 manifest.json 不算运行时面（不得误伤 Web 端）', () => {
  const r = run({ files: [{ filename: 'frontend/public/manifest.json', status: 'modified', patch: '' }] });
  assert.deepEqual(r.runtime, []);
  assert.equal(r.ok, true);
});
