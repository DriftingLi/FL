// 已收敛控件守卫的抽取式单测（spec #940 片一 / seam C）。
//
// 守卫是「规则的执行面」，它自己坏了必须报红 —— 否则规则退化为假绿。
// 运行：node --test scripts/check-el-controls.test.mjs（ci.yml 的 el-controls-selftest job 调用）。
//
// 判定面只认「根 <template> 块」：注释、<script> 字符串、<style> 选择器一律不进判定，
// 这组用例正是把这些边界钉住。
import { test } from 'node:test'
import assert from 'node:assert/strict'
import { execFileSync } from 'node:child_process'
import { dirname, resolve } from 'node:path'
import { fileURLToPath } from 'node:url'
import { GUARDED_CONTROLS, findViolations, isAllowedPath, parseAddedLines, scanSource } from './check-el-controls.mjs'

const ROOT = resolve(dirname(fileURLToPath(import.meta.url)), '..')

/** 包一层根模板，便于逐用例只看模板体。 */
const sfc = (templateBody, extra = '') => '<template>\n' + templateBody + '\n</template>\n' + extra

test('正例：守卫集里的控件出现在模板中即报出（逐类）', () => {
  for (const tag of GUARDED_CONTROLS) {
    const violations = findViolations(sfc('  <' + tag + ' />'))
    assert.equal(violations.length, 1, tag + ' 应报 1 处')
    assert.equal(violations[0].tag, tag)
    assert.equal(violations[0].line, 2)
  }
})

test('负例：放行控件不报（组内容项 / 表格边界 / 未封装表单域与布局类）', () => {
  const allowed = [
    'el-table',
    'el-table-column',
    'el-radio',
    'el-radio-button',
    'el-checkbox',
    'el-input',
    'el-form',
    'el-form-item',
    'el-select',
    'el-option',
    'el-icon',
    'el-row',
    'el-col',
    'el-card'
  ]
  for (const tag of allowed) {
    assert.deepEqual(findViolations(sfc('  <' + tag + ' />')), [], tag + ' 属放行面')
  }
})

test('负例：封装层内部路径整体放行（内部本来就是 EP 控件）', () => {
  const src = sfc('  <el-dialog /><el-button /><el-tooltip />')
  assert.ok(isAllowedPath('frontend/src/components/ui/UiDialog.vue'))
  assert.deepEqual(scanSource(src, 'frontend/src/components/ui/UiDialog.vue'), [])
  assert.equal(scanSource(src, 'frontend/src/pages/admin/AISettings.vue').length, 3)
})

test('负例：<script> 与 <style> 块不进判定（字符串 / 类名选择器 / 注释）', () => {
  const src = [
    '<template>',
    '  <div class="cc-actions-card" />',
    '</template>',
    '',
    '<script setup lang="ts">',
    "const cls = 'el-button--danger'",
    "const hint = '<el-dialog>'",
    '</script>',
    '',
    '<style scoped>',
    '.cc-actions-card .el-button { margin: 0; }',
    '/* <el-tag> 旧写法 */',
    '</style>'
  ].join('\n')
  assert.deepEqual(findViolations(src), [])
})

test('负例：模板内 HTML 注释（单行 / 多行 / 同行前后各一段）不报', () => {
  const src = sfc(
    [
      '  <!-- <el-dialog> 旧写法 -->',
      '  <!--',
      '    <el-button>批量</el-button>',
      '  -->',
      '  <span>ok</span><!-- <el-tag> -->',
      '  <div><!-- <el-tooltip /> --></div>'
    ].join('\n')
  )
  assert.deepEqual(findViolations(src), [])
})

test('边界：精确 tag 匹配（前缀相同但非守卫集的自定义标签不报）', () => {
  assert.deepEqual(findViolations(sfc('  <el-dialog-custom />')), [])
  assert.deepEqual(findViolations(sfc('  <el-tagx />')), [])
})

test('边界：一行多处全报，自闭合与多行属性展开都能定位到标签所在行', () => {
  const src = sfc(
    [
      '  <div>',
      '    <el-button>取消</el-button><el-tag>新</el-tag>',
      '    <el-dialog',
      '      v-model="visible"',
      '      title="标题"',
      '    >',
      '    </el-dialog>',
      '    <el-tooltip content="提示" />',
      '  </div>'
    ].join('\n')
  )
  const violations = findViolations(src)
  assert.deepEqual(
    violations.map((v) => [v.tag, v.line]),
    [
      ['el-button', 3],
      ['el-tag', 3],
      ['el-dialog', 4],
      ['el-tooltip', 9]
    ]
  )
})

test('边界：具名插槽的 </template> 不提前收尾（根模板取最后一个闭合标签）', () => {
  // 现网先例：pages/admin/RecruiterManage.vue 的插槽闭合写在列 0，
  // 早先的实现会在那里收尾，把文件后半段模板整段漏检（假绿）。
  const src = [
    '<template>',
    '  <div>',
    '    <UiCard>',
    '      <template #footer>',
    '        <span>页脚</span>',
    '      </template>',
    '    </UiCard>',
    '    <el-dialog />', // 在插槽闭合之后 —— 必须被扫到
    '  </div>',
    '</template>'
  ].join('\n')
  const violations = findViolations(src)
  assert.deepEqual(
    violations.map((v) => [v.tag, v.line]),
    [['el-dialog', 8]]
  )
})

test('边界：根模板不在文件开头（script 在前）也能定位', () => {
  const src = [
    '<script setup lang="ts">',
    "const hint = '<el-dialog>'", // script 里的字符串不算
    '</script>',
    '',
    '<template>',
    '  <el-button>提交</el-button>',
    '</template>'
  ].join('\n')
  assert.deepEqual(
    findViolations(src).map((v) => [v.tag, v.line]),
    [['el-button', 6]]
  )
})

test('边界：无根模板（纯 script 组件）无判定面', () => {
  assert.deepEqual(findViolations('<script setup>\nconst a = 1\n</script>'), [])
})

test('parseAddedLines：只取新增行，删除行与上下文行不进集合', () => {
  const diff = [
    'diff --git a/frontend/src/pages/x.vue b/frontend/src/pages/x.vue',
    '--- a/frontend/src/pages/x.vue',
    '+++ b/frontend/src/pages/x.vue',
    '@@ -10,2 +10,3 @@',
    ' 上下文',
    '-  <el-dialog />',
    '+  <el-button>新</el-button>',
    '+  <el-tag>新</el-tag>',
    '+  <span>o</span>'
  ].join('\n')
  const added = parseAddedLines(diff)
  assert.deepEqual([...added.get('frontend/src/pages/x.vue')].sort((a, b) => a - b), [11, 12, 13])
})

test('集成：CLI 全量模式在当前工作树上为绿（守卫不假红）', () => {
  const out = execFileSync(process.execPath, [resolve(ROOT, 'scripts/check-el-controls.mjs'), '--all'], {
    cwd: ROOT,
    encoding: 'utf8'
  })
  assert.match(out, /无违规/)
})
