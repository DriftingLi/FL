#!/usr/bin/env node
/**
 * audit-ui.mjs 的可执行单测（#2864；契约 #2836 §9）
 *
 * 六条判据，逐条对应契约 §9.1–§9.6：
 *   ① 判据表合规（id 唯一、seq 1–10 无重、coverage 四值、suites 路径存在、requiredSources 合法）
 *   ② 26 件扫描面全部被至少一项引用（并集闭合）
 *   ③ 两栏判定是纯函数（coverage → status，未知值抛错）
 *   ④ reasonCode 互斥（未扫描行必填且不含 ruled-no-gate；已扫描行恒 null）
 *   ⑤ fail-closed（空 stdout / 非法 JSON / 零套件 / 来源崩 ⇒ run-failed 且 exit 2）
 *   ⑥ golden：同一棵树两次 --json 逐字节相同，且不含绝对路径与时间戳
 *
 * 运行：node scripts/audit-ui.test.mjs
 * 登记：.github/workflows/ci.yml 的 mobile-test job，unit 腿第 1 片（与其它 scripts/*.test.mjs 同款逐条点名）
 *
 * 为什么不用 node --test：本仓 scripts/ 下的自测统一用「自定义 test() + process.exitCode」，
 * 直接 node 与 node --test 都能跑，且失败时能一次打全六条（node --test 首条失败即中断）。
 */
import assert from 'node:assert/strict'
import { execFileSync } from 'node:child_process'
import { dirname, join, resolve } from 'node:path'
import { fileURLToPath } from 'node:url'

import {
  buildReport,
  exitCodeFor,
  loadItems,
  parseJestJson,
  parseSpacingJson,
  reasonCodeForCoverage,
  statusForCoverage,
} from './audit-ui.mjs'

const APP_ROOT = resolve(dirname(fileURLToPath(import.meta.url)), '..')
const SCRIPT = join(APP_ROOT, 'scripts', 'audit-ui.mjs')

/** 契约 §5 的 26 件扫描面（#2865 追加 sharedComponentUiContract；顺序无关，比对用集合）。 */
const SCAN_FACE = [
  'utils/appNavBarCapsuleToolbarContract.test.js',
  'utils/appNavBarLeftEntryContract.test.js',
  'utils/batch2UiContract.test.js',
  'utils/batch4UiContract.test.js',
  'utils/batch5UiContract.test.js',
  'utils/batch6UiContract.test.js',
  'utils/dashboardPage.test.js',
  'utils/forumMarkdownInputContract.test.js',
  'utils/gradientSyntaxContract.test.js',
  'utils/iconGlyphContract.test.js',
  'utils/imageWebpAttrContract.test.js',
  'utils/learnerUiContract.test.js',
  'utils/legacyPrimaryColorRatchetContract.test.js',
  'utils/loginContract.test.js',
  'utils/markdownContract.test.js',
  'utils/markdownTypographyContract.test.js',
  'utils/pointsGoodsDetailContract.test.js',
  'utils/recruiterUiContract.test.js',
  'utils/resourcesContract.test.js',
  'utils/resumeJobsBatch8Contract.test.js',
  'utils/safeAreaContract.test.js',
  'utils/searchContract.test.js',
  'utils/sharedComponentUiContract.test.js',
  'utils/touchTargetContract.test.js',
  'utils/uiDocDocMapContract.test.js',
  'utils/uvueFontCarrierContract.test.js',
]

let passed = 0
const failures = []

function test(name, fn) {
  try {
    fn()
    passed += 1
    console.log('ok   ' + name)
  } catch (e) {
    failures.push({ name, error: e })
    console.log('FAIL ' + name)
    console.log('     ' + String((e && e.message) || e).split('\n').join('\n     '))
  }
}

/** 造一份「两个来源都正常」的假读数，供纯函数判据使用。 */
function okSources(items) {
  const suites = new Map()
  for (const it of items) {
    for (const s of it.suites) suites.set(s, { assertions: 3, failed: 0, execError: false })
  }
  return {
    jest: { ok: true, suites },
    spacing: { ok: true, scanned: 121, warn: 0, info: 0, skip: 204 },
  }
}

// ① 判据表合规
test('① 判据表合规：10 项、id 唯一、seq 1–10 无重、coverage 四值、套件路径存在', () => {
  const items = loadItems()
  assert.equal(items.length, 10)
  assert.deepEqual(items.map((i) => i.seq).sort((a, b) => a - b), [1, 2, 3, 4, 5, 6, 7, 8, 9, 10])
  assert.equal(new Set(items.map((i) => i.id)).size, 10)
  for (const it of items) {
    assert.ok(['full', 'narrow', 'point', 'none'].includes(it.coverage), it.id + ' coverage 越界')
    assert.ok(it.requiredSources.length > 0, it.id + ' requiredSources 为空')
    for (const s of it.requiredSources) assert.ok(['jest', 'spacing'].includes(s), it.id + ' requiredSources 越界')
  }
})

// ② 扫描面并集闭合
test('② 26 件扫描面全部被至少一项引用', () => {
  const items = loadItems()
  const referenced = new Set(items.flatMap((i) => i.suites))
  const missing = SCAN_FACE.filter((s) => !referenced.has(s))
  const extra = [...referenced].filter((s) => !SCAN_FACE.includes(s))
  assert.deepEqual(missing, [], '未被任何项引用的套件：' + missing.join(', '))
  assert.deepEqual(extra, [], '引用了扫描面之外的套件：' + extra.join(', '))
})

// ③ 两栏判定纯函数
test('③ 两栏判定：full/narrow→scanned，point/none→not-scanned，未知值抛错', () => {
  assert.equal(statusForCoverage('full'), 'scanned')
  assert.equal(statusForCoverage('narrow'), 'scanned')
  assert.equal(statusForCoverage('point'), 'not-scanned')
  assert.equal(statusForCoverage('none'), 'not-scanned')
  assert.throws(() => statusForCoverage('partial'), /未知 coverage/)
})

// ④ reasonCode 互斥
test('④ reasonCode 互斥：未扫描行必填且不含 ruled-no-gate，已扫描行恒 null', () => {
  const items = loadItems()
  const report = buildReport({ items, ...okSources(items) })
  assert.equal(report.counts.scanned, 5)
  assert.equal(report.counts.notScanned, 5)
  for (const row of report.items) {
    if (row.status === 'scanned') {
      assert.equal(row.reasonCode, null, row.id + ' 已扫描行 reasonCode 必须为 null')
      assert.ok(row.note === null || row.note === 'ruled-no-gate', row.id + ' note 越界')
      assert.equal(typeof row.evidence.assertions, 'number', row.id + ' 已扫描行 assertions 必须是数字')
      assert.equal(typeof row.evidence.failed, 'number', row.id + ' 已扫描行 failed 必须是数字')
    } else {
      assert.ok(['no-criteria', 'point-only', 'run-failed'].includes(row.reasonCode), row.id + ' reasonCode 越界')
      assert.notEqual(row.reasonCode, 'ruled-no-gate', row.id + ' ruled-no-gate 不是未扫描原因码')
      assert.equal(row.note, null, row.id + ' 未扫描行 note 必须为 null')
      assert.equal(row.evidence.assertions, null, row.id + ' 未扫描行 assertions 必须为 null')
      assert.equal(row.evidence.failed, null, row.id + ' 未扫描行 failed 必须为 null')
      assert.ok(typeof row.reason === 'string' && row.reason.length > 0, row.id + ' 未扫描行必须有 reason')
    }
  }
  assert.equal(reasonCodeForCoverage('point'), 'point-only')
  assert.equal(reasonCodeForCoverage('none'), 'no-criteria')
  assert.equal(reasonCodeForCoverage('full'), null)
})

// ⑤ fail-closed
test('⑤ fail-closed：空 stdout / 非法 JSON / 零套件 / 来源崩 ⇒ run-failed 且 exit 2', () => {
  assert.equal(parseJestJson('').ok, false)
  assert.equal(parseJestJson('not json').ok, false)
  assert.equal(parseJestJson(JSON.stringify({ testResults: [] })).ok, false)
  assert.equal(parseSpacingJson('').ok, false)
  assert.equal(parseSpacingJson('{"scanned":1}').ok, false)

  const items = loadItems()

  // jest 整体崩：依赖 jest 的已扫描项（3/8/9）降 run-failed；只靠 spacing 的项（1/5）保持 scanned 但断言数填 null
  const jestDown = buildReport({ items, jest: { ok: false, reason: 'jest stdout 为空' }, spacing: okSources(items).spacing })
  assert.equal(exitCodeFor(jestDown), 2)
  for (const id of ['color-token', 'nav-bar-position', 'title-center']) {
    const row = jestDown.items.find((i) => i.id === id)
    assert.equal(row.status, 'not-scanned', id + ' 应降为 not-scanned')
    assert.equal(row.reasonCode, 'run-failed', id + ' 应为 run-failed')
    assert.equal(row.evidence.assertions, null, id + ' 断言数必须为 null（不许填 0）')
  }
  for (const id of ['page-margin', 'card-padding']) {
    const row = jestDown.items.find((i) => i.id === id)
    assert.equal(row.status, 'scanned', id + ' 靠 spacing 读数仍应 scanned')
    assert.equal(row.evidence.assertions, null, id + ' jest 不可用时断言数必须为 null 而非 0')
    assert.equal(row.readings.length, 1, id + ' spacing 读数应保留')
  }

  // spacing 崩：依赖它的项（1/5）降 run-failed
  const spacingDown = buildReport({ items, jest: okSources(items).jest, spacing: { ok: false, reason: 'audit-spacing exit 2' } })
  assert.equal(exitCodeFor(spacingDown), 2)
  for (const id of ['page-margin', 'card-padding']) {
    const row = spacingDown.items.find((i) => i.id === id)
    assert.equal(row.status, 'not-scanned', id + ' 应降为 not-scanned')
    assert.equal(row.reasonCode, 'run-failed', id + ' 应为 run-failed')
    assert.equal(row.readings.length, 0, id + ' 读数必须清空')
  }

  // 断言失败不改 status，默认 exit 0，--strict 才 exit 1
  const failing = okSources(items)
  failing.jest.suites.set('utils/batch2UiContract.test.js', { assertions: 17, failed: 2, execError: false })
  const red = buildReport({ items, ...failing })
  const redRow = red.items.find((i) => i.id === 'page-margin')
  assert.equal(redRow.status, 'scanned', '断言失败不改 status')
  assert.equal(redRow.evidence.failed, 2)
  assert.equal(exitCodeFor(red, false), 0, '默认 exit 0')
  assert.equal(exitCodeFor(red, true), 1, '--strict 下 exit 1')

  // 未知参数直接 exit 2（在起子进程之前就返回）
  let code = 0
  try {
    execFileSync(process.execPath, [SCRIPT, '--bogus'], { cwd: APP_ROOT, encoding: 'utf8', stdio: 'pipe' })
  } catch (e) {
    code = e.status
  }
  assert.equal(code, 2, '未知参数应 exit 2')
})

// ⑥ golden
test('⑥ golden：两次 --json 逐字节相同，且不含绝对路径与时间戳', () => {
  const run = () =>
    execFileSync(process.execPath, [SCRIPT, '--json'], {
      cwd: APP_ROOT,
      encoding: 'utf8',
      maxBuffer: 64 * 1024 * 1024,
    })
  const first = run()
  const second = run()
  assert.equal(first, second, '两次 --json 输出必须逐字节相同')

  const report = JSON.parse(first)
  assert.equal(report.schemaVersion, 1)
  assert.equal(report.items.length, 10)
  assert.deepEqual(report.counts, { scanned: 5, notScanned: 5 })
  assert.deepEqual(report.items.map((i) => i.seq), [1, 2, 3, 4, 5, 6, 7, 8, 9, 10])

  assert.ok(!/[A-Za-z]:[\\/]/.test(first), '--json 里不许出现绝对路径')
  for (const banned of ['generatedAt', 'startTime', 'endTime', 'duration', 'hostname']) {
    assert.ok(!first.includes(banned), '--json 里不许出现 ' + banned)
  }
  for (const row of report.items) {
    assert.deepEqual(row.evidence.suites, row.evidence.suites.slice().sort(), row.id + ' suites 必须字典序')
  }
})

if (failures.length > 0) {
  console.error('\n' + failures.length + ' 条判据失败，' + passed + ' 条通过')
  process.exitCode = 1
} else {
  console.log('\n全部 ' + passed + ' 条判据通过')
}
