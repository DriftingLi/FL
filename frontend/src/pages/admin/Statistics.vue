<template>
  <div class="statistics-page">
    <div class="page-header">
      <h2>统计分析</h2>
    </div>

    <!-- 指标卡：唯一形态 UiStatCard（ADR-0087），本页用居中无图标档 -->
    <el-row :gutter="20" class="stat-cards">
      <el-col :xs="12" :sm="6">
        <UiStatCard
          layout="plain"
          hoverable
          label="学员总数"
          :value="overview.total_students || 0"
          tone="brand"
        />
      </el-col>
      <el-col :xs="12" :sm="6">
        <UiStatCard
          layout="plain"
          hoverable
          label="今日活跃学员"
          :value="overview.active_today || 0"
          tone="ok"
        />
      </el-col>
      <el-col :xs="12" :sm="6">
        <UiStatCard
          layout="plain"
          hoverable
          label="课程总数"
          :value="overview.total_courses || 0"
          tone="warn"
        />
      </el-col>
      <el-col :xs="12" :sm="6">
        <UiStatCard
          layout="plain"
          hoverable
          label="总学习时长"
          :value="formatDuration(overview.total_study_duration || 0)"
          tone="neutral"
        />
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
        <UiButton variant="primary" :loading="exporting === 'students'" @click="handleExport('students')">
          学员名单
        </UiButton>
        <UiButton variant="warning" :loading="exporting === 'questions'" @click="handleExport('questions')">
          题库
        </UiButton>
        <UiButton variant="info" :loading="exporting === 'evaluations'" @click="handleExport('evaluations')">
          评估记录
        </UiButton>
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
// keep-alive 的 include 按**组件名**匹配：名必须与路由名（页面描述符的 name）一致，
// 否则该页不会被缓存（#1620 的锁测试会拦下这种漂移）。
defineOptions({ name: 'Statistics' })

import { ref, onMounted, nextTick, watch } from 'vue'
import { ElMessage } from 'element-plus'
import { adminApi } from '@/api/admin'
import { downloadExport, type ExportKind } from '@/api/export'
import { useECharts } from '@/composables/useECharts'
import { chartTheme } from '@/utils/chartTheme'
import { useThemeStore } from '@/stores/theme'
import UiButton from '@/components/ui/UiButton.vue'
import UiStatCard from '@/components/ui/UiStatCard.vue'
import { formatDurationCn, getProgressColor } from '@/utils/format'

const overview = ref<any>({})
const courseStats = ref<{ name: string; study_count: number; total_duration: number; avg_progress: number }[]>([])
const exporting = ref<ExportKind | ''>('')
const barChartRef = ref<HTMLDivElement | null>(null)
const progressChartRef = ref<HTMLDivElement | null>(null)
const { init: initBarChart } = useECharts(barChartRef)
const { init: initProgressChart } = useECharts(progressChartRef)
const themeStore = useThemeStore()

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

// 时长格式（中文）：抽到 utils/format（#796）
const formatDuration = formatDurationCn

// 进度→颜色：抽到 utils/format（#796）

function renderBarChart() {
  if (courseStats.value.length === 0) return

  const t = chartTheme()

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
      axisLabel: { rotate: 20, fontSize: 11, color: t.text }
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
          color: t.series[0]
        },
        barWidth: '30%'
      },
      {
        name: '学习时长(分钟)',
        type: 'bar',
        yAxisIndex: 1,
        data: durations,
        itemStyle: {
          color: t.series[1]
        },
        barWidth: '30%'
      }
    ]
  })
}

function renderProgressChart() {
  if (courseStats.value.length === 0) return

  const t = chartTheme()

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
      axisLabel: { rotate: 20, fontSize: 11, color: t.text }
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
            color: val >= 100 ? t.status.ok : val >= 60 ? t.series[0] : val >= 30 ? t.status.warn : t.status.bad
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

watch(() => themeStore.resolved, () => {
  renderBarChart()
  renderProgressChart()
})

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
  font-size: var(--text-2xl);
  color: var(--color-text-primary);
}

.stat-cards {
  margin-bottom: 20px;
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
  font-size: var(--text-base);
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

  .chart-row .el-col {
    margin-bottom: 12px;
  }

  .chart-card {
    height: 320px;
  }
}

</style>
