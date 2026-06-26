<template>
  <div class="dashboard-page">
    <div class="page-header">
      <h1 class="page-title">数据概览</h1>
      <p class="page-subtitle">系统运营数据一览</p>
    </div>

    <div class="stat-cards">
      <div class="stat-card" v-for="stat in statItems" :key="stat.label">
        <div class="stat-indicator" :style="{ background: stat.color }"></div>
        <div class="stat-body">
          <div class="stat-icon-wrap" :style="{ background: stat.bgColor }">
            <el-icon :size="22" :style="{ color: stat.color }"><component :is="stat.icon" /></el-icon>
          </div>
          <div class="stat-info">
            <div class="stat-value">{{ stat.value }}</div>
            <div class="stat-label">{{ stat.label }}</div>
          </div>
        </div>
      </div>
    </div>

    <div class="chart-row">
      <el-card class="chart-card">
        <template #header>
          <span class="card-title">课程学习热度排行</span>
        </template>
        <div ref="barChartRef" class="chart-container"></div>
      </el-card>
      <el-card class="chart-card">
        <template #header>
          <span class="card-title">课程平均进度</span>
        </template>
        <div ref="pieChartRef" class="chart-container"></div>
      </el-card>
    </div>

    <el-card class="quick-actions-card">
      <template #header>
        <span class="card-title">快捷操作</span>
      </template>
      <div class="quick-actions">
        <div class="quick-action" v-for="action in quickActions" :key="action.label" @click="$router.push(action.path)">
          <div class="action-icon-wrap" :style="{ background: action.bgColor }">
            <el-icon :size="20" :style="{ color: action.color }"><component :is="action.icon" /></el-icon>
          </div>
          <span class="action-label">{{ action.label }}</span>
        </div>
      </div>
    </el-card>
  </div>
</template>

<script setup lang="ts">
import { ref, onMounted, onUnmounted, nextTick } from 'vue'
import { User, UserFilled, Notebook, Timer, TrendCharts, MagicStick, Calendar, Setting } from '@element-plus/icons-vue'
import * as echarts from 'echarts'
import { adminApi } from '@/api/admin'

const overview = ref({})
const courseStats = ref([])
const barChartRef = ref(null)
const pieChartRef = ref(null)
let barChart = null
let pieChart = null

const statItems = ref([
  { label: '学员总数', value: '0', icon: User, color: '#2563EB', bgColor: '#EFF6FF' },
  { label: '今日活跃学员', value: '0', icon: UserFilled, color: '#10B981', bgColor: '#ECFDF5' },
  { label: '课程总数', value: '0', icon: Notebook, color: '#F59E0B', bgColor: '#FFFBEB' },
  { label: '总学习时长', value: '0', icon: Timer, color: '#8B5CF6', bgColor: '#F5F3FF' }
])

const quickActions = [
  { label: '学员管理', path: '/admin/students', icon: User, color: '#2563EB', bgColor: '#EFF6FF' },
  { label: '课程管理', path: '/admin/courses', icon: Notebook, color: '#10B981', bgColor: '#ECFDF5' },
  { label: '统计分析', path: '/admin/statistics', icon: TrendCharts, color: '#F59E0B', bgColor: '#FFFBEB' },
  { label: '内容生成', path: '/admin/content-generate', icon: MagicStick, color: '#8B5CF6', bgColor: '#F5F3FF' }
]

function formatDuration(minutes) {
  if (!minutes || minutes <= 0) return '0分钟'
  const hours = Math.floor(minutes / 60)
  const mins = minutes % 60
  if (hours > 0) {
    return mins > 0 ? `${hours}h${mins}m` : `${hours}h`
  }
  return `${mins}m`
}

const brandColors = ['#2563EB', '#3B82F6', '#60A5FA', '#10B981', '#34D399', '#F59E0B', '#FBBF24', '#8B5CF6']

function initBarChart() {
  if (!barChartRef.value || courseStats.value.length === 0) return

  if (barChart) barChart.dispose()
  barChart = echarts.init(barChartRef.value)

  const names = courseStats.value.map(c => c.name.length > 8 ? c.name.substring(0, 8) + '...' : c.name)
  const counts = courseStats.value.map(c => c.study_count)
  const durations = courseStats.value.map(c => c.total_duration)

  barChart.setOption({
    tooltip: {
      trigger: 'axis',
      axisPointer: { type: 'shadow' },
      backgroundColor: '#fff',
      borderColor: '#E2E8F0',
      borderWidth: 1,
      textStyle: { color: '#111827' },
      extraCssText: 'box-shadow: 0 4px 6px -1px rgba(0,0,0,0.1); border-radius: 8px;'
    },
    legend: {
      data: ['学习人数', '学习时长(分钟)'],
      top: 0,
      textStyle: { color: '#4B5563', fontSize: 12 }
    },
    grid: {
      left: '3%',
      right: '4%',
      bottom: '3%',
      containLabel: true
    },
    xAxis: {
      type: 'category',
      data: names,
      axisLabel: { rotate: 15, fontSize: 11, color: '#6B7280' },
      axisLine: { lineStyle: { color: '#E2E8F0' } },
      axisTick: { show: false }
    },
    yAxis: [
      { type: 'value', name: '人数', axisLine: { show: false }, axisTick: { show: false }, splitLine: { lineStyle: { color: '#F1F5F9' } } },
      { type: 'value', name: '时长(分钟)', axisLine: { show: false }, axisTick: { show: false }, splitLine: { show: false } }
    ],
    series: [
      {
        name: '学习人数',
        type: 'bar',
        data: counts,
        itemStyle: { color: '#2563EB', borderRadius: [4, 4, 0, 0] },
        barWidth: '30%'
      },
      {
        name: '学习时长(分钟)',
        type: 'bar',
        yAxisIndex: 1,
        data: durations,
        itemStyle: { color: '#10B981', borderRadius: [4, 4, 0, 0] },
        barWidth: '30%'
      }
    ]
  })
}

function initPieChart() {
  if (!pieChartRef.value || courseStats.value.length === 0) return

  if (pieChart) pieChart.dispose()
  pieChart = echarts.init(pieChartRef.value)

  const data = courseStats.value.map(c => ({
    name: c.name.length > 10 ? c.name.substring(0, 10) + '...' : c.name,
    value: Math.round(c.avg_progress * 100) / 100
  }))

  pieChart.setOption({
    tooltip: {
      trigger: 'item',
      formatter: '{b}: {c}%',
      backgroundColor: '#fff',
      borderColor: '#E2E8F0',
      borderWidth: 1,
      textStyle: { color: '#111827' },
      extraCssText: 'box-shadow: 0 4px 6px -1px rgba(0,0,0,0.1); border-radius: 8px;'
    },
    legend: {
      orient: 'vertical',
      right: 10,
      top: 'center',
      textStyle: { fontSize: 12, color: '#4B5563' }
    },
    color: brandColors,
    series: [
      {
        type: 'pie',
        radius: ['40%', '70%'],
        center: ['40%', '50%'],
        avoidLabelOverlap: false,
        itemStyle: {
          borderRadius: 6,
          borderColor: '#fff',
          borderWidth: 2
        },
        label: { show: false },
        emphasis: {
          label: {
            show: true,
            fontSize: 14,
            fontWeight: 'bold'
          }
        },
        data: data
      }
    ]
  })
}

function handleResize() {
  barChart && barChart.resize()
  pieChart && pieChart.resize()
}

async function loadStatistics() {
  try {
    const res = await adminApi.getStatistics()
    if (res.code === 200 && res.data) {
      overview.value = res.data.overview || {}
      courseStats.value = res.data.course_stats || []

      statItems.value[0].value = overview.value.total_students || 0
      statItems.value[1].value = overview.value.active_today || 0
      statItems.value[2].value = overview.value.total_courses || 0
      statItems.value[3].value = formatDuration(overview.value.total_study_duration || 0)

      await nextTick()
      initBarChart()
      initPieChart()
    }
  } catch (error) {
    console.error('加载统计数据失败:', error)
  }
}

onMounted(() => {
  loadStatistics()
  window.addEventListener('resize', handleResize)
})

onUnmounted(() => {
  window.removeEventListener('resize', handleResize)
  if (barChart) { barChart.dispose(); barChart = null }
  if (pieChart) { pieChart.dispose(); pieChart = null }
})
</script>

<style scoped>
.dashboard-page {
  display: flex;
  flex-direction: column;
  gap: var(--space-6);
}

.page-header {
  margin-bottom: 0;
}

.page-title {
  font-family: var(--font-display);
  font-size: var(--text-2xl);
  font-weight: var(--font-bold);
  color: var(--color-text-primary);
  margin-bottom: var(--space-1);
}

.page-subtitle {
  font-size: var(--text-sm);
  color: var(--color-text-tertiary);
}

.stat-cards {
  display: grid;
  grid-template-columns: repeat(4, 1fr);
  gap: var(--space-4);
}

.stat-card {
  background: var(--color-bg-card);
  border-radius: var(--radius-lg);
  box-shadow: var(--shadow-xs);
  border: 1px solid var(--color-border-light);
  overflow: hidden;
  transition: all var(--duration-normal) var(--ease-default);
}

.stat-card:hover {
  box-shadow: var(--shadow-md);
  transform: translateY(-2px);
}

.stat-indicator {
  width: 4px;
  height: 100%;
  position: absolute;
  left: 0;
  top: 0;
}

.stat-body {
  display: flex;
  align-items: center;
  gap: var(--space-4);
  padding: var(--space-5);
}

.stat-icon-wrap {
  width: 44px;
  height: 44px;
  border-radius: var(--radius-lg);
  display: flex;
  align-items: center;
  justify-content: center;
  flex-shrink: 0;
}

.stat-value {
  font-family: var(--font-display);
  font-size: var(--text-2xl);
  font-weight: var(--font-bold);
  color: var(--color-text-primary);
  line-height: 1.2;
}

.stat-label {
  font-size: var(--text-sm);
  color: var(--color-text-tertiary);
  margin-top: 2px;
}

.chart-row {
  display: grid;
  grid-template-columns: 1.4fr 1fr;
  gap: var(--space-4);
}

.chart-card {
  border-radius: var(--radius-xl);
  border: 1px solid var(--color-border-light);
  box-shadow: var(--shadow-xs);
}

.chart-card :deep(.el-card__header) {
  padding: var(--space-4) var(--space-5);
  border-bottom: 1px solid var(--color-border-light);
}

.chart-card :deep(.el-card__body) {
  padding: var(--space-3) var(--space-5);
  height: 320px;
}

.card-title {
  font-size: var(--text-base);
  font-weight: var(--font-semibold);
  color: var(--color-text-primary);
}

.chart-container {
  width: 100%;
  height: 100%;
  min-height: 260px;
}

.quick-actions-card {
  border-radius: var(--radius-xl);
  border: 1px solid var(--color-border-light);
  box-shadow: var(--shadow-xs);
}

.quick-actions-card :deep(.el-card__header) {
  padding: var(--space-4) var(--space-5);
  border-bottom: 1px solid var(--color-border-light);
}

.quick-actions-card :deep(.el-card__body) {
  padding: var(--space-4) var(--space-5);
}

.quick-actions {
  display: grid;
  grid-template-columns: repeat(4, 1fr);
  gap: var(--space-4);
}

.quick-action {
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: var(--space-2);
  padding: var(--space-4);
  border-radius: var(--radius-lg);
  cursor: pointer;
  transition: all var(--duration-fast) var(--ease-default);
}

.quick-action:hover {
  background: var(--color-bg-page);
}

.quick-action:hover .action-icon-wrap {
  transform: scale(1.1);
}

.action-icon-wrap {
  width: 44px;
  height: 44px;
  border-radius: var(--radius-lg);
  display: flex;
  align-items: center;
  justify-content: center;
  transition: transform var(--duration-fast) var(--ease-spring);
}

.action-label {
  font-size: var(--text-sm);
  font-weight: var(--font-medium);
  color: var(--color-text-secondary);
}

@media screen and (max-width: 768px) {
  .stat-cards {
    grid-template-columns: repeat(2, 1fr);
  }

  .chart-row {
    grid-template-columns: 1fr;
  }

  .chart-card :deep(.el-card__body) {
    height: 280px;
  }

  .quick-actions {
    grid-template-columns: repeat(2, 1fr);
  }
}

@media screen and (max-width: 480px) {
  .stat-value {
    font-size: var(--text-xl);
  }

  .stat-label {
    font-size: var(--text-xs);
  }
}
</style>
