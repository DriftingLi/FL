/**
 * 条件编译白名单守护（#2331）—— `#ifdef` / `#ifndef` / `#endif` **只减不增**
 *
 * 为什么要有它：条件编译是本仓**唯一**的平台分叉入口，却长期零机检——同族的 CSS 硬限制
 * 早有 `docs/agents/uvue-css.md` + `utils/uvueFontCarrierContract.test.js`，而「哪个文件多了
 * 一处平台分叉」只能靠人眼。本套件把这条失效面补上：**新增分叉在 ③ 门就地变红**。
 *
 * ## 口径（判据本体）
 * - 指令行 = 行首可有空白与注释前缀（`//`、`<!--`、块注释行首 `*`）的
 *   `#ifdef` / `#ifndef` / `#endif`。前缀**可不带**（裸指令在 UTS 里是语法错误，但守护
 *   不负责替编译器判语法，只负责数「分叉点」）。
 * - 扫描面 = 工程内全部 `.uvue` / `.uts`。**排除 `uni_modules/`**（vendor 面，随上游升级变动，
 *   vendor 里有没有分叉不是本仓的策略边界）；`.ci-verify/` `.scratch/` `unpackage/` `node_modules/`
 *   等由 `contractHarness.filesUnder` 的 `SKIP_DIRS` 统一排除。
 * - 棘轮：每文件**允许减少**，**不允许增加**（含「基线外新文件出现指令行」——那等于从 0 增到 N）。
 *
 * ## 基线（2026-11-06 现测，9 文件 / 66 行，与 #2331 票面逐行一致）
 * 基线是**策略**不是**现值**：降到 0 的文件也留在表里（留在表里才挡得住它再长回来）。
 * 只减不增 ⇒ 删分叉的票可以顺手把对应数字下调，**不要**为了「测试绿」而整体重算基线。
 *
 * ## 分类与判别力
 * - 接线守护（读源码文本），**不构成 ③ 门的行为证据**：分叉里的代码写得对不对，本套件答不了，
 *   由 `landingLoginBehavior` / `useBiometric` 等行为套件与 ①a 发版冒烟兜底（ADR-0043）。
 * - 判别力：判据是纯函数 `violations(sources, base)`，真源用例与注入自检共用同一条函数
 *   ——「加一行必红 / 删一行不必红 / 基线外新文件必红」三条注入自检证明它不是恒真。
 *
 * ## 刻意不做
 * 不判语义（不区分 `APP-ANDROID` 与 `H5`、不判成对关系）、不改业务源码、不动 `jest.config.*`。
 */
const h = require('./contractHarness');

/** 一条指令行（行首空白 + 可选注释前缀 + 指令字面）。逐行 test，不挂 `g`。 */
const DIRECTIVE_RE = /^[ \t]*(?:(?:\/\/|<!--|\*)[ \t]*)?#(?:ifdef|ifndef|endif)\b/;

/** 基线：工程内相对路径 → 允许的指令行数（#2331 现测） */
const BASE = {
  'api/request.uts': 6,
  'App.uvue': 4,
  'composables/useBiometric.uts': 12,
  'pages/forum/components/forum-contribution-form.uvue': 14,
  'pages/resume/resume-attach.uvue': 4,
  'utils/externalLinkDispatch.uts': 4,
  'utils/secureStorage.uts': 12,
  'utils/system.uts': 6,
  'utils/systemBrowser.uts': 4
};

/** vendor 面：不属本仓策略边界（见文件头口径） */
const VENDOR_DIR = 'uni_modules/';

/** 单文件里的指令行号（1 起） */
function directiveLines(text) {
  const at = [];
  text.split('\n').forEach((line, i) => {
    if (DIRECTIVE_RE.test(line)) at.push(i + 1);
  });
  return at;
}

const countOf = (text) => directiveLines(text).length;

/**
 * 违规面：`{ path, base, actual, extra, at }`。
 * `at` 给出指令行号——报红时人要知道**在哪一行**加的那条，光有数字还得自己数。
 */
function violations(sources, base = BASE) {
  const has = (k) => Object.prototype.hasOwnProperty.call(base, k);
  const out = [];
  for (const { path, text } of sources) {
    const actual = countOf(text);
    const allowed = has(path) ? base[path] : 0;
    if (actual > allowed) {
      out.push({ path, base: allowed, actual, extra: actual - allowed, at: directiveLines(text) });
    }
  }
  return out.sort((a, b) => (a.path < b.path ? -1 : a.path > b.path ? 1 : 0));
}

/** 工程内参与比对的源文件（排序稳定） */
function sources() {
  return h
    .filesUnder('.', /\.(uvue|uts)$/, true)
    .filter((rel) => !rel.startsWith(VENDOR_DIR))
    .map((rel) => ({ path: rel, text: h.read(rel) }));
}

describe('条件编译白名单：#ifdef / #ifndef / #endif 只减不增（#2331）', () => {
  it('基线文件都在磁盘上（改名不得让棘轮静默变松）', () => {
    const missing = Object.keys(BASE).filter((rel) => !h.exists(rel));
    expect(missing).toEqual([]);
  });

  it('工程内没有越界文件（新文件出现分叉 = 从 0 增到 N，必须红）', () => {
    expect(violations(sources())).toEqual([]);
  });

  it('基线与现值同量级（66 行基线不许悄悄翻倍）', () => {
    const base = Object.values(BASE).reduce((a, b) => a + b, 0);
    expect(base).toBe(66);
  });
});

describe('判据判别力（注入自检：改坏必红 / 真源必不红）', () => {
  const clean = [{ path: 'pages/x/x.uvue', text: '<template>\n  <view class="a" />\n</template>\n' }];

  it('真源无分叉 ⇒ 不违规', () => {
    expect(violations(clean, {})).toEqual([]);
  });

  it('加一行 `// #ifdef H5` ⇒ 红，且报出文件 / 多几行 / 基线', () => {
    const injected = [{ path: 'pages/x/x.uvue', text: clean[0].text + '// #ifdef H5\n' }];
    const got = violations(injected, {});
    expect(got).toHaveLength(1);
    expect(got[0]).toMatchObject({ path: 'pages/x/x.uvue', base: 0, actual: 1, extra: 1, at: [4] });
  });

  it('基线内减少 ⇒ 绿（棘轮只挡增长）', () => {
    const shrunk = [{ path: 'api/request.uts', text: '#ifdef APP-ANDROID\n#endif\n' }];
    expect(violations(shrunk, { 'api/request.uts': 6 })).toEqual([]);
  });

  it('超出基线 1 行 ⇒ 报出该行号（人要知道加在哪）', () => {
    const over = [{ path: 'api/request.uts', text: '// #ifdef APP-ANDROID\n// #endif\n' }];
    expect(violations(over, { 'api/request.uts': 1 })).toEqual([
      { path: 'api/request.uts', base: 1, actual: 2, extra: 1, at: [1, 2] }
    ]);
  });

  it('后缀式 / 行内提及不算指令行（防误杀注释与文档）', () => {
    const t = 'const s = "#ifdef APP-ANDROID"; // 见 #ifdef\n';
    expect(countOf(t)).toBe(0);
  });
});
