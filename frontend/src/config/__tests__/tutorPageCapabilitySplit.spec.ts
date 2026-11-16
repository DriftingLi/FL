// 讲师域页键拆分锁（#2595 / 规格 A3、A4）。
//
// 阶段一拆键的另一半在前端：后端把 tutor.access 拆成三把之后，页面描述符若还全挂入口键，
// 就出现「后端已能表达只读讲师，前端却把所有讲师页当一个整体放行」的错位——路由守卫按
// page.capability 判定（router/guard.ts），错位在这里就是权限面的错位。
//
// 同时锁住 A4 那处**既有**缺陷：讲师端「标签管理」页声明 question.author，而后端标签接口
// （training/handler_admin.go 的 tagG）要的是 catalog.author。这是拆键前就存在的不一致，
// 本票顺带修掉——它有测试才不会下次再漂回去。
import { describe, expect, it } from 'vitest'
import { pages } from '@/config/pages'

/** 讲师域页 → 它必须声明的能力键（spec A3 + A4）。 */
const EXPECTED: Record<string, string> = {
  // 读面：入口 + 课程/章节读
  TutorDashboard: 'tutor.access',
  TutorCourses: 'tutor.access',
  TutorChapterManage: 'tutor.access',

  // 写面：章节编辑（保存正文/提交审核/传附件）独立成一键
  TutorChapterEdit: 'course.author',

  // 文件库整组独立成一键
  TutorFileLibrary: 'tutorfile.manage',

  // 题库面沿用原有键
  TutorQuestionManage: 'question.author',
  TutorQuestionCreate: 'question.author',

  // A4：与后端 tagG 的 catalog.author 对齐（此前误写成 question.author）
  TutorQuestionTags: 'catalog.author'
}

describe('讲师域页键拆分', () => {
  it('讲师域每个页面声明了预期的能力键', () => {
    const tutorPages = pages.filter(p => p.workspace === 'tutor')
    expect(tutorPages.length).toBeGreaterThan(0)

    for (const page of tutorPages) {
      const want = EXPECTED[page.name]
      expect(want, `讲师域页 ${page.name} 未在本锁的清单里——新页面请先确认它该挂哪个键`).toBeTruthy()
      expect(page.capability, `页面 ${page.name} 的能力键`).toBe(want)
    }
  })

  it('清单里的页都存在（页面被删/改名时本锁要跟着改，不能静默失效）', () => {
    const names = new Set(pages.map(p => p.name as string))
    for (const name of Object.keys(EXPECTED)) {
      expect(names.has(name), `清单里的页面 ${name} 在 pages.ts 里不存在`).toBe(true)
    }
  })

  it('标签管理页与后端守卫同键：catalog.author（A4 的回归网）', () => {
    // 单独再断言一次，是因为这处错位曾经存在过且不会抛错——它只是让页面在讲师面前消失
    // （守卫按 question.author 放行，后端却按 catalog.author 403）。
    const tags = pages.find(p => p.name === 'TutorQuestionTags')
    expect(tags?.capability).toBe('catalog.author')
  })

  it('入口键不再盖住写面与文件库（拆键的核心判据）', () => {
    // 三个写/库页面若仍挂 tutor.access，等于本票没做——入口键一枚就能同时拿到读、写、文件库。
    const writeOrLib = ['TutorChapterEdit', 'TutorFileLibrary']
    for (const name of writeOrLib) {
      const page = pages.find(p => p.name === name)
      expect(page?.capability, `${name} 不应再挂入口键`).not.toBe('tutor.access')
    }
  })
})
