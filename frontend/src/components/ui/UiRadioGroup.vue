<script setup lang="ts">
/**
 * 单选组：包 `el-radio-group`，纯透传薄封装。
 *
 * 约定（#766 C2）：不设项目默认值，v-model 经 defineModel 转发，attrs / slot
 * 全量透传，迁移（8 处）只换标签名。组内子项保持 el-radio / el-radio-button 原生 ——
 * 项是内容而非容器，观感由全局变量层保证（口径记录于 #766）。
 */
defineOptions({ name: 'UiRadioGroup', inheritAttrs: false })

const value = defineModel<boolean | string | number>()
</script>

<template>
  <el-radio-group v-model="value" v-bind="$attrs">
    <template v-for="(_, name) in $slots" :key="name" #[name]="slotProps">
      <slot :name="name" v-bind="slotProps ?? {}" />
    </template>
  </el-radio-group>
</template>
