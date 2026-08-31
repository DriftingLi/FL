<template>
  <div class="statistics-page">
    <div class="page-header">
      <h2>统计分析</h2>
    </div>

    <el-row :gutter="20" class="stat-cards">
      <el-col :xs="12" :sm="6">
        <el-card shadow="hover" class="stat-card">
          <div class="stat-value blue">{{ overview.total_students || 0 }}</div>
          <div class="stat-label">学员总数</div>
        </el-card>
      </el-col>
      <el-col :xs="12" :sm="6">
        <el-card shadow="hover" class="stat-card">
          <div class="stat-value green">{{ overview.active_today || 0 }}</div>
          <div class="stat-label">今日活跃学员</div>
        </el-card>
      </el-col>
      <el-col :xs="12" :sm="6">
        <el-card shadow="hover" class="stat-card">
          <div class="stat-value orange">{{ overview.total_courses || 0 }}</div>
          <div class="stat-label">课程总数</div>
        </el-card>
      </el-col>
      <el-col :xs="12" :sm="6">
        <el-card shadow="hover" class="stat-card">
          <div class="stat-value gray">{{ formatDuration(overview.total_study_duration || 0) }}</div>
          <div class="stat-label">总学习时长</div>
        </el-card>
      </el-col>
    </el-row>

    <el-row :gutter="20" class="chart-row">
      <el-col :xs="24" :sm="12">
        <el-card class="chart-card">
          <template #header>
            <span class="card-title">课程学习热度排行</span>
          </template>
          <div ref="barChartRef" class="chart-container"></div>
        </el-card>
      </el-col>
      <el-col :xs="24" :sm="12">
        <el-card class="chart-card">
          <template #header>
            <span class="card-title">课程平均进度对比</span>
          </template>
          <div ref="progressChartRef" class="chart-container"></div>
        </el-card>
      </el-col>
    </el-row>

    <el-card class="table-card">
      <template #header>
        <span class="card-title">数据导出</span>
      </template>
      <div class="export-actions">
        <el-button
          type="primary"
          :loading="exporting === 'students'"
          @click="handleExport('students')"
        >
          学员名单
        </el-button>
        <el-button
          type="warning"
          :loading="exporting === 'questions'"
          @click="handleExport('questions')"
        >
          题库
        </el-button>
        <el-button
          type="info"
          :loading="exporting === 'evaluations'"
          @click="handleExport('evaluations')"
        >
          评估记录
        </el-button>
      </div>
    </el-card>

    <el-card class="table-card">
      <template #header>
        <span class="card-title">课程学习详细数据</span>
      </template>
      <el-table :data="courseStats" stripe border style="width: 100%">
        <el-table-column prop="name" label="课程名称" min-width="200" show-overflow-tooltip />
        <el-table-column label="学习人数" width="120" align="center">
          <template #default="{ row }">
            {{ row.study_count || 0 }}
          </template>
        </el-table-column>
        <el-table-column label="总学习时长" width="140" align="center">
          <template #default="{ row }">
            {{ formatDuration(row.total_duration || 0) }}
          </template>
        </el-table-column>
        <el-table-column label="平均进度" width="200" align="center">
          <template #default="{ row }">
            <el-progress
              :percentage="row.avg_progress || 0"
              :color="getProgressColor(row.avg_progress)"
              :stroke-width="16"
              :text-inside="true"
            />
          </template>
        </el-table-column>
      </el-table>
    </el-card>
  </div>
</template>

<script setup lang="ts">
import { ref, onMounted, nextTick } from 'vue'
// 按需引入（方案 11.9）：与 useECharts 共用 @/utils/echarts 的注册清单
import echarts from '@/utils/echarts'
import { ElMessage } from 'element-plus'
import { adminApi } from '@/api/admin'
import { downloadExport, type ExportKind } from '@/api/export'
import { useECharts } from '@/composables/useECharts'

const overview = ref<any>({})
const courseStats = ref<{ name: string; study_count: number; total_duration: number; avg_progress: number }[]>([])
const exporting = ref<ExportKind | ''>('')
const barChartRef = ref<HTMLDivElement | null>(null)
const progressChartRef = ref<HTMLDivElement | null>(null)
const { init: initBarChart } = useECharts(barChartRef)
const { init: initProgressChart } = useECharts(progressChartRef)

async function handleExport(kind: ExportKind) {
  exporting.value = kind
  try {
    await downloadExport(kind)
    ElMessage.success('导出成功')
  } catch {
    /* 错误已由拦截器提示 */
  } finally {
    exporting.value = ''
  }
}

function formatDuration(minutes: number) {
  if (!minutes || minutes <= 0) return '0分钟'
  const hours = Math.floor(minutes / 60)
  const mins = minutes % 60
  if (hours > 0) {
    return mins > 0 ? `${hours}小时${mins}分钟` : `${hours}小时`
  }
  return `${mins}分钟`
}

function getProgressColor(progress: number) {
  if (progress >= 100) return 'var(--color-success)'
  if (progress >= 60) return 'var(--color-primary-500)'
  if (progress >= 30) return '#e6a23c'
  return '#f56c6c'
}

function renderBarChart() {
  if (courseStats.value.length === 0) return

  const sortedStats = [...courseStats.value].sort((a, b) => b.study_count - a.study_count)
  const names = sortedStats.map(c => c.name.length > 8 ? c.name.substring(0, 8) + '...' : c.name)
  const counts = sortedStats.map(c => c.study_count)
  const durations = sortedStats.map(c => c.total_duration)

  initBarChart({
    tooltip: {
      trigger: 'axis',
      axisPointer: { type: 'shadow' }
    },
    legend: {
      data: ['学习人数', '学习时长(分钟)'],
      top: 0
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
      axisLabel: { rotate: 20, fontSize: 11 }
    },
    yAxis: [
      { type: 'value', name: '人数' },
      { type: 'value', name: '时长(分钟)' }
    ],
    series: [
      {
        name: '学习人数',
        type: 'bar',
        data: counts,
        itemStyle: {
          color: new echarts.graphic.LinearGradient(0, 0, 0, 1, [
            { offset: 0, color: '#409eff' },
            { offset: 1, color: '#79bbff' }
          ])
        },
        barWidth: '30%'
      },
      {
        name: '学习时长(分钟)',
        type: 'bar',
        yAxisIndex: 1,
        data: durations,
        itemStyle: {
          color: new echarts.graphic.LinearGradient(0, 0, 0, 1, [
            { offset: 0, color: '#67c23a' },
            { offset: 1, color: '#95d475' }
          ])
        },
        barWidth: '30%'
      }
    ]
  })
}

function renderProgressChart() {
  if (courseStats.value.length === 0) return

  const names = courseStats.value.map(c => c.name.length > 8 ? c.name.substring(0, 8) + '...' : c.name)
  const progressData = courseStats.value.map(c => Math.round(c.avg_progress * 100) / 100)

  initProgressChart({
    tooltip: {
      trigger: 'axis',
      formatter: '{b}: {c}%'
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
      axisLabel: { rotate: 20, fontSize: 11 }
    },
    yAxis: {
      type: 'value',
      name: '进度(%)',
      max: 100
    },
    series: [
      {
        type: 'bar',
        data: progressData.map(val => ({
          value: val,
          itemStyle: {
            color: val >= 100 ? '#67c23a' : val >= 60 ? '#409eff' : val >= 30 ? '#e6a23c' : '#f56c6c'
          }
        })),
        barWidth: '40%',
        label: {
          show: true,
          position: 'top',
          formatter: '{c}%',
          fontSize: 11
        }
      }
    ]
  })
}

async function loadStatistics() {
  try {
    const data = await adminApi.getStatistics()
    if (data) {
      overview.value = data.overview || {}
      courseStats.value = data.course_stats || []

      await nextTick()
      renderBarChart()
      renderProgressChart()
    }
  } catch (error) {
    console.error('加载统计数据失败:', error)
  }
}

onMounted(() => {
  loadStatistics()
})
</script>

<style scoped>
.statistics-page {
  padding: 20px;
}

.page-header {
  margin-bottom: 20px;
}

.page-header h2 {
  font-size: 22px;
  color: var(--color-text-primary);
}

.stat-cards {
  margin-bottom: 20px;
}

.stat-card {
  text-align: center;
  padding: 0;
}

.stat-card :deep(.el-card__body) {
  padding: 20px;
}

.stat-value {
  font-size: 28px;
  font-weight: bold;
  line-height: 1.3;
}

.stat-value.blue { color: var(--color-primary-500); }
.stat-value.green { color: var(--color-success); }
.stat-value.orange { color: var(--color-warning); }
.stat-value.gray { color: var(--color-text-tertiary); }

.stat-label {
  font-size: 13px;
  color: var(--color-text-tertiary);
  margin-top: 6px;
}

.chart-row {
  margin-bottom: 20px;
}

.chart-card {
  height: 380px;
}

.chart-card :deep(.el-card__body) {
  padding: 10px 16px;
  height: calc(100% - 56px);
}

.card-title {
  font-size: 16px;
  font-weight: 600;
  color: var(--color-text-primary);
}

.chart-container {
  width: 100%;
  height: 100%;
  min-height: 260px;
}

.export-actions {
  display: flex;
  gap: 12px;
  flex-wrap: wrap;
}

.table-card {
  margin-bottom: 20px;
}

@media screen and (max-width: 768px) {
  .statistics-page {
    padding: 12px;
  }

  .stat-cards .el-col {
    margin-bottom: 12px;
  }

  .stat-value {
    font-size: 22px;
  }

  .chart-row .el-col {
    margin-bottom: 12px;
  }

  .chart-card {
    height: 320px;
  }
}

@media screen and (max-width: 480px) {
  .stat-value {
    font-size: 18px;
  }

  .stat-label {
    font-size: 12px;
  }
}
</style>
