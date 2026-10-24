// check-a11y 守卫的判定逻辑自检（#1892）：守卫坏了必须红，否则规则退化为假绿。
// 口径照 check-el-controls.test.mjs：表驱动喂源码文本，断言违规集合。
import { test } from 'node:test'
import assert from 'node:assert/strict'
import { isGuardedPath, scanSource } from './check-a11y.mjs'

const VUE = 'frontend/src/components/demo/Demo.vue'

test('命中：text-ink-muted（模板类名）', () => {
  const v = scanSource('<p class="text-ink-muted">提示</p>', VUE)
  assert.equal(v.length, 1)
  assert.equal(v[0].rule, 'muted-text')
  assert.equal(v[0].line, 1)
})

test('命中：var(--color-text-muted)（scoped 样式里当正文用）', () => {
  const v = scanSource('.hint { color: var(--color-text-muted); }', VUE)
  assert.equal(v.length, 1)
  assert.equal(v[0].rule, 'muted-var')
})

test('命中：bg-ui-500 与 text-white 同行（白字 3.74:1）', () => {
  const v = scanSource('<a class="bg-ui-500 px-4 text-white">去考试</a>', VUE)
  assert.equal(v.length, 1)
  assert.equal(v[0].rule, 'brand-fill-500')
})

test('命中：变体前缀也算（group-aria-pressed:bg-ui-500 + …:text-white）', () => {
  const line =
    '<button :class="on ? \'group-aria-pressed:bg-ui-500 group-aria-pressed:text-white\' : \'\'">x</button>'
  const v = scanSource(line, VUE)
  assert.equal(v.length, 1)
  assert.equal(v[0].rule, 'brand-fill-500')
})

test('命中：bg-ui-500 配 text-panel 也算（panel 是浅色面）', () => {
  const v = scanSource('<div class="bg-ui-500 text-panel">2</div>', VUE)
  assert.equal(v.length, 1)
})

test('放行：bg-ui-600 + text-white（5.47:1）', () => {
  assert.deepEqual(scanSource('<a class="bg-ui-600 text-white">去考试</a>', VUE), [])
})

test('放行：bg-ui-500 单独出现（圆点 / 指示条等无文字图形）', () => {
  assert.deepEqual(scanSource('<span class="size-2.5 rounded-full bg-ui-500"></span>', VUE), [])
  assert.deepEqual(scanSource('<div class="absolute bg-ui-500 shadow-sm" />', VUE), [])
})

test('放行：text-ink-3 / var(--color-text-tertiary)', () => {
  assert.deepEqual(scanSource('<p class="text-ink-3">提示</p>', VUE), [])
  assert.deepEqual(scanSource('.hint { color: var(--color-text-tertiary); }', VUE), [])
})

test('放行：注释行（说明性文字常引用被禁的名字）', () => {
  const src = ['<!-- 旧写法 text-ink-muted 已收口 -->', '// .hint { color: var(--color-text-muted) }', '* bg-ui-500 text-white'].join('\n')
  assert.deepEqual(scanSource(src, VUE), [])
})

test('不误伤：bg-ui-muted / text-ink-muted-strong 这类形近名', () => {
  assert.deepEqual(scanSource('<span class="bg-ink-muted"></span>', VUE), [])
})

test('路径面：非 .vue 与 frontend/src 之外整体放行', () => {
  const line = '<p class="text-ink-muted">x</p>'
  assert.deepEqual(scanSource(line, 'frontend/src/pages/demo/Demo.ts'), [])
  assert.deepEqual(scanSource(line, 'scripts/demo.vue'), [])
  assert.equal(isGuardedPath(VUE), true)
  assert.equal(isGuardedPath('frontend/src/pages/demo/Demo.spec.ts'), false)
  assert.equal(isGuardedPath('docs/demo.vue'), false)
})

test('行号：多行源码里命中行号准确（1-based）', () => {
  const src = ['<template>', '  <p class="text-ink-muted">x</p>', '</template>'].join('\n')
  const v = scanSource(src, VUE)
  assert.equal(v.length, 1)
  assert.equal(v[0].line, 2)
})

test('CLI 全量模式：当前树判绿（防假红；判定面失灵会在这里暴露）', async () => {
  const { execFileSync } = await import('node:child_process')
  const out = execFileSync(process.execPath, ['scripts/check-a11y.mjs', '--all'], { encoding: 'utf8' })
  assert.match(out, /无违规/)
})
