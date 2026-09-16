// 列表四段式守卫的判定逻辑自检（形态对齐 check-api-seam.test.mjs）。
import { test } from 'node:test'
import assert from 'node:assert/strict'
import { scanSource, isGuardedPath, ALLOWLIST } from './check-async-section.mjs'

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
    '  <UiAsyncSection :error="loadError" :loading="loading" :empty="items.length === 0" @retry="retry">',
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
  // scanSource 不读 ALLOWLIST（判定与豁免分层）；守卫主流程负责放行
  assert.equal(scanSource(src, legacy).length, 1)
  assert.ok(ALLOWLIST[legacy].includes('#1054 存量'), '每条例外都要写明理由')
})
