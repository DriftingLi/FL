// 侧栏高亮判定测试。
//
// 这条逻辑出过两次线上问题，两次都是「改的时候看不出来、上线才发现」：
//   1. 双高亮 —— 子项命中时父分组也用了宽松匹配，进「真题练习」会同时点亮「题库练习」；
//   2. 详情页不高亮 —— 只比 route.name 精确相等，详情页（ForumDetail 等）没有独立
//      导航项，进去后侧栏整条都不亮。
// 所以这里把两类情形都钉死：该亮的要亮，不该亮的绝不能亮。
import { describe, it, expect } from 'vitest'
import {
  isNavRouteActive,
  isGroupExpanded,
  toggleGroupExpanded,
  flattenLeaves,
  isGroupActive,
  type NavItem
} from '../navigation'
import type { RouteName } from '../routeNames'

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


describe('侧栏分组判定（纯函数）', () => {
  const leaf = (key: string, routeName: RouteName): NavItem => ({ key, label: key, routeName })

  it('分组默认展开，只有显式为 false 才收起', () => {
    expect(isGroupExpanded({}, 'g')).toBe(true)
    expect(isGroupExpanded({ g: true }, 'g')).toBe(true)
    expect(isGroupExpanded({ g: false }, 'g')).toBe(false)
  })

  it('切换分组展开态返回新对象（不原地改，组件据此做响应式更新）', () => {
    const before = { a: true }
    const after = toggleGroupExpanded(before, 'a')
    expect(after).toEqual({ a: false })
    expect(before).toEqual({ a: true })
    expect(toggleGroupExpanded(after, 'a')).toEqual({ a: true })
    expect(toggleGroupExpanded({}, 'g')).toEqual({ g: false })
  })

  it('flattenLeaves 收集两层叶子项，跳过没有 routeName/externalUrl 的容器', () => {
    const group: NavItem = {
      key: 'g',
      label: '分组',
      children: [
        leaf('l1', 'CourseList'),
        { key: 'mid', label: '二级分组', children: [leaf('l2', 'ChapterView'), { key: 'bad', label: '空容器' }] }
      ]
    }
    const got = flattenLeaves(group).map(i => i.routeName)
    expect(got).toEqual(['CourseList', 'ChapterView'])
  })

  it('flattenLeaves 收外链项（externalUrl 视为叶子）', () => {
    const group: NavItem = { key: 'g', label: '分组', children: [{ key: 'x', label: '官网', externalUrl: 'https://www.example.com' }] }
    expect(flattenLeaves(group).map(i => i.key)).toEqual(['x'])
  })

  it('翻页后单层分组激活：任一子项命中即激活', () => {
    const group: NavItem = { key: 'g', label: '分组', children: [leaf('a', 'CourseList'), leaf('b', 'ForumPage')] }
    expect(isGroupActive(group, 'ForumPage')).toBe(true)
    expect(isGroupActive(group, 'NotFoundPage')).toBe(false)
  })

  it('两层分组激活：二层子项命中同样点亮整条分组', () => {
    const group: NavItem = {
      key: 'g',
      label: '分组',
      children: [{ key: 'mid', label: '二级分组', children: [leaf('a', 'TutorChapterEdit')] }]
    }
    expect(isGroupActive(group, 'TutorChapterEdit')).toBe(true)
    expect(isGroupActive(group, 'Other')).toBe(false)
  })

  it('无子项的分组永不激活；详情页归属（activeRouteNames）同样点亮分组', () => {
    expect(isGroupActive(leaf('a', 'CourseList'), 'CourseList')).toBe(false)
    const group: NavItem = {
      key: 'g',
      label: '分组',
      children: [{ key: 'c', label: '课程中心', routeName: 'CourseList', activeRouteNames: ['ChapterView'] }]
    }
    expect(isGroupActive(group, 'ChapterView')).toBe(true)
  })

  it('带 routeParams 的详情项只在参数匹配时点亮（同名不同课程不互相点亮）', () => {
    const item: NavItem = { key: 'a', label: '章节', routeName: 'ChapterView', routeParams: { courseId: 7 } }
    const group: NavItem = { key: 'g', label: '分组', children: [item] }
    expect(isGroupActive(group, 'ChapterView', { courseId: '7' })).toBe(true)
    expect(isGroupActive(group, 'ChapterView', { courseId: '8' })).toBe(false)
  })
})
