<!-- PROTOTYPE — throwaway UI. Three variants of 任务中心 + 积分占位, switchable via ?variant=a|b|c -->
<!-- Question: 任务中心以何种信息密度与激励感呈现？Variants: A 卡片分组列表 / B 仪表盘+紧凑清单 / C 成就墙时间线 -->
<template>
  <div class="task-center-prototype">
    <div class="proto-banner">
      <el-tag size="small" type="warning" effect="plain">PROTOTYPE — throwaway</el-tag>
      <span class="proto-banner-text">
        任务中心 + 积分占位 · 三变体对比（?variant=a|b|c，←/→ 切换）· 纯前端 localStorage · 无后端
      </span>
    </div>

    <div class="proto-header">
      <div>
        <h2 class="proto-title">任务中心</h2>
        <p class="proto-sub">
          三组 9 任务
          <span class="proto-sub-sep">·</span>
          每日任务每日 0 点重置文案占位
          <span class="proto-sub-sep">·</span>
          终态落位 <code class="proto-code">个人 → 任务中心 /training/task-center</code>
        </p>
      </div>
      <div class="proto-actions">
        <el-button size="small" @click="handleReset">重置示例</el-button>
        <el-button size="small" type="info" plain @click="simulateProgress">模拟完成一项</el-button>
      </div>
    </div>

    <!-- 积分概览（所有变体共用头部，但在 B 中会再次强化展示） -->
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
      <div class="summary-hint">做任务 → 领取 → 前端累加，无后端校验</div>
    </div>

    <VariantA v-if="variant === 'a'" :tasks="tasks" @claim="handleClaim" />
    <VariantB v-else-if="variant === 'b'" :tasks="tasks" :points="points" @claim="handleClaim" />
    <VariantC v-else :tasks="tasks" @claim="handleClaim" />

    <div class="proto-state">
      <div class="proto-state-title">State</div>
      <div class="proto-state-grid">
        <div><span class="state-k">variant</span> {{ variant }}</div>
        <div><span class="state-k">balance</span> {{ points.balance }} · totalEarned {{ points.totalEarned }} · todayEarnable +{{ todayEarnable }}</div>
        <div><span class="state-k">tasks</span> {{ tasks.length }} 项 · claimed {{ claimedCount }} · claimable {{ claimableCount }} · todo {{ todoCount }}</div>
        <div><span class="state-k">storage</span> {{ tasksStorageKey }} / {{ pointsStorageKey }}</div>
        <div class="state-raw">
          <span class="state-k">raw (前 3 项)</span>
          <pre class="state-pre">{{ rawPreview }}</pre>
        </div>
      </div>
    </div>

    <PrototypeSwitcher :variants="switcherVariants" :current="variant" />
  </div>
</template>

<script setup lang="ts">
import { computed, ref, onMounted } from 'vue'
import { useRoute } from 'vue-router'
import { ElMessage } from 'element-plus'
import PrototypeSwitcher from '@/components/prototype/PrototypeSwitcher.vue'
import VariantA from './VariantA.vue'
import VariantB from './VariantB.vue'
import VariantC from './VariantC.vue'
import type { TaskItem } from './types'
import {
  loadTasks,
  saveTasks,
  loadPoints,
  savePoints,
  resetAll,
  POINTS_STORAGE_KEY,
  TASKS_STORAGE_KEY,
} from './mock'

const route = useRoute()
const variant = computed(() => {
  const v = String(route.query.variant || 'a').toLowerCase()
  return ['a', 'b', 'c'].includes(v) ? v : 'a'
})

const switcherVariants = [
  { key: 'a', label: '卡片分组列表' },
  { key: 'b', label: '仪表盘+紧凑清单' },
  { key: 'c', label: '成就墙时间线' },
]

const tasks = ref<TaskItem[]>(loadTasks())
const points = ref(loadPoints())
const tasksStorageKey = TASKS_STORAGE_KEY
const pointsStorageKey = POINTS_STORAGE_KEY

const todayEarnable = computed(() =>
  tasks.value.filter((t) => t.status !== 'claimed').reduce((sum, t) => sum + t.points, 0),
)
const claimedCount = computed(() => tasks.value.filter((t) => t.status === 'claimed').length)
const claimableCount = computed(() => tasks.value.filter((t) => t.status === 'claimable').length)
const todoCount = computed(() => tasks.value.filter((t) => t.status === 'todo').length)

function persist() {
  saveTasks(tasks.value)
  savePoints(points.value.balance, points.value.totalEarned)
}

function handleClaim(id: number) {
  const task = tasks.value.find((t) => t.id === id)
  if (!task || task.status !== 'claimable') return
  task.status = 'claimed'
  points.value.balance += task.points
  points.value.totalEarned += task.points
  persist()
  ElMessage.success(`已领取 +${task.points} 积分`)
}

function handleReset() {
  resetAll()
  tasks.value = loadTasks()
  points.value = loadPoints()
  ElMessage.success('已重置为示例数据')
}

function simulateProgress() {
  const todo = tasks.value.find((t) => t.status === 'todo')
  if (!todo) {
    ElMessage.info('暂无待完成任务可模拟')
    return
  }
  if (todo.total && todo.total > 1 && todo.progress !== undefined) {
    todo.progress = Math.min(todo.total, (todo.progress || 0) + 1)
    if (todo.progress >= todo.total) {
      todo.status = 'claimable'
      ElMessage.success(`「${todo.title}」已达成，可领取`)
    } else {
      ElMessage.info(`「${todo.title}」进度 ${todo.progress}/${todo.total}`)
    }
  } else {
    todo.status = 'claimable'
    ElMessage.success(`「${todo.title}」已达成，可领取`)
  }
  persist()
}

const rawPreview = computed(() => JSON.stringify(tasks.value.slice(0, 3), null, 2))

onMounted(() => {
  tasks.value = loadTasks()
  points.value = loadPoints()
})
</script>

<style scoped>
.task-center-prototype {
  max-width: 1100px;
  margin: 0 auto;
  padding-bottom: 72px;
}
.proto-banner {
  display: flex;
  align-items: center;
  gap: 10px;
  margin-bottom: 16px;
  flex-wrap: wrap;
}
.proto-banner-text {
  font-size: 12px;
  color: var(--color-text-tertiary);
}
.proto-header {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  gap: 16px;
  margin-bottom: 14px;
  flex-wrap: wrap;
}
.proto-title {
  font-size: var(--text-2xl);
  line-height: 1.2;
  margin: 0 0 6px;
}
.proto-sub {
  font-size: 13px;
  color: var(--color-text-tertiary);
  margin: 0;
}
.proto-sub-sep {
  margin: 0 6px;
  color: var(--color-text-muted);
}
.proto-code {
  font-family: var(--font-mono);
  font-size: 12px;
  background: var(--color-bg-page);
  padding: 1px 6px;
  border-radius: var(--radius-sm);
  border: 1px solid var(--color-border-light);
}
.proto-actions {
  display: flex;
  align-items: center;
  gap: 8px;
  flex-wrap: wrap;
}
.points-summary {
  display: flex;
  align-items: center;
  gap: 16px;
  padding: 14px 16px;
  background: var(--color-bg-card);
  border: 1px solid var(--color-border-light);
  border-radius: var(--radius-lg);
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
  color: var(--color-text-muted);
  letter-spacing: 0.04em;
}
.summary-value {
  font-size: 20px;
  font-weight: 800;
  color: var(--color-text-primary);
  font-family: var(--font-display);
}
.summary-value.small {
  font-size: 16px;
}
.summary-divider {
  width: 1px;
  height: 32px;
  background: var(--color-border-light);
}
.summary-hint {
  margin-left: auto;
  font-size: 12px;
  color: var(--color-text-tertiary);
}
.proto-state {
  margin-top: 18px;
  padding: 12px 14px;
  background: var(--color-bg-card);
  border: 1px solid var(--color-border-light);
  border-radius: var(--radius-lg);
}
.proto-state-title {
  font-size: 12px;
  font-weight: 700;
  letter-spacing: 0.06em;
  color: var(--color-text-muted);
  margin-bottom: 8px;
  text-transform: uppercase;
}
.proto-state-grid {
  display: flex;
  flex-direction: column;
  gap: 4px;
  font-size: 12px;
  color: var(--color-text-secondary);
  font-family: var(--font-mono);
}
.state-k {
  color: var(--color-text-muted);
  margin-right: 6px;
}
.state-raw {
  margin-top: 8px;
}
.state-pre {
  margin: 6px 0 0;
  padding: 10px 12px;
  background: var(--color-bg-page);
  border: 1px solid var(--color-border-light);
  border-radius: var(--radius-md);
  font-size: 11px;
  line-height: 1.5;
  max-height: 220px;
  overflow: auto;
  white-space: pre-wrap;
  word-break: break-all;
}
</style>
