<script setup lang="ts">
/**
 * 提示气泡：包 `el-tooltip`，纯透传薄封装。
 *
 * 约定（#766 C2）：不设项目默认值，attrs / 事件 / slot（含 #content 具名槽）
 * 全量透传，迁移（16 处）只换标签名。
 * 深色下的反相缺陷不在本组件内修 —— 走 element-overrides.css 的
 * `.el-popper.is-dark` 全局覆写（见 C1，浅色保持 EP 出厂反色逻辑）。
 */
defineOptions({ name: 'UiTooltip', inheritAttrs: false })
</script>

<template>
  <el-tooltip v-bind="$attrs">
    <template v-for="(_, name) in $slots" :key="name" #[name]="slotProps">
      <slot :name="name" v-bind="slotProps ?? {}" />
    </template>
  </el-tooltip>
</template>
