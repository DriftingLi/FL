<!-- PROTOTYPE — throwaway variant A: 紧凑卡片网格 -->
<template>
  <div class="variant-a">
    <div v-if="papers.length === 0" class="empty-wrap">
      <el-empty :description="currentCredentialName + ' 真题建设中，敬请期待'" />
    </div>
    <el-row v-else :gutter="16" class="variant-a-grid">
      <el-col v-for="p in papers" :key="p.id" :xs="24" :sm="12" :md="8">
        <el-card
          shadow="hover"
          class="paper-card"
          :class="{ 'is-selected': selectedId === p.id }"
          :style="{ borderTopColor: difficultyBorder[p.difficulty] }"
          @click="$emit('select', p.id)"
        >
          <div class="paper-year">{{ p.year }}年</div>
          <div class="paper-title">{{ p.title }}</div>
          <div class="paper-meta">
            <el-tag size="small" :type="levelTagType(p.difficulty)">{{ p.difficulty }}</el-tag>
            <el-tag size="small" type="info" effect="plain">{{ p.credential_name }}</el-tag>
            <span class="paper-source">{{ p.source }}</span>
          </div>
          <div class="paper-stats">
            <span>{{ p.question_count }}题</span>
            <span class="dot">·</span>
            <span>{{ p.duration_minutes }}分钟</span>
          </div>
        </el-card>
      </el-col>
    </el-row>
  </div>
</template>

<script setup lang="ts">
import { levelTagType } from '@/constants/level'
import type { Paper, Difficulty } from './types'

defineProps<{
  papers: Paper[]
  currentCredentialName: string
  selectedId: number | null
}>()

defineEmits<{ select: [id: number] }>()

const difficultyBorder: Record<Difficulty, string> = {
  入门: 'var(--color-success)',
  进阶: 'var(--color-primary-500)',
  专项: 'var(--color-warning)',
  认证: 'var(--color-danger)'
}
</script>

<style scoped>
.empty-wrap {
  padding: 24px 0;
  background: var(--color-bg-card);
  border: 1px solid var(--color-border-light);
  border-radius: var(--radius-lg);
}
.variant-a-grid {
  margin-bottom: 12px;
}
.paper-card {
  cursor: pointer;
  border-top: 3px solid var(--color-border);
  transition: transform 150ms var(--ease-default), box-shadow 150ms var(--ease-default);
  margin-bottom: 16px;
}
.paper-card:hover {
  transform: translateY(-2px);
}
.paper-card.is-selected {
  box-shadow: 0 0 0 2px var(--color-primary-200);
}
.paper-card :deep(.el-card__body) {
  padding: 16px;
}
.paper-year {
  font-size: 12px;
  font-weight: 600;
  color: var(--color-text-muted);
  letter-spacing: 0.04em;
  margin-bottom: 6px;
}
.paper-title {
  font-size: 15px;
  font-weight: 600;
  color: var(--color-text-primary);
  line-height: 1.45;
  margin-bottom: 10px;
  display: -webkit-box;
  -webkit-line-clamp: 2;
  -webkit-box-orient: vertical;
  overflow: hidden;
}
.paper-meta {
  display: flex;
  align-items: center;
  gap: 6px;
  flex-wrap: wrap;
  margin-bottom: 10px;
}
.paper-source {
  font-size: 12px;
  color: var(--color-text-tertiary);
}
.paper-stats {
  font-size: 13px;
  color: var(--color-text-secondary);
  display: flex;
  align-items: center;
  gap: 6px;
}
.dot {
  color: var(--color-text-muted);
}
</style>
