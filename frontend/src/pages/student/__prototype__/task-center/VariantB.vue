<!-- PROTOTYPE — throwaway variant B: 顶部积分仪表盘 + 紧凑清单 -->
<template>
  <div class="variant-b">
    <div class="points-dashboard">
      <div class="dashboard-main">
        <div class="points-circle">
          <span class="points-num">{{ points.balance }}</span>
          <span class="points-unit">积分</span>
        </div>
        <div class="points-stats">
          <div class="stat-item">
            <span class="stat-label">今日可得</span>
            <span class="stat-value">+{{ todayEarnable }}</span>
          </div>
          <div class="stat-divider"></div>
          <div class="stat-item">
            <span class="stat-label">累计获得</span>
            <span class="stat-value">{{ points.totalEarned }}</span>
          </div>
        </div>
      </div>
      <div class="dashboard-actions">
        <el-button size="small" plain disabled>积分明细（占位）</el-button>
        <el-button size="small" plain disabled>兑换商城（占位）</el-button>
      </div>
    </div>

    <div class="compact-groups">
      <div v-for="group in grouped" :key="group.key" class="compact-group">
        <div class="compact-head">
          <span class="compact-title">{{ group.label }}</span>
          <span class="compact-desc">{{ group.desc }}</span>
        </div>
        <div class="compact-list">
          <div v-for="task in group.tasks" :key="task.id" class="compact-row" :class="task.status">
            <span class="row-title">{{ task.title }}</span>
            <span class="row-progress" v-if="task.total && task.total > 1">{{ task.progress }}/{{ task.total }}</span>
            <span class="row-points">+{{ task.points }}</span>
            <el-button
              v-if="task.status === 'claimable'"
              size="small"
              type="primary"
              @click="$emit('claim', task.id)"
            >
              领取
            </el-button>
            <el-tag v-else-if="task.status === 'claimed'" size="small" type="success" effect="plain">已领</el-tag>
            <el-button v-else size="small" plain disabled>未完成</el-button>
          </div>
        </div>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import type { TaskItem, TaskGroup } from './types'
import { groupLabelMap, groupDescMap } from './types'

const props = defineProps<{
  tasks: TaskItem[]
  points: { balance: number; totalEarned: number }
}>()
defineEmits<{ claim: [id: number] }>()

const todayEarnable = computed(() => {
  return props.tasks
    .filter((t) => t.status !== 'claimed')
    .reduce((sum, t) => sum + t.points, 0)
})

const grouped = computed(() => {
  const order: TaskGroup[] = ['daily', 'newbie', 'growth']
  return order.map((key) => ({
    key,
    label: groupLabelMap[key],
    desc: groupDescMap[key],
    tasks: props.tasks.filter((t) => t.group === key),
  }))
})
</script>

<style scoped>
.points-dashboard {
  background: linear-gradient(135deg, #0f172a 0%, #1e293b 55%, #334155 100%);
  color: #f8fafc;
  border-radius: var(--radius-lg);
  padding: 18px 20px;
  display: flex;
  justify-content: space-between;
  align-items: center;
  gap: 16px;
  margin-bottom: 16px;
  flex-wrap: wrap;
}
.dashboard-main {
  display: flex;
  align-items: center;
  gap: 20px;
}
.points-circle {
  width: 72px;
  height: 72px;
  border-radius: 9999px;
  background: rgba(255, 255, 255, 0.12);
  border: 2px solid rgba(255, 255, 255, 0.25);
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  flex-shrink: 0;
}
.points-num {
  font-size: 22px;
  font-weight: 800;
  line-height: 1;
  font-family: var(--font-display);
}
.points-unit {
  font-size: 11px;
  opacity: 0.8;
  margin-top: 2px;
}
.points-stats {
  display: flex;
  align-items: center;
  gap: 16px;
}
.stat-item {
  display: flex;
  flex-direction: column;
  gap: 2px;
}
.stat-label {
  font-size: 11px;
  opacity: 0.7;
}
.stat-value {
  font-size: 15px;
  font-weight: 700;
}
.stat-divider {
  width: 1px;
  height: 28px;
  background: rgba(255, 255, 255, 0.2);
}
.dashboard-actions {
  display: flex;
  gap: 8px;
  flex-wrap: wrap;
}
.dashboard-actions :deep(.el-button) {
  --el-button-bg-color: rgba(255,255,255,0.12);
  --el-button-border-color: rgba(255,255,255,0.2);
  --el-button-text-color: #f8fafc;
}
.compact-groups {
  display: flex;
  flex-direction: column;
  gap: 16px;
}
.compact-head {
  display: flex;
  align-items: center;
  gap: 8px;
  margin-bottom: 8px;
}
.compact-title {
  font-size: 14px;
  font-weight: 700;
  color: var(--color-text-primary);
}
.compact-desc {
  font-size: 12px;
  color: var(--color-text-tertiary);
}
.compact-list {
  display: flex;
  flex-direction: column;
  gap: 8px;
}
.compact-row {
  display: flex;
  align-items: center;
  gap: 10px;
  padding: 10px 14px;
  background: var(--color-bg-card);
  border: 1px solid var(--color-border-light);
  border-radius: var(--radius-lg);
}
.compact-row.claimed {
  opacity: 0.6;
  background: #fafafa;
}
.row-title {
  flex: 1;
  font-size: 13px;
  font-weight: 600;
  color: var(--color-text-primary);
  min-width: 0;
}
.row-progress {
  font-size: 11px;
  color: var(--color-text-muted);
  font-family: var(--font-mono);
  background: var(--color-bg-page);
  padding: 2px 6px;
  border-radius: var(--radius-full);
  border: 1px solid var(--color-border-light);
}
.row-points {
  font-size: 13px;
  font-weight: 700;
  color: #d97706;
  min-width: 36px;
  text-align: right;
}
</style>
