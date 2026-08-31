<script setup lang="ts">
/**
 * 按钮：包 `el-button`，收敛 variant / block / 图标写法。
 *
 * 颜色与圆角沿用 element-overrides.css 对 `.el-button--primary` 的渐变定义，
 * 此处不覆盖色值，只做语义化封装。
 */
const props = withDefaults(
  defineProps<{
    /**
     * secondary 为默认值；ghost 走 EP 的 plain 样式（带描边）；
     * text 走 EP 的 text 样式（无边框、无背景，常用于行内/导航操作）。
     * text 与 danger 等色调可叠加：`<UiButton variant="text" class="text-bad">删除</UiButton>`
     * —— 原子类在 utilities 层，天然压过 vendor 层的 .el-button--danger.is-text，无需 !important。
     */
    variant?: 'primary' | 'secondary' | 'ghost' | 'text' | 'danger' | 'success' | 'warning'
    size?: 'small' | 'default' | 'large'
    loading?: boolean
    block?: boolean
    /** 全局注册的图标组件名 */
    icon?: string
  }>(),
  { variant: 'secondary', size: 'default', loading: false, block: false }
)

const EP_TYPE: Record<string, 'primary' | 'default' | 'danger' | 'success' | 'warning'> = {
  primary: 'primary',
  secondary: 'default',
  ghost: 'default',
  text: 'default',
  danger: 'danger',
  success: 'success',
  warning: 'warning'
}
</script>

<template>
  <el-button
    :type="EP_TYPE[props.variant]"
    :size="props.size"
    :loading="props.loading"
    :plain="props.variant === 'ghost'"
    :text="props.variant === 'text'"
    :class="props.block ? 'w-full' : undefined"
  >
    <el-icon v-if="props.icon">
      <component :is="props.icon" />
    </el-icon>
    <slot />
  </el-button>
</template>
