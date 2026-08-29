<template>
  <el-dialog v-model="visible" title="每日打卡" width="560px" @open="onOpen">
    <el-tabs v-model="activeTab" @tab-change="handleTabChange">
      <el-tab-pane label="日历签到" name="calendar">
        <div class="calendar-header">
          <div class="streak-info">
            已连续打卡 <span class="num">{{ calendar.streak }}</span> 天
            <span class="divider">|</span>
            累计打卡 <span class="num">{{ calendar.total }}</span> 天
          </div>
          <div class="month-nav">
            <el-button text size="small" @click="prevMonth">‹</el-button>
            <span class="month-label">{{ calendarYear }}年{{ calendarMonth }}月</span>
            <el-button text size="small" @click="nextMonth">›</el-button>
          </div>
        </div>

        <div class="calendar-grid">
          <div class="weekday-row">
            <span v-for="w in weekdays" :key="w" class="weekday">{{ w }}</span>
          </div>
          <div class="days-grid">
            <span
              v-for="cell in calendarCells"
              :key="cell.key"
              class="day-cell"
              :class="{
                'other-month': !cell.isCurrentMonth,
                'today': cell.isToday,
                'checked': cell.checked,
              }"
            >
              {{ cell.day }}
            </span>
          </div>
        </div>

        <div class="calendar-action">
          <el-button type="primary" :loading="checking" :disabled="calendar.today_checked" @click="doCheckIn">
            {{ calendar.today_checked ? '今日已打卡' : '立即打卡' }}
          </el-button>
        </div>
      </el-tab-pane>

      <el-tab-pane label="打卡排行" name="rank">
        <div v-if="rankMe" class="my-rank-card">
          我的排名：<span class="rank-num">NO.{{ rankMe.rank }}</span>
          <span class="divider">|</span>
          累计 <span class="num">{{ rankMe.total }}</span> 天
          <span class="divider">|</span>
          连续 <span class="num">{{ rankMe.streak }}</span> 天
        </div>
        <el-table :data="rankItems" stripe size="small" v-loading="rankLoading" max-height="320">
          <el-table-column label="排名" width="70">
            <template #default="{ row }"> {{ row.rank }} </template>
          </el-table-column>
          <el-table-column label="学员">
            <template #default="{ row }">
              <div class="rank-user">
                <el-avatar :size="24" :src="row.user.avatar_url || undefined">{{ (row.user.username || '?').charAt(0).toUpperCase() }}</el-avatar>
                <span class="rank-name">{{ row.user.username }}</span>
                <el-tag v-if="row.today_checked" size="small" type="success" class="today-tag">今日已打卡</el-tag>
              </div>
            </template>
          </el-table-column>
          <el-table-column label="累计" width="80" prop="total" />
          <el-table-column label="连续" width="80" prop="streak" />
        </el-table>
        <div v-if="rankTotal > rankPageSize" class="rank-pagination">
          <el-pagination
            v-model:current-page="rankPage"
            :page-size="rankPageSize"
            :total="rankTotal"
            layout="prev, pager, next"
            size="small"
            @current-change="loadRank"
          />
        </div>
      </el-tab-pane>
    </el-tabs>
  </el-dialog>
</template>

<script setup lang="ts">
import { ref, computed, watch } from 'vue'
import { ElMessage } from 'element-plus'
import { forumApi, type CheckInRankItem, type CheckInRankMe } from '@/api/forum'

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

function handleTabChange(tab: string) {
  if (tab === 'rank') loadRank()
  if (tab === 'calendar') loadCalendar()
}

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

<style scoped>
.calendar-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  margin-bottom: 12px;
}

.streak-info {
  font-size: 13px;
  color: #303133;
}

.streak-info .num {
  font-weight: 600;
  color: #409eff;
}

.divider {
  margin: 0 6px;
  color: #dcdfe6;
}

.month-nav {
  display: flex;
  align-items: center;
  gap: 4px;
}

.month-label {
  font-size: 13px;
  color: #606266;
  min-width: 80px;
  text-align: center;
}

.calendar-grid {
  border: 1px solid #ebeef5;
  border-radius: 8px;
  overflow: hidden;
  margin-bottom: 16px;
}

.weekday-row {
  display: grid;
  grid-template-columns: repeat(7, 1fr);
  background: #f5f7fa;
}

.weekday {
  text-align: center;
  font-size: 12px;
  color: #909399;
  padding: 6px 0;
}

.days-grid {
  display: grid;
  grid-template-columns: repeat(7, 1fr);
}

.day-cell {
  text-align: center;
  font-size: 13px;
  color: #303133;
  padding: 10px 0;
  border-right: 1px solid #f5f7fa;
  border-bottom: 1px solid #f5f7fa;
  position: relative;
}

.day-cell:nth-child(7n) {
  border-right: none;
}

.day-cell.other-month {
  color: #c0c4cc;
  background: #fafafa;
}

.day-cell.today {
  background: #ecf5ff;
  font-weight: 600;
}

.day-cell.checked {
  background: #409eff;
  color: #fff;
}

.day-cell.checked.today {
  background: #337ecc;
}

.calendar-action {
  display: flex;
  justify-content: center;
}

.my-rank-card {
  background: #f0f7ff;
  border: 1px solid #d9ecff;
  border-radius: 8px;
  padding: 10px 12px;
  font-size: 13px;
  color: #303133;
  margin-bottom: 12px;
  text-align: center;
}

.my-rank-card .rank-num {
  font-weight: 700;
  color: #409eff;
}

.my-rank-card .num {
  font-weight: 600;
  color: #303133;
}

.rank-user {
  display: flex;
  align-items: center;
  gap: 8px;
}

.rank-name {
  font-size: 13px;
  color: #303133;
}

.today-tag {
  margin-left: 6px;
}

.rank-pagination {
  display: flex;
  justify-content: center;
  margin-top: 12px;
}
</style>
