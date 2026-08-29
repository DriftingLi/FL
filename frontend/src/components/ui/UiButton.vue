<script setup lang="ts">
/**
 * 按钮：包 `el-button`，收敛 variant / block / 图标写法。
 *
 * 颜色与圆角沿用 element-overrides.css 对 `.el-button--primary` 的渐变定义，
 * 此处不覆盖色值，只做语义化封装。
 */
const props = withDefaults(
  defineProps<{
    variant?: 'primary' | 'secondary' | 'ghost' | 'danger'
    size?: 'small' | 'default' | 'large'
    loading?: boolean
    block?: boolean
    /** 全局注册的图标组件名 */
    icon?: string
  }>(),
  { variant: 'secondary', size: 'default', loading: false, block: false }
)

const EP_TYPE: Record<string, 'primary' | 'default' | 'danger'> = {
  primary: 'primary',
  secondary: 'default',
  ghost: 'default',
  danger: 'danger'
}
</script>

<template>
  <el-button
    :type="EP_TYPE[props.variant]"
    :size="props.size"
    :loading="props.loading"
    :plain="props.variant === 'ghost'"
    :class="props.block ? 'w-full' : undefined"
  >
    <el-icon v-if="props.icon">
      <component :is="props.icon" />
    </el-icon>
    <slot />
  </el-button>
</template>
