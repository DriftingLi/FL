<script setup lang="ts">
withDefaults(
  defineProps<{
    active?: boolean
    name: string
    count: number
    warn?: boolean
  }>(),
  { active: false, warn: false }
)

defineEmits<{ select: [] }>()
</script>

<template>
  <button
    type="button"
    class="cc-nav-item"
    :class="{ active, 'cc-nav-warn': warn }"
    @click="$emit('select')"
  >
    <span class="cc-nav-name">{{ name }}</span>
    <span class="cc-nav-count">{{ count }}</span>
  </button>
</template>

<style scoped>
.cc-nav-item {
  display: flex;
  align-items: center;
  justify-content: space-between;
  width: 100%;
  padding: var(--space-2) var(--space-3);
  margin-bottom: 2px;
  border: none;
  border-radius: var(--radius-md);
  background: transparent;
  cursor: pointer;
  font-family: var(--font-body);
  font-size: var(--text-sm);
  color: var(--color-text-primary);
  transition: background var(--duration-fast) var(--ease-default);
}

.cc-nav-item:hover {
  /* #1619：原用 --color-bg-sidebar-hover（侧栏深色体系的 token）——它在本组件所在的**内容区
     浅表面上**本就是错位配色（深底配深字），随恒深侧栏退役一并改判为「比所在表面深一档」。
     bg-page 浅色下 #F8FAFC（卡片白之下一档）、深色下 #0F172A（卡片 #1E293B 之下一档）。 */
  background: var(--color-bg-page);
}

.cc-nav-item.active {
  background: var(--color-primary-50);
  color: var(--color-primary-600);
  font-weight: var(--font-semibold);
}

.cc-nav-name {
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.cc-nav-count {
  font-size: var(--text-xs);
  color: var(--color-text-tertiary);
}

.cc-nav-warn.active {
  background: var(--color-danger-light);
  color: var(--color-danger);
}
</style>
