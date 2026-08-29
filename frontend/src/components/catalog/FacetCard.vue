<script setup lang="ts">
const props = withDefaults(
  defineProps<{
    title: string
    /**
     * 卡片外观。
     * - `bordered`：**默认值 = 改造前行为**，带描边卡片
     * - `plain`：无描边无底色，用于已处在容器内的筛选组
     */
    variant?: 'bordered' | 'plain'
  }>(),
  { variant: 'bordered' }
)
</script>

<template>
  <div class="cc-filter-card" :class="{ 'is-plain': props.variant === 'plain' }">
    <div class="cc-card-title">{{ title }}</div>
    <slot />
  </div>
</template>

<style scoped>
.cc-filter-card {
  background: var(--color-bg-card);
  border: 1px solid var(--color-border-light);
  border-radius: var(--radius-lg);
  padding: var(--space-3) var(--space-2);
}

/* plain：去掉描边与底色，只留分组间距 */
.cc-filter-card.is-plain {
  background: transparent;
  border-color: transparent;
  padding: 0;
}

.cc-card-title {
  font-size: var(--text-xs);
  color: var(--color-text-tertiary);
  padding: var(--space-1) var(--space-3);
  margin-bottom: var(--space-1);
}
</style>
