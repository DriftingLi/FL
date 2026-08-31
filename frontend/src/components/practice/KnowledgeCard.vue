<template>
  <UiCard class="knowledge-card" padding="base">
    <template #header><span class="card-title">考点</span></template>
    <div class="content">
      <template v-if="tags.length>0">
        <el-tag v-for="t in tags" :key="t.id" size="small" style="margin:4px 6px 0 0">{{ t.name }}</el-tag>
      </template>
      <span v-else class="empty">暂无</span>
    </div>
  </UiCard>
</template>

<script setup lang="ts">
import UiCard from '@/components/ui/UiCard.vue'

defineProps<{ tags: any[] }>()
</script>

<style scoped>
/* 卡片容器样式（描边/圆角/内距/底色）已由 UiCard 承担，此处只留卡片自身的外边距 */
.knowledge-card { margin-bottom:12px; }
.card-title { font-weight: var(--font-semibold); color: var(--color-text-primary); }
.content { min-height:24px; }
.empty { color: var(--color-text-tertiary); font-size:13px; }
</style>
