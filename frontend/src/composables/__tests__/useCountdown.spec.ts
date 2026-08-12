// 通用倒计时 composable（useCountdown.ts）单测：
// 倒计时状态机（start/tick/autosave/expire）
import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import { useCountdown } from '../useCountdown'

describe('倒计时状态机', () => {
  beforeEach(() => vi.useFakeTimers())
  afterEach(() => vi.useRealTimers())

  it('start 设置剩余秒数并开始递减', () => {
    const { remaining, start, tick } = useCountdown()
    start(5)
    expect(remaining.value).toBe(5)
    tick()
    expect(remaining.value).toBe(4)
    vi.advanceTimersByTime(3000)
    expect(remaining.value).toBe(1)
  })

  it('按 autosaveInterval 触发 onAutosave（含归零当次）', () => {
    const onAutosave = vi.fn()
    const { remaining, start } = useCountdown({ autosaveInterval: 30, onAutosave })
    start(60)
    vi.advanceTimersByTime(30000)
    expect(remaining.value).toBe(30)
    expect(onAutosave).toHaveBeenCalledTimes(1)
    vi.advanceTimersByTime(30000)
    expect(remaining.value).toBe(0)
    expect(onAutosave).toHaveBeenCalledTimes(2)
  })

  it('剩余为 0 时 tick 触发 onExpire', () => {
    const onExpire = vi.fn()
    const { remaining, tick } = useCountdown({ onExpire })
    tick()
    tick()
    tick()
    expect(onExpire).toHaveBeenCalledTimes(3)
    vi.advanceTimersByTime(1000)
    expect(remaining.value).toBe(0)
    expect(onExpire).toHaveBeenCalledTimes(3)
  })

  it('倒计时走完后定时器停止，不再 tick', () => {
    const onExpire = vi.fn()
    const { remaining, start } = useCountdown({ onExpire })
    start(1)
    vi.advanceTimersByTime(3000)
    expect(remaining.value).toBe(0)
    expect(onExpire).toHaveBeenCalledTimes(1)
  })

  it('重复 start 重置倒计时并重建定时器', () => {
    const { remaining, start, tick } = useCountdown()
    start(3)
    vi.advanceTimersByTime(2000)
    expect(remaining.value).toBe(1)
    start(10)
    expect(remaining.value).toBe(10)
    tick()
    expect(remaining.value).toBe(9)
  })

  it('stop 立即停止倒计时', () => {
    const { remaining, start, stop } = useCountdown()
    start(10)
    stop()
    vi.advanceTimersByTime(5000)
    expect(remaining.value).toBe(10)
  })

  it('start/expire 全流程：归零触发自动交卷且之后不再触发', () => {
    const onAutosave = vi.fn()
    const onExpire = vi.fn()
    const { remaining, start } = useCountdown({ autosaveInterval: 30, onAutosave, onExpire })
    start(30)
    vi.advanceTimersByTime(30000)
    expect(remaining.value).toBe(0)
    expect(onAutosave).toHaveBeenCalledTimes(1)
    expect(onExpire).toHaveBeenCalledTimes(0)
    vi.advanceTimersByTime(1000)
    expect(onExpire).toHaveBeenCalledTimes(1)
    vi.advanceTimersByTime(10000)
    expect(onExpire).toHaveBeenCalledTimes(1)
  })
})
