<script setup lang="ts">
// 维度评分雷达图（Tesla 极简配色：Electric Blue 单色 + 极淡灰背景）
// 改用 echarts.init 直接渲染（维修培训统一用法，不再依赖 vue-echarts）
// 重构说明：维度顺序由 DIMENSION_LABELS 常量提供，但以 scores 实际数据为准（数据驱动，避免硬编码漏维度）
import { computed, onBeforeUnmount, onMounted, ref, watch } from 'vue'
import * as echarts from 'echarts'
import { DIMENSION_LABELS } from '@/utils/valuationConstants'

interface Props {
  /** 维度评分（中文标签 → 0~1） */
  scores: Record<string, number>
  height?: string
}

const props = withDefaults(defineProps<Props>(), { height: '320px' })

const chartRef = ref<HTMLDivElement | null>(null)
let chart: echarts.ECharts | null = null

// 唯一主题色：Electric Blue（与 CTA 同一色系）
const COLOR_PRIMARY = '#3E6AE1'

/** 排序后的维度列表：先按 DIMENSION_LABELS 顺序，再补齐后端新增的维度 */
const orderedDimensions = computed(() => {
  const scoreMap: Record<string, number> = props.scores ?? {}
  const seen = new Set<string>()
  const result: { name: string; value: number }[] = []
  // 1) 按 DIMENSION_LABELS 顺序加入存在的维度
  for (const label of DIMENSION_LABELS) {
    if (scoreMap[label] !== undefined) {
      result.push({ name: label, value: Number(scoreMap[label].toFixed(3)) })
      seen.add(label)
    }
  }
  // 2) 补齐后端可能新增的、不在常量中的维度
  for (const [label, value] of Object.entries(scoreMap)) {
    if (!seen.has(label)) {
      result.push({ name: label, value: Number(value.toFixed(3)) })
    }
  }
  return result
})

const chartOption = computed(() => {
  const dims = orderedDimensions.value
  const indicators = dims.map((d) => ({ name: d.name, max: 1 }))
  const safeValues = dims.map((d) => d.value)
  return {
    tooltip: {
      trigger: 'item',
      backgroundColor: '#171A20',
      borderColor: 'transparent',
      textStyle: { color: '#fff', fontSize: 12 },
      formatter: (params: { value: number[] }) => {
        const lines = (params.value || []).map(
          (v, i) => `${indicators[i]?.name ?? ''} · ${(v * 100).toFixed(0)} 分`
        )
        return lines.join('<br/>')
      }
    },
    radar: {
      indicator: indicators,
      radius: '68%',
      center: ['50%', '52%'],
      splitNumber: 4,
      axisName: {
        color: '#393C41',
        fontSize: 13,
        fontWeight: 500
      },
      splitLine: { lineStyle: { color: '#EEEEEE' } },
      splitArea: { areaStyle: { color: ['#FFFFFF', '#FAFAFA'] } },
      axisLine: { lineStyle: { color: '#EEEEEE' } }
    },
    series: [
      {
        type: 'radar',
        data: [
          {
            value: safeValues,
            name: '维度评分',
            areaStyle: { color: COLOR_PRIMARY, opacity: 0.12 },
            lineStyle: { color: COLOR_PRIMARY, width: 2 },
            itemStyle: { color: COLOR_PRIMARY },
            symbol: 'circle',
            symbolSize: 5
          }
        ]
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
  <div ref="chartRef" class="dimension-radar" :style="{ height: props.height, width: '100%' }" />
</template>

<style scoped>
.dimension-radar {
  width: 100%;
}
</style>
