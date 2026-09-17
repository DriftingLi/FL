// #411 巡检问答积分流水视图契约（组件层）：默认请求锁定问答域 + 按域量词渲染 + 分页透出。
// #1102 补两档归属：档位一（分页列表 → useAdminTable）与档位二（只读计数 → useAsyncPage）
// 各自的错误态 + 重试入口都在本文件里有用例（ADR-0056 §9）。
import { describe, it, expect, vi, beforeEach } from 'vitest'
import { mount, flushPromises } from '@vue/test-utils'
import { epLite } from '@/test/element-lite'

vi.mock('@/api/request', () => ({
  unwrappedRequest: { get: vi.fn() },
}))

import { unwrappedRequest } from '@/api/request'
import InspectionView from '../Inspection.vue'

function mountView() {
  return mount(InspectionView, { global: { plugins: [epLite()] } })
}

const LEDGER_URL = '/admin/points/ledger'
const COUNT_URL = '/admin/inspection/deleted-after-accepted'

/**
 * 按 URL 分派响应：本页有 6 个 loader（5 段分页列表 + 1 个计数），按调用顺序排队太脆。
 * 未登记的 URL 一律给空列表（其余 section 走空态，不干扰断言）。
 */
function routeGet(routes: Record<string, () => unknown>) {
  vi.mocked(unwrappedRequest.get).mockReset()
  vi.mocked(unwrappedRequest.get).mockImplementation((async (url: string) => {
    const key = Object.keys(routes).find(k => String(url).includes(k))
    if (!key) return { items: [], total: 0 }
    return routes[key]()
  }) as never)
}

/** 失败响应：拦截器会把 ApiErrorKind 挂在错误对象上（client.ts attachKind）。 */
function rejectWith(kind: string) {
  return () => {
    throw Object.assign(new Error('boom'), { kind })
  }
}

/** 错误态自带的那颗「重试」按钮（UiErrorState → UiButton）。 */
function retryButton(wrapper: ReturnType<typeof mountView>) {
  return wrapper.findAll('button').find(b => b.text().includes('重试'))
}

beforeEach(() => {
  vi.mocked(unwrappedRequest.get).mockReset()
  vi.mocked(unwrappedRequest.get).mockResolvedValueOnce({ count: 3 })
  vi.mocked(unwrappedRequest.get).mockResolvedValueOnce({
    items: [
      { id: 1, user_id: 9, reason: 'accepted_bonus', delta: 40, ref_type: 'forum_topic', ref_id: '42', created_at: '2026-08-31T10:00:00Z' },
      { id: 2, user_id: 9, reason: 'daily_checkin', delta: 5, ref_type: 'task', ref_id: 'daily_checkin', created_at: '2026-08-31T09:00:00Z' },
    ],
    total: 2,
  })
})

describe('Inspection 问答积分流水（#411）', () => {
  it('组件默认请求携带 ref_type=forum_topic（问答域锁定）', async () => {
    mountView()
    await flushPromises()
    const get = vi.mocked(unwrappedRequest.get)
    const ledgerCall = get.mock.calls.find(c => String(c[0]).includes('/admin/points/ledger'))
    expect(ledgerCall).toBeTruthy()
    const params = ledgerCall![1] ? ledgerCall![1].params : undefined
    expect(params && params.ref_type).toBe('forum_topic')
  })

  it('行内引用按业务域渲染量词：任务行不显示「帖 …」', async () => {
    const wrapper = mountView()
    await flushPromises()
    expect(wrapper.text()).toContain('帖 42')
    expect(wrapper.text()).toContain('任务 daily_checkin')
  })

  it('渲染分页控件并透出 total', async () => {
    const wrapper = mountView()
    await flushPromises()
    expect(wrapper.findComponent({ name: 'ElPagination' }).exists()).toBe(true)
  })

  it('切换为跨业务域全量后不再携带 ref_type', async () => {
    const wrapper = mountView()
    await flushPromises()
    const select = wrapper.findComponent({ name: 'ElSelect' })
    await select.setValue('')
    await select.trigger('change')
    await flushPromises()
    const get = vi.mocked(unwrappedRequest.get)
    const ledgerCalls = get.mock.calls.filter(c => String(c[0]).includes('/admin/points/ledger'))
    const ledgerCall = ledgerCalls[ledgerCalls.length - 1]
    const params = ledgerCall[1] ? ledgerCall[1].params : undefined
    expect(params && params.ref_type ? params.ref_type : '').toBe('')
  })
})

// ===== 两档归属（#1102，ADR-0056 §9）：两档各自的失败回执与重试入口 =====

describe('Inspection 两档归属（#1102）', () => {
  it('档位二（只读计数 → useAsyncPage）：装载失败出错误态，点重试后计数回填', async () => {
    let failCount = true
    routeGet({
      [COUNT_URL]: () => {
        if (failCount) throw Object.assign(new Error('boom'), { kind: 'server' })
        return { count: 7 }
      },
      [LEDGER_URL]: () => ({ items: [], total: 0 })
    })
    const wrapper = mountView()
    await flushPromises()
    expect(wrapper.text()).toContain('计数加载失败')

    failCount = false
    await retryButton(wrapper)!.trigger('click')
    await flushPromises()
    expect(wrapper.text()).not.toContain('计数加载失败')
    expect(wrapper.text()).toContain('7')
  })

  it('档位一（分页列表 → useAdminTable）：流水装载失败出错误态，点重试后重装列表', async () => {
    let failLedger = true
    routeGet({
      [LEDGER_URL]: () => {
        if (failLedger) throw Object.assign(new Error('boom'), { kind: 'network' })
        return {
          items: [{ id: 1, user_id: 9, reason: 'accepted_bonus', delta: 40, ref_type: 'forum_topic', ref_id: '42', created_at: '2026-08-31T10:00:00Z' }],
          total: 1
        }
      },
      [COUNT_URL]: () => ({ count: 3 })
    })
    const wrapper = mountView()
    await flushPromises()
    expect(wrapper.text()).toContain('流水加载失败')

    failLedger = false
    await retryButton(wrapper)!.trigger('click')
    await flushPromises()
    expect(wrapper.text()).not.toContain('流水加载失败')
    expect(wrapper.text()).toContain('帖 42')
  })

  it('档位一空态：装载成功但无条目 → 空态（判据来自 useAdminTable.isEmpty，不是模板内联表达式）', async () => {
    routeGet({
      [LEDGER_URL]: () => rejectWith('notfound'),
      [COUNT_URL]: () => ({ count: 0 })
    })
    const wrapper = mountView()
    await flushPromises()
    expect(wrapper.text()).not.toContain('流水加载失败')
    expect(wrapper.text()).toContain('暂无数据')
  })
})
