// 讲师端落点与四预置角色的可见面（#2600 / 规格 F6、AC「四预置角色实际可见面」）。
//
// 为什么需要这组锁：讲师自 #2600 起是动态角色，可见面完全由所挂角色的能力集回答。
// 落点若与「侧栏过滤」用了两套判据，就会出现「守卫把讲师送到一页它自己看不见的页面」
// ——管理端 #1638 踩过的那种回弹空白页。这里逐条钉住「能力集 → 落点」与「能力集 → 侧栏叶子」，
// 并让两者用**同一个** `holds` 谓词构造，与生产侧 guard.ts / TutorLayout.vue 的用法一致。
import { describe, expect, it } from 'vitest'
import {
  TUTOR_DASHBOARD,
  TUTOR_NO_ACCESS,
  buildNavigation,
  filterNavByCapability,
  tutorLandingPage,
  type NavItem
} from '@/config/navigation'

/** 用一组能力构造 holds 谓词（与生产侧 holdsCapability 的入参形状一致）。 */
function holds(capabilities: string[]): (capability: string) => boolean {
  const set = new Set(capabilities)
  return capability => set.has(capability)
}

/** 侧栏过滤后剩下的叶子路由名（按侧栏顺序）。 */
function visibleLeaves(capabilities: string[]): string[] {
  const collect = (items: NavItem[]): string[] => {
    const out: string[] = []
    for (const item of items) {
      if (item.children?.length) out.push(...collect(item.children))
      else if (item.routeName) out.push(item.routeName)
    }
    return out
  }
  return collect(filterNavByCapability(buildNavigation('tutor'), holds(capabilities)))
}

describe('tutorLandingPage', () => {
  it('有 tutor.access → 仪表盘（与管理端「仪表盘可达即仪表盘」同一条判据）', () => {
    expect(tutorLandingPage(holds(['tutor.access']))).toBe(TUTOR_DASHBOARD)
    expect(tutorLandingPage(holds(['tutor.access', 'question.author']))).toBe(TUTOR_DASHBOARD)
  })

  it('空集 → 无讲师权限页（不是仪表盘，否则落在自己看不见的页上）', () => {
    expect(tutorLandingPage(holds([]))).toBe(TUTOR_NO_ACCESS)
  })

  it('没有入口键但有其它能力 → 按侧栏顺序取第一个可达叶子', () => {
    // 只有题库写权：侧栏顺序里题库管理（order 3）先于标签管理（order 4）
    expect(tutorLandingPage(holds(['question.author']))).toBe('TutorQuestionManage')
    // 只有标签管理权
    expect(tutorLandingPage(holds(['catalog.author']))).toBe('TutorQuestionTags')
    // 只有文件库
    expect(tutorLandingPage(holds(['tutorfile.manage']))).toBe('TutorFileLibrary')
  })

  it('能力位只覆盖非导航页 → 落无权限页（侧栏里没有可达叶子）', () => {
    // course.author 只挂在 TutorChapterEdit 上，而那是带参详情页、没有 nav 项：
    // 按 F6「按侧栏导航顺序挑」它不该成为落点，只能落无权限页。
    expect(tutorLandingPage(holds(['course.author']))).toBe(TUTOR_NO_ACCESS)
  })

  it('落点永远是侧栏可达页或无权限页（不落到看不见的页面）', () => {
    const samples = [
      ['tutor.access'],
      ['question.author'],
      ['catalog.author'],
      ['tutorfile.manage'],
      ['course.author'],
      ['tutor.access', 'course.author', 'tutorfile.manage', 'question.author', 'catalog.author'],
      []
    ]
    for (const caps of samples) {
      const landing = tutorLandingPage(holds(caps))
      const leaves = visibleLeaves(caps)
      expect(
        landing === TUTOR_NO_ACCESS || leaves.includes(landing),
        `能力集 ${JSON.stringify(caps)} 的落点 ${landing} 不在它自己的侧栏里（${leaves.join(',')}）`
      ).toBe(true)
    }
  })
})

describe('四预置角色的实际可见面（#2600 AC）', () => {
  // 四模板的能力集取自后端 authz.TutorTemplateRoles()（规格 E6）；
  // 这里断言的是「这些能力位在侧栏里具体打开哪几页」，后端只回答能力、不回答页面。
  const FULL = ['tutor.access', 'course.author', 'tutorfile.manage', 'question.author', 'contribution.review', 'catalog.author', 'notification.use']
  const BANK = ['tutor.access', 'question.author', 'catalog.author', 'notification.use']
  const COURSE = ['tutor.access', 'course.author', 'tutorfile.manage', 'notification.use']
  const READONLY = ['tutor.access', 'notification.use']

  it('全权讲师：全部讲师页', () => {
    expect(visibleLeaves(FULL)).toEqual([
      'TutorDashboard',
      'TutorCourses',
      'TutorQuestionManage',
      'TutorQuestionTags',
      'TutorFileLibrary'
    ])
  })

  it('题库讲师：仪表盘 + 我的课程 + 题库管理 + 标签管理（没有文件库）', () => {
    expect(visibleLeaves(BANK)).toEqual([
      'TutorDashboard',
      'TutorCourses',
      'TutorQuestionManage',
      'TutorQuestionTags'
    ])
  })

  it('课程讲师：仪表盘 + 我的课程 + 文件库（没有题库与标签）', () => {
    expect(visibleLeaves(COURSE)).toEqual(['TutorDashboard', 'TutorCourses', 'TutorFileLibrary'])
  })

  it('只读讲师：仪表盘 + 我的课程', () => {
    expect(visibleLeaves(READONLY)).toEqual(['TutorDashboard', 'TutorCourses'])
  })

  it('未挂角色：侧栏为空，落无权限页', () => {
    expect(visibleLeaves([])).toEqual([])
    expect(tutorLandingPage(holds([]))).toBe(TUTOR_NO_ACCESS)
  })

  it('四模板的落点都是仪表盘（它们都含 tutor.access）', () => {
    for (const caps of [FULL, BANK, COURSE, READONLY]) {
      expect(tutorLandingPage(holds(caps))).toBe(TUTOR_DASHBOARD)
    }
  })
})
