/**
 * uvue 内容页滚动容器契约守护（#1134 回归锁）。
 *
 * 为什么需要它（2026-09-18 确认的回归）：`f1ab2648`（#361「学员端模块重构」）把
 * `pages/courses/courses.uvue` 的 `<scroll-view class="course-scroll" scroll-y="true" @scrolltolower="onLoadMore">`
 * **整个删掉**，而根容器仍绑着 `:style="{ height: windowHeight + 'px' }"` ⇒ 页面被钉成恰好一屏高、
 * 内容溢出被**裁掉**且没有任何可滚动区域 —— 真机现象是「课程页整页滑不动，手指拖动无反应」，
 * 且第 12 条之后的课程（`@scrolltolower` 分页）**不可达**。6 周无人发现，因为全仓没有一条守护
 * 钉「页面自持滚动承载」这件事。
 *
 * 规则（本仓 2026-09-18 现测的页面形态，见 `docs/ui-spec.md:187-226`）：
 *   页面的**模板根元素**若绑定了**固定视口高度**（`windowHeight` / `pageHeight`），
 *   则该页**必须自持滚动承载** —— uvue 原生端页面本身不滚动，滚动能力只能由 `scroll-view` 提供。
 *
 * 合规写法有**两种**（都必须接受，2026-09-18 实测：`pages/exam-info/exam-info.uvue` 用的是第二种）：
 *   · `<scroll-view class="content-scroll" scroll-y="true">`      ← 多数页（dashboard / profile / courses 修好后…）
 *   · `<scroll-view class="content-scroll" direction="vertical">`  ← exam-info.uvue:24
 * 只写成 `scroll-y="false"` **不算**合规。
 *
 * 反例边界（为什么判据是「根元素绑定」而不是「页面长得长」）：
 *   · 页面根元素**不**绑视口高度 ⇒ 不适用本条（布局各页自定，本守护不越权管）；
 *   · `pages/**\/components/**` 下的子组件不是页面、没有根容器契约 ⇒ 跳过；
 *   · 注释里出现 `windowHeight` 不算绑定、注释掉的 `<scroll-view scroll-y>` 不算承载
 *     —— 故两条判据都**先剥注释**再判（同族先例：`utils/gradientSyntaxContract.test.js` 的 `stripComments`）。
 *
 * 设计沿用本仓既有守护的形态（`utils/gradientSyntaxContract.test.js`）：
 * ① 先对「注入违规」的变形样本断言**判得出来**（防空跑假绿），
 * ② 再对合规/无关样本断言**不误报**，
 * ③ 最后对真实文件断言零命中，并带**扫描面锚点自检**（页面数下限，防扫描面退化成空跑）。
 *
 * 跑位：与 `gradientSyntaxContract` / `iconGlyphContract` / `uvueEntityLiteralContract` 同族 ——
 * 放 `utils/*.test.js`，由 CI 的 `npm run test:unit`（`jest.config.unit.js` 扫 `utils/**`）执行；
 * **未**把 token 加进 `scripts/lib/contract-tests.ps1` 的 `Get-ContractTestPattern`
 * （那是 `dev:finish` 的 20 项 scoped 收口，全仓源码契约守护按惯例不在其中）。
 */
const fs = require('fs');
const path = require('path');

/** 读源码一律经共享读者归一 EOL（ADR-0019）：与检出平台无关，Windows CRLF 也免疫。 */
const { readText } = require('./utsHarness');
const ROOT = path.join(__dirname, '..');

/** 剥掉 HTML 注释与 CSS 注释：注释里写着 windowHeight / scroll-y 都不算实现 */
function stripComments(text) {
  return text
    .replace(/<!--[\s\S]*?-->/g, ' ')
    .replace(/\/\*[\s\S]*?\*\//g, ' ');
}

/** 取模板里**第一个**元素的完整开标签（根元素）。取不到返回 '' */
function rootTagOf(source) {
  const i = source.indexOf('<template>');
  if (i === -1) {
    return '';
  }
  const after = source.slice(i + '<template>'.length);
  const start = after.indexOf('<');
  if (start === -1) {
    return '';
  }
  const end = after.indexOf('>', start);
  if (end === -1) {
    return '';
  }
  return after.slice(start, end + 1);
}

/** 该页是否自持滚动承载（两种合规写法任一即可；`scroll-y="false"` 不算） */
function hasScrollContainer(source) {
  return (
    /<scroll-view[^>]*\sscroll-y(?![^>]*=\s*"false")/.test(source) ||
    /<scroll-view[^>]*\sdirection\s*=\s*"vertical"/.test(source)
  );
}

/**
 * 纯函数：单个 `.uvue` 源码 → 违规说明字符串（`null` = 合规 / 本条不适用）。
 */
function scanPageScroll(source) {
  const stripped = stripComments(source);
  const rootTag = rootTagOf(stripped);
  if (!rootTag) {
    return null; // 不是页面（无 template）—— 本条不适用
  }
  if (!/windowHeight|pageHeight/.test(rootTag)) {
    return null; // 根元素没绑固定视口高度 —— 本条不适用
  }
  if (hasScrollContainer(stripped)) {
    return null;
  }
  return (
    '根元素绑定了固定视口高度（' + rootTag.replace(/\s+/g, ' ').trim() + '）但页面内没有滚动承载' +
    '（需要 <scroll-view scroll-y="true"> 或 <scroll-view direction="vertical">）：uvue 原生端页面本身不滚动，' +
    '内容超出这一屏后既看不到也滑不动。先例：#1134（课程页，f1ab2648 删掉了 content-scroll）'
  );
}

/** 递归收集 pages/ 下的页面 .uvue（跳过 components 子目录与构建产物） */
function collectPages(dir, acc = []) {
  let entries;
  try {
    entries = fs.readdirSync(dir, { withFileTypes: true });
  } catch {
    return acc;
  }
  for (const e of entries) {
    const full = path.join(dir, e.name);
    if (e.isDirectory()) {
      if (e.name === 'components' || e.name === 'unpackage' || e.name === 'node_modules') {
        continue;
      }
      collectPages(full, acc);
    } else if (e.name.endsWith('.uvue')) {
      acc.push(full);
    }
  }
  return acc;
}

describe('uvue 内容页滚动容器契约（#1134 回归锁）', () => {
  // ---- ① 注入自检：检测器必须能抓住违规，否则本测试是空跑假绿 ----
  describe('① 检测器自检（注入违规样本必须被判红）', () => {
    const violating = [
      ['根容器绑 windowHeight 且无任何 scroll-view（courses.uvue 回归时的形状）',
        '<template><view class="container" :style="{ height: windowHeight + \'px\' }"><view class="x">a</view></view></template>'],
      ['根容器绑 pageHeight 且无任何 scroll-view',
        '<template><view class="page" :style="{ height: pageHeight + \'px\' }"><view class="x">a</view></view></template>'],
      ['只写了 scroll-y="false"（不算滚动承载）',
        '<template><view :style="{ height: windowHeight + \'px\' }"><scroll-view class="c" scroll-y="false"></scroll-view></view></template>'],
      ['滚动容器被注释掉了（剥注释后不得算数）',
        '<template><view :style="{ height: windowHeight + \'px\' }"><!-- <scroll-view class="c" scroll-y="true"></scroll-view> --></view></template>'],
    ];
    violating.forEach(([name, src]) => {
      it(name, () => {
        const v = scanPageScroll(src);
        expect(v).not.toBeNull();
        expect(v).toMatch(/没有滚动承载/);
      });
    });
  });

  // ---- ② 自检的另一半：合规与「本条不适用」的样本必须判绿（防误报） ----
  describe('② 检测器自检（合规/无关样本必须判绿）', () => {
    const compliant = [
      ['根容器绑 windowHeight + scroll-y="true"（本仓多数页的形态）',
        '<template><view :style="{ height: windowHeight + \'px\' }"><scroll-view class="content-scroll" scroll-y="true"></scroll-view></view></template>'],
      ['根容器绑 windowHeight + direction="vertical"（exam-info.uvue:24 的写法）',
        '<template><view :style="{ height: windowHeight + \'px\' }"><scroll-view class="content-scroll" direction="vertical"></scroll-view></view></template>'],
      ['根元素不绑视口高度（本条不适用，布局由各页自定）',
        '<template><view class="wrap"><view class="x">a</view></view></template>'],
      ['注释里出现 windowHeight 不算绑定',
        '<template><view class="wrap"><!-- height: windowHeight + \'px\' --><view class="x">a</view></view></template>'],
    ];
    compliant.forEach(([name, src]) => {
      it(name, () => {
        expect(scanPageScroll(src)).toBeNull();
      });
    });
  });

  // ---- ③ 对真实文件断言零命中 ----
  it('③ pages/ 下每个绑定固定视口高度的页面都自持滚动容器', () => {
    const files = collectPages(path.join(ROOT, 'pages'));
    expect(files.length).toBeGreaterThan(40); // 锚点自检：扫描面不许退化成空跑

    const offenders = [];
    for (const f of files) {
      const v = scanPageScroll(readText(f));
      if (v) {
        offenders.push(path.relative(ROOT, f).split(path.sep).join('/') + '\n    ' + v);
      }
    }
    expect(offenders).toEqual([]);
  });
});
