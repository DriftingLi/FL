<template>
  <div class="task-center-page">
    <div class="task-header">
      <h1 class="task-title">任务中心</h1>
    </div>

    <div class="points-summary">
      <div class="summary-item">
        <span class="summary-label">当前积分</span>
        <span class="summary-value">{{ points.balance }}</span>
      </div>
      <div class="summary-divider"></div>
      <div class="summary-item">
        <span class="summary-label">今日可得</span>
        <span class="summary-value small">+{{ todayEarnable }}</span>
      </div>
      <div class="summary-divider"></div>
      <div class="summary-item">
        <span class="summary-label">累计获得</span>
        <span class="summary-value small">{{ points.totalEarned }}</span>
      </div>
      <div class="summary-hint">做任务 → 领取 → 前端累加</div>
    </div>

    <div class="group-stack">
      <div v-for="group in grouped" :key="group.key" class="group-section">
        <div class="group-head">
          <span class="group-title">{{ group.label }}</span>
          <span class="group-desc">{{ group.desc }}</span>
          <span class="group-count">{{ group.tasks.length }}项</span>
        </div>
        <div class="task-list">
          <div v-for="task in group.tasks" :key="task.code" class="task-card" :class="task.status">
            <div class="task-left">
              <div class="task-icon" :class="task.status">
                <el-icon v-if="task.status === 'claimed'"><CircleCheckFilled /></el-icon>
                <el-icon v-else-if="task.status === 'claimable'"><Trophy /></el-icon>
                <el-icon v-else><List /></el-icon>
              </div>
              <div class="task-info">
                <div class="task-title-text">{{ task.title }}</div>
                <div class="task-desc">{{ task.desc }}</div>
                <div v-if="task.total && task.total > 1" class="task-progress">
                  <el-progress :percentage="Math.round(((task.progress || 0) / task.total) * 100)" :stroke-width="6" :show-text="false" class="progress-bar" />
                  <span class="progress-text">{{ task.progress }}/{{ task.total }}</span>
                </div>
              </div>
            </div>
            <div class="task-right">
              <span class="task-points">+{{ task.points }}</span>
              <el-button
                v-if="task.status === 'claimable'"
                size="small"
                type="primary"
                :loading="loading"
                @click="handleClaim(task.code)"
              >
                领取
              </el-button>
              <el-button v-else-if="task.status === 'todo'" size="small" plain disabled>去完成</el-button>
              <el-tag v-else type="success" size="small" effect="plain">已领取</el-tag>
            </div>
          </div>
        </div>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed, ref, onMounted } from 'vue'
import { ElMessage } from 'element-plus'
import { CircleCheckFilled, Trophy, List } from '@element-plus/icons-vue'
import { useAuthStore } from '@/stores/auth'
import { pointsApi, type PointsTaskItem } from '@/api/points'
import { groupLabelMap, groupDescMap } from '@/utils/taskCenter'
import type { TaskGroup } from '@/utils/taskCenter'
import { loadTasks as loadLocalTasks, loadPoints as loadLocalPoints } from '@/utils/taskCenter'

const authStore = useAuthStore()

const tasks = ref<PointsTaskItem[]>([])
const points = ref({ balance: 0, totalEarned: 0 })
const loading = ref(false)

async function refresh() {
  loading.value = true
  try {
    const [bal, ts] = await Promise.all([pointsApi.getBalance(), pointsApi.getTasks()])
    points.value = { balance: bal.balance, totalEarned: bal.total_earned }
    tasks.value = ts.tasks || []
  } catch {
    // 静默回退占位（无后端时）
    const uid = authStore.userInfo?.user_id
    const localTasks = loadLocalTasks(uid) as unknown as PointsTaskItem[]
    const localPoints = loadLocalPoints(uid)
    tasks.value = localTasks
    points.value = localPoints
  } finally {
    loading.value = false
  }
}

const todayEarnable = computed(() =>
  tasks.value.filter((t) => t.status !== 'claimed').reduce((sum, t) => sum + t.points, 0),
)

const grouped = computed(() => {
  const order: TaskGroup[] = ['daily', 'newbie', 'growth']
  return order.map((key) => ({
    key,
    label: groupLabelMap[key],
    desc: groupDescMap[key],
    tasks: tasks.value.filter((t) => t.group === key),
  }))
})

async function handleClaim(code: string) {
  const task = tasks.value.find((t) => t.code === code)
  if (!task || task.status !== 'claimable') return
  try {
    const res = await pointsApi.claim(code)
    task.status = 'claimed'
    points.value.balance = res.balance
    points.value.totalEarned = res.total_earned
    ElMessage.success(`已领取 +${task.points} 积分`)
  } catch (e: unknown) {
    const msg = e instanceof Error ? e.message : String(e)
    if (msg.includes('已领取') || msg.includes('今日已领取')) {
      task.status = 'claimed'
      ElMessage.warning(msg)
    } else {
      ElMessage.error(msg || '领取失败')
    }
  }
}

onMounted(() => {
  refresh()
})
</script>

<style scoped>
.task-center-page {
  max-width: 960px;
  margin: 0 auto;
  padding: 0 16px 40px;
}
.task-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  margin-bottom: 16px;
}
.task-title {
  font-size: 24px;
  font-weight: 600;
  color: #303133;
  margin: 0;
}
.points-summary {
  display: flex;
  align-items: center;
  gap: 16px;
  padding: 14px 16px;
  background: #fff;
  border-radius: 12px;
  box-shadow: 0 2px 12px rgba(0, 0, 0, 0.06);
  margin-bottom: 16px;
  flex-wrap: wrap;
}
.summary-item {
  display: flex;
  flex-direction: column;
  gap: 2px;
  min-width: 90px;
}
.summary-label {
  font-size: 11px;
  color: #909399;
  letter-spacing: 0.04em;
}
.summary-value {
  font-size: 20px;
  font-weight: 800;
  color: #303133;
  font-family: var(--font-display, system-ui);
}
.summary-value.small {
  font-size: 16px;
}
.summary-divider {
  width: 1px;
  height: 32px;
  background: #ebeef5;
}
.summary-hint {
  margin-left: auto;
  font-size: 12px;
  color: #c0c4cc;
}
.group-stack {
  display: flex;
  flex-direction: column;
  gap: 20px;
}
.group-head {
  display: flex;
  align-items: center;
  gap: 8px;
  margin-bottom: 10px;
}
.group-title {
  font-size: 15px;
  font-weight: 700;
  color: #303133;
}
.group-desc {
  font-size: 12px;
  color: #909399;
}
.group-count {
  margin-left: auto;
  font-size: 12px;
  color: #909399;
  background: #f5f7fa;
  border: 1px solid #ebeef5;
  padding: 2px 8px;
  border-radius: 9999px;
}
.task-list {
  display: flex;
  flex-direction: column;
  gap: 10px;
}
.task-card {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
  padding: 14px 16px;
  background: #fff;
  border: 1px solid #ebeef5;
  border-radius: 12px;
  box-shadow: 0 2px 12px rgba(0, 0, 0, 0.04);
  transition: border-color 150ms ease;
}
.task-card:hover {
  border-color: #dcdfe6;
}
.task-card.claimed {
  opacity: 0.7;
  background: #fafafa;
}
.task-left {
  display: flex;
  align-items: flex-start;
  gap: 12px;
  flex: 1;
  min-width: 0;
}
.task-icon {
  width: 36px;
  height: 36px;
  border-radius: 9999px;
  display: flex;
  align-items: center;
  justify-content: center;
  flex-shrink: 0;
  font-size: 18px;
}
.task-icon.todo {
  background: #f5f7fa;
  color: #909399;
  border: 1px solid #ebeef5;
}
.task-icon.claimable {
  background: #fef3c7;
  color: #d97706;
  border: 1px solid #fde68a;
}
.task-icon.claimed {
  background: #dcfce7;
  color: #16a34a;
  border: 1px solid #bbf7d0;
}
.task-info {
  flex: 1;
  min-width: 0;
}
.task-title-text {
  font-size: 14px;
  font-weight: 600;
  color: #303133;
  margin-bottom: 2px;
}
.task-desc {
  font-size: 12px;
  color: #909399;
  line-height: 1.4;
}
.task-progress {
  display: flex;
  align-items: center;
  gap: 8px;
  margin-top: 6px;
}
.progress-bar {
  width: 90px;
  flex-shrink: 0;
}
.progress-text {
  font-size: 11px;
  color: #909399;
  font-family: monospace;
}
.task-right {
  display: flex;
  align-items: center;
  gap: 10px;
  flex-shrink: 0;
}
.task-points {
  font-size: 14px;
  font-weight: 700;
  color: #d97706;
  min-width: 36px;
  text-align: right;
}
</style>
