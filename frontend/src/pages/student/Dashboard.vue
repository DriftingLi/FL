<template>
  <div class="student-dashboard">
    <!-- Welcome Banner -->
    <div class="welcome-banner">
      <div class="banner-content">
        <h1 class="banner-title">欢迎回来，{{ userName }}！</h1>
        <p class="banner-subtitle">继续学习，向叉车维修专家迈进</p>
      </div>
      <router-link to="/training/courses" class="banner-action">
        浏览全部课程
        <el-icon><ArrowRight /></el-icon>
      </router-link>
    </div>

    <!-- 三列快捷卡片 -->
    <div class="quick-cards">
      <QuickCard
        title="进行中的课程"
        :items="activeCourses"
        :max-items="5"
        more-link="/training/courses"
        empty-text="暂无进行中的课程"
      />

      <QuickCard
        title="待完成考试"
        :items="pendingExams"
        :max-items="5"
        more-link="/training/level-exam"
        empty-text="暂无待完成的考试"
      />

      <QuickCard
        title="最近学习"
        :items="recentLearning"
        :max-items="5"
        empty-text="暂无学习记录"
      />
    </div>

    <!-- 学习统计 -->
    <div class="stats-section">
      <div class="stats-header">
        <div class="stats-title-group">
          <h2 class="stats-title">学习统计</h2>
          <span v-if="studyStats" class="stats-summary">
            共 {{ studyStats.total_minutes }} 分钟 · 活跃 {{ studyStats.active_days }} 天
          </span>
        </div>
        <div class="time-range-tabs">
          <button
            v-for="tab in timeTabs"
            :key="tab.value"
            class="time-tab"
            :class="{ active: currentTab === tab.value }"
            @click="currentTab = tab.value"
          >
            {{ tab.label }}
          </button>
        </div>
      </div>
      <div class="chart-container">
        <div v-if="statsLoading" class="chart-empty">加载中…</div>
        <div v-else-if="statsEmpty" class="chart-empty">暂无学习记录</div>
        <div v-show="!statsLoading && !statsEmpty" ref="chartRef" class="chart-area"></div>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, computed, onMounted, nextTick, watch } from 'vue'
import { ArrowRight } from '@element-plus/icons-vue'
import { ElMessage } from 'element-plus'
import { useAuthStore } from '@/stores/auth'
import QuickCard from '@/components/dashboard/QuickCard.vue'
import type { QuickCardItem } from '@/components/dashboard/QuickCard.vue'
import { useECharts } from '@/composables/useECharts'
import { studentApi } from '@/api/student'

const authStore = useAuthStore()

const userName = computed(() =>
  authStore.userInfo?.name || authStore.userInfo?.username || '同学'
)

// 进行中的课程
const activeCourses = ref<QuickCardItem[]>([])

// 待完成考试
const pendingExams = ref<QuickCardItem[]>([])

// 最近学习
const recentLearning = ref<QuickCardItem[]>([])

// 图表
const chartRef = ref<HTMLElement | null>(null)
const { init: initChart } = useECharts(chartRef)

const timeTabs = [
  { label: '近7天', value: '7d' },
  { label: '近30天', value: '30d' }
]
const currentTab = ref('7d')

// 学习统计数据
interface StudyStats {
  days: number
  labels: string[]
  data: number[]
  total_minutes: number
  active_days: number
}
const studyStats = ref<StudyStats | null>(null)
const statsLoading = ref(false)
const statsEmpty = computed(() => {
  if (!studyStats.value) return true
  return studyStats.value.data.every((v) => v === 0)
})

async function loadCourses() {
  try {
    const res = await studentApi.getProfile()
    if (res.code === 200 && res.data?.course_progress) {
      activeCourses.value = res.data.course_progress
        .filter((c: any) => c.progress > 0 && c.progress < 100)
        .sort((a: any, b: any) => (b.study_date || '').localeCompare(a.study_date || ''))
        .slice(0, 5)
        .map((c: any) => ({
          title: c.course_name,
          badge: `${Math.round(c.progress)}%`,
          path: `/training/course/${c.course_id}`
        }))
    }
  } catch (error: any) {
    console.error('加载课程失败:', error)
    const isTimeout = error?.code === 'ECONNABORTED' || /timeout/i.test(error?.message || '')
    ElMessage.warning(isTimeout ? '课程信息加载超时，可稍后刷新重试' : '课程信息加载失败，请稍后重试')
  }
}

async function loadRecentLearning() {
  try {
    const res = await studentApi.getRecords({ page: 1, page_size: 5 })
    if (res.code === 200 && res.data?.records) {
      recentLearning.value = res.data.records.map((r: any) => ({
        title: r.course_name || '未知课程',
        subtitle: r.chapter_title || `${r.study_duration || 0} 分钟`,
        badge: r.study_duration ? `${r.study_duration}分钟` : '',
        path: r.course_id ? `/training/course/${r.course_id}` : ''
      }))
    }
  } catch (error: any) {
    console.error('加载最近学习失败:', error)
    const isTimeout = error?.code === 'ECONNABORTED' || /timeout/i.test(error?.message || '')
    ElMessage.warning(isTimeout ? '最近学习记录加载超时，可稍后刷新重试' : '最近学习记录加载失败，请稍后重试')
  }
}

async function loadStudyStats() {
  statsLoading.value = true
  try {
    const days = currentTab.value === '30d' ? 30 : 7
    const res = await studentApi.getStudyStats({ days })
    if (res.code === 200 && res.data) {
      studyStats.value = res.data as StudyStats
    }
  } catch (error: any) {
    console.error('加载学习统计失败:', error)
    studyStats.value = null
    const isTimeout = error?.code === 'ECONNABORTED' || /timeout/i.test(error?.message || '')
    ElMessage.warning(isTimeout ? '学习统计加载超时，可点击切换时间范围重试' : '学习统计加载失败，请稍后重试')
  } finally {
    statsLoading.value = false
  }
}

function renderStudyChart() {
  if (!chartRef.value || !studyStats.value) return

  const labels = studyStats.value.labels
  const data = studyStats.value.data

  initChart({
    tooltip: {
      trigger: 'axis',
      backgroundColor: '#fff',
      borderColor: '#E2E8F0',
      borderWidth: 1,
      textStyle: { color: '#0F172A' },
      extraCssText: 'box-shadow: 0 4px 6px -1px rgba(0,0,0,0.1); border-radius: 8px;',
      valueFormatter: (val: any) => `${val} 分钟`
    },
    grid: {
      left: '3%',
      right: '4%',
      bottom: '3%',
      top: '10%',
      containLabel: true
    },
    xAxis: {
      type: 'category',
      data: labels,
      axisLabel: { fontSize: 11, color: '#64748B' },
      axisLine: { lineStyle: { color: '#E2E8F0' } },
      axisTick: { show: false }
    },
    yAxis: {
      type: 'value',
      name: '分钟',
      nameTextStyle: { color: '#94A3B8', fontSize: 11 },
      axisLine: { show: false },
      axisTick: { show: false },
      splitLine: { lineStyle: { color: '#F1F5F9' } }
    },
    series: [
      {
        type: 'line',
        data: data,
        smooth: true,
        symbol: 'circle',
        symbolSize: 6,
        lineStyle: { color: '#0EA5E9', width: 2.5 },
        itemStyle: { color: '#0EA5E9', borderWidth: 2, borderColor: '#fff' },
        areaStyle: {
          color: {
            type: 'linear',
            x: 0, y: 0, x2: 0, y2: 1,
            colorStops: [
              { offset: 0, color: 'rgba(14, 165, 233, 0.15)' },
              { offset: 1, color: 'rgba(14, 165, 233, 0.01)' }
            ]
          }
        }
      }
    ]
  })
}

// tab 切换时重新加载
watch(currentTab, async () => {
  await loadStudyStats()
  await nextTick()
  renderStudyChart()
})

onMounted(async () => {
  await Promise.all([loadCourses(), loadRecentLearning(), loadStudyStats()])
  await nextTick()
  renderStudyChart()
})
</script>

<style scoped>
.student-dashboard {
  display: flex;
  flex-direction: column;
  gap: var(--space-6);
}

/* Welcome Banner */
.welcome-banner {
  background: var(--color-primary-50);
  border: 1px solid var(--color-primary-100);
  border-radius: var(--radius-xl);
  padding: var(--space-6) var(--space-8);
  display: flex;
  align-items: center;
  justify-content: space-between;
  color: var(--color-text-primary);
}

.banner-title {
  font-family: var(--font-display);
  font-size: var(--text-2xl);
  font-weight: var(--font-bold);
  margin-bottom: var(--space-1);
  color: var(--color-text-primary);
}

.banner-subtitle {
  font-size: var(--text-base);
  color: var(--color-text-secondary);
}

.banner-action {
  display: flex;
  align-items: center;
  gap: var(--space-2);
  padding: var(--space-3) var(--space-5);
  background: var(--color-primary-500);
  border: 1px solid var(--color-primary-600);
  border-radius: var(--radius-lg);
  color: white;
  font-size: var(--text-sm);
  font-weight: var(--font-medium);
  text-decoration: none;
  transition: background var(--duration-fast);
  white-space: nowrap;
}

.banner-action:hover {
  background: var(--color-primary-600);
}

/* 三列快捷卡片 */
.quick-cards {
  display: grid;
  grid-template-columns: repeat(3, 1fr);
  gap: var(--space-4);
}

/* 学习统计 */
.stats-section {
  background: var(--color-bg-card);
  border-radius: var(--radius-xl);
  border: 1px solid var(--color-border-light);
  box-shadow: var(--shadow-xs);
  overflow: hidden;
}

.stats-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: var(--space-4) var(--space-5);
  border-bottom: 1px solid var(--color-border-light);
}

.stats-title-group {
  display: flex;
  align-items: baseline;
  gap: var(--space-3);
  flex-wrap: wrap;
}

.stats-summary {
  font-size: var(--text-xs);
  color: var(--color-text-secondary);
  font-weight: var(--font-regular);
}

.chart-empty {
  width: 100%;
  height: 260px;
  display: flex;
  align-items: center;
  justify-content: center;
  color: var(--color-text-secondary);
  font-size: var(--text-sm);
}

.stats-title {
  font-size: var(--text-base);
  font-weight: var(--font-semibold);
  color: var(--color-text-primary);
  margin: 0;
}

.time-range-tabs {
  display: flex;
  gap: var(--space-1);
  background: var(--color-bg-page);
  border-radius: var(--radius-md);
  padding: 2px;
}

.time-tab {
  padding: var(--space-1) var(--space-3);
  border: none;
  border-radius: var(--radius-sm);
  font-size: var(--text-xs);
  font-weight: var(--font-medium);
  color: var(--color-text-secondary);
  background: transparent;
  cursor: pointer;
  transition: all var(--duration-fast);
  font-family: var(--font-body);
}

.time-tab.active {
  background: var(--color-bg-card);
  color: var(--color-primary-600);
  box-shadow: var(--shadow-xs);
}

.time-tab:hover:not(.active) {
  color: var(--color-text-primary);
}

.chart-container {
  padding: var(--space-4) var(--space-5);
}

.chart-area {
  width: 100%;
  height: 260px;
}

@media screen and (max-width: 1024px) {
  .quick-cards {
    grid-template-columns: repeat(2, 1fr);
  }

  .chart-area {
    height: 220px;
  }
}

@media screen and (max-width: 768px) {
  .welcome-banner {
    flex-direction: column;
    align-items: flex-start;
    gap: var(--space-4);
    padding: var(--space-5) var(--space-6);
  }

  .quick-cards {
    grid-template-columns: 1fr;
  }

  .banner-title {
    font-size: var(--text-xl);
  }
}
</style>
