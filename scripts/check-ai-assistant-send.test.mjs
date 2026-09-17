// AI 助手发送编排守卫的抽取式单测（ADR-0056 §10 / issue #1104）。
//
// 守卫是「规则的执行面」，它自己坏了必须报红 —— 否则规则退化为假绿。
// 运行：node --test scripts/check-ai-assistant-send.test.mjs
//
// 判定面只认 pages/ai-assistant 下 .vue/.ts 里的两个「发送编排入口」名字
// （sendMessage / streamChat），这组用例把这些边界钉住。
import { test } from 'node:test'
import assert from 'node:assert/strict'
import { spawnSync } from 'node:child_process'
import { dirname, resolve } from 'node:path'
import { fileURLToPath } from 'node:url'
import {
  ALLOWLIST,
  GUARDED_PATH_SEGMENT,
  GUARDED_TOKENS,
  GUARD_SPEC,
  isGuardedPath,
  isTestFile,
  scanSource
} from './check-ai-assistant-send.mjs'

const ROOT = resolve(dirname(fileURLToPath(import.meta.url)), '..')
const PAGE = 'frontend/src/pages/ai-assistant/AIAssistantPage.vue'

test('正例：页面里重新起流式编排（streamChat + handlers）报出，行号精确', () => {
  const src = [
    "import { aiAssistantApi } from '@/api/aiAssistant'",
    'const controller = aiAssistantApi.streamChat(req, {',
    "  onChunk: (c) => { content.value += c },",
    "  onDone: () => { streaming.value = false }",
    '})'
  ].join('\n')
  const violations = scanSource(src, PAGE)
  assert.equal(violations.length, 1)
  assert.equal(violations[0].line, 2)
  assert.equal(violations[0].token, 'streamChat')
})

test('正例：旧 store 发送编排入口（sendMessage）报出 —— #1104 已更名为 send', () => {
  const v = scanSource("await store.sendMessage(text)", PAGE)
  assert.equal(v.length, 1)
  assert.equal(v[0].token, 'sendMessage')
})

test('负例：收编后的单次委派（store.send）不报', () => {
  const src = [
    'const text = inputText.value.trim()',
    "inputText.value = ''",
    'await store.send(text)',
    'await store.send(buildContent(text), opts)',
    'await store.retryLastTurn()'
  ].join('\n')
  assert.deepEqual(scanSource(src, PAGE), [])
})

test('负例：形近名不误报（resendMessage / streamChatter / chatStream）', () => {
  const src = [
    'const resendMessage = 1',
    'const streamChatter = 2',
    'const chatStream = 3',
    'obj.sendMessageLater()'
  ].join('\n')
  assert.deepEqual(scanSource(src, PAGE), [])
})

test('负例：注释是说明不是调用（整行注释不算判定对象）', () => {
  const src = [
    '// 旧实现：await store.sendMessage(text)',
    ' * 收编前这里调 streamChat（见 #1104）',
    '/* streamChat(req, handlers) */'
  ].join('\n')
  assert.deepEqual(scanSource(src, PAGE), [])
})

test('负例：测试文件整体不进判定面（重试路径用例要替身 streamChat）', () => {
  const src = 'streamChat: mocks.streamChat'
  for (const p of [
    'frontend/src/pages/ai-assistant/__tests__/AIAssistantPage.spec.ts',
    'frontend/src/pages/ai-assistant/__tests__/aiAssistantRetryPath.spec.ts',
    'frontend/src/pages/ai-assistant/Foo.test.ts'
  ]) {
    assert.ok(isTestFile(p), p + ' 应判为测试文件')
    assert.equal(isGuardedPath(p), false, p + ' 不进守卫面')
    assert.deepEqual(scanSource(src, p), [])
  }
})

test('负例：编排宿主不在守卫面（stores / components / 其它页面）', () => {
  for (const p of [
    'frontend/src/stores/aiAssistant.ts',
    'frontend/src/components/ai-assistant/ChatPageShell.vue',
    'frontend/src/pages/admin/X.vue'
  ]) {
    assert.equal(isGuardedPath(p), false, p + ' 不在守卫面')
  }
  assert.equal(isGuardedPath('frontend/src/pages/ai-assistant/FeatureChatPage.vue'), true)
})

test('白名单：例外文件仍在判定面内（豁免由 runner 逐行放行）且理由非空（当前为空表，规则绝对执行）', () => {
  // #1123：豁免从 isGuardedPath 移交给 runner（--all 整体放行 / --diff 只放行基线违规行号）。
  for (const [p, reason] of Object.entries(ALLOWLIST)) {
    assert.equal(isGuardedPath(p), true, p + ' 必须在判定面内（豁免交给 runner 逐行判）')
    assert.ok(typeof reason === 'string' && reason.length > 10, p + ' 的理由要写清')
  }
})

test('判定面契约：GUARD_SPEC 用的就是本文件导出的判定函数（#1094 runner 可直接收）', () => {
  assert.equal(GUARD_SPEC.isGuardedPath, isGuardedPath)
  assert.equal(GUARD_SPEC.scanSource, scanSource)
  assert.equal(GUARD_SPEC.allowlist, ALLOWLIST)
  assert.equal(GUARD_SPEC.name, 'check-ai-assistant-send')
  assert.deepEqual(GUARDED_TOKENS, ['sendMessage', 'streamChat'])
  assert.equal(GUARDED_PATH_SEGMENT, '/pages/ai-assistant/')
  // runner 面（argv 解析 / --diff / 退出码）属 guard.mjs：本文件的 CLI 必须能跑通全量模式
  assert.equal(typeof GUARD_SPEC.all.ok, 'function')
  assert.equal(typeof GUARD_SPEC.diff.ok, 'function')
})

test('端到端：守卫在当前工作树上判绿（两页均只经 store.send 委派）', () => {
  const out = spawnSync(process.execPath, ['scripts/check-ai-assistant-send.mjs', '--all'], {
    cwd: ROOT,
    encoding: 'utf8'
  })
  assert.equal(out.status, 0, out.stdout + out.stderr)
  assert.match(out.stdout, /无违规/)
  // 守卫面非空：守卫坏了（例如把扫描目录写错）时不能靠「一个文件都没扫到」判绿
  assert.match(out.stdout, /[1-9][0-9]* 个页面文件均未出现发送编排/)
})

test('端到端：--diff fail-closed —— base 解析不了即非零退出，绝不落「通过」', () => {
  const out = spawnSync(
    process.execPath,
    ['scripts/check-ai-assistant-send.mjs', '--diff', 'no-such-ref-for-guard-selftest'],
    { cwd: ROOT, encoding: 'utf8' }
  )
  assert.equal(out.status, 2)
  assert.match(out.stderr, /无法解析 base ref/)
})
