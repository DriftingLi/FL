<script setup lang="ts">
/**
 * 多选框组：包 `el-checkbox-group`，纯透传薄封装。
 *
 * 约定（#766 C2）：不设项目默认值，v-model 经 defineModel 转发，attrs / slot
 * 全量透传。组内子项直接写 UiCheckbox（单体封装同样支持组内用法）。
 */
defineOptions({ name: 'UiCheckboxGroup', inheritAttrs: false })

const value = defineModel<unknown[]>()
</script>

<template>
  <el-checkbox-group v-model="value" v-bind="$attrs">
    <template v-for="(_, name) in $slots" :key="name" #[name]="slotProps">
      <slot :name="name" v-bind="slotProps ?? {}" />
    </template>
  </el-checkbox-group>
</template>
