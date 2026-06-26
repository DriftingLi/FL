<script setup lang="ts">
// 6 维度评分雷达图（Tesla 极简配色：Electric Blue 单色 + 极淡灰背景）
// 改用 echarts.init 直接渲染（维修培训统一用法，不再依赖 vue-echarts）
import { computed, onBeforeUnmount, onMounted, ref, watch } from 'vue'
import * as echarts from 'echarts'

interface Props {
  /** 6 维度评分（中文标签 → 0~1） */
  scores: Record<string, number>
  height?: string
}

const props = withDefaults(defineProps<Props>(), { height: '320px' })

const chartRef = ref<HTMLDivElement | null>(null)
let chart: echarts.ECharts | null = null

/** 稳定的展示顺序 */
const dimensionOrder = ['时间维度', '使用强度', '工况', '品牌', '车况', '市场']

// 唯一主题色：Electric Blue（与 CTA 同一色系）
const COLOR_PRIMARY = '#3E6AE1'

const chartOption = computed(() => {
  // 容错：scores 可能是 undefined（详情接口未带此字段时）
  const scoreMap: Record<string, number> = props.scores ?? {}
  const indicators = dimensionOrder
    .filter((k) => scoreMap[k] !== undefined)
    .map((k) => ({ name: k, max: 1 }))
  // 关键：如果没有维度数据，给一组 0 占位，避免 ECharts 雷达图初始化时对 undefined 调用 push
  const hasData = indicators.length > 0
  const safeValues = hasData
    ? dimensionOrder
        .filter((k) => scoreMap[k] !== undefined)
        .map((k) => Number((scoreMap[k] ?? 0).toFixed(3)))
    : []
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
