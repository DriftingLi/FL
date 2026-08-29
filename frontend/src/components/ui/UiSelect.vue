<script lang="ts">
export interface UiSelectOption {
  label: string
  value: string | number
}
</script>

<script setup lang="ts">
/**
 * 下拉选择：包 `el-select`，把选项收成 `options` 数组，避免每处手写 el-option。
 * 圆角与聚焦环同样由 element-overrides.css 统一处理。
 */
const value = defineModel<string | number | undefined>({ default: undefined })

withDefaults(
  defineProps<{
    options: UiSelectOption[]
    placeholder?: string
    size?: 'small' | 'default' | 'large'
    disabled?: boolean
    clearable?: boolean
  }>(),
  { size: 'default', disabled: false, clearable: false }
)
</script>

<template>
  <el-select
    v-model="value"
    :placeholder="placeholder"
    :size="size"
    :disabled="disabled"
    :clearable="clearable"
  >
    <el-option
      v-for="opt in options"
      :key="opt.value"
      :label="opt.label"
      :value="opt.value"
    />
  </el-select>
</template>
