import { test } from 'node:test';
import assert from 'node:assert/strict';
import { readFileSync } from 'node:fs';
import { parseArgs, toValidatorFiles, pickCiSummaryRun, makeFetchCompare } from './preflight-pr.mjs';
import { validatePrEvidence } from './lib/gate-validator.mjs';

// #1770 S4/S5/S6：本地预检与 CI 必须同判据、同结论。
// A1（2026-10-29，#2100）后本文件收缩：`--fill-body` / `fillCiLine`（补 ③ run 链接）与
// 「commit 评论承载证据」的用例随判据退役一起删除 —— ③ 已判纯税，评论也不再是证据载体。
// 保留：CLI 参数形态、compare → files 的转换、ci-summary 取数（**只作提示，不参与判据**）、
// 以及 #2044 那条「预检 CLI ↔ 校验器 ↔ 工作流」的 fetchCompare 形态契约钉。

const HEAD = 'a'.repeat(40);
const HEAD7 = HEAD.slice(0, 7);
const ANC40 = 'b'.repeat(40);
const ANC7 = ANC40.slice(0, 7);
const uvue = { filename: 'training-app/叉车维修培训学员端跨端应用/pages/exam/exam.uvue', status: 'modified', patch: '' };
const COMPARE = {
  status: 'ahead',
  merge_base_commit: { sha: ANC40 },
  files: [{ filename: uvue.filename, patch: '@@ -1 +1 @@' }, { filename: 'docs/agents/checks.md', patch: '' }],
};

const KOTLIN = 'KOTLIN_ALL_RESULT errors=0 classes=1261 files=88 freshness=fresh';
const bodyWith = (sha7) => `## 验收证据\n\n- ④ 跑于 \`${sha7}\` · \`${KOTLIN}\`\n`;

test('parseArgs：默认值 / --pr 整数校验 / 未知参数 / --fill-body 已退役', () => {
  const def = parseArgs([]);
  assert.equal(def.base, 'origin/master');
  assert.equal(def.json, false);
  assert.equal(def.pr, 0);
  assert.equal(parseArgs(['--pr', '12']).pr, 12);
  assert.deepEqual(parseArgs(['--body-file', 'x.md']).bodyFile, 'x.md');
  assert.throws(() => parseArgs(['--nope']), /未知参数/);
  assert.throws(() => parseArgs(['--pr', 'abc']), /--pr 只能是整数/);
  // ③ 判据退役后，正文不再需要 CI run 链接 ⇒ 自动补链接的开关一并删掉；写了要明确报未知参数，
  // 不能静默忽略（否则有人以为链接被补上了，其实一个字都没动）。
  assert.throws(() => parseArgs(['--fill-body', '--body-file', 'x.md']), /未知参数/);
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
  assert.equal(pickCiSummaryRun([{ name: 'ci', conclusion: 'success' }], HEAD), null, '别的 check 不能冒充 ci-summary');
});

test('双跑一致：同一份正文，本地预检与 CI 用同一个函数、得同一个结论（sha7 绑 head ⇒ 绿）', async () => {
  const files = toValidatorFiles(COMPARE).files;
  const ciSide = await validatePrEvidence({ files, body: bodyWith(HEAD7), headSha: HEAD, fetchCompare: async () => null });
  const localSide = await validatePrEvidence({ files, body: bodyWith(HEAD7), headSha: HEAD, fetchCompare: async () => null });
  assert.equal(ciSide.ok, true, ciSide.errors.join(' | '));
  assert.deepEqual(ciSide.gates, localSide.gates);
  assert.deepEqual(ciSide.errors, localSide.errors);
});

test('双跑一致：sha7 绑不上时两边都红（不存在「本地绿、CI 红」的分叉）', async () => {
  const files = toValidatorFiles(COMPARE).files;
  const ciSide = await validatePrEvidence({ files, body: bodyWith(ANC7), headSha: HEAD, fetchCompare: async () => null });
  const localSide = await validatePrEvidence({ files, body: bodyWith(ANC7), headSha: HEAD, fetchCompare: async () => null });
  assert.equal(ciSide.ok, false);
  assert.deepEqual(ciSide.errors, localSide.errors);
});

// ── #2044 契约钉：预检 CLI ↔ 校验器 ↔ CI 工作流的 fetchCompare 形态 ──────────────
// 事故链：CLI 写 (from, to) 位置参数、校验器按 fetchCompare({ base, head }) 调用
// ⇒ 对象撞进 from.replace 当场炸 ⇒ 被 catch 收敛成 null ⇒ 祖先绑定本地恒按 fail-closed 判红，
// 同一份正文 CI 却判绿。下面用例是该场景的最小复现：
// 真校验器 + 真 CLI 工厂，只把 compareOf 注桩（不打网络）。

test('#2044 祖先绑定·真 CLI 工厂 × 真校验器：对象契约下必须判绿', async () => {
  const calls = [];
  const fetchCompare = makeFetchCompare('DriftingLi/FL', {
    compareOfImpl: (repo, base, head) => {
      calls.push({ repo, base, head });
      return { status: 'ahead', mergeBaseSha: String(base), files: [{ filename: 'docs/agents/checks.md', patch: '' }], truncated: false };
    },
    warn: () => {},
  });
  const r = await validatePrEvidence({
    files: toValidatorFiles(COMPARE).files,
    body: bodyWith(ANC7),
    headSha: HEAD,
    fetchCompare,
  });
  assert.equal(r.ok, true, '祖先绑定 + 其间非运行时面应判绿：' + r.errors.join(' | '));
  assert.equal(calls.length, 1);
  assert.equal(calls[0].base, ANC7, 'base 应为正文里 `跑于` 那个 sha7');
  assert.equal(calls[0].head, HEAD, 'head 应为校验器传入的小写 sha');
});

test('#2044 工厂形态：剥 origin/ 前缀、按 base 缓存、抛异常收敛成 null（fail-closed 输入）', () => {
  const calls = [];
  const fetchCompare = makeFetchCompare('r/e', {
    compareOfImpl: (repo, base, head) => {
      calls.push(String(base));
      if (String(base).includes('boom')) throw new Error('HTTP 404: no such commit');
      return { status: 'ahead', mergeBaseSha: String(base), files: [{ filename: 'x.md', patch: '' }], truncated: false };
    },
    warn: () => {},
  });
  assert.deepEqual(
    fetchCompare({ base: 'origin/' + ANC40, head: HEAD }),
    { status: 'ahead', mergeBaseSha: ANC40, files: ['x.md'], filesTruncated: false },
  );
  fetchCompare({ base: 'origin/' + ANC40, head: HEAD });
  assert.ok(!calls.some((b) => b.includes('origin')), 'origin/ 前缀不得带进 compareOf');
  assert.equal(fetchCompare({ base: ANC40, head: HEAD }).status, 'ahead');
  fetchCompare({ base: ANC40, head: HEAD });
  assert.equal(calls.length, 2, '同 key 两次只应各取一次');
  assert.equal(fetchCompare({ base: 'boom', head: HEAD }), null, 'compare 抛异常 ⇒ 收敛成 null，校验器按 fail-closed 判不满足');
});

test('#2044 三方形态文本钉：校验器 / pr-evidence 工作流 / 预检 CLI 都写对象参数契约', () => {
  const read = (rel) => readFileSync(new URL(rel, import.meta.url), 'utf8');
  const validator = read('./lib/gate-validator.mjs');
  const prEvidence = read('../workflows/pr-evidence.yml');
  const cli = read('./preflight-pr.mjs');
  assert.match(validator, /fetchCompare\(\{ base, head:/, '校验器调用形态是契约真源');
  assert.match(prEvidence, /const fetchCompare = async \(\{ base, head \}\) =>/, 'pr-evidence.yml 注入是对象参数');
  assert.match(cli, /return \(\{ base, head \}.*\) => \{/, '预检 CLI 工厂必须收对象参数（与上面两方同构，防再各写各的）');
  // 管家（mobile-gates.yml）已随 A1 整份删除：它曾经是第三个消费者，删了就不该再有第四份实现。
  assert.throws(() => read('../workflows/mobile-gates.yml'), /ENOENT/);
});
