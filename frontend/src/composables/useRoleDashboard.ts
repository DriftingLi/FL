// useRoleDashboard：学员/导师工作台 time-series 统计 section 的唯一实现。
// deep module：小 interface（statsFetcher/series/unit/yAxisName/summaryText/emptyText/timeTabs）
// 藏大量 implementation（统计加载、空态、tab 切换、ECharts option 组装、倒计时重绘时序）。
// banner 与 QuickCard 三列因角色语义差异留在各页面（薄 adapter 注入数据），QuickCard 组件复用不变。
import { ref, computed, watch, nextTick } from 'vue'
import { useECharts } from './useECharts'
import type { StudyStats } from '@/api/student'

/** 归一化后的按天统计：days/labels/data/active_days 直接复用学员 StudyStats 形状，
 *  total 为各角色总数（学员 total_minutes / 导师 total_count）归一化而来 */
export type RoleDashboardStats = Omit<StudyStats, 'total_minutes'> & { total: number }

export interface RoleDashboardTimeTab {
  label: string
  /** 当前 tab 的 active 判定 key */
  value: string
  /** 该 tab 请求的天数 */
  days: number
}

export interface RoleDashboardOptions {
  /** 按天统计拉取：入参为天数，返回归一化 stats（失败返回 null） */
  statsFetcher: (days: number) => Promise<RoleDashboardStats | null>
  /** 图表序列类型：line（学习/阅卷分钟曲线）或 bar（阅卷题数柱状） */
  seriesType: 'line' | 'bar'
  /** 统计单位：tooltip 值后缀（分钟 / 题） */
  unit: string
  /** y 轴名称（分钟 / 题数） */
  yAxisName: string
  /** 标题下方摘要文案（基于归一化 stats） */
  summaryText: (stats: RoleDashboardStats) => string
  /** 时间范围 tab（value 判 active，days 决定请求天数） */
  timeTabs: RoleDashboardTimeTab[]
}

export function useRoleDashboard(options: RoleDashboardOptions) {
  const chartRef = ref<HTMLElement | null>(null)
  const { init: initChart } = useECharts(chartRef)

  const timeTabs = options.timeTabs
  const currentTab = ref(timeTabs[0]?.value ?? '')

  const stats = ref<RoleDashboardStats | null>(null)
  const statsLoading = ref(false)
  const statsEmpty = computed(() => {
    if (!stats.value) return true
    return stats.value.data.every((v) => v === 0)
  })
  const summary = computed(() => {
    if (!stats.value) return ''
    return options.summaryText(stats.value)
  })

  async function loadStats() {
    statsLoading.value = true
    try {
      const tab = timeTabs.find((t) => t.value === currentTab.value)
      const days = tab ? tab.days : 7
      const res = await options.statsFetcher(days)
      stats.value = res ?? null
    } catch (error) {
      console.error('加载统计失败:', error)
      stats.value = null
    } finally {
      statsLoading.value = false
    }
    // #506：数据就绪后自动重绘（容器若仍被骨架隐藏则跳过，等 watch(chartRef) 补渲染）
    await nextTick()
    renderChart()
  }

  function renderChart() {
    if (!chartRef.value || !stats.value) return
    // 数据全为 0 时 chartRef 被 v-show 隐藏（display:none），此时初始化会触发
    // ECharts "Can't get DOM width or height" 警告，直接跳过
    if (statsEmpty.value) return

    const labels = stats.value.labels
    const data = stats.value.data

    const common = {
      tooltip: {
        trigger: 'axis',
        backgroundColor: '#fff',
        borderColor: '#E2E8F0',
        borderWidth: 1,
        textStyle: { color: '#0F172A' },
        extraCssText: 'box-shadow: 0 4px 6px -1px rgba(0,0,0,0.1); border-radius: 8px;',
        valueFormatter: (val: any) => `${val} ${options.unit}`
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
      }
    }

    const yAxis: Record<string, unknown> = {
      type: 'value',
      name: options.yAxisName,
      nameTextStyle: { color: '#94A3B8', fontSize: 11 },
      axisLine: { show: false },
      axisTick: { show: false },
      splitLine: { lineStyle: { color: '#F1F5F9' } }
    }
    if (options.seriesType === 'bar') {
      yAxis.minInterval = 1
    }

    // ECharts 在 canvas 绘制，无法消费 CSS 变量：此处字面量需与 --color-primary-500 同步
    const series =
      options.seriesType === 'bar'
        ? [
            {
              type: 'bar',
              data,
              barWidth: '40%',
              itemStyle: {
                color: '#0D9488',
                borderRadius: [4, 4, 0, 0]
              }
            }
          ]
        : [
            {
              type: 'line',
              data,
              smooth: true,
              symbol: 'circle',
              symbolSize: 6,
              lineStyle: { color: '#0D9488', width: 2.5 },
              itemStyle: { color: '#0D9488', borderWidth: 2, borderColor: '#fff' },
              areaStyle: {
                color: {
                  type: 'linear',
                  x: 0,
                  y: 0,
                  x2: 0,
                  y2: 1,
                  colorStops: [
                    { offset: 0, color: 'rgba(13, 148, 136, 0.15)' },
                    { offset: 1, color: 'rgba(13, 148, 136, 0.01)' }
                  ]
                }
              }
            }
          ]

    initChart({
      ...common,
      yAxis,
      series
    })
  }

  // #506 渲染自治：容器一旦挂载且已有 stats 即补渲染——首屏骨架包裹图表（chartRef
  // 晚于数据出现）不再需要调用方手动编排渲染时机。渲染条件与 renderChart 内一致。
  watch(chartRef, (el) => {
    if (el && stats.value && !statsEmpty.value) {
      renderChart()
    }
  })

  // tab 切换时重新加载并重绘（loadStats 尾部已自动重绘，此处不再手动 render）
  watch(currentTab, async () => {
    await loadStats()
  })

  return {
    chartRef,
    timeTabs,
    currentTab,
    stats,
    statsLoading,
    statsEmpty,
    summary,
    loadStats,
    renderChart
  }
}
