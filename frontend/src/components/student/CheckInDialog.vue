<template>
  <UiDialog v-model="visible" title="每日打卡" width="560px" hide-footer @open="onOpen">
    <UiSegmentTabs
      v-model="activeTab"
      :options="[
        { label: '日历签到', value: 'calendar' },
        { label: '打卡排行', value: 'rank' }
      ]"
      class="mb-3"
    />

    <template v-if="activeTab === 'calendar'">
      <div class="mb-3 flex items-center justify-between gap-3">
        <div class="text-[13px] text-ink">
          已连续打卡 <span class="font-semibold text-ui-600">{{ calendar.streak }}</span> 天
          <span class="mx-1.5 text-line-strong">|</span>
          累计打卡 <span class="font-semibold text-ui-600">{{ calendar.total }}</span> 天
        </div>
        <div class="flex items-center gap-1">
          <UiButton variant="text" size="small" @click="prevMonth">‹</UiButton>
          <span class="min-w-20 text-center text-[13px] text-ink-2">{{ calendarYear }}年{{ calendarMonth }}月</span>
          <UiButton variant="text" size="small" @click="nextMonth">›</UiButton>
        </div>
      </div>

      <div class="mb-4 overflow-hidden rounded-[8px] border border-line">
        <div class="grid grid-cols-7 bg-canvas">
          <span v-for="w in weekdays" :key="w" class="py-1.5 text-center text-xs text-ink-3">{{ w }}</span>
        </div>
        <div class="grid grid-cols-7">
          <!--
            每格都显式给出 border-b / border-r：本项目不引 Tailwind preflight，
            浏览器默认 border-width 是 medium(3px)，只给方向不挂宽度会翻车。
          -->
          <span
            v-for="cell in calendarCells"
            :key="cell.key"
            class="border-b border-r border-canvas py-2.5 text-center text-[13px] text-ink [&:nth-child(7n)]:border-r-0"
            :class="{
              'bg-canvas text-ink-muted': !cell.isCurrentMonth,
              'bg-ui-50 font-semibold': cell.isToday && !cell.checked,
              'bg-ui-500 text-panel': cell.checked && !cell.isToday,
              'bg-ui-600 text-panel': cell.checked && cell.isToday
            }"
          >
            {{ cell.day }}
          </span>
        </div>
      </div>

      <div class="flex justify-center">
        <UiButton variant="primary" :loading="checking" :disabled="calendar.today_checked" @click="doCheckIn">
          {{ calendar.today_checked ? '今日已打卡' : '立即打卡' }}
        </UiButton>
      </div>
    </template>

    <template v-else>
      <div v-if="rankMe" class="mb-3 rounded-[8px] border border-ui-100 bg-ui-50 px-3 py-2.5 text-center text-[13px] text-ink">
        我的排名：<span class="font-bold text-ui-600">NO.{{ rankMe.rank }}</span>
        <span class="mx-1.5 text-line-strong">|</span>
        累计 <span class="font-semibold">{{ rankMe.total }}</span> 天
        <span class="mx-1.5 text-line-strong">|</span>
        连续 <span class="font-semibold">{{ rankMe.streak }}</span> 天
      </div>
      <el-table :data="rankItems" stripe size="small" v-loading="rankLoading" max-height="320">
        <el-table-column label="排名" width="70">
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
        <el-table-column label="累计" width="80" prop="total" />
        <el-table-column label="连续" width="80" prop="streak" />
      </el-table>
      <div v-if="rankTotal > rankPageSize" class="mt-3 flex justify-center">
        <el-pagination
          v-model:current-page="rankPage"
          :page-size="rankPageSize"
          :total="rankTotal"
          layout="prev, pager, next"
          size="small"
          @current-change="loadRank"
        />
      </div>
    </template>
  </UiDialog>
</template>

<script setup lang="ts">
import { ref, computed, watch } from 'vue'
import { ElMessage } from 'element-plus'
import { forumApi, type CheckInRankItem, type CheckInRankMe } from '@/api/forum'
import UiButton from '@/components/ui/UiButton.vue'
import UiDialog from '@/components/ui/UiDialog.vue'
import UiTag from '@/components/ui/UiTag.vue'
import UiSegmentTabs from '@/components/ui/UiSegmentTabs.vue'

const visible = defineModel<boolean>({ default: false })
const props = withDefaults(defineProps<{ initialTab?: 'calendar' | 'rank' }>(), { initialTab: 'calendar' })
const emit = defineEmits<{ (e: 'checked', data: { streak: number; total: number; today_checked: boolean }): void }>()

const activeTab = ref<'calendar' | 'rank'>('calendar')
const checking = ref(false)

const calendar = ref<{ dates: string[]; streak: number; total: number; today_checked: boolean }>({
  dates: [],
  streak: 0,
  total: 0,
  today_checked: false
})
const calendarYear = ref(new Date().getFullYear())
const calendarMonth = ref(new Date().getMonth() + 1)

const weekdays = ['日', '一', '二', '三', '四', '五', '六']

const checkedSet = computed(() => new Set(calendar.value.dates))

const calendarCells = computed(() => {
  const y = calendarYear.value
  const m = calendarMonth.value
  const first = new Date(y, m - 1, 1)
  const startWeek = first.getDay()
  const daysInMonth = new Date(y, m, 0).getDate()
  const prevMonthDays = new Date(y, m - 1, 0).getDate()
  const localToday = new Intl.DateTimeFormat('en-CA', { timeZone: 'Asia/Shanghai' }).format(new Date())

  const cells: Array<{ key: string; day: number; isCurrentMonth: boolean; isToday: boolean; checked: boolean }> = []
  // prev month filler
  for (let i = startWeek - 1; i >= 0; i--) {
    const day = prevMonthDays - i
    cells.push({ key: `prev-${day}`, day, isCurrentMonth: false, isToday: false, checked: false })
  }
  for (let d = 1; d <= daysInMonth; d++) {
    const dateStr = `${y}-${String(m).padStart(2, '0')}-${String(d).padStart(2, '0')}`
    cells.push({
      key: `cur-${d}`,
      day: d,
      isCurrentMonth: true,
      isToday: dateStr === localToday,
      checked: checkedSet.value.has(dateStr)
    })
  }
  const totalCells = 42
  const nextNeeded = totalCells - cells.length
  for (let d = 1; d <= nextNeeded; d++) {
    cells.push({ key: `next-${d}`, day: d, isCurrentMonth: false, isToday: false, checked: false })
  }
  return cells
})

async function loadCalendar() {
  try {
    const res = await forumApi.getCheckInCalendar({ year: calendarYear.value, month: calendarMonth.value })
    calendar.value = res
    emit('checked', { streak: res.streak, total: res.total, today_checked: res.today_checked })
  } catch (e) {
    console.error('加载日历失败:', e)
  }
}

function prevMonth() {
  if (calendarMonth.value === 1) {
    calendarMonth.value = 12
    calendarYear.value--
  } else {
    calendarMonth.value--
  }
  loadCalendar()
}

function nextMonth() {
  if (calendarMonth.value === 12) {
    calendarMonth.value = 1
    calendarYear.value++
  } else {
    calendarMonth.value++
  }
  loadCalendar()
}

async function doCheckIn() {
  if (calendar.value.today_checked) return
  checking.value = true
  try {
    const res = await forumApi.checkIn()
    ElMessage.success(`打卡成功，已连续 ${res.streak} 天`)
    calendar.value.streak = res.streak
    calendar.value.total = res.total
    calendar.value.today_checked = res.today_checked
    // 若当前月是本月，补上今日（Asia/Shanghai）
    const beijingStr = new Intl.DateTimeFormat('en-CA', { timeZone: 'Asia/Shanghai' }).format(new Date())
    const [by, bm] = beijingStr.split('-').map(Number)
    if (calendarYear.value === by && calendarMonth.value === bm) {
      if (!checkedSet.value.has(beijingStr)) {
        calendar.value.dates = [...calendar.value.dates, beijingStr]
      }
    }
    emit('checked', { streak: res.streak, total: res.total, today_checked: res.today_checked })
    if (activeTab.value === 'rank') loadRank()
  } catch (e) {
    console.error('打卡失败:', e)
  } finally {
    checking.value = false
  }
}

// rank
const rankItems = ref<CheckInRankItem[]>([])
const rankMe = ref<CheckInRankMe | null>(null)
const rankTotal = ref(0)
const rankPage = ref(1)
const rankPageSize = ref(20)
const rankLoading = ref(false)

async function loadRank() {
  rankLoading.value = true
  try {
    const res = await forumApi.getCheckInRank({ page: rankPage.value, page_size: rankPageSize.value })
    rankItems.value = res.items || []
    rankTotal.value = res.total || 0
    rankMe.value = res.me
  } catch (e) {
    console.error('加载排行失败:', e)
  } finally {
    rankLoading.value = false
  }
}

watch(activeTab, (tab) => {
  if (tab === 'rank') loadRank()
  else loadCalendar()
})

function onOpen() {
  activeTab.value = props.initialTab
  if (activeTab.value === 'calendar') loadCalendar()
  else loadRank()
}

watch(
  () => props.initialTab,
  v => {
    activeTab.value = v
  }
)

defineExpose({ loadCalendar, loadRank })
</script>
