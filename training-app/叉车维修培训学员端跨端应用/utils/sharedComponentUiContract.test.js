/**
 * 共享件层值面契约（#2865，T3 判据落地）—— components/** 24 件自身合规 + 逐文件棘轮
 *
 * 判据源：docs/design/20261125-共享件层判据.md 第 6 / 10 节（X.1-X.7）。
 * 层 = components 目录下所有 .uvue（24 件），是资产库，不是「全站标准件」。
 * 件数随 #2912 清零引用件（app-card / app-badge / app-list-item）从 27 收到 24。
 *
 * 三条值面判据（只判自身合规，不判引用率）：
 *  1. style 块内声明位裸 hex 属于 uni.scss token 值集合或 utils/uiHexWhitelist.js；
 *  2. 有 style 块则每个块都必须是 lang="scss"；
 *  3. 主色字面 #239EDD = 0（必须写 $primary-color）。
 *
 * 棘轮（两向都红，缺一条就会长成假锁）：
 *  - 实测 > 基线（含基线没有的新件）=> 新写入判红；
 *  - 基线 > 实测（含文件被删）=> 清了没改账判红。
 *  基线 utils/sharedComponentUiBaseline.json 由扫描产出、键排序；存量 9 件 26 处只减不增，
 *  新共享件不在基线 => 按 0 算 => 任何一处值面违规即红（X.7）。
 *
 * 为什么引用率不判：见判据第 8 / 9 节——「该不该用某件共享件」是判断类，机检只能回答
 * 值在不在白名单；唯一例外 app-nav-bar 的 N06 五页豁免由 batch6 页面级套件守，不在本套件重复。
 *
 * 归腿：接线守护——只读源码文本与基线，不起子进程、不 loadUts、无动态 import()；
 * scripts/lib/guard-split.js 会自动把它落进 test:unit 腿。
 */
const h = require('./contractHarness');
const read = h.read;
const WHITELIST = require('./uiHexWhitelist');
const BASELINE = require('./sharedComponentUiBaseline.json');

const LAYER_DIR = 'components';
const BASELINE_PATH = 'utils/sharedComponentUiBaseline.json';
const COMPONENTS = h.filesUnder(LAYER_DIR, /\.uvue$/, true);
const UNI_SCSS = read('uni.scss');

/** 3 位归一 6 位、8 位取前 6 位（alpha 不参与值面比较），统一大写。 */
function normalizeHex(raw) {
  const body = raw.slice(1);
  const six = body.length === 3
    ? body[0] + body[0] + body[1] + body[1] + body[2] + body[2]
    : body.slice(0, 6);
  return '#' + six.toUpperCase();
}

/** token 值集合（uni.scss 的 $name: #hex; 声明，归一大写 6 位）。 */
const TOKEN_VALUES = new Set(
  [...UNI_SCSS.matchAll(/^\s*\$[\w-]+\s*:\s*(#[0-9a-fA-F]{3,8})\s*(?:!default)?\s*;/gm)]
    .map((m) => normalizeHex(m[1])),
);

/** 白名单值集合（utils/uiHexWhitelist.js 的键，归一大写 6 位）。 */
const WHITELIST_VALUES = new Set(Object.keys(WHITELIST).map(normalizeHex));

/** 只认 3 / 6 / 8 位，且后一位不得还是 hex（4/5/7 位与更长串不匹配）。 */
const HEX_RE = /#(?:[0-9a-fA-F]{8}|[0-9a-fA-F]{6}|[0-9a-fA-F]{3})(?![0-9a-fA-F])/g;

/** 剥掉会「提到」style 标签的注释：块注释、HTML 注释、行注释（行注释只认行首或空白后的 //）。 */
function stripCommentsForTags(src) {
  return src
    .replace(/\/\*[\s\S]*?\*\//g, '')
    .replace(/<!--[\s\S]*?-->/g, '')
    .replace(/(^|[ \t])\/\/[^\n]*/gm, '$1');
}

/** 所有 style 块的内容（先剥注释，避免注释里的 <style> 被当成真块）。 */
function styleBlocks(src) {
  const noComments = stripCommentsForTags(src);
  return [...noComments.matchAll(/<style\b[^>]*>([\s\S]*?)<\/style>/g)].map((m) => m[1]);
}

/** style 块内声明位裸 hex 违规（不属于 token 值也不属于白名单）。 */
function hexViolations(src) {
  const out = [];
  for (const block of styleBlocks(src)) {
    const style = block.replace(/^\s*\/\/.*$/gm, '');
    for (const m of style.matchAll(HEX_RE)) {
      const value = normalizeHex(m[0]);
      if (!TOKEN_VALUES.has(value) && !WHITELIST_VALUES.has(value)) out.push(m[0]);
    }
  }
  return out;
}

/** 纯函数：{ 相对路径: 源码 } => { 相对路径: 违规处数 }（只收录 >=1 的文件，键排序）。 */
function scanCountsFromSources(sources) {
  const counts = {};
  for (const rel of Object.keys(sources).sort()) {
    const n = hexViolations(sources[rel]).length;
    if (n > 0) counts[rel] = n;
  }
  return counts;
}

/** 现测：components/** 24 件的逐文件违规处数。 */
function scanCounts() {
  const sources = {};
  for (const rel of COMPONENTS) sources[rel] = read(rel);
  return scanCountsFromSources(sources);
}

/** 两向棘轮：increases = 实测高于基线；decreases = 基线高于实测。 */
function ratchetFindings(counts, baseline) {
  const increases = [];
  const decreases = [];
  const keys = [...new Set([...Object.keys(counts), ...Object.keys(baseline)])].sort();
  for (const k of keys) {
    const actual = counts[k] || 0;
    const base = baseline[k] || 0;
    if (actual > base) {
      increases.push(
        k + ' —— 现测 ' + actual + ' 处，基线 ' + base + ' 处：共享件值面只减不增。'
        + '新写入请改用 token（$primary-color 等）或先登记白名单；确属存量清退的，'
        + '在本 PR 内同批清掉并下调基线',
      );
    } else if (base > actual) {
      decreases.push(
        k + ' —— 基线 ' + base + ' 处，现测 ' + actual + ' 处：基线高于实测 = 清了没改账。'
        + '把 ' + BASELINE_PATH + ' 里该条目下调到实测数（清到 0 就删条目），禁止让基线虚挂着',
      );
    }
  }
  return { increases, decreases };
}

/** style 标签问题：缺 lang="scss" 或开闭标签数不等（fail-closed，不静默跳过）。 */
function styleTagProblems(src) {
  const noComments = stripCommentsForTags(src);
  const opens = noComments.match(/<style\b[^>]*>/g) || [];
  const closes = noComments.match(/<\/style>/g) || [];
  const problems = [];
  if (opens.length !== closes.length) {
    problems.push('style 开闭标签数不等（' + opens.length + '/' + closes.length + '）');
  }
  for (const tag of opens) {
    if (!/\blang="scss"/.test(tag)) problems.push('缺 lang="scss"：' + tag);
  }
  return problems;
}

/** 主色字面 #239EDD 的出现次数（大小写照算；注释不算）。 */
function primaryLiterals(src) {
  let n = 0;
  for (const block of styleBlocks(src)) {
    n += (block.match(/#239EDD/gi) || []).length;
  }
  return n;
}

describe('共享件层值面契约（#2865，components/** 24 件）', () => {
  describe('① 扫描面完整性', () => {
    it('层 = components 目录下所有 .uvue，24 件（增删件须同步本套件与基线，不得静默漂移）', () => {
      expect(COMPONENTS.length).toBe(24);
    });

    it('具名钉住：页头件 / 违规存量件都在扫描面内', () => {
      const pinned = [
        'components/app-nav-bar/app-nav-bar.uvue',
        'components/ai-chat/ai-chat-input.uvue',
        'components/forum-wysiwyg-input.uvue',
      ];
      for (const rel of pinned) {
        expect([rel, COMPONENTS.includes(rel)]).toEqual([rel, true]);
      }
    });
  });

  describe('② 检测器自检：合成样本上的计数、注释剥离与棘轮两向', () => {
    const sample = [
      '<template><view class="x" /></template>',
      '<style lang="scss">',
      '.a { color: #123456; }',
      '.b { color: #123; }',
      '.c { color: #12345678; }',
      '.d { color: #fff; }',
      '.e { color: #E7F4F9; }',
      '.f { color: #239EDD; }',
      '.g { color: #12345; }',
      '.h { color: #1234567; }',
      '.i { color: #123456789; }',
      '.j { color: #123456a; }',
      '/* .k { color: #ABCDEF; } */',
      '// .l { color: #ABCDEF; }',
      '</style>',
    ].join('\n');

    it('精确计数：3/6/8 位、token/白名单放行、注释不计数、非法位数不匹配', () => {
      expect(hexViolations(sample)).toEqual(['#123456', '#123', '#12345678']);
      expect(scanCountsFromSources({ 'components/x.uvue': sample })).toEqual({
        'components/x.uvue': 3,
      });
    });

    it('新件缺省 0：基线没有的文件出现一处违规即 increases 判红', () => {
      const counts = scanCountsFromSources({ 'components/new/new.uvue': sample });
      const { increases, decreases } = ratchetFindings(counts, {});
      expect(increases).toHaveLength(1);
      expect(increases[0]).toContain('components/new/new.uvue');
      expect(increases[0]).toContain('现测 3 处');
      expect(increases[0]).toContain('只减不增');
      expect(decreases).toEqual([]);
    });

    it('实测与基线逐文件一致 => 两向都绿', () => {
      const counts = scanCountsFromSources({ 'components/x.uvue': sample });
      expect(ratchetFindings(counts, counts)).toEqual({ increases: [], decreases: [] });
    });

    it('清了没改账 => decreases 判红；文件被删同判', () => {
      const counts = scanCountsFromSources({ 'components/x.uvue': sample });
      const cleaned = { 'components/x.uvue': 1 };
      const partial = ratchetFindings(cleaned, counts);
      expect(partial.increases).toEqual([]);
      expect(partial.decreases).toHaveLength(1);
      expect(partial.decreases[0]).toContain('基线高于实测');
      expect(partial.decreases[0]).toContain(BASELINE_PATH);

      const deleted = ratchetFindings({}, counts);
      expect(deleted.decreases).toHaveLength(1);
      expect(deleted.decreases[0]).toContain('components/x.uvue');
    });

    it('lang="scss" 检测：缺 lang / 开闭不等都判红，注释里的 <style> 不算', () => {
      expect(styleTagProblems('<style lang="scss">.a { color: #fff; }</style>')).toEqual([]);
      expect(styleTagProblems('<style lang="scss" scoped>.a { color: #fff; }</style>')).toEqual([]);
      expect(styleTagProblems('<template><view /></template>')).toEqual([]);
      expect(styleTagProblems('<style>.a { color: #fff; }</style>')).toEqual([
        '缺 lang="scss"：<style>',
      ]);
      expect(styleTagProblems('<style lang="scss">.a { color: #fff; }')).toEqual([
        'style 开闭标签数不等（1/0）',
      ]);
      expect(styleTagProblems('/* <style scoped> */<style lang="scss">.a { color: #fff; }</style>')).toEqual([]);
    });

    it('主色字面检测：大小写照算，token 引用与注释不算', () => {
      expect(primaryLiterals('<style lang="scss">.a { color: #239EDD; }</style>')).toBe(1);
      expect(primaryLiterals('<style lang="scss">.a { color: #239edd; }</style>')).toBe(1);
      expect(primaryLiterals('<style lang="scss">.a { color: $primary-color; }</style>')).toBe(0);
      expect(primaryLiterals('<style lang="scss">/* #239EDD */ .a { color: $primary-color; }</style>')).toBe(0);
    });
  });

  describe('③ 基线数据卫生', () => {
    it('键是排序的 POSIX 相对路径、值都是 >=1 的整数', () => {
      const keys = Object.keys(BASELINE);
      expect(keys).toEqual([...keys].sort());
      for (const k of keys) {
        expect([k, Number.isInteger(BASELINE[k]) && BASELINE[k] >= 1]).toEqual([k, true]);
        expect([k, /^components\/[\w/.-]+\.uvue$/.test(k) && !k.includes('\\')]).toEqual([k, true]);
      }
    });

    it('基线键都指向现存的共享件（无死键）', () => {
      const dead = Object.keys(BASELINE).filter((k) => !COMPONENTS.includes(k));
      expect(dead).toEqual([]);
    });
  });

  describe('④ 真实文件（components/** 24 件）', () => {
    it('lang="scss" 100%（有 style 块则每个块都要 lang="scss"）', () => {
      const bad = COMPONENTS
        .map((rel) => [rel, styleTagProblems(read(rel))])
        .filter(([, problems]) => problems.length > 0)
        .map(([rel, problems]) => rel + ': ' + problems.join('；'));
      expect(bad).toEqual([]);
    });

    it('主色字面 #239EDD = 0（必须写 $primary-color）', () => {
      const bad = COMPONENTS
        .map((rel) => [rel, primaryLiterals(read(rel))])
        .filter(([, n]) => n > 0)
        .map(([rel, n]) => rel + ': ' + n + ' 处');
      expect(bad).toEqual([]);
    });

    it('新共享件缺省 0：不在基线的件现测违规 = 0', () => {
      const bad = COMPONENTS
        .filter((rel) => !(rel in BASELINE))
        .map((rel) => [rel, hexViolations(read(rel)).length])
        .filter(([, n]) => n > 0)
        .map(([rel, n]) => rel + ': ' + n + ' 处');
      expect(bad).toEqual([]);
    });

    it('棘轮主判据：实测与基线逐文件相等（两向任何一侧非空即红）', () => {
      const { increases, decreases } = ratchetFindings(scanCounts(), BASELINE);
      expect(decreases.join('\n')).toBe('');
      expect(increases.join('\n')).toBe('');
    });

    it('基线由扫描产出：键集合与逐文件计数等于现测', () => {
      expect(BASELINE).toEqual(scanCounts());
    });
  });
});
