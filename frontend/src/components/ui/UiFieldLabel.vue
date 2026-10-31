<script setup lang="ts">
/**
 * 字段标签（#2156 表单批）。
 *
 * 变红不靠 prop：父级 `UiField` 处于无效态时会给 `data-invalid`，这里用
 * `group-data-[invalid]/field:text-bad` 跟随 —— 调用方不必同步两处状态。
 */
withDefaults(
  defineProps<{
    /** 关联控件 id；省略时不渲染 for（如一组开关的组标签） */
    forId?: string
    /** 显示必填星号（星号对读屏隐藏，由控件自己的 required/aria-required 表达） */
    required?: boolean
  }>(),
  { forId: '', required: false }
)
</script>

<template>
  <label
    data-slot="field-label"
    :for="forId || undefined"
    class="block text-sm font-medium text-ink-2 transition-colors group-data-[orientation=horizontal]/field:whitespace-nowrap group-data-[invalid]/field:text-bad"
  >
    <slot />
    <span v-if="required" class="ml-0.5 text-bad" aria-hidden="true">*</span>
  </label>
</template>
