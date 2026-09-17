#!/usr/bin/env node
/**
 * 已收敛 Element Plus 控件守卫（ADR-0035「备选转正」/ spec #940 片一）。
 *
 * 背景：弹窗 / 空态 / 分页与筛选栏 / 分段控件四类收敛完成（#714/#716/#717/#718）后，
 * 「业务页面不得直接使用 Element Plus 控件」这条规则一直靠评审人眼守。本脚本把它变成
 * CI 可核验的事实：已收敛控件出现在业务模板里 → 报红并指路封装组件。
 *
 * 用法（runner 面单点在 `scripts/lib/guard.mjs`，ADR-0056 §5 / #1094；本文件只有判定面）：
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
import { isDirectRun, runGuardCli } from './lib/guard.mjs'

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

/**
 * 本守卫的声明：判定面 + 报告措辞。runner（argv / 走查 / --diff / allowlist / 退出码）
 * 在 scripts/lib/guard.mjs —— 新增守卫只需实现 scanSource 并声明这一份。
 */
export const GUARD_SPEC = {
  name: 'check-el-controls',
  usage: '用法: node scripts/check-el-controls.mjs --all [目录] | --diff [base]',
  cli: { noArgs: 'all', helpFlag: false, scanDirArg: true, usageOnUnknown: true, usageStream: 'stderr' },
  all: {
    scanDir: 'frontend/src',
    extensions: ['.vue'],
    skipNodeModules: false,
    tolerateWalkErrors: false,
    stream: 'stdout',
    header: (ctx) => [
      '===== 已收敛控件守卫：全量扫描（业务模板不得裸用封装层已收的控件）=====',
      '扫描目录: ' + ctx.scanDirRel,
      '守卫集: ' + GUARDED_CONTROLS.join(' / '),
      '---'
    ],
    ok: (ctx) => '无违规。' + ctx.files.length + ' 个单文件组件的模板均未裸用已收敛控件。',
    violation: (v) => v.file + ':' + v.line + ': <' + v.tag + '>  ' + v.text,
    footer: (ctx) => [
      '---',
      '共 ' + ctx.count + ' 处。请改用 components/ui/ 的对应封装组件：',
      '  el-dialog→UiDialog / el-empty→UiEmptyState / el-pagination→UiPagination / el-button→UiButton',
      '  el-tag→UiTag / el-switch→UiSwitch / el-checkbox-group→UiCheckboxGroup / el-radio-group→UiRadioGroup',
      '  el-upload→UiUpload / el-tooltip→UiTooltip',
      '（表格 el-table、组内内容项 el-radio / el-checkbox、表单域与布局类 EP 不在守卫集，属有意放行。）'
    ]
  },
  diff: {
    pathspec: ['*.vue'],
    defaultBase: 'origin/master',
    stream: 'stderr',
    empty: (base) => '[check-el-controls] 相对 ' + base + ' 无 .vue 新增行，跳过。',
    ok: () => '[check-el-controls] 新增行未裸用已收敛控件，通过。',
    header: () => ['===== 新增行裸用了已收敛控件（业务页面一律走 components/ui/ 封装层）====='],
    violation: (v) => v.file + ':' + v.line + ': <' + v.tag + '>  ' + v.text,
    footer: (ctx) => [
      '---',
      '共 ' + ctx.count + ' 处。请改用对应封装组件（见 ADR-0035 / docs/agents/ui-conventions.md）。'
    ]
  },
  isGuardedPath: (filePath) => !isAllowedPath(filePath),
  scanSource,
  allowlist: null
}

if (isDirectRun(import.meta.url)) runGuardCli(GUARD_SPEC)
