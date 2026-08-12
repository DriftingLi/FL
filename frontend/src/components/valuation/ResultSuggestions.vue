<template>
  <ul v-if="items.length" class="suggestion-list" :class="variant">
    <li v-for="(s, idx) in items" :key="idx">
      <span class="suggestion-num">{{ String(idx + 1).padStart(2, '0') }}</span>
      <span class="suggestion-text">{{ s }}</span>
    </li>
  </ul>
  <el-empty v-else description="暂无建议" />
</template>

<script setup lang="ts">
// 评估建议列表（评估结果 / 评估报告 / 电池 RUL 三页共用；battery 变体为电池页视觉）。
withDefaults(
  defineProps<{
    items: string[]
    variant?: 'default' | 'battery'
  }>(),
  { variant: 'default' }
)
</script>

<style scoped>
.suggestion-list {
  margin: 0;
  padding: 0;
  list-style: none;
  display: grid;
  grid-template-columns: 1fr 1fr;
  gap: var(--sp-3) var(--sp-6);
}
.suggestion-list li {
  display: flex;
  align-items: flex-start;
  gap: 12px;
  font-size: var(--fs-sm);
  line-height: 1.75;
  color: var(--color-text-secondary);
}
.suggestion-list.battery li {
  font-size: var(--fs-base);
  line-height: 1.6;
  color: var(--color-text);
}
.suggestion-num {
  font-family: var(--font-mono);
  font-size: var(--fs-xs);
  font-weight: var(--fw-medium);
  color: var(--color-accent);
  background: rgba(62, 106, 225, 0.08);
  padding: 2px 8px;
  border-radius: var(--radius-sm);
  flex-shrink: 0;
  margin-top: 2px;
}
.suggestion-list.battery .suggestion-num {
  font-size: var(--fs-sm);
  color: var(--color-primary);
}
.suggestion-text {
  flex: 1;
}
@media (max-width: 768px) {
  .suggestion-list {
    grid-template-columns: 1fr;
  }
}
</style>
