/**
 * 表单域存量盘点（#1981，只读）。
 *
 * 回答票里卡住的三件事之一：`UiInput` / `UiSelect` 到底覆不覆盖现存用法。
 * 输出：按端 × 文件 × 处数 × 特殊用法，供定批次边界用。
 *
 * 用法：node scripts/inventory-form-fields.mjs [--md <path>] [--json <path>]
 */
import { readdirSync, readFileSync, statSync, writeFileSync } from 'node:fs'
import path from 'node:path'
import { fileURLToPath } from 'node:url'
import { visibleTemplateLines } from './lib/vue-template.mjs'

const ROOT = path.resolve(path.dirname(fileURLToPath(import.meta.url)), '..')
const SRC = path.join(ROOT, 'frontend', 'src')

/** 端：按目录归类（`components/**` 是共享件，单独一档）。 */
function workspaceOf(rel) {
  const rules = [
    ['pages/admin/', '管理端'],
    ['pages/student/', '学员端'],
    ['pages/tutor/', '讲师端'],
    ['pages/recruit/', '招聘端'],
    ['pages/ai-assistant/', 'AI 助手'],
    ['pages/onboarding/', '学员端'],
    ['pages/', '认证页'],
    ['layouts/', '布局壳'],
    ['components/', '共享组件']
  ]
  for (const [prefix, name] of rules) if (rel.startsWith(prefix)) return name
  return '其它'
}

/** 特殊用法：不落 `UiInput` / `UiSelect` 现有 props 的那几种。 */
const MARKERS = [
  ['#prefix', /#prefix\b/],
  ['#suffix', /#suffix\b/],
  ['#prepend', /#prepend\b/],
  ['#append', /#append\b/],
  ['textarea', /type="textarea"/],
  ['show-password', /show-password/],
  ['multiple', /\bmultiple\b/],
  ['filterable', /\bfilterable\b/],
  ['option-group', /<el-option-group/],
  ['ref', /\bref="/],
  ['@keyup/@keydown', /@(keyup|keydown)/],
  ['@blur/@focus', /@(blur|focus)/],
  ['inline-style', /<(el-input|el-select)[^>]*\bstyle="/],
  ['maxlength', /\bmaxlength\b/],
  ['remote', /\bremote\b/]
]

function walk(dir) {
  const out = []
  for (const e of readdirSync(dir, { withFileTypes: true })) {
    const full = path.join(dir, e.name)
    if (e.isDirectory()) out.push(...walk(full))
    else if (e.name.endsWith('.vue')) out.push(full)
  }
  return out
}

export function inventory() {
  const rows = []
  for (const abs of walk(SRC)) {
    const rel = path.relative(SRC, abs).split(path.sep).join('/')
    const source = readFileSync(abs, 'utf8')
    const lines = visibleTemplateLines(source)
    let input = 0
    let select = 0
    let option = 0
    let optionGroup = 0
    for (const { text } of lines) {
      // `\b` 在 `el-input-number` 的 `-` 前成立 ⇒ 必须显式排除带后缀的兄弟标签。
      // 初版踩过：把 21 处 `el-input-number` 全算成 `el-input`，170 报成 191。
      input += (text.match(/<el-input(?![\w-])/g) ?? []).length
      select += (text.match(/<el-select(?![\w-])/g) ?? []).length
      option += (text.match(/<el-option(?![\w-])/g) ?? []).length
      optionGroup += (text.match(/<el-option-group\b/g) ?? []).length
    }
    if (input + select === 0) continue
    const markers = MARKERS.filter(([, re]) => re.test(source)).map(([name]) => name)
    rows.push({
      file: rel,
      workspace: workspaceOf(rel),
      // 与 inventory-ui-forms.mjs 的口径对齐：那边报的 169 是「封装层之外」的数，
      // 本表把封装层也算进来（UiInput / UiFormField 内部本来就有 el-input），两个数要能对上。
      inWrapper: rel.startsWith('components/ui/'),
      input, select, option, optionGroup, markers
    })
  }
  rows.sort((a, b) => b.input + b.select - (a.input + a.select))
  return rows
}

export function totals(rows) {
  const t = { input: 0, select: 0, option: 0, optionGroup: 0, inputBare: 0, selectBare: 0 }
  for (const r of rows) {
    t.input += r.input
    t.select += r.select
    t.option += r.option
    t.optionGroup += r.optionGroup
    if (!r.inWrapper) { t.inputBare += r.input; t.selectBare += r.select }
  }
  return t
}

export function byWorkspace(rows) {
  const map = new Map()
  for (const r of rows) {
    const cur = map.get(r.workspace) ?? { workspace: r.workspace, files: 0, input: 0, select: 0, optionGroup: 0 }
    cur.files += 1
    cur.input += r.input
    cur.select += r.select
    cur.optionGroup += r.optionGroup
    map.set(r.workspace, cur)
  }
  return [...map.values()].sort((a, b) => b.input + b.select - (a.input + a.select))
}

function toMarkdown(rows) {
  const t = totals(rows)
  const ws = byWorkspace(rows)
  const lines = [
    '# 1981 附录 · 表单域存量盘点',
    '',
    '> 由 `scripts/inventory-form-fields.mjs --md` 生成，**不要手改**。',
    '> 口径：单文件组件的根 `<template>` 内 `<el-input>` / `<el-select>` 开标签计数（注释与 `<script>` 不进面）。',
    '',
    '## 按端',
    '',
    '| 端 | 文件 | `el-input` | `el-select` | `el-option-group` |',
    '| --- | --- | --- | --- | --- |'
  ]
  for (const w of ws) lines.push('| ' + w.workspace + ' | ' + w.files + ' | ' + w.input + ' | ' + w.select + ' | ' + w.optionGroup + ' |')
  lines.push('| **合计** | **' + rows.length + '** | **' + t.input + '** | **' + t.select + '** | **' + t.optionGroup + '** |')
  lines.push('', '## 逐文件（按处数降序）', '', '| 文件 | 端 | input | select | option | 特殊用法 |', '| --- | --- | --- | --- | --- | --- |')
  for (const r of rows) {
    lines.push('| `' + r.file + '` | ' + r.workspace + ' | ' + r.input + ' | ' + r.select + ' | ' + r.option + ' | ' + (r.markers.join(' / ') || '—') + ' |')
  }
  lines.push('')
  return lines.join('\n')
}

const argv = process.argv.slice(2)
const rows = inventory()
const t = totals(rows)
const mdIdx = argv.indexOf('--md')
const jsonIdx = argv.indexOf('--json')
if (jsonIdx >= 0 && argv[jsonIdx + 1]) {
  writeFileSync(argv[jsonIdx + 1], JSON.stringify({ totals: t, rows }, null, 2) + '\n', 'utf8')
  console.log('json → ' + argv[jsonIdx + 1])
}
if (mdIdx >= 0 && argv[mdIdx + 1]) {
  writeFileSync(argv[mdIdx + 1], toMarkdown(rows), 'utf8')
  console.log('md → ' + argv[mdIdx + 1])
}
console.log('文件 ' + rows.length + ' · el-input ' + t.input + ' · el-select ' + t.select + ' · el-option ' + t.option + ' · el-option-group ' + t.optionGroup)
for (const w of byWorkspace(rows)) console.log('  ' + w.workspace.padEnd(8) + ' 文件 ' + String(w.files).padStart(3) + '  input ' + String(w.input).padStart(3) + '  select ' + String(w.select).padStart(3) + '  group ' + w.optionGroup)
