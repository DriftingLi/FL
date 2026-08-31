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
    </div>

    <UiErrorState
      v-if="loadError"
      title="任务加载失败"
      description="网络或服务端异常，可重试"
      :retrying="retrying"
      @retry="retryLoad"
    />

    <UiSkeleton v-else-if="loading" variant="card" :count="6" />

    <div v-else class="group-stack">
      <div
        v-for="(group, gi) in grouped"
        :key="group.key"
        class="group-section stagger-in"
        :style="staggerStyle(gi)"
      >
        <div class="group-head">
          <span class="group-title">{{ group.label }}</span>
          <span class="group-desc">{{ group.desc }}</span>
          <span class="group-count">{{ group.tasks.length }}项</span>
        </div>
        <div class="task-list">
          <UiCard v-for="task in group.tasks" :key="task.code" class="task-card" :class="task.status" padding="base">
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
                  <UiProgress
                    :value="Math.round(((task.progress || 0) / task.total) * 100)"
                    size="sm"
                    tone="brand"
                    class="progress-bar"
                  />
                  <span class="progress-text">{{ task.progress }}/{{ task.total }}</span>
                </div>
              </div>
            </div>
            <div class="task-right">
              <span class="task-points">+{{ task.points }}</span>
              <UiButton variant="primary" v-if="task.status === 'claimable'" size="small" :loading="claimingCode === task.code" @click="handleClaim(task.code)">
                领取
              </UiButton>
              <UiButton variant="ghost" v-else-if="task.status === 'todo'" size="small" disabled>去完成</UiButton>
              <el-tag v-else type="success" size="small" effect="plain">已领取</el-tag>
            </div>
          </UiCard>
        </div>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed, ref, onMounted } from 'vue'
import { ElMessage } from 'element-plus'
import { CircleCheckFilled, Trophy, List } from '@element-plus/icons-vue'
import { pointsApi, type PointsTaskItem } from '@/api/points'
import { groupLabelMap, groupDescMap } from '@/utils/taskCenter'
import type { TaskGroup } from '@/utils/taskCenter'
import { useAsyncPage } from '@/composables/useAsyncPage'
import { useStagger } from '@/composables/useStagger'
import UiProgress from '@/components/ui/UiProgress.vue'
import UiErrorState from '@/components/ui/UiErrorState.vue'
import UiSkeleton from '@/components/ui/UiSkeleton.vue'
import UiButton from '@/components/ui/UiButton.vue'
import UiCard from '@/components/ui/UiCard.vue'

const staggerStyle = useStagger(6)

const tasks = ref<PointsTaskItem[]>([])
const points = ref({ balance: 0, totalEarned: 0 })
const claimingCode = ref<string | null>(null)

// 三态收编（#388）：加载失败进错误态可重试（原「静默回退本地占位」退役，
// 占位数据不再是后端异常的正确呈现）
const { loading, loadError, retrying, retry: retryLoad, run: refresh } = useAsyncPage(async () => {
  const [bal, ts] = await Promise.all([pointsApi.getBalance(), pointsApi.getTasks()])
  points.value = { balance: bal.balance, totalEarned: bal.total_earned }
  tasks.value = ts.tasks || []
})

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
  claimingCode.value = code
  try {
    const res = await pointsApi.claim(code)
    task.status = 'claimed'
    task.progress = task.total
    points.value.balance = res.balance
    points.value.totalEarned = res.total_earned
    // 强制刷新以确保与服务端一致
    tasks.value = [...tasks.value]
    ElMessage.success(`已领取 +${task.points} 积分`)
    // 后台静默刷新，确保幂等状态持久
    void refresh()
  } catch (e: unknown) {
    const msg = e instanceof Error ? e.message : String(e)
    if (msg.includes('已领取') || msg.includes('今日已领取')) {
      task.status = 'claimed'
      task.progress = task.total
      tasks.value = [...tasks.value]
      ElMessage.warning(msg)
    } else {
      ElMessage.error(msg || '领取失败')
    }
  } finally {
    claimingCode.value = null
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
  color: var(--color-text-primary);
  margin: 0;
}
.points-summary {
  display: flex;
  align-items: center;
  gap: 16px;
  padding: 14px 16px;
  background: var(--color-bg-card);
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
  color: var(--color-text-tertiary);
  letter-spacing: 0.04em;
}
.summary-value {
  font-size: 20px;
  font-weight: 800;
  color: var(--color-text-primary);
  font-family: var(--font-display, system-ui);
}
.summary-value.small {
  font-size: 16px;
}
.summary-divider {
  width: 1px;
  height: 32px;
  background: var(--color-border-light);
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
  color: var(--color-text-primary);
}
.group-desc {
  font-size: 12px;
  color: var(--color-text-tertiary);
}
.group-count {
  margin-left: auto;
  font-size: 12px;
  color: var(--color-text-tertiary);
  background: var(--color-bg-page);
  border: 1px solid var(--color-border-light);
  padding: 2px 8px;
  border-radius: 9999px;
}
.task-list {
  display: flex;
  flex-direction: column;
  gap: 10px;
}
/* 容器（描边 / 圆角 / 内距 / 底色 / 投影）已由 UiCard 承担，此处只留卡片内部的横向布局 */
.task-card {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
}
.task-card:hover {
  border-color: var(--color-border-dark);
}
.task-card.claimed {
  opacity: 0.7;
  background: var(--color-bg-page);
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
  background: var(--color-bg-page);
  color: var(--color-text-tertiary);
  border: 1px solid var(--color-border-light);
}
.task-icon.claimable {
  background: var(--color-warning-light);
  color: var(--color-warning-strong);
  border: 1px solid var(--color-warning-light);
}
.task-icon.claimed {
  background: var(--color-success-light);
  color: var(--color-success-strong);
  border: 1px solid var(--color-success-light);
}
.task-info {
  flex: 1;
  min-width: 0;
}
.task-title-text {
  font-size: 14px;
  font-weight: 600;
  color: var(--color-text-primary);
  margin-bottom: 2px;
}
.task-desc {
  font-size: 12px;
  color: var(--color-text-tertiary);
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
  color: var(--color-text-tertiary);
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
  color: var(--color-warning-strong);
  min-width: 36px;
  text-align: right;
}
</style>
