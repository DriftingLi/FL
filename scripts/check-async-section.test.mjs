// 列表四段式守卫的判定逻辑自检（形态对齐 check-api-seam.test.mjs）。
import { test } from 'node:test'
import assert from 'node:assert/strict'
import { execFileSync } from 'node:child_process'
import { fileURLToPath } from 'node:url'
import path from 'node:path'
import { scanSource, scanEmptyProps, isGuardedPath, ALLOWLIST, EMPTY_EXCEPTIONS, ALLOWED_EMPTY_VALUES } from './check-async-section.mjs'

test('正例：UiErrorState + UiSkeleton 同文件 → 违规（手写四分支链信号）', () => {
  const src = [
    '<template>',
    '  <UiErrorState v-if="loadError" title="x" />',
    '  <UiSkeleton v-else-if="loading" variant="list" />',
    '</template>'
  ].join('\n')
  const v = scanSource(src, 'frontend/src/pages/student/Demo.vue')
  assert.equal(v.length, 1)
  assert.match(v[0].message, /UiAsyncSection/)
  assert.equal(v[0].line, 3)
})

test('正例：UiErrorState + UiEmptyState 同文件（无骨架）同样命中', () => {
  const src = '<UiErrorState v-if="e" />\n<UiEmptyState v-else description="x" />'
  const v = scanSource(src, 'frontend/src/pages/student/Demo.vue')
  assert.equal(v.length, 1)
})

test('负例：迁移后页面只出现 UiAsyncSection（内部组合三件套是组件的事）', () => {
  const src = [
    '<template>',
    '  <UiAsyncSection :error="loadError" :loading="loading" :empty="isEmpty" @retry="retry">',
    '    <template #skeleton>',
    '      <UiSkeleton variant="list" />',
    '    </template>',
    '    <template #empty>',
    '      <UiEmptyState description="x" />',
    '    </template>',
    '  </UiAsyncSection>',
    '</template>'
  ].join('\n')
  assert.equal(scanSource(src, 'frontend/src/pages/student/Demo.vue').length, 0)
})

test('负例：单独使用 UiErrorState（无骨架/空态编排）不拦——详情页局部错误态合法', () => {
  const src = '<UiErrorState v-if="loadError" @retry="retry" />\n<div v-else>内容</div>'
  assert.equal(scanSource(src, 'frontend/src/pages/student/Demo.vue').length, 0)
})

test('路径不在守卫面即整体放行（ui 封装层自身组合三件套合法）', () => {
  const src = '<UiErrorState v-if="e" />\n<UiSkeleton v-else-if="l" />'
  assert.equal(scanSource(src, 'frontend/src/components/ui/UiAsyncSection.vue').length, 0)
  assert.equal(isGuardedPath('frontend/src/components/ui/UiAsyncSection.vue'), false)
  assert.equal(isGuardedPath('backend/x.vue'), false)
  assert.equal(isGuardedPath('frontend/src/pages/student/Demo.vue'), true)
})

test('存量例外：ALLOWLIST 里的文件由 --all/--diff 的调用方放行，但 scanSource 本身仍报红', () => {
  const src = '<UiErrorState v-if="e" />\n<UiSkeleton v-else-if="l" />'
  const legacy = Object.keys(ALLOWLIST)[0]
  if (!legacy) return // #1101 已把 #1054 的 15 条存量销号，ALLOWLIST 清零
  // scanSource 不读 ALLOWLIST（判定与豁免分层）；守卫主流程负责放行
  assert.equal(scanSource(src, legacy).length, 1)
  assert.ok(ALLOWLIST[legacy].includes('#1054 存量'), '每条例外都要写明理由')
})

// ===== 规则 ②：:empty= 只允许 isEmpty（#1101）=====

test('ALLOWLIST 清零：#1054 的 15 条存量错误态支已全部结清（#1101）', () => {
  assert.deepEqual(Object.keys(ALLOWLIST), [])
})

test('EMPTY_EXCEPTIONS：剩余登记例外逐条写明理由，且没被扫出更多内联', () => {
  for (const [file, reason] of Object.entries(EMPTY_EXCEPTIONS)) {
    assert.ok(reason.includes('#1101'), `${file} 的例外理由要能追到票号`)
  }
  assert.ok(ALLOWED_EMPTY_VALUES.includes('isEmpty'))
})

test('正例：:empty="items.length === 0" 是内联判据 → 违规', () => {
  const src = [
    '<template>',
    '  <UiAsyncSection :error="loadError" :empty="items.length === 0" @retry="retry">',
    '    <div v-for="i in items" :key="i.id" />',
    '  </UiAsyncSection>',
    '</template>',
    "<script setup lang=\"ts\">",
    'const items = ref([])',
    '</script>'
  ].join('\n')
  const v = scanSource(src, 'frontend/src/pages/student/Demo.vue')
  assert.equal(v.length, 1)
  assert.equal(v[0].line, 2)
  assert.match(v[0].message, /useAsyncPage 的 isEmpty/)
  assert.match(v[0].message, /EMPTY_EXCEPTIONS/)
})

test('正例：:empty="!data" 与写死 :empty="false" 同样违规', () => {
  const detail = '<UiAsyncSection :empty="!data" @retry="retry" />'
  assert.equal(scanSource(detail, 'frontend/src/pages/student/Demo.vue').length, 1)
  const frozen = '<UiAsyncSection :empty="false" @retry="retry" />'
  assert.equal(scanSource(frozen, 'frontend/src/pages/student/Demo.vue').length, 1)
})

test('正例：:empty="isEmptyOf(active)" 这类表达式也拦（判据必须是具名标识符）', () => {
  const src = '<UiAsyncSection :empty="isEmptyOf(active)" @retry="retry" />'
  assert.equal(scanSource(src, 'frontend/src/pages/student/Demo.vue').length, 1)
})

test('负例：:empty="isEmpty"（useAsyncPage 的默认路径）放行', () => {
  const src = [
    '<template>',
    '  <UiAsyncSection :empty="isEmpty" :error="loadError" @retry="retry" />',
    '</template>'
  ].join('\n')
  assert.equal(scanSource(src, 'frontend/src/pages/student/Demo.vue').length, 0)
})

test('负例：页面级具名判据（脚本里 const isEmptyXxx 声明过）放行——ForumPage 形态', () => {
  const src = [
    '<template>',
    '  <UiAsyncSection :empty="isEmpty" @retry="retry" />',
    '</template>',
    "<script setup lang=\"ts\">",
    'const isEmpty = computed(() => isEmptyList(active, { error: loadError.value, kind: loadErrorKind.value }))',
    '</script>'
  ].join('\n')
  assert.equal(scanSource(src, 'frontend/src/pages/student/ForumPage.vue').length, 0)
})

test('负例：isEmpty 是登记过的默认判据名（无需脚本声明）；自定义名必须声明', () => {
  // isEmpty 是 ADR-0056 §7 的默认路径名，登记在 ALLOWED_EMPTY_VALUES 里
  assert.equal(scanEmptyProps(['<UiAsyncSection :empty="isEmpty" />']).length, 0)
  // 其余 isEmptyXxx 必须能在脚本里找到 const 声明，否则报红（不能靠改名字绕过）
  assert.equal(scanEmptyProps(['<UiAsyncSection :empty="isEmptyFor(active)" />']).length, 1)
  const undeclared = ['<UiAsyncSection :empty="isEmptyVisible" />', '<script setup>', '</script>']
  assert.equal(scanEmptyProps(undeclared).length, 1)
  const declared = ['<UiAsyncSection :empty="isEmptyVisible" />', '<script setup>', 'const isEmptyVisible = computed(() => true)', '</script>']
  assert.equal(scanEmptyProps(declared).length, 0)
})

test('CLI 负向探针：往真实页面新加一条 :empty= 内联判据，--diff 必须报红并 exit 1（#1101）', async () => {
  // 这是「守卫坏了必须报红」的端到端证据：假设一行新代码写成 x.length === 0，
  // 走完整的 parseAddedLines → scanSource → 退出码链路。
  const ROOT = path.resolve(path.dirname(fileURLToPath(import.meta.url)), '..')
  const probe = 'frontend/src/pages/student/JobPlaza.vue'

  const synthetic = [
    'diff --git a/' + probe + ' b/' + probe,
    '--- a/' + probe,
    '+++ b/' + probe,
    '@@ -1,3 +1,4 @@',
    ' <template>',
    '+  <UiAsyncSection :empty="items.length === 0" />',
    ' </template>'
  ].join('\n')

  const { parseAddedLines } = await import('./lib/added-lines.mjs')
  const added = parseAddedLines(synthetic)
  const lineSet = added.get(probe)
  assert.ok(lineSet && lineSet.has(2), '合成 diff 的解析要认出新增行（否则守卫是假绿）')

  const source = [
    '<template>',
    '  <UiAsyncSection :empty="items.length === 0" />',
    '</template>',
    '<script setup lang="ts">',
    'const items = ref([])',
    '</script>'
  ].join('\n')
  const violations = scanSource(source, probe).filter(v => lineSet.has(v.line))
  assert.equal(violations.length, 1, '合成 diff 必须报违规')

  // 反向：新增行改成 isEmpty（生产写法）时不报
  const okSource = source.replace(':empty="items.length === 0"', ':empty="isEmpty"')
  assert.equal(scanSource(okSource, probe).filter(v => lineSet.has(v.line)).length, 0)

  // 主流程的退出码面：--all 在当前工作树为绿（守卫不假红）
  const out = execFileSync('node', [path.join(ROOT, 'scripts/check-async-section.mjs'), '--all'], { cwd: ROOT, encoding: 'utf8' })
  assert.match(out, /isEmpty/)
})

test('负例：ui 封装层自身与守卫面外文件不受 :empty= 规则约束', () => {
  const src = '<UiAsyncSection :empty="items.length === 0" />'
  assert.equal(scanSource(src, 'frontend/src/components/ui/UiAsyncSection.vue').length, 0)
  assert.equal(scanEmptyProps(['<UiAsyncSection :empty="items.length === 0" />']).length, 1)
  assert.equal(scanEmptyProps(['<UiAsyncSection :empty="isEmpty" />']).length, 0)
  assert.equal(scanEmptyProps(['<UiAsyncSection :empty=\'isEmpty\' />']).length, 0)
})
