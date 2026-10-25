#!/usr/bin/env node
/**
 * 图标入口守卫（ADR-0079 / 票 #1911）。
 *
 * 背景：图标原先有两个事实源 —— 全局注册的一批 Element Plus 图标，加上各页面自己
 * import 的 64 个名字；尺寸、描边、可访问名各写各的。ADR-0079 把图标收成「注册表 + UiIcon
 * 单一入口」之后，这条口径必须由 CI 兜住，否则半年后又一次长回两种写法。
 *
 * 用法（runner 面单点在 `scripts/lib/guard.mjs`，本文件只有判定面）：
 *   node scripts/check-icons.mjs --all  [目录]   全量扫描（默认 frontend/src；有违规则退出 1）
 *   node scripts/check-icons.mjs --diff [base]   只查相对 base 的新增行（base 默认 origin/master）
 *
 * 四条判据：
 *   1. `ep-icon-import`   —— 不得从 `@element-plus/icons-vue` 取图标（业务侧一律走注册表）。
 *   2. `el-icon-tag`      —— 模板里不得再用 `<el-icon>` 包图形。
 *   3. `bare-svg-icon`    —— 不得手写 24 网格图标 `<svg viewBox="0 0 24 24">`（走注册表登记）；
 *                            **跨行开标签也判**（从 `<svg` 起拼到第一个 `>`）。唯一例外是渲染器本体
 *                            `UiIcon.vue`（它就是那处手写 svg 的地方，见 `RENDERER_FILES`）。
 *   4. `uiicon-name`      —— `<UiIcon>` 必须带 `name`（静态或绑定）。
 *
 * 迁移期例外：`icon-legacy-baseline.json` 逐条登记尚未迁移的文件（#1912 清空）。
 * 该基线**只减不增**：表驱动自检会断言每条登记项仍真的违规，迁干净了就必须删行。
 */
import { readFileSync } from 'node:fs'
import { dirname, resolve } from 'node:path'
import { fileURLToPath } from 'node:url'
import { isDirectRun, runGuardCli } from './lib/guard.mjs'
// 根 <template> 的取行口（跳注释、取最后一个 </template>）与 check-el-controls 同一份实现：
// 判定面不同、边界规则相同，抄第二份迟早两边跑偏。
import { visibleTemplateLines } from './check-el-controls.mjs'

const HERE = dirname(fileURLToPath(import.meta.url))

/** 迁移期基线：仓库相对路径 → 理由。清空即「全站只剩 UiIcon」。 */
export const LEGACY_BASELINE = JSON.parse(readFileSync(resolve(HERE, 'icon-legacy-baseline.json'), 'utf8'))

/** 判定面：前端单文件组件。 */
export const GUARDED_PATH_SEGMENT = 'frontend/src/'

/** 从 Element Plus 取图标（含 `import type` 之外的任何形态）。 */
const EP_ICONS_IMPORT = /from\s+['"]@element-plus\/icons-vue['"]/

/** 模板里的 <el-icon> 包裹层。 */
const EL_ICON_TAG = /<el-icon\b/g

/** 手写的 24 网格图标 svg（对拼好的开标签判，单行/跨行同一口径）。 */
const BARE_ICON_SVG = /<svg\b[^>]*\bviewBox\s*=\s*["']0 0 24 24["']/

/**
 * 渲染器本体：它就是「手写 24 网格 svg」的那一处，不可能是「绕开入口的调用点」。
 * 例外只此一条，且按文件名判 —— 不想给 `components/ui/**` 开整片后门。
 */
export const RENDERER_FILES = ['/components/ui/UiIcon.vue']

/** `<UiIcon …>` 开标签（单行）。 */
const UI_ICON_TAG = /<UiIcon\b[^>]*>/g

/** 开标签里是否带了 name（静态或绑定）。 */
const UI_ICON_NAME = /\s:?name\s*=/

/** 规则的报告措辞（自检与报告共用一份，免得两边说法不一致）。 */
export const RULE_HINTS = {
  'ep-icon-import': "改用 UiIcon + 注册表（@/icons/registry）：图标是数据，不是组件。",
  'el-icon-tag': '改用 <UiIcon name="…" />：尺寸/描边/可访问名由同一个入口负责。',
  'bare-svg-icon': '把这个图形登记进 @/icons/registry，再用 <UiIcon :name="…" /> 渲染。',
  'uiicon-name': '<UiIcon> 必须带 name（静态字面量或 :name 绑定），否则渲染什么由运行期决定。'
}

/** 路径是否进判定面（不含放行判断 —— 例外由 runner 的 allowlist 承担）。 */
export function isGuardedPath(filePath) {
  const p = String(filePath).replace(/\\/g, '/')
  return p.includes(GUARDED_PATH_SEGMENT) && p.endsWith('.vue')
}

/**
 * 单文件组件源码 → 违规项。`scanSource` 与自检都走这一个入口，避免「测的函数」
 * 与「跑的函数」不是同一个（静态守卫最典型的假绿形态）。
 */
export function findViolations(source) {
  const text = String(source)
  const lines = text.split('\n')
  const violations = []

  for (let i = 0; i < lines.length; i++) {
    if (EP_ICONS_IMPORT.test(lines[i])) {
      violations.push({ line: i + 1, rule: 'ep-icon-import', text: lines[i].trim() })
    }
  }

  const tpl = visibleTemplateLines(text)
  for (const { line, text: t, raw } of tpl) {
    // 逐处报：一行两个 <el-icon> 是两笔账，报一处等于让人以为改完了一处就干净了
    for (const _ of t.matchAll(EL_ICON_TAG)) {
      violations.push({ line, rule: 'el-icon-tag', text: raw.trim() })
    }
    for (const open of t.matchAll(UI_ICON_TAG)) {
      if (!UI_ICON_NAME.test(open[0])) {
        violations.push({ line, rule: 'uiicon-name', text: raw.trim() })
      }
    }
  }

  // 跨行开标签：从 <svg 那行拼到第一个 '>'（限 8 行，免得属性没闭合时一路吃到文件尾）
  for (let i = 0; i < tpl.length; i++) {
    if (!/<svg\b/.test(tpl[i].text)) continue
    let joined = tpl[i].text
    let j = i
    while (!joined.includes('>') && j + 1 < tpl.length && j - i < 8) {
      j += 1
      joined += ' ' + tpl[j].text
    }
    if (BARE_ICON_SVG.test(joined)) {
      violations.push({ line: tpl[i].line, rule: 'bare-svg-icon', text: tpl[i].raw.trim() })
    }
  }

  return violations
}

/** 源码 + 路径 → 违规项（路径放行由 runner 的 allowlist 处理，渲染器例外只在这里）。 */
export function scanSource(source, filePath) {
  if (!isGuardedPath(filePath)) return []
  const rel = String(filePath).replace(/\\/g, '/')
  const isRenderer = RENDERER_FILES.some((suffix) => rel.endsWith(suffix))
  const violations = findViolations(source)
  return isRenderer ? violations.filter((v) => v.rule !== 'bare-svg-icon') : violations
}

/** 违规项 → 一行报告。 */
export function formatViolation(v) {
  return v.file + ':' + v.line + ': [' + v.rule + '] ' + (RULE_HINTS[v.rule] ?? '') + '\n    ' + v.text
}

export const GUARD_SPEC = {
  name: 'check-icons',
  usage: '用法: node scripts/check-icons.mjs --all [目录] | --diff [base]',
  cli: { noArgs: 'all', helpFlag: false, scanDirArg: true, usageOnUnknown: true, usageStream: 'stderr' },
  all: {
    scanDir: 'frontend/src',
    extensions: ['.vue'],
    skipNodeModules: false,
    tolerateWalkErrors: false,
    stream: 'stdout',
    minChecked: 90,
    header: (ctx) => [
      '===== 图标入口守卫：全量扫描（图标一律走 UiIcon + 注册表）=====',
      '扫描目录: ' + ctx.scanDirRel,
      '判据: ep-icon-import / el-icon-tag / bare-svg-icon / uiicon-name',
      '迁移期基线: ' + Object.keys(LEGACY_BASELINE).length + ' 个文件（#1912 清空，只减不增）',
      '---'
    ],
    ok: (ctx) => '无违规。' + ctx.files.length + ' 个单文件组件都走同一入口。',
    violation: formatViolation,
    footer: (ctx) => [
      '---',
      '共 ' + ctx.count + ' 处。图标是数据：注册进 @/icons/registry，渲染走 <UiIcon name="…" />。',
      '（迁移期例外见 scripts/icon-legacy-baseline.json；迁干净的文件必须从基线删行。）'
    ]
  },
  diff: {
    pathspec: ['*.vue'],
    defaultBase: 'origin/master',
    stream: 'stderr',
    empty: (base) => '[check-icons] 相对 ' + base + ' 无 .vue 新增行，跳过。',
    ok: () => '[check-icons] 新增行没有绕开 UiIcon，通过。',
    header: () => ['===== 新增行绕开了统一图标入口（ADR-0079）====='],
    violation: formatViolation,
    footer: (ctx) => [
      '---',
      '共 ' + ctx.count + ' 处。图标是数据：注册进 @/icons/registry，渲染走 <UiIcon name="…" />。'
    ]
  },
  isGuardedPath,
  scanSource,
  allowlist: LEGACY_BASELINE
}

if (isDirectRun(import.meta.url)) runGuardCli(GUARD_SPEC)
