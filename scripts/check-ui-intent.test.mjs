// UI 意图登记制守卫的抽取式单测（ADR-0083 决策 6 / #1982）。
//
// 守卫是「规则的执行面」，它自己坏了必须报红 —— 否则规则退化为假绿。
// 运行：node --test scripts/check-ui-intent.test.mjs（ci.yml 的 el-controls-selftest job 调用）。
//
// 三组用例：① 登记表派生（文档 → 集合）；② 规则①（未登记 el-*）；③ 规则②（越档字面量）。
// 派生那组同时钉住「同一格写多个子件」「父件未登记」两处写出来会假红/假绿的形状。
import { test } from 'node:test'
import assert from 'node:assert/strict'
import { execFileSync } from 'node:child_process'
import { dirname, resolve } from 'node:path'
import { fileURLToPath } from 'node:url'
import { classify, loadRegistry, loadRegistryFromFile } from './lib/ui-intent-registry.mjs'
import { findIntentViolations, findStyleViolations, scanSource, setStrictNewCode } from './check-ui-intent.mjs'

const ROOT = resolve(dirname(fileURLToPath(import.meta.url)), '..')

/** 包一层根模板，便于逐用例只看模板体。 */
const sfc = (templateBody, extra = '') => '<template>\n' + templateBody + '\n</template>\n' + extra

/** 合成的登记表：表头与真文档同形，只留用例要的东西。 */
const FIXTURE = [
  '### 意图登记表（fixture）',
  '',
  '| 意图 | 三轴 | 唯一形态 | 存量 |',
  '| --- | --- | --- | --- |',
  '| **文本输入** | a / b / c | `UiInput` | 15（+ `el-input` 169 待迁，按域分批） |',
  '| **下拉选择** | a / b / c | `UiSelect` | 3（+ `el-select` 57 待迁，按域分批） |',
  '| **面板切换** | a / b / c | `el-tabs`（原生，登记） | 4 |',
  '| **菜单收纳** | a / b / c | `UiMoreMenu`；三处保留原生 `el-dropdown` | 16 |',
  '',
  '| 件 | 唯一形态 | 存量 |',
  '| --- | --- | --- |',
  '| 数据表 | `<el-table>`（不封装） | 32 / 173 列 |',
  '| 表单容器 | `el-form`（原生，登记） | 38 |',
  '',
  '| 子件 | 父件 |',
  '| --- | --- |',
  '| `el-form-item` | `el-form` |',
  '| `el-option` / `el-option-group` | `el-select` |',
  '| `el-dropdown-menu` / `el-dropdown-item` | `el-dropdown` |',
  '| `el-ghost-item` | `el-ghost` |',
  '',
  '### 下一节',
  ''
].join('\n')

// ---------------------------------------------------------------- ① 派生

test('派生：唯一形态里的 el-* 进原生集，存量里的进按域分批集', () => {
  const reg = loadRegistry(FIXTURE)
  assert.ok(reg.native.has('el-tabs'))
  assert.ok(reg.native.has('el-table'))
  assert.ok(reg.native.has('el-form'))
  assert.ok(reg.native.has('el-dropdown'))
  assert.ok(reg.pending.has('el-input'))
  assert.ok(!reg.native.has('el-input'))
})

test('派生：一格写多个子件时每个都登记（曾经只取第一个 → 第二个假红）', () => {
  const reg = loadRegistry(FIXTURE)
  assert.equal(reg.children.get('el-option'), 'el-select')
  assert.equal(reg.children.get('el-option-group'), 'el-select')
  assert.equal(reg.children.get('el-dropdown-menu'), 'el-dropdown')
  assert.equal(reg.children.get('el-dropdown-item'), 'el-dropdown')
})

test('派生：原生优先于欠账（同一标签在两行里出现时不算待迁）', () => {
  const reg = loadRegistry(FIXTURE)
  assert.ok(!reg.pending.has('el-tabs'))
})

test('派生：表头被改名即抛错，不静默产出空集（那等于全量放行）', () => {
  assert.throws(() => loadRegistry('### 意图登记表\n\n| 别的表 |\n| --- |\n| x |\n'), /缺少/)
})

test('派生：真文档可解析，且原生集里确实有原生登记件', () => {
  const reg = loadRegistryFromFile()
  for (const tag of ['el-tabs', 'el-drawer', 'el-popconfirm', 'el-avatar', 'el-table', 'el-image', 'el-dropdown']) {
    assert.ok(reg.native.has(tag), tag + ' 应在原生集')
  }
  for (const tag of ['el-input', 'el-select', 'el-alert', 'el-skeleton']) {
    assert.ok(reg.pending.has(tag), tag + ' 应在按域分批集')
  }
  assert.ok(reg.children.size >= 7, '子件表至少 7 个标签')
})

test('派生：子件表的父件必须能解析（写错父件名 = 静默放行整个子件）', () => {
  const reg = loadRegistryFromFile()
  const unresolved = [...reg.children].filter(([, parent]) =>
    !reg.native.has(parent) && !reg.pending.has(parent) && !reg.children.has(parent)
  )
  assert.deepEqual(unresolved, [], '子件指向了未登记的父件：' + JSON.stringify(unresolved))
})

// ---------------------------------------------------------------- ② 规则① 意图

test('规则①：未登记的 el-* 报红，行号指向标签所在行', () => {
  const reg = loadRegistry(FIXTURE)
  const violations = findIntentViolations(sfc('  <el-unknown-thing />'), reg, 'pages/a.vue')
  assert.equal(violations.length, 1)
  assert.equal(violations[0].tag, 'el-unknown-thing')
  assert.equal(violations[0].line, 2)
  assert.equal(violations[0].rule, 'intent')
})

test('规则①：登记的三种形态都放行（原生 / 按域分批 / 子件）', () => {
  const reg = loadRegistry(FIXTURE)
  const body = ['  <el-tabs>', '    <el-input v-model="a" />', '    <el-form-item label="x" />', '  </el-tabs>'].join('\n')
  assert.deepEqual(findIntentViolations(sfc(body), reg, 'pages/a.vue'), [])
})

test('规则①：父件没登记的孤儿子件报红（子件不是免死金牌）', () => {
  const reg = loadRegistry(FIXTURE)
  const violations = findIntentViolations(sfc('  <el-ghost-item />'), reg, 'pages/a.vue')
  assert.equal(violations.length, 1)
  assert.equal(violations[0].orphanParent, 'el-ghost')
})

test('规则①：封装层整面放行（内部本来就是 EP 控件）', () => {
  const reg = loadRegistry(FIXTURE)
  assert.deepEqual(findIntentViolations(sfc('  <el-unknown-thing />'), reg, 'components/ui/UiFoo.vue'), [])
  assert.deepEqual(findIntentViolations(sfc('  <el-unknown-thing />'), reg, 'components/ui/nested/UiFoo.vue'), [])
})

test('规则①：注释里的 el-* 不算代码', () => {
  const reg = loadRegistry(FIXTURE)
  const body = [
    '  <!-- <el-unknown-thing /> 写在注释里 -->',
    '  <div>{{ el-unknown-thing }}</div>'
  ].join('\n')
  assert.deepEqual(findIntentViolations(sfc(body), reg, 'pages/a.vue'), [])
})

test('规则①：<script> 里的 el-* 不算代码', () => {
  const reg = loadRegistry(FIXTURE)
  const withScript = sfc('  <div />', '<script setup>' + '\n' + 'const s = el-unknown-thing' + '\n' + '</script>')
  assert.deepEqual(findIntentViolations(withScript, reg, 'pages/a.vue'), [])
})

test('规则①：多行开标签只报一次', () => {
  const reg = loadRegistry(FIXTURE)
  const body = ['  <el-unknown-thing', '    :a="1"', '  />'].join('\n')
  assert.equal(findIntentViolations(sfc(body), reg, 'pages/a.vue').length, 1)
})

// ---------------------------------------------------------------- ③ 规则② 档位

test('规则②：纯字面量的圆角/阴影报红，并给出最近档', () => {
  const src = '<style scoped>' + '\n' + '.a { border-radius: 8px; }' + '\n' + '.b { box-shadow: 0 1px 3px rgba(0,0,0,.1); }' + '\n' + '</style>'
  const violations = findStyleViolations(src)
  assert.equal(violations.length, 2)
  assert.equal(violations[0].prop, 'border-radius')
  assert.equal(violations[0].value, '8px')
  assert.match(violations[0].hint, /var\(--radius-md\)/)
  assert.equal(violations[1].prop, 'box-shadow')
})

test('规则②：token 与档内值放行', () => {
  const src = [
    '<style scoped>',
    '.a { border-radius: var(--radius-md); }',
    '.b { border-radius: 0; }',
    '.c { border-radius: 50%; }',
    '.d { border-radius: 0 !important; }',
    '.e { border-radius: 0 var(--radius-full) var(--radius-full) 0; }',
    '.f { box-shadow: none; }',
    '.g { box-shadow: none !important; }',
    '.h { box-shadow: 0 0 0 1px var(--color-border) inset; }',
    '</style>'
  ].join('\n')
  assert.deepEqual(findStyleViolations(src), [])
})

test('规则②：inline style 也扫（模板里同样是容器外观）', () => {
  const src = '<template>' + '\n' + '  <img style="border-radius: 8px" />' + '\n' + '</template>'
  const violations = findStyleViolations(src)
  assert.equal(violations.length, 1)
  assert.equal(violations[0].value, '8px')
})

// ---------------------------------------------------------------- ④ 防恒绿 / CLI

test('扫描面：合成的违规源码必须判红（防守卫恒绿）', () => {
  const bad = sfc('  <el-unknown-thing />', '<style scoped>' + '\n' + '.a { border-radius: 7px; }' + '\n' + '</style>')
  const violations = scanSource(bad, 'pages/bad.vue')
  assert.equal(violations.length, 2, '两条规则各报一处：' + JSON.stringify(violations))
  assert.deepEqual(violations.map((v) => v.rule).sort(), ['intent', 'tier'])
})

test('扫描面：干净源码必须判绿（防守卫恒红）', () => {
  const body = ['  <el-table>', '    <el-table-column />', '  </el-table>'].join('\n')
  const good = sfc(body, '<style scoped>' + '\n' + '.a { border-radius: var(--radius-card); }' + '\n' + '</style>')
  assert.deepEqual(scanSource(good, 'pages/good.vue'), [])
})

test('CLI --all 在当前树上全绿（防假红）', () => {
  const out = execFileSync(process.execPath, ['scripts/check-ui-intent.mjs', '--all'], { cwd: ROOT, encoding: 'utf8' })
  assert.match(out, /无违规/)
})

// ---------------- ⑤ 新代码必用（#1981 的生效点）

test('模式敏感：按域分批的标签在 --all 放行、在新代码口径下报红', () => {
  const src = sfc(['  <el-input v-model="x" />', '  <el-select v-model="y" />'].join('\n'))
  setStrictNewCode(false)
  try {
    assert.deepEqual(scanSource(src, 'pages/a.vue'), [], '存量口径应放行')
    setStrictNewCode(true)
    const strict = scanSource(src, 'pages/a.vue')
    assert.equal(strict.length, 2, '新增行口径应两条都报')
    assert.deepEqual(strict.map((v) => v.wrapper).sort(), ['UiInput', 'UiSelect'])
  } finally {
    setStrictNewCode(false)
  }
})

test('模式敏感：封装件内部不受影响（components/ui 整面放行）', () => {
  setStrictNewCode(true)
  try {
    assert.deepEqual(scanSource(sfc('  <el-input />'), 'components/ui/UiFoo.vue'), [])
  } finally {
    setStrictNewCode(false)
  }
})

test('classify：四类归法可区分', () => {
  const reg = loadRegistry(FIXTURE)
  assert.equal(classify('el-tabs', reg).kind, 'native')
  assert.equal(classify('el-input', reg).kind, 'pending')
  assert.equal(classify('el-option', reg).kind, 'child')
  assert.equal(classify('el-ghost-item', reg).kind, 'unknown')
})
