// api seam 守卫的抽取式单测（ADR-0053 §7 / spec #1049）。
//
// 守卫是「规则的执行面」，它自己坏了必须报红 —— 否则规则退化为假绿。
// 运行：node --test scripts/check-api-seam.test.mjs
//
// 判定面只认「页面/业务组件里的 import 说明符」：api 层自身、测试文件、其它目录一律不进判定，
// 这组用例正是把这些边界钉住。
import { test } from 'node:test'
import assert from 'node:assert/strict'
import { dirname, resolve } from 'node:path'
import { fileURLToPath } from 'node:url'
import { execFileSync } from 'node:child_process'
import {
  ALLOWLIST,
  GUARDED_MODULES,
  guardedSpecifier,
  isGuardedPath,
  isTestFile,
  parseAddedLines,
  scanSource
} from './check-api-seam.mjs'

const ROOT = resolve(dirname(fileURLToPath(import.meta.url)), '..')

test('正例：页面里直接引用请求层的四种写法都报出', () => {
  const src = [
    "import { unwrappedRequest } from '@/api/request'",
    "import { getValidAccessToken } from '@/api/client'",
    "export { unwrappedRequest } from '@/api/request'",
    "const req = await import('@/api/request')",
    "const c = require('../../api/client')"
  ].join('\n')
  const violations = scanSource(src, 'frontend/src/pages/admin/Inspection.vue')
  assert.equal(violations.length, 5)
  assert.deepEqual(
    violations.map((v) => v.line),
    [1, 2, 3, 4, 5]
  )
})

test('负例：相对路径写法同样命中（不只看 @/ 别名）', () => {
  const v = scanSource("import { unwrappedRequest } from '../api/request'", 'frontend/src/pages/admin/X.vue')
  assert.equal(v.length, 1)
  assert.equal(v[0].module, 'api/request')
})

test('负例：api 层自身的模块互引不报（api/*.ts 就是请求层的消费面）', () => {
  const src = "import { unwrappedRequest } from './request'\nimport type { PositionDict } from './generated/training'"
  assert.deepEqual(scanSource(src, 'frontend/src/api/position.ts'), [])
  assert.equal(isGuardedPath('frontend/src/api/position.ts'), false)
})

test('负例：测试文件整体不进判定面（按定义要 mock 请求层）', () => {
  const src = "import { unwrappedRequest } from '@/api/request'"
  for (const p of [
    'frontend/src/api/__tests__/position.spec.ts',
    'frontend/src/pages/admin/__tests__/X.spec.ts',
    'frontend/src/components/student/Foo.test.ts'
  ]) {
    assert.ok(isTestFile(p), p + ' 应判为测试文件')
    assert.equal(isGuardedPath(p), false, p + ' 不进守卫面')
  }
  assert.deepEqual(scanSource(src, 'frontend/src/pages/admin/__tests__/X.spec.ts'), [])
})

test('负例：非页面/组件目录不进判定面（utils / composables / stores）', () => {
  for (const p of [
    'frontend/src/utils/region.ts',
    'frontend/src/composables/useAsyncPage.ts',
    'frontend/src/stores/auth.ts',
    'frontend/src/config/navigation.ts'
  ]) {
    assert.equal(isGuardedPath(p), false, p + ' 不在守卫面')
  }
})

test('负例：形近模块名不误报（requests / request-utils / clientX）', () => {
  for (const spec of ['@/api/requests', '@/api/request-utils', '@/api/clientX', '@/apis/request']) {
    assert.equal(guardedSpecifier(spec), null, spec + ' 不应命中')
  }
  for (const spec of ['@/api/request', '@/api/client', '../api/request', '../../api/client']) {
    assert.ok(guardedSpecifier(spec), spec + ' 应命中')
  }
})

test('负例：URL 字符串字面量与注释不进判定（只认 import 说明符）', () => {
  const src = [
    '// 历史上这里写过：import { unwrappedRequest } from "@/api/request"',
    "const url = '/admin/positions'",
    'const s = "api/request"'
  ].join('\n')
  assert.deepEqual(scanSource(src, 'frontend/src/pages/admin/PositionManage.vue'), [])
})

test('白名单：登记的例外放行，且理由非空', () => {
  const entries = Object.entries(ALLOWLIST)
  for (const [p, reason] of entries) {
    assert.equal(isGuardedPath(p), false, p + ' 应在白名单内放行')
    assert.ok(typeof reason === 'string' && reason.length > 10, p + ' 的理由要写清')
  }
})

test('diff 模式：只认新增行号（删除行 / 上下文行不推进误判）', () => {
  const diff = [
    'diff --git a/frontend/src/pages/x.vue b/frontend/src/pages/x.vue',
    '--- a/frontend/src/pages/x.vue',
    '+++ b/frontend/src/pages/x.vue',
    '@@ -10,0 +11,3 @@',
    "+import { unwrappedRequest } from '@/api/request'",
    "+const a = 1",
    '+const b = 2'
  ].join('\n')
  const added = parseAddedLines(diff)
  assert.deepEqual([...added.get('frontend/src/pages/x.vue')], [11, 12, 13])
})

test('端到端：守卫在当前工作树上判绿（全站无未登记的直接引用）', () => {
  const out = execFileSync(process.execPath, ['scripts/check-api-seam.mjs', '--all'], {
    cwd: ROOT,
    encoding: 'utf8'
  })
  assert.match(out, /无违规/)
  // 守卫面非空：守卫坏了（例如把扫描目录写错）时不能靠「一个文件都没扫到」判绿
  assert.match(out, /[1-9][0-9]* 个文件均未直接引用请求层/)
})

test('守卫集与 ADR-0053 §7 的射程一致', () => {
  assert.deepEqual(GUARDED_MODULES, ['api/request', 'api/client'])
})
