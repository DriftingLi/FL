// ECharts 系数贡献柱图
// 改用 echarts.init 直接渲染（维修培训统一用法，不再依赖 vue-echarts）
<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, ref, watch } from 'vue'
import * as echarts from 'echarts'
import { COEFFICIENT_DEFS } from '@/utils/valuationConstants'
import type { CoefficientMap } from '@/utils/valuationConstants'

interface Props {
  /** 6 个 K 系数（0~1 之间） */
  coefficients: CoefficientMap
  /** 原始购买价格（用于计算贡献量） */
  originalPrice: number
  /** 高度 */
  height?: string
}

const props = withDefaults(defineProps<Props>(), { height: '340px' })

const chartRef = ref<HTMLDivElement | null>(null)
let chart: echarts.ECharts | null = null

// 系数贡献量 = 原价 × 该系数（粗略估算单因子影响）
// 实际 V = V₀ × Kt × Kh × (w₁·Kw + ...)，这里把每个 K 视为乘性贡献
const option = computed(() => {
  const labels: string[] = []
  const values: number[] = []
  const colors: string[] = []

  for (const def of COEFFICIENT_DEFS) {
    const v = props.coefficients[def.key]
    labels.push(def.label)
    values.push(Number((props.originalPrice * v).toFixed(2)))
    colors.push(def.color)
  }

  return {
    title: {
      text: '各系数对残值的乘性贡献（= 原价 × 系数）',
      left: 'left',
      top: 0,
      textStyle: { fontSize: 14, fontWeight: 600, color: '#0F172A' }
    },
    tooltip: {
      trigger: 'axis',
      axisPointer: { type: 'shadow' },
      formatter: (params: { dataIndex: number; value: number }[]) => {
        const i = params[0]?.dataIndex ?? 0
        const def = COEFFICIENT_DEFS[i]
        if (!def) return ''
        const raw = props.coefficients[def.key]
        return `<div style="font-weight:600;margin-bottom:4px">${def.label}</div>
          <div>${def.description}</div>
          <div style="margin-top:6px">系数值：<b>${raw?.toFixed(4) ?? '-'}</b></div>
          <div>贡献量：<b>${params[0]?.value?.toFixed(2) ?? '-'} 万元</b></div>`
      }
    },
    grid: { top: 60, right: 24, bottom: 32, left: 80 },
    xAxis: {
      type: 'value',
      name: '万元',
      axisLabel: { color: '#475569' },
      splitLine: { lineStyle: { color: '#E2E8F0' } }
    },
    yAxis: {
      type: 'category',
      data: labels,
      axisLabel: { color: '#0F172A', fontWeight: 500 },
      axisLine: { show: false },
      axisTick: { show: false }
    },
    series: [
      {
        type: 'bar',
        data: values.map((v, i) => ({ value: v, itemStyle: { color: colors[i] } })),
        barWidth: 24,
        label: {
          show: true,
          position: 'right',
          formatter: (p: { value: number }) => `${p.value.toFixed(2)} 万`,
          color: '#0F172A',
          fontFamily: 'var(--font-mono)'
        }
      }
    ]
  } as unknown as Record<string, unknown>
})

function renderChart() {
  if (!chartRef.value) return
  if (!chart) {
    chart = echarts.init(chartRef.value)
  }
  chart.setOption(option.value as echarts.EChartsCoreOption, true)
}

function handleResize() {
  chart?.resize()
}

onMounted(() => {
  renderChart()
  window.addEventListener('resize', handleResize)
})

watch(
  () => option.value,
  () => renderChart(),
  { deep: true }
)

onBeforeUnmount(() => {
  window.removeEventListener('resize', handleResize)
  chart?.dispose()
  chart = null
})
</script>

<template>
  <div class="coefficient-chart" :style="{ height }">
    <div ref="chartRef" :style="{ height: '100%', width: '100%' }" />
  </div>
</template>

<style scoped>
.coefficient-chart {
  width: 100%;
  background: var(--color-bg-elevated);
  border-radius: var(--radius-md);
  border: 1px solid var(--color-border);
  padding: 16px;
}
</style>
