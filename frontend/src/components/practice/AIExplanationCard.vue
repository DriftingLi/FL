<template>
  <UiCard class="ai-card" padding="base">
    <template #header><span class="card-title">AI解析</span></template>
    <div v-if="loading" class="loading">AI 解析生成中...</div>
    <div v-else class="content">{{ displayText }}</div>
  </UiCard>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import UiCard from '@/components/ui/UiCard.vue'

const props = defineProps<{ aiExplanation?: string; fallbackExplanation?: string; loading?: boolean }>()

const displayText = computed(() => {
  if (props.aiExplanation) return props.aiExplanation
  if (props.fallbackExplanation) return props.fallbackExplanation
  return '暂无解析'
})
</script>

<style scoped>
/* 容器样式（描边/圆角/内距/底色）已由 UiCard 承担，此处只留外边距 */
.ai-card { margin-bottom:12px; }
.card-title { font-weight: var(--font-semibold); color: var(--color-text-primary); }
.loading { color: var(--color-text-tertiary); font-size:13px; }
.content { white-space:pre-wrap; color: var(--color-text-secondary); font-size:14px; line-height:1.6; }
</style>
