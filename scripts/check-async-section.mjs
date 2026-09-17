#!/usr/bin/env node
/**
 * 列表四段式守卫（ADR-0053 §6 / issue #1054）。
 *
 * 「错误态 → 骨架 → 空态 → 内容」的渲染优先级唯一实现在 `components/ui/UiAsyncSection.vue`。
 * 本守卫拦的是**页面/业务组件重新手写四分支链**：同一文件里既出现错误态组件
 * （UiErrorState）又出现骨架或空态组件（UiSkeleton / UiEmptyState）——这正是
 * 「四态先后与互斥在页面里重新表述」的信号（收敛前 39 页各写一遍、顺序还不一致）。
 *
 * 用法（runner 面单点在 `scripts/lib/guard.mjs`，ADR-0056 §5 / #1094；本文件只有判定面）：
 *   node scripts/check-async-section.mjs --all            全量扫描（CI 用）
 *   node scripts/check-async-section.mjs --diff [base]    只查相对 base 的新增行（base 默认 origin/master）
 *
 * 退出码：有违规 1，否则 0；判据坏了（base 取不到 / git diff 失败）2。
 */
import path from 'node:path'
import { fileURLToPath } from 'node:url'
import { isDirectRun, runGuardCli } from './lib/guard.mjs'

const ROOT = path.resolve(path.dirname(fileURLToPath(import.meta.url)), '..')

/**
 * 逐条登记的有意例外（相对路径 → 理由）。全部是 #1054 的**存量待迁移**页面——
 * 它们在守卫上线前就手写了四分支链，迁移按批次推进、迁一个销一个号；
 * 新代码不允许再往这里加（新增违规直接报红）。
 * @type {Record<string, string>}
 */
export const ALLOWLIST = {
  'frontend/src/components/student/ChapterDiscussion.vue': '#1054 存量：章节讨论区局部列表，待迁移',
  'frontend/src/pages/admin/ForumManage.vue': '#1054 存量：管理端列表页，待迁移',
  'frontend/src/pages/admin/Inspection.vue': '#1054 存量：巡检页多 tab 结构，待迁移',
  'frontend/src/pages/recruit/Resumes.vue': '#1054 存量：简历库列表页，待迁移',
  'frontend/src/pages/student/ChapterView.vue': '#1054 存量：章节学习页（挂在 v-loading 链上），待迁移',
  'frontend/src/pages/student/CourseList.vue': '#1054 存量：课程列表页（骨架优先于错误的旧顺序），待迁移',
  'frontend/src/pages/student/Dashboard.vue': '#1054 存量：学员工作台，待迁移',
  'frontend/src/pages/student/ForumDetail.vue': '#1054 存量：帖子详情回复区，待迁移',
  'frontend/src/pages/student/JobPlaza.vue': '#1054 存量：岗位广场，待迁移',
  'frontend/src/pages/student/RealExamPapers.vue': '#1054 存量：真题卷列表，待迁移',
  'frontend/src/pages/student/ResumePage.vue': '#1054 存量：我的简历，待迁移',
  'frontend/src/pages/student/SearchPage.vue': '#1054 存量：搜索页多区结构，待迁移',
  'frontend/src/pages/tutor/Dashboard.vue': '#1054 存量：导师工作台，待迁移',
  'frontend/src/pages/tutor/QuestionCreate.vue': '#1054 存量：出题页题库预览区，待迁移',
  'frontend/src/pages/tutor/TutorChapterEdit.vue': '#1054 存量：章节编辑页资料区，待迁移'
}

/** 归一化为 POSIX 相对路径。 */
function rel(file) {
  const p = path.isAbsolute(file) ? path.relative(ROOT, file) : file
  return p.split(path.sep).join('/')
}

/** 路径不在守卫面即整体放行（ui 封装层自身合法组合三件套）。 */
export function isGuardedPath(file) {
  const r = rel(file)
  if (!r.startsWith('frontend/src/')) return false
  if (!r.endsWith('.vue')) return false
  if (r.startsWith('frontend/src/components/ui/')) return false
  return true
}

/**
 * 扫一份源码，返回违规说明（1-based 行号 + 命中组件 + 原文）。
 * 规则：UiErrorState 与（UiSkeleton 或 UiEmptyState）在同一文件出现 = 手写四分支链。
 * （只 import 不使用的情况罕见且无害——按文本信号判定，与既有守卫同口径。）
 */
export function scanSource(source, file) {
  if (!isGuardedPath(file)) return []
  const violations = []
  const lines = String(source).split('\n')
  const hits = { error: null, skeleton: null, empty: null }
  for (let i = 0; i < lines.length; i++) {
    const line = lines[i]
    if (hits.error === null && /<UiErrorState\b/.test(line)) hits.error = { line: i + 1, lineText: line.trim() }
    if (hits.skeleton === null && /<UiSkeleton\b/.test(line)) hits.skeleton = { line: i + 1, lineText: line.trim() }
    if (hits.empty === null && /<UiEmptyState\b/.test(line)) hits.empty = { line: i + 1, lineText: line.trim() }
  }
  if (hits.error && (hits.skeleton || hits.empty)) {
    const partner = hits.skeleton ? 'UiSkeleton' : 'UiEmptyState'
    const at = hits.skeleton ?? hits.empty
    violations.push({
      line: at.line,
      message: `手写四分支链：UiErrorState(第 ${hits.error.line} 行) 与 ${partner}(第 ${at.line} 行) 同文件编排 —— 改用 components/ui/UiAsyncSection.vue（渲染优先级唯一实现，#1054）`
    })
  }
  return violations
}

/**
 * 本守卫的声明：判定面 + 报告措辞。runner（argv / 走查 / --diff / allowlist / 退出码）
 * 在 scripts/lib/guard.mjs —— 新增守卫只需实现 scanSource 并声明这一份。
 * 注：全量与增量都把 ALLOWLIST 当整体放行（存量页面迁移前不报），语义与收敛前一致。
 */
export const GUARD_SPEC = {
  name: 'check-async-section',
  usage: '用法: node scripts/check-async-section.mjs --all | --diff [base]',
  cli: { noArgs: 'usage', helpFlag: true, scanDirArg: false, usageOnUnknown: false, usageStream: 'stdout' },
  all: {
    scanDir: 'frontend/src',
    extensions: ['.vue'],
    skipNodeModules: false,
    tolerateWalkErrors: false,
    stream: 'stderr',
    ok: () => '✓ 未发现手写四分支链',
    violation: (v) => '✗ ' + v.file + ':' + v.line + '\n  ' + v.message,
    footer: (ctx) => ['', '共 ' + ctx.count + ' 处违规。']
  },
  diff: {
    pathspec: ['*.vue'],
    defaultBase: 'origin/master',
    stream: 'stderr',
    empty: (base) => '[check-async-section] 相对 ' + base + ' 无 .vue 新增行，跳过。',
    ok: () => '[check-async-section] 新增行未手写四分支链，通过。',
    header: () => ['===== 新增行手写了四分支链（业务页面一律走 components/ui/UiAsyncSection.vue）====='],
    violation: (v) => '✗ ' + v.file + ':' + v.line + '\n  ' + v.message,
    footer: (ctx) => ['---', '共 ' + ctx.count + ' 处违规。']
  },
  isGuardedPath,
  scanSource,
  allowlist: ALLOWLIST
}

if (isDirectRun(import.meta.url)) runGuardCli(GUARD_SPEC)
