// 「新增行解析」单点实现的表驱动单测（ADR-0053 §10 / spec #1053）。
//
// 这一层是**判据的判据**：它错了，两个守卫都会静默漏扫（假绿）。所以边界要逐个钉：
// 普通路径 / 含非 ASCII 的转义路径（八进制 → UTF-8）/ 新增与删除行混合 / 空 diff /
// 上下文行推进行号 / /dev/null（删除文件）。
import { test } from 'node:test'
import assert from 'node:assert/strict'
import { addedLineList, decodeGitPath, parseAddedLines } from './added-lines.mjs'

test('普通路径：逐行走 hunk，新增行号正确', () => {
  const diff = [
    'diff --git a/frontend/src/pages/x.vue b/frontend/src/pages/x.vue',
    '--- a/frontend/src/pages/x.vue',
    '+++ b/frontend/src/pages/x.vue',
    '@@ -10,0 +11,3 @@',
    '+import { unwrappedRequest } from "@/api/request"',
    '+const a = 1',
    '+const b = 2'
  ].join('\n')
  const added = parseAddedLines(diff)
  assert.deepEqual([...added.get('frontend/src/pages/x.vue')], [11, 12, 13])
  assert.deepEqual(addedLineList(diff), [
    'frontend/src/pages/x.vue:11',
    'frontend/src/pages/x.vue:12',
    'frontend/src/pages/x.vue:13'
  ])
})

test('非 ASCII 路径（core.quotepath 转义形态）必须解码成真实路径', () => {
  // git 对 "文档/说明.md" 的输出：八进制转义 + 外层引号
  const quoted = '"b/\\346\\226\\207\\346\\241\\243/\\350\\257\\264\\346\\230\\216.md"'
  assert.equal(decodeGitPath(quoted), 'b/文档/说明.md')

  const diff = [
    'diff --git "a/文档/说明.md" "b/文档/说明.md"',
    '--- "a/文档/说明.md"',
    '+++ ' + quoted,
    '@@ -1 +1,2 @@',
    ' 已有行',
    '+新增行'
  ].join('\n')
  const added = parseAddedLines(diff)
  assert.deepEqual([...added.keys()], ['文档/说明.md'], '转义路径必须还原（否则 open 静默失败 = 漏扫）')
  assert.deepEqual(addedLineList(diff), ['文档/说明.md:2'])
})

test('转义解码：单个八进制、常见 C 转义、普通字符混排', () => {
  assert.equal(decodeGitPath('"\\346\\234\\252"'), '未')
  assert.equal(decodeGitPath('"a\\tb"'), 'a\tb')
  assert.equal(decodeGitPath('"a\\\\b"'), 'a\\b')
  assert.equal(decodeGitPath('plain/path.md'), 'plain/path.md')
  assert.equal(decodeGitPath('"b/ascii-only.md"'), 'b/ascii-only.md')
})

test('新增与删除行混合：删除行不推进新侧行号', () => {
  const diff = [
    '+++ b/a.vue',
    '@@ -5,3 +5,2 @@',
    '-被删的行', // 不推进
    ' 上下文行', // 推进到 5→6
    '+新增在 6',
    ' 上下文行 2' // 6→7
  ].join('\n')
  assert.deepEqual(addedLineList(diff), ['a.vue:6'])
})

test('空 diff / 只有文件头：产出为空', () => {
  assert.deepEqual(addedLineList(''), [])
  assert.deepEqual(addedLineList('diff --git a/x b/x\n--- a/x\n+++ b/x\n'), [])
})

test('删除文件（/dev/null）：不进新增行面，且不污染后续文件', () => {
  const diff = [
    '+++ /dev/null',
    '@@ -1 +0,0 @@',
    '-整份删掉',
    '+++ b/keep.ts',
    '@@ -0,0 +1,1 @@',
    '+第一行'
  ].join('\n')
  assert.deepEqual(addedLineList(diff), ['keep.ts:1'])
})

test('多文件：行号各自独立累计', () => {
  const diff = [
    '+++ b/a.ts',
    '@@ -0,0 +1,2 @@',
    '+a1',
    '+a2',
    '+++ b/b.ts',
    '@@ -3,0 +4,1 @@',
    '+b4'
  ].join('\n')
  assert.deepEqual(addedLineList(diff), ['a.ts:1', 'a.ts:2', 'b.ts:4'])
})

test('"\\ No newline at end of file" 不推进行号', () => {
  const diff = ['+++ b/a.ts', '@@ -1 +1,2 @@', '+第一行', '\\ No newline at end of file', '+第二行'].join('\n')
  assert.deepEqual(addedLineList(diff), ['a.ts:1', 'a.ts:2'])
})
