<!-- PROTOTYPE — throwaway variant C: 成就墙 / 时间线 -->
<template>
  <div class="variant-c">
    <div class="achievement-head">
      <span class="head-title">成就进度</span>
      <span class="head-sub">完成越多，积分越高 · 占位期仅前端</span>
      <span class="head-count">{{ claimedCount }}/{{ tasks.length }} 已领取</span>
    </div>

    <div class="timeline">
      <div v-for="task in tasks" :key="task.id" class="timeline-row" :class="task.status">
        <div class="timeline-node" :class="task.status">
          <el-icon v-if="task.status === 'claimed'"><CircleCheckFilled /></el-icon>
          <el-icon v-else-if="task.status === 'claimable'"><Trophy /></el-icon>
          <el-icon v-else><Clock /></el-icon>
        </div>
        <div class="timeline-card">
          <div class="card-top">
            <el-tag size="small" :type="groupTagType(task.group)" effect="plain">{{ groupLabelMap[task.group] }}</el-tag>
            <span class="card-points">+{{ task.points }} 积分</span>
          </div>
          <div class="card-title">{{ task.title }}</div>
          <div class="card-desc">{{ task.desc }}</div>
          <div v-if="task.total && task.total > 1" class="card-progress">
            <el-progress :percentage="Math.round(((task.progress || 0) / task.total) * 100)" :stroke-width="6" :show-text="false" style="width: 160px" />
            <span class="progress-text">{{ task.progress }}/{{ task.total }}</span>
          </div>
          <div class="card-action">
            <el-button v-if="task.status === 'claimable'" size="small" type="primary" @click="$emit('claim', task.id)">领取积分</el-button>
            <el-tag v-else-if="task.status === 'claimed'" size="small" type="success" effect="plain">已领取</el-tag>
            <el-button v-else size="small" plain disabled>待完成</el-button>
          </div>
        </div>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import { CircleCheckFilled, Trophy, Clock } from '@element-plus/icons-vue'
import type { TaskItem, TaskGroup } from './types'
import { groupLabelMap } from './types'

const props = defineProps<{ tasks: TaskItem[] }>()
defineEmits<{ claim: [id: number] }>()

const claimedCount = computed(() => props.tasks.filter((t) => t.status === 'claimed').length)

function groupTagType(group: TaskGroup) {
  if (group === 'daily') return 'info'
  if (group === 'newbie') return 'warning'
  return 'success'
}
</script>

<style scoped>
.achievement-head {
  display: flex;
  align-items: center;
  gap: 10px;
  margin-bottom: 16px;
  flex-wrap: wrap;
}
.head-title {
  font-size: 15px;
  font-weight: 700;
  color: var(--color-text-primary);
}
.head-sub {
  font-size: 12px;
  color: var(--color-text-tertiary);
}
.head-count {
  margin-left: auto;
  font-size: 12px;
  color: var(--color-text-muted);
  background: var(--color-bg-page);
  border: 1px solid var(--color-border-light);
  padding: 4px 10px;
  border-radius: var(--radius-full);
  font-family: var(--font-mono);
}
.timeline {
  position: relative;
  display: flex;
  flex-direction: column;
  gap: 14px;
  padding-left: 18px;
}
.timeline::before {
  content: '';
  position: absolute;
  left: 7px;
  top: 8px;
  bottom: 8px;
  width: 1px;
  background: var(--color-border);
}
.timeline-row {
  display: flex;
  gap: 14px;
  align-items: flex-start;
}
.timeline-node {
  width: 16px;
  height: 16px;
  border-radius: 9999px;
  display: flex;
  align-items: center;
  justify-content: center;
  font-size: 10px;
  flex-shrink: 0;
  margin-top: 10px;
  border: 2px solid var(--color-bg-card);
  box-shadow: 0 0 0 2px var(--color-border);
}
.timeline-node.todo {
  background: var(--color-bg-page);
  color: var(--color-text-muted);
}
.timeline-node.claimable {
  background: var(--color-warning-light);
  color: var(--color-warning-strong);
  box-shadow: 0 0 0 2px var(--color-warning-light);
}
.timeline-node.claimed {
  background: var(--color-success-light);
  color: var(--color-success-strong);
  box-shadow: 0 0 0 2px var(--color-success-light);
}
.timeline-card {
  flex: 1;
  min-width: 0;
  background: var(--color-bg-card);
  border: 1px solid var(--color-border-light);
  border-radius: var(--radius-lg);
  padding: 14px 16px;
}
.timeline-row.claimed .timeline-card {
  opacity: 0.65;
  background: var(--color-bg-page);
}
.card-top {
  display: flex;
  justify-content: space-between;
  align-items: center;
  margin-bottom: 6px;
}
.card-points {
  font-size: 13px;
  font-weight: 700;
  color: var(--color-warning-strong);
}
.card-title {
  font-size: 14px;
  font-weight: 600;
  color: var(--color-text-primary);
  margin-bottom: 4px;
}
.card-desc {
  font-size: 12px;
  color: var(--color-text-tertiary);
  margin-bottom: 8px;
}
.card-progress {
  display: flex;
  align-items: center;
  gap: 8px;
  margin-bottom: 10px;
}
.progress-text {
  font-size: 11px;
  color: var(--color-text-muted);
  font-family: var(--font-mono);
}
.card-action {
  display: flex;
  justify-content: flex-end;
}
</style>
