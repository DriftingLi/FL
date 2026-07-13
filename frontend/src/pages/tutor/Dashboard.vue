<template>
  <div class="tutor-dashboard">
    <!-- Welcome Banner -->
    <div class="welcome-banner">
      <div class="banner-content">
        <h1 class="banner-title">欢迎回来，{{ userName }}！</h1>
        <p class="banner-subtitle">{{ pendingCount > 0 ? `今日有 ${pendingCount} 份试卷待批阅` : '今日暂无待批阅试卷' }}</p>
      </div>
      <router-link v-if="pendingCount > 0" to="/training/tutor/grading" class="banner-action">
        前往批阅
        <el-icon><ArrowRight /></el-icon>
      </router-link>
    </div>

    <!-- 三列快捷卡片 -->
    <div class="quick-cards">
      <QuickCard
        title="待批阅试卷"
        :items="pendingGrading"
        :max-items="5"
        more-link="/training/tutor/grading"
        empty-text="暂无待批阅试卷"
      />

      <QuickCard
        title="我的课程"
        :items="myCourses"
        :max-items="5"
        more-link="/training/tutor/courses"
        empty-text="暂无课程"
      />

      <QuickCard
        title="最近批阅"
        :items="recentGrading"
        :max-items="5"
        empty-text="暂无批阅记录"
      />
    </div>

    <!-- 阅卷统计 -->
    <div class="stats-section">
      <div class="stats-header">
        <h2 class="stats-title">阅卷统计</h2>
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
import { tutorApi } from '@/api/tutor'
import { gradingApi } from '@/api/grading'

const authStore = useAuthStore()

const userName = computed(() =>
  authStore.userInfo?.name || authStore.userInfo?.username || '导师'
)

const pendingCount = ref(0)
const pendingGrading = ref<QuickCardItem[]>([])
const myCourses = ref<QuickCardItem[]>([])
const recentGrading = ref<QuickCardItem[]>([])

const chartRef = ref<HTMLElement | null>(null)
const { init: initChart } = useECharts(chartRef)

const timeTabs = [
  { label: '本周', value: 'week' },
  { label: '本月', value: 'month' }
]
const currentTab = ref('week')

async function loadData() {
  try {
    // 加载导师课程
    const courseRes = await tutorApi.getCourses({})
    if (courseRes.code === 200 && courseRes.data) {
      const courses = Array.isArray(courseRes.data) ? courseRes.data : (courseRes.data.courses || [])
      myCourses.value = courses.slice(0, 5).map((c: any) => ({
        title: c.name || c.course_name,
        subtitle: `${c.student_count || 0} 名学员`,
        path: c.course_id ? `/training/tutor/course/${c.course_id}/chapters` : ''
      }))
    }
  } catch (error) {
    console.error('加载导师数据失败:', error)
  }

  try {
    // 加载待批阅
    const gradingRes = await gradingApi.getSubmittedParticipants({ status: 'pending', page: 1, page_size: 5 })
    if (gradingRes.code === 200 && gradingRes.data) {
      const items = Array.isArray(gradingRes.data) ? gradingRes.data : (gradingRes.data.participants || gradingRes.data.items || [])
      pendingCount.value = gradingRes.data.total || items.length
      pendingGrading.value = items.slice(0, 5).map((p: any) => ({
        title: `${p.student_name || '学员'} - ${p.exam_name || p.session_name || '考试'}`,
        badge: `${p.pending_count || p.question_count || '?'}题待批`,
        path: p.participant_id ? `/training/tutor/grading?participant=${p.participant_id}` : '/training/tutor/grading'
      }))
    }
  } catch (error) {
    console.error('加载批阅数据失败:', error)
  }
}

function initGradingChart() {
  if (!chartRef.value) return

  const days = currentTab.value === 'week' ? 7 : 30
  const labels: string[] = []
  const data: number[] = []

  for (let i = days - 1; i >= 0; i--) {
    const d = new Date()
    d.setDate(d.getDate() - i)
    labels.push(`${d.getMonth() + 1}/${d.getDate()}`)
    data.push(Math.floor(Math.random() * 15 + 2))
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
      name: '份',
      nameTextStyle: { color: '#94A3B8', fontSize: 11 },
      axisLine: { show: false },
      axisTick: { show: false },
      splitLine: { lineStyle: { color: '#F1F5F9' } }
    },
    series: [
      {
        type: 'bar',
        data: data,
        barWidth: '40%',
        itemStyle: {
          color: '#0EA5E9',
          borderRadius: [4, 4, 0, 0]
        }
      }
    ]
  })
}

onMounted(async () => {
  await loadData()
  await nextTick()
  initGradingChart()
})
</script>

<style scoped>
.tutor-dashboard {
  display: flex;
  flex-direction: column;
  gap: var(--space-6);
  max-width: 1200px;
}

.welcome-banner {
  background: linear-gradient(135deg, #059669 0%, #14B8A6 100%);
  border-radius: var(--radius-xl);
  padding: var(--space-6) var(--space-8);
  display: flex;
  align-items: center;
  justify-content: space-between;
  color: white;
}

.banner-title {
  font-family: var(--font-display);
  font-size: var(--text-2xl);
  font-weight: var(--font-bold);
  margin-bottom: var(--space-1);
}

.banner-subtitle {
  font-size: var(--text-base);
  opacity: 0.85;
}

.banner-action {
  display: flex;
  align-items: center;
  gap: var(--space-2);
  padding: var(--space-3) var(--space-5);
  background: rgba(255, 255, 255, 0.2);
  backdrop-filter: blur(8px);
  border: 1px solid rgba(255, 255, 255, 0.3);
  border-radius: var(--radius-lg);
  color: white;
  font-size: var(--text-sm);
  font-weight: var(--font-medium);
  text-decoration: none;
  transition: background var(--duration-fast);
  white-space: nowrap;
}

.banner-action:hover {
  background: rgba(255, 255, 255, 0.3);
}

.quick-cards {
  display: grid;
  grid-template-columns: repeat(3, 1fr);
  gap: var(--space-4);
}

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
