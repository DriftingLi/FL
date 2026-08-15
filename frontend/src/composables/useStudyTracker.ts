// useStudyTracker：章节学习时长上报 module（ticket #221 架构深化）。
// deep module：小 interface（loadDetail / reportDuration 两个 adapter）
// 藏全部时序与顺序约束——begin/stop、visibility 暂停恢复、beforeunload/卸载追报、
// 60s 阈值、切章前先报当前章、reportDuration 返回累计秒数推进上报游标（消除 detail===null 脆弱耦合）。
// 页面只注入两个 adapter：loadDetail 提供当前有效上下文，reportDuration 负责真正上报。
import { ref } from 'vue'

export interface StudyTrackerContext {
  courseId: number
  chapterId: number
}

/** 两个持久化 adapter：当前学习上下文 / 上报增量学习时长 */
export interface StudyTrackerAdapters {
  /**
   * 获取当前有效学习上下文（course/chapter id）；返回 null 表示无有效会话，上报被跳过。
   * 切章前调用报表时返回的是「将被离开」的当前章上下文，保证先报当前章。
   */
  loadDetail: () => StudyTrackerContext | null
  /**
   * 上报一段增量学习时长（incrementalSeconds 秒）。
   * 返回值语义：后端已确认上报的累计秒数（number）；composable 依此推进上报游标，
   * 从而把「是否推进」从响应的 detail===null 形态解耦。
   * 失败（抛错）时不应推进，调用方应返回当前已确认累计值。
   */
  reportDuration: (incrementalSeconds: number) => Promise<number>
}

/** 自动上报阈值（秒）：不足该值时增量不触发上报（与现状 AUTO_REPORT_INTERVAL / 60s 阈值一致） */
export const REPORT_THRESHOLD_SECONDS = 60

export function useStudyTracker(adapters: StudyTrackerAdapters) {
  const studySeconds = ref(0)
  const isStudying = ref(false)
  // 已确认上报的累计秒数游标（由 reportDuration 返回值推进）
  let confirmedSeconds = 0
  let studyStartTime: number | null = null
  let studyTimer: ReturnType<typeof setInterval> | null = null
  let autoReportTimer: ReturnType<typeof setInterval> | null = null

  function clearTimers() {
    if (studyTimer) {
      clearInterval(studyTimer)
      studyTimer = null
    }
    if (autoReportTimer) {
      clearInterval(autoReportTimer)
      autoReportTimer = null
    }
  }

  function startTimers() {
    studyTimer = setInterval(() => {
      if (studyStartTime) {
        studySeconds.value = Math.floor((Date.now() - studyStartTime) / 1000)
      }
    }, 1000)
    autoReportTimer = setInterval(() => {
      void reportIncremental(false)
    }, REPORT_THRESHOLD_SECONDS * 1000)
  }

  /**
   * 上报未提交增量：
   * - 60s 阈值：非 final 时增量 <60s 不报；isFinal（卸载/刷新）时强制报满。
   * - 上报成功后依据 reportDuration 返回的「已确认累计秒数」推进游标（max 兜底，防回退）。
   */
  async function reportIncremental(isFinal = false) {
    // 无有效学习上下文（loadDetail 返回 null）时跳过上报（切章前仍返回将被离开的当前章）
    if (!adapters.loadDetail()) return

    const incrementalSeconds = studySeconds.value - confirmedSeconds
    if (incrementalSeconds <= 0) return

    if (!isFinal && incrementalSeconds < REPORT_THRESHOLD_SECONDS) return

    try {
      const confirmed = await adapters.reportDuration(incrementalSeconds)
      if (confirmed > confirmedSeconds) confirmedSeconds = confirmed
    } catch (error) {
      if (!isFinal) console.warn('上报学习时长增量失败:', error)
    }
  }

  /** 开始学习计时（每次 begin 重置会话与游标） */
  function begin() {
    clearTimers()
    isStudying.value = true
    studyStartTime = Date.now()
    studySeconds.value = 0
    confirmedSeconds = 0
    startTimers()
  }

  /** 结束学习会话：清除计时与自动上报，停止上报（卸载/离开前配合 reportIncremental 追报） */
  function stop() {
    clearTimers()
    isStudying.value = false
  }

  /** 暂停（visibilitychange 隐藏）：停表并上报已达增量，保留会话态以便恢复 */
  function pause() {
    if (!isStudying.value) return
    clearTimers()
    void reportIncremental(false)
  }

  /** 恢复（visibilitychange 可见）：基于已累计秒数续跑计时与自动上报 */
  function resume() {
    if (!isStudying.value || studyTimer) return
    studyStartTime = Date.now() - studySeconds.value * 1000
    startTimers()
  }

  return { studySeconds, isStudying, begin, stop, pause, resume, reportIncremental }
}
