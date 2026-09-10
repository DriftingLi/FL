<script lang="ts">
/** tone 的类型导出：供调用方的映射表/函数收窄返回类型（如 Record<string, UiTagTone>） */
export type UiTagTone = 'brand' | 'primary' | 'success' | 'info' | 'warning' | 'danger' | 'neutral'
</script>

<script setup lang="ts">
/**
 * 标签：包 `el-tag`，用 tone 表达语义而非 EP 的 type。
 *
 * brand 走 EP 的 info 底色 + 品牌文字色（EP 无对应 type），其余直接映射。
 *
 * 约定（R2 纯增量）：effect 是后加的，默认 'light' 等同 EP 自身默认值，
 * 让存量的 `<UiTag tone="success">` 零 diff。
 *
 * 约定（#766 C2）：tone 是全站唯一标签入口（用户决策——不另建 type 直通薄封装）。
 * 为承接 78 处 el-tag 迁移：tone 扩展 primary / info 直通（EP 原值）；size 兼容
 * EP 原值（small/default/large）与旧值（sm/md）；attrs 显式透传（class/事件等）。
 * 迁移时 `type="success"` → `tone="success"`、`size="small"` 原样保留。
 */
import { computed } from 'vue'

const props = withDefaults(
  defineProps<{
    tone?: 'brand' | 'primary' | 'success' | 'info' | 'warning' | 'danger' | 'neutral'
    size?: 'sm' | 'md' | 'small' | 'default' | 'large'
    /**
     * 填充效果，直接透传给 EP 的 effect：
     * light 淡底（默认）/ dark 实心 / plain 描边。
     * 论坛的「✓ 已解决」用 dark 强调，「求助」用 plain 弱化。
     */
    effect?: 'light' | 'dark' | 'plain'
  }>(),
  { tone: 'neutral', size: 'sm', effect: 'light' }
)

const EP_TYPE: Record<string, 'primary' | 'success' | 'warning' | 'danger' | 'info'> = {
  brand: 'info',
  primary: 'primary',
  success: 'success',
  info: 'info',
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

// 旧值 sm/md 与 EP 原值 small/default/large 并存：前者是论坛域的历史 API，后者
// 供 el-tag 迁移直通（78 处全是 size="small"，不改写调用点）。
const sizeValue = computed(() => {
  if (props.size === 'sm') return 'small'
  if (props.size === 'md') return 'default'
  return props.size
})
</script>

<template>
  <el-tag
    :type="EP_TYPE[props.tone]"
    :effect="props.effect"
    :size="sizeValue"
    v-bind="$attrs"
    :class="toneClass"
  >
    <slot />
  </el-tag>
</template>
