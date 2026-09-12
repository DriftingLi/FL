<script setup lang="ts">
/**
 * 开关：包 `el-switch`，纯透传薄封装。
 *
 * 约定（#766 C2）：不设项目默认值，attrs / 事件 / slot 全量透传给 el-switch，
 * 迁移（9 处）只换标签名，行为零变化。v-model 经 defineModel 转发。
 */
defineOptions({ name: 'UiSwitch', inheritAttrs: false })

const value = defineModel<boolean | string | number>()
</script>

<template>
  <el-switch v-model="value" v-bind="$attrs">
    <template v-for="(_, name) in $slots" :key="name" #[name]="slotProps">
      <slot :name="name" v-bind="slotProps ?? {}" />
    </template>
  </el-switch>
</template>
