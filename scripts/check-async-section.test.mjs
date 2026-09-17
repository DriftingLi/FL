// 列表四段式守卫的判定逻辑自检（形态对齐 check-api-seam.test.mjs）。
import { test } from 'node:test'
import assert from 'node:assert/strict'
import { execFileSync } from 'node:child_process'
import { fileURLToPath } from 'node:url'
import path from 'node:path'
import { scanSource, scanEmptyProps, isGuardedPath, ALLOWLIST, EMPTY_EXCEPTIONS, ALLOWED_EMPTY_VALUES, scanAdminTiers, parseTierLine, TIER_FORMS } from './check-async-section.mjs'

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

test('EMPTY_EXCEPTIONS 清零：#1101 唯一一条例外（Inspection 走 useAdminTable、无对等判据）已由 #1102 销号', () => {
  assert.deepEqual(Object.keys(EMPTY_EXCEPTIONS), [])
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
// ===== 规则 ② 的补充：同页多实例的具名判据（#1102）=====

test('负例：:empty="isEmptyViews" 由档位 composable 解构改名而来 → 放行（同页五个列表各一个具名判据）', () => {
  const src = [
    '<template>',
    '  <UiAsyncSection :empty="isEmptyViews" @retry="retryViews" />',
    '</template>',
    '<script setup lang="ts">',
    'const { isEmpty: isEmptyViews, load: loadViews, retry: retryViews } = useAdminTable<TrailView>({ fetch })',
    '</script>'
  ].join('\n')
  assert.equal(scanEmptyProps(src.split('\n')).length, 0)
})

test('正例：没声明过的 isEmptyXxx 仍然报红（不能靠改名字绕过）', () => {
  const lines = [
    '<template>',
    '  <UiAsyncSection :empty="isEmptyViews" @retry="retryViews" />',
    '</template>'
  ]
  assert.equal(scanEmptyProps(lines).length, 1)
})

// ===== 规则 ③：admin 页两档档位登记（#1102，ADR-0056 §9）=====

const TWO_TIER_BODY = [
  '<script setup lang="ts">',
  "import { useAdminTable } from '@/composables/useAdminTable'",
  "import { useAsyncPage } from '@/composables/useAsyncPage'",
  'const table = useAdminTable<Row>({ fetch: async () => ({ list: [], total: 0 }) })',
  'const count = useAsyncPage(async () => {})',
  '</script>'
].join('\n')

test('正例：两档在场的页面逐档登记且实据成立 → 放行', () => {
  const src = [
    '<!--',
    '  列表档位：useAdminTable（分页列表）—— 流水 / 留痕',
    '  列表档位：useAsyncPage（只读计数）—— 删除已解决帖计数',
    '-->',
    TWO_TIER_BODY
  ].join('\n')
  assert.equal(scanAdminTiers(src, 'frontend/src/pages/admin/Demo.vue').length, 0)
})

test('正例：两档在场却没登记 → 违规（第二档没写明归属）', () => {
  const v = scanAdminTiers(TWO_TIER_BODY, 'frontend/src/pages/admin/Demo.vue')
  assert.equal(v.length, 1)
  assert.match(v[0].message, /没有在文件顶部登记档位/)
})

test('正例：单档页面不要求登记（归属由既有约定唯一确定）', () => {
  const one = [
    '<script setup lang="ts">',
    'const table = useAdminTable<Row>({ fetch: async () => ({ list: [], total: 0 }) })',
    '</script>'
  ].join('\n')
  assert.equal(scanAdminTiers(one, 'frontend/src/pages/admin/Demo.vue').length, 0)
})

test('正例：自造档位形态的登记行 → 违规（登记表只有三行）', () => {
  const src = [
    '// 列表档位：useCrudTable（分页列表）',
    '<script setup lang="ts">',
    'const table = useAdminTable<Row>({ fetch: async () => ({ list: [], total: 0 }) })',
    '</script>'
  ].join('\n')
  const v = scanAdminTiers(src, 'frontend/src/pages/admin/Demo.vue')
  assert.equal(v.length, 1)
  assert.match(v[0].message, /不在登记表里/)
  assert.ok(TIER_FORMS.every(f => f.evidence instanceof RegExp))
})

test('正例：登记了却拿不出实据 → 违规（登记行不能是一句没兑现的声明）', () => {
  const src = [
    '// 列表档位：useAsyncPage（只读计数）',
    '<script setup lang="ts">',
    'const table = useAdminTable<Row>({ fetch: async () => ({ list: [], total: 0 }) })',
    '</script>'
  ].join('\n')
  const v = scanAdminTiers(src, 'frontend/src/pages/admin/Demo.vue')
  assert.equal(v.length, 1)
  assert.match(v[0].message, /拿不出实据/)
})

test('负例：非 admin 页不适用档位规则；解析器只认登记行', () => {
  assert.equal(scanAdminTiers(TWO_TIER_BODY, 'frontend/src/pages/student/Demo.vue').length, 0)
  assert.equal(scanAdminTiers(TWO_TIER_BODY, 'backend/x.vue').length, 0)
  const parsed = parseTierLine('  // 列表档位：useAsyncPage（只读计数）—— 计数')
  assert.equal(parsed.tier, 'useAsyncPage')
  assert.equal(parsed.form, '只读计数')
  assert.equal(parseTierLine('<!-- 列表档位：useAdminTable（分页列表）—— 列表 -->').tier, 'useAdminTable')
  assert.equal(parseTierLine('// 列表档位：useCrudTable（分页列表）').tier, '')
  assert.equal(parseTierLine('// 这段说明 why 两档归属要写明'), null)
})

test('CLI 正向：真实 admin 页四份档位登记在 --all 下全绿（守卫不假红）', () => {
  const ROOT = path.resolve(path.dirname(fileURLToPath(import.meta.url)), '..')
  const out = execFileSync('node', [path.join(ROOT, 'scripts/check-async-section.mjs'), '--all'], { cwd: ROOT, encoding: 'utf8' })
  assert.match(out, /档位登记齐备/)
})
