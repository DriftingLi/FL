import { onMounted, onUnmounted, type Ref } from 'vue'
import * as echarts from 'echarts'

/**
 * ECharts 组合式封装
 *
 * 主要防御点：
 * 1. init 时如果 DOM 尺寸为 0（例如被 v-show/display:none 隐藏），
 *    会用 requestAnimationFrame 延迟初始化，最多重试 10 次（约 16 帧 * 10 ≈ 160ms）。
 * 2. 使用 ResizeObserver 监听容器尺寸变化：
 *    - 容器从 0 变为非 0 时，如果尚未初始化则触发初始化
 *    - 容器尺寸变化时自动调用 resize()（替代 window resize 监听，更精准）
 * 3. setOption 时如果实例尚未创建（延迟初始化场景），缓存 option 待初始化后应用
 */
export function useECharts(chartRef: Ref<HTMLElement | null>) {
  // 不使用 ref 包裹：Vue 响应式代理会破坏 ECharts 内部状态，
  // 导致 "cartesian2d cannot be found for series.line" 等错误
  let chartInstance: echarts.ECharts | null = null
  // 缓存的 option：当 init 被调用但 DOM 尺寸为 0 时，缓存 option 等 DOM 就绪后应用
  let pendingOption: echarts.EChartsOption | null = null
  // 延迟初始化的 raf id（用于取消）
  let initRafId: number | null = null
  // 重试计数（防止无限重试）
  let initRetryCount = 0
  const MAX_INIT_RETRY = 10
  // ResizeObserver 实例
  let resizeObserver: ResizeObserver | null = null

  function init(option: echarts.EChartsOption) {
    if (!chartRef.value) return
    // 缓存 option，供延迟初始化或 ResizeObserver 触发时使用
    pendingOption = option

    // 取消上一次的延迟初始化（如果存在）
    if (initRafId !== null) {
      cancelAnimationFrame(initRafId)
      initRafId = null
    }

    // 检查 DOM 尺寸：v-show/display:none 的元素 clientWidth/clientHeight 为 0
    const width = chartRef.value.clientWidth
    const height = chartRef.value.clientHeight
    if (width === 0 || height === 0) {
      // DOM 尺寸为 0，延迟初始化，等 ResizeObserver 或 raf 触发
      initRetryCount = 0
      scheduleInit()
      return
    }

    // DOM 尺寸正常，立即初始化
    doInit()
  }

  // 实际执行初始化（DOM 尺寸已就绪）
  function doInit() {
    if (!chartRef.value || !pendingOption) return
    if (chartInstance) {
      chartInstance.dispose()
    }
    chartInstance = echarts.init(chartRef.value)
    chartInstance.setOption(pendingOption)
    // 初始化成功后清空缓存（保留 pendingOption 供后续 setOption 使用）
  }

  // 用 requestAnimationFrame 延迟初始化（兜底机制，主要靠 ResizeObserver）
  function scheduleInit() {
    if (initRafId !== null) {
      cancelAnimationFrame(initRafId)
    }
    initRafId = requestAnimationFrame(() => {
      initRafId = null
      if (!chartRef.value || !pendingOption) return
      const width = chartRef.value.clientWidth
      const height = chartRef.value.clientHeight
      if (width > 0 && height > 0) {
        doInit()
        return
      }
      // 尺寸仍为 0，继续重试
      initRetryCount++
      if (initRetryCount < MAX_INIT_RETRY) {
        scheduleInit()
      }
    })
  }

  function dispose() {
    if (initRafId !== null) {
      cancelAnimationFrame(initRafId)
      initRafId = null
    }
    if (resizeObserver) {
      resizeObserver.disconnect()
      resizeObserver = null
    }
    if (chartInstance) {
      chartInstance.dispose()
      chartInstance = null
    }
    pendingOption = null
  }

  function resize() {
    if (chartInstance) {
      chartInstance.resize()
    }
  }

  function handleResize() {
    resize()
  }

  onMounted(() => {
    window.addEventListener('resize', handleResize)
    // 使用 ResizeObserver 监听容器尺寸变化（比 window resize 更精准，能捕获 v-show/display 切换）
    if (chartRef.value && typeof ResizeObserver !== 'undefined') {
      resizeObserver = new ResizeObserver((entries) => {
        for (const entry of entries) {
          const { width, height } = entry.contentRect
          // 容器尺寸从 0 变为非 0：如果尚未初始化但有 pendingOption，触发初始化
          if (width > 0 && height > 0) {
            if (!chartInstance && pendingOption) {
              doInit()
            } else if (chartInstance) {
              // 已初始化的实例，直接 resize
              chartInstance.resize()
            }
          }
        }
      })
      resizeObserver.observe(chartRef.value)
    }
  })

  onUnmounted(() => {
    window.removeEventListener('resize', handleResize)
    dispose()
  })

  return {
    getChartInstance: () => chartInstance,
    init,
    dispose,
    resize
  }
}
