<script setup lang="ts">
// 残值评估首页：页面壳（表单逻辑已拆至 components/valuation/ValuationForm.vue）
import ValuationForm from '@/components/valuation/ValuationForm.vue'
</script>

<template>
  <div class="valuation-home valuation-root">
    <ValuationForm />
  </div>
</template>

<style scoped>
.valuation-home {
  background: var(--color-bg-card, #FFFFFF);
  min-height: calc(100vh - var(--header-h, 72px));
}
</style>
