// 「模板不得内联状态文案裸串」静态扫描（#1103 / ADR-0056 §8）。
//
// 做法：把 SFC 的 <template> 编译成渲染函数，取其中出现过的字符串字面量——
// 注释与 <script> 段天然不在编译产物里，插值与属性里的字面量则一视同仁。
// 命中 descriptor 文案即红，逼消费面走 descriptor。
import { describe, it, expect } from 'vitest'
import { readFileSync } from 'node:fs'
import { resolve } from 'node:path'
import { parse, compileTemplate } from 'vue/compiler-sfc'
import { CONTACT_REQUEST_STATUSES, describeContactRequest } from '../contactRequestStatus'
import { APPLICATION_STATUSES, describeApplication } from '../applicationStatus'

/** frontend/src（本文件在 src/utils/__tests__ 下）。 */
const SRC = resolve(__dirname, '../..')

/** 票面「涉及文件」里的模板面；换名 / 搬走必须同步这里（路径不存在即读取失败，不会静默漏扫）。 */
const PAGES = [
  'pages/student/ResumePage.vue',
  'pages/student/MyApplications.vue',
  'pages/recruit/MyRequests.vue',
  'pages/recruit/ApplicationList.vue',
  'pages/recruit/Resumes.vue',
  'pages/recruit/ResumeDetail.vue',
  'pages/admin/Inspection.vue'
]

/** 状态词表（descriptor 文案）——模板里只能经 descriptor 出，写成裸串即红。 */
const STATUS_WORDS = [
  ...new Set([
    ...CONTACT_REQUEST_STATUSES.map((s) => describeContactRequest(s).label),
    ...APPLICATION_STATUSES.map((s) => describeApplication(s).label)
  ])
]

/**
 * 统一前的漂移文案（#1103 动作 4）。「待处理」是通用词（举报队列等相邻域还在用），
 * 所以只在**当初写错的这 4 个消费面**里禁回流，不做全站扫。
 */
const RETIRED: Record<string, string[]> = {
  'pages/student/ResumePage.vue': ['待处理'],
  'pages/student/MyApplications.vue': ['待处理'],
  'pages/recruit/MyRequests.vue': ['待处理'],
  'pages/recruit/Resumes.vue': ['已授权', '待学员确认']
}

/** 模板编译后的字符串字面量（双引号 / 单引号两种形态）。 */
function templateLiterals(src: string): string[] {
  const { descriptor } = parse(src)
  const { code } = compileTemplate({ source: descriptor.template?.content ?? '', id: 'status-word-scan', filename: 'status-word-scan.vue' })
  return [...code.matchAll(/"([^"\\]*)"|'([^'\\]*)'/g)].map((m) => (m[1] ?? m[2]) as string)
}

const inlineHits = (file: string, words: string[]) =>
  templateLiterals(readFileSync(resolve(SRC, file), 'utf8')).filter((literal) => words.includes(literal))

describe('模板不得内联状态文案（#1103）', () => {
  for (const page of PAGES) {
    it(`${page} 的状态文案只经 descriptor`, () => {
      expect(inlineHits(page, STATUS_WORDS)).toEqual([])
    })
  }

  for (const [page, words] of Object.entries(RETIRED)) {
    it(`${page} 不得回流统一前的漂移文案`, () => {
      expect(inlineHits(page, words)).toEqual([])
    })
  }

  it('负向探针：模板里注入一条状态文案裸串 / 一条旧文案，扫描必红', () => {
    expect(templateLiterals('<template><UiTag>待同意</UiTag></template>')).toContain('待同意')
    expect(templateLiterals('<template><UiTag>待处理</UiTag></template>')).toContain('待处理')
  })
})
