/**
 * 页面私有子件层契约（#2866，判据 docs/design/20261125-页面私有子件层判据.md §8/§10/§11）。
 *
 * 层定义 = 路径（pages/<族>/components/**）+ import 图；判据三条：
 * ① 裸 hex ∈ token 值 ∪ 白名单（3/6/8 位钉死，4 位票号不算）；
 * ② 有 style 块则必须 lang="scss"；
 * ③ 逐文件棘轮 utils/privateComponentUiBaseline.json（两向判红，新文件缺省 0，基线须扫描产出）；
 * ④ 同族封闭：私有件的导入方必须在同族、且在 pages/ 下。
 *
 * 本套件是接线腿（只读源码文本，不起子进程）；扫描逻辑在 utils/privateComponentUiScan.js，
 * 基线由 scripts/gen-private-component-ui-baseline.mjs 生成，禁止手抄。
 */
const fs = require('fs');
const os = require('os');
const path = require('path');
const scan = require('./privateComponentUiScan');

const APP_ROOT = path.join(__dirname, '..');
const BASELINE = require('./privateComponentUiBaseline.json');

function writeFixture(root, files) {
  for (const [rel, content] of Object.entries(files)) {
    const abs = path.join(root, rel);
    fs.mkdirSync(path.dirname(abs), { recursive: true });
    fs.writeFileSync(abs, content);
  }
}

function makeFixture(files) {
  const root = fs.mkdtempSync(path.join(os.tmpdir(), 'private-ui-'));
  writeFixture(root, files);
  return root;
}

const FIXTURE = makeFixture({
  'uni.scss': '$brand: #112233;\n$legacy: #abc;\n$alias: $brand;\n',
  'utils/uiHexWhitelist.js': "module.exports = { '#445566': 'fixture' };\n",
  'pages/alpha/alpha.uvue': "<template><view /></template>\n<script>\nimport Clean from './components/clean.uvue'\n</script>\n",
  'pages/alpha/alias.uvue': "<template><view /></template>\n<script>\nimport Clean from '@/pages/alpha/components/clean.uvue'\n</script>\n",
  'pages/beta/beta.uvue': "<template><view /></template>\n<script>\nimport Clean from '../alpha/components/clean.uvue'\n</script>\n",
  'components/shared.uvue': "<template><view /></template>\n<script>\nimport Clean from '../pages/alpha/components/clean.uvue'\n</script>\n",
  'pages/alpha/components/clean.uvue':
    '<template><view /></template>\n<style lang="scss">\n/* #ff0000 注释不算 */\n// #ff0000 整行注释不算\n.x { color: #112233; background: #ABC; border-color: #445566; }\n</style>\n',
  'pages/alpha/components/dirty.uvue':
    '<template><view /></template>\n<style lang="scss">\n.x { color: #FF0000; border-color: #ff0000; }\n/* 票号与位数陷阱 */\n.y { content: "#2520"; content: "#12345"; }\n</style>\n',
  'pages/beta/components/empty.uvue': '<template><view /></template>\n',
  'pages/beta/components/no-lang.uvue': '<template><view /></template>\n<style>\n.x { color: #112233; }\n</style>\n',
  'pages/beta/components/css-lang.uvue': '<template><view /></template>\n<style lang="css">\n.x { color: #112233; }\n</style>\n'
});

describe('页面私有子件层契约 · 反重言自检（合成树）', () => {
  test('hex 正则钉 3/6/8 位，4/5/7 位与注释不计', () => {
    expect(scan.hexLiterals('#abc #ABCDEF #11223344')).toEqual(['abc', 'ABCDEF', '11223344']);
    expect(scan.hexLiterals('#2520 #12345 #1234567')).toEqual([]);
    expect(scan.hexLiterals('#abcdef12')).toEqual(['abcdef12']);
    expect(scan.hexLiterals('/* #ff0000 */\n// #ff0000\nx')).toEqual(['ff0000', 'ff0000']);
    expect(scan.hexLiterals(scan.stripStyleComments('/* #ff0000 */\n// #ff0000\nx'))).toEqual([]);
  });

  test('字面值三分类：token / 白名单 / 违规（大小写与 3 位展开归一）', () => {
    const tokens = new Set([scan.normalizeHex('#112233'), scan.normalizeHex('#abc')]);
    const whitelist = new Set([scan.normalizeHex('#445566')]);
    const got = scan.classifyHex(['#112233', '#ABC', '#445566', '#FF0000'], tokens, whitelist);
    expect(got).toEqual({ token: 2, whitelist: 1, violations: ['#FF0000'] });
  });

  test('lang 规则：无 style 块免检，有则必须 scss', () => {
    const blocksOf = (src) => scan.extractStyleBlocks(src).map((block) => block.attrs);
    expect(scan.hasScssLang(blocksOf('<template><view /></template>')[0] || '')).toBe(false);
    expect(scan.extractStyleBlocks('<template><view /></template>')).toHaveLength(0);
    expect(scan.extractStyleBlocks('<style lang="scss">.x{}</style>').map((b) => scan.hasScssLang(b.attrs))).toEqual([true]);
    expect(scan.extractStyleBlocks('<style>.x{}</style>').map((b) => scan.hasScssLang(b.attrs))).toEqual([false]);
    expect(scan.extractStyleBlocks('<style lang="css">.x{}</style>').map((b) => scan.hasScssLang(b.attrs))).toEqual([false]);
  });

  test('真实树同口径：合成树扫描读数与基线、两向棘轮', () => {
    const result = scan.scanPrivateComponents(FIXTURE);
    const byRel = Object.fromEntries(result.files.map((file) => [file.rel, file]));
    expect(result.totals).toMatchObject({ files: 5, hexCount: 7, tokenCount: 4, whitelistCount: 1, violationCount: 2, langMissing: 2 });
    expect(byRel['pages/alpha/components/clean.uvue'].violations).toEqual([]);
    expect(byRel['pages/alpha/components/dirty.uvue'].violations).toEqual(['#FF0000', '#FF0000']);
    expect(byRel['pages/beta/components/empty.uvue'].langMissing).toBe(0);

    const baseline = scan.ratchetBaseline(result);
    expect(baseline).toEqual({ 'pages/alpha/components/dirty.uvue': 2 });
    expect(scan.ratchetFindings(baseline, baseline)).toEqual([]);
    expect(scan.ratchetFindings({ ...baseline, 'pages/alpha/components/dirty.uvue': 3 }, baseline)).toHaveLength(1);
    expect(scan.ratchetFindings({ 'pages/alpha/components/dirty.uvue': 1 }, baseline)).toHaveLength(1);
    expect(scan.ratchetFindings({ 'pages/alpha/components/dirty.uvue': 2, 'pages/beta/components/new.uvue': 1 }, baseline)).toHaveLength(1);
  });

  test('同族封闭：同族/别名放行，跨族与 pages 外导入判红', () => {
    const privateRels = scan.collectUvue(FIXTURE, path.join(FIXTURE, 'pages')).filter((rel) => scan.PRIVATE_REL_RE.test(rel));
    const { findings, edges } = scan.familyClosureFindings(FIXTURE, privateRels);
    expect(privateRels).toHaveLength(5);
    expect(edges).toBe(4);
    expect(findings).toHaveLength(2);
    expect(findings.join('\n')).toContain('pages/alpha/components/clean.uvue 被 跨族（beta → alpha） 导入');
    expect(findings.join('\n')).toContain('来源不在 pages/ 下（components/shared.uvue）');
  });

  test('白名单死条自检能抓到未出现值', () => {
    const dead = scan.whitelistDeadEntries(FIXTURE, new Set([scan.normalizeHex('#445566'), scan.normalizeHex('#DEADBE')]));
    expect(dead).toEqual(['#DEADBE']);
  });
});

describe('页面私有子件层契约 · 真实树（#2866 落地实测）', () => {
  const result = scan.scanPrivateComponents(APP_ROOT);
  const byRel = Object.fromEntries(result.files.map((file) => [file.rel, file]));
  const closure = scan.familyClosureFindings(APP_ROOT, result.privateRels);

  test('扫描面活性：60 件私有子件、60 件全被导入、10 个族', () => {
    expect(result.totals.files).toBeGreaterThanOrEqual(60);
    expect(new Set(result.files.map((file) => file.family)).size).toBeGreaterThanOrEqual(10);
    expect(closure.edges).toBeGreaterThanOrEqual(60);
    for (const rel of [
      'pages/courses/components/course-cover-section.uvue',
      'pages/forum/components/forum-topic-card.uvue',
      'pages/forum/components/forum-markdown-blocks.uvue',
      'pages/exam/components/exam-result-summary.uvue'
    ]) {
      expect(byRel[rel]).toBeDefined();
    }
  });

  test('有 style 块则 100% lang="scss"（本票同批修 2 份）', () => {
    expect(result.totals.langMissing).toBe(0);
    expect(byRel['pages/exam/components/exam-result-summary.uvue'].styleBlocks).toBeGreaterThan(0);
    expect(byRel['pages/forum/components/forum-topic-body.uvue'].styleBlocks).toBeGreaterThan(0);
    expect(byRel['pages/forum/components/forum-markdown-blocks.uvue'].styleBlocks).toBe(0);
  });

  test('裸 hex 三分类读数：token + 白名单 + 存量违规（与 T4 账本一致）', () => {
    expect(result.totals.hexCount).toBeGreaterThan(0);
    expect(result.totals.tokenCount + result.totals.whitelistCount + result.totals.violationCount).toBe(result.totals.hexCount);
    // #2863 删 11 条业务死件后 50 → 39；保留下界 30 仍能挡住「uni.scss 解析失效读到 0/少量值」这类假绿。
    expect(result.tokenValues.size).toBeGreaterThanOrEqual(30);
  });

  test('基线数据卫生：排序、整数 ≥1、路径合法、键 ⊆ 扫描面（禁手抄漂移）', () => {
    const keys = Object.keys(BASELINE);
    expect(keys).toEqual([...keys].sort());
    expect(keys.length).toBeGreaterThan(0);
    for (const [rel, count] of Object.entries(BASELINE)) {
      expect(rel).toMatch(scan.PRIVATE_REL_RE);
      expect(Number.isInteger(count) && count >= 1).toBe(true);
      expect(byRel[rel]).toBeDefined();
    }
  });

  test('两向棘轮：实测逐文件等于基线（只减不增，新文件缺省 0）', () => {
    const actual = Object.fromEntries(result.files.filter((file) => file.violations.length > 0).map((file) => [file.rel, file.violations.length]));
    expect(scan.ratchetFindings(actual, BASELINE)).toEqual([]);
  });

  test('同族封闭：0 跨族 / 0 跨层导入', () => {
    expect(closure.findings).toEqual([]);
  });

  test('白名单值全层无死条（在册值都在 style 块真实出现）', () => {
    expect(result.whitelistValues.size).toBeGreaterThanOrEqual(1);
    expect(scan.whitelistDeadEntries(APP_ROOT, result.whitelistValues)).toEqual([]);
  });
});
