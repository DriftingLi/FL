// 图标入口守卫的抽取式单测（ADR-0079 / 票 #1911）。
//
// 守卫是「规则的执行面」，它自己坏了必须报红 —— 否则规则退化为假绿。
// 运行：node --test scripts/check-icons.test.mjs（ci.yml 的 el-controls-selftest job 调用）。
//
// 这里钉三件事：四条判据的正例，四条判据的负例（注释 / <script> 字符串 / 非 24 网格 / 带 name），
// 以及**迁移期基线只减不增**（登记了却不再违规 → 报红，逼着删行）。
import { test } from 'node:test'
import assert from 'node:assert/strict'
import { execFileSync } from 'node:child_process'
import { readFileSync } from 'node:fs'
import { dirname, resolve } from 'node:path'
import { fileURLToPath } from 'node:url'
import { LEGACY_BASELINE, findViolations, isGuardedPath, scanSource } from './check-icons.mjs'

const ROOT = resolve(dirname(fileURLToPath(import.meta.url)), '..')

/** 包一层根模板，便于逐用例只看模板体。 */
const sfc = (templateBody, extra = '') => '<template>\n' + templateBody + '\n</template>\n' + extra

const rulesOf = (src) => findViolations(src).map((v) => v.rule)

test('正例：从 Element Plus 图标包取图标即报（业务侧一律走注册表）', () => {
  const src = sfc('  <div />', "import { Sun } from '@element-plus/icons-vue'\n")
  assert.deepEqual(rulesOf(src), ['ep-icon-import'])
  assert.equal(findViolations(src)[0].line, 4)
})

test('正例：模板里 <el-icon> 包裹层即报', () => {
  assert.deepEqual(rulesOf(sfc('  <el-icon :size="16"><Sun /></el-icon>')), ['el-icon-tag'])
})

test('正例：手写 24 网格图标 svg 即报', () => {
  assert.deepEqual(rulesOf(sfc('  <svg viewBox="0 0 24 24"><path d="M4 4" /></svg>')), ['bare-svg-icon'])
  // 属性换序 / 双引号换单引号同样命中
  assert.deepEqual(rulesOf(sfc("  <svg class=\"x\" viewBox='0 0 24 24' />")), ['bare-svg-icon'])
})

test('正例：<UiIcon> 不带 name 即报（静态或绑定都算带）', () => {
  assert.deepEqual(rulesOf(sfc('  <UiIcon :size="16" />')), ['uiicon-name'])
  assert.deepEqual(rulesOf(sfc('  <UiIcon name="sun" />')), [])
  assert.deepEqual(rulesOf(sfc('  <UiIcon :name="themeIcon" :size="18" morph />')), [])
})

test('正例：跨行开标签的 24 网格 svg 同样报（渲染器本体除外）', () => {
  const src = sfc([
    '  <div>',
    '    <svg',
    '      class="x"',
    '      viewBox="0 0 24 24"',
    '    >',
    '      <path d="M4 4" />',
    '    </svg>',
    '  </div>'
  ].join('\n'))
  assert.deepEqual(findViolations(src).map((v) => v.rule), ['bare-svg-icon'])
  // 渲染器本体（UiIcon.vue）就是那处手写 svg 的地方：只免这一条判据，其它判据照查
  assert.deepEqual(scanSource(src, 'frontend/src/components/ui/UiIcon.vue'), [])
  assert.equal(
    scanSource(sfc('  <el-icon><Sun /></el-icon>'), 'frontend/src/components/ui/UiIcon.vue').length,
    1,
    '渲染器例外不得扩到其他判据'
  )
})

test('负例：注释、<script> 字符串、非 24 网格图形都不报', () => {
  const src = [
    '<template>',
    '  <!-- <el-icon><Sun /></el-icon> -->',
    '  <svg viewBox="0 0 16 16"><path d="M3 3" /></svg>',
    '</template>',
    '',
    '<script setup lang="ts">',
    "const hint = '<el-icon>'",
    "const pkg = '@element-plus/icons-vue'",
    '</script>'
  ].join('\n')
  assert.deepEqual(findViolations(src), [])
})

test('负例：非 .vue 文件与守卫面之外的路径不进判定', () => {
  const src = sfc('  <el-icon><Sun /></el-icon>')
  assert.equal(isGuardedPath('frontend/src/pages/A.vue'), true)
  assert.equal(isGuardedPath('frontend/src/pages/A.ts'), false)
  assert.equal(isGuardedPath('scripts/check-icons.mjs'), false)
  assert.deepEqual(scanSource(src, 'frontend/src/pages/A.ts'), [])
})

test('正例：一行多处全部报出（不因第一个命中就收手）', () => {
  const src = sfc('  <el-icon><Sun /></el-icon><el-icon><Moon /></el-icon>')
  assert.equal(findViolations(src).length, 2)
})

test('迁移期基线只减不增：每条登记项今天仍真的违规', () => {
  const stale = []
  for (const rel of Object.keys(LEGACY_BASELINE)) {
    const hits = scanSource(readFileSync(resolve(ROOT, rel), 'utf8'), rel)
    if (hits.length === 0) stale.push(rel)
  }
  assert.deepEqual(stale, [], '这些文件已经迁干净了，请从 scripts/icon-legacy-baseline.json 删行')
})

test('迁移期基线覆盖当前全部违规面（新增违规不会因为基线存在而被吞）', () => {
  const src = sfc('  <el-icon><Sun /></el-icon>')
  const rel = 'frontend/src/pages/Unlisted.vue'
  assert.equal(scanSource(src, rel).length, 1)
  assert.equal(Object.hasOwn(LEGACY_BASELINE, rel), false)
})

test('CLI：--all 在当前树上通过（基线生效，且射程没漂）', () => {
  const out = execFileSync(process.execPath, [resolve(ROOT, 'scripts/check-icons.mjs'), '--all'], {
    cwd: ROOT,
    encoding: 'utf8'
  })
  assert.match(out, /无违规/)
})
