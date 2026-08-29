<script setup lang="ts">
/**
 * 标签：包 `el-tag`，用 tone 表达语义而非 EP 的 type。
 *
 * brand 走 EP 的 info 底色 + 品牌文字色（EP 无对应 type），其余直接映射。
 */
const props = withDefaults(
  defineProps<{
    tone?: 'brand' | 'success' | 'warning' | 'danger' | 'neutral'
    size?: 'sm' | 'md'
  }>(),
  { tone: 'neutral', size: 'sm' }
)

const EP_TYPE: Record<string, 'success' | 'warning' | 'danger' | 'info'> = {
  brand: 'info',
  success: 'success',
  warning: 'warning',
  danger: 'danger',
  neutral: 'info'
}
</script>

<template>
  <el-tag
    :type="EP_TYPE[props.tone]"
    :size="props.size === 'sm' ? 'small' : 'default'"
    :class="props.tone === 'brand' ? 'border-ui-200 bg-ui-50 text-ui-700' : undefined"
  >
    <slot />
  </el-tag>
</template>
