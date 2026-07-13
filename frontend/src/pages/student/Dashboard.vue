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
      >
        <template #default>
          <div v-for="course in activeCourses" :key="course.course_id" class="course-progress-item">
            <div class="progress-info">
              <span class="progress-title">{{ course.name }}</span>
              <div class="progress-bar-wrap">
                <div class="progress-bar" :style="{ width: course.progress + '%' }"></div>
              </div>
            </div>
            <span class="progress-percent">{{ course.progress }}%</span>
          </div>
        </template>
      </QuickCard>

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
        <h2 class="stats-title">学习统计</h2>
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
        <div ref="chartRef" class="chart-area"></div>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, computed, onMounted, nextTick } from 'vue'
import { ArrowRight } from '@element-plus/icons-vue'
import { useAuthStore } from '@/stores/auth'
import QuickCard from '@/components/dashboard/QuickCard.vue'
import type { QuickCardItem } from '@/components/dashboard/QuickCard.vue'
import { useECharts } from '@/composables/useECharts'
import { courseApi } from '@/api/course'

const authStore = useAuthStore()

const userName = computed(() =>
  authStore.userInfo?.name || authStore.userInfo?.username || '同学'
)

// 进行中的课程
const activeCourses = ref<any[]>([])

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

async function loadCourses() {
  try {
    const res = await courseApi.getCourses({ page: 1, page_size: 5 })
    if (res.code === 200 && res.data?.courses) {
      activeCourses.value = res.data.courses.map((c: any) => ({
        ...c,
        progress: Math.floor(Math.random() * 80 + 10) // mock progress
      }))
    }
  } catch (error) {
    console.error('加载课程失败:', error)
  }
}

function initStudyChart() {
  if (!chartRef.value) return

  const days = currentTab.value === '7d' ? 7 : 30
  const labels: string[] = []
  const data: number[] = []

  for (let i = days - 1; i >= 0; i--) {
    const d = new Date()
    d.setDate(d.getDate() - i)
    labels.push(`${d.getMonth() + 1}/${d.getDate()}`)
    data.push(Math.floor(Math.random() * 120 + 20))
  }

  initChart({
    tooltip: {
      trigger: 'axis',
      backgroundColor: '#fff',
      borderColor: '#E2E8F0',
      borderWidth: 1,
      textStyle: { color: '#0F172A' },
      extraCssText: 'box-shadow: 0 4px 6px -1px rgba(0,0,0,0.1); border-radius: 8px;'
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

onMounted(async () => {
  await loadCourses()
  await nextTick()
  initStudyChart()
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

/* 进行中的课程进度列表（替代 QuickCard 默认插槽） */
.course-progress-item {
  display: flex;
  align-items: center;
  gap: var(--space-3);
  padding: var(--space-3) var(--space-2);
  border-radius: var(--radius-md);
  cursor: pointer;
  transition: background var(--duration-fast);
}

.course-progress-item:hover {
  background: var(--color-bg-page);
}

.progress-info {
  flex: 1;
  min-width: 0;
  display: flex;
  flex-direction: column;
  gap: 6px;
}

.progress-title {
  font-size: var(--text-sm);
  font-weight: var(--font-medium);
  color: var(--color-text-primary);
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
}

.progress-bar-wrap {
  height: 4px;
  background: var(--color-border-light);
  border-radius: var(--radius-full);
  overflow: hidden;
}

.progress-bar {
  height: 100%;
  background: var(--color-primary-500);
  border-radius: var(--radius-full);
  transition: width var(--duration-slow);
}

.progress-percent {
  font-size: var(--text-xs);
  font-weight: var(--font-semibold);
  color: var(--color-primary-600);
  flex-shrink: 0;
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
