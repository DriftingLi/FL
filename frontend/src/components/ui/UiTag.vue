<script setup lang="ts">
/**
 * 标签：包 `el-tag`，用 tone 表达语义而非 EP 的 type。
 *
 * brand 走 EP 的 info 底色 + 品牌文字色（EP 无对应 type），其余直接映射。
 *
 * 约定（R2 纯增量）：effect 是后加的，默认 'light' 等同 EP 自身默认值，
 * 让存量的 `<UiTag tone="success">` 零 diff。
 */
import { computed } from 'vue'

const props = withDefaults(
  defineProps<{
    tone?: 'brand' | 'success' | 'warning' | 'danger' | 'neutral'
    size?: 'sm' | 'md'
    /**
     * 填充效果，直接透传给 EP 的 effect：
     * light 淡底（默认）/ dark 实心 / plain 描边。
     * 论坛的「✓ 已解决」用 dark 强调，「求助」用 plain 弱化。
     */
    effect?: 'light' | 'dark' | 'plain'
  }>(),
  { tone: 'neutral', size: 'sm', effect: 'light' }
)

const EP_TYPE: Record<string, 'success' | 'warning' | 'danger' | 'info'> = {
  brand: 'info',
  success: 'success',
  warning: 'warning',
  danger: 'danger',
  neutral: 'info'
}

// brand 的底色/描边只在 light 下有意义：dark 与 plain 由 EP 自己按 type 出效果，
// 再叠 bg 会把实心标签的底色盖成浅色。
const toneClass = computed(() => {
  if (props.tone !== 'brand') return undefined
  return props.effect === 'light' ? 'border-ui-200 bg-ui-50 text-ui-700' : 'text-ui-700'
})
</script>

<template>
  <el-tag
    :type="EP_TYPE[props.tone]"
    :effect="props.effect"
    :size="props.size === 'sm' ? 'small' : 'default'"
    :class="toneClass"
  >
    <slot />
  </el-tag>
</template>
