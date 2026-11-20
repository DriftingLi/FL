#!/usr/bin/env node
/**
 * 间距 / 对齐审计（**诊断工具，不是门禁**）
 *
 * 为什么有它：2026-11 的「页面间距与对齐」整改连续做了 5 轮，每轮都是同一套动作——
 * 多组正则 grep 全仓 → 逐文件读上下文判断是否原型锁定 → 批量改 → 全量单测回归。
 * 重复审计的根因是**口径没有可执行载体**：24rpx 页面边距 / 卡片内边距、箭槽三件套
 * 只写在 `docs/ui-spec.md` §2 与人脑里。本脚本把这三条判据变成一次扫描能出的候选表。
 *
 * 判据（三条，全部确定性；值源 `docs/ui-spec.md` §2 `$spacing-md` = 24rpx）：
 *   R1 页面级容器（.page/.container/.body/.content/.content-scroll…）水平内距 = 24rpx
 *   R2 卡 / 行（*-card / *-item / *-cell / .card / .cell）水平内距 ∈ {0, 24rpx}
 *   R3 箭头类（*-arrow / *-chevron）若补了 width|line-height|text-align 任一项，则三项须齐全
 *
 * 自动跳过（与人工判据一致，不重复读注释）：
 *   - 规则前 6 行内出现「原型 / 实测 / 票号 NNN / #NNN」的（原型锁值）；
 *   - `scripts/spacing-whitelist.json` 登记的（整文件前缀或单条选择器，必带理由）。
 *
 * 用法：
 *   node scripts/audit-spacing.mjs            # 只列 WARN，SKIP/INFO 只给计数
 *   node scripts/audit-spacing.mjs --all      # 连 SKIP / INFO 一起列
 *   node scripts/audit-spacing.mjs --json     # 机读
 *   node scripts/audit-spacing.mjs --strict   # 有 WARN 时 exit 1（本地自选，**不挂 CI**）
 *
 * 退出码：0 = 完成；1 = --strict 下有 WARN；2 = 读目录 / 白名单失败（fail-closed）
 */

import { readFileSync, readdirSync } from 'node:fs'
import { dirname, join, relative, resolve, sep } from 'node:path'
import { fileURLToPath, pathToFileURL } from 'node:url'

const MOBILE_ROOT = resolve(dirname(fileURLToPath(import.meta.url)), '..')
const PAGES_DIR = join(MOBILE_ROOT, 'pages')
const WHITELIST_PATH = join(MOBILE_ROOT, 'scripts', 'spacing-whitelist.json')

/** 页面级容器（水平内距应为 24rpx；无 padding 声明则不判） */
const PAGE_BASES = new Set(['.page', '.container', '.body', '.content', '.content-scroll', '.page-scroll', '.main'])
/** 卡 / 单元：水平内距只允许 0 或 24rpx */
const CARD_BASE_RE = /(?:^|[-_.])(?:card|cell)$/
/** 行 / 项：只盯「28/32rpx 级」的遗留值；< 24 的属元素自身内衬，不判 */
const ROW_BASE_RE = /(?:^|[-_.])(?:item|row|block)$/
/** 箭头：箭槽三件套（width / line-height / text-align）要么不写，要么写全 */
const ARROW_BASE_RE = /(?:^|[-_.])(?:arrow|chevron)(?:-text)?$/
/** 小于该值的水平内距视为元素自身微调（日历格、标签内衬），不判 */
const MIN_FLAG_PX = 12
/** 规则前 N 行内出现这些字样 ⇒ 原型锁值，SKIP（决策号 / 票号本身不算原型） */
const PROTOTYPE_RE = /原型|实测|票\s*\d{3,}/

const args = new Set(process.argv.slice(2))
const STRICT = args.has('--strict')
const SHOW_ALL = args.has('--all')
const JSON_OUT = args.has('--json')

/** 1-based 行号：fullText 前 idx 个字符里有几个换行 */
function lineOf(fullText, idx) {
  return fullText.slice(0, idx).split('\n').length
}

/** rpx 值解析；`$spacing-md` 按 24 计；解析不出返回 null（如 50%、rgba） */
function px(v) {
  const s = String(v).trim().replace(/;$/, '')
  if (s === '0') return 0
  if (s === '$spacing-md') return 24
  const m = s.match(/^(-?\d+(?:\.\d+)?)rpx$/)
  return m ? Number(m[1]) : null
}

/** 取一条规则的左 / 右水平内距（分别吃 `padding` 简写与 `padding-left/right`） */
function horizontal(decls, prop) {
  let left = null
  let right = null
  let seen = false
  for (const d of decls) {
    if (d.prop === prop) {
      const t = d.value.split(/\s+/).map(px)
      if (t.some((n) => n === null)) continue
      seen = true
      if (t.length === 1) { left = right = t[0] }
      else if (t.length === 2 || t.length === 3) { left = right = t[1] }
      else if (t.length >= 4) { left = t[3]; right = t[1] }
    } else if (d.prop === `${prop}-left` || d.prop === `${prop}-right`) {
      const n = px(d.value)
      if (n === null) continue
      seen = true
      if (d.prop === `${prop}-left`) left = n
      else right = n
    }
  }
  return seen ? { left, right } : null
}

/** 极简 SCSS 规则解析：花括号配对 + 分号切声明（本仓 .uvue 的 style 是扁平的，嵌套只用于拼接选择器） */
function parseRules(css) {
  const rules = []
  const stack = []
  let buf = ''
  let start = -1
  const lineAt = (i) => css.slice(0, i).split('\n').length
  for (let i = 0; i < css.length; i++) {
    const ch = css[i]
    if (ch === '/' && css[i + 1] === '*') {
      const end = css.indexOf('*/', i + 2)
      i = end < 0 ? css.length : end + 1
      continue
    }
    if (ch === '{') {
      stack.push({ sel: buf.trim().replace(/\s+/g, ' '), line: lineAt(start < 0 ? i : start), decls: [] })
      buf = ''
      start = -1
    } else if (ch === '}') {
      const r = stack.pop()
      if (r && r.sel && !r.sel.startsWith('@')) rules.push(r)
      buf = ''
      start = -1
    } else if (ch === ';') {
      const d = buf.trim()
      const top = stack[stack.length - 1]
      if (top && d) {
        const idx = d.indexOf(':')
        if (idx > 0) top.decls.push({ prop: d.slice(0, idx).trim().toLowerCase(), value: d.slice(idx + 1).trim() })
      }
      buf = ''
      start = -1
    } else {
      if (buf.trim() === '' && !/\s/.test(ch)) start = i
      buf += ch
    }
  }
  return rules
}

function walk(dir, out = []) {
  for (const ent of readdirSync(dir, { withFileTypes: true })) {
    const p = join(dir, ent.name)
    if (ent.isDirectory()) walk(p, out)
    else if (ent.name.endsWith('.uvue')) out.push(p)
  }
  return out
}

function loadWhitelist() {
  try {
    const raw = JSON.parse(readFileSync(WHITELIST_PATH, 'utf8'))
    return { files: raw.files ?? [], selectors: raw.selectors ?? [] }
  } catch (e) {
    if (e.code === 'ENOENT') return { files: [], selectors: [] }
    throw e
  }
}

function whitelistHit(wl, rel, base) {
  const byFile = wl.files.find((f) => rel.startsWith(f.prefix.replace(/\\/g, '/')))
  if (byFile) return byFile.reason
  const bySel = wl.selectors.find((s) => s.file === rel && s.selector === base)
  return bySel ? bySel.reason : null
}

/** 审计单个 .uvue：返回 {warn, info, skip} 三类命中 */
function auditFile(abs, wl) {
  const rel = relative(MOBILE_ROOT, abs).split(sep).join('/')
  if (wl.files.some((f) => rel.startsWith(f.prefix.replace(/\\/g, '/')))) {
    return { skip: [{ rel, line: 0, selector: '(整文件)', reason: '白名单：整文件锁定' }], warn: [], info: [] }
  }
  const text = readFileSync(abs, 'utf8')
  const warn = []
  const info = []
  const skip = []
  const lines = text.split('\n')
  const styleRe = /<style\b[^>]*>([\s\S]*?)<\/style>/g
  let m
  while ((m = styleRe.exec(text)) !== null) {
    const css = m[1]
    const cssStart = m.index + m[0].indexOf(css)
    /** css 起点在整份 .uvue 里的行号（1-based）；rule.line 是 css 内的行号 */
    const styleStartLine = lineOf(text, cssStart)
    for (const rule of parseRules(css)) {
      const base = rule.sel.split(/\s+/).pop().replace(/::?[a-z-]+(\([^)]*\))?$/g, '')
      if (!base.startsWith('.')) continue
      const absLine = styleStartLine + rule.line - 1
      const ctx = lines.slice(Math.max(0, absLine - 7), absLine - 1).join('\n')
      const wlReason = whitelistHit(wl, rel, base)
      if (wlReason) {
        skip.push({ rel, line: absLine, selector: base, reason: `白名单：${wlReason}` })
        continue
      }
      if (PROTOTYPE_RE.test(ctx)) {
        skip.push({ rel, line: absLine, selector: base, reason: '上下文标注原型 / 实测 / 票号' })
        continue
      }
      // R1 页面级容器
      if (PAGE_BASES.has(base)) {
        const h = horizontal(rule.decls, 'padding')
        if (h) {
          for (const side of ['left', 'right']) {
            const v = h[side]
            if (v !== null && v !== 0 && v !== 24) {
              warn.push({ rel, line: absLine, selector: base, rule: 'R1 页面边距', current: `${v}rpx`, expected: '24rpx' })
              break
            }
          }
        }
        const mh = horizontal(rule.decls, 'margin')
        if (mh) {
          for (const side of ['left', 'right']) {
            const v = mh[side]
            if (v !== null && v !== 0 && v !== 24) {
              warn.push({ rel, line: absLine, selector: base, rule: 'R1 页面边距', current: `margin ${v}rpx`, expected: '0 或 24rpx' })
              break
            }
          }
        }
        continue
      }
      // R2 卡 / 行
      if (CARD_BASE_RE.test(base)) {
        const h = horizontal(rule.decls, 'padding')
        if (h) {
          const bad = ['left', 'right'].map((s) => h[s]).filter((v) => v !== null && v >= MIN_FLAG_PX && v !== 24)
          if (bad.length > 0) {
            warn.push({ rel, line: absLine, selector: base, rule: 'R2 卡内边距', current: `${bad[0]}rpx`, expected: '24rpx' })
          }
        }
        continue
      }
      if (ROW_BASE_RE.test(base)) {
        const h = horizontal(rule.decls, 'padding')
        if (h) {
          const bad = ['left', 'right'].map((s) => h[s]).filter((v) => v !== null && v >= 24 && v !== 24)
          if (bad.length > 0) {
            warn.push({ rel, line: absLine, selector: base, rule: 'R2 行 / 项内边距', current: `${bad[0]}rpx`, expected: '24rpx' })
          }
        }
        continue
      }
      // R3 箭头（只判文字箭头：有 font-size 的才是 <text>；纯点击盒如 .month-arrow 不适用）
      if (ARROW_BASE_RE.test(base) && rule.decls.some((d) => d.prop === 'font-size')) {
        const has = (p) => rule.decls.some((d) => d.prop === p)
        const slot = ['width', 'line-height', 'text-align'].filter(has)
        if (slot.length > 0 && slot.length < 3) {
          const missing = ['width', 'line-height', 'text-align'].filter((p) => !has(p))
          warn.push({ rel, line: absLine, selector: base, rule: 'R3 箭槽三件套', current: `只有 ${slot.join('/')}`, expected: `缺 ${missing.join('/')}` })
        } else if (slot.length === 0) {
          info.push({ rel, line: absLine, selector: base, rule: 'R3 箭槽三件套', current: '未固定箭槽', expected: '单行行内箭头可保留；多行卡必需' })
        }
      }
    }
  }
  return { warn, info, skip }
}

function fmt(x) {
  return `${x.rel}:${x.line}  ${x.selector}  [${x.rule}] ${x.current} → ${x.expected}`
}

function main() {
  let wl
  try {
    wl = loadWhitelist()
  } catch (e) {
    console.error(`[audit-spacing] 白名单读不出（fail-closed）：${e.message}`)
    return 2
  }
  let files
  try {
    files = walk(PAGES_DIR).sort()
  } catch (e) {
    console.error(`[audit-spacing] 扫不了 pages/（fail-closed）：${e.message}`)
    return 2
  }
  const warn = []
  const info = []
  const skip = []
  for (const abs of files) {
    const r = auditFile(abs, wl)
    warn.push(...r.warn)
    info.push(...r.info)
    skip.push(...r.skip)
  }
  if (JSON_OUT) {
    console.log(JSON.stringify({ scanned: files.length, warn, info, skip }, null, 2))
    return STRICT && warn.length > 0 ? 1 : 0
  }
  for (const w of warn) console.log(`[WARN] ${fmt(w)}`)
  if (SHOW_ALL) {
    for (const i of info) console.log(`[INFO] ${fmt(i)}`)
    for (const s of skip) console.log(`[SKIP] ${s.rel}:${s.line}  ${s.selector}  ${s.reason}`)
  }
  console.log(`---\n扫描 ${files.length} 个 .uvue：WARN ${warn.length} / INFO ${info.length} / SKIP ${skip.length}`)
  if (!SHOW_ALL && (info.length > 0 || skip.length > 0)) {
    console.log('（INFO / SKIP 明细用 --all 或 --json；SKIP 来源 = 原型标注或 spacing-whitelist.json）')
  }
  return STRICT && warn.length > 0 ? 1 : 0
}

if (process.argv[1] && import.meta.url === pathToFileURL(process.argv[1]).href) {
  process.exitCode = main()
}
