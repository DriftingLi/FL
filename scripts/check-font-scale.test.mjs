// check-font-scale 守卫的判定逻辑自检（#1894 / ADR-0080）：守卫坏了必须红，否则规则退化为假绿。
// 口径照 check-a11y.test.mjs：表驱动喂源码文本，断言违规集合。
import { test } from 'node:test'
import assert from 'node:assert/strict'
import { isGuardedPath, scanSource, suggestStep } from './check-font-scale.mjs'

const VUE = 'frontend/src/pages/demo/Demo.vue'
const TS = 'frontend/src/config/demo.ts'

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

test('放行：token 阶梯本身', () => {
  const src = '<p class="text-xs text-sm text-base text-lg text-xl text-2xl text-3xl">x</p>'
  assert.deepEqual(scanSource(src, VUE), [])
})

test('放行：text-[var(--…)] 是颜色不是字号', () => {
  assert.deepEqual(
    scanSource('<span class="text-[var(--wechat-green)]">微信</span>', VUE),
    []
  )
})

test('放行：行高直写（leading-[1.8]）与尺寸直写（w-[110px]）不在本守卫射程', () => {
  assert.deepEqual(scanSource('<p class="leading-[1.8] w-[110px]">x</p>', VUE), [])
})

test('放行：注释行（说明文字常引用被禁的旧写法）', () => {
  const src = [
    '<!-- 旧写法 text-[13px] 已收口成 text-sm -->',
    '// .content { font-size: 13px }  // 不是工具类，不在射程',
    '* text-[15px] → text-base'
  ].join('\n')
  assert.deepEqual(scanSource(src, VUE), [])
})

test('不误伤：text-[13px] 之外的形近写法', () => {
  assert.deepEqual(scanSource('<p class="text-13px text-[13] tracking-[0.04em]">x</p>', VUE), [])
})

test('路径面：非 frontend/src 与非法后缀整体放行', () => {
  const line = '<p class="text-[13px]">x</p>'
  assert.deepEqual(scanSource(line, 'scripts/demo.vue'), [])
  assert.deepEqual(scanSource(line, 'frontend/src/assets/styles/demo.css'), [])
  assert.equal(isGuardedPath(VUE), true)
  assert.equal(isGuardedPath(TS), true)
  assert.equal(isGuardedPath('docs/demo.vue'), false)
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
