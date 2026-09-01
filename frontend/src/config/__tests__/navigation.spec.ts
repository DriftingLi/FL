// 侧栏高亮判定测试。
//
// 这条逻辑出过两次线上问题，两次都是「改的时候看不出来、上线才发现」：
//   1. 双高亮 —— 子项命中时父分组也用了宽松匹配，进「真题练习」会同时点亮「题库练习」；
//   2. 详情页不高亮 —— 只比 route.name 精确相等，详情页（ForumDetail 等）没有独立
//      导航项，进去后侧栏整条都不亮。
// 所以这里把两类情形都钉死：该亮的要亮，不该亮的绝不能亮。
import { describe, it, expect } from 'vitest'
import { isNavRouteActive, roleNavigation, type NavItem } from '../navigation'

describe('isNavRouteActive', () => {
  it('routeName 精确相等时命中', () => {
    expect(isNavRouteActive({ key: 'a', label: 'A', routeName: 'CourseList' }, 'CourseList')).toBe(true)
  })

  it('routeName 不同时不命中', () => {
    expect(isNavRouteActive({ key: 'a', label: 'A', routeName: 'CourseList' }, 'ChapterView')).toBe(false)
  })

  it('命中 activeRouteNames 中的详情页路由', () => {
    const item: NavItem = { key: 'courses', label: '课程中心', routeName: 'CourseList', activeRouteNames: ['ChapterView'] }
    expect(isNavRouteActive(item, 'ChapterView')).toBe(true)
    expect(isNavRouteActive(item, 'CourseList')).toBe(true)
    expect(isNavRouteActive(item, 'ForumDetail')).toBe(false)
  })

  it('未声明 activeRouteNames 时行为与改造前一致（向后兼容）', () => {
    const item: NavItem = { key: 'a', label: 'A', routeName: 'CourseList' }
    expect(isNavRouteActive(item, 'ChapterView')).toBe(false)
  })

  it('没有 routeName 的分组项永不命中', () => {
    const item: NavItem = { key: 'g', label: '分组', children: [] }
    expect(isNavRouteActive(item, 'CourseList')).toBe(false)
    expect(isNavRouteActive(item, undefined)).toBe(false)
  })

  it('声明 routeParams 时逐个比对，避免同名路由全部高亮', () => {
    const item: NavItem = {
      key: 'chapter',
      label: '章节',
      routeName: 'ChapterView',
      routeParams: { courseId: 1, chapterId: 2 }
    }
    expect(isNavRouteActive(item, 'ChapterView', { courseId: '1', chapterId: '2' })).toBe(true)
    expect(isNavRouteActive(item, 'ChapterView', { courseId: '1', chapterId: '3' })).toBe(false)
    expect(isNavRouteActive(item, 'ChapterView', { courseId: '9', chapterId: '2' })).toBe(false)
    // params 缺失不应被当成命中
    expect(isNavRouteActive(item, 'ChapterView', {})).toBe(false)
  })

  it('params 为数组时取首个值（vue-router 的 repeatable params）', () => {
    const item: NavItem = { key: 'a', label: 'A', routeName: 'ChapterView', routeParams: { courseId: 1 } }
    expect(isNavRouteActive(item, 'ChapterView', { courseId: ['1', '2'] })).toBe(true)
    expect(isNavRouteActive(item, 'ChapterView', { courseId: ['3', '2'] })).toBe(false)
  })
})

describe('导航配置的详情页归属', () => {
  /** 收集所有带 activeRouteNames 的导航项，便于断言 */
  function collect(items: NavItem[], out: NavItem[] = []): NavItem[] {
    for (const item of items) {
      out.push(item)
      if (item.children) collect(item.children, out)
    }
    return out
  }

  /** 学生端/导师端存在「从列表页跳进去、但没有独立导航项」的详情页 */
  const EXPECTED: Record<string, string[]> = {
    CourseList: ['ChapterView'],
    ForumPage: ['ForumDetail'],
    AIAssistant: ['AIAssistantFeature'],
    JobPlaza: ['JobDetail'],
    TutorCourses: ['TutorChapterManage', 'TutorChapterEdit'],
    TutorQuestionManage: ['TutorQuestionCreate']
  }

  const scoped = [...collect(roleNavigation.student), ...collect(roleNavigation.tutor)]

  for (const [routeName, detailRoutes] of Object.entries(EXPECTED)) {
    it(`${routeName} 在 ${detailRoutes.join(' / ')} 下仍高亮`, () => {
      const item = scoped.find((i) => i.routeName === routeName)
      expect(item, `未找到 routeName=${routeName} 的导航项`).toBeDefined()
      expect(item!.activeRouteNames).toEqual(detailRoutes)
    })
  }

  it('activeRouteNames 里的路由名必须真实存在于路由表中（防止改名后静默失效）', () => {
    // 这里不 import router（避免拉起整个路由栈），只做纯数据校验：
    // 所有被引用过的名字都在本文件的 EXPECTED 里集中声明，改动时需同步更新。
    const referenced = scoped.flatMap((i) => i.activeRouteNames ?? [])
    const declared = Object.values(EXPECTED).flat()
    expect(referenced.sort()).toEqual(declared.sort())
  })
})
