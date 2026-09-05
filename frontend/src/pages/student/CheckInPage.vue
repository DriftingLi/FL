<template>
  <div class="mx-auto max-w-[960px] px-4 pb-10">
    <div class="mb-4 flex items-center justify-between gap-3">
      <h1 class="m-0 text-2xl font-semibold text-ink">每日打卡</h1>
      <RouterLink
        to="/training/task-center/points"
        class="inline-flex items-center gap-1 rounded-pill border border-line-strong bg-panel px-3 py-1.5 text-xs font-semibold text-ink-2 transition-colors duration-150 hover:border-ui-300 hover:text-ui-600"
      >
        积分明细
        <el-icon><ArrowRight /></el-icon>
      </RouterLink>
    </div>

    <!-- 概览：连击 / 累计 / 今日得分 -->
    <div class="mb-4 grid grid-cols-3 gap-3">
      <div class="rounded-card border border-line bg-panel px-4 py-3 text-center shadow-card">
        <div class="text-[11px] tracking-[0.04em] text-ink-3">连续打卡</div>
        <div class="font-heading text-2xl font-extrabold text-ui-600">{{ calendar.streak }}<span class="ml-1 text-xs font-semibold text-ink-3">天</span></div>
      </div>
      <div class="rounded-card border border-line bg-panel px-4 py-3 text-center shadow-card">
        <div class="text-[11px] tracking-[0.04em] text-ink-3">累计打卡</div>
        <div class="font-heading text-2xl font-extrabold text-ink">{{ calendar.total }}<span class="ml-1 text-xs font-semibold text-ink-3">天</span></div>
      </div>
      <div class="rounded-card border border-line bg-panel px-4 py-3 text-center shadow-card">
        <div class="text-[11px] tracking-[0.04em] text-ink-3">今日得分</div>
        <div class="font-heading text-2xl font-extrabold text-warn-strong">{{ todayPointsText }}</div>
      </div>
    </div>

    <UiErrorState
      v-if="loadError"
      title="打卡数据加载失败"
      description="网络或服务端异常，可重试"
      :retrying="retrying"
      @retry="retryLoad"
    />
    <UiSkeleton v-else-if="loading" variant="card" :count="3" />

    <template v-else>
      <div class="rounded-card border border-line bg-panel p-4 shadow-card">
        <!-- 月份切换 + 打卡按钮 -->
        <div class="mb-3 flex items-center justify-between gap-3">
          <div class="flex items-center gap-1">
            <UiButton variant="text" size="small" @click="prevMonth">‹</UiButton>
            <span class="min-w-24 text-center text-[13px] font-semibold text-ink-2">{{ viewYear }}年{{ viewMonth }}月</span>
            <UiButton variant="text" size="small" @click="nextMonth">›</UiButton>
          </div>
          <div class="flex items-center gap-2">
            <!-- 连续段图例：主色实心=当前连续；浅底=已断开的打卡；灰底=未打卡 -->
            <span class="hidden items-center gap-1 text-[11px] text-ink-3 sm:flex">
              <i class="inline-block size-2.5 rounded-[3px] bg-ui-500"></i>连续中
              <i class="ml-2 inline-block size-2.5 rounded-[3px] bg-ui-100"></i>已断开
              <i class="ml-2 inline-block size-2.5 rounded-[3px] bg-canvas border border-line"></i>未打卡
            </span>
            <UiButton variant="primary" size="small" :loading="checking" :disabled="calendar.today_checked" @click="doCheckIn">
              {{ calendar.today_checked ? '今日已打卡' : '立即打卡' }}
            </UiButton>
          </div>
        </div>

        <div class="mb-3 flex items-center gap-3 text-[13px] text-ink">
          每日 <span class="font-semibold text-ui-600">+5</span>
          <span class="text-line-strong">|</span>
          连续满
          <span class="font-semibold">3</span> 天 +5
          <span class="font-semibold">7</span> 天 +10
          <span class="font-semibold">30</span> 天 +50
          <span class="text-xs text-ink-3">（断签重新起算，跨档当日随打卡到账）</span>
        </div>

        <div class="overflow-hidden rounded-[8px] border border-line">
          <div class="grid grid-cols-7 bg-canvas">
            <span v-for="w in weekdays" :key="w" class="py-1.5 text-center text-xs text-ink-3">{{ w }}</span>
          </div>
          <div class="grid grid-cols-7">
            <div
              v-for="cell in calendarCells"
              :key="cell.key"
              class="relative border-b border-r border-canvas py-2.5 text-center text-[13px] text-ink [&:nth-child(7n)]:border-r-0"
              :class="cellClass(cell)"
            >
              <span class="font-medium">{{ cell.day }}</span>
              <span v-if="cell.checked && cell.points > 0" class="absolute right-0.5 top-0.5 text-[9px] leading-none text-ink-3">
                +{{ cell.points }}
              </span>
            </div>
          </div>
        </div>
      </div>

      <!-- 排行榜 -->
      <div class="mt-5">
        <div class="mb-2.5 flex items-center gap-2">
          <span class="text-[15px] font-bold text-ink">打卡排行</span>
          <span class="ml-auto text-xs text-ink-3">按累计天数排序</span>
        </div>
        <div v-if="rankMe" class="mb-3 rounded-[8px] border border-ui-100 bg-ui-50 px-3 py-2.5 text-center text-[13px] text-ink">
          我的排名：<span class="font-bold text-ui-600">NO.{{ rankMe.rank }}</span>
          <span class="mx-1.5 text-line-strong">|</span>
          累计 <span class="font-semibold">{{ rankMe.total }}</span> 天
          <span class="mx-1.5 text-line-strong">|</span>
          连续 <span class="font-semibold">{{ rankMe.streak }}</span> 天
        </div>
        <div class="overflow-hidden rounded-card border border-line bg-panel shadow-card">
          <div v-if="rankLoading" class="p-6 text-center text-sm text-ink-3">加载中…</div>
          <el-table v-else :data="rankItems" stripe size="small" max-height="360">
            <el-table-column label="排名" width="80">
              <template #default="{ row }"> {{ row.rank }} </template>
            </el-table-column>
            <el-table-column label="学员">
              <template #default="{ row }">
                <div class="flex items-center gap-2">
                  <el-avatar :size="24" :src="row.user.avatar_url || undefined">{{ (row.user.username || '?').charAt(0).toUpperCase() }}</el-avatar>
                  <span class="text-[13px] text-ink">{{ row.user.username }}</span>
                  <UiTag v-if="row.today_checked" tone="success" class="ml-1.5">今日已打卡</UiTag>
                </div>
              </template>
            </el-table-column>
            <el-table-column label="累计" width="90" prop="total" />
            <el-table-column label="连续" width="90" prop="streak" />
          </el-table>
          <div v-if="rankTotal > rankPageSize" class="flex justify-center py-3">
            <el-pagination
              v-model:current-page="rankPage"
              :page-size="rankPageSize"
              :total="rankTotal"
              layout="prev, pager, next"
              size="small"
              @current-change="loadRank"
            />
          </div>
        </div>
      </div>
    </template>
  </div>
</template>

<script setup lang="ts">
import { ref, computed, onMounted } from 'vue'
import { ElMessage } from 'element-plus'
import { ArrowRight } from '@element-plus/icons-vue'
import { checkInApi, type CheckInDay, type CheckInRankItem, type CheckInRankMe } from '@/api/checkin'
import { shanghaiDateStr } from '@/utils/format'
import UiButton from '@/components/ui/UiButton.vue'
import UiTag from '@/components/ui/UiTag.vue'
import UiErrorState from '@/components/ui/UiErrorState.vue'
import UiSkeleton from '@/components/ui/UiSkeleton.vue'
import { useAsyncPage } from '@/composables/useAsyncPage'

const weekdays = ['日', '一', '二', '三', '四', '五', '六']

interface CalendarCell {
  key: string
  day: number
  isCurrentMonth: boolean
  isToday: boolean
  checked: boolean
  points: number
  /** 属于「当前正在连续的打卡段」（从今日/昨日连续回溯到该日） */
  inActiveStreak: boolean
}

const calendar = ref<{ days: CheckInDay[]; streak: number; total: number; today_checked: boolean }>({
  days: [],
  streak: 0,
  total: 0,
  today_checked: false
})
const viewYear = ref(new Date().getFullYear())
const viewMonth = ref(new Date().getMonth() + 1)
const checking = ref(false)

// 排行榜
const rankItems = ref<CheckInRankItem[]>([])
const rankMe = ref<CheckInRankMe | null>(null)
const rankTotal = ref(0)
const rankPage = ref(1)
const rankPageSize = ref(20)
const rankLoading = ref(false)

const { loading, loadError, retrying, retry: retryLoad, run: refresh } = useAsyncPage(async () => {
  const cal = await checkInApi.getCalendar({ year: viewYear.value, month: viewMonth.value })
  calendar.value = cal
  return true
})

const checkedByDate = computed(() => {
  const m = new Map<string, CheckInDay>()
  for (const d of calendar.value.days) m.set(d.date, d)
  return m
})

const pointsByDate = computed(() => {
  const m = new Map<string, number>()
  for (const d of calendar.value.days) m.set(d.date, d.points)
  return m
})

/** 当前连续段 [start, end] 日期串（streak 由后端窗口口径给出，跨月连续也能正确定段：
 * 段尾=今日（已签）或昨日（今日未签），段长=streak，段起点=段尾−(streak−1)） */
function activeStreakRange(): { start: string; end: string } | null {
  const streak = calendar.value.streak
  if (streak <= 0) return null
  const end = new Date()
  const endStr = shanghaiDateStr(end)
  const set = new Set(checkedByDate.value.keys())
  if (!set.has(endStr)) {
    // 今日未签：段尾退到昨日；若昨日也未签则无连续段（后端 streak 应已为 0）
    end.setDate(end.getDate() - 1)
  }
  const start = new Date(end)
  start.setDate(start.getDate() - (streak - 1))
  return { start: shanghaiDateStr(start), end: shanghaiDateStr(end) }
}

const activeRange = computed(() => activeStreakRange())

const calendarCells = computed<CalendarCell[]>(() => {
  const y = viewYear.value
  const m = viewMonth.value
  const first = new Date(y, m - 1, 1)
  const startWeek = first.getDay()
  const daysInMonth = new Date(y, m, 0).getDate()
  const prevMonthDays = new Date(y, m - 1, 0).getDate()
  const localToday = shanghaiDateStr(new Date())
  const range = activeRange.value

  const cells: CalendarCell[] = []
  for (let i = startWeek - 1; i >= 0; i--) {
    cells.push({ key: `prev-${prevMonthDays - i}`, day: prevMonthDays - i, isCurrentMonth: false, isToday: false, checked: false, points: 0, inActiveStreak: false })
  }
  for (let d = 1; d <= daysInMonth; d++) {
    const dateStr = `${y}-${String(m).padStart(2, '0')}-${String(d).padStart(2, '0')}`
    const checked = checkedByDate.value.has(dateStr)
    // 连续段成员 = 在 [range.start, range.end] 内且已打卡（range 由后端 streak 定段，跨月正确）
    const inActive = checked && range != null && dateStr >= range.start && dateStr <= range.end
    cells.push({
      key: `cur-${d}`,
      day: d,
      isCurrentMonth: true,
      isToday: dateStr === localToday,
      checked,
      points: pointsByDate.value.get(dateStr) ?? 0,
      inActiveStreak: inActive
    })
  }
  const totalCells = 42
  const nextNeeded = totalCells - cells.length
  for (let d = 1; d <= nextNeeded; d++) {
    cells.push({ key: `next-${d}`, day: d, isCurrentMonth: false, isToday: false, checked: false, points: 0, inActiveStreak: false })
  }
  return cells
})

function cellClass(cell: CalendarCell) {
  return {
    'bg-canvas text-ink-muted': !cell.isCurrentMonth,
    'bg-ui-50 font-semibold': cell.isToday && !cell.checked,
    'bg-ui-100 text-ink-2': cell.checked && !cell.inActiveStreak && !cell.isToday,
    'bg-ui-500 text-panel': cell.checked && cell.inActiveStreak && !cell.isToday,
    'bg-ui-600 text-panel': cell.checked && cell.inActiveStreak && cell.isToday,
    'ring-2 ring-inset ring-ui-400': cell.isToday
  }
}

const todayPointsText = computed(() => {
  const todayStr = shanghaiDateStr(new Date())
  if (!calendar.value.today_checked) return '—'
  const p = pointsByDate.value.get(todayStr)
  return p != null && p > 0 ? `+${p}` : '+0'
})

async function loadCalendar() {
  try {
    const res = await checkInApi.getCalendar({ year: viewYear.value, month: viewMonth.value })
    calendar.value = res
  } catch (e) {
    console.error('加载日历失败:', e)
  }
}

function prevMonth() {
  if (viewMonth.value === 1) {
    viewMonth.value = 12
    viewYear.value--
  } else {
    viewMonth.value--
  }
  loadCalendar()
}

function nextMonth() {
  if (viewMonth.value === 12) {
    viewMonth.value = 1
    viewYear.value++
  } else {
    viewMonth.value++
  }
  loadCalendar()
}

async function doCheckIn() {
  if (calendar.value.today_checked) return
  checking.value = true
  try {
    const res = await checkInApi.checkIn()
    const gained = res.points > 0 ? `，+${res.points} 积分已到账` : ''
    ElMessage.success(`打卡成功，已连续 ${res.streak} 天${gained}`)
    // 后端已落「今日已签」记录：直接重拉日历（整月逐日契约，含今日 checked/points）
    await loadCalendar()
    loadRank()
  } catch (e) {
    ElMessage.error((e instanceof Error ? e.message : '打卡失败，请重试') || '打卡失败，请重试')
  } finally {
    checking.value = false
  }
}

async function loadRank() {
  rankLoading.value = true
  try {
    const res = await checkInApi.getRank({ page: rankPage.value, page_size: rankPageSize.value })
    rankItems.value = res.items || []
    rankTotal.value = res.total || 0
    rankMe.value = res.me
  } catch (e) {
    console.error('加载排行失败:', e)
  } finally {
    rankLoading.value = false
  }
}

onMounted(() => {
  refresh()
  loadRank()
})
</script>
