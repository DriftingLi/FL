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
    /** textarea / password 等；默认 text，与 el-input 自身默认值一致 */
    type?: 'text' | 'textarea' | 'password'
    /** 仅 type="textarea" 生效。不传时 el-input 取自身默认值 2 */
    rows?: number
  }>(),
  { size: 'default', disabled: false, clearable: false, type: 'text' }
)
</script>

<template>
  <el-input
    v-model="value"
    :placeholder="placeholder"
    :size="size"
    :disabled="disabled"
    :clearable="clearable"
    :type="type"
    :rows="rows"
  />
</template>
