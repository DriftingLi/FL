// 目录排序串第二源守卫的抽取式单测（ADR-0060 决策 10 / 第十三波票 10）。
//
// 守卫是「规则的执行面」，它自己坏了必须报红 —— 否则规则退化为假绿。
// 运行：node --test scripts/check-catalog-sort.test.mjs
//
// 判定面只认「目录面（文件名含 catalog 的非测试 .go）里 .Order( 的实参出现了与 spec 声明逐字同串的
// 字符串字面量」这一条，这组用例把这些边界（含三条 ADR 明记「不动」的存量行）钉住。
import { test } from 'node:test'
import assert from 'node:assert/strict'
import { mkdtempSync, readFileSync, rmSync, writeFileSync } from 'node:fs'
import { tmpdir } from 'node:os'
import { dirname, join, resolve } from 'node:path'
import { fileURLToPath } from 'node:url'
import { spawnSync } from 'node:child_process'
import {
  ALLOWLIST,
  GUARDED_PATH_SEGMENT,
  GUARD_SPEC,
  SPEC_ORDER_BY,
  SPEC_ORDER_BY_DECLARATIONS,
  declaredOrderByLiterals,
  isGuardedPath,
  isTestFile,
  scanSource,
  stringLiterals
} from './check-catalog-sort.mjs'

const ROOT = resolve(dirname(fileURLToPath(import.meta.url)), '..')
const READ_PATH = 'backend/internal/service/training_catalog_service.go'
const SPECS = 'backend/internal/service/catalog_specs.go'

/** 收口后的读面形态（票 10 (a)）：引用 spec 声明，不重抄字面量。 */
const FIXED_SRC = [
  'func (s *TrainingCatalogService) getCatalogTree(activeOnly, withChapters bool, cred *int) *CatalogTreeDTO {',
  '\tq.Order(specialtyCatalogSpec().OrderBy).Find(&specialties)',
  '\tq.Order(levelCatalogSpec().OrderBy).Find(&levels)',
  '\tq.Order("course.sort_order ASC, course.course_id ASC").Find(&rows)',
  '\ts.db.Order("order_num ASC, chapter_id ASC").Find(&chapters)',
  '\tq.Order("sort_order ASC, " + idCol + " ASC")',
  '}'
].join('\n')

test('正例：收口前的两行逐字同串就是本守卫的靶子，行号精确', () => {
  const src = [
    '\tq := s.db.Model(&model.Specialty{})',
    '\tq.Order("sort_order ASC, specialty_id ASC").Find(&specialties)',
    '\tq.Order("sort_order ASC, level_id ASC").Find(&levels)'
  ].join('\n')
  const violations = scanSource(src, READ_PATH)
  assert.deepEqual(
    violations.map((v) => v.line),
    [2, 3]
  )
  assert.equal(violations[0].orderBy, 'sort_order ASC, specialty_id ASC')
  assert.match(violations[0].entity, /专业方向 specialty/)
  assert.match(violations[0].entity, /catalog_specs\.go/)
  assert.match(violations[1].entity, /课程等级 course_level/)
})

test('正例：spec 声明表里每一条串被重抄都报（含「id ASC」这类短串），大小写与多余空格不误伤', () => {
  const cases = [
    ['q.Order("sort_order ASC, id ASC")', 'sort_order ASC, id ASC'],
    ['q.Order("id ASC").Find(&rows)', 'id ASC'],
    ['q.Order("sort_order ASC, position_id ASC")', 'sort_order ASC, position_id ASC'],
    ['q.Order(  "sort_order ASC, level_id ASC"  )', 'sort_order ASC, level_id ASC'],
    ['q.Order(`sort_order ASC, specialty_id ASC`)', 'sort_order ASC, specialty_id ASC']
  ]
  for (const [line, orderBy] of cases) {
    const v = scanSource('\t' + line, READ_PATH)
    assert.equal(v.length, 1, line + ' 应报一处')
    assert.equal(v[0].orderBy, orderBy)
  }
})

test('负例：收口后的读面逐行不报（引用 spec 声明 = 单一宿主）', () => {
  assert.deepEqual(scanSource(FIXED_SRC, READ_PATH), [])
})

test('负例：ADR-0060 决策 10 明记「不动」的三类形态不进射程', () => {
  // ① 带表别名的课程行与 spec 不同源；② 章节行的列未被任何 spec 声明；
  // ③ renumberSortGroup 是按 ID 列参数化派生的串，没有任何一份字面量等于 spec 声明。
  const cases = [
    'q.Order("course.sort_order ASC, course.course_id ASC").Find(&rows)',
    's.db.Order("order_num ASC, chapter_id ASC").Find(&chapters)',
    'q.Order("sort_order ASC, " + idCol + " ASC")',
    'q.Order("created_at DESC")'
  ]
  for (const line of cases) {
    assert.deepEqual(scanSource('\t' + line, READ_PATH), [], line + ' 不该报')
  }
})

test('负例：spec 声明自身（OrderBy 字段）是合法宿主，不报', () => {
  const src = [
    '\treturn CatalogEntitySpec[model.Specialty, SpecialtyInput, SpecialtyDict]{',
    '\t\tTable:       "specialty",',
    '\t\tOrderBy:     "sort_order ASC, specialty_id ASC",',
    '\t}'
  ].join('\n')
  assert.deepEqual(scanSource(src, SPECS), [])
  // engine 消费声明（q.Order(spec.OrderBy)）同样不报
  assert.deepEqual(scanSource('\tq.Order(spec.OrderBy).Find(&rows)', 'backend/internal/service/catalog_engine.go'), [])
})

test('负例：整行注释与文档说明不是调用', () => {
  const src = [
    '// 此前这里逐字抄了 "sort_order ASC, specialty_id ASC"（ADR-0060 决策 10）',
    '/* q.Order("sort_order ASC, level_id ASC") */',
    '\t * q.Order("id ASC")'
  ].join('\n')
  assert.deepEqual(scanSource(src, READ_PATH), [])
})

test('负例：非目录面文件与 Go 测试文件整体不进判定面', () => {
  const line = 'q.Order("sort_order ASC, specialty_id ASC")'
  for (const p of [
    'backend/internal/service/faq_service.go',
    'backend/internal/service/course_service.go',
    'backend/internal/api/admin.go',
    'backend/internal/service/catalog_tree_shape_test.go',
    'backend/internal/api/training_catalog_contract_test.go'
  ]) {
    assert.equal(isGuardedPath(p), false, p + ' 不在守卫面')
    assert.deepEqual(scanSource(line, p), [], p + ' 不进判定面')
  }
  for (const p of [
    'backend/internal/service/training_catalog_service.go',
    'backend/internal/service/training_catalog_types.go',
    'backend/internal/service/catalog_specs.go',
    'backend/internal/service/catalog_engine.go',
    'backend/internal/service/position_catalog.go',
    'backend/internal/api/training_catalog.go'
  ]) {
    assert.equal(isGuardedPath(p), true, p + ' 应在守卫面')
  }
  assert.equal(isTestFile('backend/internal/service/catalog_sort_test.go'), true)
  assert.equal(GUARDED_PATH_SEGMENT, 'catalog')
})

test('一致性锁：本脚本的声明表与 spec 文件的真实 OrderBy 互等（判据不许改回双份）', () => {
  const declared = new Set()
  for (const rel of ['backend/internal/service/catalog_specs.go', 'backend/internal/service/position_catalog.go']) {
    for (const lit of declaredOrderByLiterals(readFileSync(join(ROOT, rel), 'utf8'))) declared.add(lit)
  }
  const mine = new Set(SPEC_ORDER_BY_DECLARATIONS.map((d) => d.orderBy))
  // 真实声明里有、表里没有 → 守卫漏判（新串的第二源抓不到）
  for (const d of declared) assert.ok(mine.has(d), '声明表缺少 spec 串: ' + d)
  // 表里有、真实声明里没有 → 死条目（规则凭空宽/严）
  for (const m of mine) assert.ok(declared.has(m), '声明表有死条目: ' + m)
})

test('白名单：当前为空表（规则绝对执行）；例外文件必须仍在判定面内且理由非空', () => {
  // #1123：豁免从 isGuardedPath 移交给 runner（--all 整体放行 / --diff 只放行基线违规行号）。
  assert.deepEqual(ALLOWLIST, {}, '本守卫的存量是 0，加例外要先想清楚：--all 下例外是整文件放行')
  for (const [p, reason] of Object.entries(ALLOWLIST)) {
    assert.equal(isGuardedPath(p), true, p + ' 必须在判定面内（豁免交给 runner 逐行判）')
    assert.ok(scanSource('q.Order("sort_order ASC, id ASC")', p).length === 1, p + ' 的判定面必须真的扫得动')
    assert.ok(typeof reason === 'string' && reason.length > 10, p + ' 的理由要写清')
  }
})

test('判定面契约：GUARD_SPEC 用的就是本文件导出的判定函数（#1094 runner 可直接收）', () => {
  assert.equal(GUARD_SPEC.isGuardedPath, isGuardedPath)
  assert.equal(GUARD_SPEC.scanSource, scanSource)
  assert.equal(GUARD_SPEC.allowlist, ALLOWLIST)
  assert.equal(GUARD_SPEC.name, 'check-catalog-sort')
  assert.deepEqual(GUARD_SPEC.all.extensions, ['.go'])
  assert.equal(SPEC_ORDER_BY.get('id ASC').includes('证书模板'), true)
  assert.deepEqual(stringLiterals('q.Order("a ASC", "b")'), ['a ASC', 'b'])
  assert.equal(typeof GUARD_SPEC.all.ok, 'function')
  assert.equal(typeof GUARD_SPEC.diff.ok, 'function')
})

test('端到端：守卫在当前工作树上判绿（读面已改引用 spec 声明）', () => {
  const out = spawnSync(process.execPath, ['scripts/check-catalog-sort.mjs', '--all'], {
    cwd: ROOT,
    encoding: 'utf8'
  })
  assert.equal(out.status, 0, out.stdout + out.stderr)
  assert.match(out.stdout, /无违规/)
  // 守卫面非空：守卫坏了（例如把扫描目录或文件名判据写错）时不能靠「一个文件都没扫到」判绿
  assert.match(out.stdout, /[1-9][0-9]* 个目录面文件均未出现/)
})

test('端到端：合成违规目录必须报红（守卫真的扫得动，不是恒绿）', () => {
  const dir = mkdtempSync(join(tmpdir(), 'check-catalog-sort-'))
  try {
    writeFileSync(
      join(dir, 'catalog_stub.go'),
      'package service\n\nfunc f() {\n\tq.Order("sort_order ASC, specialty_id ASC").Find(&rows)\n}\n',
      'utf8'
    )
    const out = spawnSync(process.execPath, ['scripts/check-catalog-sort.mjs', '--all', dir], {
      cwd: ROOT,
      encoding: 'utf8'
    })
    assert.equal(out.status, 1, '有违规必须非零退出：' + out.stdout + out.stderr)
    assert.match(out.stdout, /catalog_stub\.go:4: 排序串 "sort_order ASC, specialty_id ASC"/)
    assert.match(out.stdout, /共 1 处/)
  } finally {
    rmSync(dir, { recursive: true, force: true })
  }
})

test('端到端：--diff fail-closed —— base 解析不了即非零退出，绝不落「通过」', () => {
  const out = spawnSync(
    process.execPath,
    ['scripts/check-catalog-sort.mjs', '--diff', 'no-such-ref-for-guard-selftest'],
    { cwd: ROOT, encoding: 'utf8' }
  )
  assert.equal(out.status, 2)
  assert.match(out.stderr, /无法解析 base ref/)
})
