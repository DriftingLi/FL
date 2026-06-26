// 统一页头（极简，无下边线）
<script setup lang="ts">
defineProps<{
  title: string
  subtitle?: string
  icon?: unknown
}>()
</script>

<template>
  <header class="page-header">
    <div class="page-header-left">
      <component v-if="icon" :is="icon" class="page-header-icon" />
      <div>
        <p v-if="subtitle" class="page-header-eyebrow">{{ subtitle }}</p>
        <h1 class="page-header-title">{{ title }}</h1>
      </div>
    </div>
    <div class="page-header-right">
      <slot name="actions" />
    </div>
  </header>
</template>

<style scoped>
.page-header {
  display: flex;
  align-items: flex-end;
  justify-content: space-between;
  gap: var(--sp-6);
  margin-bottom: var(--sp-8);
  padding-bottom: var(--sp-5);
}
.page-header-left {
  display: flex;
  gap: var(--sp-3);
  align-items: center;
}
.page-header-icon {
  font-size: 24px;
  color: var(--color-text);
  line-height: 1;
}
.page-header-eyebrow {
  font-size: var(--fs-sm);
  font-weight: var(--fw-medium);
  letter-spacing: 0.12em;
  text-transform: uppercase;
  color: var(--color-text-tertiary);
  margin: 0 0 6px;
}
.page-header-title {
  font-family: var(--font-text);
  font-size: var(--fs-2xl);    /* 28px */
  font-weight: var(--fw-medium);
  margin: 0;
  color: var(--color-text);
  line-height: 1.2;
}
.page-header-right {
  display: flex;
  gap: var(--sp-2);
}

/* ===== 移动端适配 ===== */
@media (max-width: 768px) {
  .page-header {
    flex-direction: column;
    align-items: flex-start;
    gap: var(--sp-4);
    margin-bottom: var(--sp-6);
    padding-bottom: var(--sp-4);
  }
  .page-header-title {
    font-size: 22px;
  }
  .page-header-right {
    width: 100%;
    flex-wrap: wrap;
  }
}
</style>
