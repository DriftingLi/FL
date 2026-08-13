<script setup lang="ts">
// 未来估价柱状图：future_value(n) = estimated_value × decay_anchor^n。
// 衰减公式唯一实现在后端（ADR-0012 §8）：decay_anchor 为评估时点锁定锚点，
// 前端只做数据渲染与乘法，不持有任何领域公式。
import { computed, onMounted, ref, watch } from 'vue'
import { useECharts } from '@/composables/useECharts'

interface Props {
  /** 当前残值（元） */
  estimatedValue: number
  /** 年衰减锚点 d（评估时点锁定，API 下发） */
  decayAnchor: number
  /** 评估年份（用于 X 轴标签，不传则用相对标签） */
  saleYear?: number
  /** 预测未来年数 */
  years?: number
  height?: string
}

const props = withDefaults(defineProps<Props>(), {
  saleYear: 0,
  years: 5,
  height: '320px'
})

const chartRef = ref<HTMLDivElement | null>(null)
const { init } = useECharts(chartRef)

// 主题色（与设计稿一致）
const COLOR_PRIMARY = '#3E6AE1'
const COLOR_CURRENT = '#A8C0F5'
const COLOR_TEXT = '#1A1A1A'
const COLOR_TEXT_MUTED = '#999999'
const COLOR_GRID = '#F0F0F0'
const COLOR_AXIS = '#EEEEEE'

function computeAnnualDecay(): number {
  return props.decayAnchor
}

interface FuturePoint {
  label: string
  value: number
}

/** 未来估价序列（含当前年，共 years+1 个点） */
const futureValues = computed<FuturePoint[]>(() => {
  const decay = computeAnnualDecay()
  const base = props.estimatedValue
  const points: FuturePoint[] = []

  for (let n = 0; n <= props.years; n++) {
    const value = base * Math.pow(decay, n)
    const label =
      n === 0
        ? '当前'
        : props.saleYear > 0
          ? `${props.saleYear + n}年`
          : `+${n}年`
    points.push({ label, value: Math.max(0, value) })
  }
  return points
})

/** 年衰减率（用于副标题展示） */
const annualDecayRate = computed(() => {
  const decay = computeAnnualDecay()
  return Math.max(0, 1 - decay)
})

const chartOption = computed(() => {
  const data = futureValues.value
  return {
    tooltip: {
      trigger: 'axis',
      backgroundColor: '#171A20',
      borderColor: 'transparent',
      textStyle: { color: '#fff', fontSize: 12 },
      formatter: (params: { name: string; value: number }[]) => {
        const p = Array.isArray(params) ? params[0] : params
        const wan = (p.value / 10000).toFixed(2)
        return `${p.name}<br/>估价：<b>${wan} 万元</b>`
      }
    },
    grid: {
      left: '2%',
      right: '3%',
      bottom: '2%',
      top: '12%',
      containLabel: true
    },
    xAxis: {
      type: 'category',
      data: data.map((d) => d.label),
      axisLine: { lineStyle: { color: COLOR_AXIS } },
      axisTick: { show: false },
      axisLabel: { color: COLOR_TEXT_MUTED, fontSize: 12 }
    },
    yAxis: {
      type: 'value',
      axisLabel: {
        color: COLOR_TEXT_MUTED,
        fontSize: 11,
        formatter: (v: number) => `${(v / 10000).toFixed(1)}万`
      },
      splitLine: { lineStyle: { color: COLOR_GRID } },
      axisLine: { show: false },
      axisTick: { show: false }
    },
    series: [
      {
        type: 'bar',
        data: data.map((d, i) => ({
          value: d.value,
          itemStyle: { color: i === 0 ? COLOR_CURRENT : COLOR_PRIMARY }
        })),
        barWidth: '45%',
        itemStyle: {
          borderRadius: [4, 4, 0, 0]
        },
        label: {
          show: true,
          position: 'top',
          color: COLOR_TEXT,
          fontSize: 11,
          formatter: (p: { value: number }) => `${(p.value / 10000).toFixed(2)}万`
        }
      }
    ]
  }
})

function renderChart() {
  if (!chartRef.value) return
  init(chartOption.value, true)
}

onMounted(() => {
  renderChart()
})

watch(
  () => chartOption.value,
  () => renderChart(),
  { deep: true }
)
</script>

<template>
  <div class="future-value-chart">
    <div ref="chartRef" class="chart-canvas" :style="{ height: props.height, width: '100%' }" />
    <p class="chart-subtitle">
      年衰减率 <span class="decay-rate">{{ (annualDecayRate * 100).toFixed(1) }}%</span>
    </p>
  </div>
</template>

<style scoped>
.future-value-chart {
  width: 100%;
}
.chart-canvas {
  width: 100%;
}
.chart-subtitle {
  font-size: var(--fs-sm);
  color: var(--color-text-tertiary);
  margin: var(--sp-4) 0 0;
  letter-spacing: 0.04em;
}
.decay-rate {
  font-family: var(--font-mono);
  font-weight: var(--fw-medium);
  color: var(--color-accent);
  margin-left: 4px;
}
</style>
