#!/usr/bin/env node
/**
 * 列表四段式守卫（ADR-0053 §6 / issue #1054）。
 *
 * 「错误态 → 骨架 → 空态 → 内容」的渲染优先级唯一实现在 `components/ui/UiAsyncSection.vue`。
 * 本守卫拦的是**页面/业务组件重新手写四分支链**：同一文件里既出现错误态组件
 * （UiErrorState）又出现骨架或空态组件（UiSkeleton / UiEmptyState）——这正是
 * 「四态先后与互斥在页面里重新表述」的信号（收敛前 39 页各写一遍、顺序还不一致）。
 *
 * 用法：
 *   node scripts/check-async-section.mjs --all            全量扫描（CI 用）
 *   node scripts/check-async-section.mjs --diff [base]    只查相对 base 的新增行（本地用）
 *
 * 退出码：有违规 1，否则 0。
 */
import { readFileSync, readdirSync } from 'node:fs'
import { fileURLToPath, pathToFileURL } from 'node:url'
import path from 'node:path'

const __filename = fileURLToPath(import.meta.url)
const ROOT = path.resolve(path.dirname(__filename), '..')

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

/** 递归收集 .vue 文件。 */
function walk(dir, out = []) {
  for (const entry of readdirSync(dir, { withFileTypes: true })) {
    const full = path.join(dir, entry.name)
    if (entry.isDirectory()) walk(full, out)
    else if (entry.name.endsWith('.vue')) out.push(full)
  }
  return out
}

function runAll() {
  const files = walk(path.join(ROOT, 'frontend', 'src'))
  let count = 0
  for (const f of files) {
    const r = rel(f)
    const vs = scanSource(readFileSync(f, 'utf8'), f)
    if (vs.length && ALLOWLIST[r]) continue
    for (const v of vs) {
      count++
      console.error(`✗ ${r}:${v.line}\n  ${v.message}`)
    }
  }
  return count
}

function runDiff(parseAddedLines, base) {
  let added
  try {
    added = parseAddedLines(base, '*.vue')
  } catch (e) {
    console.error('[check-async-section] 解析新增行失败: ' + (e && e.message ? e.message : e))
    return 1
  }
  let count = 0
  for (const [file, lineSet] of added) {
    if (!isGuardedPath(file)) continue
    const r = rel(file)
    let source
    try {
      source = readFileSync(path.join(ROOT, file), 'utf8')
    } catch {
      continue
    }
    const vs = scanSource(source, file)
    for (const v of vs) {
      if (!lineSet.has(v.line)) continue
      if (ALLOWLIST[r]) continue
      count++
      console.error(`✗ ${r}:${v.line}\n  ${v.message}`)
    }
  }
  return count
}

async function main() {
  const argv = process.argv.slice(2)
  if (argv.length === 0 || argv[0] === '--help') {
    console.log('用法: node scripts/check-async-section.mjs --all | --diff [base]')
    process.exit(0)
  }
  let count
  if (argv[0] === '--all') {
    count = runAll()
  } else if (argv[0] === '--diff') {
    const { parseAddedLines } = await import('./lib/added-lines.mjs')
    count = runDiff(parseAddedLines, argv[1] || 'origin/master')
  } else {
    console.error('未知参数: ' + argv[0])
    process.exit(2)
  }
  if (count > 0) {
    console.error(`\n共 ${count} 处违规。`)
    process.exit(1)
  }
  console.log('✓ 未发现手写四分支链')
}


const isDirectRun = process.argv[1] && pathToFileURL(process.argv[1]).href === import.meta.url
if (isDirectRun) {
  main().catch((e) => {
    console.error(e)
    process.exit(2)
  })
}
