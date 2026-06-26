// 电池 RUL 结果大字号卡片（Tesla 极简风：白底 + Electric Blue 主数字）
// 与叉车 ResultCard 视觉一致，但显示 RUL 循环数 + SOH%
<script setup lang="ts">
import { computed } from 'vue'

interface Props {
  // RUL 循环数（主数字）
  rulCycles: number
  // SOH 健康度百分比（次要大字）
  sohPercent: number
  // 置信区间
  confidenceLow: number
  confidenceHigh: number
  // 置信度 0~1
  confidence: number
}

const props = defineProps<Props>()

const sohColor = computed(() => {
  if (props.sohPercent >= 90) return '#52c41a' // green
  if (props.sohPercent >= 80) return '#faad14' // amber
  return '#ff4d4f' // red
})
</script>

<template>
  <div class="battery-result-card card-surface">
    <div class="battery-result-head">
      <span class="battery-result-label">remaining useful life</span>
    </div>

    <!-- 主数字：RUL 循环数 -->
    <div class="battery-result-value num-hero">
      <span>{{ rulCycles }}</span>
      <span class="battery-result-unit">循环</span>
    </div>

    <p class="battery-result-suffix">预测剩余循环数</p>

    <div class="battery-result-divider" />

    <!-- 次要指标：SOH 进度条 + 置信区间 -->
    <div class="metric-row">
      <div class="metric">
        <div class="metric-label">健康度 SOH</div>
        <div class="metric-value num" :style="{ color: sohColor }">
          {{ sohPercent.toFixed(1) }}%
        </div>
        <el-progress
          :percentage="sohPercent"
          :color="sohColor"
          :show-text="false"
          :stroke-width="4"
          style="margin-top: 6px"
        />
      </div>
      <div class="metric">
        <div class="metric-label">置信区间</div>
        <div class="metric-value num">
          {{ confidenceLow }} ~ {{ confidenceHigh }}
        </div>
      </div>
      <div class="metric">
        <div class="metric-label">预测置信度</div>
        <div class="metric-value num">
          {{ (confidence * 100).toFixed(1) }}%
        </div>
      </div>
    </div>
  </div>
</template>

<style scoped>
.battery-result-card {
  padding: var(--sp-7) var(--sp-8);
  display: flex;
  flex-direction: column;
  gap: var(--sp-4);
  min-height: 320px;
}
.battery-result-head {
  display: flex;
  align-items: center;
  justify-content: space-between;
}
.battery-result-label {
  font-size: var(--fs-sm);
  font-weight: var(--fw-medium);
  letter-spacing: 0.12em;
  text-transform: uppercase;
  color: var(--color-text-tertiary);
}
.battery-result-value {
  display: flex;
  align-items: baseline;
  gap: var(--sp-3);
  color: var(--color-primary);
  margin: var(--sp-2) 0 0;
}
.battery-result-unit {
  font-size: 28px;
  font-weight: var(--fw-medium);
  color: var(--color-text);
}
.battery-result-suffix {
  margin: 0;
  font-size: var(--fs-sm);
  color: var(--color-text-tertiary);
}
.battery-result-divider {
  height: 1px;
  background: var(--color-border);
  margin: var(--sp-3) 0;
}
.metric-row {
  display: grid;
  grid-template-columns: 1fr 1fr 1fr;
  gap: var(--sp-5);
}
.metric {
  display: flex;
  flex-direction: column;
  gap: 2px;
}
.metric-label {
  font-size: var(--fs-sm);
  color: var(--color-text-tertiary);
}
.metric-value {
  font-size: var(--fs-lg);
  font-weight: var(--fw-medium);
  color: var(--color-text);
}

/* ===== 移动端适配 ===== */
@media (max-width: 768px) {
  .battery-result-card {
    padding: var(--sp-5) var(--sp-4);
    min-height: auto;
    gap: var(--sp-3);
  }
  .battery-result-value {
    font-size: 44px;
  }
  .battery-result-unit {
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
