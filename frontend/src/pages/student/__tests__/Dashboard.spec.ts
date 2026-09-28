/**
 * 学员工作台首屏（#1355 的第二处销账）：三路并行装载的写回形状。
 *
 * 本页此前的 loader 只是 `await Promise.all([loadCourses(), loadRecentLearning(), loadStudyStats()])`
 * —— 三份状态各自写在 helper 体内（其中统计那路住在 `useRoleDashboard`），R4「loader 不写回」看不见
 * 这个形状，只能登记成 R4c 的例外。#1355 把三条路都改成「取数回传 + `apply` 写回」：
 * 课程/最近学习两路在本页直接回传，统计那路走 `useRoleDashboard` 的 `fetchStats()` + `applyStats()`。
 *
 * 本文件按行为钉两件事：
 * ① 搬迁后的派生判据没变（进行中的课程取 0<progress<100、最近学习按课程去重取前 5、概览指标读统计总数）；
 * ② 连发两轮时，三份状态都停在**最新一轮**的读数上 —— 旧形状里写回发生在代数校验之前，
 *    先起飞的慢响应后到会把新筛选/新证件的结果盖掉。
 *
 * seam：网络出口（mock `@/api/student`）+ 图表出口（mock `useECharts`），不触达 API 层与 canvas。
 */
import { describe, it, expect, vi, beforeEach } from 'vitest'
import { mount, flushPromises, type VueWrapper } from '@vue/test-utils'
import { createPinia, setActivePinia } from 'pinia'
import { WarningFilled } from '@element-plus/icons-vue'
import { epLite } from '@/test/element-lite'
import type { StudentCourseItem, StudyRecordItem, StudyStats } from '@/api/student'

const getStudentCourses = vi.fn()
const getRecords = vi.fn()
const getStudyStats = vi.fn()

vi.mock('@/api/student', () => ({
  studentApi: {
    getStudentCourses: (...args: unknown[]) => getStudentCourses(...args),
    getRecords: (...args: unknown[]) => getRecords(...args),
    getStudyStats: (...args: unknown[]) => getStudyStats(...args)
  }
}))

// 图表装配与本票无关（且 echarts 在 jsdom 下需要真实容器尺寸）：按 useRoleDashboard.spec.ts 的口径 mock 掉
vi.mock('@/composables/useECharts', () => ({
  useECharts: vi.fn(() => ({ init: vi.fn() }))
}))

import Dashboard from '../Dashboard.vue'

/** script setup 的绑定在 dev 模式经 proxy 可访问（口径同 ValuationHistoryView.spec.ts）。 */
const api = (w: VueWrapper) => w.vm as unknown as Record<string, any>

function courseOf(id: number, name: string, progress: number, over: Partial<StudentCourseItem> = {}): StudentCourseItem {
  return {
    completed_chapters: 0,
    course_id: id,
    course_name: name,
    cover: '',
    last_chapter_id: id * 10,
    last_chapter_title: `${name} 第 1 章`,
    last_position: 0,
    last_studied_at: '2026-09-01T10:00:00+08:00',
    level_id: null,
    progress,
    specialty_id: null,
    study_duration: 0,
    total_chapters: 5,
    ...over
  }
}

function recordOf(id: number, courseId: number, courseName: string): StudyRecordItem {
  return {
    chapter_id: id,
    chapter_title: `第 ${id} 节`,
    course_id: courseId,
    course_name: courseName,
    progress: 50,
    record_id: id,
    student_id: 1,
    study_date: '2026-09-01',
    study_duration: 30
  }
}

function statsOf(totalMinutes: number, activeDays: number): StudyStats {
  return {
    active_days: activeDays,
    data: [totalMinutes, 0, 0],
    days: 3,
    labels: ['d0', 'd1', 'd2'],
    total_minutes: totalMinutes
  }
}

function coursesPayload(courses: StudentCourseItem[], continueLearning: StudentCourseItem | null) {
  return { courses, continue_learning: continueLearning }
}

function mountPage() {
  return mount(Dashboard, {
    global: {
      plugins: [epLite()],
      // UiErrorState 的图标在生产由 main.ts 全局注册；错误态用例需要它，否则 vue 报 unresolved
      components: { WarningFilled },
      stubs: { 'router-link': { props: ['to'], template: '<span><slot /></span>' } }
    }
  })
}

beforeEach(() => {
  vi.clearAllMocks()
  setActivePinia(createPinia())
  getStudentCourses.mockResolvedValue(
    coursesPayload(
      [
        courseOf(1, '液压系统检修', 40),
        courseOf(2, '已完成的课程', 100),
        courseOf(3, '还没开始的课程', 0),
        courseOf(4, '发动机异响排查', 70)
      ],
      courseOf(1, '液压系统检修', 40, { last_chapter_title: '液压泵拆解' })
    )
  )
  // 同一门课占多条学习记录：去重后「最近学习」只出一张
  getRecords.mockResolvedValue({
    records: [recordOf(11, 7, '电气线路识图'), recordOf(12, 7, '电气线路识图'), recordOf(13, 8, '门架升降故障')],
    page: 1,
    pages: 1,
    total: 3
  })
  getStudyStats.mockResolvedValue(statsOf(180, 6))
})

describe('Dashboard 首屏三路装载（#1355 写回槽）', () => {
  it('派生判据搬迁后不变：进行中的课程取 0<progress<100、最近学习按课程去重', async () => {
    const w = mountPage()
    await flushPromises()

    const cards = w.findAll('.quick-card')
    expect(cards).toHaveLength(2)
    const [active, recent] = cards
    expect(active.text()).toContain('液压系统检修')
    expect(active.text()).toContain('发动机异响排查')
    // 100% 与 0% 都不进「进行中的课程」
    expect(active.text()).not.toContain('已完成的课程')
    expect(active.text()).not.toContain('还没开始的课程')
    // 两条同课记录只出一张卡，另一门课正常在列
    expect(recent.findAll('.item-title')).toHaveLength(2)
    expect(recent.text()).toContain('电气线路识图')
    expect(recent.text()).toContain('门架升降故障')
  })

  it('概览指标与「继续学习」入口由同一轮回传写入', async () => {
    const w = mountPage()
    await flushPromises()

    // 180 分钟 / 活跃 6 天 → 日均 30
    expect(api(w).overviewStats).toEqual({ minutes: 180, activeDays: 6, perDay: 30 })
    expect(api(w).continueLearning.course_id).toBe(1)
    expect(w.text()).toContain('继续学习：液压泵拆解')
  })

  it('连发两轮（第一轮慢、第二轮快）：三份状态都停在最新一轮（写回全在 apply、代数校验之后）', async () => {
    let releaseSlow!: (v: unknown) => void
    const slow = new Promise<unknown>(resolve => {
      releaseSlow = resolve
    })
    // 两条取数流各自计数：第一轮（旧证件/旧读数）故意挂在 slow 上，第二轮先落地
    let coursesCalls = 0
    getStudentCourses.mockImplementation(() => {
      coursesCalls++
      return coursesCalls === 1
        ? slow
        : Promise.resolve(coursesPayload([courseOf(9, '第二轮的课程', 20)], null))
    })
    let recordsCalls = 0
    getRecords.mockImplementation(() => {
      recordsCalls++
      return Promise.resolve(
        recordsCalls === 1
          ? { records: [recordOf(11, 7, '第一轮最近学习')], page: 1, pages: 1, total: 1 }
          : { records: [recordOf(21, 9, '第二轮的最近学习')], page: 1, pages: 1, total: 1 }
      )
    })

    const w = mountPage()
    await flushPromises()
    // 第二轮：切证件/点重试都会走这条（这里直接调 run，避开与本页无关的证件上下文搭建）
    await api(w).loadAll()
    await flushPromises()

    const [active, recent] = w.findAll('.quick-card')
    expect(active.text()).toContain('第二轮的课程')
    expect(recent.text()).toContain('第二轮的最近学习')
    expect(api(w).overviewStats.minutes).toBe(180)

    // 第一轮此刻才落地：本轮已作废，三份状态都不被旧读数改写
    releaseSlow(coursesPayload([courseOf(1, '液压系统检修', 40)], courseOf(1, '液压系统检修', 40)))
    await flushPromises()
    expect(active.text()).toContain('第二轮的课程')
    expect(active.text()).not.toContain('液压系统检修')
    expect(recent.text()).not.toContain('第一轮最近学习')
    expect(api(w).continueLearning).toBe(null)
  })

  it('任一路失败即整页错误态（既有语义：课程/最近学习两路把错误上抛），重试走同一条 run', async () => {
    getStudentCourses.mockRejectedValue(new Error('boom'))
    const w = mountPage()
    await flushPromises()

    expect(w.text()).toContain('页面加载失败')
    expect(api(w).pageError).toBe(true)

    getStudentCourses.mockResolvedValue(
      coursesPayload([courseOf(1, '液压系统检修', 40)], null)
    )
    await api(w).handleRetry()
    await flushPromises()

    expect(api(w).pageError).toBe(false)
    expect(w.findAll('.quick-card')[0].text()).toContain('液压系统检修')
  })
})
