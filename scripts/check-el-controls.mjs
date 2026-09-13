#!/usr/bin/env node
/**
 * 已收敛 Element Plus 控件守卫（ADR-0035「备选转正」/ spec #940 片一）。
 *
 * 背景：弹窗 / 空态 / 分页与筛选栏 / 分段控件四类收敛完成（#714/#716/#717/#718）后，
 * 「业务页面不得直接使用 Element Plus 控件」这条规则一直靠评审人眼守。本脚本把它变成
 * CI 可核验的事实：已收敛控件出现在业务模板里 → 报红并指路封装组件。
 *
 * 用法：
 *   node scripts/check-el-controls.mjs --all  [目录]   全量扫描（默认 frontend/src；有违规则退出 1）
 *   node scripts/check-el-controls.mjs --diff [base]   只查相对 base 的新增行（base 默认 origin/master）
 *
 * 判定面：只扫单文件组件（.vue）的**根 <template> 块**，跳过 <!-- --> 注释。
 *   —— 避开 <script> 里的字符串（如 EP 类名字面量）与注释里的控件名造成的假红。
 *
 * 放行面（有意不进守卫集，理由见 ADR-0035 §3 / ADR-0038 §4）：
 *   1. frontend/src/components/ui/** —— 封装层内部本来就是 EP 控件；
 *   2. el-radio / el-radio-button / el-checkbox —— 组的「内容项」而非「容器」，保持原生；
 *   3. el-table / el-table-column —— 明确的适用边界，走全局样式变量、不封装；
 *   4. 表单域与布局类 EP（el-input / el-form / el-select / el-icon / el-row …）—— 尚未封装的例外。
 */
import { readFileSync, readdirSync, statSync } from 'node:fs'
import { execFileSync } from 'node:child_process'
import { dirname, join, resolve } from 'node:path'
import { fileURLToPath, pathToFileURL } from 'node:url'

const ROOT = resolve(dirname(fileURLToPath(import.meta.url)), '..')
const DEFAULT_SCAN_DIR = join(ROOT, 'frontend', 'src')

/** 守卫集：已收敛到封装层的控件，业务代码出现即违规。 */
export const GUARDED_CONTROLS = [
  'el-dialog', // → UiDialog
  'el-empty', // → UiEmptyState
  'el-pagination', // → UiPagination
  'el-button', // → UiButton
  'el-tag', // → UiTag（tone 语义 API）
  'el-switch', // → UiSwitch
  'el-checkbox-group', // → UiCheckboxGroup（组内 el-checkbox 保持原生）
  'el-radio-group', // → UiRadioGroup
  'el-upload', // → UiUpload
  'el-tooltip' // → UiTooltip
]

/** 封装层自身放行（内部就是 EP 控件）。 */
export const ALLOWED_PATH_SEGMENTS = ['/components/ui/']

/** 路径是否在放行面（封装层内部）。 */
export function isAllowedPath(filePath) {
  const normalized = String(filePath).replace(/\\/g, '/')
  return ALLOWED_PATH_SEGMENTS.some((seg) => normalized.includes(seg))
}

/**
 * 取根 <template> 块的可见行（1-based 行号 + 去掉注释后的文本）。
 * 无根模板（如纯 script 组件）→ 空数组，即无判定面。
 */
export function visibleTemplateLines(source) {
  const lines = String(source).split('\n')
  let start = -1
  let end = -1
  for (let i = 0; i < lines.length; i++) {
    if (start < 0) {
      if (/^\s*<template[\s>]/.test(lines[i])) start = i + 1
      continue
    }
    // 取**最后一个**闭合标签：具名插槽的 <template #footer> 也会出现 </template>，
    // 若取第一个就会在插槽处提前收尾 —— 那等于把文件后半段的模板静默漏检（假绿）。
    if (/^\s*<\/template>\s*$/.test(lines[i])) end = i
  }
  if (start < 0) return []
  if (end < 0) end = lines.length

  const out = []
  let inComment = false
  for (let i = start; i < end; i++) {
    const raw = lines[i]
    let text = raw
    if (inComment) {
      const close = text.indexOf('-->')
      if (close < 0) continue
      text = text.slice(close + 3)
      inComment = false
    }
    // 同一行可能有多段注释：循环剥离
    for (;;) {
      const open = text.indexOf('<!--')
      if (open < 0) break
      const close = text.indexOf('-->', open + 4)
      if (close < 0) {
        text = text.slice(0, open)
        inComment = true
        break
      }
      text = text.slice(0, open) + ' ' + text.slice(close + 3)
    }
    out.push({ line: i + 1, text, raw })
  }
  return out
}

/** 单文件组件源码 → 违规项（不做路径放行判断，供表驱动自检直接喂文本）。 */
export function findViolations(source) {
  const violations = []
  for (const { line, text, raw } of visibleTemplateLines(source)) {
    const re = /<(el-[a-z0-9-]+)\b/g
    let m
    while ((m = re.exec(text))) {
      const tag = m[1]
      if (!GUARDED_CONTROLS.includes(tag)) continue
      violations.push({ line, tag, text: raw.trim() })
    }
  }
  return violations
}

/** 源码 + 路径 → 违规项（先过放行面）。 */
export function scanSource(source, filePath) {
  if (isAllowedPath(filePath)) return []
  return findViolations(source)
}

function walkVue(dir) {
  const out = []
  for (const name of readdirSync(dir)) {
    const p = join(dir, name)
    const st = statSync(p)
    if (st.isDirectory()) out.push(...walkVue(p))
    else if (name.endsWith('.vue')) out.push(p)
  }
  return out
}

function scanFiles(files) {
  const violations = []
  for (const file of files) {
    const source = readFileSync(file, 'utf8')
    for (const v of scanSource(source, file)) {
      violations.push({ file: file.replace(ROOT + '/', ''), ...v })
    }
  }
  return violations
}

function reportAll(scanDir) {
  const files = walkVue(scanDir)
  const violations = scanFiles(files)
  console.log('===== 已收敛控件守卫：全量扫描（业务模板不得裸用封装层已收的控件）=====')
  console.log('扫描目录: ' + scanDir.replace(ROOT + '/', ''))
  console.log('守卫集: ' + GUARDED_CONTROLS.join(' / '))
  console.log('---')
  if (violations.length === 0) {
    console.log('无违规。' + files.length + ' 个单文件组件的模板均未裸用已收敛控件。')
    return 0
  }
  for (const v of violations) console.log(v.file + ':' + v.line + ': <' + v.tag + '>  ' + v.text)
  console.log('---')
  console.log('共 ' + violations.length + ' 处。请改用 components/ui/ 的对应封装组件：')
  console.log('  el-dialog→UiDialog / el-empty→UiEmptyState / el-pagination→UiPagination / el-button→UiButton')
  console.log('  el-tag→UiTag / el-switch→UiSwitch / el-checkbox-group→UiCheckboxGroup / el-radio-group→UiRadioGroup')
  console.log('  el-upload→UiUpload / el-tooltip→UiTooltip')
  console.log('（表格 el-table、组内内容项 el-radio / el-checkbox、表单域与布局类 EP 不在守卫集，属有意放行。）')
  return 1
}

/** 解析 git diff 的新增行 → Map<相对路径, Set<行号>>（逐行走 hunk，上下文行也推进新侧行号）。 */
export function parseAddedLines(diffText) {
  const added = new Map()
  let file = null
  let lineNo = 0
  let inHunk = false
  for (const line of String(diffText).split('\n')) {
    const f = line.match(/^\+\+\+ b\/(.+)$/)
    if (f) {
      file = f[1]
      if (!added.has(file)) added.set(file, new Set())
      inHunk = false
      continue
    }
    const h = line.match(/^@@ -[0-9]+(?:,[0-9]+)? \+([0-9]+)(?:,[0-9]+)? @@/)
    if (h) {
      lineNo = Number(h[1])
      inHunk = true
      continue
    }
    if (!inHunk || !file) continue
    if (line.startsWith('+')) {
      if (line.startsWith('+++')) continue
      added.get(file).add(lineNo)
      lineNo++
      continue
    }
    if (line.startsWith('-')) continue // 删除行不影响新侧行号
    if (line.startsWith('\\')) continue // "\ No newline at end of file"
    lineNo++ // 上下文行（含空行）推进新侧行号
  }
  return added
}

function reportDiff(base) {
  try {
    execFileSync('git', ['rev-parse', '--verify', base], { cwd: ROOT, stdio: 'ignore' })
  } catch {
    console.error('[check-el-controls] 无法解析 base ref: ' + base + '（CI 上请先 git fetch）')
    return 2
  }
  const diff = execFileSync('git', ['diff', '-U0', base + '...HEAD', '--', '*.vue'], {
    cwd: ROOT,
    encoding: 'utf8',
    maxBuffer: 64 * 1024 * 1024
  })
  const added = parseAddedLines(diff)
  if (added.size === 0) {
    console.log('[check-el-controls] 相对 ' + base + ' 无 .vue 新增行，跳过。')
    return 0
  }
  const violations = []
  for (const [file, lines] of added) {
    const abs = join(ROOT, file)
    let source
    try {
      source = readFileSync(abs, 'utf8')
    } catch {
      continue // 删除的文件
    }
    for (const v of scanSource(source, file)) {
      if (lines.has(v.line)) violations.push({ file, ...v })
    }
  }
  if (violations.length === 0) {
    console.log('[check-el-controls] 新增行未裸用已收敛控件，通过。')
    return 0
  }
  console.error('===== 新增行裸用了已收敛控件（业务页面一律走 components/ui/ 封装层）=====')
  for (const v of violations) console.error(v.file + ':' + v.line + ': <' + v.tag + '>  ' + v.text)
  console.error('---')
  console.error('共 ' + violations.length + ' 处。请改用对应封装组件（见 ADR-0035 / docs/agents/ui-conventions.md）。')
  return 1
}

function main(argv) {
  const mode = argv[0] ?? '--all'
  if (mode === '--all') {
    const dir = argv[1] ? resolve(argv[1]) : DEFAULT_SCAN_DIR
    return reportAll(dir)
  }
  if (mode === '--diff') {
    return reportDiff(argv[1] ?? 'origin/master')
  }
  console.error('用法: node scripts/check-el-controls.mjs --all [目录] | --diff [base]')
  return 2
}

// 仅作为 CLI 直接运行时才执行：被 import（自检里喂源码文本）时不产生副作用，
// 否则「导入即全树扫描」会让自检继承工作树的结论。
if (process.argv[1] && import.meta.url === pathToFileURL(process.argv[1]).href) {
  process.exitCode = main(process.argv.slice(2))
}
