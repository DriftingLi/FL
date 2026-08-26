<!-- PROTOTYPE — throwaway variant C: 年份分组时间线 -->
<template>
  <div class="variant-c">
    <div v-if="papers.length === 0" class="empty-wrap">
      <el-empty :description="currentCredentialName + ' 真题建设中，敬请期待'" />
    </div>
    <div v-else class="variant-c-timeline">
      <div v-for="[year, list] in grouped" :key="year" class="timeline-year">
        <div class="timeline-year-head">
          <span class="timeline-dot"></span>
          <span class="timeline-year-label">{{ year }}年</span>
          <span class="timeline-year-count">{{ list.length }}套</span>
        </div>
        <div class="timeline-cards">
          <div
            v-for="p in list"
            :key="p.id"
            class="timeline-card"
            :class="{ 'is-selected': selectedId === p.id }"
            @click="$emit('select', p.id)"
          >
            <div class="timeline-card-title">{{ p.title }}</div>
            <div class="timeline-card-meta">
              <el-tag size="small" :type="levelTagType(p.difficulty)">{{ p.difficulty }}</el-tag>
              <span class="meta-text">{{ p.source }} · {{ p.question_count }}题 · {{ p.duration_minutes }}分钟</span>
            </div>
          </div>
        </div>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import { levelTagType } from '@/constants/level'
import type { Paper } from './types'

const props = defineProps<{
  papers: Paper[]
  currentCredentialName: string
  selectedId: number | null
}>()

defineEmits<{ select: [id: number] }>()

const grouped = computed(() => {
  const map = new Map<number, Paper[]>()
  for (const p of props.papers) {
    if (!map.has(p.year)) map.set(p.year, [])
    map.get(p.year)!.push(p)
  }
  return [...map.entries()].sort((a, b) => b[0] - a[0])
})
</script>

<style scoped>
.empty-wrap {
  padding: 24px 0;
  background: var(--color-bg-card);
  border: 1px solid var(--color-border-light);
  border-radius: var(--radius-lg);
}
.variant-c-timeline {
  display: flex;
  flex-direction: column;
  gap: 20px;
}
.timeline-year-head {
  display: flex;
  align-items: center;
  gap: 10px;
  margin-bottom: 10px;
}
.timeline-dot {
  width: 10px;
  height: 10px;
  border-radius: 9999px;
  background: var(--color-primary-500);
  box-shadow: 0 0 0 4px var(--color-primary-100);
  flex-shrink: 0;
}
.timeline-year-label {
  font-size: 16px;
  font-weight: 700;
  color: var(--color-text-primary);
}
.timeline-year-count {
  font-size: 12px;
  color: var(--color-text-muted);
  background: var(--color-bg-page);
  border: 1px solid var(--color-border-light);
  padding: 2px 8px;
  border-radius: var(--radius-full);
}
.timeline-cards {
  margin-left: 20px;
  padding-left: 16px;
  border-left: 1px dashed var(--color-border);
  display: flex;
  flex-direction: column;
  gap: 10px;
}
.timeline-card {
  padding: 12px 14px;
  background: var(--color-bg-card);
  border: 1px solid var(--color-border-light);
  border-radius: var(--radius-lg);
  cursor: pointer;
  transition: border-color 150ms var(--ease-default), box-shadow 150ms var(--ease-default);
}
.timeline-card:hover {
  border-color: var(--color-border);
}
.timeline-card.is-selected {
  border-color: var(--color-primary-200);
  box-shadow: 0 0 0 2px var(--color-primary-100);
}
.timeline-card-title {
  font-size: 14px;
  font-weight: 600;
  color: var(--color-text-primary);
  margin-bottom: 8px;
}
.timeline-card-meta {
  display: flex;
  align-items: center;
  gap: 8px;
  flex-wrap: wrap;
}
.meta-text {
  font-size: 12px;
  color: var(--color-text-tertiary);
}
</style>
