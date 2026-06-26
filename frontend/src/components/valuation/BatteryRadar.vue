// 电池特征重要性雷达图（6 维）
// 改用 echarts.init 直接渲染（维修培训统一用法，不再依赖 vue-echarts）
<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, ref, watch } from 'vue'
import * as echarts from 'echarts'
import { BATTERY_FEATURE_GROUPS, type FeatureImportance } from '@/types/valuation/battery'

interface Props {
  features: FeatureImportance[]
  height?: string
}
const props = withDefaults(defineProps<Props>(), { height: '300px' })

const chartRef = ref<HTMLDivElement | null>(null)
let chart: echarts.ECharts | null = null

// 按特征组聚合：组内 Top-1 作为该组得分
const grouped = computed(() => {
  const groups: Record<string, number> = {}
  for (const f of props.features) {
    const cur = groups[f.group] || 0
    if (f.normalized > cur) {
      groups[f.group] = f.normalized
    }
  }
  // 按固定顺序输出
  return BATTERY_FEATURE_GROUPS.map((g) => ({
    name: g,
    value: Math.round((groups[g] || 0) * 1000) / 10 // 百分比保留 1 位小数
  }))
})

const option = computed(() => ({
  tooltip: { trigger: 'item' },
  radar: {
    indicator: grouped.value.map((g) => ({ name: g.name, max: 50 })),
    splitArea: { areaStyle: { color: ['#fafafa', '#ffffff'] } },
    axisLine: { lineStyle: { color: '#d9d9d9' } },
    splitLine: { lineStyle: { color: '#e8e8e8' } },
    name: { textStyle: { color: '#393C41', fontSize: 13 } }
  },
  series: [
    {
      type: 'radar',
      data: [
        {
          value: grouped.value.map((g) => g.value),
          name: '特征重要性（%）',
          areaStyle: { color: 'rgba(62, 106, 225, 0.12)' },
          lineStyle: { color: '#3E6AE1', width: 2 },
          itemStyle: { color: '#3E6AE1' }
        }
      ]
    }
  ]
}))

function renderChart() {
  if (!chartRef.value) return
  if (!chart) {
    chart = echarts.init(chartRef.value)
  }
  chart.setOption(option.value, true)
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
  <div class="battery-radar" :style="{ height }">
    <div ref="chartRef" :style="{ height: '100%', width: '100%' }" />
  </div>
</template>

<style scoped>
.battery-radar {
  width: 100%;
}
</style>
