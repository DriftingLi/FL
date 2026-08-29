<template>
  <div class="ai-card">
    <div class="card-header">
      <span class="card-title">AI解析</span>
    </div>
    <div v-if="loading" class="loading">AI 解析生成中...</div>
    <div v-else class="content">{{ displayText }}</div>
  </div>
</template>

<script setup lang="ts">
import { computed } from 'vue'

const props = defineProps<{ aiExplanation?: string; fallbackExplanation?: string; loading?: boolean }>()

const displayText = computed(() => {
  if (props.aiExplanation) return props.aiExplanation
  if (props.fallbackExplanation) return props.fallbackExplanation
  return '暂无解析'
})
</script>

<style scoped>
.ai-card { border:1px solid #ebeef5; border-radius:8px; padding:16px; background:#fff; margin-bottom:12px; }
.card-header { display:flex; justify-content:space-between; align-items:center; margin-bottom:8px; }
.card-title { font-weight:600; color:#303133; }
.loading { color:#909399; font-size:13px; }
.content { white-space:pre-wrap; color:#606266; font-size:14px; line-height:1.6; }
</style>
