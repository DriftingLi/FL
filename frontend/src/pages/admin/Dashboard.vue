<template>
  <div class="dashboard-page">
    <!-- Welcome Banner -->
    <div class="welcome-banner">
      <div class="banner-content">
        <h1 class="banner-title">欢迎回来，{{ userName }}！</h1>
        <p class="banner-subtitle">系统运行正常，{{ overview.active_today || 0 }} 名学员今日活跃</p>
      </div>
    </div>

    <!-- 指标卡：唯一形态 UiStatCard（ADR-0087），本页用 icon-left + 色条 + 悬停档 -->
    <div class="stat-cards">
      <UiStatCard
        v-for="stat in statItems"
        :key="stat.label"
        layout="icon-left"
        accent
        hoverable
        :label="stat.label"
        :value="stat.value"
        :icon="stat.icon"
        :tone="stat.tone"
      />
    </div>

    <div class="chart-row">
      <UiCard divider>
        <template #header>
          <span class="card-title">课程学习热度排行</span>
        </template>
        <div ref="barChartRef" class="chart-container"></div>
      </UiCard>
      <UiCard divider>
        <template #header>
          <span class="card-title">课程平均进度</span>
        </template>
        <div ref="pieChartRef" class="chart-container"></div>
      </UiCard>
    </div>

    <UiCard divider>
      <template #header>
        <span class="card-title">快捷操作</span>
      </template>
      <div class="quick-actions">
        <div class="quick-action" v-for="action in quickActions" :key="action.label" @click="$router.push(action.to)">
          <div class="action-icon-wrap" :style="{ background: action.bgColor }">
            <UiIcon :name="action.icon" :size="20" :style="{ color: action.color }" />
          </div>
          <span class="action-label">{{ action.label }}</span>
        </div>
      </div>
    </UiCard>
  </div>
</template>

<script setup lang="ts">
import type { IconName } from '@/icons/registry'
import UiIcon from '@/components/ui/UiIcon.vue'
import UiStatCard from '@/components/ui/UiStatCard.vue'
import UiCard from '@/components/ui/UiCard.vue'
// keep-alive 的 include 按**组件名**匹配：名必须与路由名（页面描述符的 name）一致，
// 否则该页不会被缓存（#1620 的锁测试会拦下这种漂移）。
defineOptions({ name: 'AdminDashboard' })

import { ref, computed, onMounted, onUnmounted, nextTick, watch } from 'vue'
import { adminApi } from '@/api/admin'
import { useAuthStore } from '@/stores/auth'
import { href } from '@/config/pages'
import { useECharts } from '@/composables/useECharts'
import { chartTheme } from '@/utils/chartTheme'
import { useThemeStore } from '@/stores/theme'
import { displayNameOf } from '@/types/user'
import { formatDurationCompact } from '@/utils/format'

const authStore = useAuthStore()
const userName = computed(() => displayNameOf(authStore.userInfo) || '管理员')

const overview = ref<any>({})
const courseStats = ref<{ name: string; study_count: number; total_duration: number; avg_progress: number }[]>([])
const barChartRef = ref<HTMLDivElement | null>(null)
const pieChartRef = ref<HTMLDivElement | null>(null)
const { init: initBarChart } = useECharts(barChartRef)
const { init: initPieChart } = useECharts(pieChartRef)

// 移动端断点：≤768px 时切换饼图 legend 布局，避免 legend 与图像重叠
const MOBILE_BREAKPOINT = 768
const isMobile = ref(false)
function checkMobile() {
  isMobile.value = window.innerWidth <= MOBILE_BREAKPOINT
}

// 色调只取语义档（brand/ok/warn/neutral），不再逐卡手写色值：
// 同一批指标在仪表盘与统计分析两页必须同色（ADR-0087 批一）。
interface StatItem { label: string; value: string; icon: IconName; tone: 'brand' | 'ok' | 'warn' | 'neutral' }
const statItems = ref<StatItem[]>([
  { label: '学员总数', value: '0', icon: 'user', tone: 'brand' },
  { label: '今日活跃学员', value: '0', icon: 'user-filled', tone: 'ok' },
  { label: '课程总数', value: '0', icon: 'notebook', tone: 'warn' },
  { label: '总学习时长', value: '0', icon: 'timer', tone: 'neutral' }
])

interface QuickAction { label: string; to: ReturnType<typeof href>; icon: IconName; color: string; bgColor: string }
const quickActions: QuickAction[] = [
  { label: '用户管理', to: href('HrwaiUserManage'), icon: 'user', color: 'var(--color-primary-600)', bgColor: 'var(--color-primary-50)' },
  { label: '课程管理', to: href('CourseCatalog'), icon: 'notebook', color: 'var(--color-success)', bgColor: 'var(--color-success-light)' },
  { label: '统计分析', to: href('Statistics'), icon: 'chart-line', color: 'var(--color-warning)', bgColor: 'var(--color-warning-light)' },
  { label: '内容生成', to: href('ContentGenerate'), icon: 'wand-sparkles', color: 'var(--color-violet-500)', bgColor: 'var(--color-violet-50)' }
]

// 时长格式（紧凑）：抽到 utils/format（#796）——与 Statistics 的中文格式是同名不同语义
const formatDuration = formatDurationCompact

const themeStore = useThemeStore()

function renderBarChart() {
  if (courseStats.value.length === 0) return

  const t = chartTheme()

  const names = courseStats.value.map(c => c.name.length > 8 ? c.name.substring(0, 8) + '...' : c.name)
  const counts = courseStats.value.map(c => c.study_count)
  const durations = courseStats.value.map(c => c.total_duration)

  initBarChart({
    tooltip: {
      trigger: 'axis',
      axisPointer: { type: 'shadow' },
      backgroundColor: t.tooltip.backgroundColor,
      borderColor: t.tooltip.borderColor,
      borderWidth: 1,
      textStyle: t.tooltip.textStyle,
      extraCssText: 'box-shadow: var(--shadow-md); border-radius: var(--radius-md);'
    },
    legend: {
      data: ['学习人数', '学习时长(分钟)'],
      top: 0,
      textStyle: { color: t.text, fontSize: 12 }
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
      axisLabel: { rotate: 15, fontSize: 11, color: t.text },
      axisLine: { lineStyle: { color: t.axisLine } },
      axisTick: { show: false }
    },
    yAxis: [
      { type: 'value', name: '人数', axisLine: { show: false }, axisTick: { show: false }, splitLine: { lineStyle: { color: t.splitLine } } },
      { type: 'value', name: '时长(分钟)', axisLine: { show: false }, axisTick: { show: false }, splitLine: { show: false } }
    ],
    series: [
      {
        name: '学习人数',
        type: 'bar',
        data: counts,
        itemStyle: { color: t.series[0], borderRadius: [4, 4, 0, 0] },
        barWidth: '30%'
      },
      {
        name: '学习时长(分钟)',
        type: 'bar',
        yAxisIndex: 1,
        data: durations,
        itemStyle: { color: t.series[1], borderRadius: [4, 4, 0, 0] },
        barWidth: '30%'
      }
    ]
  })
}

function renderPieChart() {
  if (courseStats.value.length === 0) return

  const t = chartTheme()

  const data = courseStats.value.map(c => ({
    name: c.name.length > 10 ? c.name.substring(0, 10) + '...' : c.name,
    value: Math.round(c.avg_progress * 100) / 100
  }))

  // 移动端：legend 水平放底部并启用滚动，避免与饼图重叠；桌面端：legend 垂直放右侧
  const mobile = isMobile.value
  const legend = mobile
    ? {
        orient: 'horizontal' as const,
        bottom: 0,
        left: 'center' as const,
        type: 'scroll' as const,
        textStyle: { fontSize: 11, color: t.text },
        itemWidth: 10,
        itemHeight: 10,
        itemGap: 8
      }
    : {
        orient: 'vertical' as const,
        right: 10,
        top: 'center' as const,
        textStyle: { fontSize: 12, color: t.text }
      }

  initPieChart({
    tooltip: {
      trigger: 'item',
      formatter: '{b}: {c}%',
      backgroundColor: t.tooltip.backgroundColor,
      borderColor: t.tooltip.borderColor,
      borderWidth: 1,
      textStyle: t.tooltip.textStyle,
      extraCssText: 'box-shadow: var(--shadow-md); border-radius: var(--radius-md);'
    },
    legend,
    color: [...t.series],
    series: [
      {
        type: 'pie',
        // 移动端缩小半径并下移中心，为底部 legend 留出空间
        radius: mobile ? ['35%', '58%'] : ['40%', '70%'],
        center: mobile ? ['50%', '45%'] : ['40%', '50%'],
        avoidLabelOverlap: false,
        itemStyle: {
          borderRadius: 6,
          borderColor: t.pieBorderColor,
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

// 主题切换：option 里的颜色是快照，必须按新主题重绘
watch(() => themeStore.resolved, () => {
  renderBarChart()
  renderPieChart()
})

function handleResize() {
  // 跨越移动端断点时重新初始化饼图，切换 legend 布局避免重叠（容器缩放由 useECharts 处理）
  const wasMobile = isMobile.value
  checkMobile()
  if (wasMobile !== isMobile.value) {
    renderPieChart()
  }
}

async function loadStatistics() {
  try {
    const data = await adminApi.getStatistics()
    if (data) {
      overview.value = data.overview || {}
      courseStats.value = data.course_stats || []

      statItems.value[0].value = overview.value.total_students || 0
      statItems.value[1].value = overview.value.active_today || 0
      statItems.value[2].value = overview.value.total_courses || 0
      statItems.value[3].value = formatDuration(overview.value.total_study_duration || 0)

      await nextTick()
      renderBarChart()
      renderPieChart()
    }
  } catch (error) {
    console.error('加载统计数据失败:', error)
  }
}

onMounted(() => {
  checkMobile()
  loadStatistics()
  window.addEventListener('resize', handleResize)
})

onUnmounted(() => {
  window.removeEventListener('resize', handleResize)
})
</script>

<style scoped>
.dashboard-page {
  display: flex;
  flex-direction: column;
  gap: var(--space-6);
}

/* Welcome Banner
   底色由「表面 token + 品牌色浅混」现算：浅色下≈原来的淡紫，深色下是带紫调的深卡，
   不会因为品牌 -50 色阶不随主题翻转而变成一块白板（深色下刺眼的根因）。 */
.welcome-banner {
  background: color-mix(in srgb, var(--color-violet-500) 8%, var(--color-bg-card));
  border: 1px solid color-mix(in srgb, var(--color-violet-500) 18%, var(--color-border));
  border-radius: var(--radius-lg);
  padding: var(--space-6) var(--space-8);
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

.stat-cards {
  display: grid;
  grid-template-columns: repeat(4, 1fr);
  gap: var(--space-4);
}

.chart-row {
  display: grid;
  grid-template-columns: 1.4fr 1fr;
  gap: var(--space-4);
}

.card-title {
  font-size: var(--text-base);
  font-weight: var(--font-semibold);
  color: var(--color-text-primary);
}

/* 容器（圆角/描边/阴影/内距）由 UiCard 承担，这里只留图表的固定高度 */
.chart-container {
  width: 100%;
  height: 320px;
  min-height: 260px;
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

@media screen and (max-width: 1024px) {
  .chart-row {
    grid-template-columns: 1fr;
  }

  .chart-container {
    height: 280px;
  }
}

@media screen and (max-width: 768px) {
  .stat-cards {
    grid-template-columns: repeat(2, 1fr);
  }

  .quick-actions {
    grid-template-columns: repeat(2, 1fr);
  }

  .welcome-banner {
    padding: var(--space-5) var(--space-6);
  }

  .banner-title {
    font-size: var(--text-xl);
  }
}

</style>
