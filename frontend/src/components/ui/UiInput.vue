<script setup lang="ts">
/**
 * 输入框：包 `el-input`，统一 size 语义。
 * 圆角与聚焦环由 element-overrides.css 的 `.el-input__wrapper` 统一处理，
 * 这里不重复声明样式，只做 v-model 与 size 的收口。
 */
const value = defineModel<string>({ default: '' })

withDefaults(
  defineProps<{
    placeholder?: string
    size?: 'small' | 'default' | 'large'
    disabled?: boolean
    clearable?: boolean
  }>(),
  { size: 'default', disabled: false, clearable: false }
)
</script>

<template>
  <el-input
    v-model="value"
    :placeholder="placeholder"
    :size="size"
    :disabled="disabled"
    :clearable="clearable"
  />
</template>
