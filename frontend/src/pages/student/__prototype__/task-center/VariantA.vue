<!-- PROTOTYPE — throwaway variant A: 卡片分组列表 -->
<template>
  <div class="variant-a">
    <div class="group-stack">
      <div v-for="group in grouped" :key="group.key" class="group-section">
        <div class="group-head">
          <span class="group-title">{{ group.label }}</span>
          <span class="group-desc">{{ group.desc }}</span>
          <span class="group-count">{{ group.tasks.length }}项</span>
        </div>
        <div class="task-list">
          <div v-for="task in group.tasks" :key="task.id" class="task-card" :class="task.status">
            <div class="task-left">
              <div class="task-icon" :class="task.status">
                <el-icon v-if="task.status === 'claimed'"><CircleCheckFilled /></el-icon>
                <el-icon v-else-if="task.status === 'claimable'"><Trophy /></el-icon>
                <el-icon v-else><List /></el-icon>
              </div>
              <div class="task-info">
                <div class="task-title">{{ task.title }}</div>
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
                @click="$emit('claim', task.id)"
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
import { computed } from 'vue'
import { CircleCheckFilled, Trophy, List } from '@element-plus/icons-vue'
import type { TaskItem, TaskGroup } from './types'
import { groupLabelMap, groupDescMap } from './types'

const props = defineProps<{ tasks: TaskItem[] }>()
defineEmits<{ claim: [id: number] }>()

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
  color: var(--color-text-muted);
  background: var(--color-bg-page);
  border: 1px solid var(--color-border-light);
  padding: 2px 8px;
  border-radius: var(--radius-full);
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
  background: var(--color-bg-card);
  border: 1px solid var(--color-border-light);
  border-radius: var(--radius-lg);
  transition: border-color 150ms var(--ease-default);
}
.task-card:hover {
  border-color: var(--color-border);
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
  color: var(--color-text-muted);
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
.task-title {
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
  color: var(--color-text-muted);
  font-family: var(--font-mono);
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
