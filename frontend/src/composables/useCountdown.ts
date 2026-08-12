// 通用倒计时 composable（自动保存间隔 + 归零自动交卷回调）。
// 练习 / 模拟考试 / 定级考试 / 登录页验证码共用。
import { getCurrentInstance, onUnmounted, ref } from 'vue'
import type { Ref } from 'vue'

export interface UseCountdownOptions {
  /** 自动保存间隔（秒），每经过该间隔触发一次 onAutosave */
  autosaveInterval?: number
  /** 自动保存回调（由调用方注入持久化实现） */
  onAutosave?: () => void | Promise<void>
  /** 倒计时归零回调（自动交卷） */
  onExpire?: () => void | Promise<void>
}

export function useCountdown(options: UseCountdownOptions = {}) {
  const remaining: Ref<number> = ref(0)
  let timer: ReturnType<typeof setInterval> | null = null

  /** 单次 tick：归零触发 expire 并停止，否则递减并按间隔触发 autosave */
  function tick(): void {
    if (remaining.value <= 0) {
      stop()
      if (options.onExpire) options.onExpire()
      return
    }
    remaining.value--
    if (options.autosaveInterval && remaining.value % options.autosaveInterval === 0) {
      if (options.onAutosave) options.onAutosave()
    }
  }

  /** 从指定秒数开始倒计时（重复调用会重置并重建定时器） */
  function start(seconds: number): void {
    stop()
    remaining.value = seconds
    timer = setInterval(tick, 1000)
  }

  function stop(): void {
    if (timer) {
      clearInterval(timer)
      timer = null
    }
  }

  // 组件卸载时自动清理（非组件上下文调用时跳过，便于单测）
  if (getCurrentInstance()) onUnmounted(stop)

  return { remaining, start, stop, tick }
}
