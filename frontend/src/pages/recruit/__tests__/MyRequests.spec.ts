// MyRequests（招聘端「我的申请」）分页契约（第十四波 B 票 10，ADR-0062 决策 10）。
// 缺陷（实测）：loader 把 `page: 1, page_size: 20` 硬编码当全量，页面却显示「共 {{ total }} 条」
// ⇒ 服务端有 45 条时屏上写 45 条、手里只有 20 条，第 21 条永不可见（total 与 items 不同源）。
// 正解：页码由 useAsyncPage 的分页三件套持有，容器与 total 都来自同一份响应。
// seam：页面组件层，mock `@/api/recruit`（不依赖真实后端）。
import { describe, it, expect, vi, beforeEach } from 'vitest'
import { mount, flushPromises } from '@vue/test-utils'
import { epLite } from '@/test/element-lite'

vi.mock('@/api/recruit', () => ({
  recruitApi: { listMyRequests: vi.fn() }
}))

import { recruitApi, type RecruitContactRequest, type RecruitContactRequestList } from '@/api/recruit'
import UiPagination from '@/components/ui/UiPagination.vue'
import MyRequests from '../MyRequests.vue'

const TOTAL = 45
const PAGE_SIZE = 20

function requestOf(id: number): RecruitContactRequest {
  return {
    id,
    student_user_id: id,
    recruiter_id: 1,
    status: 'pending',
    message: `附言 ${id}`,
    created_at: '2026-09-01 10:00',
    updated_at: '2026-09-01 10:00'
  }
}

/** 照后端事实作答：读 query 上的 page / page_size，回那一页的条目 + 全量 total。 */
function servePaged() {
  vi.mocked(recruitApi.listMyRequests).mockImplementation((async (params?: { page?: number; page_size?: number }) => {
    const page = params?.page ?? 1
    const pageSize = params?.page_size ?? PAGE_SIZE
    const start = (page - 1) * pageSize
    const length = Math.max(0, Math.min(pageSize, TOTAL - start))
    const items = Array.from({ length }, (_, i) => requestOf(start + i + 1))
    return { items, page, page_size: pageSize, total: TOTAL } as RecruitContactRequestList
  }) as never)
}

function mountPage() {
  return mount(MyRequests, { global: { plugins: [epLite()] } })
}

/** 屏上可见的学员 ID（行就是按 items 渲染的，用它数行数最稳）。 */
function visibleIds(w: ReturnType<typeof mountPage>): number[] {
  return [...w.text().matchAll(/学员 ID：(\d+)/g)].map(m => Number(m[1]))
}

function gotoPage(w: ReturnType<typeof mountPage>, target: number) {
  const pag = w.findComponent(UiPagination)
  expect(pag.exists(), `第 ${target} 页应有分页器可点`).toBe(true)
  pag.vm.$emit('update:currentPage', target)
  pag.vm.$emit('current-change', target)
}

beforeEach(() => {
  vi.clearAllMocks()
  servePaged()
})

describe('MyRequests 我的申请（票 10：total 与 items 同源，>20 条可翻完）', () => {
  it('首屏按当前页请求（不再硬编码 page:1,page_size:20 当全量）', async () => {
    const w = mountPage()
    await flushPromises()
    expect(recruitApi.listMyRequests).toHaveBeenLastCalledWith({ page: 1, page_size: PAGE_SIZE })
    expect(visibleIds(w)).toEqual(Array.from({ length: PAGE_SIZE }, (_, i) => i + 1))
    expect(w.text()).toContain(`共 ${TOTAL} 条`)
  })

  it('45 条翻得完：第 2、3 页真的发出去，条目与「共 45 条」同一来源', async () => {
    const w = mountPage()
    await flushPromises()

    gotoPage(w, 2)
    await flushPromises()
    expect(recruitApi.listMyRequests).toHaveBeenLastCalledWith({ page: 2, page_size: PAGE_SIZE })
    expect(visibleIds(w)).toEqual(Array.from({ length: PAGE_SIZE }, (_, i) => PAGE_SIZE + i + 1))

    // 第 21 条（此前永不可见的那一条）在第 2 页第一行
    expect(visibleIds(w)).toContain(21)

    gotoPage(w, 3)
    await flushPromises()
    expect(visibleIds(w)).toEqual([41, 42, 43, 44, 45])
    expect(w.text()).toContain(`共 ${TOTAL} 条`)
  })

  it('切到末页后总数不变：total 只由服务端信封写（不由本页自己数）', async () => {
    const w = mountPage()
    await flushPromises()
    gotoPage(w, 3)
    await flushPromises()
    expect(w.text()).toContain(`共 ${TOTAL} 条`)
    expect(visibleIds(w)).toHaveLength(5)
  })
})
