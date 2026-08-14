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
          <span v-if="summary" class="stats-summary">{{ summary }}</span>
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
import { ref, computed, onMounted, nextTick } from 'vue'
import { ArrowRight } from '@element-plus/icons-vue'
import { useAuthStore } from '@/stores/auth'
import QuickCard from '@/components/dashboard/QuickCard.vue'
import type { QuickCardItem } from '@/components/dashboard/QuickCard.vue'
import { useRoleDashboard } from '@/composables/useRoleDashboard'
import { studentApi } from '@/api/student'
import { displayNameOf } from '@/types/user'

const authStore = useAuthStore()

const userName = computed(() => displayNameOf(authStore.userInfo) || '同学')

// 进行中的课程
const activeCourses = ref<QuickCardItem[]>([])

// 待完成考试
const pendingExams = ref<QuickCardItem[]>([])

// 最近学习
const recentLearning = ref<QuickCardItem[]>([])

// 学习统计 section（骨架/加载/空态/tab 切换/图表组装收敛进 useRoleDashboard）
const {
  chartRef,
  timeTabs,
  currentTab,
  statsLoading,
  statsEmpty,
  summary,
  loadStats: loadStudyStats,
  renderChart: renderStudyChart
} = useRoleDashboard({
  statsFetcher: async (days) => {
    const res = await studentApi.getStudyStats({ days })
    if (!res) return null
    return {
      days: res.days,
      labels: res.labels,
      data: res.data,
      total: res.total_minutes,
      active_days: res.active_days
    }
  },
  seriesType: 'line',
  unit: '分钟',
  yAxisName: '分钟',
  summaryText: (s) => `共 ${s.total} 分钟 · 活跃 ${s.active_days} 天`,
  timeTabs: [
    { label: '近7天', value: '7d', days: 7 },
    { label: '近30天', value: '30d', days: 30 }
  ]
})

async function loadCourses() {
  try {
    const res = await studentApi.getProfile()
    if (res?.course_progress) {
      activeCourses.value = res.course_progress
        .filter((c: any) => c.progress > 0 && c.progress < 100)
        .sort((a: any, b: any) => (b.study_date || '').localeCompare(a.study_date || ''))
        .slice(0, 5)
        .map((c: any) => ({
          title: c.course_name,
          badge: `${Math.round(c.progress)}%`,
          path: `/training/courses`
        }))
    }
  } catch (error: any) {
    console.error('加载课程失败:', error)
    /* 错误已由拦截器提示 */
  }
}

async function loadRecentLearning() {
  try {
    // 多拉一些记录再按课程去重：study_record 是逐章节/逐次学习的行，
    // 同一门课会占多条记录，直接取前 5 条会导致“最近学习”卡片出现重复课程。
    const res = await studentApi.getRecords({ page: 1, page_size: 50 })
    if (res?.records) {
      // 记录已按 study_date 倒序，第一次遇到的 course_id 即该课程最新学习记录
      const seenCourses = new Set<number>()
      const recentCourses: any[] = []
      for (const r of res.records) {
        if (!r.course_id || seenCourses.has(r.course_id)) continue
        seenCourses.add(r.course_id)
        recentCourses.push(r)
        if (recentCourses.length >= 5) break
      }
      recentLearning.value = recentCourses.map((r: any) => ({
        title: r.course_name || '未知课程',
        subtitle: r.chapter_title || `${r.study_duration || 0} 分钟`,
        badge: r.study_duration ? `${r.study_duration}分钟` : '',
        path: `/training/courses`
      }))
    }
  } catch (error: any) {
    console.error('加载最近学习失败:', error)
    /* 错误已由拦截器提示 */
  }
}

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
