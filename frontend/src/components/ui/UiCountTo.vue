<script setup lang="ts">
import { onMounted, ref, watch } from 'vue'

/**
 * 数字滚动：从 from 递增到 to。
 *
 * 两点刻意设计：
 * 1. 开启「减弱动效」时直接显示终值，不做动画（无障碍要求）。
 * 2. 初始值取 to，因此 SSR / 单测环境下不依赖 requestAnimationFrame 也能显示正确数字。
 */
const props = withDefaults(
  defineProps<{
    from?: number
    to: number
    /** 动画时长（毫秒） */
    duration?: number
    decimals?: number
  }>(),
  { from: 0, duration: 800, decimals: 0 }
)

const current = ref(props.to)

function prefersReducedMotion(): boolean {
  return (
    typeof window !== 'undefined' &&
    window.matchMedia?.('(prefers-reduced-motion: reduce)').matches === true
  )
}

function run() {
  if (prefersReducedMotion() || props.duration <= 0) {
    current.value = props.to
    return
  }

  const start = performance.now()
  const delta = props.to - props.from

  const step = (now: number) => {
    const p = Math.min((now - start) / props.duration, 1)
    // easeOutCubic：起步快、收尾稳，避免末段拖沓
    current.value = props.from + delta * (1 - Math.pow(1 - p, 3))
    if (p < 1) requestAnimationFrame(step)
  }

  requestAnimationFrame(step)
}

onMounted(run)
watch(() => props.to, run)
</script>

<template>
  <span class="tabular-nums">{{ current.toFixed(props.decimals) }}</span>
</template>
