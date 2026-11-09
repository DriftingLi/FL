/**
 * 阅读刻度与版式契约（#2358）—— 「四面一套刻度」的最后一道锁。
 *
 * 来路：spec #2351 把「同一份课程材料在四个阅读面（课程章节 / 内容精选 / 论坛 / AI 助手）呈现四种观感」
 * 定为要消灭的问题；#2352–#2357 已把刻度收成 `uni.scss` 的 `$md-*` 段（唯一处）、把块怎么排收进
 * `components/markdown/markdown-blocks.uvue`、把页面标题区从被列表复用的旧类名上摘下来（ADR-0057）。
 * 本守护锁的是**下一处改动**：不许再悄悄长出第五套刻度，也不许把居中规则挂回列表卡片的类名上。
 *
 * 三条守卫（票面 AC1–AC3）
 *   ① 刻度只有一处定义：`markdown-blocks.uvue` 里 `font-size` / `line-height` / `margin*` / `padding*`
 *      四类声明的值只许引 `$md-*` 或写 `0`；四个入口文件同一判据，但只判**块渲染区的类**。
 *   ② 页面标题区：`.page-heading` 必带 `text-align: center`；`.chapter-title` / `.article-title`
 *      的规则块里不得出现 `text-align`（ADR-0057「后果」段的逐条声明，不是「跨文件同名即违规」）。
 *   ③ 四个正文渲染入口各自 import 同一个块渲染件，且全仓只有这一个块渲染件。
 *
 * 射程（#2358 决议 D1，维护者已确认）：**页面外壳不锁** —— 加载态 / 上下篇导航 / 元信息标签 /
 * 收藏图标 / 气泡宽度 / 头像行 / 时间行的裸 rpx（现测 46 处：章节壳 3 + 精选页 32 + 气泡 11）是
 * 外壳布局尺寸、不是正文刻度的载体。把它们锁进来等于把「阅读刻度」扩成全局 UI 字号档，而 spec 明确
 * 拒绝过（全局档 20/24/28/32/36 不含 30/26/34/40）。代价写实：外壳漂移不设防，仍靠人眼与既有 UI 契约；
 * 将来若要锁，另起一票按「外壳尺寸档」单独定义，不混进阅读刻度守卫。
 * ⇒ ① 的入口侧判据因此是「**块渲染区的类**」而不是整文件：判整文件会把那 46 处一起拖进来判红。
 *
 * #2383 追加一组：**⑥ 链接色单源**（spec #2351 色彩条款的最后一格翻值）。`$md-color-link` 只许引
 * `$color-link`、后者取值 `#1b6f9c`（#2338 裁定；此前 `#2979ff` 作文字在四个阅读面实测不足 WCAG AA 4.5:1）。
 * 翻值前该变量停在过渡值，#2356 留下的理由「三面还没接共用件」在三面接入后失效 ⇒ 这组锁「旧蓝不许再被静默写回」。
 *
 * ③ 门身份：这是**接线守护**（读源码文本断言，不真跑被测物），归 `scripts/classify-guards.mjs` 定腿；
 * 判别力由「① 注入变异必须判红」那组自检给出（成对：必红 / 必不红），不拿「跑过一次绿」当验收。
 *
 * 形态照 `utils/uvueFontCarrierContract.test.js` 的先例：先对**注入的违规样本**断言检测有效（防空跑假绿），
 * 再对真实文件断言零命中；helper 自带、不抽公共模块（本仓既有守护一律如此）。
 */
const fs = require('fs')
const path = require('path')

// 读取层归一只有一处真源（`utils/utsHarness.js` 的 `readText`，移动端 ADR-0019 票 B）：
// `scripts/check-contract-read.mjs` 会把「新增的裸 `readFileSync` 读仓内源码」判红，本文件因此不裸读。
const { readText: readUtf8 } = require('./utsHarness')

const ROOT = path.resolve(__dirname, '..')

/** 四面共用的块渲染件：块怎么排、字号行高间距是多少，唯一实现处 */
const RENDERER = 'components/markdown/markdown-blocks.uvue'

/**
 * 四个阅读面的正文渲染入口（#2354–#2356 落地后的最终形态）。
 * 它们是「壳件」：块怎么排一律交给 RENDERER，壳件只持本面的能力位 / 行内轴 / 对外接口。
 */
const FACES = [
  'pages/courses/components/chapter-markdown.uvue',
  'pages/featured/featured-detail.uvue',
  'pages/forum/components/forum-markdown-blocks.uvue',
  'components/ai-chat/ai-chat-bubble.uvue'
]

/** 页面标题区的两个详情页（`.page-heading` 的实例位置） */
const HEADING_FILES = ['pages/courses/chapter-view.uvue', 'pages/featured/featured-detail.uvue']

/** 被列表卡片复用的旧标题类名（ADR-0057 决议 1：规则挂在它们身上会把列表一并转正） */
const LEGACY_TITLE_CLASSES = ['chapter-title', 'article-title']

/** AC1 点名的四类「阅读刻度载体」属性 */
const SCALE_PROPS = new Set([
  'font-size', 'line-height',
  'margin', 'margin-top', 'margin-right', 'margin-bottom', 'margin-left',
  'padding', 'padding-top', 'padding-right', 'padding-bottom', 'padding-left'
])

/** 合法取值：刻度变量（`$md-*`，可空白分隔多个）或 `0`（归零是复位，不是刻度值） */
const SCALE_TOKEN = /^\$md-[a-z0-9]+(?:-[a-z0-9]+)*$/i

/* ------------------------------------------------------------------ 读取与解析 */

const readText = (rel) => readUtf8(path.join(ROOT, rel))

const toPosix = (p) => p.replace(/\\/g, '/')

/** 递归收集项目内 `.uvue`（跳过点目录 / 构建产物 / 依赖，与先例同一张跳过表） */
function collectUvue(dir = ROOT, acc = []) {
  let entries
  try {
    entries = fs.readdirSync(dir, { withFileTypes: true })
  } catch {
    return acc
  }
  for (const e of entries) {
    const full = path.join(dir, e.name)
    if (e.isDirectory()) {
      if (e.name.startsWith('.')) continue
      if (e.name === 'unpackage' || e.name === 'node_modules' || e.name === 'uni_modules') continue
      collectUvue(full, acc)
    } else if (e.name.endsWith('.uvue')) {
      acc.push(toPosix(path.relative(ROOT, full)))
    }
  }
  return acc.sort()
}

/** 剥注释：注释里出现 `30rpx` 不算违规（先例 `uvueFontCarrierContract` 同一判据） */
const stripComments = (text) => text
  .replace(/\/\*[\s\S]*?\*\//g, ' ')
  .replace(/(^|[\s;{}])\/\/[^\n]*/g, '$1 ')

/** `<style>` 块内容（一个文件可能有多块；本仓现状每文件一块，多块也照收） */
function styleBlocks(source) {
  const out = []
  const re = /<style\b[^>]*>([\s\S]*?)<\/style>/g
  let m
  while ((m = re.exec(source)) !== null) out.push(m[1])
  return out
}

/**
 * `<template>` 快照（守卫 ③ 用）。
 * 不能写 `/<template>([\s\S]*?)<\/template>/` —— 本仓 `<app-nav-bar>` 这类组件带具名插槽
 * `<template #right>…</template>`（`featured-detail.uvue` 就是一例），惰性匹配会在**内层插槽**处截断，
 * 于是「模板里有没有 `<MarkdownBlocks>`」被判成没有（#2358 实测踩过一次）。按嵌套深度配平。
 */
function templateOf(source) {
  const open = /<template(\s[^>]*)?>/.exec(source)
  if (!open) return ''
  const tag = /<\/?template\b[^>]*>/g
  tag.lastIndex = open.index
  let depth = 0
  let m
  while ((m = tag.exec(source)) !== null) {
    depth += m[0].startsWith('</') ? -1 : 1
    if (depth === 0) {
      const bodyStart = source.indexOf('>', open.index) + 1
      return source.slice(bodyStart, m.index)
    }
  }
  return source.slice(open.index)
}

const scriptOf = (source) => (/<script[^>]*>([\s\S]*?)<\/script>/.exec(source) || [, ''])[1]

/**
 * 平铺规则解析：`选择器 { 声明 }`。
 * 本仓 `.uvue` 的 `<style>` 现测零 `&` 嵌套、零 `@media`（#2358 实施时现测），故不做 SCSS 展开；
 * 真出现嵌套时本解析器会把内层规则当声明丢掉 —— 那是**漏判**方向，由「① 的计数下界」兜着。
 */
function rulesOf(css) {
  const rules = []
  const re = /([^{}]+)\{([^{}]*)\}/g
  let m
  while ((m = re.exec(css)) !== null) {
    const selectors = m[1].split(',').map((s) => s.trim()).filter(Boolean)
    const declarations = []
    for (const seg of m[2].split(';')) {
      const idx = seg.indexOf(':')
      if (idx < 0) continue
      const prop = seg.slice(0, idx).trim().toLowerCase()
      const value = seg.slice(idx + 1).trim()
      if (!prop || !value) continue
      declarations.push({ prop, value })
    }
    rules.push({ selectors, declarations })
  }
  return rules
}

/** 一个文件里全部规则的平铺读数 */
const rulesOfFile = (rel) => styleBlocks(readText(rel))
  .flatMap((css) => rulesOf(stripComments(css)))

/**
 * 选择器最后一段 compound 上的 class 名。
 * 只判最后一段是既有先例的口径（`.box .label` 的属性落在 `.label` 上，祖先类不参与判定）。
 */
function classNamesOf(selector) {
  const last = selector.trim().split(/\s+/).pop() || ''
  const out = []
  const re = /\.([A-Za-z0-9_-]+)/g
  let m
  while ((m = re.exec(last)) !== null) out.push(m[1])
  return out
}

/** `font-size: 30rpx` / `line-height: 1.6` / `padding: 0 20rpx` 一律是字面量；`$md-*` 与 `0` 合法 */
function isScaleValue(value) {
  const tokens = value.split(/\s+/).filter(Boolean)
  if (tokens.length === 0) return false
  return tokens.every((t) => t === '0' || SCALE_TOKEN.test(t))
}

/**
 * 守卫 ① 的扫描器。`scope`：
 *   · `{ wholeFile: true }` —— 整份 `<style>`（块渲染件本体：它的每一条规则都是刻度载体）；
 *   · `{ surface }` —— 只判「最后一段 compound 命中块渲染区类名」的规则（四个入口文件用）。
 * 返回违规文案数组；空数组 = 零违规。
 */
function typographyViolationsOf(source, scope) {
  const out = []
  const { wholeFile = false, surface = new Set() } = scope || {}
  for (const css of styleBlocks(source)) {
    for (const rule of rulesOf(stripComments(css))) {
      const classes = rule.selectors.flatMap(classNamesOf)
      if (!wholeFile && !classes.some((c) => surface.has(c))) continue
      for (const d of rule.declarations) {
        if (!SCALE_PROPS.has(d.prop)) continue
        if (isScaleValue(d.value)) continue
        out.push(rule.selectors.join(', ') + ' { ' + d.prop + ': ' + d.value + ' }')
      }
    }
  }
  return out
}

/**
 * 块渲染区类名 = 块渲染件源码里出现过的 `md-*` / `mt-*` 标识符。
 *
 * 为什么这样取而不是手抄一张清单：块渲染件的模板用 `:class="blockClasses(idx, 'md-paragraph')"`、
 * `headingClasses()` 这类**函数产出**类名，抄清单必漂；从它自己的源码取 ⇒ 它加一个块类，入口侧判定
 * 自动跟着扩。副作用也可接受：`$md-size-h1` 里的 `md-size-h1` 也会进集合，但那不是任何选择器里的类名。
 */
function blockSurfaceClasses(source) {
  const set = new Set()
  const re = /\b((?:md|mt)-[A-Za-z0-9]+(?:-[A-Za-z0-9]+)*)\b/g
  let m
  while ((m = re.exec(stripComments(source))) !== null) set.add(m[1])
  return set
}

/** 守卫 ②-a：页面标题区居中 —— `.page-heading` 必须存在且带 `text-align: center` */
function headingCenterViolationsOf(source) {
  const out = []
  const rules = styleBlocks(source)
    .flatMap((css) => rulesOf(stripComments(css)))
    .filter((r) => r.selectors.some((s) => classNamesOf(s).includes('page-heading')))
  if (rules.length === 0) return ['缺少 `.page-heading` 规则：页面标题区没有落点']
  const centered = rules.some((r) => r.declarations.some((d) => d.prop === 'text-align'
    && d.value.replace(/\s+/g, '') === 'center'))
  if (!centered) {
    out.push('`.page-heading` 规则里没有 `text-align: center`：页面标题区没居中（ADR-0057 决议 3）')
  }
  return out
}

/** 守卫 ②-b：被列表复用的旧标题类名不得带 `text-align`（逐条声明，不判「跨文件同名」） */
function legacyTitleAlignViolationsOf(source) {
  const out = []
  for (const rule of styleBlocks(source).flatMap((css) => rulesOf(stripComments(css)))) {
    for (const selector of rule.selectors) {
      if (!classNamesOf(selector).some((c) => LEGACY_TITLE_CLASSES.includes(c))) continue
      for (const d of rule.declarations) {
        if (d.prop !== 'text-align') continue
        out.push(selector + ' { text-align: ' + d.value + ' } —— 这是列表卡片也在用的类名，'
          + '居中只许落 `.page-heading`')
      }
    }
  }
  return out
}

/** `import X from '<spec>'` 收集，spec 解析成相对仓根（`/` 分隔）的路径 */
function importsOf(source) {
  const out = []
  const re = /import\s+([A-Za-z0-9_$]+)\s+from\s+['"]([^'"]+)['"]/g
  let m
  while ((m = re.exec(scriptOf(source))) !== null) out.push({ binding: m[1], spec: m[2] })
  return out
}

/** 守卫 ③ 的单文件判定：入口必须 import 到块渲染件本体，且把绑定名用进模板 */
function faceWiringViolationsOf(source, rel) {
  const out = []
  const hits = importsOf(source).filter((i) => resolveSpec(rel, i.spec) === RENDERER)
  if (hits.length === 0) {
    out.push(rel + ' 没有 import `' + RENDERER + '`：正文渲染入口没指向共用件')
    return out
  }
  if (hits.length > 1) out.push(rel + ' 重复 import 了块渲染件 ' + hits.length + ' 次')
  for (const hit of hits) {
    const used = new RegExp('<\\s*' + hit.binding.replace(/\$/g, '\\$') + '[\\s/>]').test(templateOf(source))
    if (!used) out.push(rel + ' import 了 `' + hit.binding + '` 但模板里没用它：接线是死的')
  }
  return out
}

function resolveSpec(rel, spec) {
  if (!spec.startsWith('.')) return spec
  return toPosix(path.normalize(path.join(path.dirname(rel), spec)))
}

/** 合成 `.uvue`：自检样本用，不碰真文件 */
const uvue = (tpl, css) => '<template>\n' + tpl + '\n</template>\n\n<style lang="scss">\n' + css + '\n</style>\n'

/**
 * 守卫 ⑥ 判定（#2383）：阅读面句内链接色的单源与裁定值。
 * 两条判据都**不重算被测物**：别名形态照 #2338 裁定（同一语义不留两个字面量，先例 `$md-color-text: $text-color`），
 * 取值 `#1b6f9c` 是 spec #2351 色彩条款的独立期望值——写回旧蓝或挪走 token 值都必须从这里判红。
 */
function linkColorViolationsOf(scss) {
  const out = []
  const mdLine = /\$md-color-link\s*:\s*([^;]+);/.exec(scss)
  if (!mdLine) return ['`uni.scss` 没有 `$md-color-link` 行：四个阅读面句内链接色的唯一变量没了宿主']
  if (mdLine[1].trim() !== '$color-link') {
    out.push('`$md-color-link` 应引 `$color-link`（#2338 裁定：品牌色相在阅读面不留第二个字面量），当前 `' + mdLine[1].trim() + '`')
  }
  const baseLine = /\$color-link\s*:\s*([^;]+);/.exec(scss)
  if (!baseLine) return out.concat(['`uni.scss` 没有 `$color-link` 行：#2338 裁定 token 丢了宿主'])
  if (baseLine[1].trim().toLowerCase() !== '#1b6f9c') {
    out.push('`$color-link` 应为 `#1b6f9c`（#2338 裁定值，白卡对比度 5.53），当前 `' + baseLine[1].trim() + '`')
  }
  return out
}

/* ------------------------------------------------------------------ 测试 */

const rendererSource = readText(RENDERER)
const blockSurface = blockSurfaceClasses(rendererSource)

describe('阅读刻度与版式契约（#2358）', () => {
  /* ---------------- ① 检测器自检：注入的变异必须判红（AC4） ---------------- */
  describe('① 检测器自检：注入的违规样本必须判红，合法样本必须不红', () => {
    const fresh = (css) => uvue('', css)

    test.each([
      ['裸 rpx 字号', '.md-sz-body { font-size: 30rpx; }', /30rpx/],
      ['无单位行高（AI 气泡旧写法 `1.6` 的形态）', '.md-sz-body { line-height: 1.6; }', /line-height: 1\.6/],
      ['多值间距里混一个裸 rpx', '.md-block { padding: 0 20rpx; }', /20rpx/],
      ['px 字面量同样不是刻度', '.md-paragraph { margin-bottom: 12px; }', /12px/],
      ['刻度变量只写一半也不认（`$md-` 后没有 token）', '.mt-16 { margin-top: $md-; }', /\$md-/]
    ])('%s ⇒ 判红', (_name, css, expected) => {
      expect(typographyViolationsOf(fresh(css), { wholeFile: true }).join('\n')).toMatch(expected)
    })

    test('合法样本不红：`$md-*` 与归零 `0` 都算刻度内', () => {
      const css = [
        '.md-sz-body { font-size: $md-size-body; line-height: $md-line-body; }',
        '.md-table-row { margin-bottom: 0; }',
        '.md-table-cell { padding: $md-table-padding-y $md-table-padding-x; }'
      ].join('\n')
      expect(typographyViolationsOf(fresh(css), { wholeFile: true })).toEqual([])
    })

    test('注释里出现的裸 rpx 不算违规（剥注释先于取值）', () => {
      const css = '/* 这里的 30rpx 只是注释 */\n.md-sz-body { font-size: $md-size-body; }'
      expect(typographyViolationsOf(fresh(css), { wholeFile: true })).toEqual([])
    })

    test('射程边界：外壳类（不在块渲染区）的裸 rpx 不判 —— D1 写实的代价', () => {
      const css = '.message-text { font-size: 28rpx; line-height: 1.6; }'
      const scope = { surface: blockSurfaceClasses(rendererSource) }
      expect(typographyViolationsOf(fresh(css), scope)).toEqual([])
    })

    test('射程边界：入口文件里重新声明块类且写死字面量 ⇒ 判红', () => {
      const css = '.md-paragraph { font-size: 30rpx; }'
      const scope = { surface: blockSurfaceClasses(rendererSource) }
      expect(typographyViolationsOf(fresh(css), scope).join('\n')).toMatch(/md-paragraph[\s\S]*30rpx/)
    })

    test('AC4 变异：把共用件一处 `$md-*` 写回裸 30rpx ⇒ 必须判红（不碰真文件）', () => {
      const mutated = rendererSource.replace('font-size: $md-size-body;', 'font-size: 30rpx;')
      expect(mutated).not.toBe(rendererSource)
      expect(typographyViolationsOf(mutated, { wholeFile: true })).toEqual([
        expect.stringContaining('font-size: 30rpx')
      ])
    })

    test('AC4 变异：把共用件一处间距 `$md-*` 写回裸 24rpx ⇒ 必须判红', () => {
      const mutated = rendererSource.replace('margin-top: $md-space-24;', 'margin-top: 24rpx;')
      expect(mutated).not.toBe(rendererSource)
      expect(typographyViolationsOf(mutated, { wholeFile: true }).join('\n')).toMatch(/margin-top: 24rpx/)
    })

    test('守卫 ② 自检：`.page-heading` 只要没居中就判红；旧类名只要带 text-align 就判红', () => {
      expect(headingCenterViolationsOf(fresh('.page-heading { font-size: $md-size-h1; }')))
        .toEqual([expect.stringContaining('text-align: center')])
      expect(headingCenterViolationsOf(fresh('.page-heading { text-align: center; }'))).toEqual([])
      expect(legacyTitleAlignViolationsOf(fresh('.chapter-title { text-align: center; }')).join('\n'))
        .toMatch(/chapter-title[\s\S]*text-align/)
      expect(legacyTitleAlignViolationsOf(fresh('.chapter-title { font-size: 28rpx; }'))).toEqual([])
    })

    test('守卫 ③ 自检：入口没 import 共用件 ⇒ 判红；import 了但模板不用 ⇒ 判红', () => {
      const face = 'pages/x/x.uvue'
      expect(faceWiringViolationsOf(uvue('<view class="a" />', ''), face)).toEqual([
        expect.stringContaining('没有 import')
      ])
      const deadImport = '<script setup lang="uts">\nimport MarkdownBlocks from \'../../components/markdown/markdown-blocks.uvue\'\n</script>'
      expect(faceWiringViolationsOf(deadImport + '<template>\n<view class="a" />\n</template>', face).join('\n'))
        .toMatch(/模板里没用它/)
    })
  })

  /* ---------------- ② 块渲染件本体：刻度只有一处定义（AC1） ---------------- */
  describe('② 块渲染件：四类声明只许引 `$md-*`（AC1 的一半）', () => {
    test('零裸字面量', () => {
      expect(typographyViolationsOf(rendererSource, { wholeFile: true })).toEqual([])
    })

    test('扫描器没有空跑：块渲染区的类与在判声明都有下界', () => {
      // 下界取现测值的 2/3 量级：排版级改动不该让它掉下去；掉下去先怀疑解析器读不到 `<style>`
      expect(blockSurface.size).toBeGreaterThanOrEqual(20)
      const judged = rulesOfFile(RENDERER)
        .flatMap((r) => r.declarations)
        .filter((d) => SCALE_PROPS.has(d.prop))
      expect(judged.length).toBeGreaterThanOrEqual(25)
    })
  })

  /* ---------------- ③ 四个入口：块渲染区里也不许长第二套刻度（AC1 的另一半） ---------------- */
  describe('③ 四个入口：块渲染区不许再声明字面量（AC1）', () => {
    test('四个入口文件对块渲染区类零违规', () => {
      for (const rel of FACES) {
        expect({ rel, violations: typographyViolationsOf(readText(rel), { surface: blockSurface }) })
          .toEqual({ rel, violations: [] })
      }
    })
  })

  /* ---------------- ④ 页面标题区：居中落实例、不落被复用的类名（AC2） ---------------- */
  describe('④ 页面标题区居中（AC2 / ADR-0057）', () => {
    test.each(HEADING_FILES)('%s 的 `.page-heading` 带 `text-align: center`', (rel) => {
      expect({ rel, violations: headingCenterViolationsOf(readText(rel)) }).toEqual({ rel, violations: [] })
    })

    test('`.chapter-title` / `.article-title` 的规则块里不得出现 `text-align`', () => {
      const violations = collectUvue().flatMap((rel) => legacyTitleAlignViolationsOf(readText(rel))
        .map((v) => rel + ' ' + v))
      expect(violations).toEqual([])
    })

    test('扫描器没有空跑：两个旧类名仍在被扫描的源码里（不是删了才绿的）', () => {
      const all = collectUvue().map((rel) => readText(rel)).join('\n')
      for (const name of LEGACY_TITLE_CLASSES) expect(all).toContain('.' + name)
    })
  })

  /* ---------------- ⑤ 四面入口都指向同一个块渲染件（AC3） ---------------- */
  describe('⑤ 四面入口指向同一个块渲染件（AC3）', () => {
    test.each(FACES)('%s 的正文渲染入口指向共用件', (rel) => {
      expect({ rel, violations: faceWiringViolationsOf(readText(rel), rel) }).toEqual({ rel, violations: [] })
    })

    test('块渲染件只有一个：模板带块分支（`block.type ==`）的 `.uvue` 全仓唯一', () => {
      const renderers = collectUvue().filter((rel) => /block\.type\s*==/.test(templateOf(readText(rel))))
      expect(renderers).toEqual([RENDERER])
    })

    test('消费方就是这四个面：`<MarkdownBlocks` 只出现在入口壳件里', () => {
      const consumers = collectUvue().filter((rel) => /<MarkdownBlocks\b/.test(templateOf(readText(rel))))
      expect(consumers).toEqual([...FACES].sort())
    })

    test('`components/markdown/` 目录里只有这一个块渲染件', () => {
      const dir = path.join(ROOT, 'components/markdown')
      expect(fs.readdirSync(dir).sort()).toEqual(['markdown-blocks.uvue'])
    })
  })

  /* ---------------- ⑥ 链接色单源：#2338 裁定值，#2383 翻值落地 ---------------- */
  describe('⑥ 阅读面句内链接色取单源（#2338 裁定 / #2383 落地）', () => {
    test('合法样本不红：`$md-color-link: $color-link` + `$color-link: #1b6f9c`', () => {
      expect(linkColorViolationsOf('$color-link: #1b6f9c;\n$md-color-link: $color-link;\n')).toEqual([])
    })

    test('变异：`$md-color-link` 写回旧蓝 `#2979ff` ⇒ 必红（不碰真文件）', () => {
      expect(linkColorViolationsOf('$color-link: #1b6f9c;\n$md-color-link: #2979ff;\n').join('\n'))
        .toMatch(/\$md-color-link[\s\S]*#2979ff/)
    })

    test('变异：`$color-link` 取值被挪走 ⇒ 必红', () => {
      expect(linkColorViolationsOf('$color-link: #2979ff;\n$md-color-link: $color-link;\n').join('\n'))
        .toMatch(/\$color-link[\s\S]*#1b6f9c/)
    })

    test('真实 `uni.scss` 零违规：四面句内链接此刻按裁定色渲染', () => {
      expect({ violations: linkColorViolationsOf(readText('uni.scss')) }).toEqual({ violations: [] })
    })
  })
})
