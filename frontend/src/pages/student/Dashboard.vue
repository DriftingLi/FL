<template>
  <div class="flex flex-col gap-6">
    <!-- 首屏骨架 -->
    <template v-if="pageLoading">
      <div class="h-32 rounded-card border border-line bg-panel" />
      <div class="grid gap-4 md:grid-cols-2">
        <div class="h-64 rounded-card border border-line bg-panel" />
        <div class="h-64 rounded-card border border-line bg-panel" />
      </div>
      <UiSkeleton variant="chart" />
    </template>

    <!-- 整页错误态：拦截器已 toast，这里只给可操作的重试入口 -->
    <UiErrorState
      v-else-if="pageError"
      title="页面加载失败"
      description="网络或服务端异常，可重试"
      :retrying="retrying"
      @retry="handleRetry"
    />

    <template v-else>
      <!-- Welcome Banner -->
      <section
        class="rounded-card border border-ui-100 bg-gradient-to-br from-ui-50 to-panel p-6 sm:p-8"
      >
        <div class="flex flex-wrap items-center justify-between gap-4">
          <div class="min-w-0">
            <h1 class="font-heading text-2xl font-bold text-ink">
              欢迎回来，{{ userName }}！
            </h1>
            <p class="mt-1 text-sm text-ink-2">继续学习，向叉车维修专家迈进</p>
          </div>

          <div class="flex flex-wrap items-center gap-2">
            <router-link
              v-if="continueLearning"
              :to="continueLearningPath"
              class="inline-flex items-center gap-1 rounded-ctl bg-ui-500 px-4 py-2 text-sm font-medium text-white transition-colors duration-150 hover:bg-ui-600"
            >
              继续学习：{{ continueLearning.last_chapter_title || continueLearning.course_name }}
              <el-icon><ArrowRight /></el-icon>
            </router-link>
            <router-link
              to="/training/courses"
              class="inline-flex items-center gap-1 rounded-ctl border border-line-strong bg-panel px-4 py-2 text-sm text-ink-2 transition-colors duration-150 hover:border-ui-300 hover:text-ui-600"
            >
              浏览全部课程
              <el-icon><ArrowRight /></el-icon>
            </router-link>
          </div>
        </div>
      </section>

      <!-- 概览指标（数值滚动进场；reduced-motion 下 UiCountTo 直出终值） -->
      <div class="grid gap-4 sm:grid-cols-3">
        <UiStatCard
          label="学习时长"
          :value="overviewStats.minutes"
          unit="分钟"
          icon="Clock"
          tone="brand"
          :loading="statsLoading"
          count-to
        />
        <UiStatCard
          label="活跃天数"
          :value="overviewStats.activeDays"
          unit="天"
          icon="Calendar"
          tone="ok"
          :loading="statsLoading"
          count-to
        />
        <UiStatCard
          label="日均学习"
          :value="overviewStats.perDay"
          unit="分钟/天"
          icon="TrendCharts"
          tone="warn"
          :loading="statsLoading"
          count-to
        />
      </div>

      <!-- 快捷卡片 -->
      <div class="grid gap-4 md:grid-cols-2">
        <QuickCard
          class="stagger-in"
          :style="stagger(0)"
          title="进行中的课程"
          :items="activeCourses"
          :max-items="5"
          more-link="/training/courses"
          empty-text="暂无进行中的课程"
          variant="elevated"
        />
        <QuickCard
          class="stagger-in"
          :style="stagger(1)"
          title="最近学习"
          :items="recentLearning"
          :max-items="5"
          empty-text="暂无学习记录"
          variant="elevated"
        />
      </div>

      <!-- 学习统计 -->
      <section class="rounded-card border border-line bg-panel p-5">
        <UiSectionHeader title="学习统计" :subtitle="summary || undefined">
          <template #actions>
            <div class="flex items-center gap-1 rounded-ctl bg-canvas p-1">
              <button
                v-for="tab in timeTabs"
                :key="tab.value"
                class="rounded-ctl px-3 py-1 text-xs transition-colors duration-150"
                :class="
                  currentTab === tab.value
                    ? 'bg-panel text-ui-700 shadow-card'
                    : 'text-ink-3 hover:text-ink-2'
                "
                @click="currentTab = tab.value"
              >
                {{ tab.label }}
              </button>
            </div>
          </template>
        </UiSectionHeader>

        <div>
          <UiSkeleton v-if="statsLoading" variant="chart" />
          <UiEmptyState v-else-if="statsEmpty" description="暂无学习记录" size="sm" />
          <div v-show="!statsLoading && !statsEmpty" ref="chartRef" class="h-[260px] w-full"></div>
        </div>
      </section>
    </template>
  </div>
</template>

<script setup lang="ts">
import { ref, computed, onMounted } from 'vue'
import { ArrowRight } from '@element-plus/icons-vue'
import { useAuthStore } from '@/stores/auth'
import QuickCard from '@/components/dashboard/QuickCard.vue'
import type { QuickCardItem } from '@/components/dashboard/QuickCard.vue'
import UiEmptyState from '@/components/ui/UiEmptyState.vue'
import UiStatCard from '@/components/ui/UiStatCard.vue'
import UiErrorState from '@/components/ui/UiErrorState.vue'
import UiSectionHeader from '@/components/ui/UiSectionHeader.vue'
import UiSkeleton from '@/components/ui/UiSkeleton.vue'
import { useRoleDashboard } from '@/composables/useRoleDashboard'
import { useAsyncPage } from '@/composables/useAsyncPage'
import { useStagger } from '@/composables/useStagger'
import { studentApi, type StudyRecordItem, type StudentCourseItem } from '@/api/student'
import { displayNameOf } from '@/types/user'

const authStore = useAuthStore()

const userName = computed(() => displayNameOf(authStore.userInfo) || '同学')

// 首屏三态收编（#388，详情聚合页无分页）：pageLoading 初始为 true 的语义由
// 首次 onMounted 装载即触发保持（骨架在首个 tick 内不被穿透）
const stagger = useStagger()

// 进行中的课程
const activeCourses = ref<QuickCardItem[]>([])

// 继续学习（最后学习时间最新的课程，ADR-0017）
// 若无 last_chapter_id（仅报名未学或历史数据缺失），回退至课程详情页以便开始学习
const continueLearning = ref<StudentCourseItem | null>(null)
const continueLearningPath = computed(() => {
  const cl = continueLearning.value
  if (!cl) return ''
  if (cl.last_chapter_id) return `/training/course/${cl.course_id}/chapter/${cl.last_chapter_id}`
  return `/training/courses?course_id=${cl.course_id}`
})

// 最近学习
const recentLearning = ref<QuickCardItem[]>([])

// 学习统计 section（骨架/加载/空态/tab 切换/图表组装收敛进 useRoleDashboard）
const {
  chartRef,
  timeTabs,
  currentTab,
  stats,
  statsLoading,
  statsEmpty,
  summary,
  loadStats: loadStudyStats
} = useRoleDashboard({
  statsFetcher: async (days) => {
    const res = await studentApi.getStudyStats({ days })
    if (!res) return null
    // RoleDashboardStats 与 StudyStats 共享 days/labels/data/active_days，仅需归一化 total
    return { ...res, total: res.total_minutes }
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

/** 顶部概览三指标：时长 / 活跃天数 / 日均。跟随 statsLoading 一起进骨架 */
const overviewStats = computed(() => {
  const s = stats.value
  if (!s) return { minutes: 0, activeDays: 0, perDay: 0 }
  const activeDays = s.active_days || 0
  return {
    minutes: s.total || 0,
    activeDays,
    perDay: activeDays > 0 ? Math.round((s.total || 0) / activeDays) : 0
  }
})

async function loadCourses() {
  try {
    // 我的课程（ADR-0017）：含最后学习位置，点击直达最后学习章节
    const res = await studentApi.getStudentCourses()
    continueLearning.value = res?.continue_learning || null
    if (res?.courses) {
      activeCourses.value = res.courses
        .filter((c) => {
          const p = c.progress ?? 0
          return p > 0 && p < 100
        })
        .slice(0, 5)
        .map((c) => ({
          title: c.course_name || '未命名课程',
          subtitle: c.last_chapter_title || '',
          badge: `${Math.round(c.progress ?? 0)}%`,
          path: c.last_chapter_id
            ? `/training/course/${c.course_id}/chapter/${c.last_chapter_id}`
            : `/training/courses?course_id=${c.course_id}`
        }))
    }
  } catch (error) {
    console.error('加载课程失败:', error)
    // 向上抛，交给 loadAll 决定渲染错误态（拦截器已 toast，这里不重复提示）
    throw error
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
      const recentCourses: StudyRecordItem[] = []
      for (const r of res.records) {
        if (!r.course_id || seenCourses.has(r.course_id)) continue
        seenCourses.add(r.course_id)
        recentCourses.push(r)
        if (recentCourses.length >= 5) break
      }
      recentLearning.value = recentCourses.map((r) => ({
        title: r.course_name || '未知课程',
        subtitle: r.chapter_title || `${r.study_duration || 0} 分钟`,
        badge: r.study_duration ? `${r.study_duration}分钟` : '',
        path: `/training/courses`
      }))
    }
  } catch (error) {
    console.error('加载最近学习失败:', error)
    throw error
  }
}

const { loading: pageLoading, loadError: pageError, retrying, retry: handleRetry, run: loadAll } = useAsyncPage(
  async () => {
    // 三路并行：课程 / 最近学习 / 统计
    // #506：统计图渲染由 useRoleDashboard 自治（容器挂载即绘），此处不再手动编排
    await Promise.all([loadCourses(), loadRecentLearning(), loadStudyStats()])
  }
)

onMounted(loadAll)
</script>
