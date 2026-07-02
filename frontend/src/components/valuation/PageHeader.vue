// 统一页头（设计稿风格：eyebrow + 主标题 + 右侧操作，底部 1px 分隔线）
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
  padding: var(--sp-8) 0 var(--sp-5);
  margin-bottom: var(--sp-6);
  border-bottom: 1px solid var(--color-border);
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
  letter-spacing: 0.18em;
  text-transform: uppercase;
  color: var(--color-text-tertiary);
  margin: 0 0 var(--sp-1);
}
.page-header-title {
  font-family: var(--font-text);
  font-size: var(--fs-3xl);
  font-weight: var(--fw-semibold);
  margin: 0;
  color: var(--color-text);
  line-height: 1.2;
}
.page-header-right {
  display: flex;
  gap: var(--sp-2);
  flex-shrink: 0;
}

/* ===== 移动端适配 ===== */
@media (max-width: 768px) {
  .page-header {
    flex-direction: column;
    align-items: flex-start;
    gap: var(--sp-4);
    padding: var(--sp-5) 0 var(--sp-4);
    margin-bottom: var(--sp-4);
  }
  .page-header-title {
    font-size: var(--fs-2xl);
  }
  .page-header-right {
    width: 100%;
    flex-wrap: wrap;
  }
}
</style>
