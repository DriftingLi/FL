<script setup lang="ts">
/**
 * 进度条：包 `el-progress`，用 tone 表达语义色。
 * 颜色传 CSS 变量字符串而非 hex，随品牌 token 联动换肤。
 */
const props = withDefaults(
  defineProps<{
    /** 0 - 100 */
    value: number
    tone?: 'brand' | 'ok' | 'warn' | 'bad'
    size?: 'sm' | 'md' | 'lg'
    showLabel?: boolean
  }>(),
  { tone: 'brand', size: 'md', showLabel: false }
)

const TONE: Record<string, string> = {
  brand: 'var(--color-ui-500)',
  ok: 'var(--color-success)',
  warn: 'var(--color-warning)',
  bad: 'var(--color-danger)'
}

const STROKE: Record<string, number> = { sm: 4, md: 8, lg: 12 }
</script>

<template>
  <el-progress
    :percentage="Math.min(Math.max(props.value, 0), 100)"
    :color="TONE[props.tone]"
    :stroke-width="STROKE[props.size]"
    :show-text="props.showLabel"
  />
</template>
