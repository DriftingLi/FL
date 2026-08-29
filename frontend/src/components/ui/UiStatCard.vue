<script setup lang="ts">
/**
 * 统计卡：自写。Dashboard 与各类概览页的指标块。
 *
 * 数值用 tabular-nums 保证等宽，避免刷新时数字跳动导致宽度抖动。
 * trend 为正显示上升、为负显示下降；reduced-motion 由 global.css 统一压平。
 */
const props = withDefaults(
  defineProps<{
    label: string
    value?: number | string
    unit?: string
    /** 环比变化，正=上升 负=下降；不传则不显示 */
    trend?: number
    icon?: string
    tone?: 'brand' | 'ok' | 'warn' | 'bad' | 'neutral'
    loading?: boolean
  }>(),
  { tone: 'brand', loading: false }
)

const TONE: Record<string, string> = {
  brand: 'bg-ui-50 text-ui-600',
  ok: 'bg-ok-soft text-ok',
  warn: 'bg-warn-soft text-warn',
  bad: 'bg-bad-soft text-bad',
  neutral: 'bg-canvas text-ink-2'
}
</script>

<template>
  <div class="rounded-card border border-line bg-panel p-5">
    <div class="flex items-start justify-between gap-3">
      <div class="min-w-0 flex-1">
        <p class="text-xs text-ink-3">{{ props.label }}</p>

        <div v-if="props.loading" class="mt-2 h-7 w-16 animate-pulse rounded-ctl bg-canvas" />
        <p v-else class="mt-1 flex items-baseline gap-1">
          <span class="font-heading text-2xl font-semibold tabular-nums text-ink">
            {{ props.value ?? '—' }}
          </span>
          <span v-if="props.unit" class="text-xs text-ink-3">{{ props.unit }}</span>
        </p>

        <p v-if="props.trend !== undefined" class="mt-1 text-xs" :class="props.trend >= 0 ? 'text-ok' : 'text-bad'">
          {{ props.trend >= 0 ? '↑' : '↓' }} {{ Math.abs(props.trend) }}%
        </p>
      </div>

      <span
        v-if="props.icon"
        class="grid h-9 w-9 shrink-0 place-items-center rounded-ctl"
        :class="TONE[props.tone]"
      >
        <el-icon><component :is="props.icon" /></el-icon>
      </span>
    </div>

    <slot />
  </div>
</template>
