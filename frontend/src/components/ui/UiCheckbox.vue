<script setup lang="ts">
/**
 * 多选框：包 `el-checkbox`，纯透传薄封装。
 *
 * 约定（#766 C2）：不设项目默认值，attrs / 事件 / slot 全量透传，迁移只换标签名。
 * 两种用法都支持 ——
 * - 单体：`<UiCheckbox v-model="x" label="...">`（v-model 经 defineModel 转发）
 * - 组内：`<UiCheckboxGroup v-model="arr"><UiCheckbox value="a" label="A" /></UiCheckboxGroup>`
 *   （group 模式下不传 v-model，value/label 走 attrs；状态由 group 统一管理）
 */
defineOptions({ name: 'UiCheckbox', inheritAttrs: false })

const value = defineModel<boolean | string | number | unknown[]>()
</script>

<template>
  <el-checkbox v-model="value" v-bind="$attrs">
    <template v-for="(_, name) in $slots" :key="name" #[name]="slotProps">
      <slot :name="name" v-bind="slotProps ?? {}" />
    </template>
  </el-checkbox>
</template>
