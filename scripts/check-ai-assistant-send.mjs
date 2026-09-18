#!/usr/bin/env node
/**
 * AI 助手发送编排守卫（ADR-0056 §10 / issue #1104）。
 *
 * 背景：两个页面（AIAssistantPage / FeatureChatPage）此前逐字各写一份发送编排
 * （trim → streaming 守卫 → 清 inputText → await sendMessage → 吞错），底层 SSE 面
 * （`aiAssistantApi.streamChat` + onChunk/onDone/onError handlers）也随之长在页面里；
 * 「可发送」四处派生、收尾三份 implementation。这次把一轮发送的编排与三种终态
 * （done / error / aborted）收进 `stores/aiAssistant.ts`，本守卫把
 * 「`pages/ai-assistant` 不得再出现发送编排」变成 CI 可核验的事实。
 *
 * 用法（runner 面在 #1094 收成 `scripts/lib/guard.mjs`；本分支尚未并入该 runner，
 * 故下方自带同形的极简 runner —— 并入后删掉 `main/isDirectRun` 一段，改调
 * `runGuardCli(GUARD_SPEC)` 即可，判定面（GUARD_SPEC 对象）不动）：
 *   node scripts/check-ai-assistant-send.mjs --all  [目录]   全量扫描（默认 frontend/src；有违规则退出 1）
 *   node scripts/check-ai-assistant-send.mjs --diff [base]   只查相对 base 的新增行（base 默认 origin/master）
 *
 * 判定面：只扫 `frontend/src/pages/ai-assistant/**` 的 .vue / .ts，只认两个「发送编排入口」的名字：
 *   - `sendMessage`：收编前的 store 发送 action（已更名为 `send`）——页面里再出现它即旧编排回潮；
 *   - `streamChat`：SSE 流式面（handlers 只作为它的实参存在）——页面直接起流即绕过 store 的编排与终态。
 * 页面调用 `store.send(正文, opts)`（单次委派）不在判定面内：编排（前置守卫 / 清空 / await / 收尾）
 * 才是本守卫的对象。判据刻意窄 —— 不扫 `onDone` / `onError` 这类通用标识符（页面自己的回调用同款
 * 命名就会假红）；整行注释也不算（说明性文字常引用被禁的名字，注释不是调用）。
 *
 * 放行面（有意不进守卫集）：
 *   1. `__tests__` / *.spec.* / *.test.* —— 测试按定义要 mock store 与流式面（重试路径的页面用例
 *      还必须替身 streamChat）；
 *   2. 其余目录（stores / components / composables …）—— 壳与 store 正是编排的宿主，不在射程。
 * 白名单（ALLOWLIST）初始为空：确有例外时逐条登记并写明理由，不要放宽规则本身。
 */

// runner 面走守卫单点（ADR-0056 §5 / #1094）：argv / 走查 / --diff 取 diff / allowlist / 报告与退出码。
import { isDirectRun, runGuardCli } from './lib/guard.mjs'

/** 守卫面：AI 助手页面目录。 */
export const GUARDED_PATH_SEGMENT = '/pages/ai-assistant/'

/** 扫描面后缀。 */
export const SCAN_EXTENSIONS = ['.vue', '.ts']

/**
 * 判定面 = 两个「发送编排入口」的名字。见文件头：判据窄是刻意的，
 * 编排换个名字重写本守卫抓不到 —— 那属于评审面，守卫只钉住既有的两个入口。
 */
export const GUARDED_TOKENS = ['sendMessage', 'streamChat']

/**
 * 逐条登记的有意例外（路径 → 理由）。收编本身把两页的编排全部收回 store，
 * 没有为任何页面预留豁免：本表保持为空。
 * @type {Record<string, string>}
 */
export const ALLOWLIST = {}

/** 整行注释（含 JSDoc / HTML 注释）不算判定对象。 */
export function isCommentLine(line) {
  const t = String(line).trim()
  return t.startsWith('//') || t.startsWith('*') || t.startsWith('/*') || t.startsWith('<!--')
}

/** 测试文件不进判定面（按定义要 mock store 与流式面）。 */
export function isTestFile(filePath) {
  const p = String(filePath).replace(/\\/g, '/')
  return p.includes('/__tests__/') || /\.(spec|test)\.[jt]s$/.test(p)
}

/**
 * 路径是否在守卫面（pages/ai-assistant 下的页面文件，且不是测试）。
 * **ALLOWLIST 不在这里判**（#1123）：豁免由 runner 承载（`--all` 整体放行 / `--diff` 只放行基线
 * 违规行号）—— 判定面先吞掉例外文件的话 `scanSource` 恒返回空，行号级放行会静默失效。
 */
export function isGuardedPath(filePath) {
  const p = String(filePath).replace(/\\/g, '/')
  if (isTestFile(p)) return false
  return p.includes(GUARDED_PATH_SEGMENT)
}

/** 标识符级匹配：`resendMessage` / `streamChatter` 这类形近名不误报。 */
function tokenRe(token) {
  return new RegExp('(?<![\\w$])' + token + '(?![\\w$])')
}

/**
 * 扫一份源码，返回违规列表（1-based 行号 + 命中的名字 + 原文）。
 * **路径不在守卫面即整体放行**（测试、其它目录、白名单）——判定收敛在这里，
 * 调用方不必自己记得先过滤（少一个「忘了过滤就假红」的面）。
 */
export function scanSource(source, file) {
  if (!isGuardedPath(file)) return []
  const violations = []
  const lines = String(source).split('\n')
  for (let i = 0; i < lines.length; i++) {
    const line = lines[i]
    if (isCommentLine(line)) continue
    for (const token of GUARDED_TOKENS) {
      if (tokenRe(token).test(line)) {
        violations.push({ file, line: i + 1, token, text: line.trim() })
      }
    }
  }
  return violations
}

/**
 * 本守卫的声明：判定面 + 报告措辞（形状对齐 #1094 的 `scripts/lib/guard.mjs` 契约）。
 * runner（argv / 走查 / --diff / allowlist / 退出码）在 #1094 并入后由 guard.mjs 独占。
 */
export const GUARD_SPEC = {
  name: 'check-ai-assistant-send',
  usage: '用法: node scripts/check-ai-assistant-send.mjs --all [目录] | --diff [base]',
  cli: { noArgs: 'all', helpFlag: true, scanDirArg: true, usageOnUnknown: true, usageStream: 'stderr' },
  all: {
    scanDir: 'frontend/src',
    extensions: SCAN_EXTENSIONS,
    skipNodeModules: true,
    tolerateWalkErrors: true,
    stream: 'stdout',
    header: (ctx) => [
      '===== AI 助手发送编排守卫：全量扫描（pages/ai-assistant 不得再出现发送编排）=====',
      '扫描目录: ' + ctx.scanDirRel,
      '守卫面: ' + GUARDED_PATH_SEGMENT + '*（跳过测试与白名单）',
      '禁入名字: ' + GUARDED_TOKENS.join(' / ') + '（编排宿主 = stores/aiAssistant.ts 的 send/finalizeTurn）',
      '---'
    ],
    ok: (ctx) => '无违规。' + ctx.checked + ' 个页面文件均未出现发送编排（只经 store.send 单次委派）。',
    violation: (v) => v.file + ':' + v.line + ': ' + v.token + '  ' + v.text,
    footer: (ctx) => [
      '---',
      '共 ' + ctx.count + ' 处。发送编排与三种终态（done / error / aborted）在 store 的 send / finalizeTurn 里：',
      '  —— 页面只提交「正文 + 差异参数」，失败/中断读 store.lastTurnError 渲染重试（ADR-0056 §10）。'
    ]
  },
  diff: {
    pathspec: ['*.vue', '*.ts'],
    defaultBase: 'origin/master',
    stream: 'stderr',
    empty: (base) => '[check-ai-assistant-send] 相对 ' + base + ' 无 .vue/.ts 新增行，跳过。',
    ok: () => '[check-ai-assistant-send] 新增行未出现发送编排，通过。',
    header: () => ['===== 新增行出现 AI 助手发送编排（一律收进 store）====='],
    violation: (v) => v.file + ':' + v.line + ': ' + v.token + '  ' + v.text,
    footer: (ctx) => ['---', '共 ' + ctx.count + ' 处。见 ADR-0056 §10 与 docs/agents/ui-conventions.md。']
  },
  isGuardedPath,
  scanSource,
  allowlist: ALLOWLIST
}

if (isDirectRun(import.meta.url)) runGuardCli(GUARD_SPEC)
