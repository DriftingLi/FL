// 残值结果大字号卡片（官网风格：渐变底色 + 蓝绿渐变主数字 + 三段指标）
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
      <span class="result-card-tag">实时计算</span>
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
        <div class="metric-value num metric-value-accent">{{ formatPercent(rate) }}</div>
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
  background: linear-gradient(180deg, #F0F9FF 0%, #FFFFFF 60%, #F0FDFA 100%);
  border-color: var(--color-brand-200, #BAE6FD);
}
.result-card-head {
  display: flex;
  align-items: center;
  justify-content: space-between;
  margin-bottom: var(--sp-5, 20px);
  gap: var(--sp-3, 12px);
}
.result-card-label {
  font-family: var(--font-display, 'DM Sans', sans-serif);
  font-size: var(--text-sm, 14px);
  font-weight: var(--fw-medium, 500);
  letter-spacing: 0.18em;
  text-transform: uppercase;
  color: var(--color-text-tertiary, #64748B);
}
.result-card-tag {
  display: inline-flex;
  align-items: center;
  gap: 4px;
  font-size: 11px;
  font-weight: var(--fw-medium, 500);
  color: var(--color-brand-700, #0369A1);
  background: var(--color-brand-50, #F0F9FF);
  padding: 4px 10px;
  border-radius: var(--radius-full, 9999px);
  border: 1px solid var(--color-brand-200, #BAE6FD);
  letter-spacing: 0.04em;
}
.result-card-tag::before {
  content: '';
  width: 6px;
  height: 6px;
  border-radius: 50%;
  background: var(--color-brand-500, #0EA5E9);
  box-shadow: 0 0 0 3px rgba(14, 165, 233, 0.2);
  animation: pulse-dot 2s ease-in-out infinite;
}
@keyframes pulse-dot {
  0%, 100% { opacity: 1; transform: scale(1); }
  50% { opacity: 0.7; transform: scale(0.9); }
}
.result-card-value {
  display: flex;
  align-items: baseline;
  gap: 12px;
}
.result-card-unit {
  font-family: var(--font-text, 'Noto Sans SC', sans-serif);
  font-size: 28px;
  font-weight: var(--fw-semibold, 600);
  color: var(--color-text, #0F172A);
  letter-spacing: normal;
}
.result-card-suffix {
  margin: var(--sp-2, 8px) 0 0;
  font-size: var(--text-sm, 14px);
  color: var(--color-text-tertiary, #64748B);
  letter-spacing: 0.04em;
}
.result-card-divider {
  height: 1px;
  background: linear-gradient(
    90deg,
    transparent 0%,
    var(--color-brand-200, #BAE6FD) 50%,
    transparent 100%
  );
  margin: var(--sp-6, 24px) 0;
}
.metric-row {
  display: grid;
  grid-template-columns: 1fr 1fr 1fr;
  gap: var(--sp-4, 16px);
}
.metric {
  display: flex;
  flex-direction: column;
  gap: 4px;
}
.metric-label {
  font-size: var(--text-xs, 12px);
  color: var(--color-text-tertiary, #64748B);
}
.metric-value {
  font-size: var(--fs-lg, 18px);
  font-weight: var(--fw-semibold, 600);
  color: var(--color-text, #0F172A);
  font-family: var(--font-mono, 'JetBrains Mono', monospace);
  font-feature-settings: 'tnum' 1;
}
.metric-value-accent {
  background: var(--gradient-brand, linear-gradient(135deg, #0EA5E9, #14B8A6));
  -webkit-background-clip: text;
  background-clip: text;
  -webkit-text-fill-color: transparent;
  color: transparent;
}

/* ===== 移动端适配 ===== */
@media (max-width: 768px) {
  .result-card {
    padding: var(--sp-6, 24px) var(--sp-4, 16px);
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
    gap: var(--sp-3, 12px);
  }
  .metric-value {
    font-size: var(--fs-md, 16px);
  }
}
</style>
