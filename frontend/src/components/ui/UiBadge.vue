<script setup lang="ts">
/**
 * 角标：自写。用于数字提示（未读数、待办数）与纯状态点。
 * 数量超过 max 显示为 `max+`。
 */
const props = withDefaults(
  defineProps<{
    count?: number
    tone?: 'brand' | 'ok' | 'warn' | 'bad' | 'neutral'
    /** 只显示圆点，忽略 count */
    dot?: boolean
    max?: number
  }>(),
  { count: 0, tone: 'brand', dot: false, max: 99 }
)

const TONE: Record<string, string> = {
  brand: 'bg-ui-500',
  ok: 'bg-ok',
  warn: 'bg-warn',
  bad: 'bg-bad',
  neutral: 'bg-ink-muted'
}
</script>

<template>
  <span
    v-if="props.dot"
    class="inline-block h-2 w-2 shrink-0 rounded-pill"
    :class="TONE[props.tone]"
  />
  <span
    v-else-if="props.count > 0"
    class="inline-flex min-w-[18px] items-center justify-center rounded-pill px-1.5 py-0.5 text-xs font-medium leading-none text-white"
    :class="TONE[props.tone]"
  >
    {{ props.count > props.max ? `${props.max}+` : props.count }}
  </span>
</template>
