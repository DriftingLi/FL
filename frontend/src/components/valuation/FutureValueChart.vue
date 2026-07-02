<script setup lang="ts">
// 未来估价柱状图：基于时间衰减系数推算未来 5 年的残值走势
// 公式推导：
//   当前 Kt_adj = Kt^(Kh/Kb) = e^(-λ·(Kh/Kb)·age)
//   未来 n 年后 Kt_adj_future = e^(-λ·(Kh/Kb)·(age+n)) = Kt_adj^(1+n/age)
//   future_value(n) = estimated_value × Kt_adj^(n/age)
//   即每年衰减乘数 d = Kt_adj^(1/age)，future_value(n) = estimated_value × d^n
import { computed, onBeforeUnmount, onMounted, ref, watch } from 'vue'
import * as echarts from 'echarts'

interface Props {
  /** 当前残值（元） */
  estimatedValue: number
  /** 使用年限（sale_year - factory_year） */
  age: number
  /** 时间衰减系数 Kt = e^(-λ·age) */
  kTime: number
  /** 使用强度系数 Kh */
  kHours: number
  /** 品牌系数 Kb */
  kBrand: number
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
let chart: echarts.ECharts | null = null

// 主题色（与设计稿一致）
const COLOR_PRIMARY = '#3E6AE1'
const COLOR_CURRENT = '#A8C0F5'
const COLOR_TEXT = '#1A1A1A'
const COLOR_TEXT_MUTED = '#999999'
const COLOR_GRID = '#F0F0F0'
const COLOR_AXIS = '#EEEEEE'

// age=0 时无法反推 λ，用电动 0.12 与内燃 0.10 的均值兜底
const DEFAULT_LAMBDA = 0.11

/** 计算年衰减乘数 d：future_value(n) = estimated_value × d^n */
function computeAnnualDecay(): number {
  const { age, kTime, kHours, kBrand } = props
  if (kBrand <= 0 || kTime <= 0) return Math.exp(-DEFAULT_LAMBDA)

  const ktAdjusted = Math.pow(kTime, kHours / kBrand)

  if (age > 0) {
    return Math.pow(ktAdjusted, 1 / age)
  }
  return Math.exp(-DEFAULT_LAMBDA)
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
  if (!chart) {
    chart = echarts.init(chartRef.value)
  }
  chart.setOption(chartOption.value, true)
}

function handleResize() {
  chart?.resize()
}

onMounted(() => {
  renderChart()
  window.addEventListener('resize', handleResize)
})

watch(
  () => chartOption.value,
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
