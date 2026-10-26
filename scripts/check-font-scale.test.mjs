// check-font-scale 守卫的判定逻辑自检（#1894 / ADR-0080）：守卫坏了必须红，否则规则退化为假绿。
// 口径照 check-a11y.test.mjs：表驱动喂源码文本，断言违规集合。两条规则各一半用例。
import { test } from 'node:test'
import assert from 'node:assert/strict'
import {
  DISPLAY_TIER,
  isGuardedPath,
  judgeFontSizeValue,
  scanSource,
  suggestStep
} from './check-font-scale.mjs'

const VUE = 'frontend/src/pages/demo/Demo.vue'
const TS = 'frontend/src/config/demo.ts'
const CSS = 'frontend/src/assets/styles/demo.css'

// ---------- 规则一：工具类任意值 ----------

test('命中：text-[13px] → text-sm（最大的一类，64 处）', () => {
  const v = scanSource('<p class="text-[13px] text-ink-3">提示</p>', VUE)
  assert.equal(v.length, 1)
  assert.equal(v[0].rule, 'arbitrary-font-size')
  assert.equal(v[0].line, 1)
  assert.match(v[0].why, /text-sm（14px）/)
})

test('命中：text-[15px] → text-base / text-[11px] → text-xs / text-[22px] → text-2xl', () => {
  assert.match(scanSource('<p class="text-[15px]">x</p>', VUE)[0].why, /text-base（16px）/)
  assert.match(scanSource('<p class="text-[11px]">x</p>', VUE)[0].why, /text-xs（12px）/)
  assert.match(scanSource('<h1 class="text-[22px]">x</h1>', VUE)[0].why, /text-2xl（24px）/)
})

test('命中：变体前缀也算（max-md:text-[10px] / hover:text-[9px]）', () => {
  assert.equal(scanSource('<p class="max-md:text-[10px]">x</p>', VUE).length, 1)
  assert.equal(scanSource('<p class="hover:text-[9px]">x</p>', VUE).length, 1)
})

test('命中：.ts 里的类名常量与三元分支', () => {
  const src = "const cls = computed(() => (compact.value ? 'text-[13px]' : 'text-[14px]'))"
  assert.equal(scanSource(src, TS).length, 2)
})

test('命中：非 px 的绝对单位（rem）同样不认', () => {
  assert.equal(scanSource('<p class="text-[0.875rem]">x</p>', VUE).length, 1)
})

test('放行：em 相对单位（UiActionChip 用 text-[1em] 让图标跟随父字号）', () => {
  assert.deepEqual(scanSource('<UiIcon name="x" class="text-[1em]" />', VUE), [])
})

test('放行：token 阶梯本身与 text-[var(--…)]（那是颜色）', () => {
  assert.deepEqual(scanSource('<p class="text-xs text-sm text-base text-lg text-xl text-2xl">x</p>', VUE), [])
  assert.deepEqual(scanSource('<span class="text-[var(--wechat-green)]">微信</span>', VUE), [])
})

test('放行：行高直写（leading-[1.8]）与尺寸直写（w-[110px]）不在本守卫射程', () => {
  assert.deepEqual(scanSource('<p class="leading-[1.8] w-[110px]">x</p>', VUE), [])
})

test('不误伤：text-[13px] 之外的形近写法', () => {
  assert.deepEqual(scanSource('<p class="text-13px text-[13] tracking-[0.04em]">x</p>', VUE), [])
})

// ---------- 规则二：scoped font-size 直写 ----------

test('命中：font-size: 13px（scoped 样式）', () => {
  const v = scanSource('.reply { font-size: 13px; }', VUE)
  assert.equal(v.length, 1)
  assert.equal(v[0].rule, 'literal-font-size')
  assert.match(v[0].why, /var\(--text-\*\)/)
  assert.match(v[0].why, /text-sm（14px）/)
})

test('命中：无空格写法（font-size:13px）与 rem / em / calc', () => {
  assert.equal(scanSource('.x { font-size:13px; }', VUE).length, 1)
  assert.equal(scanSource('.x { font-size: 0.875rem; }', VUE).length, 1)
  assert.equal(scanSource('.x { font-size: 0.9em; }', VUE).length, 1)
  assert.equal(scanSource('.x { font-size: calc(1rem + 2px); }', VUE).length, 1)
})

test('命中：.css 与 .vue 的 <style> 同一口径', () => {
  assert.equal(scanSource('.el-table { font-size: 13px; }', CSS).length, 1)
  const sfc = ['<style scoped>', '.a { font-size: 15px; }', '</style>'].join('\n')
  const v = scanSource(sfc, VUE)
  assert.equal(v.length, 1)
  assert.equal(v[0].line, 2)
})

test('命中：不在展示级的裸 px（100px）也不放行', () => {
  assert.equal(scanSource('.hero { font-size: 100px; }', VUE).length, 1)
})

test('放行：var(--text-*) / var(--fs-*, Npx) / !important 后缀 / 关键字值', () => {
  assert.deepEqual(scanSource('.a { font-size: var(--text-sm); }', VUE), [])
  assert.deepEqual(scanSource('.b { font-size: var(--fs-sm, 14px); }', VUE), [])
  assert.deepEqual(scanSource('.c { font-size: var(--fs-2xl, 24px) !important; }', VUE), [])
  assert.deepEqual(scanSource('.d { font-size: inherit; }', VUE), [])
})

test('放行：展示级 28 / 40 / 44 / 64px（大号数字与占位字形）', () => {
  for (const px of DISPLAY_TIER) {
    assert.deepEqual(scanSource(`.big { font-size: ${px}px; }`, VUE), [], px + 'px 应放行')
  }
})

test('放行：rem 基准（html）上一行带 font-scale-root 标记', () => {
  const src = [
    'html {',
    '  /* font-scale-root：布局基准（rem 尺度由它派生），不并入字号阶梯 */',
    '  font-size: 16px;',
    '}'
  ].join('\n')
  assert.deepEqual(scanSource(src, CSS), [])
})

test('命中：rem 基准没有标记（或标记被别的声明隔开）', () => {
  assert.equal(scanSource('html {\n  font-size: 16px;\n}', CSS).length, 1)
  const distant = ['/* font-scale-root：基准 */', '.a { color: red; }', 'html { font-size: 16px; }'].join('\n')
  assert.equal(scanSource(distant, CSS).length, 1)
})

test('放行：注释行（说明文字常引用被禁的旧写法）', () => {
  const src = [
    '<!-- 旧写法 text-[13px] 已收口成 text-sm -->',
    '// .content { font-size: 13px }  // 已被判据覆盖，别再写回来',
    '* text-[15px] → text-base'
  ].join('\n')
  assert.deepEqual(scanSource(src, VUE), [])
})

test('判定面单测：judgeFontSizeValue 脱离整段源码也能判', () => {
  assert.ok(judgeFontSizeValue('14px', ['x'], 0))
  assert.ok(judgeFontSizeValue('13px', ['x'], 0))
  assert.ok(judgeFontSizeValue('100px', ['x'], 0))
  assert.equal(judgeFontSizeValue('28px', ['x'], 0), null)
  assert.equal(judgeFontSizeValue('var(--text-sm)', ['x'], 0), null)
  assert.equal(judgeFontSizeValue('inherit', ['x'], 0), null)
})

// ---------- 路径面与 CLI ----------

test('路径面：非 frontend/src 整体放行；.css 现在也在面内', () => {
  const line = '.a { font-size: 13px; }'
  assert.deepEqual(scanSource(line, 'scripts/demo.vue'), [])
  assert.equal(isGuardedPath(VUE), true)
  assert.equal(isGuardedPath(TS), true)
  assert.equal(isGuardedPath(CSS), true)
  assert.equal(isGuardedPath('docs/demo.css'), false)
})

test('行号：多行源码里命中行号准确（1-based），同一行多处各自计数', () => {
  const src = ['<template>', '  <p class="text-[13px]">x</p>', '  <i class="text-[11px] hover:text-[10px]" />', '</template>'].join('\n')
  const v = scanSource(src, VUE)
  assert.equal(v.length, 3)
  assert.deepEqual(v.map((x) => x.line), [2, 3, 3])
})

test('等距向上取：17px → text-lg、22px → text-2xl（口径是「宁可大 1px」）', () => {
  assert.match(suggestStep('17', 'px'), /text-lg（18px）/)
  assert.match(suggestStep('22', 'px'), /text-2xl（24px）/)
  assert.match(suggestStep('13', 'px'), /text-sm（14px）/)
  assert.match(suggestStep('0.875', 'rem'), /阶梯里的一档/)
})

test('CLI 全量模式：当前树判绿（防假红；判定面失灵会在这里暴露）', async () => {
  const { execFileSync } = await import('node:child_process')
  const out = execFileSync(process.execPath, ['scripts/check-font-scale.mjs', '--all'], { encoding: 'utf8' })
  assert.match(out, /无违规/)
})
