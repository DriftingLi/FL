/**
 * UI 意图登记表的机器可读派生（ADR-0083 决策 6 / #1982）。
 *
 * **数据源只有一个**：`docs/agents/ui-conventions.md` 的「意图登记表」小节。本模块把那里的
 * 三张表解析成集合，守卫与自检都从这里取 —— 不落一份手维护的 JSON 清单（那就是新的漂移源，
 * 也正是本票要收掉的东西）。
 *
 * 解析出的三张表（按表头认，不按位置认 —— 加一行、换个顺序都不会错配）：
 *   | 意图 | 三轴 | 唯一形态 | 存量 |   → 唯一形态里出现的 `el-*` = **原生登记**；存量里的 = **按域分批欠账**
 *   | 件 | 唯一形态 | 存量 |              → 结构件，口径同上
 *   | 子件 | 父件 |                        → **子件随父件**：父件被登记，子件即被登记
 *
 * 为什么「子件」要单独一张表：`<el-option>` 只在 `<el-select>` 里合法、`<el-table-column>` 只在
 * `<el-table>` 里合法。把它们写成独立登记行会让人以为可以单独落点，写进「未登记」又会把所有
 * 表格页判红 —— 它们没有独立意图，只有父件。
 */
import { readFileSync } from 'node:fs'
import path from 'node:path'
import { fileURLToPath } from 'node:url'

const ROOT = path.resolve(path.dirname(fileURLToPath(import.meta.url)), '..', '..')

export const REGISTRY_DOC = path.join(ROOT, 'docs', 'agents', 'ui-conventions.md')

/** 小节标题：登记表与其后两张附表都在这一节里（到下一个 `### ` 为止）。 */
const SECTION_TITLE = '### 意图登记表'

const EL_TAG = /\bel-[a-z0-9-]+\b/g

/** 行 → 单元格。`\|` 是转义竖线（表格正文里出现过），不当作分隔符。 */
function splitRow(line) {
  const trimmed = line.trim()
  if (!trimmed.startsWith('|')) return null
  const cells = trimmed.split(/(?<!\\)\|/)
  if (cells.length > 0 && cells[0].trim() === '') cells.shift()
  if (cells.length > 0 && cells[cells.length - 1].trim() === '') cells.pop()
  return cells.map((c) => c.trim())
}

function isSeparatorRow(cells) {
  return cells.length > 0 && cells.every((c) => /^:?-{2,}:?$/.test(c))
}

/** 取某一节（到下一个同级或更高级标题为止）里的全部表格块。 */
export function parseTables(markdown, sectionTitle = SECTION_TITLE) {
  const lines = String(markdown).split('\n')
  const start = lines.findIndex((l) => l.trim().startsWith(sectionTitle))
  if (start < 0) throw new Error('登记表小节不存在：' + sectionTitle)
  let end = lines.length
  for (let i = start + 1; i < lines.length; i++) {
    if (/^#{2,3}\s/.test(lines[i].trim())) { end = i; break }
  }

  const tables = []
  let current = null
  for (let i = start + 1; i < end; i++) {
    const cells = splitRow(lines[i])
    if (!cells) { current = null; continue }
    if (isSeparatorRow(cells)) continue
    if (!current) {
      current = { header: cells, rows: [], line: i + 1 }
      tables.push(current)
      continue
    }
    current.rows.push(cells)
  }
  return tables
}

function tagsIn(text) {
  const out = []
  let m
  EL_TAG.lastIndex = 0
  const s = String(text ?? '')
  while ((m = EL_TAG.exec(s))) out.push(m[0])
  return out
}

function tableOf(tables, firstHeaderCell) {
  const t = tables.find((x) => x.header[0] === firstHeaderCell)
  if (!t) throw new Error('登记表缺少「' + firstHeaderCell + '」表 —— 表头被改名了？')
  return t
}

/**
 * markdown → 登记面。
 * @returns {{native:Set<string>, pending:Set<string>, children:Map<string,string>, counts:object}}
 */
export function loadRegistry(markdown) {
  const tables = parseTables(markdown)
  const intent = tableOf(tables, '意图')
  const structural = tableOf(tables, '件')
  const childTable = tableOf(tables, '子件')

  const native = new Set()
  const pending = new Set()
  const children = new Map()
  const wrapperOf = new Map()

  // 意图表：cells = [意图, 三轴, 唯一形态, 存量]；结构件表：cells = [件, 唯一形态, 存量]
  for (const cells of [...intent.rows, ...structural.rows]) {
    const form = cells.length >= 4 ? cells[2] : cells[1]
    const stock = cells.length >= 4 ? cells[3] : cells[2]
    for (const tag of tagsIn(form)) native.add(tag)
    for (const tag of tagsIn(stock)) pending.add(tag)
  }
  // 欠账标签 → 它该改用的封装件（只用于报告措辞，不参与判定）：
  // 意图表「存量」写了 `el-input` 待迁、「唯一形态」写了 `UiInput` ⇒ 映射成立。
  for (const cells of intent.rows) {
    const wrapper = (String(cells[2]).match(/`(Ui[A-Za-z]+)`/) ?? [])[1]
    if (!wrapper) continue
    for (const tag of tagsIn(cells[3])) wrapperOf.set(tag, wrapper)
  }

  for (const cells of childTable.rows) {
    // 一格可以写多个子件（`el-option` / `el-option-group`、`el-radio` / `el-radio-button`）：
    // 只取第一个会让同格里的第二个子件落进「未登记」—— 那正是本表要消灭的假红。
    const parent = tagsIn(cells[1] ?? '')[0]
    if (!parent) continue
    for (const child of tagsIn(cells[0])) children.set(child, parent)
  }

  // 原生优先于欠账：`el-tabs` 同时出现在「面板切换（原生登记）」与「参数选择（2 处待迁）」两行，
  // 那是同一标签的不同落点，登记过原生就是原生。
  for (const tag of native) pending.delete(tag)

  return {
    native,
    pending,
    children,
    wrapperOf,
    counts: { native: native.size, pending: pending.size, children: children.size, tables: tables.length }
  }
}

/** 从磁盘读文档并派生（守卫与 CLI 的入口）。 */
export function loadRegistryFromFile(file = REGISTRY_DOC) {
  return loadRegistry(readFileSync(file, 'utf8'))
}

function known(tag, registry, seen = new Set()) {
  if (registry.native.has(tag)) return true
  if (registry.pending.has(tag)) return true
  if (seen.has(tag)) return false
  seen.add(tag)
  const parent = registry.children.get(tag)
  return parent ? known(parent, registry, seen) : false
}

/**
 * 单个 `el-*` 的归类。
 * kind: native（原生登记）/ pending（按域分批欠账）/ child（子件，随父件）/ unknown（未登记）
 */
export function classify(tag, registry) {
  if (registry.native.has(tag)) return { kind: 'native' }
  if (registry.pending.has(tag)) return { kind: 'pending' }
  if (registry.children.has(tag)) {
    const parent = registry.children.get(tag)
    return known(parent, registry) ? { kind: 'child', parent } : { kind: 'unknown', parent, orphan: true }
  }
  return { kind: 'unknown' }
}
