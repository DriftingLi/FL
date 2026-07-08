<script setup lang="ts">
// 统一页头：eyebrow + 主标题 + 右侧操作 + 底部渐变下划线（与官网风格一致）
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
  gap: var(--sp-6, 24px);
  padding: var(--sp-10, 40px) 0 var(--sp-5, 20px);
  margin-bottom: var(--sp-6, 24px);
  position: relative;
}
.page-header::after {
  content: '';
  position: absolute;
  left: 0;
  right: 0;
  bottom: 0;
  height: 1px;
  background: linear-gradient(
    90deg,
    var(--color-brand-200, #BAE6FD) 0%,
    var(--color-border, #E2E8F0) 30%,
    var(--color-border, #E2E8F0) 100%
  );
}
.page-header-left {
  display: flex;
  gap: var(--sp-3, 12px);
  align-items: center;
}
.page-header-icon {
  font-size: 24px;
  color: var(--color-text, #0F172A);
  line-height: 1;
}
.page-header-eyebrow {
  font-family: var(--font-display, 'DM Sans', sans-serif);
  font-size: var(--text-sm, 14px);
  font-weight: var(--fw-medium, 500);
  letter-spacing: 0.18em;
  text-transform: uppercase;
  color: var(--color-text-tertiary, #64748B);
  margin: 0 0 var(--sp-1, 4px);
}
.page-header-title {
  font-family: var(--font-text, 'Noto Sans SC', sans-serif);
  font-size: var(--fs-3xl, 30px);
  font-weight: var(--fw-bold, 700);
  margin: 0;
  color: var(--color-text, #0F172A);
  line-height: 1.2;
  letter-spacing: -0.025em;
}
.page-header-right {
  display: flex;
  gap: var(--sp-2, 8px);
  flex-shrink: 0;
}

/* ===== 移动端适配 ===== */
@media (max-width: 768px) {
  .page-header {
    flex-direction: column;
    align-items: flex-start;
    gap: var(--sp-4, 16px);
    padding: var(--sp-6, 24px) 0 var(--sp-4, 16px);
    margin-bottom: var(--sp-4, 16px);
  }
  .page-header-title {
    font-size: var(--fs-2xl, 24px);
  }
  .page-header-right {
    width: 100%;
    flex-wrap: wrap;
  }
}
</style>
