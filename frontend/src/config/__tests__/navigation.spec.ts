// 侧栏高亮判定测试。
//
// 这条逻辑出过两次线上问题，两次都是「改的时候看不出来、上线才发现」：
//   1. 双高亮 —— 子项命中时父分组也用了宽松匹配，进「真题练习」会同时点亮「题库练习」；
//   2. 详情页不高亮 —— 只比 route.name 精确相等，详情页（ForumDetail 等）没有独立
//      导航项，进去后侧栏整条都不亮。
// 所以这里把两类情形都钉死：该亮的要亮，不该亮的绝不能亮。
//
// 第十三波 票9 补的是同一族里**当时还只写在注释上**的四条判据：
//   3. `exact` 已不参与判定（name 匹配天然精确，别再拿它当「前缀开关」）；
//   4. 命中唯一性（双高亮的正面表述：同一路由下兄弟项里至多一项亮）；
//   5. 两层深度上限（flattenLeaves 与 isGroupActive 都不看第三层往下的子孙）；
//   6. 分组高亮的结构性不对称（isGroupActive 只看子孙，分组行自己的 routeName 不算）。
import { describe, it, expect } from 'vitest'
import {
  isNavRouteActive,
  isGroupExpanded,
  toggleGroupExpanded,
  flattenLeaves,
  isGroupActive,
  type NavItem
} from '../navigation'
import type { RouteName } from '../pages'

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

  it('当前路由尚未解析出来（undefined / null / 空串）时，配了详情页归属的项也不能亮', () => {
    const item: NavItem = { key: 'a', label: 'A', routeName: 'CourseList', activeRouteNames: ['ChapterView'] }
    expect(isNavRouteActive(item, undefined)).toBe(false)
    expect(isNavRouteActive(item, null)).toBe(false)
    expect(isNavRouteActive(item, '')).toBe(false)
  })

  it('exact 只是历史标记，不再参与判定（name 匹配天然精确，无前缀放宽也无额外收紧）', () => {
    const exactLeaf: NavItem = { key: 'a', label: 'A', routeName: 'ForumPage', exact: true }
    expect(isNavRouteActive(exactLeaf, 'ForumPage')).toBe(true)
    // 前缀匹配早在描述符改造里取消了：同前缀的兄弟路由不被带动
    expect(isNavRouteActive(exactLeaf, 'ForumDetail')).toBe(false)
    // exact 也不改变详情页归属的判定（配了就算，不因为它为 true 而作废）
    const exactWithDetail: NavItem = {
      key: 'b',
      label: 'B',
      routeName: 'ForumPage',
      activeRouteNames: ['ForumDetail'],
      exact: true
    }
    expect(isNavRouteActive(exactWithDetail, 'ForumDetail')).toBe(true)
  })

  it('双高亮的正面表述：同一路由下兄弟项里至多一项命中（含详情页归属）', () => {
    // 「真题练习 / 题库练习」当年的形状：两项都吃宽松匹配就会同时点亮
    const siblings: NavItem[] = [
      { key: 'practice', label: '题库练习', routeName: 'QuestionBank' },
      { key: 'real', label: '真题练习', routeName: 'RealExamPractice' },
      { key: 'notes', label: '学习笔记', routeName: 'StudentNotebook', activeRouteNames: ['StudentFeaturedDetail'] }
    ]
    const atMostOne = (name: string, params?: Record<string, string>) => {
      const hits = siblings.filter(i => isNavRouteActive(i, name, params)).map(i => i.key)
      expect(hits.length, `路由 ${name} 同时点亮了 ${hits.join(' / ')}`).toBeLessThanOrEqual(1)
    }
    for (const name of ['QuestionBank', 'RealExamPractice', 'StudentNotebook', 'StudentFeaturedDetail', 'WrongQuestions', 'MockExam']) {
      atMostOne(name)
    }
    // 逐项核对确实各归各：列表页亮列表项、详情页亮同一项，别的不亮
    expect(siblings.filter(i => isNavRouteActive(i, 'StudentFeaturedDetail')).map(i => i.key)).toEqual(['notes'])
    expect(siblings.filter(i => isNavRouteActive(i, 'QuestionBank')).map(i => i.key)).toEqual(['practice'])
    expect(siblings.filter(i => isNavRouteActive(i, 'RealExamPractice')).map(i => i.key)).toEqual(['real'])
    expect(siblings.filter(i => isNavRouteActive(i, 'WrongQuestions'))).toEqual([])
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

  it('分组高亮只看子孙：分组行自己的 routeName 不算（它是行，不是分组）', () => {
    // 侧栏里分组行的高亮走 isNavRouteActive，分组亮不亮走 isGroupActive —— 两条不能互相顶包
    const group: NavItem = { key: 'g', label: '题库练习', routeName: 'QuestionBank', children: [leaf('r', 'RealExamPractice')] }
    expect(isGroupActive(group, 'QuestionBank')).toBe(false)
    expect(isNavRouteActive(group, 'QuestionBank')).toBe(true)
    expect(isGroupActive(group, 'RealExamPractice')).toBe(true)
  })

  it('两层深度上限：第三层往下的子孙既不点亮分组，也不进 tooltip 清单', () => {
    // 侧栏结构只到三层（分组 ┬ 项 ┬ 子项）；再深的层级属数据表越界，判据不跟着无限递归
    const deep: NavItem = {
      key: 'g',
      label: '分组',
      children: [
        {
          key: 'mid',
          label: '二级分组',
          children: [
            { key: 'third', label: '三级项', routeName: 'WrongQuestions', children: [leaf('fourth', 'ForumDetail')] }
          ]
        }
      ]
    }
    expect(isGroupActive(deep, 'WrongQuestions')).toBe(true) // 第三层是「一级子级的子级」，在判据内
    expect(isGroupActive(deep, 'ForumDetail')).toBe(false) // 第四层不参与
    expect(flattenLeaves(deep).map(i => i.key)).toEqual(['third'])
  })

  it('flattenLeaves 只收子孙：分组行自身即使可点也不进 tooltip（点了没意义，它一直在侧栏上）', () => {
    const group: NavItem = {
      key: 'g',
      label: '题库练习',
      routeName: 'QuestionBank',
      children: [{ key: 'c1', label: '外部题源', externalUrl: 'https://bank.example.net' }]
    }
    expect(flattenLeaves(group).map(i => i.key)).toEqual(['c1'])
  })

  it('展开态按 key 记账：切换一个分组不牵连其它分组', () => {
    const map = { a: true, b: false, c: true }
    const next = toggleGroupExpanded(map, 'a')
    expect(next).toEqual({ a: false, b: false, c: true })
    expect(isGroupExpanded(next, 'b')).toBe(false)
    expect(isGroupExpanded(next, 'c')).toBe(true)
    expect(isGroupExpanded(next, 'unregistered')).toBe(true)
  })
})
