// 残值结果大字号卡片（设计稿：白底细边框 + Electric Blue 主数字 + 三段指标）
<script setup lang="ts">
import { computed } from 'vue'
import { formatWan, formatPercent } from '@/utils/valuationFormat'

interface Props {
  estimatedValue: number
  confidenceLow: number
  confidenceHigh: number
  originalPrice: number
}

const props = defineProps<Props>()

const rate = computed(() => {
  if (!props.originalPrice || props.originalPrice <= 0) return 0
  return props.estimatedValue / props.originalPrice
})
</script>

<template>
  <div class="result-card card-surface">
    <div class="result-card-head">
      <span class="result-card-label">estimated residual value</span>
    </div>

    <div class="result-card-value num-hero">
      <span>{{ (Number(estimatedValue) / 10000).toFixed(2) }}</span>
      <span class="result-card-unit">万元</span>
    </div>

    <p class="result-card-suffix">残值区间 · 残值率</p>

    <div class="result-card-divider" />

    <div class="metric-row">
      <div class="metric">
        <div class="metric-label">置信下限</div>
        <div class="metric-value num">{{ formatWan(confidenceLow) }}</div>
      </div>
      <div class="metric">
        <div class="metric-label">置信上限</div>
        <div class="metric-value num">{{ formatWan(confidenceHigh) }}</div>
      </div>
      <div class="metric">
        <div class="metric-label">残值率</div>
        <div class="metric-value num">{{ formatPercent(rate) }}</div>
      </div>
    </div>
  </div>
</template>

<style scoped>
.result-card {
  min-height: 320px;
  display: flex;
  flex-direction: column;
  justify-content: center;
}
.result-card-head {
  margin-bottom: var(--sp-5);
}
.result-card-label {
  font-size: var(--fs-sm);
  font-weight: var(--fw-medium);
  letter-spacing: 0.18em;
  text-transform: uppercase;
  color: var(--color-text-tertiary);
}
.result-card-value {
  display: flex;
  align-items: baseline;
  gap: 12px;
}
.result-card-unit {
  font-family: var(--font-text);
  font-size: 28px;
  font-weight: var(--fw-medium);
  color: var(--color-text);
  letter-spacing: normal;
}
.result-card-suffix {
  margin: var(--sp-2) 0 0;
  font-size: var(--fs-sm);
  color: var(--color-text-tertiary);
  letter-spacing: 0.04em;
}
.result-card-divider {
  height: 1px;
  background: var(--color-border);
  margin: var(--sp-6) 0;
}
.metric-row {
  display: grid;
  grid-template-columns: 1fr 1fr 1fr;
  gap: var(--sp-4);
}
.metric {
  display: flex;
  flex-direction: column;
  gap: 4px;
}
.metric-label {
  font-size: var(--fs-xs);
  color: var(--color-text-tertiary);
}
.metric-value {
  font-size: var(--fs-lg);
  font-weight: var(--fw-medium);
  color: var(--color-text);
  font-family: var(--font-mono);
  font-feature-settings: 'tnum' 1;
}

/* ===== 移动端适配 ===== */
@media (max-width: 768px) {
  .result-card {
    padding: var(--sp-6) var(--sp-4);
    min-height: auto;
  }
  .result-card-value {
    font-size: 44px;
  }
  .result-card-unit {
    font-size: 20px;
  }
  .metric-row {
    grid-template-columns: 1fr;
    gap: var(--sp-3);
  }
  .metric-value {
    font-size: var(--fs-md);
  }
}
</style>
