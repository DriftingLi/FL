// useStudyTracker：章节学习时长上报 module 的接口级测试。
// seam：composable 接口——loadDetail/reportDuration 两个 adapter 用内存 fake，
// 不触达 API 层；时序用 vi.useFakeTimers（同步伪造 Date.now 驱动 studySeconds）。
import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import { useStudyTracker } from '@/composables/useStudyTracker'
import type { StudyTrackerAdapters } from '@/composables/useStudyTracker'

/** fake adapter：loadDetail 返回固定上下文；reportDuration 记录接收到的增量秒并按累计推进确认游标 */
function makeTracker(overrides: Partial<StudyTrackerAdapters> = {}) {
  const received: number[] = []
  let confirmedTotal = 0
  const adapters: StudyTrackerAdapters = {
    loadDetail: () => ({ courseId: 1, chapterId: 1 }),
    reportDuration: async (incrementalSeconds: number) => {
      received.push(incrementalSeconds)
      confirmedTotal += incrementalSeconds
      return confirmedTotal
    },
    ...overrides
  }
  return { adapters, received }
}

beforeEach(() => {
  vi.useFakeTimers()
})

afterEach(() => {
  vi.useRealTimers()
})

describe('useStudyTracker（学习时长上报收敛）', () => {
  it('增量不足 60s 不触发上报（60s 阈值语义，期望独立字面量）', async () => {
    const { adapters, received } = makeTracker()
    const mod = useStudyTracker(adapters)

    mod.begin()
    // 经过 30s：studySeconds=30 < 60，无自动上报（60s 定时未到），手工上报亦被阈值拦截
    await vi.advanceTimersByTimeAsync(30_000)

    await mod.reportIncremental(false)

    expect(received).toEqual([])
  })

  it('增量达到 60s 触发自动上报（60s 定时触发，携带完整增量秒，adapter 据实确认）', async () => {
    const { adapters, received } = makeTracker()
    const mod = useStudyTracker(adapters)

    mod.begin()
    // 60s 时：1s 计时先到 → studySeconds=60；60s 自动上报随之触发 → incremental=60
    await vi.advanceTimersByTimeAsync(60_000)

    expect(received).toEqual([60])
    expect(mod.isStudying.value).toBe(true)
  })

  it('reach 90s（非整分钟）仍据 ≥60s 阈值按增量秒上报（页面 adapter 负责 ceil 取整为分钟）', async () => {
    const { adapters, received } = makeTracker()
    const mod = useStudyTracker(adapters)

    mod.begin()
    // 拉到 90s：60s 时自动上报一次（incremental=60 → 游标=60），90s 时增量=30<60 不再上报
    await vi.advanceTimersByTimeAsync(90_000)

    expect(received).toEqual([60])

    // 下一 60s 窗口：120s 自动上报增量=120-60=60（游标已推进，不重报前 60）
    await vi.advanceTimersByTimeAsync(30_000)
    expect(received).toEqual([60, 60])
  })

  it('reportDuration 返回累计秒数推进游标，重复调用不重报', async () => {
    const { adapters, received } = makeTracker()
    const mod = useStudyTracker(adapters)

    mod.begin()
    await vi.advanceTimersByTimeAsync(60_000)
    expect(received).toEqual([60])

    // 游标已推进到 60；同一时刻再上报增量=0，不再触发 adapter
    await mod.reportIncremental(false)
    expect(received).toEqual([60])

    await mod.reportIncremental(true)
    expect(received).toEqual([60])
  })

  it('stop 后清空计时与自动上报，不再上报', async () => {
    const { adapters, received } = makeTracker()
    const mod = useStudyTracker(adapters)

    mod.begin()
    await vi.advanceTimersByTimeAsync(30_000)

    mod.stop()
    expect(mod.isStudying.value).toBe(false)

    // 若未停表，60s/120s 会自动上报；停表后时间流逝不再触发
    await vi.advanceTimersByTimeAsync(180_000)
    await mod.reportIncremental(false)

    expect(received).toEqual([])
  })
})
