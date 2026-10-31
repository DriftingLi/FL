<script setup lang="ts">
/**
 * 字段族容器（#2156 表单批）：一组字段的纵向编排。
 *
 * 与 `UiField` 的分工：`UiFieldGroup` 管「字段之间」的间距，`UiField` 管「一个字段内部」的
 * 标签 / 说明 / 错误 / 控件。shadcn 的 `FieldGroup` 同形。
 */
</script>

<template>
  <div data-slot="field-group" class="flex w-full min-w-0 flex-col gap-4">
    <slot />
  </div>
</template>
