<script setup lang="ts">
defineProps<{
  title: string
}>()
</script>

<template>
  <div class="cc-filter-card">
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

.cc-card-title {
  font-size: var(--text-xs);
  color: var(--color-text-tertiary);
  padding: var(--space-1) var(--space-3);
  margin-bottom: var(--space-1);
}
</style>
