import { test } from 'node:test';
import assert from 'node:assert/strict';
import { parseArgs, toValidatorFiles, pickCiSummaryRun, fillCiLine } from './preflight-pr.mjs';
import { validatePrEvidence } from './lib/gate-validator.mjs';

// #1770 S4/S5/S6：本地预检与 CI 必须同判据、同结论。
//   - fillCiLine / toValidatorFiles / pickCiSummaryRun 是纯函数，单独钉；
//   - 最后一条是真正的「双跑一致」：同一份正文，CI 判红 → 预检补 ③ 链接 → CI 判绿。
//     （本地预检就是拿这份正文去跑同一个 validatePrEvidence，所以它绿 ⇔ CI 绿。）

const AUTHOR = 'zhengcookie';
const HEAD = 'a'.repeat(40);
const uvue = { filename: 'training-app/叉车维修培训学员端跨端应用/pages/exam/exam.uvue', status: 'modified', patch: '' };
const COMPARE = {
  status: 'ahead',
  merge_base_commit: { sha: 'b'.repeat(40) },
  files: [{ filename: uvue.filename, patch: '@@ -1 +1 @@' }, { filename: 'docs/agents/checks.md', patch: '' }],
};

const unboundBody = [
  '## 验收证据',
  '',
  '- ① 执行人：@zhengcookie · 日期：2026-10-22 · 复测对象：exam.uvue · 结论：docs/verification/mobile/1770/exam.jpg',
  '- ③ 结论：两腿全绿',
  '- ④ 执行人：@zhengcookie · 日期：2026-10-22 · 复测对象：整模块 · 结论：build.log 无 error',
  '',
].join('\n');

const CI_URL = 'https://github.com/DriftingLi/FL/actions/runs/123456789';

test('parseArgs：默认值 / --fill-body 规则 / 未知参数', () => {
  const def = parseArgs([]);
  assert.equal(def.base, 'origin/master');
  assert.equal(def.json, false);
  assert.equal(def.fillBody, false);
  assert.equal(parseArgs(['--fill-body', '--body-file', 'x.md']).fillBody, true);
  assert.throws(() => parseArgs(['--fill-body']), /--fill-body 需要 --body-file/);
  assert.throws(() => parseArgs(['--nope']), /未知参数/);
  assert.throws(() => parseArgs(['--pr', 'abc']), /--pr 只能是整数/);
});

test('toValidatorFiles：compare → 校验器 files，300 个到顶按截断处理', () => {
  const { files, truncated } = toValidatorFiles(COMPARE);
  assert.deepEqual(files.map((f) => f.filename), [uvue.filename, 'docs/agents/checks.md']);
  assert.equal(files[0].patch, '@@ -1 +1 @@');
  assert.equal(truncated, false);
  const many = toValidatorFiles({ files: Array.from({ length: 300 }, (_, i) => ({ filename: `f${i}`, patch: '' })) });
  assert.equal(many.truncated, true, '到顶不能假装看全了（校验器据此 fail-closed）');
});

test('pickCiSummaryRun：同 sha 优先、绿优先、取最新；没有就 null', () => {
  const runs = [
    { name: 'ci-summary', conclusion: 'failure', head_sha: HEAD, started_at: '2026-10-22T10:00:00Z', details_url: 'u-old' },
    { name: 'ci-summary', conclusion: 'success', head_sha: HEAD, started_at: '2026-10-22T11:00:00Z', details_url: 'u-new' },
    { name: 'backend-test', conclusion: 'success', head_sha: HEAD, started_at: '2026-10-22T12:00:00Z', details_url: 'u-other' },
  ];
  assert.deepEqual(pickCiSummaryRun(runs, HEAD), { conclusion: 'success', url: 'u-new', sha: HEAD, name: 'ci-summary' });
  assert.equal(pickCiSummaryRun([], HEAD), null);
  assert.equal(pickCiSummaryRun([{ name: 'ci', conclusion: 'success' }], HEAD), null, '别的 check 不能冒充 ③');
});

test('fillCiLine：只补链接，不覆盖结论；占位/已有链接一律不动', () => {
  const ok = fillCiLine(unboundBody, CI_URL);
  assert.equal(ok.filled, true);
  assert.match(ok.line, /③ 结论：两腿全绿（https:\/\/github\.com\/DriftingLi\/FL\/actions\/runs\/123456789）/);

  assert.equal(fillCiLine(ok.body, CI_URL).reason, 'already-has-run');
  assert.equal(fillCiLine('- ③ 结论：待补', CI_URL).reason, 'conclusion-placeholder');
  assert.equal(fillCiLine('- ③ 结论：', CI_URL).reason, 'conclusion-placeholder');
  assert.equal(fillCiLine('- ③ 结论：—', CI_URL).reason, 'conclusion-placeholder');
  assert.equal(fillCiLine('- ③ 两腿全绿', CI_URL).reason, 'no-conclusion-field');
  assert.equal(fillCiLine('- ① 结论：x', CI_URL).reason, 'no-gate-3-line');
  assert.equal(fillCiLine(unboundBody, 'https://github.com/x/y/pull/1').reason, 'url-invalid');
});

test('双跑一致：同一正文 CI 判红（③ 未绑）→ 预检补链接 → CI 判绿', async () => {
  const files = toValidatorFiles(COMPARE).files;
  const before = await validatePrEvidence({ files, body: unboundBody, author: AUTHOR, headSha: HEAD, fetchCompare: async () => null });
  assert.equal(before.ok, false);
  assert.ok(before.errors.some((e) => e.includes('③')), `应缺 ③：${before.errors.join(' | ')}`);

  const filled = fillCiLine(unboundBody, CI_URL);
  const after = await validatePrEvidence({ files, body: filled.body, author: AUTHOR, headSha: HEAD, fetchCompare: async () => null });
  assert.equal(after.ok, true, `补链接后应判绿：${after.errors.join(' | ')}`);
  assert.deepEqual(after.errors, []);
});

test('双跑一致：只在 commit 评论上的 ④ 证据，本地预检与 CI 都认（S2 同源）', async () => {
  const comment = [
    '<!-- gate-evidence:④ -->',
    'commit: ' + HEAD,
    '- ④ 执行人：@zhengcookie · 日期：2026-10-22 · 复测对象：整模块 · 结论：build.log 无 error',
  ].join('\n');
  const body = unboundBody
    .split('\n')
    .filter((l) => !l.startsWith('- ④'))
    .join('\n');
  const filled = fillCiLine(body, CI_URL).body;
  const opts = { files: toValidatorFiles(COMPARE).files, body: filled, author: AUTHOR, headSha: HEAD, fetchCompare: async () => null };

  const ciSide = await validatePrEvidence({ ...opts, gateComments: [], commitComments: [comment] });
  const localSide = await validatePrEvidence({ ...opts, gateComments: [], commitComments: [comment] });
  assert.deepEqual(ciSide.gates, localSide.gates);
  assert.equal(ciSide.ok, true, `commit 评论证据应判绿：${ciSide.errors.join(' | ')}`);
  assert.ok(ciSide.notes.some((n) => n.includes('④')), '应注明 ④ 证据来自评论');
});
