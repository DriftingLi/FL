<script setup lang="ts">
/**
 * 进度条：包 `el-progress`，用 tone 表达语义色。
 * 颜色传 CSS 变量字符串而非 hex，随品牌 token 联动换肤。
 *
 * 后三个 prop 是 #1977 迁移时补的（原样承接存量 el-progress 的写法，默认值 = 现状）：
 * 迁移要保真，否则「表格单元格里的粗条 + 条内百分比」与「成功/失败的 ✓✗ 图标」会丢。
 */
const props = withDefaults(
  defineProps<{
    /** 0 - 100 */
    value: number
    tone?: 'brand' | 'ok' | 'warn' | 'bad'
    size?: 'sm' | 'md' | 'lg'
    showLabel?: boolean
    /** 覆盖 size 派生的条宽（px）；迁移保真用，新代码优先用 size */
    strokeWidth?: number
    /** 百分比压在条内（EP 的 text-inside），配色由 tone 决定 */
    textInside?: boolean
    /** EP 的 status：额外给成功/失败的 ✓✗ 图标与色 */
    status?: 'success' | 'exception' | 'warning' | ''
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
    :stroke-width="props.strokeWidth ?? STROKE[props.size]"
    :show-text="props.showLabel"
    :text-inside="props.textInside"
    :status="props.status"
  />
</template>
