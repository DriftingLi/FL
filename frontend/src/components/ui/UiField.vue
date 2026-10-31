<script setup lang="ts">
/**
 * 字段（#2156 表单批）：一个字段的容器，承载**无效态**与朝向。
 *
 * 无效态是「位置 + 标记」两件事，缺一不可：
 * - `data-invalid` 挂在这里 —— 样式据此把标签、描边、错误文案变红（`:has()` / 属性选择器）；
 * - `aria-invalid` 挂在**控件**上（由调用方传给 `UiInput` / `UiSelect` 等）。
 *
 * 朝向：`vertical`（默认，标签在上）与 `horizontal`（标签在左）。用 `data-orientation`
 * 表达，样式按属性选择器分档，与分段控件的 `data-*` 口径一致。
 */
const props = withDefaults(
  defineProps<{
    /** 该字段当前是否处于校验失败态 */
    invalid?: boolean
    orientation?: 'vertical' | 'horizontal'
  }>(),
  { invalid: false, orientation: 'vertical' }
)
</script>

<template>
  <div
    data-slot="field"
    role="group"
    :data-invalid="props.invalid ? '' : undefined"
    :data-orientation="props.orientation"
    class="group/field flex w-full min-w-0 flex-col data-[orientation=horizontal]:flex-row data-[orientation=horizontal]:items-center data-[orientation=horizontal]:gap-3"
  >
    <slot />
  </div>
</template>
