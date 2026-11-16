/**
 * 旧主色 #2979ff 全量棘轮守卫（#2532，规格 #2493 缝三）—— 全仓 `.uvue` / `.uts` 源码守护。
 *
 * 为什么需要它：§1.6 机检的射程是「受管页并集」（各批 UI 契约套件扫的 pages 下 60 个页面），
 * **组件目录从来不在射程内** —— 这才是旧蓝在组件里活得最久的原因（规格 Problem Statement 3：
 * 56 个命中文件里 31 个是组件，即使白名单条目消失也没人扫到它们）。更麻烦的是这批旧色曾被
 * 写进 §1.6 白名单（`utils/uiHexWhitelist.js` 的 `#2979FF` 行，**#2533 批 A 已删该条目**），
 * 所有批次 UI 套件对它一律放行 —— 机检不报警，谁也不清。本守卫补的是这块制度缺口，比
 * 「再加一批页面进白名单」更根治；删条目后组件面的残留只由本守卫兜住（批 B/C 继续清）。
 *
 * 判据（棘轮，两个方向都红 —— 缺一条就会长成假锁）：
 *   · 每个文件实测旧色数 **≤ 基线**（`utils/legacyPrimaryColorBaseline.json`）—— 新写入判红；
 *   · 基线 **≥** 实测的文件也要红（基线高于实测 = 有人清了却没改账 ⇒ 防「基线只加不减」；
 *     清退批次 #2533 / #2534 / #2536 各自把对应文件基线**下调到 0（删条目）**）。
 *   ⇒ 两向合起来是「逐文件相等」：任何一侧变化都必须显式改基线，PR diff 里看得见。
 *
 * 同时断言主色 token 本身（uni.scss 的 `$primary-color` = #239EDD）：否则有人可以不清页面、
 * 直接把 token 改回旧蓝让全站「视觉一致」——那是把 #1597 的裁定整个倒回去，必须在这条上红。
 * `$uni-color-primary`（uni 框架变量）同值同断：改它绕过主色的路径与改 `$primary-color` 完全同构。
 *
 * 计数口径（写实，不藏着）：
 *   · `/#2979ff/gi` 对**整文件**计数，**不剥注释**。模板 / script / style 三种注释语法各剥一遍
 *     的解析复杂度本身就是假绿洞（本仓先例：分类器第一版栽在
 *     「注释里提到 .ps1 也算真跑」）。代价是注释里的死值字面量也计数 —— 这是**要的**性质：
 *     棘轮要的是字面量灭绝（复制粘贴的传染源），注释里留着它照样会被人抄回代码。
 *     现测唯一一处注释命中是 `pages/login/composables/useLoginForm.uts:119`（引用
 *     login-agreement 默认值的旧字面），随批 C 清退 login-agreement 时一并改写。
 *   · 大小写不敏感：仓内 `#2979ff` 与 `#2979FF` 两种写法都真实存在（后者 10 处）。
 *   · 8 位带 alpha 的 `#2979FF88` 照算 —— 它渲染出来就是旧蓝。`rgba(41,121,255,·)` 与旧蓝族
 *     浅色（#e8f0fe 等）**不在射程**（规格「明确不动」，见 ② 节合规样本）。
 *
 * 排除面（每条都有 ① 节的真实样本对着，不靠注释自述）：
 *   · `node_modules` 依赖 · `docs` 文档（票面点名；docs 下现无源码，防将来文档代码样例误入）
 *   · `unpackage` 构建产物 · `uni_modules` 第三方插件面（不属于本仓源码，先例 `uvueFontCarrierContract`）
 *   · `.ci-verify` ①a 取证的**术前备份**副本（票面点名；已被点目录规则覆盖，仍显式列出 ——
 *     基线锁的是当前源码，不是术前快照，这条丢了批次 PR 会永远红）
 *   · 点目录一律跳（`.git` / `.scratch` 同族，先例同上）。
 *
 * 反重言自检（票面 AC 明确点名）：临时注入一处旧色时扫描必须报红，否则等于空转。形态沿用
 * 本仓既有全仓守护（先例 `uvueFontCarrierContract.test.js`）：先对**注入的违规样本**断言检测有效，
 * 再对真实文件断言棘轮两向零命中；注入走 `os.tmpdir()` 下的 mkdtemp 唯一目录，不碰真实树。
 *
 * 归腿：接线守护 —— 只读文件文本断言字面量与基线，不起子进程、不 loadUts、无动态 import()；
 * `scripts/lib/guard-split.js` 的判据会自动把它落进 `test:unit` 腿（分类器是唯一真源，不手工登记）。
 */
const fs = require('fs');
const os = require('os');
const path = require('path');

/** 读源码一律经共享读者归一 EOL（ADR-0019）：与检出平台无关，Windows CRLF 也免疫。 */
const { readText } = require('./utsHarness');

const ROOT = path.join(__dirname, '..');
const BASELINE_PATH = 'utils/legacyPrimaryColorBaseline.json';
const BASELINE = require('./legacyPrimaryColorBaseline.json');

/** 旧主色字面（大小写都算，见文件头「计数口径」） */
const OLD_PRIMARY_RE = /#2979ff/gi;
/** 新主色 token 的唯一合法值（#1597 裁定 #2979ff → #239EDD、#1603 落地） */
const PRIMARY_VALUE = '239EDD';

/** 排除目录（都不递归进去；理由见文件头「排除面」） */
const SKIP_DIRS = new Set(['node_modules', 'docs', 'unpackage', 'uni_modules', '.ci-verify']);

/* ------------------------------------------------------------------ 扫描 */

/** 递归收集仓内 `.uvue` / `.uts`（点目录 + 排除目录一律跳过），返回绝对路径（顺序无关，消费方排序） */
function collectScannable(dir, acc = []) {
  let entries;
  try {
    entries = fs.readdirSync(dir, { withFileTypes: true });
  } catch {
    return acc;
  }
  for (const e of entries) {
    const full = path.join(dir, e.name);
    if (e.isDirectory()) {
      if (e.name.startsWith('.')) continue; // .git / .scratch / .ci-verify 同族
      if (SKIP_DIRS.has(e.name)) continue;
      collectScannable(full, acc);
    } else if (e.name.endsWith('.uvue') || e.name.endsWith('.uts')) {
      acc.push(full);
    }
  }
  return acc;
}

/** 单文件旧色计数（整文件、大小写不敏感、不剥注释 —— 见文件头） */
function countOldPrimary(source) {
  return (source.match(OLD_PRIMARY_RE) || []).length;
}

/**
 * 纯函数：扫描根 → `{ '<POSIX 相对路径>': 旧色数 }`（只收录 ≥1 的文件）。
 * 键排序 ⇒ 两次调用可直接 diff，判据不依赖 readdir 顺序（Windows / ubuntu 必须相同）。
 */
function scanCounts(rootDir) {
  const counts = {};
  for (const abs of collectScannable(rootDir)) {
    const n = countOldPrimary(readText(abs));
    if (n > 0) counts[path.relative(rootDir, abs).split(path.sep).join('/')] = n;
  }
  const out = {};
  for (const k of Object.keys(counts).sort()) out[k] = counts[k];
  return out;
}

/* ------------------------------------------------------------ 棘轮判定 */

/**
 * 纯函数：实测 vs 基线 → 两个方向的红清单。两条各对一种失效方向：
 *   · `increases` —— 实测 > 基线（含基线没有的新文件）：旧主色长回来了 / 新页面又写它了；
 *   · `decreases` —— 基线 > 实测（含文件被删 / 条目指向不存在文件时实测按 0 算）：
 *     基线虚高 = 清了没改账 ⇒ 这条逼着清退 PR「同批下调基线」，防这张表长成只加不减的豁免册。
 */
function ratchetFindings(counts, baseline) {
  const increases = [];
  const decreases = [];
  const keys = new Set([...Object.keys(counts), ...Object.keys(baseline)]);
  for (const k of keys) {
    const actual = counts[k] || 0;
    const base = baseline[k] || 0;
    if (actual > base) {
      increases.push(
        k + ' —— 现测 ' + actual + ' 处，基线 ' + base + ' 处：旧主色只减不增。'
        + '新写入请改用 $primary-color（脚本侧字面用 \'#239EDD\'）；确属存量清退的，'
        + '在本 PR 内同批清掉'
      );
    } else if (base > actual) {
      decreases.push(
        k + ' —— 基线 ' + base + ' 处，现测 ' + actual + ' 处：基线高于实测 = 清了没改账。'
        + '把 ' + BASELINE_PATH + ' 里该条目下调到实测数（清到 0 就删条目），禁止让基线虚挂着'
      );
    }
  }
  return { increases, decreases };
}

/* ------------------------------------------------------------ token 断言 */

/**
 * 从 uni.scss 取一个 `$name: #hex;` 声明的值（大写 6 位）。找不到返回 null —— 调用方必须
 * 对 null 判红（fail-closed）：token 被删掉或改形时，这条断言不能静默变绿。
 */
function tokenValue(uniScss, name) {
  const m = uniScss.match(new RegExp('^\\$' + name + '\\s*:\\s*#([0-9a-fA-F]{6})\\s*;', 'm'));
  return m ? m[1].toUpperCase() : null;
}

/* ------------------------------------------------------------------ 测试 */

describe('旧主色 #2979ff 全量棘轮守卫（#2532）', () => {
  /* ---------------- ① 检测器自检：注入必须判红、排除面逐支有真实样本（防空跑假绿） ---------------- */
  describe('① 反重言自检：合成树上的计数、排除面与棘轮两向', () => {
    /** 一处旧蓝的 .uvue 形态（style 小写 + script 大写 —— 两种真实写法各一） */
    const uvueWithTwo = [
      '<template>',
      '  <view class="btn">甲</view>',
      '</template>',
      '<script>',
      'export default {',
      '  data() { return { accent: \'#2979FF\' } },',
      '}',
      '</script>',
      '<style>',
      '.btn { color: #2979ff; }',
      '</style>',
      '',
    ].join('\n');
    const utsWithOne = 'export function accentColor() : string {\n  return \'#2979ff\'\n}\n';

    /**
     * 合成树：两个真命中文件 + 每条排除分支一个带旧蓝的样本。
     * 全部落 os.tmpdir() 的 mkdtemp 唯一目录（本仓 -i 纪律），afterAll 删净。
     */
    let tmpRoot;
    let fixtureCounts;
    beforeAll(() => {
      tmpRoot = fs.mkdtempSync(path.join(os.tmpdir(), 'legacy-primary-'));
      const put = (rel, content) => {
        const abs = path.join(tmpRoot, rel);
        fs.mkdirSync(path.dirname(abs), { recursive: true });
        fs.writeFileSync(abs, content);
      };
      put('a.uvue', uvueWithTwo);
      put('nested/b.uts', utsWithOne);
      // 排除面样本：每条排除分支一个真实文件（guards.md ④：环境分支要能在本机造出来）
      put('docs/old-note.uvue', '.x { color: #2979ff; }');
      put('node_modules/pkg/dep.uvue', '.x { color: #2979ff; }');
      put('unpackage/dist/dev/build.uvue', '.x { color: #2979ff; }');
      put('uni_modules/plug/utssdk/index.uts', "export const c = '#2979ff'\n");
      put('.ci-verify/before.uvue', '.x { color: #2979ff; }');
      put('.scratch/shot.uvue', '.x { color: #2979ff; }');
      fixtureCounts = scanCounts(tmpRoot);
    });
    afterAll(() => {
      fs.rmSync(tmpRoot, { recursive: true, force: true });
    });

    it('注入的旧色必须被**精确**数出来：计数不是空转（票面 AC「注入一处必须报红」的计数面）', () => {
      // 精确值断言（不是 >0）：多计 / 少计 / 恒 0 三种空转形态都在这里露馅
      expect(fixtureCounts['a.uvue']).toBe(2);
      expect(fixtureCounts['nested/b.uts']).toBe(1);
      // 除这两个外全仓无第三键 —— 排除面整体没漏（漏一条这里就会多出一个键）
      expect(Object.keys(fixtureCounts)).toEqual(['a.uvue', 'nested/b.uts']);
    });

    it('排除面逐支都有真实样本：六条分支各自在本机造出来，漏跳过规则哪条哪条红', () => {
      const excluded = [
        'docs/old-note.uvue',          // docs（票面点名）
        'node_modules/pkg/dep.uvue',   // 依赖
        'unpackage/dist/dev/build.uvue', // 构建产物
        'uni_modules/plug/utssdk/index.uts', // 第三方插件面
        '.ci-verify/before.uvue',      // ①a 术前备份（票面点名）
        '.scratch/shot.uvue',          // 点目录规则
      ];
      for (const rel of excluded) {
        expect([rel, fixtureCounts[rel] || 0]).toEqual([rel, 0]);
      }
    });

    it('注入一处旧色而基线没有该条 → increases 判红（反重言主判据）', () => {
      const { increases, decreases } = ratchetFindings(fixtureCounts, {});
      expect(increases).toHaveLength(2); // a.uvue 与 b.uts 各一条
      expect(increases[0]).toContain('a.uvue');
      expect(increases[0]).toContain('现测 2 处');
      expect(increases[0]).toContain('$primary-color'); // 文案必须可执行：写清改法
      expect(decreases).toEqual([]);
    });

    it('实测与基线逐文件一致 → 双向都绿（必不红的一半）', () => {
      expect(ratchetFindings(fixtureCounts, fixtureCounts)).toEqual({ increases: [], decreases: [] });
    });

    it('清掉一处而基线没跟着降 → decreases 判红（防「基线只加不减」）', () => {
      // 部分清退：a.uvue 从 2 清到 1，账没动
      const cleaned = { ...fixtureCounts, 'a.uvue': 1 };
      const { increases, decreases } = ratchetFindings(cleaned, fixtureCounts);
      expect(increases).toEqual([]);
      expect(decreases).toHaveLength(1);
      expect(decreases[0]).toContain('a.uvue');
      expect(decreases[0]).toContain('基线高于实测');
      // 全清（实测 0）同判：清到 0 的文件基线条目必须删，不能虚挂
      const allClean = { 'a.uvue': 0, 'nested/b.uts': 0 };
      expect(ratchetFindings(allClean, fixtureCounts).decreases).toHaveLength(2);
    });

    it('文件被删而基线条目还在 → 按「基线 > 实测(0)」判红（条目自净，不留死键）', () => {
      const { decreases } = ratchetFindings({}, fixtureCounts);
      expect(decreases).toHaveLength(2);
      expect(decreases[0]).toContain(BASELINE_PATH);
    });
  });

  /* ---------------- ② 检测器自检：合规形态不能判红（防误报） ---------------- */
  describe('② 合规样本不误报（射程边界逐条对着规格「明确不动」）', () => {
    const cases = [
      ['新主色 token 值', 'color: #239EDD;'],
      ['脚本侧新主色字面', "return '#239EDD'"],
      ['$primary-color 引用', 'color: $primary-color;'],
      ['主色浅 tint', 'background: #ebf5ff;'],
      ['rgba(41,121,255,·) 旧蓝族（规格明确不动）', 'background: rgba(41, 121, 255, 0.2);'],
      ['近邻 hex：#2979FE 不是 #2979FF', 'color: #2979FE;'],
      ['近邻 hex：#1879ff 不是 #2979ff', 'color: #1879ff;'],
      ['近邻 hex：#29795f 不是 #2979ff', 'color: #29795f;'],
    ];
    test.each(cases)('%s ⇒ 计 0', (_name, src) => {
      expect(countOldPrimary(src)).toBe(0);
    });

    it('大小写三种写法都计（仓内小写 150 / 大写 10 处真实共存），8 位带 alpha 照算', () => {
      expect(countOldPrimary('a: #2979ff')).toBe(1);
      expect(countOldPrimary('a: #2979FF')).toBe(1);
      expect(countOldPrimary('a: #2979Ff')).toBe(1);
      // 渲染出来就是旧蓝（带 alpha），子串计数是**要的**性质，不是缺陷
      expect(countOldPrimary('a: #2979FF88')).toBe(1);
      // '#' 后面不是 2979ff 的（如 #E2979FF）不算 —— 子串语义的负样本
      expect(countOldPrimary('a: #E2979FF')).toBe(0);
    });
  });

  /* ---------------- ③ 真实文件：扫描面是活的 + 棘轮两向零命中 + token 断言 ---------------- */
  describe('③ 真实文件（全仓 .uvue / .uts）', () => {
    const scannedRel = collectScannable(ROOT)
      .map((p) => path.relative(ROOT, p).split(path.sep).join('/'))
      .sort();

    it('扫描面是活的：文件数下界 + 具名钉住（扫描器坏掉 / 根错位时这里是红的，不是「恰好绿」）', () => {
      // 现测 278（2026-11-15）；取 200 留余量。塌到 0（根错位 / readdir 失败吞掉）时主判据
      // 「实测 == 基线」在基线清空后会失去判别力，这条下界就是那时候的兜底。
      expect(scannedRel.length).toBeGreaterThanOrEqual(200);
      // 具名钉住：页面 / 组件 / .uts 三个面各至少一个，且含两个真实携带基线条目的文件 ——
      // 基线谈的文件必须真的被扫到，否则主判据对它是瞎的还自以为绿。
      const pinned = [
        'App.uvue',
        'main.uts',
        'pages/login/login.uvue',
        'components/login-agreement/login-agreement.uvue',
        'utils/forumDisplay.uts',
      ];
      for (const rel of pinned) {
        expect([rel, scannedRel.includes(rel)]).toEqual([rel, true]);
      }
    });

    it('基线数据卫生：键是排序的 POSIX 相对路径、值都是 ≥1 的整数（0 值条目直接删，不许虚挂）', () => {
      const keys = Object.keys(BASELINE);
      expect(keys).toEqual([...keys].sort()); // 排序 ⇒ 批次改账的 PR diff 稳定可读
      for (const k of keys) {
        expect([k, Number.isInteger(BASELINE[k]) && BASELINE[k] >= 1]).toEqual([k, true]);
        expect([k, /^[\w][\w/.-]*\.(uvue|uts)$/.test(k) && !k.includes('\\')]).toEqual([k, true]);
      }
    });

    it('棘轮主判据：实测与基线逐文件相等（两向任何一侧非空即红）', () => {
      const { increases, decreases } = ratchetFindings(scanCounts(ROOT), BASELINE);
      expect(decreases.join('\n')).toBe('');
      expect(increases.join('\n')).toBe('');
    });

    it('主色 token 本身：uni.scss 的 $primary-color 仍是 #239EDD（防「改 token 迎合旧色」绕过）', () => {
      const uniScss = readText(path.join(ROOT, 'uni.scss'));
      // 先 fail-closed：声明没了（被删 / 改形）是红，不是静默放行
      expect([`$primary-color`, tokenValue(uniScss, 'primary-color')]).toEqual(['$primary-color', PRIMARY_VALUE]);
      // uni 框架变量同值同断：改它绕过主色的路径与改 $primary-color 完全同构
      expect([`$uni-color-primary`, tokenValue(uniScss, 'uni-color-primary')]).toEqual(['$uni-color-primary', PRIMARY_VALUE]);
    });
  });
});
