<!-- PROTOTYPE — throwaway variant B: 列表 + 筛选头 -->
<template>
  <div class="variant-b">
    <div class="variant-b-filters">
      <el-select :model-value="yearFilter" placeholder="年份" clearable style="width: 130px" @update:model-value="$emit('update:yearFilter', $event)">
        <el-option v-for="y in yearOptions" :key="y" :label="y + '年'" :value="y" />
      </el-select>
      <el-select :model-value="difficultyFilter" placeholder="难度" clearable style="width: 130px" @update:model-value="$emit('update:difficultyFilter', $event)">
        <el-option label="入门" value="入门" />
        <el-option label="进阶" value="进阶" />
        <el-option label="专项" value="专项" />
        <el-option label="认证" value="认证" />
      </el-select>
      <el-input :model-value="keyword" placeholder="搜索标题/来源" clearable style="width: 220px" @update:model-value="$emit('update:keyword', $event)" />
      <el-button @click="$emit('clear')">重置</el-button>
    </div>
    <div v-if="papers.length === 0" class="empty-wrap">
      <el-empty :description="currentCredentialName + ' 真题建设中，敬请期待'" />
    </div>
    <div v-else class="variant-b-list">
      <div
        v-for="p in papers"
        :key="p.id"
        class="variant-b-row"
        :class="{ 'is-selected': selectedId === p.id }"
        @click="$emit('select', p.id)"
      >
        <div class="row-main">
          <div class="row-title">
            <span class="row-year">{{ p.year }}</span>
            <span class="row-title-text">{{ p.title }}</span>
            <el-tag size="small" :type="levelTagType(p.difficulty)" class="row-diff">{{ p.difficulty }}</el-tag>
          </div>
          <div class="row-sub">
            <span>{{ p.source }}</span>
            <span class="dot">·</span>
            <span>{{ p.credential_name }}</span>
            <span class="dot">·</span>
            <span>{{ p.question_count }}题 · {{ p.duration_minutes }}分钟</span>
          </div>
        </div>
        <el-button size="small" type="primary" plain>去练习</el-button>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { levelTagType } from '@/constants/level'
import type { Paper, Difficulty } from './types'

defineProps<{
  papers: Paper[]
  currentCredentialName: string
  selectedId: number | null
  yearFilter: number | null
  difficultyFilter: Difficulty | null
  keyword: string
  yearOptions: number[]
}>()

defineEmits<{
  'update:yearFilter': [v: number | null]
  'update:difficultyFilter': [v: Difficulty | null]
  'update:keyword': [v: string]
  select: [id: number]
  clear: []
}>()
</script>

<style scoped>
.empty-wrap {
  padding: 24px 0;
  background: var(--color-bg-card);
  border: 1px solid var(--color-border-light);
  border-radius: var(--radius-lg);
}
.variant-b-filters {
  display: flex;
  align-items: center;
  gap: 10px;
  flex-wrap: wrap;
  padding: 12px;
  background: var(--color-bg-card);
  border: 1px solid var(--color-border-light);
  border-radius: var(--radius-lg);
  margin-bottom: 14px;
}
.variant-b-list {
  display: flex;
  flex-direction: column;
  gap: 10px;
}
.variant-b-row {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
  padding: 14px 16px;
  background: var(--color-bg-card);
  border: 1px solid var(--color-border-light);
  border-radius: var(--radius-lg);
  cursor: pointer;
  transition: border-color 150ms var(--ease-default), box-shadow 150ms var(--ease-default);
}
.variant-b-row:hover {
  border-color: var(--color-border);
}
.variant-b-row.is-selected {
  border-color: var(--color-primary-200);
  box-shadow: 0 0 0 2px var(--color-primary-100);
}
.row-main {
  min-width: 0;
  flex: 1;
}
.row-title {
  display: flex;
  align-items: center;
  gap: 8px;
  flex-wrap: wrap;
  margin-bottom: 6px;
}
.row-year {
  font-size: 12px;
  font-weight: 700;
  color: var(--color-primary-600);
  background: var(--color-primary-50);
  padding: 2px 6px;
  border-radius: var(--radius-full);
}
.row-title-text {
  font-size: 14px;
  font-weight: 600;
  color: var(--color-text-primary);
}
.row-sub {
  font-size: 12px;
  color: var(--color-text-tertiary);
  display: flex;
  align-items: center;
  gap: 6px;
  flex-wrap: wrap;
}
.row-diff {
  margin-left: 2px;
}
.dot {
  color: var(--color-text-muted);
}
</style>
