// 注释三态守卫的判定逻辑自检（issue #1445 P4；形态对齐 check-catalog-sort.test.mjs）。
import { test } from 'node:test'
import assert from 'node:assert/strict'
import { runGuard } from './lib/guard.mjs'
import {
  GUARD_SPEC,
  commentBody,
  isDecorativeSeparator,
  packageClause,
  packageDocName,
  scanSource
} from './check-comment-cleanliness.mjs'

/** 造一段合成 diff（-U0 hunk：从 startLine 起新增 lines）。 */
function synthDiff(file, startLine, lines) {
  return [
    'diff --git a/' + file + ' b/' + file,
    '--- a/' + file,
    '+++ b/' + file,
    '@@ -1,0 +' + startLine + ',' + lines.length + ' @@',
    ...lines.map((l) => '+' + l)
  ].join('\n')
}

/** 端到端驱动一次 --diff：合成 diff + 假源码。 */
function probeDiff(diff, sources) {
  const out = []
  const err = []
  const code = runGuard(GUARD_SPEC, {
    argv: ['--diff', 'origin/master'],
    root: '/p4-probe',
    stdout: (line) => out.push(line),
    stderr: (line) => err.push(line),
    resolveBase: () => true,
    readDiff: () => diff,
    readSource: (file) => {
      if (Object.prototype.hasOwnProperty.call(sources, file)) return sources[file]
      const e = new Error('ENOENT: ' + file)
      e.code = 'ENOENT'
      throw e
    }
  })
  return { code, stdout: out.join('\n'), stderr: err.join('\n') }
}

// ===== 判定面：三类欠账 =====

test('正例：文件头包文档与 package 子句不符（P3 更名漏网的形态）', () => {
  const src = [
    '// Package service 实现业务服务层。',
    '// 本文件：口令写面的唯一动作。',
    '',
    'package core',
    ''
  ].join('\n')
  const v = scanSource(src, BE);
  assert.equal(v.length, 1)
  assert.equal(v[0].kind, 'package-doc-mismatch')
  assert.equal(v[0].line, 1)
})

test('负例：包文档与 package 子句一致 → 不报', () => {
  const src = ["// Package core 承载跨域共享的业务助手。", "", "package core", ""].join('\n')
  assert.equal(packageDocName(src.split('\n')[0]), 'core')
  assert.equal(packageClause(src), 'core')
  assert.deepEqual(scanSource(src, BE), [])
})

test('负例：注释里提到 Package 但不是文件头那条 → 不报', () => {
  const src = [
    '// 本文件说明：',
    '// Package service 的旧形态见 ADR-0070。',
    '',
    'package core',
    ''
  ].join('\n')
  assert.deepEqual(scanSource(src, BE), [])
})

test('正例：装饰分隔线报红，夹在中间的标题行不报', () => {
  const src = [
    '// ==============================',
    '// 测试替身：内存验证码存储 + 测试通道',
    '// ==============================',
    '',
    'package api'
  ].join('\n')
  const v = scanSource(src, BE);
  assert.deepEqual(v.map((x) => x.line), [1, 3])
  for (const item of v) assert.equal(item.kind, 'decorative-separator')
})

test('负例：行尾的等号串与短横线不算分隔线', () => {
  assert.equal(isDecorativeSeparator('// 见 foo ========'), false)
  assert.equal(isDecorativeSeparator('// -------'), false)
  assert.equal(isDecorativeSeparator('// ========='), true)
  assert.equal(commentBody('//   好    '), '好')
})

test('正例：相邻重复注释行只报第二行', () => {
  const src = [
    '// HandleReport 处理举报 PUT /api/admin/forum/reports/:id',
    '// HandleReport 处理举报 PUT /api/admin/forum/reports/:id',
    'package forum'
  ].join('\n')
  const v = scanSource(src, BE);
  assert.equal(v.length, 1)
  assert.equal(v[0].kind, 'duplicate-comment')
  assert.equal(v[0].line, 2)
})

test('负例：相邻重复的代码行（非注释）不报', () => {
  const src = ['package x', '', 'if a {', 'if a {', ''].join('\n')
  assert.deepEqual(scanSource(src, BE), [])
})

// ===== 射程与棘轮口径 =====

test('守住射程：backend 下的 .go 进面，其余路径不进', () => {
  assert.equal(GUARD_SPEC.isGuardedPath('backend/internal/core/mailer.go'), true)
  assert.equal(GUARD_SPEC.isGuardedPath('backend/cmd/server/main.go'), true)
  assert.equal(GUARD_SPEC.isGuardedPath('frontend/src/a.ts'), false)
  assert.equal(GUARD_SPEC.isGuardedPath('training-app/x/y.uts'), false)
})

test('基线为 0：allowlist 为空（只减不增；确有例外须逐条登记并写明理由）', () => {
  assert.deepEqual(Object.keys(GUARD_SPEC.allowlist), [])
})

test('防空转下界已登记，且低于当前实测射程', () => {
  assert.equal(typeof GUARD_SPEC.all.minChecked, 'number')
  assert.ok(GUARD_SPEC.all.minChecked >= 500, '下界取得太低会放过「射程塌了」')
})

// ===== 端到端：合成违规必须判红、干净 diff 必须判绿（防恒绿 / 防恒红）=====

const BE = 'backend/internal/core/password_write.go'

test('合成违规必须判红（包文档失真）', () => {
  const added = ['// Package service 实现业务服务层。', '', 'package core']
  const diff = synthDiff(BE, 1, added)
  const sources = {}
  sources[BE] = added.join('\n')
  const res = probeDiff(diff, sources)
  assert.equal(res.code, 1)
  assert.match(res.stderr, /package-doc-mismatch/)
})

test('合成违规必须判红（分隔线）', () => {
  const added = ['// ==============================', '// 新区段', '// ==============================']
  const diff = synthDiff(BE, 3, added)
  const sources = {}
  sources[BE] = ['package core', '', ...added].join('\n')
  const res = probeDiff(diff, sources)
  assert.equal(res.code, 1)
  assert.match(res.stderr, /decorative-separator/)
})

test('干净 diff 必须判绿（防恒红）', () => {
  const added = ['// 本文件：口令写面。', 'package core']
  const diff = synthDiff(BE, 1, added)
  const sources = {}
  sources[BE] = added.join('\n')
  const res = probeDiff(diff, sources)
  assert.equal(res.code, 0)
  assert.match(res.stdout, /无违规/)
})
