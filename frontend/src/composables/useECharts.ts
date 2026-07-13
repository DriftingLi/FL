import { ref, onMounted, onUnmounted, type Ref } from 'vue'
import * as echarts from 'echarts'

export function useECharts(chartRef: Ref<HTMLElement | null>) {
  const chartInstance = ref<echarts.ECharts | null>(null)

  function init(option: echarts.EChartsOption) {
    if (!chartRef.value) return

    if (chartInstance.value) {
      chartInstance.value.dispose()
    }

    chartInstance.value = echarts.init(chartRef.value)
    chartInstance.value.setOption(option)
  }

  function dispose() {
    if (chartInstance.value) {
      chartInstance.value.dispose()
      chartInstance.value = null
    }
  }

  function resize() {
    if (chartInstance.value) {
      chartInstance.value.resize()
    }
  }

  function handleResize() {
    resize()
  }

  onMounted(() => {
    window.addEventListener('resize', handleResize)
  })

  onUnmounted(() => {
    window.removeEventListener('resize', handleResize)
    dispose()
  })

  return {
    chartInstance,
    init,
    dispose,
    resize
  }
}
